package lock

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	foreignKey  = "wt-other"
	validRecord = `{"token": "0123456789abcdef0123456789abcdef", "holder": "agent-b"}`
)

// mustOverview returns the Lock overview of locks with key's state and fails the test on error.
func mustOverview(t *testing.T, locks *Locks, key string) Overview {
	t.Helper()
	overview, err := locks.Overview(key)
	if err != nil {
		t.Fatalf("Overview(%s): %v", key, err)
	}
	return overview
}

// makeLinkedGitDir creates the git dir of the linked worktree key names under locks' git common
// dir, as `git worktree add` does, so key's lock can be acquired and its record is not orphaned.
func makeLinkedGitDir(t *testing.T, locks *Locks, key string) string {
	t.Helper()
	gitDir, isLinked := locks.linkedGitDir(key)
	if !isLinked {
		t.Fatalf("%s names no linked worktree", key)
	}
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	return gitDir
}

// lockKeys returns the keys of statuses, in order.
func lockKeys(statuses []Status) []string {
	keys := make([]string, 0, len(statuses))
	for _, status := range statuses {
		keys = append(keys, status.Key)
	}
	return keys
}

// skipUnlessPermissionsBind skips a test that needs an unreadable file: Windows ignores the mode
// bits and root reads past them.
func skipUnlessPermissionsBind(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("an unreadable file test does not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("an unreadable file test does not run as root")
	}
}

func TestOverview_OwnState_NamesEachState(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		ttl          time.Duration
		arrange      func(t *testing.T, locks *Locks)
		wantState    State
		wantLocks    []string
		wantProblems int
	}{
		{
			name: "free beside a held lock",
			ttl:  testTTL,
			arrange: func(t *testing.T, locks *Locks) {
				makeLinkedGitDir(t, locks, foreignKey)
				mustAcquire(t, locks, foreignKey)
			},
			wantState: StateFree,
			wantLocks: []string{foreignKey},
		},
		{
			name:      "held",
			ttl:       testTTL,
			arrange:   func(t *testing.T, locks *Locks) { mustAcquire(t, locks, MainWorktreeKey) },
			wantState: StateHeld,
			wantLocks: []string{MainWorktreeKey},
		},
		{
			name:      "stale",
			ttl:       staleTTL,
			arrange:   func(t *testing.T, locks *Locks) { mustAcquire(t, locks, MainWorktreeKey) },
			wantState: StateStale,
			wantLocks: []string{MainWorktreeKey},
		},
		{
			name:         "invalid",
			ttl:          testTTL,
			arrange:      func(t *testing.T, locks *Locks) { writeRecord(t, locks, MainWorktreeKey, "{") },
			wantState:    StateInvalid,
			wantLocks:    []string{},
			wantProblems: 1,
		},
		{
			name: "unreadable: a directory at the record",
			ttl:  testTTL,
			arrange: func(t *testing.T, locks *Locks) {
				if err := os.MkdirAll(locks.RecordPath(MainWorktreeKey), 0o755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
			},
			wantState:    StateUnreadable,
			wantLocks:    []string{},
			wantProblems: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			locks := Open(t.TempDir(), tc.ttl)
			tc.arrange(t, locks)

			overview := mustOverview(t, locks, MainWorktreeKey)

			if overview.Key != MainWorktreeKey || overview.State != tc.wantState {
				t.Errorf("overview = key %q state %q, want %q %q", overview.Key, overview.State, MainWorktreeKey, tc.wantState)
			}
			if got := strings.Join(lockKeys(overview.Locks), ","); got != strings.Join(tc.wantLocks, ",") {
				t.Errorf("locks = %s, want %s", got, strings.Join(tc.wantLocks, ","))
			}
			if len(overview.Problems) != tc.wantProblems {
				t.Fatalf("problems = %v, want %d", overview.Problems, tc.wantProblems)
			}
			isOwnHeld := tc.wantState == StateHeld || tc.wantState == StateStale
			if overview.Own.Key != MainWorktreeKey || overview.Own.IsHeld != isOwnHeld {
				t.Errorf("own = %+v, want key %q held %v", overview.Own, MainWorktreeKey, isOwnHeld)
			}
			for _, problem := range overview.Problems {
				if problem.Key != MainWorktreeKey || problem.State != tc.wantState || problem.Path != locks.RecordPath(MainWorktreeKey) {
					t.Errorf("problem = %+v, want key %q state %q path %s", problem, MainWorktreeKey, tc.wantState, locks.RecordPath(MainWorktreeKey))
				}
			}
		})
	}
}

func TestOverview_TwoWorktrees_HoldIndependentLocks(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	makeLinkedGitDir(t, locks, foreignKey)
	mustAcquire(t, locks, foreignKey)
	mustAcquire(t, locks, MainWorktreeKey)

	overview := mustOverview(t, locks, MainWorktreeKey)

	if got := strings.Join(lockKeys(overview.Locks), ","); got != MainWorktreeKey+","+foreignKey {
		t.Errorf("locks = %s, want %s,%s ordered by key", got, MainWorktreeKey, foreignKey)
	}
	if overview.State != StateHeld || len(overview.Problems) != 0 {
		t.Errorf("overview = %+v; want this worktree held and no problems", overview)
	}
}

