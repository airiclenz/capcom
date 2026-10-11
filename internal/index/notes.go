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
}

// parseNote parses and validates one Note file and returns every problem that keeps it out of the
// index.
func parseNote(file foundFile, data []byte) (knowledge.Note, []Warning) {
	parsed, err := knowledge.Parse(file.path, data)
	if err != nil {
		var failure *knowledge.Error
		if errors.As(err, &failure) {
			return knowledge.Note{}, []Warning{{Path: failure.Path, Line: failure.Line, Message: failure.Message}}
		}
		return knowledge.Note{}, []Warning{{Path: file.path, Message: err.Error()}}
	}

	var problems []Warning
	for _, failure := range knowledge.Validate(file.path, parsed) {
		problems = append(problems, Warning{Path: failure.Path, Line: failure.Line, Message: failure.Message})
	}
	return parsed, append(problems, aliasProblems(file, parsed.Aliases)...)
}

// insertNote writes the rows of one valid Note.
func (r *refresher) insertNote(file foundFile, note knowledge.Note) error {
	statements := []statement{{
		`INSERT INTO notes (path, id, parent, title) VALUES (?, ?, ?, ?)`,
		[]any{file.path, file.id, file.parentID, note.Title},
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
	return r.exec(file, statements)
}

// Note returns the indexed Note whose id is exactly id. found is false when no usable Note has
// that id, including when a duplicate keeps it out.
func (x *Index) Note(id string) (entry NoteEntry, found bool, err error) {
	ctx := context.Background()
	err = x.db.QueryRowContext(ctx, `SELECT id, path, parent, title FROM notes
		WHERE id = ? AND excluded = 0`, id).Scan(&entry.ID, &entry.Path, &entry.ParentID, &entry.Title)
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
	}
	for _, list := range lists {
		if *list.target, err = x.strings(ctx, list.query, list.args...); err != nil {
			return NoteEntry{}, false, err
		}
	}
	return entry, true, nil
}

// ResolveNote finds the usable Note an argument names, the way Resolve finds an issue: an exact
// id, then an Alias, then the one id the argument is a prefix of. It fails with an error matching
// tree.ErrNotFound when nothing matches, and with a *tree.AmbiguousError when the prefix matches
// several ids.
func (x *Index) ResolveNote(arg string) (NoteEntry, error) {
	id, err := x.resolveID(noteNodes, arg)
	if err != nil {
		return NoteEntry{}, err
	}
	entries, err := x.noteList([]string{id})
	if err != nil {
		return NoteEntry{}, err
	}
	return entries[0], nil
}

// Notes returns every usable Note in tree order: by path, so every Note comes before its
// children.
func (x *Index) Notes() ([]NoteEntry, error) {
	ids, err := x.strings(context.Background(), `SELECT id FROM notes WHERE excluded = 0 ORDER BY path`)
	if err != nil {
		return nil, err
	}
	return x.noteList(ids)
}

// SearchNotes returns the usable Notes whose title or body contains every word of text and that
// carry every one of labels, best match first. Words match as Search's do.
func (x *Index) SearchNotes(text string, labels []string) ([]NoteEntry, error) {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil, nil
	}
	query := `SELECT notes.id FROM fts JOIN notes ON notes.path = fts.path
		WHERE fts MATCH ? AND fts.kind = ? AND notes.excluded = 0`
	args := []any{matchExpression(words), kindNote}
	for _, label := range labels {
		query += ` AND EXISTS (SELECT 1 FROM labels
			WHERE labels.path = notes.path AND labels.kind = ? AND label = ?)`
		args = append(args, kindNote, label)
	}
	ids, err := x.strings(context.Background(), query+` ORDER BY fts.rank, notes.id`, args...)
	if err != nil {
		return nil, err
	}
	return x.noteList(ids)
}

// noteList returns the indexed Note of each id, in order.
func (x *Index) noteList(ids []string) ([]NoteEntry, error) {
	entries := make([]NoteEntry, 0, len(ids))
	for _, id := range ids {
		entry, found, err := x.Note(id)
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
