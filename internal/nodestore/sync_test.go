package nodestore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// errSyncFailed is the failure of a sync hook that fails.
var errSyncFailed = errors.New("sync failed")

// countingSync is a sync hook that counts its calls and records the file at path as each call
// finds it, "" when it is missing; it returns isSynced and err.
type countingSync struct {
	path     string
	isSynced bool
	err      error
	calls    int
	seen     []string
}

// hook is the SyncFunc of c.
func (c *countingSync) hook() (bool, error) {
	c.calls++
	data, err := os.ReadFile(c.path)
	if err != nil {
		data = nil
	}
	c.seen = append(c.seen, string(data))
	return c.isSynced, c.err
}

// assertSyncedOnce checks that the hook ran once, that the write's effects report it synced, and
// returns what the hook found at its path.
func assertSyncedOnce(t *testing.T, sync *countingSync, effects Effects) string {
	t.Helper()
	if sync.calls != 1 {
		t.Fatalf("hook calls = %d, want 1", sync.calls)
	}
	if want := (Effects{IsSynced: true}); effects != want {
		t.Errorf("Effects = %+v, want %+v", effects, want)
	}
	return sync.seen[0]
}

func TestSync_CreateAtTheTop_RunsTheHookOnceAfterTheWrite(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	sync := &countingSync{path: filepath.Join(s.IssuesDir(), newIssueID+".md"), isSynced: true}

	written, _, err := Issues(s, nil).WithSync(sync.hook).Create(newIssueID, "", buildIssue(t, "open"))

	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if seen := assertSyncedOnce(t, sync, written.Effects); !strings.Contains(seen, "title: Fix the parser") {
		t.Errorf("hook found %q, want the new node already on disk", seen)
	}
}

func TestSync_CreateBelowAParent_RunsNoHook(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeNodeFile(t, s.IssuesDir(), "parser-epic.md", canonicalIssue)
	sync := &countingSync{path: filepath.Join(s.IssuesDir(), "parser-epic", newIssueID+".md"), isSynced: true}

	written, _, err := Issues(s, nil).WithSync(sync.hook).Create(newIssueID, "parser-epic", buildIssue(t, "open"))

	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if sync.calls != 0 || written.Effects != (Effects{}) {
		t.Errorf("hook calls = %d, Effects = %+v; want no call and zero Effects", sync.calls, written.Effects)
	}
}

func TestSync_Edit_RunsTheHookOnceAfterTheWrite(t *testing.T) {
	t.Parallel()
	issues, path := newEditedIssue(t)
	sync := &countingSync{path: path, isSynced: true}
	calls := 0

	written, err := issues.WithSync(sync.hook).Edit(editedIssueID, "", retitle("Fix the lexer", &calls))

	if err != nil {
		t.Fatalf("Edit failed: %v", err)
	}
	if seen := assertSyncedOnce(t, sync, written.Effects); !strings.Contains(seen, "title: Fix the lexer") {
		t.Errorf("hook found %q, want the edit already on disk", seen)
	}
}

func TestSync_Move_RunsTheHookOnceWithTheNewFileOnDisk(t *testing.T) {
	t.Parallel()
	issues, dir := newMoveIssues(t)
	sync := &countingSync{path: filepath.Join(dir, "auth-flow.md"), isSynced: true}

	moved, _, err := issues.WithSync(sync.hook).Move("login", Destination{ID: "auth-flow"}, "")

	if err != nil {
		t.Fatalf("Move failed: %v", err)
	}
	if seen := assertSyncedOnce(t, sync, moved.Effects); !strings.Contains(seen, "aliases: [login-flow]") {
		t.Errorf("hook found %q, want the moved node already at its new file", seen)
	}
}

func TestSync_Remove_RunsTheHookOnceWithTheFileGone(t *testing.T) {
	t.Parallel()
	issues, dir := newRemoveIssues(t)
	path := filepath.Join(dir, "login-flow", "child-issue.md")
	sync := &countingSync{path: path, isSynced: true}

	removed, _, err := issues.WithSync(sync.hook).Remove("child", "", false)

	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if seen := assertSyncedOnce(t, sync, removed.Effects); seen != "" {
		t.Errorf("hook found %q, want the removed node's file already gone", seen)
	}
}

