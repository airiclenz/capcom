package ui

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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
// them. The index is closed when the test ends. Tests using it never run in parallel: it sets
// the process environment.
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

// newTestModel returns a model over the test store on the Ready tab, sized width by height by a
// tea.WindowSizeMsg as a terminal would send it.
func newTestModel(t *testing.T, ts *testStore, width, height int) model {
	t.Helper()
	m := newModel(newSession(ts.store, ts.index), TabReady, "")
	return send(t, m, tea.WindowSizeMsg{Width: width, Height: height})
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
	got := ansi.Strip(view.Content)
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
