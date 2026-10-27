package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
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

// recreateWorktree drops the git dir of the linked worktree at worktree and adds a worktree of the
// same name on its branch at a fresh path, with a capcom store, returning that path. It leaves the
// old directory in place: t.Chdir keeps it open until the test ends, and Windows cannot delete an
// open directory.
func recreateWorktree(t *testing.T, root, worktree string) string {
	t.Helper()
	name := filepath.Base(worktree)
	if err := os.RemoveAll(filepath.Join(root, ".git", "worktrees", name)); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	recreated := filepath.Join(filepath.Dir(root), "recreated", name)
	runGit(t, root, "worktree", "add", "-q", recreated, "branch-"+name)
	mustRun(t, recreated, "init")
	return recreated
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

// lockHeartbeatYear dates a lock record's heartbeat far in the future, as a fast clock would.
const lockHeartbeatYear = 2099

// futureLockAge is the negative age that dates a lock record at the start of lockHeartbeatYear.
func futureLockAge() time.Duration {
	return -time.Until(time.Date(lockHeartbeatYear, time.January, 1, 0, 0, 0, 0, time.UTC))
}

func TestLockAcquire_FutureHeartbeat_IsTakenWithTakeStale(t *testing.T) {
	root := newLockStore(t)
	token := acquireToken(t, root, "agent-a")
	ageLockRecord(t, lockRecordPath(root), futureLockAge())

	taken := mustRun(t, root, "lock", "acquire", "--purpose", "me", "--take-stale", "--kv")

	if kvValue(t, taken, "LOCK") != "acquired" || kvValue(t, taken, "TOKEN") == token {
		t.Errorf("with --take-stale: %q", taken)
	}
}

func TestLockAcquire_FutureHeartbeatWithoutTakeStale_PrintsNoNegativeAge(t *testing.T) {
	root := newLockStore(t)
	acquireToken(t, root, "agent-a")
	ageLockRecord(t, lockRecordPath(root), futureLockAge())

	_, stderr, exitCode := runCLI(t, root, "lock", "acquire", "--purpose", "me")

	if exitCode != ExitNotFound || !strings.Contains(stderr, "locked by agent-a") {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	if regexp.MustCompile(` -[0-9]`).MatchString(stderr) || !strings.Contains(stderr, "for 0s, heartbeat 0s ago, stale") {
		t.Errorf("stderr = %q, want zero ages and no negative duration", stderr)
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

// writeLockRecord replaces root's lock record of key with data.
func writeLockRecord(t *testing.T, root, key, data string) {
	t.Helper()
	dir := filepath.Join(root, ".git", "capcom", "locks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, key+".json"), []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestLockBreak_CorruptRecord_DiscardsItWithANotice(t *testing.T) {
	root := newLockStore(t)
	writeLockRecord(t, root, "_main", "{")

	stdout, stderr, exitCode := runCLI(t, root, "lock", "break")
	status := mustRun(t, root, "lock", "status", "--kv")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d, want 0\nstdout = %s\nstderr = %s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "_main: free\n") || !strings.Contains(stdout, "_main.json: invalid lock record; discarded") {
		t.Errorf("stdout = %q, want the free lock and the discard notice", stdout)
	}
	if kvValue(t, status, "LOCK") != "free" {
		t.Errorf("status = %q", status)
	}
}

func TestLockStatusAll_CorruptRecord_ListsTheRestWithAWarning(t *testing.T) {
	root := newLockStore(t)
	worktree := addWorktree(t, root, "feature")
	acquireToken(t, worktree, "agent-b")
	writeLockRecord(t, root, "_main", "{}")

	stdout := mustRun(t, root, "lock", "status", "--all", "--kv")

	if !strings.Contains(stdout, "_main.json: invalid lock record: no token") {
		t.Errorf("stdout = %q, want a warning naming the corrupt record", stdout)
	}
	if strings.Count(stdout, "WORKTREE=") != 1 || !strings.Contains(stdout, "WORKTREE=wt-feature\nLOCK=held\nHOLDER=agent-b\n") {
		t.Errorf("stdout = %q, want the wt-feature lock alone", stdout)
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

func TestLockCheck_CorruptRecord_FailsWithExitOne(t *testing.T) {
	root := newLockStore(t)
	writeLockRecord(t, root, "_main", "{")

	stdout, stderr, exitCode := runCLI(t, root, "lock", "check")

	if exitCode != ExitError || !strings.Contains(stderr, "_main.json: the lock record cannot be read: invalid lock record") {
		t.Errorf("exit = %d, want %d\nstdout = %s\nstderr = %s", exitCode, ExitError, stdout, stderr)
	}
}

func TestLockCommands_CorruptRecord_FailInvalidWithTheBreakHint(t *testing.T) {
	const wantMessage = "the lock record cannot be read"
	records := []struct {
		name string
		data string
	}{
		{"unparsable", "not json{{{"},
		{"token-less", "{}"},
	}
	commands := [][]string{
		{"lock", "check"},
		{"lock", "acquire", "--purpose", "item 6"},
		{"lock", "touch", "--token", "any"},
		{"lock", "release", "--token", "any"},
		{"start", startIssue},
	}
	for _, record := range records {
		t.Run(record.name, func(t *testing.T) {
			root := newLockStore(t)
			mustRun(t, root, "create", "--id", startIssue, "--title", "Start me")
			writeLockRecord(t, root, "_main", record.data)

			for _, args := range commands {
				_, stderr, exitCode := runCLI(t, root, args...)
				if exitCode != ExitError || !strings.Contains(stderr, "_main.json: "+wantMessage) ||
					lastLine(stderr) != "hint: "+corruptLockHint {
					t.Errorf("%v: exit = %d, want %d\nstderr = %s", args, exitCode, ExitError, stderr)
				}

				stdout, _, jsonExit := runCLI(t, root, append(args, "--json")...)
				failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
				message, _ := failure["message"].(string)
				path, _ := failure["path"].(string)
				if jsonExit != ExitError || failure["code"] != string(CodeInvalid) || !strings.HasPrefix(message, wantMessage) ||
					!strings.HasSuffix(path, "_main.json") || failure["hint"] != corruptLockHint {
					t.Errorf("%v --json: exit = %d, error = %v", args, jsonExit, failure)
				}
			}

			stdout, stderr, exitCode := runCLI(t, root, "lock", "status")
			if exitCode != ExitOK || !strings.Contains(stdout, "invalid") {
				t.Errorf("lock status: exit = %d\nstdout = %s\nstderr = %s", exitCode, stdout, stderr)
			}
		})
	}
}

func TestLockReads_FreshRepo_CreateNoLockDirectory(t *testing.T) {
	root := newLockStore(t)
	locksDir := filepath.Join(root, ".git", "capcom", "locks")

	for _, args := range [][]string{{"lock", "check"}, {"lock", "status"}, {"lock", "status", "--all"}, {"prime"}} {
		stdout, stderr, exitCode := runCLI(t, root, args...)

		if exitCode != ExitOK {
			t.Errorf("%v: exit = %d, want 0\nstdout = %s\nstderr = %s", args, exitCode, stdout, stderr)
		}
		if _, err := os.Lstat(locksDir); !os.IsNotExist(err) {
			t.Fatalf("after %v: Lstat(%s) = %v, want no lock directory", args, locksDir, err)
		}
	}
}

func TestLockCheck_ReadOnlyLockDirectory_ExitsByTheRecord(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a read-only directory test does not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("a read-only directory test does not run as root")
	}
	cases := []struct {
		name     string
		record   string
		wantExit int
	}{
		{"missing record", "", ExitOK},
		{"held record", `{"token": "0123456789abcdef0123456789abcdef", "holder": "agent-b"}`, ExitNotFound},
		{"record that does not parse", "{", ExitError},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newLockStore(t)
			locksDir := filepath.Join(root, ".git", "capcom", "locks")
			if err := os.MkdirAll(locksDir, 0o755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			if testCase.record != "" {
				writeLockRecord(t, root, "_main", testCase.record)
			}
			if err := os.Chmod(locksDir, 0o555); err != nil {
				t.Fatalf("Chmod: %v", err)
			}
			t.Cleanup(func() { _ = os.Chmod(locksDir, 0o755) })

			stdout, stderr, exitCode := runCLI(t, root, "lock", "check")

			if exitCode != testCase.wantExit {
				t.Errorf("exit = %d, want %d\nstdout = %s\nstderr = %s", exitCode, testCase.wantExit, stdout, stderr)
			}
			if strings.Contains(stdout+stderr, ".lck") {
				t.Errorf("stdout = %q stderr = %q, want no word of a guard file", stdout, stderr)
			}
		})
	}
}

func TestLockStatus_CorruptOwnRecord_ReportsInvalidWithAWarning(t *testing.T) {
	root := newLockStore(t)
	writeLockRecord(t, root, "_main", "{")
	warning := "_main.json: invalid lock record: unexpected end of JSON input"

	text, textStderr, textExit := runCLI(t, root, "lock", "status")
	kv, _, kvExit := runCLI(t, root, "lock", "status", "--kv")
	jsonOut, _, jsonExit := runCLI(t, root, "lock", "status", "--json")

	if textExit != ExitOK || !strings.HasPrefix(text, "_main: invalid\n") || !strings.Contains(text, warning) {
		t.Errorf("text: exit = %d stdout = %q stderr = %q, want the invalid state and the warning", textExit, text, textStderr)
	}
	if kvExit != ExitOK || kvValue(t, kv, "LOCK") != "invalid" || !strings.Contains(kv, warning) || lastLine(kv) != "STATUS=ok" {
		t.Errorf("--kv: exit = %d stdout = %q, want LOCK=invalid, the warning and STATUS=ok", kvExit, kv)
	}
	envelope := decodeEnvelope(t, jsonOut)
	data, _ := envelope["data"].(map[string]any)
	warnings, _ := envelope["warnings"].([]any)
	if jsonExit != ExitOK || data["lock"] != "invalid" || data["worktree"] != "_main" || len(warnings) != 1 ||
		!strings.Contains(warnings[0].(string), warning) {
		t.Errorf("--json: exit = %d envelope = %v, want lock invalid and one warning", jsonExit, envelope)
	}
}

func TestLockStatus_DirectoryAtOwnRecord_ReportsUnreadableWithAWarning(t *testing.T) {
	root := newLockStore(t)
	recordPath := filepath.Join(root, ".git", "capcom", "locks", "_main.json")
	if err := os.MkdirAll(recordPath, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	stdout, stderr, exitCode := runCLI(t, root, "lock", "status", "--kv")

	if exitCode != ExitOK || kvValue(t, stdout, "LOCK") != "unreadable" || !strings.Contains(stdout, "_main.json: ") {
		t.Errorf("exit = %d stdout = %q stderr = %q, want LOCK=unreadable and a warning naming the record", exitCode, stdout, stderr)
	}
}

func TestLockStatus_ForeignInvalidRecord_PrintsNoWarning(t *testing.T) {
	root := newLockStore(t)
	acquireToken(t, root, "agent-a")
	writeLockRecord(t, root, "wt-ghost", "{")

	stdout, _, exitCode := runCLI(t, root, "lock", "status")

	if exitCode != ExitOK || !strings.HasPrefix(stdout, "_main: held by agent-a") || strings.Contains(stdout, "warning") {
		t.Errorf("exit = %d stdout = %q, want this worktree's held lock and no warning", exitCode, stdout)
	}
}

func TestLockStatusAll_UnreadableForeignRecord_ListsTheRestWithOneWarning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("an unreadable file test does not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("an unreadable file test does not run as root")
	}
	root := newLockStore(t)
	acquireToken(t, root, "agent-a")
	writeLockRecord(t, root, "wt-ghost", "{}")
	foreignPath := filepath.Join(root, ".git", "capcom", "locks", "wt-ghost.json")
	if err := os.Chmod(foreignPath, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	stdout, stderr, exitCode := runCLI(t, root, "lock", "status", "--all", "--kv")

	if exitCode != ExitOK || strings.Count(stdout, "WORKTREE=") != 1 ||
		!strings.Contains(stdout, "WORKTREE=_main\nLOCK=held\nHOLDER=agent-a\n") {
		t.Errorf("exit = %d stdout = %q stderr = %q, want the _main lock alone", exitCode, stdout, stderr)
	}
	if strings.Count(stdout, "WARNING=") != 1 || !strings.Contains(stdout, "wt-ghost.json: ") {
		t.Errorf("stdout = %q, want one warning naming wt-ghost.json", stdout)
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

func TestLockAcquire_ReCreatedWorktree_TakesOverTheOrphanedRecord(t *testing.T) {
	root := newLockStore(t)
	worktree := addWorktree(t, root, "feature")
	oldToken := acquireToken(t, worktree, "agent-a")
	worktree = recreateWorktree(t, root, worktree)

	stdout, stderr, exitCode := runCLI(t, worktree, "lock", "acquire", "--purpose", "item 15", "--holder", "agent-b", "--kv")

	if exitCode != ExitOK || kvValue(t, stdout, "LOCK") != "acquired" || kvValue(t, stdout, "HOLDER") != "agent-b" {
		t.Fatalf("exit = %d stdout = %q stderr = %q, want the orphaned lock taken over", exitCode, stdout, stderr)
	}
	if kvValue(t, stdout, "TOKEN") == oldToken {
		t.Errorf("token = %q, want a new token, not the removed worktree's", oldToken)
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

func TestLockAcquire_HeldByAnother_HintsLockStatus(t *testing.T) {
	root := newLockStore(t)
	acquireToken(t, root, "agent-a")

	assertFailureHint(t, root, "capcom lock status", "lock", "acquire", "--purpose", "other", "--holder", "agent-b")
}

func TestLockTouch_FreeLock_HintsLockStatus(t *testing.T) {
	root := newLockStore(t)

	assertFailureHint(t, root, "capcom lock status", "lock", "touch", "--token", "any")
}

func TestLockRelease_HeldByAnother_HintsLockStatus(t *testing.T) {
	root := newLockStore(t)
	acquireToken(t, root, "agent-a")

	assertFailureHint(t, root, "capcom lock status", "lock", "release", "--token", "not-the-token")
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

// keptTokenFiles returns the token files in root's lock directory.
func keptTokenFiles(t *testing.T, root string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root, ".git", "capcom", "locks", "*.token"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	return paths
}

func TestLockAcquire_ResumeUnderNewPurpose_OldPurposeIsLocked(t *testing.T) {
	root := newLockStore(t)
	token := acquireToken(t, root, "agent-a")

	resumed := mustRun(t, root, "lock", "acquire", "--purpose", "beta", "--token", token, "--kv")
	stdout, _, exitCode := runCLI(t, root, "lock", "acquire", "--purpose", "item 12", "--kv")

	if kvValue(t, resumed, "LOCK") != "resumed" {
		t.Errorf("resume as beta = %q, want LOCK=resumed", resumed)
	}
	if paths := keptTokenFiles(t, root); len(paths) != 1 {
		t.Errorf("token files after the resume = %v, want one", paths)
	}
	if exitCode != ExitNotFound || kvValue(t, stdout, "LOCK") != "held" {
		t.Errorf("acquire under the old purpose: exit = %d stdout = %q, want %d and LOCK=held", exitCode, stdout, ExitNotFound)
	}
}

func TestLockAcquire_SamePurposeWithoutToken_ResumesFromKeptToken(t *testing.T) {
	cases := []struct {
		name    string
		isStale bool
	}{
		{"fresh lock", false},
		{"stale lock", true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newLockStore(t)
			token := acquireToken(t, root, "agent-a")
			if testCase.isStale {
				makeLocksStale(t, root)
			}

			stdout := mustRun(t, root, "lock", "acquire", "--purpose", "item 12", "--kv")

			if kvValue(t, stdout, "LOCK") != "resumed" || kvValue(t, stdout, "TOKEN") != token {
				t.Errorf("stdout = %q, want LOCK=resumed and TOKEN=%s", stdout, token)
			}
		})
	}
}

func TestLockAcquire_WrongTokenBesideKeptOne_FailsLocked(t *testing.T) {
	root := newLockStore(t)
	acquireToken(t, root, "agent-a")

	stdout, _, exitCode := runCLI(t, root, "lock", "acquire", "--purpose", "item 12", "--token", "someone-else", "--kv")

	if exitCode != ExitNotFound || lastLine(stdout) != "STATUS=locked" || kvValue(t, stdout, "TOKEN") != "" {
		t.Errorf("exit = %d stdout = %q, want STATUS=locked and no token", exitCode, stdout)
	}
}

func TestLock_ReleaseAndBreak_DeleteTheKeptTokenFiles(t *testing.T) {
	cases := []struct {
		name string
		args func(token string) []string
	}{
		{"release", func(token string) []string { return []string{"lock", "release", "--token", token} }},
		{"break", func(string) []string { return []string{"lock", "break"} }},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newLockStore(t)
			token := acquireToken(t, root, "agent-a")
			if len(keptTokenFiles(t, root)) != 1 {
				t.Fatalf("token files after acquire = %v, want one", keptTokenFiles(t, root))
			}

			mustRun(t, root, testCase.args(token)...)

			if left := keptTokenFiles(t, root); len(left) != 0 {
				t.Errorf("token files after %s = %v, want none", testCase.name, left)
			}
		})
	}
}

func TestLock_TokenFileBesideRecord_IsNoLockAndNoWarning(t *testing.T) {
	root := newLockStore(t)
	token := acquireToken(t, root, "agent-a")

	all := mustRun(t, root, "lock", "status", "--all", "--kv")
	check := mustRun(t, root, "lock", "check", "--token", token, "--kv")

	if strings.Count(all, "WORKTREE=") != 1 || strings.Contains(all, "WARNING=") {
		t.Errorf("status --all = %q, want one worktree and no warning", all)
	}
	if strings.Contains(check, "WARNING=") || lastLine(check) != "STATUS=ok" {
		t.Errorf("check = %q, want no warning and STATUS=ok", check)
	}
}

func TestLockReleaseHelp_DoesNotSayChangesNothing(t *testing.T) {
	root := newLockStore(t)

	stdout := mustRun(t, root, "lock", "release", "--help")

	if strings.Contains(stdout, "changes nothing") || !strings.Contains(stdout, "token files") {
		t.Errorf("release --help = %q, want the token files and no \"changes nothing\"", stdout)
	}
}

// assertRecordHoldsNoToken fails the test unless root's lock record keeps token only as its
// `sha256:` digest.
func assertRecordHoldsNoToken(t *testing.T, root, token string) {
	t.Helper()
	data, err := os.ReadFile(lockRecordPath(root))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if token == "" || strings.Contains(string(data), token) || !strings.Contains(string(data), `"token": "sha256:`) {
		t.Errorf("record holds %q or no digest:\n%s", token, data)
	}
}

func TestLockAcquire_Record_HoldsTheTokenDigestAndStatusNamesTheHolder(t *testing.T) {
	root := newLockStore(t)

	token := acquireToken(t, root, "agent-a")
	status := mustRun(t, root, "lock", "status", "--kv")

	assertRecordHoldsNoToken(t, root, token)
	if kvValue(t, status, "HOLDER") != "agent-a" || kvValue(t, status, "TOKEN") != "" {
		t.Errorf("status = %q, want the holder named and no token", status)
	}
	mustRun(t, root, "lock", "touch", "--token", token)
	mustRun(t, root, "lock", "check", "--token", token)
}

func TestStart_Record_HoldsTheTokenDigest(t *testing.T) {
	root := newStartStore(t, nil)

	stdout := mustRun(t, root, "start", startIssue, "--kv")

	assertRecordHoldsNoToken(t, root, kvValue(t, stdout, "TOKEN"))
}

func TestLockTouch_LegacyCleartextRecord_MatchesAndIsRewrittenAsADigest(t *testing.T) {
	root := newLockStore(t)
	const legacyToken = "0123456789abcdef0123456789abcdef"
	heartbeat := time.Now().UTC().Format(time.RFC3339)
	writeLockRecord(t, root, "_main", `{"token": "`+legacyToken+`", "purpose": "item 12", "holder": "agent-a", `+
		`"since": "`+heartbeat+`", "heartbeat": "`+heartbeat+`"}`)

	_, _, wrongExit := runCLI(t, root, "lock", "touch", "--token", "fedcba9876543210fedcba9876543210")
	touched := mustRun(t, root, "lock", "touch", "--token", legacyToken, "--kv")

	if wrongExit != ExitNotFound {
		t.Errorf("touch with a wrong token: exit = %d, want %d", wrongExit, ExitNotFound)
	}
	if kvValue(t, touched, "LOCK") != lockHeld || kvValue(t, touched, "HOLDER") != "agent-a" {
		t.Errorf("touch = %q", touched)
	}
	assertRecordHoldsNoToken(t, root, legacyToken)
}

// orphanedRecordLine is the text line of the feature worktree's orphaned lock record.
const orphanedRecordLine = "wt-feature: free (orphaned record of a removed worktree)\n"

// orphanFeatureLock adds the linked worktree feature to root, acquires its lock, removes the
// worktree and adds it again under the same name, leaving its record orphaned; root's own lock is
// held by agent-a. It returns the re-created worktree's root.
func orphanFeatureLock(t *testing.T, root string) string {
	t.Helper()
	worktree := addWorktree(t, root, "feature")
	acquireToken(t, worktree, "agent-b")
	acquireToken(t, root, "agent-a")
	return recreateWorktree(t, root, worktree)
}

// lockEntries returns list, the --json objects of a list of locks, keyed by worktree.
func lockEntries(list any) map[string]map[string]any {
	items, _ := list.([]any)
	entries := map[string]map[string]any{}
	for _, item := range items {
		entry, _ := item.(map[string]any)
		worktree, _ := entry["worktree"].(string)
		entries[worktree] = entry
	}
	return entries
}

func TestLockStatusAll_ReCreatedWorktree_ListsTheOrphanedRecordAsFree(t *testing.T) {
	root := newLockStore(t)
	orphanFeatureLock(t, root)

	text := mustRun(t, root, "lock", "status", "--all")
	entries := lockEntries(decodeEnvelope(t, mustRun(t, root, "lock", "status", "--all", "--json"))["data"])

	if !strings.HasPrefix(text, "_main: held by agent-a") || !strings.HasSuffix(text, "\n"+orphanedRecordLine) {
		t.Errorf("status --all = %q, want the held lock, then %q", text, orphanedRecordLine)
	}
	orphan := entries["wt-feature"]
	if orphan["lock"] != "free" || orphan["orphaned"] != true || orphan["holder"] != nil {
		t.Errorf("wt-feature = %v, want free and orphaned with no holder", orphan)
	}
	if _, isMarked := entries["_main"]["orphaned"]; isMarked || entries["_main"]["lock"] != "held" {
		t.Errorf("_main = %v, want held and not marked orphaned", entries["_main"])
	}
}

func TestLockStatusAll_OnlyAnOrphanedRecord_SaysNoLocksHeld(t *testing.T) {
	root := newLockStore(t)
	orphanFeatureLock(t, root)
	mustRun(t, root, "lock", "break")

	stdout := mustRun(t, root, "lock", "status", "--all")

	if stdout != "no locks held\n"+orphanedRecordLine {
		t.Errorf("status --all = %q", stdout)
	}
}

func TestLockStatus_ReCreatedWorktree_ShowsItsOwnRecordOrphaned(t *testing.T) {
	root := newLockStore(t)
	worktree := orphanFeatureLock(t, root)

	text := mustRun(t, worktree, "lock", "status")
	data, _ := decodeEnvelope(t, mustRun(t, worktree, "lock", "status", "--json"))["data"].(map[string]any)

	if text != orphanedRecordLine {
		t.Errorf("status = %q, want %q", text, orphanedRecordLine)
	}
	if data["lock"] != "free" || data["orphaned"] != true {
		t.Errorf("data = %v, want free and orphaned", data)
	}
}

func TestPrime_OrphanedLockRecord_IsListedButNotCounted(t *testing.T) {
	root := newLockStore(t)
	orphanFeatureLock(t, root)

	stdout := mustRun(t, root, "prime")
	data, _ := decodeEnvelope(t, mustRun(t, root, "prime", "--json"))["data"].(map[string]any)
	entries := lockEntries(data["locks"])

	pattern := regexp.MustCompile(`(?m)^locks: 1\n  _main: held by agent-a [^\n]*\n  ` +
		regexp.QuoteMeta(orphanedRecordLine))
	if !pattern.MatchString(stdout) {
		t.Errorf("prime stdout:\n%s\nwant one lock counted and the orphaned record listed after it", stdout)
	}
	if entries["wt-feature"]["orphaned"] != true || entries["wt-feature"]["lock"] != "free" {
		t.Errorf("prime locks = %v, want wt-feature free and orphaned", entries)
	}
}
