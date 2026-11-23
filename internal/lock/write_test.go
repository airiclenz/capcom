package lock

import (
	"errors"
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/proc"
)

// Writes the CheckWrites tests pass: one of the lock's purpose (testClaim's), one of another
// Issue, a move of the purpose Issue (old id and new), and one of no Issue (a Note's).
var (
	purposeWrite = []string{testClaim.Purpose}
	otherWrite   = []string{"other"}
	movedWrite   = []string{"other", testClaim.Purpose}
	noIssueWrite = []string(nil)
)

// testWorktree returns the main worktree's lock in locks.
func testWorktree(locks *Locks) Worktree {
	return Worktree{locks: locks, key: MainWorktreeKey}
}

// tempNamePattern matches the random suffix of the temp file a record write creates.
var tempNamePattern = regexp.MustCompile(`\.tmp-[0-9]+`)

// maskTempName replaces the random suffix of a temp file named in text, so two failed writes of
// the same record compare equal.
func maskTempName(text string) string {
	return tempNamePattern.ReplaceAllString(text, ".tmp-N")
}

// countingChain returns a chain func that returns chain and counts its calls in calls.
func countingChain(chain []proc.Process, calls *int) func() []proc.Process {
	return func() []proc.Process {
		*calls++
		return chain
	}
}

func TestCheckWrites_DecisionTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		holder   *proc.Process
		chain    []proc.Process
		writes   [][]string
		isWarned bool
	}{
		{"write of another Issue warns", nil, nil, [][]string{otherWrite}, true},
		{"write of the purpose Issue is silent", nil, nil, [][]string{purposeWrite}, false},
		{"move of the purpose Issue is silent", nil, nil, [][]string{movedWrite}, false},
		{"write of no Issue warns", nil, nil, [][]string{noIssueWrite}, true},
		{"purpose and another write warn", nil, nil, [][]string{purposeWrite, otherWrite}, true},
		{"holder process writing another Issue is silent", &agentProcess, []proc.Process{agentProcess}, [][]string{otherWrite}, false},
		{"descendant of the holder process is silent", &agentProcess, []proc.Process{otherProcess, agentProcess}, [][]string{noIssueWrite}, false},
		{"other process writing another Issue warns", &agentProcess, []proc.Process{otherProcess}, [][]string{otherWrite}, true},
		{"other process writing the purpose Issue is silent", &agentProcess, []proc.Process{otherProcess}, [][]string{purposeWrite}, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			locks, _ := openTest(t)
			if _, _, err := locks.Acquire(MainWorktreeKey, claimFrom(testCase.holder), "", false); err != nil {
				t.Fatalf("Acquire: %v", err)
			}
			chain := func() []proc.Process { return testCase.chain }

			held, err := testWorktree(locks).CheckWrites(chain, testCase.writes)

			if err != nil {
				t.Fatalf("CheckWrites: %v", err)
			}
			if isWarned := held != nil; isWarned != testCase.isWarned {
				t.Fatalf("held = %v, want warned %t", held, testCase.isWarned)
			}
			if held != nil && (held.Status.Record.Purpose != testClaim.Purpose || held.Status.IsStale) {
				t.Errorf("held = %+v, want the live lock for %q", held.Status, testClaim.Purpose)
			}
		})
	}
}

func TestCheckWrites_FreeLock_IsSilentAndCreatesNothing(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	calls := 0

	held, err := testWorktree(locks).CheckWrites(countingChain(nil, &calls), [][]string{otherWrite})

	if held != nil || err != nil {
		t.Errorf("CheckWrites = %v, %v; want nil, nil", held, err)
	}
	if calls != 0 {
		t.Errorf("chain called %d times, want 0", calls)
	}
	if _, statErr := os.Stat(locks.dir); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("Stat(%s) = %v, want no lock directory", locks.dir, statErr)
	}
}

func TestCheckWrites_RecordWithoutHolderProcess_NeverReadsTheChain(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	mustAcquire(t, locks, MainWorktreeKey)
	calls := 0

	held, err := testWorktree(locks).CheckWrites(countingChain([]proc.Process{agentProcess}, &calls), [][]string{otherWrite})

	if err != nil || held == nil {
		t.Fatalf("CheckWrites = %v, %v; want the held lock", held, err)
	}
	if calls != 0 {
		t.Errorf("chain called %d times, want 0", calls)
	}
}

func TestCheckWrites_RecordWithHolderProcess_ReadsTheChainOnce(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	if _, _, err := locks.Acquire(MainWorktreeKey, claimFrom(&agentProcess), "", false); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	calls := 0

	held, err := testWorktree(locks).CheckWrites(countingChain([]proc.Process{agentProcess}, &calls), [][]string{otherWrite})

	if held != nil || err != nil {
		t.Errorf("CheckWrites = %v, %v; want nil, nil", held, err)
	}
	if calls != 1 {
		t.Errorf("chain called %d times, want 1", calls)
	}
}