func TestOverview_ForeignInvalidRecordBesideAHeldLock_IsOneProblem(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	mustAcquire(t, locks, MainWorktreeKey)
	writeRecord(t, locks, foreignKey, "{")

	overview := mustOverview(t, locks, MainWorktreeKey)

	if overview.State != StateHeld || len(overview.Locks) != 1 || overview.Locks[0].Key != MainWorktreeKey {
		t.Fatalf("overview = %+v; want this worktree held and its lock alone", overview)
	}
	if len(overview.Problems) != 1 {
		t.Fatalf("problems = %v, want one", overview.Problems)
	}
	problem := overview.Problems[0]
	path := locks.RecordPath(foreignKey)
	want := path + ": invalid lock record: unexpected end of JSON input"
	if problem.Key != foreignKey || problem.Path != path || problem.State != StateInvalid || problem.Error() != want {
		t.Errorf("problem = %+v (%q), want key %q path %s state invalid, wording %q", problem, problem.Error(), foreignKey, path, want)
	}
	if !errors.Is(problem, ErrCorruptRecord) {
		t.Errorf("problem %v does not carry read's error", problem)
	}
}

func TestOverview_ForeignUnreadableRecordBesideAHeldLock_IsOneProblem(t *testing.T) {
	skipUnlessPermissionsBind(t)
	t.Parallel()
	locks, _ := openTest(t)
	mustAcquire(t, locks, MainWorktreeKey)
	writeRecord(t, locks, foreignKey, validRecord)
	path := locks.RecordPath(foreignKey)
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	_, ioErr := os.ReadFile(path)
	if ioErr == nil {
		t.Fatalf("reading %s after chmod 000 succeeded", path)
	}

	overview := mustOverview(t, locks, MainWorktreeKey)

	if overview.State != StateHeld || len(overview.Locks) != 1 || overview.Locks[0].Key != MainWorktreeKey {
		t.Fatalf("overview = %+v; want this worktree held and its lock alone", overview)
	}
	if len(overview.Problems) != 1 {
		t.Fatalf("problems = %v, want one", overview.Problems)
	}
	problem := overview.Problems[0]
	want := path + ": " + ioErr.Error()
	if problem.Key != foreignKey || problem.Path != path || problem.State != StateUnreadable || problem.Error() != want {
		t.Errorf("problem = %+v (%q), want key %q path %s state unreadable, wording %q", problem, problem.Error(), foreignKey, path, want)
	}
}

func TestOverview_DirectoryAtAForeignGuardBesideAHeldLock_ListsBothLocks(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	mustAcquire(t, locks, MainWorktreeKey)
	makeLinkedGitDir(t, locks, foreignKey)
	writeRecord(t, locks, foreignKey, validRecord)
	guardPath := filepath.Join(locks.dir, foreignKey+guardExtension)
	if err := os.Mkdir(guardPath, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	overview := mustOverview(t, locks, MainWorktreeKey)

	keys := lockKeys(overview.Locks)
	if overview.State != StateHeld || len(keys) != 2 || keys[0] != MainWorktreeKey || keys[1] != foreignKey {
		t.Fatalf("overview = %+v; want this worktree held beside %s", overview, foreignKey)
	}
	if len(overview.Problems) != 0 {
		t.Errorf("problems = %v, want none: reading takes no guard", overview.Problems)
	}
}

func TestOverview_NoLockDirectory_IsEmptyFreeAndCreatesNothing(t *testing.T) {
	t.Parallel()
	locks, commonDir := openTest(t)

	overview := mustOverview(t, locks, MainWorktreeKey)

	if overview.State != StateFree || overview.Key != MainWorktreeKey || len(overview.Locks) != 0 || len(overview.Problems) != 0 {
		t.Fatalf("overview = %+v; want empty with this worktree free", overview)
	}
	if overview.Locks == nil {
		t.Errorf("locks = nil, want an empty list")
	}
	entries, err := os.ReadDir(commonDir)
	if err != nil || len(entries) != 0 {
		t.Errorf("git common dir holds %v (err %v), want nothing created", entries, err)
	}
}

func TestOverview_FileWhereTheLockDirectoryBelongs_Fails(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	if err := os.MkdirAll(filepath.Dir(locks.dir), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(locks.dir, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	overview, err := locks.Overview(MainWorktreeKey)

	if err == nil || !strings.Contains(err.Error(), locks.dir) {
		t.Fatalf("Overview = %+v, err %v; want an error naming %s", overview, err, locks.dir)
	}
}

func TestWorktree_Overview_GivesThisWorktreesState(t *testing.T) {
	t.Parallel()
	worktree := ForWorktree(testStore(t, func(commonDir string) string { return commonDir }))
	if _, _, err := worktree.Acquire(testClaim, "", false); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	overview, err := worktree.Overview()

	if err != nil || overview.Key != MainWorktreeKey || overview.State != StateHeld || overview.Own.Record.Holder != testClaim.Holder {
		t.Fatalf("Overview = %+v, err %v; want this worktree held by %s", overview, err, testClaim.Holder)
	}
}

func TestOverview_TokenFileBesideTheRecord_ListsOneWorktree(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	mustAcquire(t, locks, MainWorktreeKey)
	if _, err := os.Stat(locks.tokenPath(MainWorktreeKey, testClaim.Purpose)); err != nil {
		t.Fatalf("token file: %v", err)
	}

	overview := mustOverview(t, locks, MainWorktreeKey)

	if got := strings.Join(lockKeys(overview.Locks), ","); got != MainWorktreeKey {
		t.Errorf("locks = %s, want %s", got, MainWorktreeKey)
	}
	if len(overview.Problems) != 0 {
		t.Errorf("problems = %v, want none", overview.Problems)
	}
}
