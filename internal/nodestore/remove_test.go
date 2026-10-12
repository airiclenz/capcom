package nodestore

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/tree"
)

// removedIssuePath is the file of the issue the Remove tests remove, relative to the working tree
// root.
const removedIssuePath = ".capcom/issues/fix-parser.md"

// parserDependentIssue is plainIssue depending on fix-parser.
var parserDependentIssue = strings.Replace(plainIssue, "---\n", "---\ndeps: [fix-parser]\n", 1)

// newRemoveIssues writes fix-parser, session-work depending on it, and login-flow with child-issue
// below it into the issue tree of a fresh store, and returns the store's issue Kind and its issues
// directory.
func newRemoveIssues(t *testing.T) (*Kind[issue.Issue], string) {
	t.Helper()
	s := newStore(t)
	dir := s.IssuesDir()
	writeNodeFile(t, dir, "fix-parser.md", plainIssue)
	writeNodeFile(t, dir, "session-work.md", parserDependentIssue)
	writeNodeFile(t, dir, "login-flow.md", plainIssue)
	writeNodeFile(t, filepath.Join(dir, "login-flow"), "child-issue.md", plainIssue)
	return Issues(s, nil), dir
}

func TestRemove_LastChild_RemovesItAndTheEmptiedDirectory(t *testing.T) {
	t.Parallel()
	issues, dir := newRemoveIssues(t)

	removed, problems, err := issues.Remove("child", "", false)

	if err != nil || len(problems) != 0 {
		t.Fatalf("Remove = %v, %v; want no failure and no problems", problems, err)
	}
	if removed.ID != "child-issue" || removed.Path != ".capcom/issues/login-flow/child-issue.md" {
		t.Errorf("Removed = %+v, want child-issue at its file relative to the worktree", removed)
	}
	if _, err := os.Stat(filepath.Join(dir, "login-flow")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("stat login-flow = %v, want the emptied directory removed", err)
	}
}

func TestRemove_DropDeps_RemovesTheIssueAndItsDependentsDeps(t *testing.T) {
	t.Parallel()
	issues, dir := newRemoveIssues(t)
	etag := edit.ETag([]byte(plainIssue))

	removed, _, err := issues.Remove("fix-parser", etag, true)

	if err != nil || removed.Path != removedIssuePath || !slices.Equal(removed.DepsDropped, []string{"session-work"}) {
		t.Fatalf("Remove = %+v, %v; want fix-parser removed and session-work's deps dropped", removed, err)
	}
	if dependent := readFile(t, filepath.Join(dir, "session-work.md")); dependent != plainIssue {
		t.Errorf("session-work.md =\n%s\nwant its deps entry dropped", dependent)
	}
}

func TestRemove_StaleIfMatchOnANote_ConflictsNamingANote(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeNodeFile(t, s.KnowledgeDir(), "release-flow.md", canonicalNote)

	_, _, err := Notes(s, nil).Remove("release-flow", "stale", false)

	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Path != ".capcom/knowledge/release-flow.md" ||
		!strings.Contains(conflict.Message, "show the note again") {
		t.Errorf("err = %v, want a *ConflictError at the note file", err)
	}
}

