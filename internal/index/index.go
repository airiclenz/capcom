// Package index keeps the disposable SQLite index of a store at `<git-dir>/capcom/index.db`. The
// index mirrors the issue tree and the Knowledge tree and is never committed; the markdown files
// stay the truth. Open returns the index already up to date: ModeRefresh brings it up to date
// incrementally, re-parsing only the files whose size or mtime changed, and ModeRebuild (or
// deleting the file) recreates it from scratch. A file that cannot be used is reported as a
// diagnose.Finding and left out while the rest stays queryable. The index only reads `.capcom/`;
// it never writes there.
package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver

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

// dsnOptions wait up to five seconds for a concurrent command's write, keep readers off the
// writer's back with WAL, and begin every transaction IMMEDIATE so two refreshes never deadlock
// upgrading a read lock.
const dsnOptions = "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_txlock=immediate"

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
// database and returns the failure.
func Open(s *store.Store, mode Mode, options ...Option) (*Index, error) {
	path := Path(s)
	if err := os.MkdirAll(filepath.Dir(path), dirPermissions); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Dir(path), err)
	}
	db, err := sql.Open(driverName, path+dsnOptions)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	// One connection keeps the IMMEDIATE transaction and the queries of a command on the same
	// SQLite handle.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		return nil, errors.Join(fmt.Errorf("%s: %w", path, err), db.Close())
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
	ctx := context.Background()
	var created, closed string
	err = q.QueryRowContext(ctx, `SELECT id, path, parent, status, category, type, priority,
		plan, created, closed, reason, title, standing FROM issues WHERE id = ? AND excluded = 0`, id).Scan(
		&entry.ID, &entry.Path, &entry.ParentID, &entry.Status, &entry.Category, &entry.Type,
		&entry.Priority, &entry.Plan, &created, &closed, &entry.Reason, &entry.Title,
		&entry.IsStanding,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Entry{}, false, nil
	}
	if err != nil {
		return Entry{}, false, fmt.Errorf("%s: read issue %q: %w", Path(x.store), id, err)
	}
	if entry.Created, err = parseTime(created); err != nil {
		return Entry{}, false, err
	}
	if entry.Closed, err = parseTime(closed); err != nil {
		return Entry{}, false, err
	}

	lists := []struct {
		query  string
		target *[]string
	}{
		{`SELECT label FROM labels WHERE path = ? ORDER BY rowid`, &entry.Labels},
		{`SELECT to_id FROM deps WHERE path = ? ORDER BY rowid`, &entry.Deps},
		{`SELECT to_id FROM related WHERE path = ? ORDER BY rowid`, &entry.Related},
		{`SELECT alias FROM aliases WHERE path = ? ORDER BY rowid`, &entry.Aliases},
	}
	for _, list := range lists {
		if *list.target, err = x.strings(ctx, q, list.query, entry.Path); err != nil {
			return Entry{}, false, err
		}
	}
	return entry, true, nil
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
