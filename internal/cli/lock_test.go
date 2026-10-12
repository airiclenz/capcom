package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// staleLockConfig makes every Tree lock stale as soon as it is written.
const staleLockConfig = "lock:\n  ttl: 1ns\n"

// lockKVKeys is the --kv record of every single-lock command, in order.
var lockKVKeys = []string{"LOCK", "HOLDER", "PURPOSE", "SINCE", "HEARTBEAT", "TOKEN", "STATUS"}

// newLockStore creates a git repository with a capcom store and returns its root.
func newLockStore(t *testing.T) string {
	t.Helper()
	root := newGitRepo(t)
	mustRun(t, root, "init")
	return root
}

// addWorktree commits once in root, adds a linked worktree at a sibling directory called name
// on a new branch, and gives it a capcom store. It returns the worktree's root.
func addWorktree(t *testing.T, root, name string) string {
	t.Helper()
	worktree := filepath.Join(filepath.Dir(root), name)
	runGit(t, root, "commit", "-q", "--allow-empty", "-m", "base")
	runGit(t, root, "worktree", "add", "-q", "-b", "branch-"+name, worktree)
	mustRun(t, worktree, "init")
	return worktree
}

// runGit runs a git command in dir and fails the test when it fails.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

// acquireToken acquires root's lock for holder and returns the token.
func acquireToken(t *testing.T, root, holder string) string {
	t.Helper()
	stdout := mustRun(t, root, "lock", "acquire", "--purpose", "item 12", "--holder", holder, "--kv")
	return kvValue(t, stdout, "TOKEN")
}

