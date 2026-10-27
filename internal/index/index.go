// Package index keeps the disposable SQLite index of a store at `<git-dir>/capcom/index.db`. The
// index mirrors the issue tree and the Knowledge tree and is never committed; the markdown files
// stay the truth. Open returns the index already up to date: ModeRefresh brings it up to date
// incrementally, re-parsing only the files whose size, mtime, change time or identity changed,
// and ModeRebuild (or deleting the file) recreates it from scratch. A file that cannot be used is
// reported as a diagnose.Finding and left out while the rest stays queryable. The index only
// reads `.capcom/`; it never writes there.
package index

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"modernc.org/sqlite" // also registers the "sqlite" database/sql driver
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/store"
)

// Location of the index below the git directory, and the permissions it is created with.
const (
	dirName        = "capcom"
	fileName       = "index.db"
	dirPermissions = 0o755
)

// driverName is the database/sql driver modernc.org/sqlite registers.
const driverName = "sqlite"

// sidecarSuffixes name the files SQLite keeps beside index.db in WAL mode; a corrupt index is
// discarded together with them.
var sidecarSuffixes = []string{"-wal", "-shm"}

// ErrCorrupt marks a failure to open an index SQLite reports as not a database or corrupt. Open in
// ModeRebuild discards such an index and builds it again; any other mode returns the failure, and
// capcom reindex is the way past it.
var ErrCorrupt = errors.New("the index is corrupt")

// corruptError is a failure to open a corrupt index: it reads as the SQLite failure it wraps and
// matches ErrCorrupt.
type corruptError struct {
	err error
}

func (e corruptError) Error() string { return e.err.Error() }

func (e corruptError) Unwrap() []error { return []error{e.err, ErrCorrupt} }

// ErrBusy marks a failure to open or refresh an index another connection kept locked past the
// busy timeout, as capcom reindex rebuilding it does. It is never ErrCorrupt: the index is left
// as it is, and the command can simply be retried.
var ErrBusy = errors.New("the index is busy")

// busyAdvice follows ErrBusy in a busyError's message: what holds the index and what to do.
const busyAdvice = "another capcom command, such as capcom reindex, is rebuilding or writing it; " +
	"retry when it finishes"

// busyError is a failure to open or refresh a busy index: it reads as the index path, ErrBusy and
// busyAdvice in place of the bare SQLite failure, and matches both that failure and ErrBusy.
type busyError struct {
	path string
	err  error
}

func (e busyError) Error() string { return fmt.Sprintf("%s: %v: %s", e.path, ErrBusy, busyAdvice) }

func (e busyError) Unwrap() []error { return []error{e.err, ErrBusy} }

// asBusyError returns err as a busyError naming the index at path when it carries a SQLite
// failure whose primary result code is BUSY or LOCKED and is not one already, and err unchanged
// otherwise.
func asBusyError(path string, err error) error {
	if errors.Is(err, ErrBusy) || !hasPrimaryCode(err, sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED) {
		return err
	}
	return busyError{path: path, err: err}
}

// defaultBusyTimeout is how long a connection waits for a concurrent command's lock on the index
// before its statement fails busy.
const defaultBusyTimeout = 5 * time.Second

// dsnOptions wait defaultBusyTimeout for a concurrent command's write, keep readers off the
// writer's back with WAL, and begin every transaction IMMEDIATE so two refreshes never deadlock
// upgrading a read lock. It is a variable only so a test can shorten the busy timeout.
var dsnOptions = connectionOptions(defaultBusyTimeout)

// connectionOptions builds the DSN query string of dsnOptions with busyTimeout as SQLite's busy
// timeout. The busy timeout comes first, so it already applies to the journal_mode pragma.
func connectionOptions(busyTimeout time.Duration) string {
	return fmt.Sprintf(
		"?_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)&_txlock=immediate",
		busyTimeout.Milliseconds(),
	)
}

// Index is an open index of one store. It is not safe for concurrent use: open one per command,
// or serialise every call, Refresh included, on the one Index a long-lived caller keeps.
type Index struct {
	db     *sql.DB
	store  *store.Store
	now    func() time.Time
	report Report
}

// Mode is how Open brings the index up to date before returning it.
type Mode int

