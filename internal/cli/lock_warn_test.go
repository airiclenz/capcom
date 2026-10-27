package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// lockWarnAdvice ends the warning of a write made while another session holds the Tree lock.
const lockWarnAdvice = "; if it is yours, carry on"

// newLockWarnStore creates a store whose Issues and Note every write command of the lock warning
// tests can act on: login-flow and other open, shipped-fix done, dependent depending on other,
// linked related to other, and the Note guide.
func newLockWarnStore(t *testing.T) string {
	t.Helper()
	root := newIssueStore(t, map[string]string{
		"login-flow.md":  readIssue("Login flow", "open", "", "\nThe flow.\n"),
		"other.md":       readIssue("Other", "open", "", ""),
		"shipped-fix.md": readIssue("Shipped fix", "done", "", ""),
		"dependent.md":   readIssue("Dependent", "open", "deps: [other]\n", ""),
		"linked.md":      readIssue("Linked", "open", "related: [other]\n", ""),
	})
	writeNoteFiles(t, root, map[string]string{
		"guide.md": "---\ntitle: Guide\n---\n\nRead me.\n",
	})
	return root
}

// readRootFile returns the file at a slash path below root, and false when there is none or name
// is empty.
func readRootFile(t *testing.T, root string, name string) (string, bool) {
	t.Helper()
	if name == "" {
		return "", false
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if os.IsNotExist(err) {
		return "", false
	}
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(data), true
}

// lockWarning returns the warning line of stdout that ends in lockWarnAdvice, or "" when there is
// none.
func lockWarning(stdout string) string {
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "warning: ") && strings.HasSuffix(line, lockWarnAdvice) {
			return line
		}
	}
	return ""
}

func TestLockWarn_WriteUnderAForeignLock_WarnsAndWrites(t *testing.T) {
	tests := []struct {
		name string
		args []string
		// path is the file the write creates or changes, a slash path below the root; isGone says
		// the write deletes it instead; empty checks no file.
		path   string
		isGone bool
	}{
		{"create", []string{"create", "--id", "fresh", "--title", "Fresh"}, ".capcom/issues/fresh.md", false},
		{"update", []string{"update", "login-flow", "--priority", "1"}, ".capcom/issues/login-flow.md", false},
		{"log", []string{"log", "login-flow", "progress"}, ".capcom/issues/login-flow.md", false},
		{"close", []string{"close", "login-flow", "--reason", "done"}, ".capcom/issues/login-flow.md", false},
		{"reopen", []string{"reopen", "shipped-fix", "--reason", "again"}, ".capcom/issues/shipped-fix.md", false},
		{"dep add", []string{"dep", "add", "login-flow", "other"}, ".capcom/issues/login-flow.md", false},
		{"dep rm", []string{"dep", "rm", "dependent", "other"}, ".capcom/issues/dependent.md", false},
		{"related add", []string{"related", "add", "login-flow", "other"}, ".capcom/issues/login-flow.md", false},
		{"related rm", []string{"related", "rm", "linked", "other"}, ".capcom/issues/linked.md", false},
		{"mv", []string{"mv", "login-flow", "login-moved"}, ".capcom/issues/login-moved.md", false},
		{"rm", []string{"rm", "shipped-fix"}, ".capcom/issues/shipped-fix.md", true},
		{"know add", []string{"know", "add", "--id", "fresh-note", "--title", "Fresh note"}, ".capcom/knowledge/fresh-note.md", false},
		{"know update", []string{"know", "update", "guide", "--title", "New guide"}, ".capcom/knowledge/guide.md", false},
		{"know mv", []string{"know", "mv", "guide", "guide-moved"}, ".capcom/knowledge/guide-moved.md", false},
		{"know rm", []string{"know", "rm", "guide"}, ".capcom/knowledge/guide.md", true},
		{"fmt", []string{"fmt"}, "", false},
		{"agents", []string{"agents"}, "AGENTS.md", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newLockWarnStore(t)
			acquireToken(t, root, "other-session")
			before, _ := readRootFile(t, root, test.path)

			stdout, stderr, exitCode := runCLI(t, root, test.args...)

			if exitCode != ExitOK {
				t.Fatalf("exit = %d, want 0\nstdout = %s\nstderr = %s", exitCode, stdout, stderr)
			}
			warning := lockWarning(stdout)
			if !strings.Contains(warning, "_main.json: locked by other-session (item 12) for ") ||
				!strings.Contains(warning, ", heartbeat ") {
				t.Errorf("stdout = %q, want the lock warning naming holder, purpose and heartbeat", stdout)
			}
			if strings.Count(stdout, lockWarnAdvice) != 1 {
				t.Errorf("stdout = %q, want exactly one lock warning", stdout)
			}
			if test.path == "" {
				return
			}
			after, isPresent := readRootFile(t, root, test.path)
			switch {
			case test.isGone && isPresent:
				t.Errorf("%s still exists", test.path)
			case !test.isGone && (!isPresent || after == before):
				t.Errorf("%s was not written", test.path)
			}
		})
	}
}

