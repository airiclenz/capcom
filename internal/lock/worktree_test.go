package lock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/store"
)

const linkedWorktreeName = "feature"

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

	touched, err := worktree.Touch(acquired.Record.Token)

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
	if record.Token != acquired.Record.Token || !record.Heartbeat.Equal(touched.Record.Heartbeat) {
		t.Errorf("record = %+v, want token %q and heartbeat %v", record, acquired.Record.Token, touched.Record.Heartbeat)
	}
	if touched.Key != MainWorktreeKey {
		t.Errorf("touched key = %q, want %q", touched.Key, MainWorktreeKey)
	}
}
