package tree_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/airiclenz/capcom/internal/tree"
)

// errInjectedRemove is the failure an injected remove reports.
var errInjectedRemove = errors.New("injected remove failure")

// planRemove loads dir and plans the removal of the issue id, failing the test on any error.
func planRemove(t *testing.T, dir string, id string) *tree.Removal {
	t.Helper()
	removal, err := mustLoad(t, dir, nil).PlanRemove(id, issueKind)
	if err != nil {
		t.Fatalf("PlanRemove(%s): %v", id, err)
	}
	return removal
}

// failRemoveOf makes removal's removes fail for the path ending in name and succeed otherwise.
func failRemoveOf(removal *tree.Removal, name string) {
	tree.SetRemovalRemove(removal, func(path string) error {
		if filepath.Base(path) == name {
			return errInjectedRemove
		}
		return os.Remove(path)
	})
}

func TestPlanRemove_WithChildren_RefusedNamingKindAndCount(t *testing.T) {
	dir := t.TempDir()
	writeNodes(t, dir, "parent-node.md", "parent-node/kid-one.md", "parent-node/kid-two.md")

	_, err := mustLoad(t, dir, nil).PlanRemove("parent-node", noteKind)

	var refusal *tree.ChildrenError
	if !errors.As(err, &refusal) || refusal.Count != 2 || err.Error() != "note parent-node has 2 child notes; remove or move them first" {
		t.Errorf("PlanRemove of a parent = %v; want a *tree.ChildrenError counting 2 child notes", err)
	}
}

func TestPlanRemove_ChildrenDirHoldsAFile_RefusedNamingTheDir(t *testing.T) {
	dir := t.TempDir()
	writeNodes(t, dir, "lone-node.md", "lone-node/.keep")

	_, err := mustLoad(t, dir, nil).PlanRemove("lone-node", issueKind)

	var refusal *tree.ChildrenDirError
	if !errors.As(err, &refusal) || refusal.Dir != filepath.Join(dir, "lone-node") ||
		err.Error() != "the issue's children directory still holds 1 entry; remove them first" {
		t.Errorf("PlanRemove with a stray file = %v; want a *tree.ChildrenDirError naming the directory", err)
	}
}

func TestPlanRemove_UnknownID_FailsNotFound(t *testing.T) {
	dir := t.TempDir()
	writeNodes(t, dir, "lone-node.md")

	_, err := mustLoad(t, dir, nil).PlanRemove("other-node", issueKind)

	if !errors.Is(err, tree.ErrNotFound) {
		t.Errorf("PlanRemove of an unknown id = %v; want ErrNotFound", err)
	}
}

func TestRemovalApply_LastChildWithEmptyChildrenDir_RemovesAllThree(t *testing.T) {
	dir := t.TempDir()
	writeNodes(t, dir, "parent-node.md", "parent-node/kid-one.md")
	if err := os.Mkdir(filepath.Join(dir, "parent-node", "kid-one"), 0o755); err != nil {
		t.Fatal(err)
	}
	removal := planRemove(t, dir, "kid-one")

	warnings, err := removal.Apply()

	if err != nil || len(warnings) > 0 {
		t.Fatalf("Apply = %v, %v", warnings, err)
	}
	want := map[string]string{"parent-node.md": "x\n"}
	if after := snapshotDir(t, dir); !reflect.DeepEqual(after, want) {
		t.Errorf("tree after removal = %v, want %v", after, want)
	}
}

func TestRemovalApply_TopLevelLeaf_KeepsTheTreeDirectory(t *testing.T) {
	dir := t.TempDir()
	writeNodes(t, dir, "lone-node.md")
	removal := planRemove(t, dir, "lone-node")

	warnings, err := removal.Apply()

	if err != nil || len(warnings) > 0 {
		t.Fatalf("Apply = %v, %v", warnings, err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("the tree directory is gone: %v", err)
	}
}

func TestRemovalApply_FileRemovalFails_RestoresChildrenDirAndRewrite(t *testing.T) {
	dir := t.TempDir()
	writeNodes(t, dir, "parent-node.md", "parent-node/kid-one.md", "dependent-node.md")
	childrenDir := filepath.Join(dir, "parent-node", "kid-one")
	if err := os.Mkdir(childrenDir, 0o700); err != nil {
		t.Fatal(err)
	}
	before := snapshotDir(t, dir)
	removal := planRemove(t, dir, "kid-one")
	removal.AddRewrite(tree.Rewrite{
		Path:     filepath.Join(dir, "dependent-node.md"),
		Original: []byte("x\n"),
		Content:  []byte("rewritten\n"),
	})
	failRemoveOf(removal, "kid-one.md")

	_, err := removal.Apply()

	if !errors.Is(err, errInjectedRemove) {
		t.Fatalf("Apply err = %v, want the injected failure", err)
	}
	if after := snapshotDir(t, dir); !reflect.DeepEqual(after, before) {
		t.Errorf("tree changed:\n%v\nwant\n%v", after, before)
	}
	info, err := os.Stat(childrenDir)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
		t.Errorf("children directory mode = %v, want 0700 kept", info.Mode().Perm())
	}
}

func TestRemovalApply_LaterStepFails_RestoresFileBytesAndMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lone-node.md")
	original := "---\r\ntitle: Lone\r\n---\r\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	removal := planRemove(t, dir, "lone-node")
	removal.AddStep(func() error { return errInjectedRemove }, func() error { return nil })

	_, err := removal.Apply()

	if !errors.Is(err, errInjectedRemove) {
		t.Fatalf("Apply err = %v, want the injected failure", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != original {
		t.Errorf("lone-node.md = %q, want %q", content, original)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("lone-node.md mode = %v, want 0600 kept", info.Mode().Perm())
	}
}

func TestRemovalApply_EmptiedParentDirRemovalFails_WarnsAndKeepsTheRemoval(t *testing.T) {
	dir := t.TempDir()
	writeNodes(t, dir, "parent-node.md", "parent-node/kid-one.md")
	removal := planRemove(t, dir, "kid-one")
	failRemoveOf(removal, "parent-node")

	warnings, err := removal.Apply()

	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	var cleanup *tree.CleanupError
	if len(warnings) != 1 || !errors.As(warnings[0], &cleanup) ||
		cleanup.Path != filepath.Join(dir, "parent-node") || !errors.Is(cleanup, errInjectedRemove) {
		t.Errorf("warnings = %v, want one *tree.CleanupError naming parent-node", warnings)
	}
	want := map[string]string{"parent-node.md": "x\n", "parent-node/": ""}
	if after := snapshotDir(t, dir); !reflect.DeepEqual(after, want) {
		t.Errorf("tree after removal = %v, want %v", after, want)
	}
}

func TestPlanRemove_ParentDirHoldsAnEntry_LeavesIt(t *testing.T) {
	dir := t.TempDir()
	writeNodes(t, dir, "parent-node.md", "parent-node/kid-node.md", "parent-node/.keep")

	warnings, err := planRemove(t, dir, "kid-node").Apply()

	if _, statErr := os.Stat(filepath.Join(dir, "parent-node")); err != nil || warnings != nil || errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("Apply = %v, %v, stat %v; want the parent directory left", warnings, err, statErr)
	}
}
