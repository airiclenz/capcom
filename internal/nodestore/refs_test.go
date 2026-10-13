package nodestore

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/tree"
)

func TestCodecs_ListTheirReferenceFields(t *testing.T) {
	t.Parallel()
	keys := func(fields []string) string { return strings.Join(fields, ",") }
	var issueKeys, noteKeys, selfRefused []string
	for _, field := range issueCodec(newStore(t).Config).refs {
		issueKeys = append(issueKeys, field.key)
		if field.isSelfRefused {
			selfRefused = append(selfRefused, field.key)
		}
	}
	for _, field := range noteCodec().refs {
		noteKeys = append(noteKeys, field.key)
		if field.isSelfRefused {
			selfRefused = append(selfRefused, field.key)
		}
	}

	if keys(issueKeys) != "deps,related" || keys(noteKeys) != "superseded-by" || keys(selfRefused) != "superseded-by" {
		t.Errorf("issue refs = %v, note refs = %v, self-refused = %v; want [deps related], [superseded-by], [superseded-by]",
			issueKeys, noteKeys, selfRefused)
	}
}

func TestRenameRefs(t *testing.T) {
	t.Parallel()
	names := []string{"old-id", "older-id"}

	tests := []struct {
		name        string
		entries     []string
		want        []string
		wantRenamed bool
	}{
		{name: "none named", entries: []string{"other", "other"}, want: []string{"other", "other"}},
		{name: "by id", entries: []string{"other", "old-id"}, want: []string{"other", "new-id"}, wantRenamed: true},
		{name: "by alias", entries: []string{"older-id"}, want: []string{"new-id"}, wantRenamed: true},
		{name: "before the new id", entries: []string{"old-id", "new-id"}, want: []string{"new-id"}, wantRenamed: true},
		{name: "after the new id", entries: []string{"new-id", "older-id"}, want: []string{"new-id"}, wantRenamed: true},
		{name: "id and alias", entries: []string{"old-id", "other", "older-id"}, want: []string{"new-id", "other"}, wantRenamed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, isRenamed := renameRefs(test.entries, names, "new-id")
			if !slices.Equal(got, test.want) || isRenamed != test.wantRenamed {
				t.Errorf("renameRefs(%v) = %v, %t; want %v, %t", test.entries, got, isRenamed, test.want, test.wantRenamed)
			}
		})
	}
}

// unreadSwap is one way of swapping a node file after the tree load for one
// nodefile.ReadRegularFile refuses unread: swap replaces the regular node file at path, whose
// content is content.
type unreadSwap struct {
	name string
	swap func(t *testing.T, path string, content string)
}