func TestLockWarn_WriteOfThePurposeIssue_IsSilent(t *testing.T) {
	tests := []struct {
		name string
		take []string
		args []string
	}{
		{"log after start", []string{"start", "login-flow"}, []string{"log", "login-flow", "progress"}},
		{"log after acquire", []string{"lock", "acquire", "--purpose", "login-flow"}, []string{"log", "login-flow", "progress"}},
		{"mv of the purpose", []string{"lock", "acquire", "--purpose", "login-flow"}, []string{"mv", "login-flow", "login-moved"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newLockWarnStore(t)
			mustRun(t, root, test.take...)

			stdout := mustRun(t, root, test.args...)

			if strings.Contains(stdout, lockWarnAdvice) {
				t.Errorf("stdout = %q, want no lock warning", stdout)
			}
		})
	}
}

func TestLockWarn_LockVerbsAndFreeLock_AreSilent(t *testing.T) {
	tests := []struct {
		name string
		take func(t *testing.T, root string) []string
	}{
		{"start", func(_ *testing.T, _ string) []string {
			return []string{"start", "login-flow"}
		}},
		{"finish under a foreign lock", func(t *testing.T, root string) []string {
			acquireToken(t, root, "other-session")
			return []string{"finish", "login-flow", "--reason", "done"}
		}},
		{"finish keep-lock under another purpose", func(t *testing.T, root string) []string {
			token := kvValue(t, mustRun(t, root, "lock", "acquire", "--purpose", "batch", "--kv"), "TOKEN")
			return []string{"finish", "login-flow", "--reason", "done", "--keep-lock", "--token", token}
		}},
		{"free lock", func(_ *testing.T, _ string) []string {
			return []string{"update", "login-flow", "--priority", "1"}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newLockWarnStore(t)
			args := test.take(t, root)

			stdout := mustRun(t, root, args...)

			if strings.Contains(stdout, lockWarnAdvice) {
				t.Errorf("stdout = %q, want no lock warning", stdout)
			}
		})
	}
}

func TestLockWarn_NoteWhoseIDIsThePurpose_Warns(t *testing.T) {
	root := newLockWarnStore(t)
	mustRun(t, root, "lock", "acquire", "--purpose", "guide")

	stdout := mustRun(t, root, "know", "update", "guide", "--title", "New guide")

	if !strings.Contains(lockWarning(stdout), "(guide)") {
		t.Errorf("stdout = %q, want the lock warning for a Note write", stdout)
	}
}

func TestLockWarn_StaleLock_SaysItIsStale(t *testing.T) {
	root := newLockWarnStore(t)
	acquireToken(t, root, "other-session")
	makeLocksStale(t, root)

	stdout := mustRun(t, root, "update", "login-flow", "--priority", "1")

	if !strings.HasSuffix(lockWarning(stdout), ", stale"+lockWarnAdvice) {
		t.Errorf("stdout = %q, want the lock warning marked stale", stdout)
	}
}

func TestLockWarn_InvalidRecord_WarnsNamingLockStatus(t *testing.T) {
	root := newLockWarnStore(t)
	writeLockRecord(t, root, "_main", "{")

	stdout, stderr, exitCode := runCLI(t, root, "update", "login-flow", "--priority", "1")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d, want 0\nstdout = %s\nstderr = %s", exitCode, stdout, stderr)
	}
	want := "_main.json: the lock record cannot be read: invalid lock record: "
	if !strings.Contains(stdout, want) || !strings.Contains(stdout, "; see capcom lock status\n") {
		t.Errorf("stdout = %q, want the unreadable lock warning naming capcom lock status", stdout)
	}
	if !strings.Contains(readStoreFile(t, root, "login-flow.md"), "priority: 1") {
		t.Error("login-flow.md was not written")
	}
}

func TestLockWarn_JSONAndKV_CarryTheWarning(t *testing.T) {
	root := newLockWarnStore(t)
	acquireToken(t, root, "other-session")

	envelope := decodeEnvelope(t, mustRun(t, root, "update", "login-flow", "--priority", "1", "--json"))
	kv := mustRun(t, root, "update", "login-flow", "--priority", "2", "--kv")

	warnings, _ := envelope["warnings"].([]any)
	if len(warnings) != 1 || !strings.HasSuffix(warnings[0].(string), lockWarnAdvice) {
		t.Errorf("warnings = %v, want the lock warning", envelope["warnings"])
	}
	if !strings.Contains(kv, "WARNING=") || !strings.Contains(kv, lockWarnAdvice+"\n") {
		t.Errorf("kv = %q, want a WARNING= line with the lock warning", kv)
	}
}

func TestLockWarn_FreshRepo_CreatesNoLockFile(t *testing.T) {
	root := newLockWarnStore(t)

	mustRun(t, root, "update", "login-flow", "--priority", "1")

	if _, err := os.Stat(filepath.Join(root, ".git", "capcom", "locks")); !os.IsNotExist(err) {
		t.Errorf("Stat(locks dir) err = %v, want not exist", err)
	}
}
