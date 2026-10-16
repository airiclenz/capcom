package ui

import (
	"flag"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/store"
)

// isGoldenUpdate rewrites the golden files from the views the tests draw: go test ./internal/ui/
// -update.
var isGoldenUpdate = flag.Bool("update", false, "rewrite the golden files under testdata/golden")

// goldenDir holds the expected views, one <name>.golden file per golden call.
const goldenDir = "testdata/golden"

// testStore is a temporary store opened the way capcom ui gets it: the store and its index,
// with the working tree root the store lives in.
type testStore struct {
	root  string
	store *store.Store
	index *index.Index
}

// newTestStore builds a store in a fresh temp git repository, as runCLI's tests do: the hermetic
// git environment, the config.yaml capcom init writes, then files, keyed by their path relative
// to .capcom with forward slashes (issues/a.md, knowledge/n.md), and an index refreshed over
// them. Every store file is backdated by backdateAge before the index opens, so the index records
// them clean and an edit a test makes later is a change, never racily clean. The index is closed
// when the test ends. Tests using it never run in parallel: it sets the process environment.
func newTestStore(t *testing.T, files map[string]string) *testStore {
	t.Helper()
	root := newGitRepo(t)
	initialized, _, err := store.Init(root)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	for name, content := range files {
		path := filepath.Join(initialized.Root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}
	backdateTree(t, initialized.Root)
	opened, err := store.Open(root)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	issueIndex, err := index.Open(opened, index.ModeRefresh)
	if err != nil {
		t.Fatalf("index.Open: %v", err)
	}
	t.Cleanup(func() {
		if err := issueIndex.Close(); err != nil {
			t.Errorf("close index: %v", err)
		}
	})
	return &testStore{root: root, store: opened, index: issueIndex}
}

// backdateAge is how far newTestStore backdates the store's files.
const backdateAge = time.Hour

// backdateTree sets the modification time of every file and directory under root backdateAge in
// the past.
func backdateTree(t *testing.T, root string) {
	t.Helper()
	then := time.Now().Add(-backdateAge)
	err := filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(path, then, then)
	})
	if err != nil {
		t.Fatalf("backdate %s: %v", root, err)
	}
}

// newGitRepo creates an empty git repository in a symlink-free temp dir that git treats as
// outside any other repository, with git isolated from user and system configuration and a fixed
// author and committer.
func newGitRepo(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	environment := map[string]string{
		"GIT_CEILING_DIRECTORIES": filepath.Dir(root),
		"GIT_CONFIG_GLOBAL":       os.DevNull,
		"GIT_CONFIG_NOSYSTEM":     "1",
		"GIT_AUTHOR_NAME":         "Test Author",
		"GIT_AUTHOR_EMAIL":        "author@example.com",
		"GIT_COMMITTER_NAME":      "Test Committer",
		"GIT_COMMITTER_EMAIL":     "committer@example.com",
	}
	for key, value := range environment {
		t.Setenv(key, value)
	}
	if output, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	return root
}

// testVersion is the release version the test options hand the UI: a fixed fixture, never the
// VERSION file's, so the goldens' tab bar stays the same across releases.
const testVersion = "1.2.3"

// newTestOptions returns the Options capcom ui would pass for the test store, on the Ready tab,
// with the user config file and the state file at paths in a fresh temp dir, neither yet
// created, so no test reads or writes the real user config directory.
func newTestOptions(t *testing.T, ts *testStore) Options {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "capcom")
	return Options{
		Store:          ts.store,
		Index:          ts.index,
		Tab:            TabReady,
		UserConfigPath: filepath.Join(directory, "config.yaml"),
		UserStatePath:  filepath.Join(directory, "state.yaml"),
		Version:        testVersion,
	}
}

// newTestModel returns a started model over the test store on the Ready tab, sized width by
// height: see startModel.
func newTestModel(t *testing.T, ts *testStore, width, height int) model {
	t.Helper()
	return startModel(t, newTestOptions(t, ts), width, height)
}

// startModel returns the model of options after its start as a program runs it: Init's commands
// run and their messages handed back, then a tea.WindowSizeMsg of width by height as a terminal
// would send it. No real clock runs: the model schedules no tick, and a test polls the index by
// sending tickMsg itself.
func startModel(t *testing.T, options Options, width, height int) model {
	t.Helper()
	m := newModel(options)
	m.tick = noTick
	m = drive(t, m, m.Init())
	return press(t, m, tea.WindowSizeMsg{Width: width, Height: height})
}

// noTick stands in for scheduleTick in tests: it schedules nothing.
func noTick() tea.Cmd {
	return nil
}

// press hands msg to the model's Update, then drives the command it returns, and returns the
// model after all of it.
func press(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	updated, cmd := m.Update(msg)
	next, isModel := updated.(model)
	if !isModel {
		t.Fatalf("Update returned %T, want model", updated)
	}
	return drive(t, next, cmd)
}

// drive runs cmd, as a program would, and hands its message back to the model, driving the
// commands that returns in turn; a tea.Batch's commands run one after another. A nil command, and
// tea.Quit's message, end the run.
func drive(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	if cmd == nil {
		return m
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, batched := range msg {
			m = drive(t, m, batched)
		}
		return m
	case tea.QuitMsg:
		return m
	default:
		return press(t, m, msg)
	}
}

// statusRow returns the last row of the model's view, its ANSI styling stripped: the status bar.
func statusRow(m model) string {
	rows := strings.Split(ansi.Strip(m.View().Content), "\n")
	return rows[len(rows)-1]
}

// specialKey is the press of a key without text, such as tea.KeyTab or tea.KeyPgDown.
func specialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

// send hands msg to the model's Update and returns the updated model, discarding the command.
func send(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	updated, _ := m.Update(msg)
	next, isModel := updated.(model)
	if !isModel {
		t.Fatalf("Update returned %T, want model", updated)
	}
	return next
}

// runeKey is the press of a printable key without modifiers, such as q.
func runeKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// ctrlKey is the press of r with ctrl held, such as ctrl+c.
func ctrlKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl}
}

// golden compares view, its ANSI styling stripped, with testdata/golden/<name>.golden; with
// -update it rewrites that file instead.
func golden(t *testing.T, name string, view tea.View) {
	t.Helper()
	compareGolden(t, name, ansi.Strip(view.Content))
}

// goldenANSI compares view, its ANSI styling kept, with testdata/golden/<name>.golden; with
// -update it rewrites that file instead.
func goldenANSI(t *testing.T, name string, view tea.View) {
	t.Helper()
	compareGolden(t, name, view.Content)
}

// compareGolden compares got with testdata/golden/<name>.golden; with -update it rewrites that
// file instead.
func compareGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(goldenDir, name+".golden")
	if *isGoldenUpdate {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", goldenDir, err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s (run go test ./internal/ui/ -update to create it): %v", path, err)
	}
	if got != string(want) {
		t.Errorf("%s: view differs from the golden file\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}
