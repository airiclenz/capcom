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
	"strings"
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

// racyGranularity is the coarsest file time resolution the racy-clean rule allows for: a file
// whose mtime or change time falls in the same second as the refresh that recorded it is re-read
// next time, since a same-size edit in that second could leave both unchanged.
const racyGranularity = time.Second

// commitTx commits a refresh transaction. It is a variable so a test can make the commit fail.
var commitTx = (*sql.Tx).Commit

// readNodeFile reads a node file the walk found. It is a variable so a test can change the file
// between the walk and its read.
var readNodeFile = nodefile.ReadRegularFile

// Refresh brings the open index up to date with the issue tree and the Knowledge tree on disk,
// the same incremental refresh Open runs, and records its report as the one Report returns. It
// reports true when any issue or Note was added, changed or removed since the previous refresh or
// rebuild: a file was parsed again or its rows dropped, every table was rebuilt, or the files the
// walk leaves out (oversize, not regular, below an orphan directory) claim other names than
// before. With no file changed since a refresh that recorded it clean, it re-parses nothing and
// reports false; a racily clean file is read once more and counts as changed. On failure it
// returns false with the error, and Report keeps the previous refresh's report; the error matches
// ErrBusy when this refresh had changes to write and another command kept the index locked past
// the busy timeout. A refresh with nothing to write takes no write lock, so it never waits on
// another command's write.
func (x *Index) Refresh() (bool, error) {
	report, err := x.refresh(false)
	if err != nil {
		return false, err
	}
	return report.isChanged(), nil
}

// refresh brings the index up to date with the issue tree and the Knowledge tree on disk, and
// records its report as the one Report returns. An unforced refresh first walks both trees and
// checks, in a read transaction that takes no write lock, whether there is anything to write: a
// schema version or configured statuses and types changed since the index was recorded, a file new,
// changed in size, mtime, change time or identity, racily clean or gone, changed walk claims or
// changed membership verdicts. With none it returns the Report a write refresh would give, writing
// nothing, so it never waits on another command's write. Otherwise, and always when isForced, it
// writes in one IMMEDIATE transaction that walks both trees again under the lock, so it never
// forgets a file another command indexed after the read pass: it re-parses only the files that
// changed, drops the rows of files that are gone, and rebuilds everything when the schema or the
// configuration changed; isForced drops every table and indexes both trees again. It then stores
// the membership verdicts tree.DecideMembership gives on the rows it recorded. Files that cannot be
// used are reported in the Report's Warnings and left out; refresh itself fails only when the index
// or the tree cannot be read, with ErrBusy when another command held the index locked while this
// one had changes to write.
func (x *Index) refresh(isForced bool) (report Report, err error) {
	ctx := context.Background()
	start := x.now().Truncate(racyGranularity)
	// Deferred first so it runs last: any failure of either transaction reads as ErrBusy when
	// another command held the index locked.
	defer func() { err = asBusyError(Path(x.store), err) }()
	if !isForced {
		var isUnchanged bool
		report, isUnchanged, err = x.readRefresh(ctx, start)
		if err != nil || isUnchanged {
			return report, err
		}
	}
	return x.writeRefresh(ctx, start, isForced)
}

// newRefresher returns a refresher on tx for a refresh begun at start.
func (x *Index) newRefresher(ctx context.Context, tx *sql.Tx, start time.Time) *refresher {
	return &refresher{
		ctx:          ctx,
		tx:           tx,
		config:       x.store.Config,
		issues:       nodestore.IssueReader(x.store.Config),
		notes:        nodestore.NoteReader(),
		worktreeRoot: x.store.WorktreeRoot,
		refreshed:    start.UnixNano(),
	}
}

