package seed

import (
	"cmp"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/tree"
)

// embeddedNotes holds the Note files of the base set. The all: prefix embeds a file whose name
// starts with a dot or an underscore too, so LoadFS refuses it instead of the build dropping it.
//
//go:embed all:notes
var embeddedNotes embed.FS

const (
	// embeddedDir is the directory of embeddedNotes that holds the set.
	embeddedDir = "notes"
	// noteExtension ends the name of every Note file.
	noteExtension = ".md"
	// rootDir is the directory of the root Note file, as io/fs names the top of a file system.
	rootDir = "."
	// idWordSeparator joins the root id and the rest of the id of a Note below the root.
	idWordSeparator = "-"
)

// Note is one Note of a set, as its file holds it.
type Note struct {
	// ID is the file name without the .md extension.
	ID string
	// ParentID is the id of the Note this one sits below, empty for the root Note.
	ParentID string
	Title    string
	// Body is the markdown after the frontmatter, byte for byte (LF line endings); it keeps the
	// blank line that follows the frontmatter.
	Body string
}

// Set is a loaded set of Notes: one root Note and the Notes below it. The zero Set holds no Note.
type Set struct {
	// notes holds the root Note first, then the others sorted by id.
	notes []Note
}

// Load returns the base set embedded in the binary. It fails as LoadFS does.
func Load() (Set, error) {
	notesFS, err := fs.Sub(embeddedNotes, embeddedDir)
	if err != nil {
		return Set{}, fmt.Errorf("open the embedded Notes: %w", err)
	}
	return LoadFS(notesFS)
}

// LoadFS returns the set of Notes that fsys holds in a store's Knowledge layout: the root Note
// file at the top, and the file of every other Note in the directory named after its parent.
//
// LoadFS fails, naming the file, when a file is not a .md file, when its id is one
// tree.ValidateID refuses, when it does not parse or validate as a Note, when it carries Labels,
// Refs, Aliases, retired or superseded-by, when two files share an id, when the top holds no Note
// file or more than one, when a file sits outside the root's subtree, and when the id of a Note
// below the root does not start with the root id and a hyphen.
func LoadFS(fsys fs.FS) (Set, error) {
	files, err := readNoteFiles(fsys)
	if err != nil {
		return Set{}, err
	}
	rootFile, err := singleRoot(files)
	if err != nil {
		return Set{}, err
	}

	notes := make([]Note, 0, len(files))
	for _, file := range files {
		if file.path == rootFile.path {
			continue
		}
		if err := checkPlacement(file, rootFile, files); err != nil {
			return Set{}, err
		}
		notes = append(notes, file.note)
	}
	slices.SortFunc(notes, func(a, b Note) int { return cmp.Compare(a.ID, b.ID) })
	return Set{notes: slices.Insert(notes, 0, rootFile.note)}, nil
}

// Root returns the root Note of the set, the zero Note for the zero Set.
func (s Set) Root() Note {
	if len(s.notes) == 0 {
		return Note{}
	}
	return s.notes[0]
}

// Notes returns every Note of the set: the root Note first, then the others sorted by id.
func (s Set) Notes() []Note {
	return slices.Clone(s.notes)
}

// Note returns the Note of the set with the given id, and whether the set holds one.
func (s Set) Note(id string) (Note, bool) {
	for _, note := range s.notes {
		if note.ID == id {
			return note, true
		}
	}
	return Note{}, false
}

// noteFile is one Note file of a set being loaded.
type noteFile struct {
	// path is the file's slash-separated path in the file system it was read from.
	path string
	note Note
}

