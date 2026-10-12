package lock

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testTTL    = time.Hour
	staleTTL   = time.Nanosecond
	racerCount = 20
)

var tokenPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

var testClaim = Claim{Purpose: "item 12", Holder: "agent-a", Host: "pi", User: "airic", Branch: "main"}

// openTest returns locks in a fresh temp git common dir, with the hour TTL.
func openTest(t *testing.T) (*Locks, string) {
	t.Helper()
	commonDir := t.TempDir()
	return Open(commonDir, testTTL), commonDir
}

// mustAcquire acquires key for testClaim with no token and fails the test on error.
func mustAcquire(t *testing.T, locks *Locks, key string) Status {
	t.Helper()
	status, _, err := locks.Acquire(key, testClaim, "", false)
	if err != nil {
		t.Fatalf("Acquire(%s): %v", key, err)
	}
	return status
}

// asHeldError fails the test unless err is a *HeldError, which it returns.
func asHeldError(t *testing.T, err error) *HeldError {
	t.Helper()
	var held *HeldError
	if !errors.As(err, &held) {
		t.Fatalf("err = %v, want a *HeldError", err)
	}
	return held
}

func TestAcquire_FreeLock_IssuesNewTokenAndWritesRecord(t *testing.T) {
	t.Parallel()
	locks, commonDir := openTest(t)

	status, isResumed, err := locks.Acquire(MainWorktreeKey, testClaim, "", false)

	if err != nil || isResumed {
		t.Fatalf("Acquire = resumed %v, err %v", isResumed, err)
	}
	if !tokenPattern.MatchString(status.Record.Token) {
		t.Errorf("token = %q, want 32 hex digits", status.Record.Token)
	}
	if status.Record.Holder != "agent-a" || status.Record.Branch != "main" || status.IsStale {
		t.Errorf("status = %+v", status)
	}
	if _, err := os.Stat(filepath.Join(commonDir, "capcom", "locks", "_main.json")); err != nil {
		t.Errorf("record missing: %v", err)
	}
}

func TestLocks_AcquireTouchRelease_CycleFreesTheLock(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	acquired := mustAcquire(t, locks, MainWorktreeKey)

	touched, err := locks.Touch(MainWorktreeKey, acquired.Record.Token)
	if err != nil {
		t.Fatalf("Touch: %v", err)
	}
	released, err := locks.Release(MainWorktreeKey, acquired.Record.Token)
	if err != nil {
		t.Fatalf("Release: %v", err)
	}
	after, err := locks.Read(MainWorktreeKey)

	if err != nil || after.IsHeld {
		t.Fatalf("after release: %+v, err %v", after, err)
	}
	if touched.Record.Heartbeat.Before(acquired.Record.Heartbeat) || !touched.Record.Since.Equal(acquired.Record.Since) {
		t.Errorf("touched = %+v, acquired = %+v", touched.Record, acquired.Record)
	}
	if released.Record.Token != acquired.Record.Token {
		t.Errorf("released token = %q", released.Record.Token)
	}
}

func TestAcquire_HeldByAnother_FailsWithHolderAndPurpose(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	mustAcquire(t, locks, MainWorktreeKey)

	_, _, err := locks.Acquire(MainWorktreeKey, Claim{Purpose: "other", Holder: "agent-b"}, "", false)

	held := asHeldError(t, err)
	if held.Status.IsStale || held.Status.Record.Holder != "agent-a" {
		t.Errorf("held = %+v", held.Status)
	}
	if message := err.Error(); !strings.Contains(message, "_main.json: locked by agent-a (item 12) for ") {
		t.Errorf("message = %q", message)
	}
}

func TestAcquire_OwnTokenOnStaleLock_Resumes(t *testing.T) {
	t.Parallel()
	locks, commonDir := openTest(t)
	acquired := mustAcquire(t, locks, MainWorktreeKey)
	staleLocks := Open(commonDir, staleTTL)

	resumed, isResumed, err := staleLocks.Acquire(
		MainWorktreeKey,
		Claim{Purpose: "item 13", Holder: "agent-a"},
		acquired.Record.Token,
		false,
	)

	if err != nil || !isResumed {
		t.Fatalf("Acquire = resumed %v, err %v", isResumed, err)
	}
	if resumed.Record.Token != acquired.Record.Token || resumed.Record.Purpose != "item 13" {
		t.Errorf("resumed = %+v", resumed.Record)
	}
	if !resumed.Record.Since.Equal(acquired.Record.Since) {
		t.Errorf("since = %v, want %v kept", resumed.Record.Since, acquired.Record.Since)
	}
}