func TestCheckWrites_HolderProcess_RenewsTheHeartbeat(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	if _, _, err := locks.Acquire(MainWorktreeKey, claimFrom(&agentProcess), "", false); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	before := backdateHeartbeat(t, locks, MainWorktreeKey)
	chain := func() []proc.Process { return []proc.Process{agentProcess} }

	held, err := testWorktree(locks).CheckWrites(chain, [][]string{otherWrite})

	if held != nil || err != nil {
		t.Fatalf("CheckWrites = %v, %v; want nil, nil", held, err)
	}
	if _, record := readRecordFile(t, locks, MainWorktreeKey); !record.Heartbeat.After(before) {
		t.Errorf("heartbeat = %v, want after %v", record.Heartbeat, before)
	}
}

func TestCheckWrites_OtherProcess_LeavesTheHeartbeat(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	if _, _, err := locks.Acquire(MainWorktreeKey, claimFrom(&agentProcess), "", false); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	before := backdateHeartbeat(t, locks, MainWorktreeKey)
	chain := func() []proc.Process { return []proc.Process{otherProcess} }

	held, err := testWorktree(locks).CheckWrites(chain, [][]string{purposeWrite})

	if held != nil || err != nil {
		t.Fatalf("CheckWrites = %v, %v; want nil, nil", held, err)
	}
	if _, record := readRecordFile(t, locks, MainWorktreeKey); !record.Heartbeat.Equal(before) {
		t.Errorf("heartbeat = %v, want %v", record.Heartbeat, before)
	}
}

func TestCheckWrites_StaleLock_IsReportedStale(t *testing.T) {
	t.Parallel()
	locks, commonDir := openTest(t)
	mustAcquire(t, locks, MainWorktreeKey)
	backdateHeartbeat(t, locks, MainWorktreeKey)
	stale := Open(commonDir, staleTTL)

	held, err := testWorktree(stale).CheckWrites(func() []proc.Process { return nil }, [][]string{otherWrite})

	if err != nil || held == nil {
		t.Fatalf("CheckWrites = %v, %v; want the held lock", held, err)
	}
	if !held.Status.IsStale {
		t.Errorf("held = %+v, want a stale lock", held.Status)
	}
}

func TestCheckWrites_UnreadableRecord_FailsWithTheReadError(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	writeRecord(t, locks, MainWorktreeKey, "{not json")
	_, checkErr := locks.Check(MainWorktreeKey, "")

	held, err := testWorktree(locks).CheckWrites(func() []proc.Process { return nil }, [][]string{otherWrite})

	var renewal *RenewalError
	if held != nil || err == nil || errors.As(err, &renewal) {
		t.Fatalf("CheckWrites = %v, %v; want only the read error", held, err)
	}
	if err.Error() != checkErr.Error() {
		t.Errorf("err = %q, want Check's %q", err, checkErr)
	}
}

func TestCheckWrites_RenewalFails_CarriesTouchHoldersError(t *testing.T) {
	skipUnlessPermissionsBind(t)
	t.Parallel()
	locks, _ := openTest(t)
	if _, _, err := locks.Acquire(MainWorktreeKey, claimFrom(&agentProcess), "", false); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := os.Chmod(locks.dir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locks.dir, 0o755) })
	worktree := testWorktree(locks)
	chain := []proc.Process{agentProcess}
	_, touchErr := worktree.TouchHolder(chain)

	held, err := worktree.CheckWrites(func() []proc.Process { return chain }, [][]string{otherWrite})

	var renewal *RenewalError
	if held != nil || !errors.As(err, &renewal) {
		t.Fatalf("CheckWrites = %v, %v; want a *RenewalError", held, err)
	}
	if err.Error() != renewal.Err.Error() || errors.Unwrap(err) != renewal.Err {
		t.Errorf("err = %v, want TouchHolder's error unchanged", err)
	}
	if touchErr == nil || maskTempName(err.Error()) != maskTempName(touchErr.Error()) {
		t.Errorf("err = %v, want TouchHolder's error %v", err, touchErr)
	}
	if !errors.Is(err, fs.ErrPermission) || !strings.HasPrefix(err.Error(), worktree.RecordPath()+": ") {
		t.Errorf("err = %v, want a permission error naming the record file first", err)
	}
}

func TestRenewForHolder_LockTakenOver_ReturnsTheNewHolder(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	if _, _, err := locks.Acquire(MainWorktreeKey, claimFrom(&otherProcess), "", false); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	held, err := testWorktree(locks).renewForHolder([]proc.Process{agentProcess})

	if err != nil || held == nil || held.Status.Record.Process.PID != otherProcess.PID {
		t.Errorf("renewForHolder = %v, %v; want the other process's lock", held, err)
	}
}

func TestRenewForHolder_FreedLock_IsSilent(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)

	held, err := testWorktree(locks).renewForHolder([]proc.Process{agentProcess})

	if held != nil || err != nil {
		t.Errorf("renewForHolder = %v, %v; want nil, nil", held, err)
	}
}
