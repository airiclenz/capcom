package hooks_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/hooks"
)

// pluginScript is the Claude Code plugin's hook script, relative to this package's directory.
var pluginScript = filepath.Join("..", "..", "plugins", "capcom", "hooks", "prime.sh")

// fakeCapcomOutput is what the fake capcom binary prints when the plugin script runs it.
const fakeCapcomOutput = "primed"

// TestPluginScript_Settings_PrimesUnlessAPrimeHookIsInstalled checks that the plugin's prime.sh
// stands down only for a settings file holding a real capcom prime hook, not for any other mention
// of capcom prime.
func TestPluginScript_Settings_PrimesUnlessAPrimeHookIsInstalled(t *testing.T) {
	snippet := hooks.Snippet(hooks.Claude)
	var compact bytes.Buffer
	if err := json.Compact(&compact, snippet); err != nil {
		t.Fatalf("Compact(Snippet(Claude)): %v", err)
	}
	tests := []struct {
		name     string
		settings string
		want     string
	}{
		{"no settings file", "", fakeCapcomOutput},
		{
			name:     "permission rule naming capcom prime",
			settings: `{"permissions": {"allow": ["Bash(capcom prime)"]}}`,
			want:     fakeCapcomOutput,
		},
		{
			name:     "hook running another command",
			settings: `{"hooks": {"SessionStart": [{"hooks": [{"command": "capcom prime --json"}]}]}}`,
			want:     fakeCapcomOutput,
		},
		{"indented capcom init --claude-hooks snippet", string(snippet), ""},
		{"compact capcom init --claude-hooks snippet", compact.String(), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newPluginScriptRepo(t, true)
			if tt.settings != "" {
				writePluginScriptSettings(t, root, tt.settings)
			}

			got := runPluginScript(t, root)

			if got != tt.want {
				t.Errorf("prime.sh printed %q, want %q", got, tt.want)
			}
		})
	}
}

// TestPluginScript_NoStore_StaysSilent checks that prime.sh prints nothing in a repository without
// a .capcom/ store.
func TestPluginScript_NoStore_StaysSilent(t *testing.T) {
	root := newPluginScriptRepo(t, false)

	got := runPluginScript(t, root)

	if got != "" {
		t.Errorf("prime.sh printed %q, want nothing", got)
	}
}

// newPluginScriptRepo creates a git repository in a temporary directory, with an empty .capcom/
// directory when withStore is set, and returns its path. It skips the test when sh or git is not
// on PATH, and on Windows, where Git's sh may not run the extensionless fake capcom.
func newPluginScriptRepo(t *testing.T, withStore bool) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the plugin script test does not run on Windows")
	}
	for _, tool := range []string{"sh", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not on PATH", tool)
		}
	}
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "--quiet", root).CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v\n%s", root, err, output)
	}
	if withStore {
		if err := os.Mkdir(filepath.Join(root, ".capcom"), 0o755); err != nil {
			t.Fatalf("Mkdir(.capcom): %v", err)
		}
	}
	return root
}

// writePluginScriptSettings writes settings to the repository's .claude/settings.json.
func writePluginScriptSettings(t *testing.T, root, settings string) {
	t.Helper()
	dir := filepath.Join(root, ".claude")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("Mkdir(%s): %v", dir, err)
	}
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(settings), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

// runPluginScript runs prime.sh with sh for the repository at root, with a fake capcom on PATH
// that prints fakeCapcomOutput and HOME set to an empty directory, and returns its trimmed output.
func runPluginScript(t *testing.T, root string) string {
	t.Helper()
	script, err := filepath.Abs(pluginScript)
	if err != nil {
		t.Fatalf("Abs(%s): %v", pluginScript, err)
	}
	bin := t.TempDir()
	fake := filepath.Join(bin, "capcom")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho "+fakeCapcomOutput+"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(%s): %v", fake, err)
	}
	cmd := exec.Command("sh", script)
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"HOME="+t.TempDir(),
		"CLAUDE_PROJECT_DIR="+root,
	)

	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("sh %s: %v\n%s", script, err, output)
	}
	return strings.TrimSpace(string(output))
}
