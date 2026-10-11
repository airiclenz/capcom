package index

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"time"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/tree"
)

// racyGranularity is the coarsest file mtime resolution the racy-clean rule allows for: a file
// whose mtime falls in the same second as the refresh that recorded it is re-read next time,
// since a same-size edit in that second could leave its mtime unchanged.
const racyGranularity = time.Second

// Refresh brings the index up to date with the issue tree and the Knowledge tree on disk in one
// IMMEDIATE transaction.
// It re-parses only the files that are new, changed in size or mtime, or racily clean, drops the
// rows of files that are gone, and rebuilds everything when the schema version or the configured
// statuses and types changed. Files that cannot be used are reported in the Report's Warnings and
// left out; Refresh itself fails only when the index or the tree cannot be read.
func (x *Index) Refresh() (Report, error) {
	return x.refresh(false)
}

// Rebuild drops every table and indexes the whole issue tree and Knowledge tree again, in one
// transaction.
func (x *Index) Rebuild() (Report, error) {
	return x.refresh(true)
}

// refresh runs Refresh, or Rebuild when isForced is set.
func (x *Index) refresh(isForced bool) (report Report, err error) {
	ctx := context.Background()
	start := x.now().Truncate(racyGranularity)
	tx, err := x.db.BeginTx(ctx, nil)
	if err != nil {
		return Report{}, fmt.Errorf("%s: begin refresh: %w", Path(x.store), err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()

	report.Rebuilt, err = ensureSchema(ctx, tx, configHash(x.store.Config), isForced)
	if err != nil {
		return Report{}, x.databaseError(err)
	}
	r := &refresher{ctx: ctx, tx: tx, config: x.store.Config, refreshed: start.UnixNano()}
	found, err := walkStore(x.store, r)
	if err != nil {
		return Report{}, err
	}
	if err := r.sync(found, &report); err != nil {
		return Report{}, x.databaseError(err)
	}
	if err := r.excludeDuplicates(); err != nil {
		return Report{}, x.databaseError(err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM issues WHERE excluded = 0`).Scan(&report.Indexed); err != nil {
		return Report{}, x.databaseError(fmt.Errorf("count indexed issues: %w", err))
	}
	if err := tx.Commit(); err != nil {
		return Report{}, fmt.Errorf("%s: commit refresh: %w", Path(x.store), err)
	}

	report.Warnings = r.warnings
	slices.SortFunc(report.Warnings, func(a, b Warning) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Message, b.Message))
	})
	return report, nil
}

// walkStore lists the node files of the issue tree and then of the Knowledge tree, each in path
// order and marked with its kind.
func walkStore(s *store.Store, r *refresher) ([]foundFile, error) {
	trees := []struct {
		dir  string
		kind string
	}{{s.IssuesDir(), kindIssue}, {s.KnowledgeDir(), kindNote}}
	var found []foundFile
	for _, current := range trees {
		files, err := walkIssues(s.WorktreeRoot, current.dir, r)
		if err != nil {
			return nil, err
		}
		for i := range files {
			files[i].kind = current.kind
		}
		found = append(found, files...)
	}
	return found, nil
}

// databaseError prefixes err with the index file's path, so a failure of the database names the
// file it concerns. A file-system error from reading an issue file already names that file and is
// returned as is.
func (x *Index) databaseError(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return err
	}
	return fmt.Errorf("%s: %w", Path(x.store), err)
}

// refresher carries one refresh transaction and the warnings it collects.
type refresher struct {
	ctx    context.Context
	tx     *sql.Tx
	config config.Config
	// refreshed is the refresh start, truncated to racyGranularity, in Unix nanoseconds.
	refreshed int64
	warnings  []Warning
}

// warn records a warning about one file.
func (r *refresher) warn(path string, line int, message string) {
	r.warnings = append(r.warnings, Warning{Path: path, Line: line, Message: message})
}

// foundFile is a node file the walk found, with the stat the refresh compares.
type foundFile struct {
	// path is relative to the working tree root, with forward slashes.
	path     string
	absolute string
	// kind is kindIssue or kindNote, by the tree the file was found in.
	kind     string
	id       string
	parentID string
	size     int64
	mtime    int64
}

// recordedFile is a files row from an earlier refresh.
type recordedFile struct {
	size      int64
	mtime     int64
	refreshed int64
	problems  sql.NullString
}

// isDirty reports whether the file must be read again: its size or mtime changed, or its mtime is
// not older than the refresh that recorded it (racy-clean).
func (recorded recordedFile) isDirty(file foundFile) bool {
	return recorded.size != file.size || recorded.mtime != file.mtime || recorded.mtime >= recorded.refreshed
}

// sync re-indexes the new and changed files, keeps the clean ones and forgets the vanished ones.
func (r *refresher) sync(found []foundFile, report *Report) error {
	recorded, err := r.recordedFiles()
	if err != nil {
		return err
	}
	for _, file := range found {
		previous, isKnown := recorded[file.path]
		delete(recorded, file.path)
		if isKnown && !previous.isDirty(file) {
			if err := r.rewarn(file.path, previous.problems); err != nil {
				return err
			}
			continue
		}
		if err := r.indexFile(file); err != nil {
			return err
		}
		report.Parsed = append(report.Parsed, file.path)
	}
	for path := range recorded {
		if err := r.forget(path); err != nil {
			return err
		}
		report.Removed = append(report.Removed, path)
	}
	slices.Sort(report.Removed)
	return nil
}

// recordedFiles reads every files row, by path.
func (r *refresher) recordedFiles() (map[string]recordedFile, error) {
	rows, err := r.tx.QueryContext(r.ctx, `SELECT path, size, mtime, refreshed, problems FROM files`)
	if err != nil {
		return nil, fmt.Errorf("read indexed files: %w", err)
	}
	defer rows.Close()
	recorded := map[string]recordedFile{}
	for rows.Next() {
		var path string
		var file recordedFile
		if err := rows.Scan(&path, &file.size, &file.mtime, &file.refreshed, &file.problems); err != nil {
			return nil, fmt.Errorf("read indexed files: %w", err)
		}
		recorded[path] = file
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read indexed files: %w", err)
	}
	return recorded, nil
}

// rewarn reports again the problems stored for a clean file.
func (r *refresher) rewarn(path string, problems sql.NullString) error {
	if !problems.Valid {
		return nil
	}
	var stored []Warning
	if err := json.Unmarshal([]byte(problems.String), &stored); err != nil {
		return fmt.Errorf("index holds bad problems for %s: %w", path, err)
	}
	r.warnings = append(r.warnings, stored...)
	return nil
}

// indexFile reads and parses one file and replaces its rows. A file with problems keeps only its
// files row, which carries the problems so the next refresh reports them again.
func (r *refresher) indexFile(file foundFile) error {
	if err := r.forget(file.path); err != nil {
		return err
	}
	data, err := os.ReadFile(file.absolute)
	if errors.Is(err, fs.ErrNotExist) {
		// Removed since the walk saw it; the next refresh no longer finds it.
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s: %w", file.path, err)
	}

	var problems []Warning
	var insert func() error
	if file.kind == kindNote {
		note, noteProblems := parseNote(file, data)
		problems, insert = noteProblems, func() error { return r.insertNote(file, note) }
	} else {
		parsed, issueProblems := parseIssue(file, data, r.config)
		problems, insert = issueProblems, func() error { return r.insertIssue(file, parsed) }
	}
	var stored sql.NullString
	if len(problems) > 0 {
		encoded, err := json.Marshal(problems)
		if err != nil {
			return fmt.Errorf("%s: encode problems: %w", file.path, err)
		}
		stored = sql.NullString{String: string(encoded), Valid: true}
		r.warnings = append(r.warnings, problems...)
	}
	_, err = r.tx.ExecContext(
		r.ctx,
		`INSERT INTO files (path, kind, size, mtime, refreshed, problems) VALUES (?, ?, ?, ?, ?, ?)`,
		file.path, file.kind, file.size, file.mtime, r.refreshed, stored,
	)
	if err != nil {
		return fmt.Errorf("index %s: %w", file.path, err)
	}
	if len(problems) > 0 {
		return nil
	}
	return insert()
}

// parseIssue parses and validates one issue file and returns every problem that keeps it out of
// the index.
func parseIssue(file foundFile, data []byte, cfg config.Config) (issue.Issue, []Warning) {
	parsed, err := issue.Parse(file.path, data)
	if err != nil {
		var failure *issue.Error
		if errors.As(err, &failure) {
			return issue.Issue{}, []Warning{{Path: failure.Path, Line: failure.Line, Message: failure.Message}}
		}
		return issue.Issue{}, []Warning{{Path: file.path, Message: err.Error()}}
	}

	var problems []Warning
	for _, failure := range issue.Validate(file.path, parsed, cfg) {
		problems = append(problems, Warning{Path: failure.Path, Line: failure.Line, Message: failure.Message})
	}
	return parsed, append(problems, aliasProblems(file, parsed.Aliases)...)
}

// aliasProblems returns a problem for every alias of the node file that is invalid, repeats the
// node's own id or is listed twice.
func aliasProblems(file foundFile, aliases []string) []Warning {
	var problems []Warning
	seen := map[string]bool{}
	for _, alias := range aliases {
		aliasErr := tree.ValidateAlias(alias)
		switch {
		case aliasErr != nil:
			problems = append(problems, Warning{Path: file.path, Message: aliasErr.Error()})
		case alias == file.id:
			problems = append(problems, Warning{Path: file.path, Message: fmt.Sprintf("alias %q repeats the %s's own id", alias, file.kind)})
		case seen[alias]:
			problems = append(problems, Warning{Path: file.path, Message: fmt.Sprintf("alias %q is listed twice", alias)})
		}
		seen[alias] = true
	}
	return problems
}

// statement is one SQL statement with its arguments.
type statement struct {
	query string
	args  []any
}

// insertIssue writes the rows of one valid issue.
func (r *refresher) insertIssue(file foundFile, parsed issue.Issue) error {
	category, _ := r.config.CategoryOf(parsed.Status) // Validate has checked the status is declared
	statements := []statement{{
		`INSERT INTO issues (path, id, parent, status, category, type, priority, plan, created,
			closed, reason, title) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		[]any{
			file.path, file.id, file.parentID, parsed.Status, string(category), parsed.Type,
			parsed.Priority, parsed.Plan, formatTime(parsed.Created), formatTime(parsed.Closed),
			parsed.Reason, parsed.Title,
		},
	}, {
		`INSERT INTO fts (path, kind, title, body) VALUES (?, ?, ?, ?)`,
		[]any{file.path, kindIssue, parsed.Title, parsed.Body},
	}}
	statements = append(statements, sharedListStatements(file, parsed.Labels, parsed.Aliases)...)
	for _, dep := range parsed.Deps {
		statements = append(statements, statement{
			`INSERT INTO deps (path, from_id, to_id) VALUES (?, ?, ?)`, []any{file.path, file.id, dep},
		})
	}
	return r.exec(file, statements)
}

// sharedListStatements are the rows of the labels and aliases a node file of either kind records.
func sharedListStatements(file foundFile, labels []string, aliases []string) []statement {
	statements := make([]statement, 0, len(labels)+len(aliases))
	for _, label := range labels {
		statements = append(statements, statement{
			`INSERT INTO labels (path, kind, label) VALUES (?, ?, ?)`, []any{file.path, file.kind, label},
		})
	}
	for _, alias := range aliases {
		statements = append(statements, statement{
			`INSERT INTO aliases (path, kind, alias) VALUES (?, ?, ?)`, []any{file.path, file.kind, alias},
		})
	}
	return statements
}

// exec runs the statements that index one file, in order.
func (r *refresher) exec(file foundFile, statements []statement) error {
	for _, current := range statements {
		if _, err := r.tx.ExecContext(r.ctx, current.query, current.args...); err != nil {
			return fmt.Errorf("index %s: %w", file.path, err)
		}
	}
	return nil
}

// forget drops every row of one file.
func (r *refresher) forget(path string) error {
	for _, table := range rowTables {
		if _, err := r.tx.ExecContext(r.ctx, `DELETE FROM `+table+` WHERE path = ?`, path); err != nil {
			return fmt.Errorf("drop %s from index: %w", path, err)
		}
	}
	return nil
}

// claim is one name a node answers to: its id or one of its Aliases.
type claim struct {
	name string
	path string
	kind string
}

// excludeDuplicates keeps every node that shares an id or alias with another file of its own tree
// out of the queries and warns about each, naming the other file (ADR 0002: names are unique
// across the tree). An issue and a Note never clash. It runs on every refresh, so a node comes
// back once its rival is gone.
func (r *refresher) excludeDuplicates() error {
	for _, nodes := range []nodeTable{issueNodes, noteNodes} {
		if err := r.excludeDuplicatesOf(nodes); err != nil {
			return err
		}
	}
	return nil
}

// excludeDuplicatesOf runs excludeDuplicates over the nodes of one kind.
func (r *refresher) excludeDuplicatesOf(nodes nodeTable) error {
	rows, err := r.tx.QueryContext(r.ctx, `SELECT id, path, 'id' FROM `+nodes.table+`
		UNION ALL SELECT alias, path, 'alias' FROM aliases WHERE kind = ? ORDER BY 1, 2`, nodes.kind)
	if err != nil {
		return fmt.Errorf("read indexed names: %w", err)
	}
	byName := map[string][]claim{}
	var names []string
	for rows.Next() {
		var current claim
		if err := rows.Scan(&current.name, &current.path, &current.kind); err != nil {
			rows.Close()
			return fmt.Errorf("read indexed names: %w", err)
		}
		if _, isSeen := byName[current.name]; !isSeen {
			names = append(names, current.name)
		}
		byName[current.name] = append(byName[current.name], current)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return fmt.Errorf("read indexed names: %w", err)
	}

	if _, err := r.tx.ExecContext(r.ctx, `UPDATE `+nodes.table+` SET excluded = 0`); err != nil {
		return fmt.Errorf("reset excluded %s: %w", nodes.table, err)
	}
	for _, name := range names {
		if err := r.excludeClaims(nodes, byName[name]); err != nil {
			return err
		}
	}
	return nil
}

// excludeClaims excludes and warns about every claimant of one name when there is more than one.
func (r *refresher) excludeClaims(nodes nodeTable, claims []claim) error {
	if len(claims) < 2 {
		return nil
	}
	for _, current := range claims {
		for _, other := range claims {
			if other == current {
				continue
			}
			r.warn(current.path, 0, fmt.Sprintf(
				"%v %s %q: also the %s of %s; both are left out",
				tree.ErrDuplicate, current.kind, current.name, other.kind, other.path,
			))
		}
		if _, err := r.tx.ExecContext(r.ctx, `UPDATE `+nodes.table+` SET excluded = 1 WHERE path = ?`, current.path); err != nil {
			return fmt.Errorf("exclude %s: %w", current.path, err)
		}
	}
	return nil
}
