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

// movedIssuePath is the file of the issue the Move tests move, relative to the working tree root.
const movedIssuePath = ".capcom/issues/login-flow.md"

// plainIssue is canonicalIssue without its Aliases.
var plainIssue = strings.Replace(canonicalIssue, "aliases: [old-parser]\n", "", 1)

// dependentIssue is plainIssue depending on login-flow.
var dependentIssue = strings.Replace(plainIssue, "---\n", "---\ndeps: [login-flow]\n", 1)

// newMoveIssues writes login-flow, a child below it and session-work depending on it into the
// issue tree of a fresh store, and returns the store's issue Kind and its issues directory.
func newMoveIssues(t *testing.T) (*Kind[issue.Issue], string) {
	t.Helper()
	s := newStore(t)
	dir := s.IssuesDir()
	writeNodeFile(t, dir, "login-flow.md", plainIssue)
	writeNodeFile(t, filepath.Join(dir, "login-flow"), "child-issue.md", plainIssue)
	writeNodeFile(t, dir, "session-work.md", dependentIssue)
	return Issues(s, nil), dir
}

func TestMove_RenameIssue_RewritesItsDependentsAndReturnsTheMove(t *testing.T) {
	t.Parallel()
	issues, dir := newMoveIssues(t)

	moved, problems, err := issues.Move("login", Destination{ID: "auth-flow"}, "")

	if err != nil || len(problems) != 0 {
		t.Fatalf("Move = %v, %v; want no failure and no problems", problems, err)
	}
	data := readFile(t, filepath.Join(dir, "auth-flow.md"))
	want := Moved{
		Written: Written{ID: "auth-flow", Path: ".capcom/issues/auth-flow.md", ETag: edit.ETag([]byte(data))},
		OldID:   "login-flow",
		OldPath: movedIssuePath,
	}
	if !movedEqual(moved, want) {
		t.Errorf("Moved = %+v, want %+v", moved, want)
	}
	if !strings.Contains(data, "aliases: [login-flow]\n") {
		t.Errorf("auth-flow.md =\n%s\nwant the old id as an alias", data)
	}
	dependent := readFile(t, filepath.Join(dir, "session-work.md"))
	if !strings.Contains(dependent, "deps: [auth-flow]\n") {
		t.Errorf("session-work.md =\n%s\nwant its deps renamed", dependent)
	}
	if files := nodeFiles(t, dir); !slices.Contains(files, filepath.Join("auth-flow", "child-issue.md")) {
		t.Errorf("files = %v, want the child below auth-flow", files)
	}
}

func TestMove_IfMatchCurrent_Moves(t *testing.T) {
	t.Parallel()
	issues, dir := newMoveIssues(t)

	_, _, err := issues.Move("login-flow", Destination{ID: "auth-flow"}, edit.ETag([]byte(plainIssue)))

	if err != nil {
		t.Fatalf("Move = %v, want the move", err)
	}
	if files := nodeFiles(t, dir); !slices.Contains(files, "auth-flow.md") {
		t.Errorf("files = %v, want auth-flow.md", files)
	}
}

func TestMove_UnreadIssue_ReturnedRelativeToTheWorktree(t *testing.T) {
	t.Parallel()
	issues, dir := newMoveIssues(t)
	writeNodeFile(t, dir, "broken-issue.md", unparseable(dependentIssue))

	moved, _, err := issues.Move("login-flow", Destination{ID: "auth-flow"}, "")

	if err != nil || !slices.Equal(moved.Unread, []string{".capcom/issues/broken-issue.md"}) {
		t.Errorf("Move = %+v, %v; want broken-issue.md unread", moved, err)
	}
}

func TestMove_UnrelatedTreeProblem_ReturnedRelativeToTheWorktree(t *testing.T) {
	t.Parallel()
	issues, dir := newMoveIssues(t)
	writeNodeFile(t, filepath.Join(dir, "stray-dir"), "lost-issue.md", plainIssue)

	_, problems, err := issues.Move("login-flow", Destination{ID: "auth-flow"}, "")

	if err != nil || len(problems) != 1 || problems[0].Path != ".capcom/issues/stray-dir" {
		t.Errorf("Move = %v, %v; want the orphan directory, relative", problems, err)
	}
}

