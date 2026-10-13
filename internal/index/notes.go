package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/tree"
)

// NoteEntry is one indexed Note of the Knowledge tree.
type NoteEntry struct {
	ID       string
	Path     string
	ParentID string
	Title    string
	Labels   []string
	// Refs are the Note's Refs, in file order.
	Refs    []string
	Aliases []string
	// IsRetired is the Note's own retired flag; a Note under a retired ancestor keeps false.
	IsRetired bool
	// SupersededBy are the ids of the Notes that replace this one, in file order.
	SupersededBy []string
}

// retiredSubtrees is the WITH clause naming, as hidden(id), every usable Note that is retired or
// has a retired ancestor. It is computed at query time rather than stored per row, since an
// ancestor's file can change alone.
const retiredSubtrees = `WITH RECURSIVE hidden(id) AS (
		SELECT id FROM notes WHERE retired = 1 AND excluded = 0
		UNION
		SELECT notes.id FROM notes JOIN hidden ON notes.parent = hidden.id WHERE notes.excluded = 0
	) `

// retiredFilter returns the WITH clause and the condition on notes.id that leave out every Note
// of a retired subtree, or two empty strings when isRetiredIncluded is set.
func retiredFilter(isRetiredIncluded bool) (with, condition string) {
	if isRetiredIncluded {
		return "", ""
	}
	return retiredSubtrees, ` AND notes.id NOT IN (SELECT id FROM hidden)`
}

// parseNote parses and validates one Note file and returns every problem that keeps it out of the
// index.
func parseNote(file foundFile, data []byte) (knowledge.Note, []Warning) {
	parsed, err := knowledge.Parse(file.path, data)
	if err != nil {
		var failure *knowledge.Error
		if errors.As(err, &failure) {
			return knowledge.Note{}, []Warning{{Kind: WarningFrontmatter, Path: failure.Path, Line: failure.Line, Message: failure.Message}}
		}
		return knowledge.Note{}, []Warning{{Kind: WarningFrontmatter, Path: file.path, Message: err.Error()}}
	}

	var problems []Warning
	for _, failure := range knowledge.Validate(file.path, parsed) {
		problems = append(problems, Warning{Kind: WarningField, Path: failure.Path, Line: failure.Line, Message: failure.Message})
	}
	return parsed, append(problems, aliasProblems(file, parsed.Aliases)...)
}

// insertNote writes the rows of one valid Note.
func (r *refresher) insertNote(file foundFile, note knowledge.Note) error {
	statements := []statement{{
		`INSERT INTO notes (path, id, parent, title, retired) VALUES (?, ?, ?, ?, ?)`,
		[]any{file.path, file.id, file.parentID, note.Title, note.IsRetired},
	}, {
		`INSERT INTO fts (path, kind, title, body) VALUES (?, ?, ?, ?)`,
		[]any{file.path, kindNote, note.Title, note.Body},
	}}
	statements = append(statements, sharedListStatements(file, note.Labels, note.Aliases)...)
	for _, ref := range note.Refs {
		statements = append(statements, statement{
			`INSERT INTO refs (path, ref) VALUES (?, ?)`, []any{file.path, ref},
		})
	}
	for _, successor := range note.SupersededBy {
		statements = append(statements, statement{
			`INSERT INTO supersedes (path, from_id, to_id) VALUES (?, ?, ?)`,
			[]any{file.path, file.id, successor},
		})
	}
	return r.exec(file, statements)
}

// Note returns the indexed Note whose id is exactly id, retired or not. found is false when no
// usable Note has that id, including when a duplicate keeps it out. Every row comes from one read
// snapshot.
func (x *Index) Note(id string) (entry NoteEntry, found bool, err error) {
	err = x.snapshot(func(q querier) error {
		entry, found, err = x.readNote(q, id)
		return err
	})
	return entry, found, err
}

