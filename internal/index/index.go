// Package index keeps the disposable SQLite index of a store at `<git-dir>/capcom/index.db`. The
// index mirrors the issue tree and the Knowledge tree and is never committed; the markdown files
// stay the truth. Open returns the index already up to date: Refresh brings it up to date
// incrementally, re-parsing only the files whose size or mtime changed, and Rebuild (or deleting
// the file) recreates it from scratch. A file that cannot be used is reported as a Warning and
// left out while the rest stays queryable. The index only reads `.capcom/`; it never writes there.
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

// Index is an open index of one store. It is not safe for concurrent use; open one per command.
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
	// ModeRefresh re-reads only the files that changed since the index recorded them, as Refresh.
	ModeRefresh Mode = iota
	// ModeRebuild recreates every table and reads both trees again, as Rebuild.
	ModeRebuild
)

// Option configures an Index at Open.
type Option func(*Index)

// WithClock replaces the clock Refresh reads its start time from; tests use it to make the
// racy-clean rule deterministic.
func WithClock(now func() time.Time) Option {
	return func(x *Index) {
		x.now = now
	}
}

// WarningKind classifies a Warning by the problem it reports, so a caller tells problems apart
// without reading the message.
type WarningKind string

// The kinds of Warning, one per problem the index reports.
const (
	// WarningInvalidID is a node file whose name is not a valid id.
	WarningInvalidID WarningKind = "invalid-id"
	// WarningInvalidAlias is an alias that is not a valid name.
	WarningInvalidAlias WarningKind = "invalid-alias"
	// WarningDuplicate is an id or alias another file of the same tree also has.
	WarningDuplicate WarningKind = "duplicate"
	// WarningRepeatedAlias is an alias that repeats its own node's id or is listed twice.
	WarningRepeatedAlias WarningKind = "repeated-alias"
	// WarningOrphanDirectory is a directory left out because it has no valid node file beside it.
	WarningOrphanDirectory WarningKind = "orphan-directory"
	// WarningFrontmatter is a file whose frontmatter cannot be read.
	WarningFrontmatter WarningKind = "frontmatter"
	// WarningField is a field value the file's kind does not accept.
	WarningField WarningKind = "field"
)

// Warning is a file the index left out, or a problem it found in one. Path is relative to the
// working tree root with forward slashes; Line is 1-based, or 0 when no single line is to blame.
type Warning struct {
	Kind    WarningKind `json:"kind"`
	Path    string      `json:"path"`
	Line    int         `json:"line,omitempty"`
	Message string      `json:"message"`
}

// String renders "path:line: message", or "path: message" when the line is unknown.
func (w Warning) String() string {
	if w.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", w.Path, w.Line, w.Message)
	}
	return fmt.Sprintf("%s: %s", w.Path, w.Message)
}

// Report is what one Refresh or Rebuild did.
type Report struct {
	// Rebuilt is true when every table was recreated: on Rebuild, on a new database, and when the
	// schema version or the configured statuses and types changed since the index was recorded.
	Rebuilt bool
	// Parsed lists the files read and parsed again, in path order.
	Parsed []string
	// Removed lists the files whose rows were dropped because the file is gone, in path order.
	Removed []string
	// Indexed is the number of issues every query sees once the refresh is done.
	Indexed int
	// Warnings are every file left out and every problem found, in path then line order. A
	// problem is reported on every refresh until its file changes.
	Warnings []Warning
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
	Labels   []string
	Deps     []string
	Related  []string
	Aliases  []string
}

// Path returns where the index of s lives: `<git-dir>/capcom/index.db`.
func Path(s *store.Store) string {
	return filepath.Join(s.GitDir, dirName, fileName)
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
	prepare := index.Refresh
	if mode == ModeRebuild {
		prepare = index.Rebuild
	}
	if _, err := prepare(); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return index, nil
}

// Report returns what the latest refresh or rebuild did: Open's own, until Refresh or Rebuild
// runs again. Its slices are copies, so a caller may change them without touching the index.
func (x *Index) Report() Report {
	report := x.report
	report.Parsed = slices.Clone(report.Parsed)
	report.Removed = slices.Clone(report.Removed)
	report.Warnings = slices.Clone(report.Warnings)
	return report
}

// Close releases the database.
func (x *Index) Close() error {
	return x.db.Close()
}

// Issue returns the indexed issue whose id is exactly id. found is false when no usable issue has
// that id, including when a duplicate keeps it out.
func (x *Index) Issue(id string) (entry Entry, found bool, err error) {
	ctx := context.Background()
	var created, closed string
	err = x.db.QueryRowContext(ctx, `SELECT id, path, parent, status, category, type, priority,
		plan, created, closed, reason, title FROM issues WHERE id = ? AND excluded = 0`, id).Scan(
		&entry.ID, &entry.Path, &entry.ParentID, &entry.Status, &entry.Category, &entry.Type,
		&entry.Priority, &entry.Plan, &created, &closed, &entry.Reason, &entry.Title,
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
		if *list.target, err = x.strings(ctx, list.query, entry.Path); err != nil {
			return Entry{}, false, err
		}
	}
	return entry, true, nil
}

// RelatedBy returns the ids of the usable issues whose related links name the usable issue id, by
// its id or by one of its aliases, each once in id order. The issue itself is left out, and an id
// no usable issue has yields none.
func (x *Index) RelatedBy(id string) ([]string, error) {
	return x.strings(
		context.Background(),
		`SELECT DISTINCT holder.id FROM related
			JOIN issues AS holder ON holder.path = related.path
			WHERE holder.excluded = 0 AND holder.id <> ?1 AND (
				related.to_id IN (SELECT id FROM issues WHERE id = ?1 AND excluded = 0)
				OR related.to_id IN (SELECT aliases.alias FROM aliases
					JOIN issues AS target ON target.path = aliases.path
					WHERE aliases.kind = ?2 AND target.id = ?1 AND target.excluded = 0)
			)
			ORDER BY holder.id`,
		id, kindIssue,
	)
}

// Search returns the ids of the usable issues whose title or body contains every word of text,
// best match first. Words are matched as whole tokens, case-insensitively; FTS5 operators in
// text are taken literally.
func (x *Index) Search(text string) ([]string, error) {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil, nil
	}
	return x.strings(
		context.Background(),
		`SELECT issues.id FROM fts JOIN issues ON issues.path = fts.path
			WHERE fts MATCH ? AND fts.kind = ? AND issues.excluded = 0 ORDER BY fts.rank, issues.id`,
		matchExpression(words), kindIssue,
	)
}

// matchExpression turns words into an FTS5 query that needs every word, each quoted so FTS5
// operators in it are taken literally.
func matchExpression(words []string) string {
	terms := make([]string, 0, len(words))
	for _, word := range words {
		terms = append(terms, `"`+strings.ReplaceAll(word, `"`, `""`)+`"`)
	}
	return strings.Join(terms, " ")
}

// strings runs a query selecting one text column and returns its values in order.
func (x *Index) strings(ctx context.Context, query string, args ...any) ([]string, error) {
	rows, err := x.db.QueryContext(ctx, query, args...)
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
