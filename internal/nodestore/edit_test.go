package nodestore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodefile"
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

	written, err := issues.Edit("old-parser", nodefile.ETag([]byte(canonicalIssue)), retitle("Fix the lexer", &calls))

	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	content := readFile(t, path)
	want := Written{ID: editedIssueID, Path: editedIssuePath, ETag: nodefile.ETag([]byte(content))}
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

// TestEdit_FileChangedBeforeTheWrite_FailsWithAConflictAndKeepsTheNewerBytes is not parallel: it
// sets the package-level beforeRecheck hook, which the parallel tests only run after it restores.
func TestEdit_FileChangedBeforeTheWrite_FailsWithAConflictAndKeepsTheNewerBytes(t *testing.T) {
	newer := strings.Replace(canonicalIssue, "title: Fix the parser", "title: Fix the tokenizer", 1)
	tests := []struct {
		name    string
		ifMatch string
	}{
		{name: "without an etag", ifMatch: ""},
		{name: "with the matching etag", ifMatch: nodefile.ETag([]byte(canonicalIssue))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			issues, path := newEditedIssue(t)
			setBeforeRecheck(t, func(string) {
				writeNodeFile(t, issues.dir, editedIssueID+".md", newer)
			})
			calls := 0

			written, err := issues.Edit(editedIssueID, test.ifMatch, retitle("Fix the lexer", &calls))

			var conflict *ConflictError
			if !errors.As(err, &conflict) || conflict.Path != editedIssuePath ||
				conflict.IfMatch != nodefile.ETag([]byte(canonicalIssue)) ||
				conflict.ETag != nodefile.ETag([]byte(newer)) {
				t.Errorf("err = %v, want a *ConflictError naming the first-read etag as stale", err)
			}
			if content := readFile(t, path); content != newer {
				t.Errorf("file =\n%s\nwant the newer bytes kept", content)
			}
			if calls != 1 {
				t.Errorf("change ran %d times, want once", calls)
			}
			if want := (Written{ID: editedIssueID, Path: editedIssuePath}); written != want {
				t.Errorf("Written = %+v, want %+v", written, want)
			}
		})
	}
}

// setBeforeRecheck installs hook as beforeRecheck for the rest of the test.
func setBeforeRecheck(t *testing.T, hook func(path string)) {
	t.Helper()
	previous := beforeRecheck
	beforeRecheck = hook
	t.Cleanup(func() { beforeRecheck = previous })
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
			conflict.ETag != nodefile.ETag([]byte(canonicalIssue)) {
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

func TestEdit_NodeFileBecameASymlink_FailsWithAFileErrorAtItAndWritesNothing(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests do not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("symlink tests do not run as root")
	}
	issues, path := newEditedIssue(t)
	target := writeNodeFile(t, t.TempDir(), editedIssueID+".md", canonicalIssue)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("Symlink(%s): %v", path, err)
	}
	calls := 0

	_, err := issues.Edit(editedIssueID, "", retitle("Fix the lexer", &calls))

	var failure *FileError
	if !errors.As(err, &failure) || failure.Path != editedIssuePath ||
		!strings.Contains(failure.Message, "not a regular file") {
		t.Errorf("err = %v, want a *FileError at %s saying it is not a regular file", err, editedIssuePath)
	}
	if calls != 0 {
		t.Errorf("change ran %d times, want none", calls)
	}
	if content := readFile(t, target); content != canonicalIssue {
		t.Errorf("symlink target changed:\n%s", content)
	}
}

// keylessConfig is a config.yaml that declares no store format, so the store is at format 1.
const keylessConfig = "types: [bug, feature, task, chore]\n"

// writeStoreConfig writes content as the config.yaml of the store holding the issue file
// issuePath, a file at the top of its issue tree, and returns the config file's path.
func writeStoreConfig(t *testing.T, issuePath string, content string) string {
	t.Helper()
	return writeNodeFile(t, filepath.Dir(filepath.Dir(issuePath)), "config.yaml", content)
}

// linkDBSchema is a change giving an issue the related entry db-schema and the title title.
func linkDBSchema(title string) func(issue.Issue) (issue.Issue, error) {
	return func(node issue.Issue) (issue.Issue, error) {
		node.Title = title
		node.Related = []string{"db-schema"}
		return node, nil
	}
}