// The modes of Open.
const (
	// ModeRefresh re-reads only the files that changed since the index recorded them.
	ModeRefresh Mode = iota
	// ModeRebuild recreates every table and reads both trees again.
	ModeRebuild
)

// Option configures an Index at Open.
type Option func(*Index)

// withClock replaces the clock a refresh reads its start time from; tests use it to make the
// racy-clean rule deterministic.
func withClock(now func() time.Time) Option {
	return func(x *Index) {
		x.now = now
	}
}

// Report is what one refresh or rebuild did.
type Report struct {
	// rebuilt is true when every table was recreated: on a rebuild, on a new database, and when
	// the schema version or the configured statuses and types changed since the index was
	// recorded.
	rebuilt bool
	// parsed lists the files read and parsed again, in path order.
	parsed []string
	// removed lists the files whose rows were dropped because the file is gone, in path order.
	removed []string
	// walkClaimsChanged is true when the names the files the walk leaves out claim differ from
	// the ones the previous refresh stored.
	walkClaimsChanged bool
	// Indexed is the number of issues every query sees once the refresh is done.
	Indexed int
	// Warnings are every file left out and every problem found, in path then line order. A
	// problem is reported on every refresh until its file changes.
	Warnings []diagnose.Finding
}

// Entry is one indexed issue. Times are zero when the file leaves them out. Related holds the
// related links as the file writes them; the issues linking to this one come from RelatedBy.
type Entry struct {
	ID       string
	Path     string
	ParentID string
	Status   string
	Category config.Category
	Type     string
	Priority int
	Plan     string
	Created  time.Time
	Closed   time.Time
	Reason   string
	Title    string
	// IsStanding is the issue's own standing mark: a standing Parent that only groups its
	// children.
	IsStanding bool
	Labels     []string
	Deps       []string
	Related    []string
	Aliases    []string
}

// isChanged reports whether the refresh behind the report saw any issue or Note added, changed
// or removed: it rebuilt every table, parsed or dropped a file, or stored other walk claims.
func (report Report) isChanged() bool {
	return report.rebuilt || len(report.parsed) > 0 || len(report.removed) > 0 || report.walkClaimsChanged
}

// Dir returns the directory the index of s lives in, `<git-dir>/capcom`: per worktree and never
// committed, it also holds capcom ui's worktree state file, ui-state.yaml.
func Dir(s *store.Store) string {
	return filepath.Join(s.GitDir, dirName)
}

// Path returns where the index of s lives: `<git-dir>/capcom/index.db`, in Dir.
func Path(s *store.Store) string {
	return filepath.Join(Dir(s), fileName)
}

// Open opens the index of s, creating its directory and database file when missing, and refreshes
// or rebuilds it as mode asks, so the index it returns is up to date with the trees on disk and
// Report holds what that first pass did. When the refresh or rebuild fails, Open closes the
// database and returns the failure. An index SQLite reports as not a database or corrupt fails
// with ErrCorrupt, except in ModeRebuild, which deletes it with its WAL files and builds it again.
// An index another command keeps locked past the busy timeout fails with ErrBusy; an index failing
// for any other reason, busy or unreadable, is never deleted.
func Open(s *store.Store, mode Mode, options ...Option) (*Index, error) {
	path := Path(s)
	if err := os.MkdirAll(filepath.Dir(path), dirPermissions); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Dir(path), err)
	}
	index, err := openFile(s, path, mode, options)
	if !isCorruptIndex(err) {
		return index, err
	}
	if mode != ModeRebuild {
		return nil, corruptError{err: err}
	}
	if err := removeIndexFiles(path); err != nil {
		return nil, err
	}
	index, err = openFile(s, path, mode, options)
	if isCorruptIndex(err) {
		return nil, corruptError{err: err}
	}
	return index, err
}

// openFile opens the database at path and runs Open's first refresh or rebuild on it, closing the
// database again when either fails.
func openFile(s *store.Store, path string, mode Mode, options []Option) (*Index, error) {
	db, err := sql.Open(driverName, path+dsnOptions)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	// One connection keeps the IMMEDIATE transaction and the queries of a command on the same
	// SQLite handle.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		return nil, errors.Join(asBusyError(path, fmt.Errorf("%s: %w", path, err)), db.Close())
	}

	index := &Index{db: db, store: s, now: time.Now}
	for _, option := range options {
		option(index)
	}
	if _, err := index.refresh(mode == ModeRebuild); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return index, nil
}

