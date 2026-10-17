package userconfig_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/userconfig"
)

// worktreeStateHeader is the first line SaveWorktreeState writes.
const worktreeStateHeader = "# capcom ui worktree state: written by capcom ui, not meant for editing (ADR 0015).\n"

// worktreeStatePath returns a worktree state file path inside a fresh temp dir whose capcom
// directory does not exist yet.
func worktreeStatePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "capcom", "ui-state.yaml")
}

// writeWorktreeState writes content as the worktree state file at a fresh path and returns that
// path.
func writeWorktreeState(t *testing.T, content string) string {
	t.Helper()
	path := worktreeStatePath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestSaveWorktreeState_LoadWorktreeState_RoundTrips(t *testing.T) {
	tests := []struct {
		name    string
		state   userconfig.WorktreeState
		content string
	}{
		{"nothing stored", userconfig.WorktreeState{}, worktreeStateHeader},
		{
			"every field",
			userconfig.WorktreeState{Tab: "knowledge", Ready: "fix-x", Issues: "epic-y", Knowledge: "adr-0015"},
			worktreeStateHeader + "ui:\n  tab: \"knowledge\"\n  selected:\n" +
				"    ready: \"fix-x\"\n    issues: \"epic-y\"\n    knowledge: \"adr-0015\"\n",
		},
		{
			"only a tab",
			userconfig.WorktreeState{Tab: "issues"},
			worktreeStateHeader + "ui:\n  tab: \"issues\"\n",
		},
		{
			"only a selection",
			userconfig.WorktreeState{Issues: "fix-x"},
			worktreeStateHeader + "ui:\n  selected:\n    issues: \"fix-x\"\n",
		},
		{
			"ids YAML would read as other types",
			userconfig.WorktreeState{Ready: "2026", Issues: "null", Knowledge: "true"},
			worktreeStateHeader + "ui:\n  selected:\n" +
				"    ready: \"2026\"\n    issues: \"null\"\n    knowledge: \"true\"\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := worktreeStatePath(t)

			saveErr := userconfig.SaveWorktreeState(path, test.state)
			loaded, loadErr := userconfig.LoadWorktreeState(path)

			if saveErr != nil || loadErr != nil {
				t.Fatalf("SaveWorktreeState() error = %v, LoadWorktreeState() error = %v", saveErr, loadErr)
			}
			if loaded != test.state {
				t.Errorf("LoadWorktreeState() = %#v, want %#v", loaded, test.state)
			}
			if got := readFile(t, path); got != test.content {
				t.Errorf("worktree state file = %q, want %q", got, test.content)
			}
		})
	}
}

func TestSaveWorktreeState_ExistingFile_ReplacesItAndLeavesNoTempFile(t *testing.T) {
	path := writeWorktreeState(t, "ui:\n  tab: \"ready\"\n")
	want := userconfig.WorktreeState{Tab: "issues", Issues: "fix-x"}

	err := userconfig.SaveWorktreeState(path, want)

	if err != nil {
		t.Fatalf("SaveWorktreeState() error = %v", err)
	}
	if got, err := userconfig.LoadWorktreeState(path); err != nil || got != want {
		t.Errorf("LoadWorktreeState() = %#v, %v; want %#v", got, err, want)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "ui-state.yaml" {
		t.Errorf("directory holds %v, want only ui-state.yaml", entries)
	}
}

func TestSaveWorktreeState_MissingDirectory_CreatesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent", "capcom", "ui-state.yaml")

	err := userconfig.SaveWorktreeState(path, userconfig.WorktreeState{Tab: "ready"})

	if err != nil {
		t.Fatalf("SaveWorktreeState() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Stat(%s) error = %v, want the file", path, err)
	}
}

func TestSaveWorktreeState_ParentIsAFile_Errors(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "capcom")
	if err := os.WriteFile(parent, nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := userconfig.SaveWorktreeState(
		filepath.Join(parent, "ui-state.yaml"),
		userconfig.WorktreeState{Tab: "ready"},
	)

	if err == nil || !strings.HasPrefix(err.Error(), "save the worktree state file: ") {
		t.Errorf("SaveWorktreeState() error = %v, want a wrapped save error", err)
	}
}

func TestLoadWorktreeState_MissingFile_ZeroAndNothingCreated(t *testing.T) {
	path := worktreeStatePath(t)

	state, err := userconfig.LoadWorktreeState(path)

	if err != nil || state != (userconfig.WorktreeState{}) {
		t.Fatalf("LoadWorktreeState() = %#v, %v; want the zero WorktreeState and no error", state, err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(%s) error = %v, want nothing created", filepath.Dir(path), err)
	}
}

func TestLoadWorktreeState_UnknownKeys_IgnoredAndTheRestKept(t *testing.T) {
	path := writeWorktreeState(
		t,
		"future: 1\nui:\n  tab: \"issues\"\n  filter: [a]\n  selected:\n    issues: \"fix-x\"\n    graph: 3\n",
	)

	state, err := userconfig.LoadWorktreeState(path)

	if want := (userconfig.WorktreeState{Tab: "issues", Issues: "fix-x"}); err != nil || state != want {
		t.Errorf("LoadWorktreeState() = %#v, %v; want %#v and no error", state, err, want)
	}
}

func TestLoadWorktreeState_UnknownTabName_ReturnedAsWritten(t *testing.T) {
	path := writeWorktreeState(t, "ui:\n  tab: future-tab\n")

	state, err := userconfig.LoadWorktreeState(path)

	if want := (userconfig.WorktreeState{Tab: "future-tab"}); err != nil || state != want {
		t.Errorf("LoadWorktreeState() = %#v, %v; want %#v and no error", state, err, want)
	}
}

func TestLoadWorktreeState_InvalidFile_ErrorsWithLineOnZero(t *testing.T) {
	tests := []struct {
		name    string
		content string
		line    int
		message string
	}{
		{"malformed YAML", "ui:\n  tab: \"ready\"\n  other: [\n", 3, "invalid YAML"},
		{"tab a list", "ui:\n  tab: [a]\n", 2, "ui.tab: expected a quoted string"},
		{"tab a number", "ui:\n  tab: 3\n", 2, "ui.tab: expected a quoted string"},
		{"selected not a mapping", "ui:\n  selected: 3\n", 2, "ui.selected: expected a mapping"},
		{
			"selection an unquoted number",
			"ui:\n  selected:\n    ready: 2026\n",
			3,
			"ui.selected.ready: expected a quoted string",
		},
		{
			"selection a boolean",
			"ui:\n  selected:\n    knowledge: true\n",
			3,
			"ui.selected.knowledge: expected a quoted string",
		},
		{"ui of the wrong type", "ui: 36\n", 1, "ui: expected a mapping"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeWorktreeState(t, worktreeStateHeader+test.content)

			state, err := userconfig.LoadWorktreeState(path)

			var failure *userconfig.Error
			if !errors.As(err, &failure) {
				t.Fatalf("LoadWorktreeState() error = %v, want *userconfig.Error", err)
			}
			prefix := path + ":" + strconv.Itoa(test.line+1) + ": "
			if !strings.HasPrefix(err.Error(), prefix) || !strings.Contains(err.Error(), test.message) {
				t.Errorf("LoadWorktreeState() error = %q, want prefix %q containing %q", err, prefix, test.message)
			}
			if state != (userconfig.WorktreeState{}) {
				t.Errorf("LoadWorktreeState() = %#v, want the zero WorktreeState", state)
			}
		})
	}
}
