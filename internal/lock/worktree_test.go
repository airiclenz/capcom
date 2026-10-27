package lock

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/store"
)

const linkedWorktreeName = "feature"

// Worktree ids the orphan tests write: the one a record names and another one.
const (
	recordedWorktreeID = "0123456789abcdef0123456789abcdef"
	otherWorktreeID    = "fedcba9876543210fedcba9876543210"
)

// testStore returns a store literal whose git common dir is a fresh temp dir and whose git dir is
// gitDirOf(commonDir), with the hour TTL.
func testStore(t *testing.T, gitDirOf func(commonDir string) string) *store.Store {
	t.Helper()
	commonDir := t.TempDir()
	return &store.Store{
		GitDir:       gitDirOf(commonDir),
		GitCommonDir: commonDir,
		Config:       config.Config{Lock: config.Lock{TTL: testTTL}},
	}
}

func TestForWorktree_Key_NamesTheWorktree(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		gitDirOf func(commonDir string) string
		want     string
	}{
		{
			name:     "main worktree",
			gitDirOf: func(commonDir string) string { return commonDir },
			want:     MainWorktreeKey,
		},
		{
			name: "linked worktree",
			gitDirOf: func(commonDir string) string {
				return filepath.Join(commonDir, "worktrees", linkedWorktreeName)
			},
			want: linkedWorktreePrefix + linkedWorktreeName,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			worktree := ForWorktree(testStore(t, testCase.gitDirOf))

			got := worktree.Key()

			if got != testCase.want {
				t.Errorf("Key() = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestWorktree_AcquireThenTouch_WritesTheRecordAtRecordPath(t *testing.T) {
	t.Parallel()
	worktree := ForWorktree(testStore(t, func(commonDir string) string { return commonDir }))
	acquired, _, err := worktree.Acquire(testClaim, "", false)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	touched, err := worktree.Touch(acquired.Token)

	if err != nil {
		t.Fatalf("Touch: %v", err)
	}
	data, err := os.ReadFile(worktree.RecordPath())
	if err != nil {
		t.Fatalf("reading the record: %v", err)
	}
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("parsing the record: %v", err)
	}
	if record.Token != digestToken(acquired.Token) || !record.Heartbeat.Equal(touched.Record.Heartbeat) {
		t.Errorf("record = %+v, want token %q and heartbeat %v", record, digestToken(acquired.Token), touched.Record.Heartbeat)
	}
	if touched.Key != MainWorktreeKey {
		t.Errorf("touched key = %q, want %q", touched.Key, MainWorktreeKey)
	}
}

// writeRecordOfWorktree writes key's record as held by agent-b with legacyToken and a heartbeat of
// now, naming worktreeID.
func writeRecordOfWorktree(t *testing.T, locks *Locks, key, worktreeID string) {
	t.Helper()
	now := time.Now().UTC()
	data, err := json.Marshal(Record{
		Token:     legacyToken,
		Purpose:   "removed worktree",
		Holder:    "agent-b",
		Since:     now,
		Heartbeat: now,
		Worktree:  worktreeID,
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	writeRecord(t, locks, key, string(data))
}

// writeWorktreeID keeps id as the worktree id under gitDir.
func writeWorktreeID(t *testing.T, gitDir, id string) {
	t.Helper()
	idDir := filepath.Join(gitDir, capcomDir)
	if err := os.MkdirAll(idDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(idDir, worktreeIDFile), []byte(id+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// mustReadWorktreeID returns the worktree id kept under gitDir, failing the test on error.
func mustReadWorktreeID(t *testing.T, gitDir string) string {
	t.Helper()
	id, err := readWorktreeID(gitDir)
	if err != nil {
		t.Fatalf("readWorktreeID: %v", err)
	}
	return id
}

func TestRead_LinkedWorktreeRecord_IsOrphanedOnlyWhenItsWorktreeIsGone(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		hasGitDir      bool
		keptID         string
		recordedID     string
		wantIsOrphaned bool
	}{
		{"id matches", true, recordedWorktreeID, recordedWorktreeID, false},
		{"id differs", true, otherWorktreeID, recordedWorktreeID, true},
		{"id missing", true, "", recordedWorktreeID, true},
		{"git dir missing", false, "", recordedWorktreeID, true},
		{"legacy record, git dir present", true, "", "", false},
		{"legacy record, git dir missing", false, "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			locks, _ := openTest(t)
			if tc.hasGitDir {
				gitDir := makeLinkedGitDir(t, locks, foreignKey)
				if tc.keptID != "" {
					writeWorktreeID(t, gitDir, tc.keptID)
				}
			}
			writeRecordOfWorktree(t, locks, foreignKey, tc.recordedID)

			status, readErr := locks.Read(foreignKey)
			_, checkErr := locks.Check(foreignKey, "")

			if readErr != nil {
				t.Fatalf("Read: %v", readErr)
			}
			if status.IsOrphaned != tc.wantIsOrphaned || status.IsHeld == tc.wantIsOrphaned {
				t.Errorf("status = %+v, want orphaned %v", status, tc.wantIsOrphaned)
			}
			if tc.wantIsOrphaned && (status.State() != StateFree || status.Record.Holder != "agent-b" || checkErr != nil) {
				t.Errorf("orphaned status = %+v, state %q, check %v; want free with the record and Check passing", status, status.State(), checkErr)
			}
			if !tc.wantIsOrphaned {
				asHeldError(t, checkErr)
			}
		})
	}
}

func TestRead_MainRecordNamingAWorktreeID_IsNeverOrphaned(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	writeRecordOfWorktree(t, locks, MainWorktreeKey, recordedWorktreeID)

	status, err := locks.Read(MainWorktreeKey)

	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !status.IsHeld || status.IsOrphaned {
		t.Errorf("status = %+v, want held and not orphaned", status)
	}
}

func TestRead_InvalidLinkedRecordWithNoGitDir_StaysInvalid(t *testing.T) {
	t.Parallel()
	for _, data := range []string{"{", "{}"} {
		t.Run(data, func(t *testing.T) {
			t.Parallel()
			locks, _ := openTest(t)
			writeRecord(t, locks, foreignKey, data)

			status, err := locks.Read(foreignKey)

			if !errors.Is(err, ErrCorruptRecord) || status.IsOrphaned {
				t.Errorf("Read = %+v, %v; want an invalid record, not an orphaned one", status, err)
			}
		})
	}
}

func TestAcquire_OrphanedRecord_TakesItOverWithANewTokenAndDropsItsTokenFiles(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	gitDir := makeLinkedGitDir(t, locks, foreignKey)
	writeRecordOfWorktree(t, locks, foreignKey, recordedWorktreeID)
	writeTokenFileContent(t, locks, foreignKey, "removed worktree", legacyToken+"\nremoved worktree\n")

	acquired, isResumed, err := locks.Acquire(foreignKey, testClaim, "", false)

	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if isResumed || acquired.Token == legacyToken || !tokenPattern.MatchString(acquired.Token) {
		t.Errorf("acquired = %+v, resumed %v; want a new token, not a resume", acquired, isResumed)
	}
	assertExists(t, locks.tokenPath(foreignKey, "removed worktree"), false)
	assertExists(t, locks.tokenPath(foreignKey, testClaim.Purpose), true)
	_, record := readRecordFile(t, locks, foreignKey)
	keptID := mustReadWorktreeID(t, gitDir)
	if record.Holder != testClaim.Holder || keptID == "" || record.Worktree != keptID {
		t.Errorf("record = %+v, want held by %s naming the kept worktree id %q", record, testClaim.Holder, keptID)
	}
}

func TestAcquire_LinkedWorktree_KeepsOneIDInTheRecordAcrossAResume(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	gitDir := makeLinkedGitDir(t, locks, foreignKey)
	acquired := mustAcquire(t, locks, foreignKey)
	firstID := mustReadWorktreeID(t, gitDir)

	resumed, isResumed, err := locks.Acquire(foreignKey, testClaim, acquired.Token, false)

	if err != nil || !isResumed {
		t.Fatalf("resume = %v, %v; want a resume", isResumed, err)
	}
	if !tokenPattern.MatchString(firstID) || acquired.Record.Worktree != firstID || resumed.Record.Worktree != firstID {
		t.Errorf("ids = kept %q, acquired %q, resumed %q; want one random id", firstID, acquired.Record.Worktree, resumed.Record.Worktree)
	}
	if got := mustReadWorktreeID(t, gitDir); got != firstID {
		t.Errorf("kept id = %q after the resume, want %q", got, firstID)
	}
}

func TestAcquire_LinkedWorktreeWithNoGitDir_FailsAndCreatesNothing(t *testing.T) {
	t.Parallel()
	locks, _ := openTest(t)
	gitDir, _ := locks.linkedGitDir(foreignKey)

	_, _, err := locks.Acquire(foreignKey, testClaim, "", false)

	if err == nil {
		t.Fatal("Acquire succeeded, want a failure naming the missing git dir")
	}
	assertExists(t, gitDir, false)
	assertExists(t, locks.RecordPath(foreignKey), false)
	assertExists(t, locks.tokenPath(foreignKey, testClaim.Purpose), false)
}