// isCorruptIndex reports whether err carries a SQLite failure whose primary result code says the
// file is not a database or is corrupt; a busy, locked or unreadable index is not corrupt.
func isCorruptIndex(err error) bool {
	return hasPrimaryCode(err, sqlite3.SQLITE_NOTADB, sqlite3.SQLITE_CORRUPT)
}

// primaryCodeMask selects the primary result code from a SQLite extended result code.
const primaryCodeMask = 0xff

// hasPrimaryCode reports whether err carries a SQLite failure whose primary result code is one of
// codes.
func hasPrimaryCode(err error, codes ...int) bool {
	var failure *sqlite.Error
	if !errors.As(err, &failure) {
		return false
	}
	return slices.Contains(codes, failure.Code()&primaryCodeMask)
}

// removeIndexFiles deletes the index at path and its WAL files; a file already missing is not a
// failure.
func removeIndexFiles(path string) error {
	paths := []string{path}
	for _, suffix := range sidecarSuffixes {
		paths = append(paths, path+suffix)
	}
	for _, file := range paths {
		err := os.Remove(file)
		if err == nil || errors.Is(err, fs.ErrNotExist) {
			continue
		}
		var pathFailure *fs.PathError
		if errors.As(err, &pathFailure) {
			err = pathFailure.Err
		}
		return fmt.Errorf("%s: discard the corrupt index: %w", file, err)
	}
	return nil
}

// Report returns what the latest refresh or rebuild did: Open's own, until refresh runs again.
// Its slices are copies, so a caller may change them without touching the index.
func (x *Index) Report() Report {
	report := x.report
	report.parsed = slices.Clone(report.parsed)
	report.removed = slices.Clone(report.removed)
	report.Warnings = slices.Clone(report.Warnings)
	return report
}

// Close releases the database.
func (x *Index) Close() error {
	return x.db.Close()
}

// Issue returns the indexed issue whose id is exactly id. found is false when no usable issue has
// that id, including when a duplicate keeps it out. Every row comes from one read snapshot.
func (x *Index) Issue(id string) (entry Entry, found bool, err error) {
	err = x.snapshot(func(q querier) error {
		entry, found, err = x.readIssue(q, id)
		return err
	})
	return entry, found, err
}

// readIssue reads the issue Issue returns through q.
func (x *Index) readIssue(q querier, id string) (entry Entry, found bool, err error) {
	byID, err := x.loadIssues(q, []string{id})
	if err != nil {
		return Entry{}, false, err
	}
	entry, found = byID[id]
	return entry, found, nil
}

// issueLists are the list fields of an issue, each with the table and column it is stored in,
// one row per value in the order the file gives them.
var issueLists = []struct {
	table, column string
	target        func(entry *Entry) *[]string
}{
	{"labels", "label", func(entry *Entry) *[]string { return &entry.Labels }},
	{"deps", "to_id", func(entry *Entry) *[]string { return &entry.Deps }},
	{"related", "to_id", func(entry *Entry) *[]string { return &entry.Related }},
	{"aliases", "alias", func(entry *Entry) *[]string { return &entry.Aliases }},
}

// loadIssues reads the usable issue of each of ids through q, keyed by id, in one row query plus
// one query per list field, however many ids there are. The ids go in as one JSON array, so no
// bound-variable limit caps them. An id no usable issue has is absent from the map.
func (x *Index) loadIssues(q querier, ids []string) (map[string]Entry, error) {
	ctx := context.Background()
	idsJSON, err := json.Marshal(ids)
	if err != nil {
		return nil, fmt.Errorf("%s: read issues: %w", Path(x.store), err)
	}
	rows, err := q.QueryContext(ctx, `SELECT id, path, parent, status, category, type, priority,
		plan, created, closed, reason, title, standing FROM issues
		WHERE excluded = 0 AND id IN (SELECT value FROM json_each(?))`, string(idsJSON))
	if err != nil {
		return nil, fmt.Errorf("%s: read issues: %w", Path(x.store), err)
	}
	byPath, err := x.scanIssues(rows)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	pathsJSON, err := json.Marshal(paths)
	if err != nil {
		return nil, fmt.Errorf("%s: read issues: %w", Path(x.store), err)
	}
	for _, list := range issueLists {
		values, err := x.pathValues(ctx, q, `SELECT path, `+list.column+` FROM `+list.table+`
			WHERE path IN (SELECT value FROM json_each(?)) ORDER BY rowid`, string(pathsJSON))
		if err != nil {
			return nil, err
		}
		for path, entry := range byPath {
			*list.target(entry) = values[path]
		}
	}

	byID := make(map[string]Entry, len(byPath))
	for _, entry := range byPath {
		byID[entry.ID] = *entry
	}
	return byID, nil
}

