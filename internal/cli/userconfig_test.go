package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// userConfigEnvironment lists every variable os.UserConfigDir reads on Linux, macOS and Windows.
var userConfigEnvironment = []string{"XDG_CONFIG_HOME", "HOME", "AppData"}

func TestUserConfig_EveryOtherCommand_LeavesUserConfigDirUntouched(t *testing.T) {
	root := newGitRepo(t)
	homes := make(map[string]string, len(userConfigEnvironment))
	for _, name := range userConfigEnvironment {
		homes[name] = t.TempDir()
		t.Setenv(name, homes[name])
	}

	mustRun(t, root, "init")
	mustRun(t, root, "create", "--id", "login-flow", "--title", "Login flow")
	mustRun(t, root, "update", "login-flow", "--status", "active")
	mustRun(t, root, "close", "login-flow", "--reason", "shipped")
	mustRun(t, root, "list")
	mustRun(t, root, "ready")
	mustRun(t, root, "show", "login-flow")
	mustRun(t, root, "know", "tree")
	mustRun(t, root, "check")

	for name, directory := range homes {
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Fatalf("ReadDir(%s): %v", directory, err)
		}
		for _, entry := range entries {
			t.Errorf("$%s holds %s after the commands, want it empty", name, entry.Name())
		}
	}
	worktreeStatePath := filepath.Join(root, ".git", "capcom", "ui-state.yaml")
	if _, err := os.Stat(worktreeStatePath); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(%s) = %v after the commands, want it not to exist", worktreeStatePath, err)
	}
}