// readRefresh walks the store and checks, in a deferred read transaction that never takes the write
// lock, whether a write refresh would change anything: the schema and configuration are current,
// every found file is recorded clean, no recorded file is gone, and the walk claims and membership
// verdicts are the stored ones. isUnchanged reports that all hold; report is then the Report a
// write refresh would give, warnings included, and is recorded as the one Report returns. The read
// transaction ends before readRefresh returns, so it never upgrades to a write lock and two
// refreshes never deadlock.
func (x *Index) readRefresh(
	ctx context.Context,
	start time.Time,
) (report Report, isUnchanged bool, err error) {
	tx, err := x.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Report{}, false, fmt.Errorf("%s: begin read refresh: %w", Path(x.store), err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			err = errors.Join(err, x.databaseError(fmt.Errorf("end read refresh: %w", rollbackErr)))
		}
	}()

	isCurrent, err := isSchemaCurrent(ctx, tx, configHash(x.store.Config))
	if err != nil {
		return Report{}, false, x.databaseError(err)
	}
	if !isCurrent {
		return Report{}, false, nil
	}
	r := x.newRefresher(ctx, tx, start)
	found, err := walkStore(x.store, r)
	if err != nil {
		return Report{}, false, err
	}
	isUnchanged, err = r.isUnchanged(found)
	if err != nil {
		return Report{}, false, x.databaseError(err)
	}
	if !isUnchanged {
		return Report{}, false, nil
	}
	if report.Indexed, err = r.countIndexed(); err != nil {
		return Report{}, false, x.databaseError(err)
	}
	return x.recordReport(report, r), true, nil
}

// writeRefresh brings the index up to date with the store in one IMMEDIATE transaction, taking the
// write lock: it rebuilds every table when isForced or when the schema or configuration changed,
// then walks the store under the lock and re-indexes only what changed since the files rows
// recorded it. The report it gives is recorded as the one Report returns.
func (x *Index) writeRefresh(
	ctx context.Context,
	start time.Time,
	isForced bool,
) (report Report, err error) {
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
	r := x.newRefresher(ctx, tx, start)
	found, err := walkStore(x.store, r)
	if err != nil {
		return Report{}, err
	}
	if err := r.sync(found, &report); err != nil {
		return Report{}, x.databaseError(err)
	}
	if report.walkClaimsChanged, err = r.replaceWalkClaims(); err != nil {
		return Report{}, x.databaseError(err)
	}
	if err := r.persistMembership(); err != nil {
		return Report{}, x.databaseError(err)
	}
	if report.Indexed, err = r.countIndexed(); err != nil {
		return Report{}, x.databaseError(err)
	}
	if err := commitTx(tx); err != nil {
		return Report{}, fmt.Errorf("%s: commit refresh: %w", Path(x.store), err)
	}
	return x.recordReport(report, r), nil
}

// recordReport gives report r's warnings, in path, line and message order, and records it as the
// one Report returns.
func (x *Index) recordReport(report Report, r *refresher) Report {
	report.Warnings = r.warnings
	slices.SortFunc(report.Warnings, func(a, b diagnose.Finding) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Message, b.Message))
	})
	x.report = report
	return report
}

// walkStore lists the node files of the issue tree and then of the Knowledge tree, each in path
// order and marked with its kind, and keeps on r the claims of the files each walk leaves out,
// marked with the same kind.
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
		files, claims, err := walkIssues(s.WorktreeRoot, current.dir, current.content, r)
		if err != nil {
			return nil, err
		}
		for i := range files {
			files[i].kind = current.kind
		}
		for i := range claims {
			claims[i].kind = current.kind
		}
		found = append(found, files...)
		r.walkClaims = append(r.walkClaims, claims...)
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

// refresher carries one refresh transaction, read or write, and the warnings it collects.
type refresher struct {
	ctx    context.Context
	tx     *sql.Tx
	config config.Config
	// issues and notes are the Kind readers of the issue tree and the Knowledge tree, the one
	// decode of every node file the refresh reads.
	issues nodestore.Reader[issue.Issue]
	notes  nodestore.Reader[knowledge.Note]
	// worktreeRoot is the working tree root the files rows' paths are relative to.
	worktreeRoot string
	// refreshed is the refresh start, truncated to racyGranularity, in Unix nanoseconds.
	refreshed int64
	warnings  []diagnose.Finding
	// walkClaims are the names the files the walk left out claim, which replaceWalkClaims stores.
	walkClaims []claim
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
	// ctime and identity are the change time, in Unix nanoseconds, and the identity changeStamp
	// gives the file: 0 and "" where the system offers none.
	ctime    int64
	identity string
}

