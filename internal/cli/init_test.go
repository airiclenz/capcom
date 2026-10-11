package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/store"
)

// newGitRepo creates an empty git repository in a temp dir and returns its symlink-free root.
// It stops git from discovering a repository above the temp dir.
func newGitRepo(t *testing.T) string {
	t.Helper()
	root := outsideGitDir(t)
	setHermeticGitEnv(t)
	command := exec.Command("git", "init", "-q", root)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	return root
}

// outsideGitDir returns a symlink-free temp dir that git treats as outside any repository.
func outsideGitDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	return dir
}

func TestInit_NewRepository_CreatesStore(t *testing.T) {
	root := newGitRepo(t)

	stdout, stderr, exitCode := runCLI(t, root, "init")

	if exitCode != ExitOK || stderr != "" {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	for _, path := range []string{"config.yaml", "issues", "knowledge"} {
		if _, err := os.Stat(filepath.Join(root, ".capcom", path)); err != nil {
			t.Errorf(".capcom/%s missing: %v", path, err)
		}
	}
	if !strings.Contains(stdout, "created .capcom/config.yaml\n") {
		t.Errorf("stdout = %q, want it to list the created config", stdout)
	}
}

func TestInit_ReRun_IsNoOp(t *testing.T) {
	root := newGitRepo(t)
	runCLI(t, root, "init")

	stdout, _, exitCode := runCLI(t, root, "init", "--json")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d\n%s", exitCode, stdout)
	}
	data := decodeEnvelope(t, stdout)["data"].(map[string]any)
	created, isList := data["created"].([]any)
	if !isList || len(created) != 0 {
		t.Errorf("created = %v, want an empty list", data["created"])
	}
	if want := filepath.Join(root, ".capcom"); data["root"] != want {
		t.Errorf("root = %v, want %s", data["root"], want)
	}
}

func TestInit_FromSubdirectory_CreatesStoreAtWorktreeRoot(t *testing.T) {
	root := newGitRepo(t)
	subdirectory := filepath.Join(root, "cmd", "tool")
	if err := os.MkdirAll(subdirectory, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	_, _, exitCode := runCLI(t, subdirectory, "init")

	if _, err := os.Stat(filepath.Join(root, ".capcom", "config.yaml")); exitCode != ExitOK || err != nil {
		t.Errorf("exit = %d, root config: %v", exitCode, err)
	}
}

func TestInit_OutsideGit_FailsInvalid(t *testing.T) {
	outside := outsideGitDir(t)

	stdout, _, exitCode := runCLI(t, outside, "--kv", "init")

	if exitCode != ExitError || lastLine(stdout) != "STATUS="+string(CodeInvalid) {
		t.Errorf("exit = %d stdout = %q, want %d and STATUS=invalid", exitCode, stdout, ExitError)
	}
	if !strings.Contains(stdout, "git repository") {
		t.Errorf("stdout = %q, want a message naming the git requirement", stdout)
	}
}

func TestInit_InvalidExistingConfig_FailsWithPathAndLine(t *testing.T) {
	root := newGitRepo(t)
	runCLI(t, root, "init")
	content := "statuses:\n  open: todo\n  review: reviewing\n  done: done\n"
	if err := os.WriteFile(filepath.Join(root, ".capcom", "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, stderr, exitCode := runCLI(t, root, "init")

	if exitCode != ExitError || !strings.HasPrefix(stderr, "capcom: .capcom/config.yaml:3: ") {
		t.Errorf("exit = %d stderr = %q, want .capcom/config.yaml:3", exitCode, stderr)
	}
}

func TestVersion_OutsideGitBeforeInit_Succeeds(t *testing.T) {
	outside := outsideGitDir(t)

	_, stderr, exitCode := runCLI(t, outside, "version")

	if exitCode != ExitOK || stderr != "" {
		t.Errorf("exit = %d stderr = %q, want version to run outside git", exitCode, stderr)
	}
}

func TestStoreFailure_MapsInitAndOpenErrors(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode Code
		wantLine int
	}{
		{"not initialized", fmt.Errorf("/repo/.capcom: %w", store.ErrNotInitialized), CodeNotInitialized, 0},
		{"outside git", fmt.Errorf("/tmp: %w", store.ErrNotGitRepository), CodeInvalid, 0},
		{"bad config", &config.Error{Path: ".capcom/config.yaml", Line: 5, Message: "bad"}, CodeInvalid, 5},
		{"other", errors.New("disk full"), CodeInvalid, 0},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			failure := storeFailure(testCase.err)

			if failure.Code != testCase.wantCode || failure.Line != testCase.wantLine {
				t.Errorf("failure = %+v, want code %s line %d", failure, testCase.wantCode, testCase.wantLine)
			}
		})
	}
}

func TestStoreFailure_StoreCommandBeforeInit_IsNotInitialized(t *testing.T) {
	root := newGitRepo(t)

	_, err := store.Open(root)

	if failure := storeFailure(err); failure.Code != CodeNotInitialized || failure.ExitCode() != ExitError {
		t.Errorf("failure = %+v, want not_initialized exiting %d", failure, ExitError)
	}
}