func TestRemove_Refusals_RemoveNothing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		ref            string
		ifMatch        string
		shouldDropDeps bool
		// setup writes more files into dir, the issues directory.
		setup  func(t *testing.T, dir string)
		assert func(t *testing.T, err error)
	}{
		{
			name:   "unknown issue",
			ref:    "no-such-issue",
			assert: assertIs(tree.ErrNotFound),
		},
		{
			name:   "dependents",
			ref:    "fix-parser",
			assert: assertDependents(removedIssuePath, "session-work"),
		},
		{
			name:    "stale etag before the dependents",
			ref:     "fix-parser",
			ifMatch: "stale",
			assert:  assertRemoveConflict(removedIssuePath),
		},
		{
			name:    "children before a stale etag",
			ref:     "login-flow",
			ifMatch: "stale",
			assert:  assertRemoveFileError(".capcom/issues/login-flow.md", "has 1 child issue"),
		},
		{
			name: "children directory holding a stray file",
			ref:  "fix-parser",
			setup: func(t *testing.T, dir string) {
				writeNodeFile(t, filepath.Join(dir, "fix-parser"), "diagram.png", "png")
			},
			shouldDropDeps: true,
			assert:         assertRemoveFileError(".capcom/issues/fix-parser", "still holds 1 entry"),
		},
		{
			name: "duplicate of the parent",
			ref:  "child-issue",
			setup: func(t *testing.T, dir string) {
				writeNodeFile(t, dir, "zz-other.md", plainIssue)
				writeNodeFile(t, filepath.Join(dir, "zz-other"), "login-flow.md", plainIssue)
			},
			assert: func(t *testing.T, err error) {
				t.Helper()
				var problems tree.Errors
				if !errors.As(err, &problems) || !strings.Contains(err.Error(), `duplicate id "login-flow"`) {
					t.Errorf("err = %v, want the duplicate parent as tree.Errors", err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			issues, dir := newRemoveIssues(t)
			if test.setup != nil {
				test.setup(t, dir)
			}
			before := treeContent(t, dir)

			_, _, err := issues.Remove(test.ref, test.ifMatch, test.shouldDropDeps)

			test.assert(t, err)
			if after := treeContent(t, dir); !maps.Equal(after, before) {
				t.Errorf("tree changed by the refusal:\n%v\nwant\n%v", after, before)
			}
		})
	}
}

func TestRemove_ApplyFails_NamesTheFailedFileRelativeToTheWorktree(t *testing.T) {
	t.Parallel()
	issues, dir := newRemoveIssues(t)
	issues.applyRemoval = func(removal *tree.Removal) ([]error, error) {
		return nil, &fs.PathError{Op: "remove", Path: removal.Node.Path, Err: fs.ErrPermission}
	}

	_, _, err := issues.Remove("fix-parser", "", true)

	var failure *FileError
	if !errors.As(err, &failure) || failure.Path != removedIssuePath ||
		strings.Contains(failure.Message, issues.worktreeRoot) {
		t.Errorf("err = %v, want a *FileError at %s naming no absolute path", err, removedIssuePath)
	}
	if files := nodeFiles(t, dir); !slices.Contains(files, "fix-parser.md") {
		t.Errorf("files = %v, want fix-parser.md kept", files)
	}
}

func TestRemove_CleanupFails_ReturnsTheDirectoryLeftInPlace(t *testing.T) {
	t.Parallel()
	issues, dir := newRemoveIssues(t)
	parentDir := filepath.Join(dir, "login-flow")
	issues.applyRemoval = func(*tree.Removal) ([]error, error) {
		cause := &fs.PathError{Op: "remove", Path: parentDir, Err: fs.ErrPermission}
		return []error{&tree.CleanupError{Path: parentDir, Err: cause}}, nil
	}

	removed, _, err := issues.Remove("child-issue", "", false)

	if err != nil || len(removed.LeftInPlace) != 1 {
		t.Fatalf("Remove = %+v, %v; want one directory left in place", removed, err)
	}
	left := removed.LeftInPlace[0]
	if left.Path != ".capcom/issues/login-flow" || strings.Contains(left.Message, issues.worktreeRoot) {
		t.Errorf("LeftInPlace = %+v, want the parent directory relative to the worktree", left)
	}
}

// assertDependents returns an assertion that err is a *DependentsError at path naming dependents.
func assertDependents(path string, dependents ...string) func(t *testing.T, err error) {
	return func(t *testing.T, err error) {
		t.Helper()
		var failure *DependentsError
		if !errors.As(err, &failure) || failure.Path != path || !slices.Equal(failure.Dependents, dependents) {
			t.Errorf("err = %v, want a *DependentsError at %s naming %v", err, path, dependents)
		}
	}
}

// assertRemoveConflict returns an assertion that err is a *ConflictError at path.
func assertRemoveConflict(path string) func(t *testing.T, err error) {
	return func(t *testing.T, err error) {
		t.Helper()
		var conflict *ConflictError
		if !errors.As(err, &conflict) || conflict.Path != path {
			t.Errorf("err = %v, want a *ConflictError at %s", err, path)
		}
	}
}

// assertRemoveFileError returns an assertion that err is a *FileError at path whose message
// contains message.
func assertRemoveFileError(path string, message string) func(t *testing.T, err error) {
	return func(t *testing.T, err error) {
		t.Helper()
		var failure *FileError
		if !errors.As(err, &failure) || failure.Path != path || !strings.Contains(failure.Message, message) {
			t.Errorf("err = %v, want a *FileError at %s containing %q", err, path, message)
		}
	}
}
