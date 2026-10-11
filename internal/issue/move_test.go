package issue_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/tree"
)

// errInjected is the failure an injected rename reports.
var errInjected = errors.New("injected rename failure")

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
func moveIssue(t *testing.T, dir string, id string, to issue.Destination) *issue.Move {
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

func TestPlanMove_RenameTwice_KeepsAliasChain(t *testing.T) {
	dir := newMoveTree(t, map[string]string{"first-name.md": moveFile("", "", "\nBody.\n")})

	moveIssue(t, dir, "first-name", issue.Destination{ID: "second-name"})
	moved := moveIssue(t, dir, "second-name", issue.Destination{ID: "third-name"})

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

	moveIssue(t, dir, "new-name", issue.Destination{ID: "old-name"})

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

	moveIssue(t, dir, "login-flow", issue.Destination{ID: "auth-flow"})

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

func TestPlanMove_ReparentWithChildren_MovesSubtree(t *testing.T) {
	dir := newMoveTree(t, map[string]string{
		"old-parent.md":                                   moveFile("", "", ""),
		"old-parent/moved-issue.md":                       moveFile("", "", ""),
		"old-parent/moved-issue/child-one.md":             moveFile("", "", ""),
		"old-parent/moved-issue/child-one/grand-child.md": moveFile("", "", ""),
		"new-parent.md":                                   moveFile("", "", ""),
	})

	moveIssue(t, dir, "moved-issue", issue.Destination{ID: "moved-issue", ParentID: "new-parent"})

	issues := loadMoveTree(t, dir)
	for id, parentID := range map[string]string{"moved-issue": "new-parent", "child-one": "moved-issue", "grand-child": "child-one"} {
		if node, isFound := issues.Lookup(id); !isFound || node.ParentID != parentID {
			t.Errorf("%s: found = %t parent = %q, want %q", id, isFound, node.ParentID, parentID)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "old-parent")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("emptied old-parent directory: stat error = %v, want it removed", err)
	}
}

func TestPlanMove_ToTopOfTree_MovesIt(t *testing.T) {
	dir := newMoveTree(t, map[string]string{
		"old-parent.md":               moveFile("", "", ""),
		"old-parent/moved-issue.md":   moveFile("", "", ""),
		"old-parent/staying-child.md": moveFile("", "", ""),
	})

	moved := moveIssue(t, dir, "moved-issue", issue.Destination{ID: "moved-issue", ParentID: ""})

	if moved.NewPath != filepath.Join(dir, "moved-issue.md") {
		t.Errorf("NewPath = %s", moved.NewPath)
	}
	if _, err := os.Stat(filepath.Join(dir, "old-parent", "staying-child.md")); err != nil {
		t.Errorf("staying-child: %v", err)
	}
}

func TestPlanMove_UnderItselfOrDescendant_Refused(t *testing.T) {
	dir := newMoveTree(t, map[string]string{
		"moved-issue.md":                       moveFile("", "", ""),
		"moved-issue/child-one.md":             moveFile("", "", ""),
		"moved-issue/child-one/grand-child.md": moveFile("", "", ""),
	})
	before := snapshotTree(t, dir)

	for _, parentID := range []string{"moved-issue", "child-one", "grand-child"} {
		_, err := issue.PlanMove(loadMoveTree(t, dir), "moved-issue", issue.Destination{ID: "moved-issue", ParentID: parentID})

		if !errors.Is(err, issue.ErrMoveIntoSubtree) {
			t.Errorf("parent %s: err = %v, want ErrMoveIntoSubtree", parentID, err)
		}
	}
	if after := snapshotTree(t, dir); !reflect.DeepEqual(after, before) {
		t.Errorf("tree changed:\n%v\nwant\n%v", after, before)
	}
}

func TestPlanMove_RenameOntoTakenIDOrAlias_Refused(t *testing.T) {
	dir := newMoveTree(t, map[string]string{
		"moved-issue.md": moveFile("", "", ""),
		"taken-issue.md": moveFile("", "taken-alias", ""),
	})

	for _, newID := range []string{"taken-issue", "taken-alias"} {
		_, err := issue.PlanMove(loadMoveTree(t, dir), "moved-issue", issue.Destination{ID: newID})

		var taken *issue.TakenError
		if !errors.As(err, &taken) || taken.Owner.ID != "taken-issue" || !errors.Is(err, tree.ErrDuplicate) {
			t.Errorf("%s: err = %v, want a TakenError owned by taken-issue", newID, err)
		}
	}
}

func TestPlanMove_RenameInvalidOrUnknown_Refused(t *testing.T) {
	dir := newMoveTree(t, map[string]string{"moved-issue.md": moveFile("", "", "")})
	issues := loadMoveTree(t, dir)

	if _, err := issue.PlanMove(issues, "moved-issue", issue.Destination{ID: "Bad_Name"}); !errors.Is(err, tree.ErrInvalidID) {
		t.Errorf("invalid id: err = %v", err)
	}
	if _, err := issue.PlanMove(issues, "moved-issue", issue.Destination{ID: "moved-issue", ParentID: "no-such-issue"}); !errors.Is(err, tree.ErrNotFound) {
		t.Errorf("unknown parent: err = %v", err)
	}
	if _, err := issue.PlanMove(issues, "no-such-issue", issue.Destination{ID: "other-name"}); !errors.Is(err, tree.ErrNotFound) {
		t.Errorf("unknown issue: err = %v", err)
	}
}

func TestMoveApply_SecondRenameFails_LeavesTreeUnchanged(t *testing.T) {
	dir := newMoveTree(t, map[string]string{
		"old-parent.md":                     moveFile("", "", ""),
		"old-parent/moved-issue.md":         moveFile("", "former-name", "\nBody.\n"),
		"old-parent/moved-issue/kid-one.md": moveFile("moved-issue", "", ""),
		"dependent-issue.md":                moveFile("former-name", "", ""),
		"new-parent.md":                     moveFile("", "", ""),
	})
	before := snapshotTree(t, dir)
	move, err := issue.PlanMove(loadMoveTree(t, dir), "moved-issue", issue.Destination{ID: "renamed-issue", ParentID: "new-parent"})
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	renames := 0
	issue.SetMoveRename(move, func(oldPath string, newPath string) error {
		renames++
		if renames == 2 {
			return errInjected
		}
		return os.Rename(oldPath, newPath)
	})

	err = move.Apply()

	if !errors.Is(err, errInjected) {
		t.Fatalf("Apply err = %v, want the injected failure", err)
	}
	if after := snapshotTree(t, dir); !reflect.DeepEqual(after, before) {
		t.Errorf("tree changed:\n%v\nwant\n%v", after, before)
	}
}