// readNoteFiles reads every file of fsys as a Note file, in the order fs.WalkDir visits them. It
// fails on the first file that is not a usable Note of a set, and on two files with one id.
func readNoteFiles(fsys fs.FS) ([]noteFile, error) {
	var files []noteFile
	pathByID := make(map[string]string)
	err := fs.WalkDir(fsys, rootDir, func(filePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%s: %w", filePath, err)
		}
		if entry.IsDir() {
			return nil
		}
		file, err := readNoteFile(fsys, filePath)
		if err != nil {
			return err
		}
		if firstPath, isTaken := pathByID[file.note.ID]; isTaken {
			return fileError(filePath, "id %q is also the id of %s", file.note.ID, firstPath)
		}
		pathByID[file.note.ID] = filePath
		files = append(files, file)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// readNoteFile reads the file at filePath in fsys as one Note of a set. The Note's parent is the
// directory the file sits in, which only checkPlacement holds against the other files.
func readNoteFile(fsys fs.FS, filePath string) (noteFile, error) {
	if !strings.HasSuffix(filePath, noteExtension) {
		return noteFile{}, fileError(filePath, "not a Note file: the name must end in %s", noteExtension)
	}
	id := tree.IDOf(filePath)
	if err := tree.ValidateID(id); err != nil {
		return noteFile{}, fileError(filePath, "%v", err)
	}
	data, err := fs.ReadFile(fsys, filePath)
	if err != nil {
		return noteFile{}, fmt.Errorf("%s: %w", filePath, err)
	}
	parsed, err := knowledge.Parse(filePath, data)
	if err != nil {
		return noteFile{}, err
	}
	if problems := knowledge.Validate(filePath, parsed); len(problems) > 0 {
		joined := make([]error, 0, len(problems))
		for _, problem := range problems {
			joined = append(joined, problem)
		}
		return noteFile{}, errors.Join(joined...)
	}
	if key := foreignKey(parsed); key != "" {
		return noteFile{}, fileError(filePath, "a Note of the set carries a title and a body only, not %s", key)
	}

	note := Note{ID: id, Title: parsed.Title, Body: parsed.Body}
	if dir := path.Dir(filePath); dir != rootDir {
		note.ParentID = path.Base(dir)
	}
	return noteFile{path: filePath, note: note}, nil
}

// foreignKey names the first frontmatter key of note that a Note of a set must not carry, or ""
// when it carries none: the store's own Note takes these from the repository, never from the set.
func foreignKey(note knowledge.Note) string {
	switch {
	case len(note.Labels) > 0:
		return "labels"
	case len(note.Refs) > 0:
		return "refs"
	case note.RetiredLine() > 0:
		return "retired"
	case note.SupersededByKeyLine() > 0:
		return "superseded-by"
	case len(note.Aliases) > 0:
		return "aliases"
	}
	return ""
}

// singleRoot returns the one file of files that sits at the top of the file system. It fails
// when there is none, and names the second when there are several.
func singleRoot(files []noteFile) (noteFile, error) {
	var roots []noteFile
	for _, file := range files {
		if file.note.ParentID == "" {
			roots = append(roots, file)
		}
	}
	switch len(roots) {
	case 0:
		return noteFile{}, fileError(rootDir, "no root Note: exactly one Note file sits at the top")
	case 1:
		return roots[0], nil
	}
	return noteFile{}, fileError(
		roots[1].path, "a second root Note beside %s: exactly one Note file sits at the top", roots[0].path,
	)
}

// checkPlacement checks that file, a file other than rootFile, sits in the directory of children
// of one of files, so in the root's subtree, and that its id starts with the root id.
func checkPlacement(file, rootFile noteFile, files []noteFile) error {
	parentPath := path.Dir(file.path) + noteExtension
	hasParent := slices.ContainsFunc(files, func(other noteFile) bool { return other.path == parentPath })
	if !hasParent {
		return fileError(
			file.path, "outside the subtree of the root Note %s: there is no Note file %s", rootFile.note.ID, parentPath,
		)
	}
	prefix := rootFile.note.ID + idWordSeparator
	if !strings.HasPrefix(file.note.ID, prefix) {
		return fileError(file.path, "id %q does not start with %q, the root id and a hyphen", file.note.ID, prefix)
	}
	return nil
}

// fileError returns the error for the file at filePath that cannot be part of a set.
func fileError(filePath, format string, args ...any) error {
	return &knowledge.Error{Path: filePath, Message: fmt.Sprintf(format, args...)}
}
