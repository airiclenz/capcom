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
	// remoteRefPrefix is the namespace of remote-tracking branches.
	remoteRefPrefix = "refs/remotes/"
	// remoteHeadName is the last component of refs/remotes/<remote>/HEAD, the remote's default
	// branch pointer, which names no branch of its own.
	remoteHeadName = "HEAD"
	// branchFormat prints one branch per line: its full ref, its tip's object name, its tip's
	// committer time in Unix seconds, "*" when it is the branch checked out in this worktree and
	// the full ref of its upstream, empty when it has none.
	branchFormat = "%(refname)%00%(objectname)%00%(committerdate:unix)%00%(HEAD)%00%(upstream)"
	branchFields = 5
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

// ErrBlobTooLarge reports a blob ShowFile refused to read because git reported it larger than
// the caller's limit.
var ErrBlobTooLarge = errors.New("blob is larger than the size limit")

// ChangeKind is how a file differs between two revisions.
type ChangeKind string

// The kinds ChangedFiles reports. A type change (file to symlink and back) counts as Modified.
const (
	Added    ChangeKind = "added"
	Modified ChangeKind = "modified"
	Removed  ChangeKind = "removed"
)

// Branch is one local branch or remote-tracking branch.
type Branch struct {
	// Name is the short name: without refs/heads/ for a local branch, e.g. "feature/login", and
	// without refs/remotes/ for a remote-tracking one, e.g. "origin/feature/login".
	Name string
	// Ref is the full ref, e.g. "refs/heads/feature/login"; pass it to git, never Name, so a
	// branch named like a tag or an option stays unambiguous.
	Ref string
	// Commit is the object name of the branch's tip commit.
	Commit string
	// Tip is the committer time of the branch's tip commit.
	Tip time.Time
	// IsCurrent is true for the branch checked out in the worktree Branches ran in.
	IsCurrent bool
	// IsRemote is true for a remote-tracking branch, one under refs/remotes/.
	IsRemote bool
	// Upstream is the full ref of the branch's upstream, e.g. "refs/remotes/origin/main" or
	// "refs/heads/main"; empty when it has none.
	Upstream string
}

// FileChange is one file ChangedFiles reports.
type FileChange struct {
	// Path is relative to the worktree root, with forward slashes, as git prints it.
	Path string
	Kind ChangeKind
}

// Branches lists the local branches of the repository containing dir, including the ones checked
// out in other worktrees, and then its remote-tracking branches, each group sorted by name. It
// never fetches, so a remote-tracking branch is as fresh as the last fetch. The refs/remotes/
// <remote>/HEAD pointers are left out.
func Branches(dir string) ([]Branch, error) {
	output, err := run(dir, "for-each-ref", "--format="+branchFormat, branchRefPrefix, remoteRefPrefix)
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
		ref := fields[0]
		if isRemoteHead(ref) {
			continue
		}
		seconds, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("git for-each-ref in %s: tip time of %s: %w", dir, ref, err)
		}
		name, isRemote := strings.CutPrefix(ref, remoteRefPrefix)
		if !isRemote {
			name = strings.TrimPrefix(ref, branchRefPrefix)
		}
		branches = append(branches, Branch{
			Name:      name,
			Ref:       ref,
			Commit:    fields[1],
			Tip:       time.Unix(seconds, 0),
			IsCurrent: fields[3] == currentMarker,
			IsRemote:  isRemote,
			Upstream:  fields[4],
		})
	}
	return branches, nil
}

// CurrentBranch returns the short name of the branch checked out in the worktree containing dir,
// as `git branch --show-current` prints it: the unborn branch's name in a repository with no
// commit yet, and "" with no error on a detached HEAD.
func CurrentBranch(dir string) (string, error) {
	output, err := run(dir, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// HasCommit reports whether revision names a commit in the repository containing dir; it is false
// for HEAD on an unborn branch.
func HasCommit(dir, revision string) (bool, error) {
	_, err := run(dir, "rev-parse", "--verify", "--quiet", revision+"^{commit}")
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
	_, err := run(dir, "merge-base", "--is-ancestor", ancestor, descendant)
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
	output, err := run(dir, append(args, pathspecs...)...)
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
// It asks git for the blob's size first and reads the blob only when it is at most limit bytes;
// a larger one fails with an error wrapping ErrBlobTooLarge, so a hostile branch cannot make the
// caller buffer an arbitrarily large file. Pass a commit's object name as revision, not a ref, so
// the size probe and the read name the same blob.
func ShowFile(dir, revision, path string, limit int64) ([]byte, error) {
	object := revision + ":" + path
	sizeOutput, err := run(dir, "cat-file", "-s", object)
	if err != nil {
		return nil, err
	}
	size, err := strconv.ParseInt(strings.TrimSpace(sizeOutput), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("git cat-file -s %s in %s: %w", object, dir, err)
	}
	if size > limit {
		return nil, fmt.Errorf("%s: %w", object, ErrBlobTooLarge)
	}

	output, err := run(dir, "cat-file", "blob", object)
	if err != nil {
		return nil, err
	}
	return []byte(output), nil
}

// isRemoteHead reports whether ref is refs/remotes/<remote>/HEAD, a remote's default branch
// pointer rather than a branch.
func isRemoteHead(ref string) bool {
	rest, isRemote := strings.CutPrefix(ref, remoteRefPrefix)
	remote, name, hasName := strings.Cut(rest, "/")
	return isRemote && hasName && remote != "" && name == remoteHeadName
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
