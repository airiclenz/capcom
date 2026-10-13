package gitx

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// showFileLimit is the size limit the tests pass to ShowFile when the limit is not under test.
const showFileLimit = 1 << 20

// commitFile writes content to the slash path name below root and commits it.
func commitFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "change "+name)
}

func TestBranches_ListsEveryLocalBranchAndMarksTheCurrentOne(t *testing.T) {
	root := newRepo(t)
	git(t, root, "branch", "feature/login")
	git(t, root, "checkout", "-q", "-b", "work")

	branches, err := Branches(root)

	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	current := map[string]bool{}
	for _, branch := range branches {
		current[branch.Name] = branch.IsCurrent
		if branch.Ref != "refs/heads/"+branch.Name || branch.Tip.IsZero() {
			t.Errorf("branch %+v: want ref refs/heads/<name> and a tip time", branch)
		}
	}
	if len(branches) != 3 || current["feature/login"] || !current["work"] {
		t.Errorf("branches = %+v, want three with only work current", branches)
	}
}

func TestBranches_Clone_ListsRemoteTrackingBranchesButNotOriginHead(t *testing.T) {
	origin := newRepo(t)
	git(t, origin, "branch", "feature/login")
	clone, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	git(t, origin, "clone", "-q", origin, clone)

	branches, err := Branches(clone)

	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	var local, remote []string
	var currentUpstream string
	for _, branch := range branches {
		if branch.Ref == "refs/remotes/origin/HEAD" {
			t.Errorf("branch %+v: refs/remotes/origin/HEAD is not a branch", branch)
		}
		if branch.Commit == "" || branch.Tip.IsZero() {
			t.Errorf("branch %+v: want a tip commit and time", branch)
		}
		if branch.IsCurrent {
			currentUpstream = branch.Upstream
		}
		if !branch.IsRemote {
			local = append(local, branch.Name)
			continue
		}
		remote = append(remote, branch.Name)
		if branch.Ref != "refs/remotes/"+branch.Name {
			t.Errorf("branch %+v: want ref refs/remotes/<name>", branch)
		}
	}
	if len(local) != 1 || len(remote) != 2 || remote[0] != "origin/feature/login" {
		t.Fatalf("local = %q, remote = %q; want one local branch, origin/feature/login and the default",
			local, remote)
	}
	if want := "refs/remotes/origin/" + local[0]; currentUpstream != want {
		t.Errorf("upstream of the current branch = %q, want %q", currentUpstream, want)
	}
}

func TestBranches_UnbornRepository_ListsNone(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	setHermeticGitEnv(t, root)
	git(t, root, "init", "-q")

	branches, err := Branches(root)

	if err != nil || len(branches) != 0 {
		t.Errorf("Branches = %+v, %v; want none", branches, err)
	}
	if hasHead, err := HasCommit(root, "HEAD"); err != nil || hasHead {
		t.Errorf("HasCommit(HEAD) = %v, %v; want false on an unborn branch", hasHead, err)
	}
}

func TestHasCommit_NotARepository_ReturnsTheGitError(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	setHermeticGitEnv(t, root)

	hasCommit, err := HasCommit(root, "HEAD")

	var gitFailure *Error
	if hasCommit || !errors.As(err, &gitFailure) {
		t.Errorf("HasCommit(HEAD) outside a repository = %v, %v; want false and the git error", hasCommit, err)
	}
}

func TestHasCommit_UnbornHead_IsFalseWithoutError(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	setHermeticGitEnv(t, root)
	git(t, root, "init", "-q")

	hasCommit, err := HasCommit(root, "HEAD")

	if hasCommit || err != nil {
		t.Errorf("HasCommit(HEAD) on an unborn branch = %v, %v; want false and no error", hasCommit, err)
	}
}

func TestIsAncestor_MergedAndDivergedBranches(t *testing.T) {
	root := newRepo(t)
	git(t, root, "branch", "merged")
	git(t, root, "checkout", "-q", "-b", "diverged")
	commitFile(t, root, "a.md", "a\n")
	git(t, root, "checkout", "-q", "-")

	isMerged, mergedErr := IsAncestor(root, "refs/heads/merged", "HEAD")
	isDiverged, divergedErr := IsAncestor(root, "refs/heads/diverged", "HEAD")

	if mergedErr != nil || divergedErr != nil || !isMerged || isDiverged {
		t.Errorf("merged = %v (%v), diverged = %v (%v); want true and false",
			isMerged, mergedErr, isDiverged, divergedErr)
	}
}

func TestIsAncestor_MissingRef_ReturnsAnError(t *testing.T) {
	root := newRepo(t)

	isAncestor, err := IsAncestor(root, "refs/heads/no-such-branch", "HEAD")

	if isAncestor || err == nil {
		t.Errorf("IsAncestor(missing ref) = %v, %v; want false and an error", isAncestor, err)
	}
}

func TestChangedFiles_RenameIsRemovedPlusAdded_PathspecLimits(t *testing.T) {
	root := newRepo(t)
	commitFile(t, root, "issues/old-name.md", "same\n")
	commitFile(t, root, "issues/edited-one.md", "before\n")
	git(t, root, "checkout", "-q", "-b", "topic")
	git(t, root, "mv", "issues/old-name.md", "issues/new-name.md")
	commitFile(t, root, "issues/edited-one.md", "after\n")
	commitFile(t, root, "elsewhere.txt", "outside the pathspec\n")
	git(t, root, "checkout", "-q", "-")

	changes, err := ChangedFiles(root, "HEAD", "refs/heads/topic", []string{"issues"})

	want := []FileChange{
		{Path: "issues/edited-one.md", Kind: Modified},
		{Path: "issues/new-name.md", Kind: Added},
		{Path: "issues/old-name.md", Kind: Removed},
	}
	if err != nil || !reflect.DeepEqual(changes, want) {
		t.Errorf("ChangedFiles = %+v, %v\nwant %+v", changes, err, want)
	}
}

func TestChangedFiles_NoChange_ReturnsNone(t *testing.T) {
	root := newRepo(t)
	git(t, root, "branch", "same")

	changes, err := ChangedFiles(root, "HEAD", "refs/heads/same", []string{"issues"})

	if err != nil || len(changes) != 0 {
		t.Errorf("ChangedFiles = %+v, %v; want none", changes, err)
	}
}

func TestShowFile_ReadsTheBranchCopy(t *testing.T) {
	root := newRepo(t)
	commitFile(t, root, "issues/note.md", "main copy\n")
	git(t, root, "checkout", "-q", "-b", "topic")
	commitFile(t, root, "issues/note.md", "topic copy\n")
	git(t, root, "checkout", "-q", "-")

	content, err := ShowFile(root, "refs/heads/topic", "issues/note.md", showFileLimit)

	if err != nil || string(content) != "topic copy\n" {
		t.Errorf("ShowFile = %q, %v; want the topic copy", content, err)
	}
	if _, err := ShowFile(root, "refs/heads/topic", "issues/absent.md", showFileLimit); err == nil {
		t.Error("ShowFile of a missing file succeeded, want an error")
	}
}

func TestShowFile_BlobOverTheLimit_FailsWithErrBlobTooLarge(t *testing.T) {
	const limit = 8
	root := newRepo(t)
	commitFile(t, root, "issues/note.md", "longer than eight bytes\n")

	content, err := ShowFile(root, "HEAD", "issues/note.md", limit)

	if !errors.Is(err, ErrBlobTooLarge) || content != nil {
		t.Errorf("ShowFile = %q, %v; want no content and ErrBlobTooLarge", content, err)
	}
}