// readNote reads the Note Note returns through q.
func (x *Index) readNote(q querier, id string) (entry NoteEntry, found bool, err error) {
	ctx := context.Background()
	err = q.QueryRowContext(ctx, `SELECT id, path, parent, title, retired FROM notes
		WHERE id = ? AND excluded = 0`, id).Scan(
		&entry.ID,
		&entry.Path,
		&entry.ParentID,
		&entry.Title,
		&entry.IsRetired,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return NoteEntry{}, false, nil
	}
	if err != nil {
		return NoteEntry{}, false, fmt.Errorf("%s: read note %q: %w", Path(x.store), id, err)
	}

	lists := []struct {
		query  string
		args   []any
		target *[]string
	}{
		{`SELECT label FROM labels WHERE path = ? AND kind = ? ORDER BY rowid`, []any{entry.Path, kindNote}, &entry.Labels},
		{`SELECT ref FROM refs WHERE path = ? ORDER BY rowid`, []any{entry.Path}, &entry.Refs},
		{`SELECT alias FROM aliases WHERE path = ? AND kind = ? ORDER BY rowid`, []any{entry.Path, kindNote}, &entry.Aliases},
		{`SELECT to_id FROM supersedes WHERE path = ? ORDER BY rowid`, []any{entry.Path}, &entry.SupersededBy},
	}
	for _, list := range lists {
		if *list.target, err = x.strings(ctx, q, list.query, list.args...); err != nil {
			return NoteEntry{}, false, err
		}
	}
	return entry, true, nil
}

// ResolveNote finds the usable Note an argument names, the way Resolve finds an issue: an exact
// id, then an Alias, then the one id the argument is a prefix of. A retired Note, or one under a
// retired ancestor, resolves like any other. It fails with an error matching
// tree.ErrNotFound when nothing matches, and with a *tree.AmbiguousError when the prefix matches
// several ids. A name only a Note left out as a duplicate answers to, or the id or an alias of a
// Note file with problems or left out by the walk, fails as Resolve says, never as a prefix. Every
// row comes from one read snapshot.
func (x *Index) ResolveNote(arg string) (entry NoteEntry, err error) {
	err = x.snapshot(func(q querier) error {
		id, err := x.resolveID(q, noteNodes, arg)
		if err != nil {
			return err
		}
		entries, err := x.noteList(q, []string{id})
		if err != nil {
			return err
		}
		entry = entries[0]
		return nil
	})
	return entry, err
}

// Notes returns every usable Note in tree order: by path, so every Note comes before its
// children. Unless isRetiredIncluded is set, it leaves out every Note that is retired or has a
// retired ancestor. Every row comes from one read snapshot.
func (x *Index) Notes(isRetiredIncluded bool) (entries []NoteEntry, err error) {
	with, condition := retiredFilter(isRetiredIncluded)
	err = x.snapshot(func(q querier) error {
		ids, err := x.strings(
			context.Background(),
			q,
			with+`SELECT id FROM notes WHERE excluded = 0`+condition+` ORDER BY path`,
		)
		if err != nil {
			return err
		}
		entries, err = x.noteList(q, ids)
		return err
	})
	return entries, err
}

// SearchNotes returns the usable Notes whose title or body contains every word of text and that
// carry every one of labels, best match first. Words match as Search's do. Unless
// isRetiredIncluded is set, it leaves out every Note that is retired or has a retired ancestor.
// Every row comes from one read snapshot.
func (x *Index) SearchNotes(text string, labels []string, isRetiredIncluded bool) ([]NoteEntry, error) {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil, nil
	}
	with, condition := retiredFilter(isRetiredIncluded)
	query := with + `SELECT notes.id FROM fts JOIN notes ON notes.path = fts.path
		WHERE fts MATCH ? AND fts.kind = ? AND notes.excluded = 0` + condition
	args := []any{matchExpression(words), kindNote}
	for _, label := range labels {
		query += ` AND EXISTS (SELECT 1 FROM labels
			WHERE labels.path = notes.path AND labels.kind = ? AND label = ?)`
		args = append(args, kindNote, label)
	}
	var entries []NoteEntry
	err := x.snapshot(func(q querier) error {
		ids, err := x.strings(context.Background(), q, query+` ORDER BY fts.rank, notes.id`, args...)
		if err != nil {
			return err
		}
		entries, err = x.noteList(q, ids)
		return err
	})
	return entries, err
}

// noteList reads the indexed Note of each id through q, in order.
func (x *Index) noteList(q querier, ids []string) ([]NoteEntry, error) {
	entries := make([]NoteEntry, 0, len(ids))
	for _, id := range ids {
		beforeEntryRead(id)
		entry, found, err := x.readNote(q, id)
		if err != nil {
			return nil, err
		}
		if !found {
			// Gone only if the index changed underneath.
			return nil, fmt.Errorf("note id %q %w", id, tree.ErrNotFound)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