func TestMove_NoteBelowAnotherThenToTheTop(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	dir := s.KnowledgeDir()
	writeNodeFile(t, dir, "release-flow.md", canonicalNote)
	writeNodeFile(t, dir, "house-conventions.md", strings.Replace(canonicalNote, "aliases: [old-rule]\n", "", 1))
	notes := Notes(s, nil)

	below, _, err := notes.Move("release-flow", Destination{Parent: "house-conventions", IsReparented: true}, "")
	if err != nil || below.Path != ".capcom/knowledge/house-conventions/release-flow.md" || below.ID != "release-flow" {
		t.Fatalf("Move below = %+v, %v", below, err)
	}
	top, _, err := notes.Move("old-rule", Destination{IsReparented: true}, "")

	if err != nil || top.Path != ".capcom/knowledge/release-flow.md" || top.OldPath != below.Path {
		t.Errorf("Move to the top = %+v, %v", top, err)
	}
	data := readFile(t, filepath.Join(dir, "release-flow.md"))
	if top.ETag != edit.ETag([]byte(data)) || data != canonicalNote {
		t.Errorf("release-flow.md =\n%s\nwant it unchanged, etag %s", data, top.ETag)
	}
}

func TestMove_StaleIfMatchOnANote_ConflictsNamingANote(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeNodeFile(t, s.KnowledgeDir(), "release-flow.md", canonicalNote)

	_, _, err := Notes(s, nil).Move("release-flow", Destination{ID: "release-steps"}, "stale")

	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Path != ".capcom/knowledge/release-flow.md" ||
		!strings.Contains(conflict.Message, "show the note again") {
		t.Errorf("err = %v, want a *ConflictError at the note file", err)
	}
}

func TestMove_Refusals_MoveNothing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ref     string
		to      Destination
		ifMatch string
		// setup writes more files into dir, the issues directory.
		setup  func(t *testing.T, dir string)
		assert func(t *testing.T, err error)
	}{
		{
			name:   "unknown issue",
			ref:    "no-such-issue",
			to:     Destination{ID: "auth-flow"},
			assert: assertIs(tree.ErrNotFound),
		},
		{
			name:   "unknown parent",
			ref:    "login-flow",
			to:     Destination{Parent: "no-such-issue", IsReparented: true},
			assert: assertIs(tree.ErrNotFound),
		},
		{
			name:   "new id taken",
			ref:    "login-flow",
			to:     Destination{ID: "session-work"},
			assert: assertExists(".capcom/issues/session-work.md", "session-work", "is already taken by issue"),
		},
		{
			name:   "parent inside its own subtree",
			ref:    "login-flow",
			to:     Destination{Parent: "child-issue", IsReparented: true},
			assert: assertMoveFileError(false),
		},
		{
			name: "moved issue does not parse",
			ref:  "login-flow",
			to:   Destination{ID: "auth-flow"},
			setup: func(t *testing.T, dir string) {
				writeNodeFile(t, dir, "login-flow.md", unparseable(plainIssue))
			},
			assert: assertMoveFileError(true),
		},
		{
			name: "new id taken inside an orphan directory",
			ref:  "login-flow",
			to:   Destination{ID: "auth-flow"},
			setup: func(t *testing.T, dir string) {
				writeNodeFile(t, filepath.Join(dir, "stray-dir"), "auth-flow.md", plainIssue)
			},
			assert: assertExists(".capcom/issues/stray-dir/auth-flow.md", "auth-flow", "inside the orphan directory"),
		},
		{
			name: "new id an alias of a dropped duplicate-id file",
			ref:  "login-flow",
			to:   Destination{ID: "old-bar"},
			setup: func(t *testing.T, dir string) {
				writeDuplicateIDFiles(t, dir, droppedIssue("aliases: [old-bar]\n"))
			},
			assert: assertExists(
				".capcom/issues/sub/foo.md",
				"foo",
				`id "old-bar" is already an alias of this file, left out for the duplicate id "foo"`,
			),
		},
		{
			name: "new id an alias of an unparseable issue",
			ref:  "login-flow",
			to:   Destination{ID: "old-parser"},
			setup: func(t *testing.T, dir string) {
				writeNodeFile(t, dir, "broken-issue.md", unparseable(canonicalIssue))
			},
			assert: assertExists(".capcom/issues/broken-issue.md", "broken-issue", "may be an alias of issue broken-issue"),
		},
		{
			name: "duplicate of the moved issue",
			ref:  "login-flow",
			to:   Destination{ID: "auth-flow"},
			setup: func(t *testing.T, dir string) {
				writeNodeFile(t, filepath.Join(dir, "session-work"), "login-flow.md", plainIssue)
			},
			assert: func(t *testing.T, err error) {
				t.Helper()
				var duplicate *tree.Error
				if !errors.As(err, &duplicate) || !errors.Is(err, tree.ErrDuplicate) ||
					!strings.Contains(err.Error(), movedIssuePath) {
					t.Errorf("err = %v, want the duplicate *tree.Error naming %s", err, movedIssuePath)
				}
			},
		},
		{
			name:    "stale etag",
			ref:     "login-flow",
			to:      Destination{ID: "auth-flow"},
			ifMatch: "stale",
			assert: func(t *testing.T, err error) {
				t.Helper()
				var conflict *ConflictError
				if !errors.As(err, &conflict) || conflict.Path != movedIssuePath || conflict.IfMatch != "stale" ||
					conflict.ETag != edit.ETag([]byte(plainIssue)) {
					t.Errorf("err = %v, want a *ConflictError at %s refusing stale", err, movedIssuePath)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			issues, dir := newMoveIssues(t)
			if test.setup != nil {
				test.setup(t, dir)
			}
			before := treeContent(t, dir)

			_, _, err := issues.Move(test.ref, test.to, test.ifMatch)

			test.assert(t, err)
			if after := treeContent(t, dir); !maps.Equal(after, before) {
				t.Errorf("tree changed by the refusal:\n%v\nwant\n%v", after, before)
			}
		})
	}
}

