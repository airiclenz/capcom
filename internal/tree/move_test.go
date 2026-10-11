package tree_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/tree"
)

// errInjected is the failure an injected rename reports.
var errInjected = errors.New("injected rename failure")

// issueKind and noteKind are the noun phrases issue and Note moves pass to PlanMove.
const (
	issueKind = "an issue"
	noteKind  = "a note"
)

// moveNode plans and applies one move in dir, failing the test on any error.
func moveNode(t *testing.T, dir string, id string, to tree.Destination) *tree.Move {
	t.Helper()
	move, err := mustLoad(t, dir, nil).PlanMove(id, to, issueKind)
	if err != nil {
		t.Fatalf("PlanMove(%s): %v", id, err)
	}
	if err := move.Apply(); err != nil {
		t.Fatalf("Apply(%s): %v", id, err)
	}
	return move
}

// snapshotDir returns every file (slash path to content) and directory (slash path ending in /)
// below dir.
func snapshotDir(t *testing.T, dir string) map[string]string {
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

func TestPlanMove_RenameWithChildren_RenamesChildrenDirAndRewritesFirst(t *testing.T) {
	dir := t.TempDir()
	writeNodes(t, dir, "moved-node.md", "moved-node/child-one.md")
	move, err := mustLoad(t, dir, nil).PlanMove("moved-node", tree.Destination{ID: "renamed-node"}, issueKind)
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	move.AddRewrite(tree.Rewrite{
		Path:     filepath.Join(dir, "moved-node.md"),
		Original: []byte("x\n"),
		Content:  []byte("rewritten\n"),
	})

	if err := move.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	want := map[string]string{
		"renamed-node.md":           "rewritten\n",
		"renamed-node/":             "",
		"renamed-node/child-one.md": "x\n",
	}
	if got := snapshotDir(t, dir); !reflect.DeepEqual(got, want) {
		t.Errorf("tree = %v, want %v", got, want)
	}
	if move.NewPath != filepath.Join(dir, "renamed-node.md") || move.Node.ID != "moved-node" {
		t.Errorf("NewPath = %s Node.ID = %s", move.NewPath, move.Node.ID)
	}
}

func TestPlanMove_ReparentWithChildren_MovesSubtreeAndRemovesEmptyDir(t *testing.T) {
	dir := t.TempDir()
	writeNodes(t, dir,
		"old-parent.md",
		"old-parent/moved-node.md",
		"old-parent/moved-node/child-one.md",
		"old-parent/moved-node/child-one/grand-child.md",
		"new-parent.md",
	)

	moveNode(t, dir, "moved-node", tree.Destination{ID: "moved-node", ParentID: "new-parent"})

	nodes := mustLoad(t, dir, nil)
	for id, parentID := range map[string]string{"moved-node": "new-parent", "child-one": "moved-node", "grand-child": "child-one"} {
		if node, isFound := nodes.Lookup(id); !isFound || node.ParentID != parentID {
			t.Errorf("%s: found = %t parent = %q, want %q", id, isFound, node.ParentID, parentID)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "old-parent")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("emptied old-parent directory: stat error = %v, want it removed", err)
	}
}

func TestPlanMove_ToTopOfTree_KeepsOldDirWithOtherChildren(t *testing.T) {
	dir := t.TempDir()
	writeNodes(t, dir, "old-parent.md", "old-parent/moved-node.md", "old-parent/staying-child.md")

	moved := moveNode(t, dir, "moved-node", tree.Destination{ID: "moved-node", ParentID: ""})

	if moved.NewPath != filepath.Join(dir, "moved-node.md") {
		t.Errorf("NewPath = %s", moved.NewPath)
	}
	if _, err := os.Stat(filepath.Join(dir, "old-parent", "staying-child.md")); err != nil {
		t.Errorf("staying-child: %v", err)
	}
}

func TestPlanMove_UnderItselfOrDescendant_RefusedNamingKind(t *testing.T) {
	dir := t.TempDir()
	writeNodes(t, dir, "moved-node.md", "moved-node/child-one.md", "moved-node/child-one/grand-child.md")
	nodes := mustLoad(t, dir, nil)
	before := snapshotDir(t, dir)

	for _, kind := range []string{issueKind, noteKind} {
		for _, parentID := range []string{"moved-node", "child-one", "grand-child"} {
			_, err := nodes.PlanMove("moved-node", tree.Destination{ID: "moved-node", ParentID: parentID}, kind)

			want := "cannot move " + kind + " below itself or its own descendants: moved-node cannot go below " + parentID
			if !errors.Is(err, tree.ErrMoveIntoSubtree) || err.Error() != want {
				t.Errorf("%s below %s: err = %v, want %q", kind, parentID, err, want)
			}
		}
	}
	if after := snapshotDir(t, dir); !reflect.DeepEqual(after, before) {
		t.Errorf("tree changed:\n%v\nwant\n%v", after, before)
	}
}

func TestPlanMove_RenameOntoTakenIDOrAlias_RefusedNamingKind(t *testing.T) {
	dir := t.TempDir()
	writeNodes(t, dir, "moved-node.md", "taken-node.md")
	nodes := mustLoad(t, dir, aliasesByFile(map[string][]string{"taken-node.md": {"taken-alias"}}))
	cases := []struct {
		newID string
		kind  string
		want  string
	}{
		{newID: "taken-node", kind: issueKind, want: `duplicate id "taken-node": already taken by issue taken-node`},
		{newID: "taken-alias", kind: issueKind, want: `duplicate id "taken-alias": already an alias of issue taken-node`},
		{newID: "taken-node", kind: noteKind, want: `duplicate id "taken-node": already taken by note taken-node`},
		{newID: "taken-alias", kind: noteKind, want: `duplicate id "taken-alias": already an alias of note taken-node`},
	}

	for _, tc := range cases {
		_, err := nodes.PlanMove("moved-node", tree.Destination{ID: tc.newID}, tc.kind)

		var taken *tree.TakenError
		if !errors.As(err, &taken) || taken.Owner.ID != "taken-node" || !errors.Is(err, tree.ErrDuplicate) || err.Error() != tc.want {
			t.Errorf("%s as %s: err = %v, want %q", tc.newID, tc.kind, err, tc.want)
		}
	}
}

func TestPlanMove_RenameInvalidOrUnknown_Refused(t *testing.T) {
	dir := t.TempDir()
	writeNodes(t, dir, "moved-node.md")
	nodes := mustLoad(t, dir, nil)
	cases := []struct {
		name    string
		id      string
		to      tree.Destination
		wantErr error
	}{
		{name: "invalid id", id: "moved-node", to: tree.Destination{ID: "Bad_Name"}, wantErr: tree.ErrInvalidID},
		{name: "unknown parent", id: "moved-node", to: tree.Destination{ID: "moved-node", ParentID: "no-such-node"}, wantErr: tree.ErrNotFound},
		{name: "unknown node", id: "no-such-node", to: tree.Destination{ID: "other-name"}, wantErr: tree.ErrNotFound},
	}

	for _, tc := range cases {
		if _, err := nodes.PlanMove(tc.id, tc.to, issueKind); !errors.Is(err, tc.wantErr) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.wantErr)
		}
	}
}