// scanIssues reads the issue rows loadIssues selects, keyed by path, and closes rows.
func (x *Index) scanIssues(rows *sql.Rows) (map[string]*Entry, error) {
	defer rows.Close()
	byPath := make(map[string]*Entry)
	for rows.Next() {
		entry := &Entry{}
		var created, closed string
		if err := rows.Scan(
			&entry.ID, &entry.Path, &entry.ParentID, &entry.Status, &entry.Category, &entry.Type,
			&entry.Priority, &entry.Plan, &created, &closed, &entry.Reason, &entry.Title,
			&entry.IsStanding,
		); err != nil {
			return nil, fmt.Errorf("%s: read issues: %w", Path(x.store), err)
		}
		var err error
		if entry.Created, err = parseTime(created); err != nil {
			return nil, err
		}
		if entry.Closed, err = parseTime(closed); err != nil {
			return nil, err
		}
		byPath[entry.Path] = entry
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: read issues: %w", Path(x.store), err)
	}
	return byPath, nil
}

// pathValues runs a query selecting a path and one text column through q, and returns the values
// of each path in the order the query gives them.
func (x *Index) pathValues(ctx context.Context, q querier, query string, args ...any) (map[string][]string, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", Path(x.store), err)
	}
	defer rows.Close()
	values := make(map[string][]string)
	for rows.Next() {
		var path, value string
		if err := rows.Scan(&path, &value); err != nil {
			return nil, fmt.Errorf("%s: query: %w", Path(x.store), err)
		}
		values[path] = append(values[path], value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: query: %w", Path(x.store), err)
	}
	return values, nil
}

// RelatedBy returns the ids of the usable issues whose related links name the usable issue id, by
// its id or by one of its aliases, each once in id order. An alias another issue file also claims,
// as an alias or its id, is refused and names no issue. The issue itself is left out, and an id no
// usable issue has yields none.
func (x *Index) RelatedBy(id string) ([]string, error) {
	return x.strings(
		context.Background(),
		x.db,
		`SELECT DISTINCT holder.id FROM related
			JOIN issues AS holder ON holder.path = related.path
			WHERE holder.excluded = 0 AND holder.id <> ?1 AND (
				related.to_id IN (SELECT id FROM issues WHERE id = ?1 AND excluded = 0)
				OR related.to_id IN (SELECT aliases.alias FROM aliases
					JOIN issues AS target ON target.path = aliases.path
					WHERE aliases.kind = ?2 AND target.id = ?1 AND target.excluded = 0
						AND NOT EXISTS (SELECT 1 FROM aliases AS other WHERE other.kind = ?2
							AND other.alias = aliases.alias AND other.path <> aliases.path)
						AND NOT EXISTS (SELECT 1 FROM issues AS owner WHERE owner.id = aliases.alias))
			)
			ORDER BY holder.id`,
		id, kindIssue,
	)
}

// Search returns the ids of the usable issues whose title or body contains every word of text,
// best match first. Words are matched as whole tokens, case-insensitively; FTS5 operators in
// text are taken literally.
func (x *Index) Search(text string) ([]string, error) {
	return x.searchIDs(x.db, text, matchExpression)
}

// searchIDs reads through q the ids of the usable issues that match the FTS5 query expression
// builds from the words of text, ordered by FTS5 rank then id. Blank text matches nothing.
func (x *Index) searchIDs(q querier, text string, expression func(words []string) string) ([]string, error) {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil, nil
	}
	return x.strings(
		context.Background(),
		q,
		`SELECT issues.id FROM fts JOIN issues ON issues.path = fts.path
			WHERE fts MATCH ? AND fts.kind = ? AND issues.excluded = 0 ORDER BY fts.rank, issues.id`,
		expression(words), kindIssue,
	)
}

