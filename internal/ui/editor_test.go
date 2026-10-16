package ui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/check"
)

// fakeEditorCommand is $VISUAL in the editor tests: fakeEditor drops its first word and hands the
// rest to the test binary, so only the flag reaches the fake editor.
const fakeEditorCommand = "fake-editor --flag"

// editWidth is wide enough for a check problem to fit the status bar whole.
const editWidth = 400

// fakeEditor stands in for the user's editor: it replaces old with new in the file it is handed,
// recording its arguments in argsPath.
type fakeEditor struct {
	old      string
	new      string
	argsPath string
}

// newFakeEditor returns a fake editor that replaces old with new.
func newFakeEditor(t *testing.T, old, new string) fakeEditor {
	t.Helper()
	return fakeEditor{old: old, new: new, argsPath: filepath.Join(t.TempDir(), "args")}
}

// exec is the model's execFn in tests: it re-executes the test binary as the fake editor with
// argv's tail, waits for it, and hands its error to done, as tea.ExecProcess does once the editor
// exits.
func (f fakeEditor) exec(argv []string, done tea.ExecCallback) tea.Cmd {
	return func() tea.Msg {
		command := exec.Command(os.Args[0], argv[1:]...)
		command.Env = append(os.Environ(),
			fakeEditorVariable+"=1",
			fakeEditOldVariable+"="+f.old,
			fakeEditNewVariable+"="+f.new,
			fakeEditorArgsVariable+"="+f.argsPath,
		)
		return done(command.Run())
	}
}

// args returns the arguments the fake editor was last run with, and nil when it never ran.
func (f fakeEditor) args(t *testing.T) []string {
	t.Helper()
	content, err := os.ReadFile(f.argsPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("ReadFile %s: %v", f.argsPath, err)
	}
	return strings.Split(string(content), "\n")
}

// startEditModel returns a started model over ts on tab, editWidth wide, with $VISUAL naming
// fakeEditorCommand, $EDITOR unset, and editor standing in for the editor.
func startEditModel(t *testing.T, ts *testStore, tab Tab, editor fakeEditor) model {
	t.Helper()
	t.Setenv(visualVariable, fakeEditorCommand)
	t.Setenv(editorVariable, "")
	options := newTestOptions(t, ts)
	options.Tab = tab
	m := startModel(t, options, editWidth, 30)
	m.execFn = editor.exec
	return m
}

// wantCheckNotice returns the status bar's report of a check of ts's store as it is now, after an
// edit of the file at path, relative to .capcom: the first problem in that file, else the first of
// all, with the count.
func wantCheckNotice(t *testing.T, ts *testStore, path string) string {
	t.Helper()
	problems, err := check.Run(ts.store)
	if err != nil {
		t.Fatalf("check.Run: %v", err)
	}
	return checkNotice(ts.store.Relative(filepath.Join(ts.store.Root, filepath.FromSlash(path))), problems)
}

func TestEdit_FakeEditor_EditsTheFileAndTheChangeShows(t *testing.T) {
	ts := newTestStore(t, readyFiles)
	editor := newFakeEditor(t, "Alpha names", "Alpha now names")
	m := startEditModel(t, ts, TabReady, editor)

	m = press(t, m, runeKey('e'))

	wantArgs := []string{"--flag", filepath.Join(ts.store.Root, filepath.FromSlash(alphaPath))}
	if got := editor.args(t); !slices.Equal(got, wantArgs) {
		t.Errorf("editor args = %q, want %q", got, wantArgs)
	}
	if got := string(readStoreFile(t, ts, alphaPath)); !strings.Contains(got, "Alpha now names") {
		t.Errorf("alpha after e:\n%s\nwant the fake editor's change", got)
	}
	if got := detailText(m); !strings.Contains(got, "Alpha now names") {
		t.Errorf("detail pane after e:\n%s\nwant the edited body", got)
	}
	if got, want := statusRow(m), wantCheckNotice(t, ts, alphaPath); !strings.Contains(got, want) {
		t.Errorf("status bar = %q, want it to hold %q", got, want)
	}
}

func TestEdit_KnowledgeTab_EditsTheNote(t *testing.T) {
	files := map[string]string{"knowledge/note.md": "---\ntitle: A Note\n---\nBody.\n"}
	ts := newTestStore(t, files)
	editor := newFakeEditor(t, "Body.", "Edited body.")
	m := startEditModel(t, ts, TabKnowledge, editor)

	m = press(t, m, runeKey('e'))

	if got := string(readStoreFile(t, ts, "knowledge/note.md")); !strings.Contains(got, "Edited body.") {
		t.Errorf("note after e:\n%s\nwant the fake editor's change", got)
	}
	if got := detailText(m); !strings.Contains(got, "Edited body.") {
		t.Errorf("detail pane after e:\n%s\nwant the edited body", got)
	}
}

func TestEdit_BrokenFrontmatter_ShowsTheCheckProblem(t *testing.T) {
	ts := newTestStore(t, readyFiles)
	editor := newFakeEditor(t, "priority: 1\n", "priority: high\n")
	m := startEditModel(t, ts, TabReady, editor)

	m = press(t, m, runeKey('e'))

	want := wantCheckNotice(t, ts, alphaPath)
	if !strings.Contains(want, "issues/alpha.md") {
		t.Fatalf("check notice = %q, want a problem in alpha's file", want)
	}
	if got := statusRow(m); !strings.Contains(got, want) {
		t.Errorf("status bar = %q, want it to hold %q", got, want)
	}
}