// makeLocksStale rewrites root's config so every lock counts as stale.
func makeLocksStale(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, ".capcom", "config.yaml")
	if err := os.WriteFile(path, []byte(staleLockConfig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// kvKeys returns the keys of output's KEY=VALUE lines, in order.
func kvKeys(output string) []string {
	var keys []string
	for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
		key, _, _ := strings.Cut(line, "=")
		keys = append(keys, key)
	}
	return keys
}

func TestLock_AcquireTouchRelease_Cycle(t *testing.T) {
	root := newLockStore(t)

	acquired := mustRun(t, root, "lock", "acquire", "--purpose", "item 12", "--holder", "agent-a", "--kv")
	token := kvValue(t, acquired, "TOKEN")
	touched := mustRun(t, root, "lock", "touch", "--token", token, "--kv")
	released := mustRun(t, root, "lock", "release", "--token", token, "--kv")
	status := mustRun(t, root, "lock", "status", "--kv")

	if kvValue(t, acquired, "LOCK") != "acquired" || len(token) != 32 {
		t.Errorf("acquire = %q", acquired)
	}
	if kvValue(t, touched, "LOCK") != "held" || kvValue(t, touched, "HOLDER") != "agent-a" {
		t.Errorf("touch = %q", touched)
	}
	if kvValue(t, released, "LOCK") != "released" {
		t.Errorf("release = %q", released)
	}
	if kvValue(t, status, "LOCK") != "free" {
		t.Errorf("status after release = %q", status)
	}
}

func TestLockAcquire_HeldByAnother_FailsLockedWithHolder(t *testing.T) {
	root := newLockStore(t)
	acquireToken(t, root, "agent-a")

	stdout, _, exitCode := runCLI(t, root, "lock", "acquire", "--purpose", "other", "--holder", "agent-b", "--kv")

	if exitCode != ExitNotFound {
		t.Fatalf("exit = %d, want %d\n%s", exitCode, ExitNotFound, stdout)
	}
	if kvValue(t, stdout, "HOLDER") != "agent-a" || kvValue(t, stdout, "PURPOSE") != "item 12" {
		t.Errorf("stdout = %q", stdout)
	}
	if kvValue(t, stdout, "TOKEN") != "" || lastLine(stdout) != "STATUS=locked" {
		t.Errorf("stdout = %q, want no token and STATUS=locked last", stdout)
	}
}

func TestLockAcquire_HeldByAnotherText_NamesHolderPurposeAndAge(t *testing.T) {
	root := newLockStore(t)
	acquireToken(t, root, "agent-a")

	_, stderr, exitCode := runCLI(t, root, "lock", "acquire", "--purpose", "other")

	if exitCode != ExitNotFound {
		t.Fatalf("exit = %d", exitCode)
	}
	if !strings.Contains(stderr, "_main.json: locked by agent-a (item 12) for ") || !strings.Contains(stderr, "heartbeat") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestLockAcquire_OwnTokenOnStaleLock_Resumes(t *testing.T) {
	root := newLockStore(t)
	token := acquireToken(t, root, "agent-a")
	makeLocksStale(t, root)

	stdout := mustRun(t, root, "lock", "acquire", "--purpose", "item 13", "--token", token, "--kv")

	if kvValue(t, stdout, "LOCK") != "resumed" || kvValue(t, stdout, "TOKEN") != token {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestLockAcquire_TakeStaleOnFreshLock_IsRefused(t *testing.T) {
	root := newLockStore(t)
	acquireToken(t, root, "agent-a")

	stdout, _, exitCode := runCLI(t, root, "lock", "acquire", "--purpose", "x", "--take-stale", "--kv")

	if exitCode != ExitNotFound || kvValue(t, stdout, "LOCK") != "held" {
		t.Errorf("exit = %d stdout = %q", exitCode, stdout)
	}
}

func TestLockAcquire_StaleLock_RefusedWithoutTakeStaleAndTakenWithIt(t *testing.T) {
	root := newLockStore(t)
	token := acquireToken(t, root, "agent-a")
	makeLocksStale(t, root)

	refused, _, refusedExit := runCLI(t, root, "lock", "acquire", "--purpose", "x", "--holder", "agent-b", "--kv")
	taken := mustRun(t, root, "lock", "acquire", "--purpose", "x", "--holder", "agent-b", "--take-stale", "--kv")

	if refusedExit != ExitNotFound || kvValue(t, refused, "LOCK") != "stale" {
		t.Errorf("without --take-stale: exit = %d stdout = %q", refusedExit, refused)
	}
	if kvValue(t, taken, "LOCK") != "acquired" || kvValue(t, taken, "TOKEN") == token {
		t.Errorf("with --take-stale: %q", taken)
	}
}

func TestLockBreak_HeldLock_FreesIt(t *testing.T) {
	root := newLockStore(t)
	acquireToken(t, root, "agent-a")

	broken := mustRun(t, root, "lock", "break", "--kv")
	status := mustRun(t, root, "lock", "status", "--kv")

	if kvValue(t, broken, "LOCK") != "released" || kvValue(t, broken, "HOLDER") != "agent-a" {
		t.Errorf("break = %q", broken)
	}
	if kvValue(t, status, "LOCK") != "free" {
		t.Errorf("status = %q", status)
	}
}

func TestLockCheck_ExitCodes(t *testing.T) {
	root := newLockStore(t)
	_, _, freeExit := runCLI(t, root, "lock", "check")
	token := acquireToken(t, root, "agent-a")
	cases := []struct {
		name     string
		args     []string
		wantExit int
	}{
		{"own token", []string{"--token", token}, ExitOK},
		{"other token", []string{"--token", "someone-else"}, ExitNotFound},
		{"no token", nil, ExitNotFound},
	}

	if freeExit != ExitOK {
		t.Errorf("free lock: exit = %d, want 0", freeExit)
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			args := append([]string{"lock", "check"}, testCase.args...)

			stdout, stderr, exitCode := runCLI(t, root, args...)

			if exitCode != testCase.wantExit {
				t.Errorf("exit = %d, want %d\nstdout = %s\nstderr = %s", exitCode, testCase.wantExit, stdout, stderr)
			}
		})
	}
}

func TestLockStatusAll_TwoWorktrees_OneGroupPerRecord(t *testing.T) {
	root := newLockStore(t)
	worktree := addWorktree(t, root, "feature")
	acquireToken(t, root, "agent-a")
	acquireToken(t, worktree, "agent-b")

	stdout := mustRun(t, root, "lock", "status", "--all", "--kv")

	want := "WORKTREE=_main\nLOCK=held\nHOLDER=agent-a\n"
	if !strings.HasPrefix(stdout, want) {
		t.Errorf("stdout = %q, want it to start %q", stdout, want)
	}
	if strings.Count(stdout, "WORKTREE=") != 2 || !strings.Contains(stdout, "WORKTREE=wt-feature\nLOCK=held\nHOLDER=agent-b\n") {
		t.Errorf("stdout = %q, want one group per worktree", stdout)
	}
	if strings.Count(stdout, "STATUS=ok\n") != 2 || lastLine(stdout) != "STATUS=ok" {
		t.Errorf("stdout = %q, want each group closed by STATUS=ok", stdout)
	}
}

func TestLock_LinkedWorktreeNamedMain_GetsItsOwnRecord(t *testing.T) {
	root := newLockStore(t)
	worktree := addWorktree(t, root, "_main")
	acquireToken(t, root, "agent-a")

	linked := mustRun(t, worktree, "lock", "acquire", "--purpose", "linked", "--holder", "agent-b", "--kv")
	all := mustRun(t, root, "lock", "status", "--all", "--kv")

	if kvValue(t, linked, "LOCK") != "acquired" {
		t.Fatalf("linked acquire = %q", linked)
	}
	if !strings.Contains(all, "WORKTREE=_main\n") || !strings.Contains(all, "WORKTREE=wt-_main\n") {
		t.Errorf("status --all = %q", all)
	}
	gitCommonDir := filepath.Join(root, ".git", "capcom", "locks")
	for _, name := range []string{"_main.json", "wt-_main.json"} {
		if _, err := os.Stat(filepath.Join(gitCommonDir, name)); err != nil {
			t.Errorf("record %s: %v", name, err)
		}
	}
}

func TestLockStatusAll_NoLocks_EndsWithStatus(t *testing.T) {
	root := newLockStore(t)

	stdout := mustRun(t, root, "lock", "status", "--all", "--kv")

	if stdout != "STATUS=ok\n" {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestLock_KVShape_EveryCommandEmitsTheLockRecord(t *testing.T) {
	root := newLockStore(t)
	acquired := mustRun(t, root, "lock", "acquire", "--purpose", "item 12", "--kv")
	token := kvValue(t, acquired, "TOKEN")
	outputs := map[string]string{
		"acquire": acquired,
		"touch":   mustRun(t, root, "lock", "touch", "--token", token, "--kv"),
		"status":  mustRun(t, root, "lock", "status", "--kv"),
		"check":   mustRun(t, root, "lock", "check", "--token", token, "--kv"),
		"release": mustRun(t, root, "lock", "release", "--token", token, "--kv"),
		"break":   mustRun(t, root, "lock", "break", "--kv"),
	}

	for command, output := range outputs {
		if got := strings.Join(kvKeys(output), ","); got != strings.Join(lockKVKeys, ",") {
			t.Errorf("%s keys = %s, want %s", command, got, strings.Join(lockKVKeys, ","))
		}
		if lastLine(output) != "STATUS=ok" {
			t.Errorf("%s output = %q, want STATUS=ok last", command, output)
		}
	}
}

func TestLockTouch_FreeLock_FailsNotFoundWithFreeRecord(t *testing.T) {
	root := newLockStore(t)

	stdout, _, exitCode := runCLI(t, root, "lock", "touch", "--token", "any", "--kv")

	if exitCode != ExitNotFound || kvValue(t, stdout, "LOCK") != "free" || lastLine(stdout) != "STATUS=not_found" {
		t.Errorf("exit = %d stdout = %q", exitCode, stdout)
	}
}

func TestLockTouch_FreeLock_JSON_NamesWorktreeAndRecordPath(t *testing.T) {
	root := newLockStore(t)

	stdout, _, exitCode := runCLI(t, root, "lock", "touch", "--token", "any", "--json")

	envelope := decodeEnvelope(t, stdout)
	data, _ := envelope["data"].(map[string]any)
	failure, _ := envelope["error"].(map[string]any)
	if exitCode != ExitNotFound || data["worktree"] != "_main" || data["lock"] != "free" {
		t.Errorf("exit = %d data = %v", exitCode, data)
	}
	path, _ := failure["path"].(string)
	if failure["code"] != string(CodeNotFound) || path == "" || strings.Contains(failure["message"].(string), path) {
		t.Errorf("error = %v, want not_found with the record path in path only", failure)
	}
}

func TestLockAcquire_JSON_CarriesTokenAndBranch(t *testing.T) {
	root := newLockStore(t)

	stdout := mustRun(t, root, "lock", "acquire", "--purpose", "item 12", "--json")

	data := decodeEnvelope(t, stdout)["data"].(map[string]any)
	if data["lock"] != "acquired" || data["worktree"] != "_main" || data["token"] == "" || data["branch"] == "" {
		t.Errorf("data = %v", data)
	}
}

func TestLockAcquire_WithoutPurpose_ExitsUsage(t *testing.T) {
	root := newLockStore(t)

	_, _, exitCode := runCLI(t, root, "lock", "acquire")

	if exitCode != ExitUsage {
		t.Errorf("exit = %d, want %d", exitCode, ExitUsage)
	}
}