func TestAcquire_TakeStaleOnFreshLock_IsRefused(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	mustAcquire(t, locks, MainWorktreeKey)

	_, _, err := locks.Acquire(MainWorktreeKey, testClaim, "", true)

	asHeldError(t, err)
}

func TestAcquire_TakeStaleOnStaleLock_TakesItWithANewToken(t *testing.T) {
	t.Parallel()
	locks, commonDir := openTest(t)
	acquired := mustAcquire(t, locks, MainWorktreeKey)
	staleLocks := Open(commonDir, staleTTL)

	_, _, refusedErr := staleLocks.Acquire(MainWorktreeKey, testClaim, "", false)
	taken, isResumed, err := staleLocks.Acquire(MainWorktreeKey, Claim{Holder: "agent-b"}, "", true)

	if held := asHeldError(t, refusedErr); !held.Status.IsStale {
		t.Errorf("refusal without --take-stale: %+v, want stale", held.Status)
	}
	if err != nil || isResumed {
		t.Fatalf("Acquire = resumed %v, err %v", isResumed, err)
	}
	if taken.Record.Token == acquired.Record.Token || taken.Record.Holder != "agent-b" {
		t.Errorf("taken = %+v", taken.Record)
	}
}

func TestBreak_HeldLock_FreesItAndReturnsTheHolder(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	mustAcquire(t, locks, MainWorktreeKey)

	broken, err := locks.Break(MainWorktreeKey)
	after, readErr := locks.Read(MainWorktreeKey)

	if err != nil || broken.Record.Holder != "agent-a" {
		t.Fatalf("Break = %+v, err %v", broken, err)
	}
	if readErr != nil || after.IsHeld {
		t.Errorf("after break: %+v, err %v", after, readErr)
	}
}

func TestRelease_TokenOfAnother_IsRefused(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	mustAcquire(t, locks, MainWorktreeKey)

	_, err := locks.Release(MainWorktreeKey, "not-the-token")

	asHeldError(t, err)
}

func TestTouch_FreeLock_FailsNotHeld(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)

	status, err := locks.Touch(MainWorktreeKey, "any")

	var notHeld *NotHeldError
	if !errors.Is(err, ErrNotHeld) || !errors.As(err, &notHeld) {
		t.Fatalf("err = %v, want a *NotHeldError matching ErrNotHeld", err)
	}
	if notHeld.Path != locks.RecordPath(MainWorktreeKey) {
		t.Errorf("Path = %q, want %q", notHeld.Path, locks.RecordPath(MainWorktreeKey))
	}
	if status.Key != MainWorktreeKey || status.IsHeld {
		t.Errorf("status = %+v, want the free lock of %q", status, MainWorktreeKey)
	}
}

func TestCheck_FreeOwnAndOtherHolder(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	if _, err := locks.Check(MainWorktreeKey, ""); err != nil {
		t.Fatalf("Check on free lock: %v", err)
	}
	acquired := mustAcquire(t, locks, MainWorktreeKey)

	_, ownErr := locks.Check(MainWorktreeKey, acquired.Record.Token)
	_, otherErr := locks.Check(MainWorktreeKey, "someone-else")
	_, noTokenErr := locks.Check(MainWorktreeKey, "")

	if ownErr != nil {
		t.Errorf("own token: %v", ownErr)
	}
	asHeldError(t, otherErr)
	asHeldError(t, noTokenErr)
}

func TestList_TwoWorktrees_HoldIndependentLocks(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	mustAcquire(t, locks, MainWorktreeKey)
	mustAcquire(t, locks, "wt-feature")

	held, warnings, err := locks.List()

	if err != nil || len(held) != 2 || len(warnings) != 0 {
		t.Fatalf("List = %+v, warnings %v, err %v", held, warnings, err)
	}
	if held[0].Key != MainWorktreeKey || held[1].Key != "wt-feature" {
		t.Errorf("keys = %s, %s", held[0].Key, held[1].Key)
	}
}

