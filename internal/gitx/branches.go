package gitx

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	// branchRefPrefix is the namespace of local branches.
	branchRefPrefix = "refs/heads/"
	// branchFormat prints one local branch per line: its full ref, its tip's committer time in
	// Unix seconds and "*" when it is the branch checked out in this worktree.
	branchFormat = "%(refname)%00%(committerdate:unix)%00%(HEAD)"
	branchFields = 3
	// currentMarker is what %(HEAD) prints for the branch checked out in this worktree.
	currentMarker = "*"
	// notAncestorExit is the exit code of `merge-base --is-ancestor` when the answer is no.
	notAncestorExit = 1
)

// Change letters of `diff --name-status`.
const (
	changeAdded    = "A"
	changeDeleted  = "D"
	changeModified = "M"
	changeType     = "T"
)

// ChangeKind is how a file differs between two revisions.
type ChangeKind string

// The kinds ChangedFiles reports. A type change (file to symlink and back) counts as Modified.
const (
	Added    ChangeKind = "added"
	Modified ChangeKind = "modified"
	Removed  ChangeKind = "removed"
)

// Branch is one local branch.
type Branch struct {
	// Name is the branch name without refs/heads/, e.g. "feature/login".
	Name string
	// Ref is the full ref, e.g. "refs/heads/feature/login"; pass it to git, never Name, so a
	// branch named like a tag or an option stays unambiguous.
	Ref string
	// Tip is the committer time of the branch's tip commit.
	Tip time.Time
	// IsCurrent is true for the branch checked out in the worktree Branches ran in.
	IsCurrent bool
}

// FileChange is one file ChangedFiles reports.
type FileChange struct {
	// Path is relative to the worktree root, with forward slashes, as git prints it.
	Path string
	Kind ChangeKind
}

// Branches lists the local branches of the repository containing dir, sorted by name, including
// the ones checked out in other worktrees.
func Branches(dir string) ([]Branch, error) {
	output, err := Run(dir, "for-each-ref", "--format="+branchFormat, branchRefPrefix)
	if err != nil {
		return nil, err
	}

	var branches []Branch
	for _, line := range strings.Split(strings.TrimRight(output, "\r\n"), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(strings.TrimRight(line, "\r"), "\x00")
		if len(fields) != branchFields {
			return nil, fmt.Errorf("git for-each-ref in %s: unexpected line %q", dir, line)
		}
		seconds, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("git for-each-ref in %s: tip time of %s: %w", dir, fields[0], err)
		}
		branches = append(branches, Branch{
			Name:      strings.TrimPrefix(fields[0], branchRefPrefix),
			Ref:       fields[0],
			Tip:       time.Unix(seconds, 0),
			IsCurrent: fields[2] == currentMarker,
		})
	}
	return branches, nil
}

// HasCommit reports whether revision names a commit in the repository containing dir; it is false
// for HEAD on an unborn branch.
func HasCommit(dir, revision string) (bool, error) {
	_, err := Run(dir, "rev-parse", "--verify", "--quiet", revision+"^{commit}")
	if err == nil {
		return true, nil
	}
	var gitFailure *Error
	if errors.As(err, &gitFailure) {
		return false, nil
	}
	return false, err
}

// IsAncestor reports whether ancestor is reachable from descendant: IsAncestor(dir, branch, "HEAD")
// is true when the branch has no commit that HEAD lacks.
func IsAncestor(dir, ancestor, descendant string) (bool, error) {
	_, err := Run(dir, "merge-base", "--is-ancestor", ancestor, descendant)
	if err == nil {
		return true, nil
	}
	var exitFailure *exec.ExitError
	if errors.As(err, &exitFailure) && exitFailure.ExitCode() == notAncestorExit {
		return false, nil
	}
	return false, err
}

// ChangedFiles lists the files below pathspecs that branch added, modified or removed since its
// merge base with base (`git diff base...branch`). Renames are not detected: a renamed file is
// reported as Removed at its old path and Added at its new one. dir must be the worktree root,
// since pathspecs and the returned paths are relative to it.
func ChangedFiles(dir, base, branch string, pathspecs []string) ([]FileChange, error) {
	args := []string{"diff", "--name-status", "--no-renames", "-z", base + "..." + branch, "--"}
	output, err := Run(dir, append(args, pathspecs...)...)
	if err != nil {
		return nil, err
	}

	fields := strings.Split(strings.TrimSuffix(output, "\x00"), "\x00")
	if len(fields) == 1 && fields[0] == "" {
		return nil, nil
	}
	if len(fields)%2 != 0 {
		return nil, fmt.Errorf("git diff in %s: unexpected output %q", dir, output)
	}
	changes := make([]FileChange, 0, len(fields)/2)
	for i := 0; i < len(fields); i += 2 {
		kind, isKnown := changeKind(fields[i])
		if !isKnown {
			return nil, fmt.Errorf("git diff in %s: unexpected change %q for %s", dir, fields[i], fields[i+1])
		}
		changes = append(changes, FileChange{Path: fields[i+1], Kind: kind})
	}
	return changes, nil
}

// ShowFile returns the content of the file at path, relative to the worktree root, in revision.
func ShowFile(dir, revision, path string) ([]byte, error) {
	output, err := Run(dir, "cat-file", "blob", revision+":"+path)
	if err != nil {
		return nil, err
	}
	return []byte(output), nil
}

// changeKind maps a `diff --name-status` letter to its kind.
func changeKind(letter string) (ChangeKind, bool) {
	switch letter {
	case changeAdded:
		return Added, true
	case changeDeleted:
		return Removed, true
	case changeModified, changeType:
		return Modified, true
	default:
		return "", false
	}
}