// unreadSwaps are the swaps nodefile.ReadRegularFile refuses: a symlink to a regular file holding
// the same content, a named pipe, and a regular file over 1 MiB written in place.
func unreadSwaps() []unreadSwap {
	return []unreadSwap{
		{name: "symlink", swap: func(t *testing.T, path string, content string) {
			if runtime.GOOS == "windows" {
				t.Skip("symlink tests do not run on Windows")
			}
			if os.Geteuid() == 0 {
				t.Skip("symlink tests do not run as root")
			}
			target := filepath.Join(t.TempDir(), filepath.Base(path))
			if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			removeSwapped(t, path)
			if err := os.Symlink(target, path); err != nil {
				t.Fatalf("Symlink(%s): %v", path, err)
			}
		}},
		{name: "fifo", swap: func(t *testing.T, path string, _ string) {
			removeSwapped(t, path)
			makeFIFO(t, path)
		}},
		{name: "over 1 MiB", swap: func(t *testing.T, path string, content string) {
			oversized := content + strings.Repeat("x", 1<<20)
			if err := os.WriteFile(path, []byte(oversized), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
		}},
	}
}

// removeSwapped removes the node file at path, so a symlink or named pipe can take its place.
func removeSwapped(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}

// lstatSwapped returns the os.Lstat of the swapped file at path.
func lstatSwapped(t *testing.T, path string) fs.FileInfo {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	return info
}

// assertSwappedAfterLoadIsUnread loads kind's tree, swaps the file name below dir, holding content,
// as swap says, then plans and applies the rename of old-name to new-name through the engine and
// checks that the swapped file is listed as unread and left as it was.
func assertSwappedAfterLoadIsUnread[T any](t *testing.T, kind *Kind[T], dir string, name string, content string, swap unreadSwap) {
	t.Helper()
	nodes, _, err := kind.load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	swappedPath := filepath.Join(dir, name)
	swap.swap(t, swappedPath, content)
	swapped := lstatSwapped(t, swappedPath)
	node, err := nodes.Resolve("old-name")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	move, err := kind.planNodeMove(nodes, node, tree.Destination{ID: "new-name"})
	if err != nil {
		t.Fatalf("planNodeMove: %v", err)
	}
	if err := move.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if want := []string{swappedPath}; !slices.Equal(move.unread, want) {
		t.Errorf("unread = %v, want %v", move.unread, want)
	}
	if got := lstatSwapped(t, swappedPath); got.Mode() != swapped.Mode() || got.Size() != swapped.Size() {
		t.Errorf("swapped file = %v %d bytes, want it as swapped: %v %d bytes",
			got.Mode(), got.Size(), swapped.Mode(), swapped.Size())
	}
}

func TestPlanNodeMove_RenameIssue_FileSwappedAfterLoad_ListsItAsUnreadAndLeavesIt(t *testing.T) {
	t.Parallel()
	for _, swap := range unreadSwaps() {
		t.Run(swap.name, func(t *testing.T) {
			t.Parallel()
			dependent := refIssue("deps: [old-name]\n", "")
			issues, dir := newRefIssues(t, map[string]string{
				"old-name.md":     refIssue("", ""),
				"swapped-work.md": dependent,
			})
			assertSwappedAfterLoadIsUnread(t, issues, dir, "swapped-work.md", dependent, swap)
		})
	}
}

func TestPlanNodeMove_RenameNote_FileSwappedAfterLoad_ListsItAsUnreadAndLeavesIt(t *testing.T) {
	t.Parallel()
	for _, swap := range unreadSwaps() {
		t.Run(swap.name, func(t *testing.T) {
			t.Parallel()
			successor := refNote("Swapped", "superseded-by: [old-name]\n", "")
			notes, dir := newRefNotes(t, map[string]string{
				"old-name.md": refNote("Old rule", "", ""),
				"swapped.md":  successor,
			})
			assertSwappedAfterLoadIsUnread(t, notes, dir, "swapped.md", successor, swap)
		})
	}
}

// assertRemovalSwappedAfterLoadIsUnread loads kind's tree, swaps the file name below dir, holding
// content, as swap says, then plans and applies the removal of gone-name through the engine and
// checks that the swapped file is listed as unread and left as it was.
func assertRemovalSwappedAfterLoadIsUnread[T any](t *testing.T, kind *Kind[T], dir string, name string, content string, swap unreadSwap) {
	t.Helper()
	nodes, _, err := kind.load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	swappedPath := filepath.Join(dir, name)
	swap.swap(t, swappedPath, content)
	swapped := lstatSwapped(t, swappedPath)
	node, err := nodes.Resolve("gone-name")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	removal, err := kind.planNodeRemoval(nodes, node, "", false)
	if err != nil {
		t.Fatalf("planNodeRemoval: %v", err)
	}
	if _, err := removal.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if want := []string{swappedPath}; !slices.Equal(removal.unread, want) {
		t.Errorf("unread = %v, want %v", removal.unread, want)
	}
	if got := lstatSwapped(t, swappedPath); got.Mode() != swapped.Mode() || got.Size() != swapped.Size() {
		t.Errorf("swapped file = %v %d bytes, want it as swapped: %v %d bytes",
			got.Mode(), got.Size(), swapped.Mode(), swapped.Size())
	}
}

func TestPlanNodeRemoval_Issue_FileSwappedAfterLoad_ListsItAsUnreadAndLeavesIt(t *testing.T) {
	t.Parallel()
	for _, swap := range unreadSwaps() {
		t.Run(swap.name, func(t *testing.T) {
			t.Parallel()
			dependent := refIssue("deps: [gone-name]\n", "")
			issues, dir := newRefIssues(t, map[string]string{
				"gone-name.md":    refIssue("", ""),
				"swapped-work.md": dependent,
			})
			assertRemovalSwappedAfterLoadIsUnread(t, issues, dir, "swapped-work.md", dependent, swap)
		})
	}
}

func TestPlanNodeRemoval_Note_FileSwappedAfterLoad_ListsItAsUnreadAndLeavesIt(t *testing.T) {
	t.Parallel()
	for _, swap := range unreadSwaps() {
		t.Run(swap.name, func(t *testing.T) {
			t.Parallel()
			successor := refNote("Swapped", "superseded-by: [gone-name]\n", "")
			notes, dir := newRefNotes(t, map[string]string{
				"gone-name.md": refNote("Gone rule", "", ""),
				"swapped.md":   successor,
			})
			assertRemovalSwappedAfterLoadIsUnread(t, notes, dir, "swapped.md", successor, swap)
		})
	}
}

func TestPlanNodeRemoval_Note_StaleIfMatch_ConflictsBeforeReadingOtherNotes(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("file permission tests do not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("file permission tests do not run as root")
	}
	notes, dir := newRefNotes(t, map[string]string{
		"old-rule.md":   refNote("Old rule", "", ""),
		"unreadable.md": refNote("Unreadable", "superseded-by: [old-rule]\n", ""),
	})
	nodes, _, err := notes.load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	node, err := nodes.Resolve("old-rule")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	unreadable := filepath.Join(dir, "unreadable.md")
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o644) })

	_, err = notes.planNodeRemoval(nodes, node, "0000000000000000", false)

	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Path != ".capcom/knowledge/old-rule.md" {
		t.Errorf("err = %v, want a *ConflictError at old-rule.md before any other Note is read", err)
	}
}