func TestMoveApply_SecondRenameFails_LeavesTreeUnchanged(t *testing.T) {
	dir := t.TempDir()
	writeNodes(t, dir,
		"old-parent.md",
		"old-parent/moved-node.md",
		"old-parent/moved-node/kid-one.md",
		"dependent-node.md",
		"new-parent.md",
	)
	before := snapshotDir(t, dir)
	move, err := mustLoad(t, dir, nil).PlanMove("moved-node", tree.Destination{ID: "renamed-node", ParentID: "new-parent"}, issueKind)
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	for _, name := range []string{"old-parent/moved-node.md", "dependent-node.md"} {
		move.AddRewrite(tree.Rewrite{
			Path:     filepath.Join(dir, filepath.FromSlash(name)),
			Original: []byte("x\n"),
			Content:  []byte("rewritten\n"),
		})
	}
	renames := 0
	tree.SetMoveRename(move, func(oldPath string, newPath string) error {
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
	if after := snapshotDir(t, dir); !reflect.DeepEqual(after, before) {
		t.Errorf("tree changed:\n%v\nwant\n%v", after, before)
	}
}

func TestRenameAliases(t *testing.T) {
	cases := []struct {
		name    string
		aliases []string
		oldID   string
		newID   string
		want    []string
	}{
		{name: "rename appends old id", aliases: []string{"first-name"}, oldID: "second-name", newID: "third-name", want: []string{"first-name", "second-name"}},
		{name: "rename back to own alias swaps it", aliases: []string{"old-name", "legacy-name"}, oldID: "new-name", newID: "old-name", want: []string{"legacy-name", "new-name"}},
		{name: "same id leaves aliases", aliases: []string{"first-name"}, oldID: "same-name", newID: "same-name", want: []string{"first-name"}},
		{name: "no aliases yet", aliases: nil, oldID: "old-name", newID: "new-name", want: []string{"old-name"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := slices.Clone(tc.aliases)

			got := tree.RenameAliases(tc.aliases, tc.oldID, tc.newID)

			if !slices.Equal(got, tc.want) || !slices.Equal(tc.aliases, original) {
				t.Errorf("RenameAliases(%v) = %v, want %v and the input unchanged", original, got, tc.want)
			}
		})
	}
}
