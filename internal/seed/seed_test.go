package seed

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/airiclenz/capcom/internal/knowledge"
)

// The made-up set the tests load: a root Note, two Notes below it and one below the first of them.
const (
	rootPath       = "rules.md"
	firstPath      = "rules/rules-first.md"
	secondPath     = "rules/rules-second.md"
	grandchildPath = "rules/rules-first/rules-first-detail.md"
)

// noteContent returns the content of a Note file with the given title and a one-line body.
func noteContent(title string) string {
	return "---\ntitle: " + title + "\n---\n\nThe text of " + title + ".\n"
}

// madeUpFiles returns the files of the made-up set, for a test to change before loading it.
func madeUpFiles() fstest.MapFS {
	return fstest.MapFS{
		rootPath:       {Data: []byte(noteContent("Root"))},
		secondPath:     {Data: []byte(noteContent("Second"))},
		firstPath:      {Data: []byte(noteContent("First"))},
		grandchildPath: {Data: []byte(noteContent("Detail"))},
	}
}

func TestLoad_EmbeddedSet_EveryNoteHasTitleAndBody(t *testing.T) {
	t.Parallel()

	set, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	notes := set.Notes()
	if len(notes) == 0 {
		t.Fatal("Load: the embedded set holds no Note")
	}
	if notes[0] != set.Root() || notes[0].ParentID != "" {
		t.Errorf("Notes()[0] = %+v, want the root Note %+v", notes[0], set.Root())
	}
	for _, note := range notes {
		if strings.TrimSpace(note.Title) == "" {
			t.Errorf("Note %s has no title", note.ID)
		}
		if strings.TrimSpace(note.Body) == "" {
			t.Errorf("Note %s has no body", note.ID)
		}
		if note.ID == set.Root().ID {
			continue
		}
		if _, hasParent := set.Note(note.ParentID); !hasParent {
			t.Errorf("Note %s: parent %q is not a Note of the set", note.ID, note.ParentID)
		}
	}
}

func TestLoadFS_MadeUpSet_ReturnsRootFirstThenIDsSorted(t *testing.T) {
	t.Parallel()

	set, err := LoadFS(madeUpFiles())
	if err != nil {
		t.Fatalf("LoadFS: %v", err)
	}

	want := []Note{
		{ID: "rules", Title: "Root", Body: "\nThe text of Root.\n"},
		{ID: "rules-first", ParentID: "rules", Title: "First", Body: "\nThe text of First.\n"},
		{ID: "rules-first-detail", ParentID: "rules-first", Title: "Detail", Body: "\nThe text of Detail.\n"},
		{ID: "rules-second", ParentID: "rules", Title: "Second", Body: "\nThe text of Second.\n"},
	}
	if got := set.Notes(); !slices.Equal(got, want) {
		t.Errorf("Notes() = %+v, want %+v", got, want)
	}
	if got := set.Root(); got != want[0] {
		t.Errorf("Root() = %+v, want %+v", got, want[0])
	}
}

func TestLoadFS_CRLFFile_ReadsBodyWithLFEndings(t *testing.T) {
	t.Parallel()
	files := madeUpFiles()
	files[rootPath] = &fstest.MapFile{Data: []byte(strings.ReplaceAll(noteContent("Root"), "\n", "\r\n"))}

	set, err := LoadFS(files)
	if err != nil {
		t.Fatalf("LoadFS: %v", err)
	}

	if got, want := set.Root().Body, "\nThe text of Root.\n"; got != want {
		t.Errorf("Root().Body = %q, want %q", got, want)
	}
}

func TestSet_Notes_ReturnsACopy(t *testing.T) {
	t.Parallel()
	set, err := LoadFS(madeUpFiles())
	if err != nil {
		t.Fatalf("LoadFS: %v", err)
	}

	set.Notes()[0].Title = "changed"

	if got := set.Root().Title; got != "Root" {
		t.Errorf("Root().Title = %q after changing the slice Notes returned, want %q", got, "Root")
	}
}

func TestSet_Note_FindsByID(t *testing.T) {
	t.Parallel()
	set, err := LoadFS(madeUpFiles())
	if err != nil {
		t.Fatalf("LoadFS: %v", err)
	}

	note, isFound := set.Note("rules-second")
	if !isFound || note.Title != "Second" {
		t.Errorf("Note(rules-second) = %+v, %t, want the Note titled Second", note, isFound)
	}
	if note, isFound := set.Note("rules-missing"); isFound {
		t.Errorf("Note(rules-missing) = %+v, true, want no Note", note)
	}
}

func TestSet_ZeroValue_HoldsNoNote(t *testing.T) {
	t.Parallel()
	var set Set

	if got := set.Root(); got != (Note{}) {
		t.Errorf("Root() = %+v, want the zero Note", got)
	}
	if got := set.Notes(); len(got) != 0 {
		t.Errorf("Notes() = %+v, want none", got)
	}
}

