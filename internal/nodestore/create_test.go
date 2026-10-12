package nodestore

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/tree"
)

// newIssueID is the id the Create tests give a new issue.
const newIssueID = "parser-fix"

// buildIssue returns a build function for Create giving the canonical issue without its Aliases,
// as status sets it.
func buildIssue(t *testing.T, status string) func() (issue.Issue, error) {
	t.Helper()
	node, err := issue.Parse("node.md", []byte(canonicalIssue))
	if err != nil {
		t.Fatal(err)
	}
	node.Aliases = nil
	node.Status = status
	return func() (issue.Issue, error) { return node, nil }
}

// nodeFiles returns every file below dir, relative to it; none when dir does not exist.
func nodeFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return fs.SkipAll
		}
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(dir, path)
		files = append(files, relative)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestCreate_FreeIDAtTheTop_WritesTheNode(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeNodeFile(t, s.IssuesDir(), "other-node.md", canonicalIssue)

	written, problems, err := Issues(s, nil).Create(newIssueID, "", buildIssue(t, "open"))

	if err != nil || len(problems) != 0 {
		t.Fatalf("Create = %v, %v; want no failure and no problems", problems, err)
	}
	data, err := os.ReadFile(filepath.Join(s.IssuesDir(), newIssueID+".md"))
	if err != nil {
		t.Fatal(err)
	}
	want := Written{ID: newIssueID, Path: ".capcom/issues/parser-fix.md", ETag: edit.ETag(data)}
	if written != want {
		t.Errorf("Written = %+v, want %+v", written, want)
	}
	if strings.Contains(string(data), "aliases:") || !strings.Contains(string(data), "title: Fix the parser") {
		t.Errorf("file =\n%s\nwant the built issue", data)
	}
}

func TestCreate_ParentByAlias_WritesIntoItsChildrenDirectory(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeNodeFile(t, s.IssuesDir(), "parser-epic.md", canonicalIssue)

	written, _, err := Issues(s, nil).Create(newIssueID, "old-parser", buildIssue(t, "open"))

	if err != nil || written.Path != ".capcom/issues/parser-epic/parser-fix.md" {
		t.Errorf("Create = %+v, %v; want the file below parser-epic", written, err)
	}
}

func TestCreate_UnrelatedTreeProblem_ReturnedRelativeToTheWorktree(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeNodeFile(t, filepath.Join(s.IssuesDir(), "stray-dir"), "stray-node.md", canonicalIssue)

	_, problems, err := Issues(s, nil).Create(newIssueID, "", buildIssue(t, "open"))

	if err != nil || len(problems) != 1 || problems[0].Path != ".capcom/issues/stray-dir" ||
		!errors.Is(problems[0], tree.ErrOrphanDirectory) {
		t.Errorf("Create = %v, %v; want the one orphan directory, relative", problems, err)
	}
}

