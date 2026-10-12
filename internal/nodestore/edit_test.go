package nodestore

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/tree"
)

// editedIssueID is the id of the issue the Edit tests change; editedIssuePath is its file.
const (
	editedIssueID   = "parser-fix"
	editedIssuePath = ".capcom/issues/parser-fix.md"
)

// fakeResolver resolves an argument by looking it up in its map, failing with tree.ErrNotFound for
// any other argument.
type fakeResolver map[string]Target

// Resolve returns the Target arg maps to.
func (r fakeResolver) Resolve(arg string) (Target, error) {
	target, isFound := r[arg]
	if !isFound {
		return Target{}, fmt.Errorf("%w: %s", tree.ErrNotFound, arg)
	}
	return target, nil
}

// newEditedIssue writes the canonical issue as parser-fix and returns the issue store of s
// resolving it by its id and its alias, and the issue file's path.
func newEditedIssue(t *testing.T) (*Kind[issue.Issue], string) {
	t.Helper()
	s := newStore(t)
	path := writeNodeFile(t, s.IssuesDir(), editedIssueID+".md", canonicalIssue)
	target := Target{ID: editedIssueID, Path: editedIssuePath}
	return Issues(s, fakeResolver{editedIssueID: target, "old-parser": target}), path
}

// retitle returns a change giving an issue title, counting its calls in calls.
func retitle(title string, calls *int) func(issue.Issue) (issue.Issue, error) {
	return func(node issue.Issue) (issue.Issue, error) {
		*calls++
		node.Title = title
		return node, nil
	}
}

// readFile returns the content of the file at path.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestEdit_ThroughAnAlias_WritesTheChangeAndReturnsTheNewETag(t *testing.T) {
	t.Parallel()
	issues, path := newEditedIssue(t)
	calls := 0

	written, err := issues.Edit("old-parser", edit.ETag([]byte(canonicalIssue)), retitle("Fix the lexer", &calls))

	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	content := readFile(t, path)
	want := Written{ID: editedIssueID, Path: editedIssuePath, ETag: edit.ETag([]byte(content))}
	if written != want {
		t.Errorf("Written = %+v, want %+v", written, want)
	}
	if calls != 1 || !strings.Contains(content, "title: Fix the lexer") {
		t.Errorf("change ran %d times, file =\n%s\nwant one run and the new title", calls, content)
	}
}

func TestEdit_Refusals_WriteNothing(t *testing.T) {
	t.Parallel()
	errChange := errors.New("the change refuses")

	tests := []struct {
		name string
		// content replaces the canonical issue file before the edit when set.
		content     string
		ref         string
		ifMatch     string
		change      func(issue.Issue) (issue.Issue, error)
		wantChanged bool
		assert      func(t *testing.T, err error)
	}{
		{
			name:   "unresolved ref",
			ref:    "missing-issue",
			change: func(node issue.Issue) (issue.Issue, error) { return node, nil },
			assert: assertIs(tree.ErrNotFound),
		},
		{
			name:    "stale etag",
			ref:     editedIssueID,
			ifMatch: "0000000000000000",
			change:  func(node issue.Issue) (issue.Issue, error) { return node, nil },
			assert:  assertConflict("0000000000000000"),
		},
		{
			name:    "unparseable file",
			content: unparseable(canonicalIssue),
			ref:     editedIssueID,
			change:  func(node issue.Issue) (issue.Issue, error) { return node, nil },
			assert:  assertFileError(2, "bogus"),
		},
		{
			name:        "change failure",
			ref:         editedIssueID,
			change:      func(issue.Issue) (issue.Issue, error) { return issue.Issue{}, errChange },
			wantChanged: true,
			assert: func(t *testing.T, err error) {
				t.Helper()
				if err != errChange {
					t.Errorf("err = %v, want the change's error unwrapped", err)
				}
			},
		},
		{
			name: "invalid result",
			ref:  editedIssueID,
			change: func(node issue.Issue) (issue.Issue, error) {
				node.Status = "nonsense"
				node.Type = "nonsense"
				return node, nil
			},
			wantChanged: true,
			assert:      assertFileError(3, `"nonsense"`),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			issues, path := newEditedIssue(t)
			if test.content != "" {
				writeNodeFile(t, issues.dir, editedIssueID+".md", test.content)
			}
			before := readFile(t, path)
			isChanged := false

			written, err := issues.Edit(test.ref, test.ifMatch, func(node issue.Issue) (issue.Issue, error) {
				isChanged = true
				return test.change(node)
			})

			test.assert(t, err)
			if isChanged != test.wantChanged {
				t.Errorf("change ran = %t, want %t", isChanged, test.wantChanged)
			}
			if after := readFile(t, path); after != before {
				t.Errorf("file changed:\n%s", after)
			}
			wantLocated := Written{ID: editedIssueID, Path: editedIssuePath}
			if test.ref != editedIssueID {
				wantLocated = Written{}
			}
			if written != wantLocated {
				t.Errorf("Written = %+v, want %+v", written, wantLocated)
			}
		})
	}
}

func TestEdit_InvalidNote_FailsAtTheNoteFileWithEveryProblem(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeNodeFile(t, s.KnowledgeDir(), "quoting.md", canonicalNote)
	notes := Notes(s, fakeResolver{"quoting": {ID: "quoting", Path: ".capcom/knowledge/quoting.md"}})

	_, err := notes.Edit("quoting", "", func(note knowledge.Note) (knowledge.Note, error) {
		note.Title = ""
		note.Labels = []string{""}
		return note, nil
	})

	var failure *FileError
	if !errors.As(err, &failure) || failure.Path != ".capcom/knowledge/quoting.md" ||
		!strings.Contains(failure.Message, problemSeparator) {
		t.Errorf("err = %v, want a *FileError at the Note file listing both problems", err)
	}
}

func TestEdit_NoResolver_Fails(t *testing.T) {
	t.Parallel()
	s := newStore(t)

	_, err := Issues(s, nil).Edit(editedIssueID, "", func(node issue.Issue) (issue.Issue, error) { return node, nil })

	if !errors.Is(err, errNoResolver) {
		t.Errorf("err = %v, want errNoResolver", err)
	}
}

// assertConflict asserts a *ConflictError at the edited issue's file refusing ifMatch.
func assertConflict(ifMatch string) func(t *testing.T, err error) {
	return func(t *testing.T, err error) {
		t.Helper()
		var conflict *ConflictError
		if !errors.As(err, &conflict) || conflict.Path != editedIssuePath || conflict.IfMatch != ifMatch ||
			conflict.ETag != edit.ETag([]byte(canonicalIssue)) {
			t.Errorf("err = %v, want a *ConflictError at %s refusing %s", err, editedIssuePath, ifMatch)
		}
	}
}

// assertFileError asserts a *FileError at the edited issue's file, citing line, whose message
// contains text.
func assertFileError(line int, text string) func(t *testing.T, err error) {
	return func(t *testing.T, err error) {
		t.Helper()
		var failure *FileError
		if !errors.As(err, &failure) || failure.Path != editedIssuePath || failure.Line != line ||
			!strings.Contains(failure.Message, text) {
			t.Errorf("err = %v, want a *FileError at %s:%d containing %q", err, editedIssuePath, line, text)
		}
	}
}