func TestList_NoLockDirectory_IsEmpty(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)

	held, warnings, err := locks.List()

	if err != nil || len(held) != 0 || len(warnings) != 0 {
		t.Fatalf("List = %+v, warnings %v, err %v", held, warnings, err)
	}
}

func TestList_FileWhereTheLockDirectoryBelongs_Fails(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	if err := os.MkdirAll(filepath.Dir(locks.dir), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(locks.dir, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	held, _, err := locks.List()

	if err == nil || !strings.Contains(err.Error(), locks.dir) {
		t.Fatalf("List = %+v, err %v; want an error naming %s", held, err, locks.dir)
	}
}

func TestRead_CorruptRecord_FailsNamingTheFile(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	mustAcquire(t, locks, MainWorktreeKey)
	if err := os.WriteFile(locks.RecordPath(MainWorktreeKey), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := locks.Read(MainWorktreeKey)

	if err == nil || !strings.Contains(err.Error(), "_main.json: invalid lock record") {
		t.Fatalf("err = %v", err)
	}
}

// writeRecord replaces key's record file with data and fails the test on error.
func writeRecord(t *testing.T, locks *Locks, key string, data string) {
	t.Helper()
	if err := os.MkdirAll(locks.dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(locks.RecordPath(key), []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestBreak_CorruptRecord_DiscardsIt(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		record string
	}{
		{"zero-length", ""},
		{"truncated JSON", "{"},
		{"no token", "{}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			locks, _ := openTest(t)
			writeRecord(t, locks, MainWorktreeKey, tc.record)

			broken, err := locks.Break(MainWorktreeKey)

			if err != nil || !broken.IsCorrupt || broken.IsHeld || broken.Key != MainWorktreeKey {
				t.Fatalf("Break = %+v, err %v; want a corrupt, free status", broken, err)
			}
			if _, statErr := os.Stat(locks.RecordPath(MainWorktreeKey)); !errors.Is(statErr, os.ErrNotExist) {
				t.Errorf("record still there: stat err %v", statErr)
			}
		})
	}
}

func TestBreak_UnreadableRecord_FailsAndKeepsIt(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	if err := os.MkdirAll(locks.RecordPath(MainWorktreeKey), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	_, err := locks.Break(MainWorktreeKey)

	if err == nil || errors.Is(err, errCorruptRecord) {
		t.Fatalf("err = %v; want a read error that is not a corrupt record", err)
	}
	if _, statErr := os.Stat(locks.RecordPath(MainWorktreeKey)); statErr != nil {
		t.Errorf("record removed: stat err %v", statErr)
	}
}

func TestList_OneCorruptRecord_ReturnsTheValidLockAndOneWarning(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	mustAcquire(t, locks, "wt-feature")
	writeRecord(t, locks, MainWorktreeKey, "{")

	held, warnings, err := locks.List()

	if err != nil || len(held) != 1 || held[0].Key != "wt-feature" {
		t.Fatalf("List = %+v, err %v; want the wt-feature lock alone", held, err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Error(), "_main.json: invalid lock record") {
		t.Errorf("warnings = %v; want one naming _main.json", warnings)
	}
}

func TestWorktreeKey(t *testing.T) {
	t.Parallel()
	commonDir := filepath.Join("repo", ".git")
	cases := []struct {
		name   string
		gitDir string
		want   string
	}{
		{"primary worktree", commonDir, "_main"},
		{"linked worktree", filepath.Join(commonDir, "worktrees", "feature"), "wt-feature"},
		{"linked worktree named _main", filepath.Join(commonDir, "worktrees", "_main"), "wt-_main"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := WorktreeKey(testCase.gitDir, commonDir); got != testCase.want {
				t.Errorf("WorktreeKey = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestAcquire_RacingAcquirers_ExactlyOneWins(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	var waitGroup sync.WaitGroup
	results := make(chan error, racerCount)

	for range racerCount {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, _, err := locks.Acquire(MainWorktreeKey, testClaim, "", false)
			results <- err
		}()
	}
	waitGroup.Wait()
	close(results)

	winners := 0
	for err := range results {
		var held *HeldError
		switch {
		case err == nil:
			winners++
		case !errors.As(err, &held):
			t.Errorf("unexpected error: %v", err)
		}
	}
	if winners != 1 {
		t.Errorf("winners = %d, want exactly 1", winners)
	}
}