// matchExpression turns words into an FTS5 query that needs every word, each quoted so FTS5
// operators in it are taken literally.
func matchExpression(words []string) string {
	return strings.Join(quotedTerms(words), " ")
}

// anyMatchExpression turns words into an FTS5 query that needs at least one of the words, each
// quoted as matchExpression quotes it.
func anyMatchExpression(words []string) string {
	return strings.Join(quotedTerms(words), " OR ")
}

// quotedTerms quotes each word as an FTS5 string, doubling its quotes, so FTS5 reads every word
// as a literal term and never as an operator.
func quotedTerms(words []string) []string {
	terms := make([]string, 0, len(words))
	for _, word := range words {
		terms = append(terms, `"`+strings.ReplaceAll(word, `"`, `""`)+`"`)
	}
	return terms
}

// Body returns the indexed body of the Issue or Note file at path, the worktree-relative path an
// Entry or a NoteEntry carries, as the full-text search reads it. A path the index holds no file
// at, as when a refresh dropped the file since it was found, has the empty body.
func (x *Index) Body(path string) (string, error) {
	var body string
	err := x.db.QueryRowContext(context.Background(), `SELECT body FROM fts WHERE path = ?`, path).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("%s: read body of %s: %w", Path(x.store), path, err)
	}
	return body, nil
}

// Bodies returns the indexed body of each Issue or Note file at paths, keyed by path, read in one
// statement whatever the number of paths. A path the index holds no file at has no key, so the
// map reads the empty body for it, as Body does.
func (x *Index) Bodies(paths []string) (map[string]string, error) {
	return x.bodies(x.db, paths)
}

// bodies reads the bodies Bodies returns through q, in one statement.
func (x *Index) bodies(q querier, paths []string) (map[string]string, error) {
	pathsJSON, err := json.Marshal(paths)
	if err != nil {
		return nil, fmt.Errorf("%s: read bodies: %w", Path(x.store), err)
	}
	values, err := x.pathValues(context.Background(), q,
		`SELECT path, body FROM fts WHERE path IN (SELECT value FROM json_each(?))`, string(pathsJSON))
	if err != nil {
		return nil, err
	}

	bodies := make(map[string]string, len(values))
	for path, pathBodies := range values {
		bodies[path] = pathBodies[0]
	}
	return bodies, nil
}

// querier is the part of *sql.DB and *sql.Conn the readers use, so one reader serves both a lone
// statement on the pool and a statement inside a snapshot.
type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// snapshot runs read inside one deferred SQLite read transaction, so every statement read issues
// through its querier sees the index as the first one did, whatever another connection commits
// meanwhile. It begins the transaction with a raw BEGIN DEFERRED rather than BeginTx, since
// dsnOptions make BeginTx take the write lock. The pool holds one connection, which read's querier
// holds until snapshot returns: read must issue every statement through it, never through x.db.
func (x *Index) snapshot(read func(q querier) error) (err error) {
	ctx := context.Background()
	conn, err := x.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", Path(x.store), err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("%s: %w", Path(x.store), closeErr))
		}
	}()

	if _, err := conn.ExecContext(ctx, "BEGIN DEFERRED"); err != nil {
		return fmt.Errorf("%s: begin read: %w", Path(x.store), err)
	}
	err = read(conn)
	if err == nil {
		_, err = conn.ExecContext(ctx, "COMMIT")
		if err == nil {
			return nil
		}
		err = fmt.Errorf("%s: end read: %w", Path(x.store), err)
	}
	// The transaction must not outlive the call on the connection the pool hands out next.
	if _, rollbackErr := conn.ExecContext(ctx, "ROLLBACK"); rollbackErr != nil {
		err = errors.Join(err, fmt.Errorf("%s: end read: %w", Path(x.store), rollbackErr))
	}
	return err
}

// strings runs a query selecting one text column through q and returns its values in order.
func (x *Index) strings(ctx context.Context, q querier, query string, args ...any) ([]string, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", Path(x.store), err)
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, fmt.Errorf("%s: query: %w", Path(x.store), err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: query: %w", Path(x.store), err)
	}
	return values, nil
}

// formatTime stores a time as RFC3339 UTC text, and a zero time as "".
func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

// parseTime reads a time formatTime stored.
func parseTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("index holds a bad time %q: %w", value, err)
	}
	return parsed, nil
}
