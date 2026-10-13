package issue_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/tree"
)

// moveFile is the content of a canonical issue file with the given deps and aliases (flow lists
// without brackets, "" for none) and body.
func moveFile(deps string, aliases string, body string) string {
	var front strings.Builder
	front.WriteString("---\ntitle: Moved\nstatus: open\ntype: task\npriority: 2\n")
	if deps != "" {
		front.WriteString("deps: [" + deps + "]\n")
	}
	if aliases != "" {
		front.WriteString("aliases: [" + aliases + "]\n")
	}
	front.WriteString("created: 2026-10-11T09:00:00Z\n---\n")
	return front.String() + body
}

// newMoveTree writes files (slash paths to content) into a fresh issue directory and returns it.
func newMoveTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	return dir
}

// loadMoveTree loads the issue tree in dir, reading each node's Aliases from its file.
func loadMoveTree(t *testing.T, dir string) *tree.Tree {
	t.Helper()
	issues, err := tree.Load(dir, func(path string) ([]string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		parsed, err := issue.Parse(path, data)
		if err != nil {
			return nil, nil
		}
		return parsed.Aliases, nil
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return issues
}

// moveIssue plans and applies one move in dir, failing the test on any error.
func moveIssue(t *testing.T, dir string, id string, to tree.Destination) *issue.Move {
	t.Helper()
	move, err := issue.PlanMove(loadMoveTree(t, dir), id, to)
	if err != nil {
		t.Fatalf("PlanMove(%s): %v", id, err)
	}
	if err := move.Apply(); err != nil {
		t.Fatalf("Apply(%s): %v", id, err)
	}
	return move
}

// parseMoveFile parses the issue file at the slash path name below dir.
func parseMoveFile(t *testing.T, dir string, name string) issue.Issue {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	parsed, err := issue.Parse(path, data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return parsed
}

// snapshotTree returns every file (slash path to content) and directory (slash path ending in /)
// below dir.
func snapshotTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return err
		}
		name := filepath.ToSlash(strings.TrimPrefix(path, dir+string(filepath.Separator)))
		if entry.IsDir() {
			snapshot[name+"/"] = ""
			return nil
		}
		data, err := os.ReadFile(path)
		snapshot[name] = string(data)
		return err
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	return snapshot
}

// makeSymlink creates a symbolic link at link pointing to target, skipping the test on Windows and
// as root.
func makeSymlink(t *testing.T, target string, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests do not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("symlink tests do not run as root")
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink(%s): %v", link, err)
	}
}

// unreadSwap is one way of swapping an issue file after Load for one nodefile.ReadNodeFile refuses
// unread: swap replaces the regular issue file at path, whose content is content.
type unreadSwap struct {
	name string
	swap func(t *testing.T, path string, content string)
}