func TestEdit_GainsRelatedBelowFormatTwo_RaisesBeforeTheWrite(t *testing.T) {
	t.Parallel()
	issues, path := newEditedIssue(t)
	configPath := writeStoreConfig(t, path, keylessConfig)
	issueAtRaise := ""
	raiseFormat := issues.raiseFormat
	issues.raiseFormat = func(version int) (bool, error) {
		issueAtRaise = readFile(t, path)
		return raiseFormat(version)
	}

	written, err := issues.Edit(editedIssueID, "", linkDBSchema("Fix the parser"))

	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if issueAtRaise != canonicalIssue {
		t.Errorf("issue file at the raise =\n%s\nwant it not yet written", issueAtRaise)
	}
	wantLine := fmt.Sprintf("format: %d", config.FormatRelated)
	if content := readFile(t, configPath); written.RaisedFormat != config.FormatRelated ||
		!strings.Contains(content, wantLine) {
		t.Errorf("RaisedFormat = %d, config.yaml = %q; want %d and %q",
			written.RaisedFormat, content, config.FormatRelated, wantLine)
	}
	if content := readFile(t, path); !strings.Contains(content, "related: [db-schema]") {
		t.Errorf("issue file =\n%s\nwant the related link written", content)
	}
}

// TestEdit_ConflictBelowFormatTwo_LeavesTheFormatAlone is not parallel: it sets the package-level
// beforeRecheck hook, which the parallel tests only run after it restores.
func TestEdit_ConflictBelowFormatTwo_LeavesTheFormatAlone(t *testing.T) {
	issues, path := newEditedIssue(t)
	const formatOne = "format: 1\n"
	configPath := writeStoreConfig(t, path, formatOne)
	newer := strings.Replace(canonicalIssue, "title: Fix the parser", "title: Fix the tokenizer", 1)
	setBeforeRecheck(t, func(string) {
		writeNodeFile(t, issues.dir, editedIssueID+".md", newer)
	})

	written, err := issues.Edit(editedIssueID, "", linkDBSchema("Fix the parser"))

	var conflict *ConflictError
	if !errors.As(err, &conflict) || written.RaisedFormat != 0 {
		t.Errorf("Written = %+v err = %v, want a *ConflictError and no raise", written, err)
	}
	if content := readFile(t, configPath); content != formatOne {
		t.Errorf("config.yaml = %q, want it unchanged", content)
	}
}

func TestEdit_InvalidResult_LeavesTheFormatAlone(t *testing.T) {
	t.Parallel()
	issues, path := newEditedIssue(t)
	configPath := writeStoreConfig(t, path, keylessConfig)

	written, err := issues.Edit(editedIssueID, "", linkDBSchema(""))

	var failure *FileError
	if !errors.As(err, &failure) || failure.Path != editedIssuePath || written.RaisedFormat != 0 {
		t.Errorf("Written = %+v err = %v, want a *FileError at the issue file and no raise", written, err)
	}
	if content := readFile(t, configPath); content != keylessConfig {
		t.Errorf("config.yaml = %q, want it unchanged", content)
	}
	if content := readFile(t, path); content != canonicalIssue {
		t.Errorf("issue file =\n%s\nwant it unchanged", content)
	}
}

func TestEdit_RaiseFails_FailsAtTheConfigAndWritesNothing(t *testing.T) {
	t.Parallel()
	issues, path := newEditedIssue(t)
	configPath := filepath.Join(filepath.Dir(filepath.Dir(path)), "config.yaml")
	if err := os.Mkdir(configPath, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := issues.Edit(editedIssueID, "", linkDBSchema("Fix the parser"))

	var failure *FileError
	if !errors.As(err, &failure) || failure.Path != ".capcom/config.yaml" ||
		failure.Message != tree.ErrNotRegular.Error() {
		t.Errorf("err = %v, want a *FileError at .capcom/config.yaml naming the refusal", err)
	}
	if content := readFile(t, path); content != canonicalIssue {
		t.Errorf("issue file =\n%s\nwant it unchanged", content)
	}
}