func TestInit_Plain_PrintsTheHooksSnippetInTextModeOnly(t *testing.T) {
	root := newGitRepo(t)

	text, _, textExit := runCLI(t, root, "init")
	jsonOut, _, jsonExit := runCLI(t, root, "init", "--json")

	if textExit != ExitOK || !strings.Contains(text, claudeHooksSnippet()) ||
		!strings.Contains(text, `"matcher": "startup|resume|clear|compact"`) {
		t.Errorf("init = %d, stdout:\n%s\nwant the hooks snippet", textExit, text)
	}
	if jsonExit != ExitOK || strings.Contains(jsonOut, primeCommand) {
		t.Errorf("init --json = %d, stdout %s; want no snippet", jsonExit, jsonOut)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude")); !os.IsNotExist(err) {
		t.Errorf("plain init created .claude: %v", err)
	}
}

// settingsBefore is a Claude Code settings file in two-space layout whose keys are not in
// alphabetical order, with an existing SessionStart hook, shell operators and numbers.
const settingsBefore = `{
  "permissions": {
    "allow": [
      "Bash(make test && make lint)",
      "Bash(sort < in > out)"
    ]
  },
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "gofmt -l . && echo '<ok>'",
            "timeout": 30
          }
        ]
      }
    ],
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "echo hi > /dev/null"
          }
        ]
      }
    ]
  },
  "cleanupPeriodDays": 1.50,
  "env": {
    "BIG": 12345678901234567890
  }
}
`

// settingsAfter is settingsBefore once init --claude-hooks merged the capcom prime hooks.
const settingsAfter = `{
  "permissions": {
    "allow": [
      "Bash(make test && make lint)",
      "Bash(sort < in > out)"
    ]
  },
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "gofmt -l . && echo '<ok>'",
            "timeout": 30
          }
        ]
      }
    ],
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "echo hi > /dev/null"
          }
        ]
      },
      {
        "matcher": "startup|resume|clear|compact",
        "hooks": [
          {
            "type": "command",
            "command": "capcom prime"
          }
        ]
      }
    ],
    "PreCompact": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "capcom prime"
          }
        ]
      }
    ]
  },
  "cleanupPeriodDays": 1.50,
  "env": {
    "BIG": 12345678901234567890
  }
}
`

// writeClaudeSettings writes content as the .claude/settings.json of root.
func writeClaudeSettings(t *testing.T, root string, content string) string {
	t.Helper()
	path := filepath.Join(root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestInitClaudeHooks_ExistingSettings_ChangesOnlyHooksAndIsIdempotent(t *testing.T) {
	root := newGitRepo(t)
	path := writeClaudeSettings(t, root, settingsBefore)

	first := mustRun(t, root, "init", "--claude-hooks")
	merged, err := os.ReadFile(path)
	second := mustRun(t, root, "init", "--claude-hooks")
	again, againErr := os.ReadFile(path)

	if err != nil || string(merged) != settingsAfter {
		t.Fatalf("settings.json = %v\n%s\nwant\n%s", err, merged, settingsAfter)
	}
	if againErr != nil || string(again) != settingsAfter {
		t.Errorf("a second init --claude-hooks changed settings.json:\n%s", again)
	}
	if !strings.HasSuffix(first, "added the capcom prime hooks to .claude/settings.json\n") ||
		!strings.HasSuffix(second, ".claude/settings.json already runs capcom prime\n") {
		t.Errorf("init --claude-hooks output = %q then %q", first, second)
	}
}

func TestInitClaudeHooks_NoSettings_CreatesThem(t *testing.T) {
	root := newGitRepo(t)

	stdout := mustRun(t, root, "init", "--claude-hooks", "--json")

	content, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	wantSnippet := claudeHooksSnippet()
	if err != nil || !strings.HasSuffix(wantSnippet, string(content)) {
		t.Errorf("settings.json = %v\n%s\nwant the snippet's settings", err, content)
	}
	if data := decodeEnvelope(t, stdout)["data"].(map[string]any); data["claude_hooks"] != claudeHooksAdded {
		t.Errorf("claude_hooks = %v, want %s", data["claude_hooks"], claudeHooksAdded)
	}
}

func TestInitClaudeHooks_UnusableSettings_FailWithPath(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		wantPrefix string
	}{
		{"syntax error", "{\n  \"a\": 1,\n  oops\n}\n", "capcom: .claude/settings.json:3: "},
		{"not an object", "[1, 2]\n", "capcom: .claude/settings.json: "},
		{"hooks not an object", "{\"hooks\": [1]}\n", "capcom: .claude/settings.json: hooks "},
		{"event not an array", "{\"hooks\": {\"PreCompact\": {}}}\n", "capcom: .claude/settings.json: hooks.PreCompact "},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newGitRepo(t)
			path := writeClaudeSettings(t, root, testCase.content)

			_, stderr, exitCode := runCLI(t, root, "init", "--claude-hooks")

			content, _ := os.ReadFile(path)
			if exitCode != ExitError || !strings.HasPrefix(stderr, testCase.wantPrefix) || string(content) != testCase.content {
				t.Errorf("init --claude-hooks = %d, stderr %q, file %q; want 1, %q and the file untouched",
					exitCode, stderr, content, testCase.wantPrefix)
			}
		})
	}
}
