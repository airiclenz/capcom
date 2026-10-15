package index

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"time"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/tree"
)

// racyGranularity is the coarsest file mtime resolution the racy-clean rule allows for: a file
// whose mtime falls in the same second as the refresh that recorded it is re-read next time,
// since a same-size edit in that second could leave its mtime unchanged.
const racyGranularity = time.Second

// commitTx commits a refresh transaction. It is a variable so a test can make the commit fail.
var commitTx = (*sql.Tx).Commit

// readNodeFile reads a node file the walk found. It is a variable so a test can change the file
// between the walk and its read.
var readNodeFile = nodefile.ReadRegularFile

// refresh brings the index up to date with the issue tree and the Knowledge tree on disk in one
// IMMEDIATE transaction, and records its report as the one Report returns. It re-parses only the
// files that are new, changed in size or mtime, or racily clean, drops the rows of files that are
// gone, and rebuilds everything when the schema version or the configured statuses and types
// changed; isForced drops every table and indexes both trees again. It then stores the membership
// verdicts tree.DecideMembership gives on the rows it recorded. Files that cannot be used are
// reported in the Report's Warnings and left out; refresh itself fails only when the index or
// the tree cannot be read.
func (x *Index) refresh(isForced bool) (report Report, err error) {
	ctx := context.Background()
	start := x.now().Truncate(racyGranularity)
	tx, err := x.db.BeginTx(ctx, nil)
	if err != nil {
		return Report{}, fmt.Errorf("%s: begin refresh: %w", Path(x.store), err)
	}
	defer func() {
		if err == nil {
			return
		}
		// A failed commit has already ended the transaction, so its rollback reports ErrTxDone:
		// that says nothing about the failure and is left out.
		if rollbackErr := tx.Rollback(); !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, rollbackErr)
		}
	}()

	report.rebuilt, err = ensureSchema(ctx, tx, configHash(x.store.Config), isForced)
	if err != nil {
		return Report{}, x.databaseError(err)
	}
	r := &refresher{
		ctx:       ctx,
		tx:        tx,
		config:    x.store.Config,
		issues:    nodestore.IssueReader(x.store.Config),
		notes:     nodestore.NoteReader(),
		refreshed: start.UnixNano(),
	}
	found, err := walkStore(x.store, r)
	if err != nil {
		return Report{}, err
	}
	if err := r.sync(found, &report); err != nil {
		return Report{}, x.databaseError(err)
	}
	if err := r.replaceWalkClaims(); err != nil {
		return Report{}, x.databaseError(err)
	}
	if err := r.persistMembership(); err != nil {
		return Report{}, x.databaseError(err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM issues WHERE excluded = 0`).Scan(&report.Indexed); err != nil {
		return Report{}, x.databaseError(fmt.Errorf("count indexed issues: %w", err))
	}
	if err := commitTx(tx); err != nil {
		return Report{}, fmt.Errorf("%s: commit refresh: %w", Path(x.store), err)
	}

	report.Warnings = r.warnings
	slices.SortFunc(report.Warnings, func(a, b diagnose.Finding) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Message, b.Message))
	})
	x.report = report
	return report, nil
}

// walkStore lists the node files of the issue tree and then of the Knowledge tree, each in path
// order and marked with its kind, and keeps on r the claims of the files each walk leaves out and
// the node files it leaves out below a repeated id, marked with the same kind.
func walkStore(s *store.Store, r *refresher) ([]foundFile, error) {
	trees := []struct {
		dir     string
		kind    string
		content tree.ContentFunc
	}{
		{s.IssuesDir(), kindIssue, contentThrough(s.WorktreeRoot, r.issues)},
		{s.KnowledgeDir(), kindNote, contentThrough(s.WorktreeRoot, r.notes)},
	}
	var found []foundFile
	for _, current := range trees {
		files, claims, leftOut, err := walkIssues(s.WorktreeRoot, current.dir, current.content, r)
		if err != nil {
			return nil, err
		}
		for i := range files {
			files[i].kind = current.kind
		}
		for i := range claims {
			claims[i].kind = current.kind
		}
		for i := range leftOut {
			leftOut[i].kind = current.kind
		}
		found = append(found, files...)
		r.walkClaims = append(r.walkClaims, claims...)
		r.leftOut = append(r.leftOut, leftOut...)
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
	// issues and notes are the Kind readers of the issue tree and the Knowledge tree, the one
	// decode of every node file the refresh reads.
	issues nodestore.Reader[issue.Issue]
	notes  nodestore.Reader[knowledge.Note]
	// refreshed is the refresh start, truncated to racyGranularity, in Unix nanoseconds.
	refreshed int64
	warnings  []diagnose.Finding
	// walkClaims are the names the files the walk left out claim, which replaceWalkClaims stores.
	walkClaims []claim
	// leftOut are the node files the walk left out below a repeated id, which persistMembership
	// decides on beside the files rows.
	leftOut []leftOutNode
}

// warn records a warning of kind about one file.
func (r *refresher) warn(kind diagnose.Kind, path string, line int, message string) {
	r.warnings = append(
		r.warnings,
		diagnose.Finding{Code: kind, Path: path, Line: line, Message: message},
	)
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
		isGone, err := r.indexFile(file)
		if err != nil {
			return err
		}
		if isGone {
			if isKnown {
				report.removed = append(report.removed, file.path)
			}
			continue
		}
		report.parsed = append(report.parsed, file.path)
	}
	for path := range recorded {
		if err := r.forget(path); err != nil {
			return err
		}
		report.removed = append(report.removed, path)
	}
	slices.Sort(report.removed)
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
	var stored []diagnose.Finding
	if err := json.Unmarshal([]byte(problems.String), &stored); err != nil {
		return fmt.Errorf("index holds bad problems for %s: %w", path, err)
	}
	r.warnings = append(r.warnings, stored...)
	return nil
}

// indexFile reads one file, decodes it through the Kind reader of its kind and replaces its rows.
// A file with problems keeps only its files row, which carries the problems so the next refresh
// reports them again, and its claims rows, one per name claimedNames gives it, each with the
// file's parse state as readingParse gives it. The file is read
// through nodefile.ReadRegularFile: one that is no longer a regular file, or has grown past 1 MiB,
// since the walk kept it is left out unread with a diagnose.NotRegular, never failing the
// refresh. A file removed since the walk kept it is left without rows, and isGone reports it.
func (r *refresher) indexFile(file foundFile) (isGone bool, err error) {
	if err := r.forget(file.path); err != nil {
		return false, err
	}
	data, err := readNodeFile(file.absolute)
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}

	var problems []diagnose.Finding
	var aliases []string
	var insert func() error
	parse := tree.UnreadFile
	refusal := diagnose.ReadRefusal(err)
	switch {
	case refusal != nil:
		problems = []diagnose.Finding{
			{Code: diagnose.NotRegular, Path: file.path, Message: refusal.Error()},
		}
	case err != nil:
		return false, fmt.Errorf("%s: %w", file.path, err)
	case file.kind == kindNote:
		reading := r.notes.Decode(file.path, data)
		problems, aliases, parse = reading.Findings(), reading.Aliases, readingParse(reading)
		insert = func() error { return r.insertNote(file, reading.Node) }
	default:
		reading := r.issues.Decode(file.path, data)
		problems, aliases, parse = reading.Findings(), reading.Aliases, readingParse(reading)
		insert = func() error { return r.insertIssue(file, reading.Node) }
	}
	var stored sql.NullString
	if len(problems) > 0 {
		encoded, err := json.Marshal(problems)
		if err != nil {
			return false, fmt.Errorf("%s: encode problems: %w", file.path, err)
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
		return false, fmt.Errorf("index %s: %w", file.path, err)
	}
	if len(problems) > 0 {
		return false, r.insertClaims(file, claimedNames(file, aliases), parse)
	}
	return false, insert()
}

// readingParse returns the parse state of reading as tree reads the same file: refused unread,
// read but not parsed, or parsed.
func readingParse[T any](reading nodestore.Reading[T]) tree.ParseState {
	switch {
	case !reading.IsRead():
		return tree.UnreadFile
	case !reading.IsParsed():
		return tree.UnparsedFile
	}
	return tree.ParsedFile
}

// claimedNames returns the names a node file with problems claims: its id, plus aliases, the
// Aliases the Kind reader of its kind found in it: the ones its parse gives or, when its
// frontmatter does not parse, the ones it recovers. A file refused unread has none and claims its
// id only.
func claimedNames(file foundFile, aliases []string) []string {
	return append([]string{file.id}, aliases...)
}

// insertClaimQuery writes one claims row: a file's path, its kind, one name it claims and its
// parse state, as parseStateNames names it.
const insertClaimQuery = `INSERT INTO claims (path, kind, name, parse) VALUES (?, ?, ?, ?)`

// insertClaims writes one claims row per name a node file with problems claims, each with parse,
// the file's parse state.
func (r *refresher) insertClaims(file foundFile, names []string, parse tree.ParseState) error {
	statements := make([]statement, 0, len(names))
	for _, name := range names {
		args := []any{file.path, file.kind, name, parseStateNames[parse]}
		statements = append(statements, statement{insertClaimQuery, args})
	}
	return r.exec(file, statements)
}

// replaceWalkClaims drops every claims row whose path has no files row, the claims an earlier
// walk recorded for the files it left out, and stores this walk's instead, so such a claim holds
// exactly as long as the walk keeps leaving its file out. It runs after sync, once the files rows
// are current.
func (r *refresher) replaceWalkClaims() error {
	_, err := r.tx.ExecContext(r.ctx, `DELETE FROM claims WHERE path NOT IN (SELECT path FROM files)`)
	if err != nil {
		return fmt.Errorf("drop the claims of left-out files from index: %w", err)
	}
	for _, current := range r.walkClaims {
		_, err := r.tx.ExecContext(
			r.ctx, insertClaimQuery, current.path, current.kind, current.name, parseStateNames[current.parse],
		)
		if err != nil {
			return fmt.Errorf("index %s: %w", current.path, err)
		}
	}
	return nil
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
	for _, related := range parsed.Related {
		statements = append(statements, statement{
			`INSERT INTO related (path, from_id, to_id) VALUES (?, ?, ?)`, []any{file.path, file.id, related},
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