func TestMove_ApplyFails_NamesTheFilesRelativeToTheWorktree(t *testing.T) {
	t.Parallel()
	issues, dir := newMoveIssues(t)
	issues.applyMove = func(move *tree.Move) error {
		return &os.LinkError{Op: "rename", Old: move.Node.Path, New: move.NewPath, Err: fs.ErrPermission}
	}

	_, _, err := issues.Move("login-flow", Destination{ID: "auth-flow"}, "")

	var failure *FileError
	if !errors.As(err, &failure) || failure.Path != movedIssuePath || failure.Line != 0 ||
		strings.Contains(failure.Message, issues.worktreeRoot) ||
		!strings.Contains(failure.Message, filepath.Join(".capcom", "issues", "auth-flow.md")) {
		t.Errorf("err = %v, want a *FileError at %s naming the new file relative", err, movedIssuePath)
	}
	if files := nodeFiles(t, dir); slices.Contains(files, "auth-flow.md") {
		t.Errorf("files = %v, want nothing moved", files)
	}
}

func TestMove_DependentChangedBeforeApply_ConflictsAndKeepsTheConcurrentBytes(t *testing.T) {
	t.Parallel()
	issues, dir := newMoveIssues(t)
	dependentPath := filepath.Join(dir, "session-work.md")
	concurrent := strings.Replace(dependentIssue, "Fix the parser", "Fix the parser today", 1)
	issues.applyMove = func(move *tree.Move) error {
		writeNodeFile(t, dir, "session-work.md", concurrent)
		return move.Apply()
	}

	_, _, err := issues.Move("login-flow", Destination{ID: "auth-flow"}, "")

	assertChangedConflict(t, err, ".capcom/issues/session-work.md")
	if got := readFile(t, dependentPath); got != concurrent {
		t.Errorf("session-work.md =\n%s\nwant the concurrent bytes kept", got)
	}
	if files := nodeFiles(t, dir); !slices.Contains(files, "login-flow.md") || slices.Contains(files, "auth-flow.md") {
		t.Errorf("files = %v, want login-flow.md at its old path", files)
	}
}

// assertChangedConflict asserts that err is a *ConflictError at path, the file changed under the
// command, with no etags and the message asking to run the command again.
func assertChangedConflict(t *testing.T, err error, path string) {
	t.Helper()
	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Path != path || conflict.IfMatch != "" || conflict.ETag != "" ||
		conflict.Message != "changed since this command read it; run the command again" {
		t.Errorf("err = %#v, want a *ConflictError at %s saying it changed since the command read it", err, path)
	}
}

// assertMoveFileError returns an assertion that err is a *FileError at the moved issue's file,
// citing a line when hasLine.
func assertMoveFileError(hasLine bool) func(t *testing.T, err error) {
	return func(t *testing.T, err error) {
		t.Helper()
		var failure *FileError
		if !errors.As(err, &failure) || failure.Path != movedIssuePath || (failure.Line > 0) != hasLine {
			t.Errorf("err = %v, want a *FileError at %s (line: %t)", err, movedIssuePath, hasLine)
		}
	}
}

// movedEqual reports whether a and b are the same move.
func movedEqual(a Moved, b Moved) bool {
	return a.Written == b.Written && a.OldID == b.OldID && a.OldPath == b.OldPath && slices.Equal(a.Unread, b.Unread)
}

// treeContent returns the content of every file below dir, by its path relative to dir.
func treeContent(t *testing.T, dir string) map[string]string {
	t.Helper()
	content := map[string]string{}
	for _, file := range nodeFiles(t, dir) {
		content[file] = readFile(t, filepath.Join(dir, file))
	}
	return content
}