// unreadSwaps are the swaps nodefile.ReadNodeFile refuses: a symlink to a regular file holding the
// same content, a named pipe, and a regular file over 1 MiB written in place.
func unreadSwaps() []unreadSwap {
	return []unreadSwap{
		{name: "symlink", swap: func(t *testing.T, path string, content string) {
			target := filepath.Join(t.TempDir(), filepath.Base(path))
			if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			removeSwapped(t, path)
			makeSymlink(t, target, path)
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

// removeSwapped removes the issue file at path, so a symlink or named pipe can take its place.
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

func TestPlanMove_RenameTwice_KeepsAliasChain(t *testing.T) {
	dir := newMoveTree(t, map[string]string{"first-name.md": moveFile("", "", "\nBody.\n")})

	moveIssue(t, dir, "first-name", tree.Destination{ID: "second-name"})
	moved := moveIssue(t, dir, "second-name", tree.Destination{ID: "third-name"})

	parsed := parseMoveFile(t, dir, "third-name.md")
	if want := []string{"first-name", "second-name"}; !slices.Equal(parsed.Aliases, want) {
		t.Errorf("aliases = %v, want %v", parsed.Aliases, want)
	}
	if moved.NewPath != filepath.Join(dir, "third-name.md") || parsed.Body != "\nBody.\n" {
		t.Errorf("path = %s body = %q", moved.NewPath, parsed.Body)
	}
	if node, err := loadMoveTree(t, dir).Resolve("first-name"); err != nil || node.ID != "third-name" {
		t.Errorf("Resolve(first-name) = %v, %v", node.ID, err)
	}
}

func TestPlanMove_RenameBackToOwnAlias_SwapsIt(t *testing.T) {
	dir := newMoveTree(t, map[string]string{"new-name.md": moveFile("", "old-name, apogee-kk0.4", "")})

	moveIssue(t, dir, "new-name", tree.Destination{ID: "old-name"})

	parsed := parseMoveFile(t, dir, "old-name.md")
	if want := []string{"apogee-kk0.4", "new-name"}; !slices.Equal(parsed.Aliases, want) {
		t.Errorf("aliases = %v, want %v", parsed.Aliases, want)
	}
}

func TestPlanMove_Rename_RewritesDepsInOtherFiles(t *testing.T) {
	dir := newMoveTree(t, map[string]string{
		"login-flow.md":                  moveFile("", "apogee-kk0.4", ""),
		"session-work.md":                moveFile("other-issue, login-flow, apogee-kk0.4", "", "\nNeeds login-flow first.\n"),
		"other-issue.md":                 moveFile("", "", ""),
		"other-issue/nested-child.md":    moveFile("apogee-kk0.4", "", ""),
		"other-issue/untouched-child.md": moveFile("other-issue", "", ""),
	})
	untouched := snapshotTree(t, dir)["other-issue/untouched-child.md"]

	moveIssue(t, dir, "login-flow", tree.Destination{ID: "auth-flow"})

	session := parseMoveFile(t, dir, "session-work.md")
	if want := []string{"other-issue", "auth-flow"}; !slices.Equal(session.Deps, want) {
		t.Errorf("session-work deps = %v, want %v", session.Deps, want)
	}
	if session.Body != "\nNeeds login-flow first.\n" {
		t.Errorf("session-work body = %q, want it unchanged", session.Body)
	}
	if nested := parseMoveFile(t, dir, "other-issue/nested-child.md"); !slices.Equal(nested.Deps, []string{"auth-flow"}) {
		t.Errorf("nested-child deps = %v", nested.Deps)
	}
	if got := snapshotTree(t, dir)["other-issue/untouched-child.md"]; got != untouched {
		t.Errorf("untouched-child = %q, want it byte for byte as before", got)
	}
}

func TestPlanMove_UnparseableIssueFile_DestinationCheckedFirst(t *testing.T) {
	dir := newMoveTree(t, map[string]string{
		"moved-issue.md":           "no front matter here\n",
		"moved-issue/child-one.md": moveFile("", "", ""),
		"taken-issue.md":           moveFile("", "", ""),
	})
	issues := loadMoveTree(t, dir)
	cases := []struct {
		name    string
		to      tree.Destination
		wantErr error
	}{
		{name: "invalid id", to: tree.Destination{ID: "Bad_Name"}, wantErr: tree.ErrInvalidID},
		{name: "taken id", to: tree.Destination{ID: "taken-issue"}, wantErr: tree.ErrDuplicate},
		{name: "own subtree", to: tree.Destination{ID: "moved-issue", ParentID: "child-one"}, wantErr: tree.ErrMoveIntoSubtree},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := issue.PlanMove(issues, "moved-issue", tc.to)

			var parseFailure *issue.Error
			if !errors.Is(err, tc.wantErr) || errors.As(err, &parseFailure) {
				t.Errorf("err = %v, want %v and no parse error", err, tc.wantErr)
			}
		})
	}
	t.Run("valid rename", func(t *testing.T) {
		_, err := issue.PlanMove(issues, "moved-issue", tree.Destination{ID: "renamed-issue"})

		var parseFailure *issue.Error
		if !errors.As(err, &parseFailure) {
			t.Errorf("err = %v, want the moved issue's parse error", err)
		}
	})
}

func TestPlanMove_Rename_RewritesRelatedInOtherFiles(t *testing.T) {
	relatedFile := func(related string) string {
		return strings.Replace(moveFile("", "", "\nSee login-flow.\n"), "created:", "related: ["+related+"]\ncreated:", 1)
	}
	dir := newMoveTree(t, map[string]string{
		"login-flow.md":      moveFile("", "apogee-kk0.4", ""),
		"by-id.md":           relatedFile("other-issue, login-flow"),
		"by-alias.md":        relatedFile("apogee-kk0.4"),
		"by-both.md":         relatedFile("login-flow, apogee-kk0.4"),
		"other-issue.md":     moveFile("", "", ""),
		"untouched-issue.md": relatedFile("other-issue"),
	})
	untouched := snapshotTree(t, dir)["untouched-issue.md"]

	move := moveIssue(t, dir, "login-flow", tree.Destination{ID: "auth-flow"})

	wants := map[string][]string{
		"by-id.md":    {"other-issue", "auth-flow"},
		"by-alias.md": {"auth-flow"},
		"by-both.md":  {"auth-flow"},
	}
	for name, want := range wants {
		parsed := parseMoveFile(t, dir, name)
		if !slices.Equal(parsed.Related, want) || parsed.Deps != nil || parsed.Body != "\nSee login-flow.\n" {
			t.Errorf("%s related = %v deps = %v body = %q, want related %v and the rest unchanged",
				name, parsed.Related, parsed.Deps, parsed.Body, want)
		}
	}
	if got := snapshotTree(t, dir)["untouched-issue.md"]; got != untouched {
		t.Errorf("untouched-issue = %q, want it byte for byte as before", got)
	}
	if len(move.Unread) != 0 {
		t.Errorf("unread = %v, want none", move.Unread)
	}
}

func TestPlanMove_Rename_NodeFileSwappedAfterLoad_ListsItAsUnreadAndLeavesIt(t *testing.T) {
	for _, tc := range unreadSwaps() {
		t.Run(tc.name, func(t *testing.T) {
			dependent := moveFile("old-name", "", "")
			dir := newMoveTree(t, map[string]string{
				"old-name.md":     moveFile("", "", ""),
				"swapped-work.md": dependent,
			})
			issues := loadMoveTree(t, dir)
			swappedPath := filepath.Join(dir, "swapped-work.md")
			tc.swap(t, swappedPath, dependent)
			swapped := lstatSwapped(t, swappedPath)

			move, err := issue.PlanMove(issues, "old-name", tree.Destination{ID: "new-name"})
			if err != nil {
				t.Fatalf("PlanMove: %v", err)
			}
			if err := move.Apply(); err != nil {
				t.Fatalf("Apply: %v", err)
			}

			if want := []string{swappedPath}; !slices.Equal(move.Unread, want) {
				t.Errorf("Unread = %v, want %v", move.Unread, want)
			}
			if got := lstatSwapped(t, swappedPath); got.Mode() != swapped.Mode() || got.Size() != swapped.Size() {
				t.Errorf("swapped file = %v %d bytes, want it as swapped: %v %d bytes",
					got.Mode(), got.Size(), swapped.Mode(), swapped.Size())
			}
		})
	}
}