func TestEdit_ProblemInAnotherFile_ShowsTheEditedFilesFirst(t *testing.T) {
	files := map[string]string{
		"issues/aardvark.md": issueFile("title: Aardvark\nstatus: done\npriority: 4\n"+
			"closed: 2026-10-12T09:00:00Z\nreason: shipped\ndeps: [no-such-issue]\n", ""),
	}
	for name, content := range readyFiles {
		files[name] = content
	}
	ts := newTestStore(t, files)
	before, err := check.Run(ts.store)
	if err != nil {
		t.Fatalf("check.Run: %v", err)
	}
	if len(before) == 0 || strings.Contains(before[0].Path, "alpha") {
		t.Fatalf("problems before the edit = %v, want aardvark's first", before)
	}
	editor := newFakeEditor(t, "priority: 1\n", "priority: high\n")
	m := startEditModel(t, ts, TabReady, editor)

	m = press(t, m, runeKey('e'))

	after, err := check.Run(ts.store)
	if err != nil {
		t.Fatalf("check.Run: %v", err)
	}
	inAlpha := slices.IndexFunc(after, func(problem check.Problem) bool {
		return strings.HasSuffix(problem.Path, alphaPath)
	})
	if inAlpha <= 0 {
		t.Fatalf("problems after the edit = %v, want alpha's after aardvark's", after)
	}
	want := fmt.Sprintf(problemsFormat, oneLine(after[inAlpha].String()), len(after))
	if got := statusRow(m); !strings.Contains(got, want) {
		t.Errorf("status bar = %q, want it to hold %q", got, want)
	}
}

func TestEdit_NoEditor_ShowsTheMessage(t *testing.T) {
	ts := newTestStore(t, readyFiles)
	editor := newFakeEditor(t, "Alpha names", "Alpha now names")
	m := startEditModel(t, ts, TabReady, editor)
	t.Setenv(visualVariable, "  ")
	t.Setenv(editorVariable, "")
	before := readStoreFile(t, ts, alphaPath)

	m = press(t, m, runeKey('e'))

	if got := statusRow(m); !strings.Contains(got, noEditorText) {
		t.Errorf("status bar = %q, want it to hold %q", got, noEditorText)
	}
	if got := editor.args(t); got != nil {
		t.Errorf("editor ran with %q, want it never run", got)
	}
	if got := readStoreFile(t, ts, alphaPath); !bytes.Equal(got, before) {
		t.Errorf("alpha after e:\n%s\nwant it unchanged", got)
	}
}

func TestEdit_EditorFails_ShowsItsError(t *testing.T) {
	ts := newTestStore(t, readyFiles)
	editor := newFakeEditor(t, "text alpha does not hold", "")
	m := startEditModel(t, ts, TabReady, editor)

	m = press(t, m, runeKey('e'))

	want := fmt.Sprintf("%sexit status %d", editorFailurePrefix, fakeEditorMissingCode)
	if got := statusRow(m); !strings.Contains(got, want) {
		t.Errorf("status bar = %q, want it to hold %q", got, want)
	}
}

func TestEdit_HeldLock_AsksBeforeEditing(t *testing.T) {
	answers := []struct {
		name     string
		key      tea.KeyPressMsg
		isEdited bool
	}{
		{name: "y", key: runeKey('y'), isEdited: true},
		{name: "n", key: runeKey('n'), isEdited: false},
	}
	for _, answer := range answers {
		t.Run(answer.name, func(t *testing.T) {
			ts := newTestStore(t, readyFiles)
			acquireTestLock(t, ts)
			before := readStoreFile(t, ts, alphaPath)
			editor := newFakeEditor(t, "Alpha names", "Alpha now names")
			m := startEditModel(t, ts, TabReady, editor)

			m = press(t, m, runeKey('e'))
			if got := statusRow(m); got != "locked by refactor the parser, write anyway? (y/n)" {
				t.Fatalf("status bar = %q, want the lock question", got)
			}
			if got := editor.args(t); got != nil {
				t.Fatalf("editor ran with %q before the answer, want it waiting", got)
			}
			m = press(t, m, answer.key)

			if m.prompt.isOpen {
				t.Errorf("prompt still open after %s", answer.name)
			}
			isEdited := !bytes.Equal(readStoreFile(t, ts, alphaPath), before)
			if isEdited != answer.isEdited {
				t.Errorf("alpha edited after %s = %v, want %v", answer.name, isEdited, answer.isEdited)
			}
		})
	}
}

func TestEditorWords_SplitsOnSpaces(t *testing.T) {
	cases := []struct {
		name   string
		visual string
		editor string
		want   []string
	}{
		{name: "visual with a flag", visual: "fake-editor --flag", editor: "vi", want: []string{"fake-editor", "--flag"}},
		{name: "extra spaces", visual: "  code   --wait ", want: []string{"code", "--wait"}},
		{name: "editor when visual is unset", editor: "nano -w", want: []string{"nano", "-w"}},
		{name: "editor when visual is blank", visual: "   ", editor: "vi", want: []string{"vi"}},
		{name: "neither", want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			environment := map[string]string{visualVariable: tc.visual, editorVariable: tc.editor}

			got := editorWords(func(name string) string { return environment[name] })

			if !slices.Equal(got, tc.want) {
				t.Errorf("editorWords = %q, want %q", got, tc.want)
			}
		})
	}
}
