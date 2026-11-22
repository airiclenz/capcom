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

// insertNote writes the rows of one valid Note.
func (r *refresher) insertNote(file foundFile, note knowledge.Note) error {
	statements := []statement{{
		`INSERT INTO notes (path, id, parent, title, retired) VALUES (?, ?, ?, ?, ?)`,
		[]any{file.path, file.id, file.parentID, note.Title, note.IsRetired},
	}, {
		`INSERT INTO fts (path, kind, id, title, body) VALUES (?, ?, ?, ?, ?)`,
		[]any{file.path, kindNote, file.id, note.Title, note.Body},
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
		return NoteEntry{}, false, fmt.Errorf("%s: read Note %q: %w", Path(x.store), id, err)
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

// ResolveNoteByTitle finds the usable Note an argument names as ResolveNote does and, only when
// that fails with a *tree.NotFoundError, by its title. It tries in order: the whole title, then
// the part of the title before its first colon, both trimmed and case-insensitive, then the
// argument turned into a slug (noteSlug) resolved as an id, Alias or prefix when the slug is not
// already the name ResolveNote tried. The first step with any match decides: one match resolves,
// and several fail with a *TitleMatchesError naming them all. Titles are read from usable Notes
// only, never from a file left out as a duplicate. When no step matches, the error is
// ResolveNote's, unchanged; an ambiguous, unusable or duplicate failure never falls back. Each
// lookup reads from one read snapshot.
func (x *Index) ResolveNoteByTitle(arg string) (NoteEntry, error) {
	entry, resolveErr := x.ResolveNote(arg)
	var notFound *tree.NotFoundError
	if !errors.As(resolveErr, &notFound) {
		return entry, resolveErr
	}
	err := x.snapshot(func(q querier) error {
		ids, err := x.noteTitleMatches(q, arg)
		if err != nil {
			return err
		}
		switch {
		case len(ids) == 0:
			return resolveErr
		case len(ids) > 1:
			return &TitleMatchesError{Arg: arg, IDs: ids}
		}
		entries, err := x.noteList(q, ids)
		if err != nil {
			return err
		}
		entry = entries[0]
		return nil
	})
	return entry, err
}

// TitleMatchesError reports an argument that resolves to no Note by id, Alias or prefix but to
// several by title (see ResolveNoteByTitle). It matches tree.ErrNotFound under errors.Is.
type TitleMatchesError struct {
	// Arg is the argument as given.
	Arg string
	// IDs are the ids of the Notes it matches, in order.
	IDs []string
}

// Error renders the argument and every matched id.
func (e *TitleMatchesError) Error() string {
	return fmt.Sprintf("id %q %v: it matches several Notes: %s", e.Arg, tree.ErrNotFound, strings.Join(e.IDs, ", "))
}

// Is reports whether target is tree.ErrNotFound.
func (e *TitleMatchesError) Is(target error) bool {
	return target == tree.ErrNotFound
}

// noteTitleMatches returns the ids of the usable Notes the first matching step of
// ResolveNoteByTitle finds for arg, in id order; none when no step matches.
func (x *Index) noteTitleMatches(q querier, arg string) ([]string, error) {
	typed := strings.TrimSpace(arg)
	if typed != "" {
		titles, err := x.noteTitles(q)
		if err != nil {
			return nil, err
		}
		steps := []func(title string) bool{
			func(title string) bool { return strings.EqualFold(strings.TrimSpace(title), typed) },
			func(title string) bool {
				head, _, _ := strings.Cut(title, ":")
				return strings.EqualFold(strings.TrimSpace(head), typed)
			},
		}
		for _, matches := range steps {
			var ids []string
			for _, title := range titles {
				if matches(title.title) {
					ids = append(ids, title.id)
				}
			}
			if len(ids) > 0 {
				return ids, nil
			}
		}
	}
	slug := noteSlug(arg)
	if slug == "" || slug == tree.NormalizeArg(arg) {
		return nil, nil
	}
	id, err := x.resolveID(q, noteNodes, slug)
	var ambiguous *tree.AmbiguousError
	switch {
	case err == nil:
		return []string{id}, nil
	case errors.As(err, &ambiguous):
		return ambiguous.Candidates, nil
	case errors.Is(err, tree.ErrNotFound), errors.Is(err, tree.ErrDuplicate):
		return nil, nil
	}
	return nil, err
}

// noteTitle is the id and title of one usable Note.
type noteTitle struct {
	id    string
	title string
}

// noteTitles reads the id and title of every usable Note through q, in id order. A Note left out
// as a duplicate has none.
func (x *Index) noteTitles(q querier) ([]noteTitle, error) {
	rows, err := q.QueryContext(context.Background(), `SELECT id, title FROM notes WHERE excluded = 0 ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("%s: read Note titles: %w", Path(x.store), err)
	}
	defer rows.Close()
	var titles []noteTitle
	for rows.Next() {
		var title noteTitle
		if err := rows.Scan(&title.id, &title.title); err != nil {
			return nil, fmt.Errorf("%s: read Note titles: %w", Path(x.store), err)
		}
		titles = append(titles, title)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: read Note titles: %w", Path(x.store), err)
	}
	return titles, nil
}

// noteSlug turns an argument into the id it spells: lower-cased, each run of characters other
// than a-z and 0-9 turned into one hyphen, and leading and trailing hyphens dropped, so
// "rel flow!" spells rel-flow.
func noteSlug(arg string) string {
	var slug strings.Builder
	isHyphenDue := false
	for _, r := range strings.ToLower(arg) {
		if ('a' <= r && r <= 'z') || ('0' <= r && r <= '9') {
			if isHyphenDue && slug.Len() > 0 {
				slug.WriteByte('-')
			}
			isHyphenDue = false
			slug.WriteRune(r)
			continue
		}
		isHyphenDue = true
	}
	return slug.String()
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

// SearchNotes returns the usable Notes whose id, title or body contains every word of text and that
// carry every one of labels, best match first. Words match as Search's do. Unless
// isRetiredIncluded is set, it leaves out every Note that is retired or has a retired ancestor.
// Every row comes from one read snapshot.
func (x *Index) SearchNotes(text string, labels []string, isRetiredIncluded bool) ([]NoteEntry, error) {
	return x.searchNotes(text, labels, isRetiredIncluded, matchExpression)
}

// SearchNotesAnyWord returns the usable Notes whose id, title or body contains at least one word of
// text, filtered by labels and isRetiredIncluded as SearchNotes filters them, ranked by FTS5
// rank then id. Words match as Search's do. Every row comes from one read snapshot.
func (x *Index) SearchNotesAnyWord(text string, labels []string, isRetiredIncluded bool) ([]NoteEntry, error) {
	return x.searchNotes(text, labels, isRetiredIncluded, anyMatchExpression)
}

// searchNotes reads, from one snapshot, the usable Notes that match the FTS5 query expression
// builds from the words of text, carry every one of labels and, unless isRetiredIncluded is set,
// are neither retired nor under a retired ancestor; ordered by FTS5 rank then id.
func (x *Index) searchNotes(
	text string,
	labels []string,
	isRetiredIncluded bool,
	expression func(words []string) string,
) ([]NoteEntry, error) {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil, nil
	}
	with, condition := retiredFilter(isRetiredIncluded)
	query := with + `SELECT notes.id FROM fts JOIN notes ON notes.path = fts.path
		WHERE fts MATCH ? AND fts.kind = ? AND notes.excluded = 0` + condition
	args := []any{expression(words), kindNote}
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

// NoteRef is one Ref of one indexed Note.
type NoteRef struct {
	// NoteID is the id of the Note that carries Ref.
	NoteID string
	// Ref is the Ref as the Note writes it: a slash path, with an optional #symbol.
	Ref string
}

// NoteRefs returns every Ref of every usable Note that is neither retired nor under a retired
// ancestor, by the Note's path and then in file order. Every row comes from one read snapshot.
func (x *Index) NoteRefs() (refs []NoteRef, err error) {
	with, condition := retiredFilter(false)
	query := with + `SELECT notes.id, refs.ref FROM refs JOIN notes ON notes.path = refs.path
		WHERE notes.excluded = 0` + condition + ` ORDER BY notes.path, refs.rowid`
	err = x.snapshot(func(q querier) error {
		rows, err := q.QueryContext(context.Background(), query)
		if err != nil {
			return fmt.Errorf("%s: query: %w", Path(x.store), err)
		}
		defer rows.Close()
		for rows.Next() {
			var ref NoteRef
			if err := rows.Scan(&ref.NoteID, &ref.Ref); err != nil {
				return fmt.Errorf("%s: query: %w", Path(x.store), err)
			}
			refs = append(refs, ref)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("%s: query: %w", Path(x.store), err)
		}
		return nil
	})
	return refs, err
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
			return nil, fmt.Errorf("Note id %q %w", id, tree.ErrNotFound)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