func TestSync_RefusedWrites_RunNoHook(t *testing.T) {
	t.Parallel()
	const staleETag = "0000000000000000"
	tests := []struct {
		name  string
		write func(t *testing.T, sync SyncFunc) error
	}{
		{
			name: "Create of a taken id",
			write: func(t *testing.T, sync SyncFunc) error {
				s := newStore(t)
				writeNodeFile(t, s.IssuesDir(), newIssueID+".md", canonicalIssue)
				_, _, err := Issues(s, nil).WithSync(sync).Create(newIssueID, "", buildIssue(t, "open"))
				return err
			},
		},
		{
			name: "Edit with a stale etag",
			write: func(t *testing.T, sync SyncFunc) error {
				issues, _ := newEditedIssue(t)
				calls := 0
				_, err := issues.WithSync(sync).Edit(editedIssueID, staleETag, retitle("Fix the lexer", &calls))
				return err
			},
		},
		{
			name: "Move with a stale etag",
			write: func(t *testing.T, sync SyncFunc) error {
				issues, _ := newMoveIssues(t)
				_, _, err := issues.WithSync(sync).Move("login", Destination{ID: "auth-flow"}, staleETag)
				return err
			},
		},
		{
			name: "Remove with a stale etag",
			write: func(t *testing.T, sync SyncFunc) error {
				issues, _ := newRemoveIssues(t)
				_, _, err := issues.WithSync(sync).Remove("child", staleETag, false)
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			sync := &countingSync{}

			err := test.write(t, sync.hook)

			if err == nil {
				t.Fatal("write succeeded, want a refusal")
			}
			if sync.calls != 0 {
				t.Errorf("hook calls = %d, want none after a refused write", sync.calls)
			}
		})
	}
}

func TestSync_HookFails_TheWriteLandsAndReportsTheFailure(t *testing.T) {
	t.Parallel()
	issues, path := newEditedIssue(t)
	sync := &countingSync{path: path, isSynced: true, err: errSyncFailed}
	calls := 0

	written, err := issues.WithSync(sync.hook).Edit(editedIssueID, "", retitle("Fix the lexer", &calls))

	if err != nil {
		t.Fatalf("Edit failed: %v; want the write to land", err)
	}
	if !strings.Contains(readFile(t, path), "title: Fix the lexer") {
		t.Errorf("file =\n%s\nwant the edit written", readFile(t, path))
	}
	if !errors.Is(written.SyncFailure, errSyncFailed) || written.IsSynced {
		t.Errorf("Effects = %+v, want the hook's failure and not synced", written.Effects)
	}
}

func TestSync_NoHook_ReportsZeroEffects(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	written, _, err := Issues(s, nil).Create(newIssueID, "", buildIssue(t, "open"))
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	issues, _ := newEditedIssue(t)
	calls := 0
	edited, err := issues.Edit(editedIssueID, "", retitle("Fix the lexer", &calls))
	if err != nil {
		t.Fatalf("Edit failed: %v", err)
	}
	movedIssues, _ := newMoveIssues(t)
	moved, _, err := movedIssues.Move("login", Destination{ID: "auth-flow"}, "")
	if err != nil {
		t.Fatalf("Move failed: %v", err)
	}
	removedIssues, _ := newRemoveIssues(t)
	removed, _, err := removedIssues.Remove("child", "", false)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	for name, effects := range map[string]Effects{
		"Create": written.Effects,
		"Edit":   edited.Effects,
		"Move":   moved.Effects,
		"Remove": removed.Effects,
	} {
		if effects != (Effects{}) {
			t.Errorf("%s Effects = %+v, want zero Effects", name, effects)
		}
	}
}

func TestWithSync_LeavesTheOriginalKindWithoutAHook(t *testing.T) {
	t.Parallel()
	issues, _ := newEditedIssue(t)
	sync := &countingSync{isSynced: true}
	synced := issues.WithSync(sync.hook)
	calls := 0

	_, err := issues.Edit(editedIssueID, "", retitle("Fix the lexer", &calls))

	if err != nil || sync.calls != 0 || synced.sync == nil {
		t.Errorf("Edit = %v, hook calls = %d; want the original Kind to run no hook", err, sync.calls)
	}
}