func TestLoadFS_BrokenSet_FailsNamingTheFile(t *testing.T) {
	t.Parallel()
	withKey := func(key string) string {
		return "---\ntitle: First\n" + key + "\n---\n\nThe text.\n"
	}
	cases := []struct {
		name string
		// change turns the made-up set into a broken one.
		change      func(files fstest.MapFS)
		wantPath    string
		wantMessage string
	}{
		{
			name: "a file that does not parse",
			change: func(files fstest.MapFS) {
				files[firstPath] = &fstest.MapFile{Data: []byte("no frontmatter\n")}
			},
			wantPath:    firstPath,
			wantMessage: "---",
		},
		{
			name: "a file that does not validate",
			change: func(files fstest.MapFS) {
				files[firstPath] = &fstest.MapFile{Data: []byte("---\ntitle: \"\"\n---\n\nThe text.\n")}
			},
			wantPath:    firstPath,
			wantMessage: "title is required",
		},
		{
			name: "two files with one id",
			change: func(files fstest.MapFS) {
				files["rules/rules-second/rules-first.md"] = &fstest.MapFile{Data: []byte(noteContent("Again"))}
			},
			wantPath:    "rules/rules-second/rules-first.md",
			wantMessage: "also the id of " + firstPath,
		},
		{
			name: "no root",
			change: func(files fstest.MapFS) {
				delete(files, rootPath)
			},
			wantPath:    ".",
			wantMessage: "no root Note",
		},
		{
			name: "two roots",
			change: func(files fstest.MapFS) {
				files["standards.md"] = &fstest.MapFile{Data: []byte(noteContent("Other root"))}
			},
			wantPath:    "standards.md",
			wantMessage: "a second root Note beside " + rootPath,
		},
		{
			name: "a child id without the root id as prefix",
			change: func(files fstest.MapFS) {
				files["rules/third.md"] = &fstest.MapFile{Data: []byte(noteContent("Third"))}
			},
			wantPath:    "rules/third.md",
			wantMessage: `does not start with "rules-"`,
		},
		{
			name: "a file with Labels",
			change: func(files fstest.MapFS) {
				files[firstPath] = &fstest.MapFile{Data: []byte(withKey("labels: [security]"))}
			},
			wantPath:    firstPath,
			wantMessage: "not labels",
		},
		{
			name: "a file with Refs",
			change: func(files fstest.MapFS) {
				files[firstPath] = &fstest.MapFile{Data: []byte(withKey("refs: [README.md]"))}
			},
			wantPath:    firstPath,
			wantMessage: "not refs",
		},
		{
			name: "a file with Aliases",
			change: func(files fstest.MapFS) {
				files[firstPath] = &fstest.MapFile{Data: []byte(withKey("aliases: [rules-old]"))}
			},
			wantPath:    firstPath,
			wantMessage: "not aliases",
		},
		{
			name: "a retired file",
			change: func(files fstest.MapFS) {
				files[firstPath] = &fstest.MapFile{Data: []byte(withKey("retired: true"))}
			},
			wantPath:    firstPath,
			wantMessage: "not retired",
		},
		{
			name: "a file with retired set to false",
			change: func(files fstest.MapFS) {
				files[firstPath] = &fstest.MapFile{Data: []byte(withKey("retired: false"))}
			},
			wantPath:    firstPath,
			wantMessage: "not retired",
		},
		{
			name: "a file with superseded-by",
			change: func(files fstest.MapFS) {
				files[firstPath] = &fstest.MapFile{Data: []byte(withKey("superseded-by: [rules-second]"))}
			},
			wantPath:    firstPath,
			wantMessage: "not superseded-by",
		},
		{
			name: "a file outside the root's subtree",
			change: func(files fstest.MapFS) {
				files["elsewhere/rules-third.md"] = &fstest.MapFile{Data: []byte(noteContent("Third"))}
			},
			wantPath:    "elsewhere/rules-third.md",
			wantMessage: "outside the subtree of the root Note rules",
		},
		{
			name: "an id of five words",
			change: func(files fstest.MapFS) {
				files["rules/rules-one-two-three-four.md"] = &fstest.MapFile{Data: []byte(noteContent("Long"))}
			},
			wantPath:    "rules/rules-one-two-three-four.md",
			wantMessage: "5 words, at most 4",
		},
		{
			name: "a file that is not a .md file",
			change: func(files fstest.MapFS) {
				files["rules/rules-third.txt"] = &fstest.MapFile{Data: []byte(noteContent("Third"))}
			},
			wantPath:    "rules/rules-third.txt",
			wantMessage: "not a Note file",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := madeUpFiles()
			tc.change(files)

			set, err := LoadFS(files)

			var fileErr *knowledge.Error
			if !errors.As(err, &fileErr) {
				t.Fatalf("LoadFS error = %v, want a *knowledge.Error", err)
			}
			if fileErr.Path != tc.wantPath {
				t.Errorf("error names %q, want %q (%v)", fileErr.Path, tc.wantPath, err)
			}
			if !strings.Contains(err.Error(), tc.wantMessage) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantMessage)
			}
			if len(set.Notes()) != 0 {
				t.Errorf("LoadFS returned %d Notes with its error, want none", len(set.Notes()))
			}
		})
	}
}