// recordedFile is a files row from an earlier refresh.
type recordedFile struct {
	size      int64
	mtime     int64
	ctime     int64
	identity  string
	refreshed int64
	problems  sql.NullString
}

// isDirty reports whether the file must be read again: its size, mtime, change time or identity
// changed, or its mtime or change time is not older than the refresh that recorded it
// (racy-clean). The change time and identity catch an edit that keeps the size and restores the
// mtime, with no file body read.
func (recorded recordedFile) isDirty(file foundFile) bool {
	return recorded.size != file.size || recorded.mtime != file.mtime ||
		recorded.ctime != file.ctime || recorded.identity != file.identity ||
		recorded.mtime >= recorded.refreshed || recorded.ctime >= recorded.refreshed
}

// takeRecorded looks up the files row of file in recorded and deletes it there, so the rows left
// once every found file is taken are those of the files that are gone. isKnown reports whether the
// row exists, and isClean whether it does and the file needs no new read.
func takeRecorded(
	recorded map[string]recordedFile,
	file foundFile,
) (previous recordedFile, isKnown, isClean bool) {
	previous, isKnown = recorded[file.path]
	delete(recorded, file.path)
	return previous, isKnown, isKnown && !previous.isDirty(file)
}

// sync re-indexes the new and changed files, keeps the clean ones and forgets the vanished ones.
func (r *refresher) sync(found []foundFile, report *Report) error {
	recorded, err := r.recordedFiles()
	if err != nil {
		return err
	}
	for _, file := range found {
		previous, isKnown, isClean := takeRecorded(recorded, file)
		if isClean {
			if err := r.rewarn(file.path, previous.problems); err != nil {
				return err
			}
			continue
		}
		isGone, err := r.indexFile(file, isKnown)
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
		if err := r.forget(path, true); err != nil {
			return err
		}
		report.removed = append(report.removed, path)
	}
	slices.Sort(report.removed)
	return nil
}

// isUnchanged reports whether a write refresh of found would change no row: every found file is
// recorded clean, no recorded file is gone, this walk's claims are the stored ones and the
// membership verdicts decided from the rows are the stored ones. It reads through r.tx and writes
// nothing. While all hold it gives r the warnings a write refresh would: the stored problems of
// every file and the membership warnings; once one fails it stops, and r's warnings are partial.
func (r *refresher) isUnchanged(found []foundFile) (bool, error) {
	recorded, err := r.recordedFiles()
	if err != nil {
		return false, err
	}
	for _, file := range found {
		previous, _, isClean := takeRecorded(recorded, file)
		if !isClean {
			return false, nil
		}
		if err := r.rewarn(file.path, previous.problems); err != nil {
			return false, err
		}
	}
	if len(recorded) > 0 {
		return false, nil
	}
	stored, err := r.storedWalkClaims()
	if err != nil {
		return false, err
	}
	if !slices.Equal(stored, r.walkClaimKeys()) {
		return false, nil
	}
	return r.isMembershipStored()
}

// countIndexed counts the issues every query sees: the issues rows not excluded.
func (r *refresher) countIndexed() (int, error) {
	var indexed int
	err := r.tx.QueryRowContext(r.ctx, `SELECT count(*) FROM issues WHERE excluded = 0`).Scan(&indexed)
	if err != nil {
		return 0, fmt.Errorf("count indexed issues: %w", err)
	}
	return indexed, nil
}

