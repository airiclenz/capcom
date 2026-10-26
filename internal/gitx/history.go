package gitx

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	// headRevision is the commit checked out in the worktree.
	headRevision = "HEAD"
	// blobType is the object type ls-tree prints for a file; a submodule prints "commit".
	blobType = "blob"
	// treeEntryFields is the number of space-separated fields before the tab of an ls-tree line:
	// mode, type and object name.
	treeEntryFields = 3
)

// TreeFile is one file TreeFiles reports.
type TreeFile struct {
	// Path is relative to the worktree root, with forward slashes, as git prints it.
	Path string
	// Blob is the object name of the file's content at the revision; two revisions hold the
	// same content for a path exactly when the Blob is the same.
	Blob string
}

// ResolveCommit returns the object name of the commit revision names, such as a branch, a tag or
// HEAD, so that later reads all see the same commit even when the ref moves. It returns "" with no
// error when revision names no commit, as HEAD on an unborn branch does. A git failure other than
// "names no commit", such as dir lying outside any repository, is returned as an error.
func ResolveCommit(dir, revision string) (string, error) {
	output, err := run(dir, "rev-parse", "--verify", "--quiet", revision+"^{commit}")
	if err == nil {
		return strings.TrimSpace(output), nil
	}
	var exitFailure *exec.ExitError
	if errors.As(err, &exitFailure) && exitFailure.ExitCode() == noCommitExit {
		return "", nil
	}
	return "", err
}

// CommitBefore returns the object name of the newest commit on HEAD whose committer time is at or
// before t. It returns "" with no error when HEAD has no such commit: every commit is newer, or
// HEAD is unborn (a repository with no commit yet). A git failure, such as dir lying outside any
// repository, is returned as an error.
func CommitBefore(dir string, t time.Time) (string, error) {
	hasHead, err := HasCommit(dir, headRevision)
	if err != nil || !hasHead {
		return "", err
	}
	output, err := run(dir, "rev-list", "-1", fmt.Sprintf("--before=@%d", t.Unix()), headRevision)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// TreeFiles lists the files below pathspecs in revision, recursively, with their blob ids, in
// git's path order. Paths and pathspecs are relative to the worktree root wherever in the
// worktree dir lies; no pathspec lists every file. Submodules are left out. A revision that names
// no commit, such as HEAD on an unborn branch, holds no file: it returns none with no error. Pass
// a commit's object name as revision, not a ref, when the listing must match a later read.
func TreeFiles(dir, revision string, pathspecs ...string) ([]TreeFile, error) {
	hasRevision, err := HasCommit(dir, revision)
	if err != nil || !hasRevision {
		return nil, err
	}
	args := []string{"ls-tree", "-r", "-z", "--full-tree", revision, "--"}
	output, err := run(dir, append(args, pathspecs...)...)
	if err != nil {
		return nil, err
	}

	var files []TreeFile
	for _, line := range strings.Split(strings.TrimSuffix(output, "\x00"), "\x00") {
		if line == "" {
			continue
		}
		file, isBlob, err := parseTreeLine(line)
		if err != nil {
			return nil, fmt.Errorf("git ls-tree %s in %s: %w", revision, dir, err)
		}
		if isBlob {
			files = append(files, file)
		}
	}
	return files, nil
}

// parseTreeLine reads one `ls-tree -z` entry, "<mode> <type> <object>\t<path>". isBlob is false
// for an entry that is not a file, such as a submodule.
func parseTreeLine(line string) (file TreeFile, isBlob bool, err error) {
	header, path, hasPath := strings.Cut(line, "\t")
	fields := strings.Fields(header)
	if !hasPath || len(fields) != treeEntryFields {
		return TreeFile{}, false, fmt.Errorf("unexpected entry %q", line)
	}
	return TreeFile{Path: path, Blob: fields[2]}, fields[1] == blobType, nil
}