func TestCreate_Refusals_WriteNothing(t *testing.T) {
	t.Parallel()
	errBuild := errors.New("no body")

	tests := []struct {
		name   string
		id     string
		parent string
		// setup writes the files of the tree below dir, the issues directory.
		setup  func(t *testing.T, dir string)
		build  func(t *testing.T) func() (issue.Issue, error)
		assert func(t *testing.T, err error)
	}{
		{
			name:   "invalid id",
			id:     "Not_Valid",
			assert: assertIs(tree.ErrInvalidID),
		},
		{
			name: "id taken by a node",
			id:   newIssueID,
			setup: func(t *testing.T, dir string) {
				writeNodeFile(t, dir, newIssueID+".md", canonicalIssue)
			},
			assert: assertExists(".capcom/issues/parser-fix.md", "parser-fix", "is already taken by issue"),
		},
		{
			name: "id an alias of a node",
			id:   "old-parser",
			setup: func(t *testing.T, dir string) {
				writeNodeFile(t, dir, "parser-epic.md", canonicalIssue)
			},
			assert: assertExists(".capcom/issues/parser-epic.md", "parser-epic", "is already an alias of issue"),
		},
		{
			name: "id taken inside an orphan directory",
			id:   newIssueID,
			setup: func(t *testing.T, dir string) {
				writeNodeFile(t, filepath.Join(dir, "stray-dir"), newIssueID+".md", canonicalIssue)
			},
			assert: assertExists(
				".capcom/issues/stray-dir/parser-fix.md",
				"parser-fix",
				"inside the orphan directory .capcom/issues/stray-dir",
			),
		},
		{
			name: "id maybe an alias of an unparseable node",
			id:   "old-parser",
			setup: func(t *testing.T, dir string) {
				writeNodeFile(t, dir, "broken-node.md", unparseable(canonicalIssue))
			},
			assert: assertExists(
				".capcom/issues/broken-node.md",
				"broken-node",
				"may be an alias of issue broken-node, whose file does not parse",
			),
		},
		{
			name:   "unknown parent",
			id:     newIssueID,
			parent: "no-such-parent",
			assert: assertIs(tree.ErrNotFound),
		},
		{
			name: "orphan directory where the children go",
			id:   newIssueID,
			setup: func(t *testing.T, dir string) {
				writeNodeFile(t, filepath.Join(dir, newIssueID), "child-node.md", canonicalIssue)
			},
			assert: func(t *testing.T, err error) {
				t.Helper()
				var problems tree.Errors
				if !errors.As(err, &problems) || len(problems) != 1 ||
					problems[0].Path != ".capcom/issues/parser-fix" {
					t.Errorf("err = %v, want the blocking orphan directory, relative", err)
				}
			},
		},
		{
			name: "build failure",
			id:   newIssueID,
			build: func(*testing.T) func() (issue.Issue, error) {
				return func() (issue.Issue, error) { return issue.Issue{}, errBuild }
			},
			assert: assertIs(errBuild),
		},
		{
			name: "invalid node",
			id:   newIssueID,
			build: func(t *testing.T) func() (issue.Issue, error) {
				return buildIssue(t, "undeclared")
			},
			assert: func(t *testing.T, err error) {
				t.Helper()
				var failure *FileError
				if !errors.As(err, &failure) || failure.Path != ".capcom/issues/parser-fix.md" ||
					failure.Line == 0 {
					t.Errorf("err = %v, want a *FileError at the new file, with a line", err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			s := newStore(t)
			if test.setup != nil {
				test.setup(t, s.IssuesDir())
			}
			build := buildIssue(t, "open")
			if test.build != nil {
				build = test.build(t)
			}
			before := nodeFiles(t, s.IssuesDir())

			_, _, err := Issues(s, nil).Create(test.id, test.parent, build)

			test.assert(t, err)
			if after := nodeFiles(t, s.IssuesDir()); len(after) != len(before) {
				t.Errorf("files %v after the refusal, want %v", after, before)
			}
		})
	}
}

func TestCreate_NoteIDMaybeAnAliasOfAnUnparseableNote_NamesANote(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeNodeFile(t, s.KnowledgeDir(), "broken-rule.md", unparseable(canonicalNote))
	build := func() (knowledge.Note, error) { return knowledge.Note{Title: "Rule"}, nil }

	_, _, err := Notes(s, nil).Create("old-rule", "", build)

	assertExists(".capcom/knowledge/broken-rule.md", "broken-rule", "may be an alias of note broken-rule")(t, err)
}

// assertIs returns an assertion that err wraps target.
func assertIs(target error) func(t *testing.T, err error) {
	return func(t *testing.T, err error) {
		t.Helper()
		if !errors.Is(err, target) {
			t.Errorf("err = %v, want %v", err, target)
		}
	}
}

// assertExists returns an assertion that err is an *ExistsError at path, held by holder, whose
// message contains text.
func assertExists(path string, holder string, text string) func(t *testing.T, err error) {
	return func(t *testing.T, err error) {
		t.Helper()
		var failure *ExistsError
		if !errors.As(err, &failure) || failure.Path != path || failure.Holder != holder ||
			!strings.Contains(failure.Message, text) {
			t.Errorf("err = %v, want an *ExistsError at %s held by %s saying %q", err, path, holder, text)
		}
	}
}