// recordedFiles reads every files row, by path.
func (r *refresher) recordedFiles() (map[string]recordedFile, error) {
	rows, err := r.tx.QueryContext(r.ctx, `SELECT path, size, mtime, ctime, identity, refreshed, problems FROM files`)
	if err != nil {
		return nil, fmt.Errorf("read indexed files: %w", err)
	}
	defer rows.Close()
	recorded := map[string]recordedFile{}
	for rows.Next() {
		var path string
		var file recordedFile
		if err := rows.Scan(
			&path, &file.size, &file.mtime, &file.ctime, &file.identity, &file.refreshed, &file.problems,
		); err != nil {
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
// isKnown reports whether the index holds a files row for the file, as forget takes it.
func (r *refresher) indexFile(file foundFile, isKnown bool) (isGone bool, err error) {
	if err := r.forget(file.path, isKnown); err != nil {
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
		`INSERT INTO files (path, kind, size, mtime, ctime, identity, refreshed, problems)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		file.path, file.kind, file.size, file.mtime, file.ctime, file.identity, r.refreshed, stored,
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
// are current. isChanged reports whether this walk's claims differ from the ones it replaced.
func (r *refresher) replaceWalkClaims() (isChanged bool, err error) {
	stored, err := r.storedWalkClaims()
	if err != nil {
		return false, err
	}
	_, err = r.tx.ExecContext(r.ctx, `DELETE FROM claims WHERE path NOT IN (SELECT path FROM files)`)
	if err != nil {
		return false, fmt.Errorf("drop the claims of left-out files from index: %w", err)
	}
	for _, walkClaim := range r.walkClaims {
		_, err := r.tx.ExecContext(
			r.ctx, insertClaimQuery,
			walkClaim.path, walkClaim.kind, walkClaim.name, parseStateNames[walkClaim.parse],
		)
		if err != nil {
			return false, fmt.Errorf("index %s: %w", walkClaim.path, err)
		}
	}
	return !slices.Equal(stored, r.walkClaimKeys()), nil
}

// walkClaimKeys spells this walk's claims as claimKey spells a claims row, sorted, as
// storedWalkClaims reads the stored ones.
func (r *refresher) walkClaimKeys() []string {
	keys := make([]string, 0, len(r.walkClaims))
	for _, walkClaim := range r.walkClaims {
		parse := parseStateNames[walkClaim.parse]
		keys = append(keys, claimKey(walkClaim.path, walkClaim.kind, walkClaim.name, parse))
	}
	slices.Sort(keys)
	return keys
}

// storedWalkClaims reads the claims an earlier walk recorded for the files it left out, the
// claims rows whose path has no files row, each as claimKey spells it, sorted.
func (r *refresher) storedWalkClaims() ([]string, error) {
	rows, err := r.tx.QueryContext(
		r.ctx, `SELECT path, kind, name, parse FROM claims WHERE path NOT IN (SELECT path FROM files)`,
	)
	if err != nil {
		return nil, fmt.Errorf("read the claims of left-out files: %w", err)
	}
	defer rows.Close()
	var stored []string
	for rows.Next() {
		var path, kind, name, parse string
		if err := rows.Scan(&path, &kind, &name, &parse); err != nil {
			return nil, fmt.Errorf("read the claims of left-out files: %w", err)
		}
		stored = append(stored, claimKey(path, kind, name, parse))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read the claims of left-out files: %w", err)
	}
	slices.Sort(stored)
	return stored, nil
}

// claimKey spells one claims row as a single comparable string. Its fields are joined by a NUL,
// which no path, kind, name or parse state name holds.
func claimKey(path, kind, name, parse string) string {
	return strings.Join([]string{path, kind, name, parse}, "\x00")
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
			closed, reason, title, standing) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		[]any{
			file.path, file.id, file.parentID, parsed.Status, string(category), parsed.Type,
			parsed.Priority, parsed.Plan, formatTime(parsed.Created), formatTime(parsed.Closed),
			parsed.Reason, parsed.Title, parsed.IsStanding,
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

// forget drops every row of one file. isKnown reports whether the index holds a files row for it.
// fts rows exist only beside a files row (insertIssue, insertNote), and fts cannot find a path
// through an index, so each delete from it scans the whole table: a file without a files row skips
// it, which keeps a cold build linear in the store. The other tables are cleared either way, by
// their path index: claims can still hold the claims an earlier walk recorded for a file it left
// out (replaceWalkClaims spares a path once it has a files row).
func (r *refresher) forget(path string, isKnown bool) error {
	for _, table := range rowTables {
		if table == "fts" && !isKnown {
			continue
		}
		if _, err := r.tx.ExecContext(r.ctx, `DELETE FROM `+table+` WHERE path = ?`, path); err != nil {
			return fmt.Errorf("drop %s from index: %w", path, err)
		}
	}
	return nil
}
