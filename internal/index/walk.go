package index

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/airiclenz/capcom/internal/tree"
)

// walkIssues lists the node files of the tree in issuesDir (the issue tree, or the Knowledge
// tree), in path order, with paths relative to worktreeRoot. It adapts tree.Walk, the one directory
// walk: each node file is stat'ed for the refresh to compare, and each walk problem — a file whose
// name is not a valid id, an entry that is not a regular file, a node file over 1 MiB, a directory
// without a valid node file, left out with its whole subtree — is warned about through r. A missing issuesDir is an empty tree. A file gone before its stat is
// skipped. A walk or stat failure names its path relative to worktreeRoot.
func walkIssues(worktreeRoot, issuesDir string, r *refresher) ([]foundFile, error) {
	nodes, problems, err := tree.Walk(issuesDir)
	if err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			return nil, fmt.Errorf("%s: %w", worktreeRelative(worktreeRoot, pathErr.Path), err)
		}
		return nil, err
	}
	for _, problem := range problems {
		r.warn(walkWarningKind(problem), worktreeRelative(worktreeRoot, problem.Path), 0, problem.Message())
	}

	found := make([]foundFile, 0, len(nodes))
	for _, node := range nodes {
		info, err := os.Lstat(node.Path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", worktreeRelative(worktreeRoot, node.Path), err)
		}
		found = append(found, foundFile{
			path:     worktreeRelative(worktreeRoot, node.Path),
			absolute: node.Path,
			id:       node.ID,
			parentID: node.ParentID,
			size:     info.Size(),
			mtime:    info.ModTime().UnixNano(),
		})
	}
	slices.SortFunc(found, func(a, b foundFile) int {
		return cmp.Compare(a.path, b.path)
	})
	return found, nil
}

// walkWarningKind is the warning kind of a tree.Walk problem: an invalid id, an entry left out
// unread (not a regular file, or a node file over 1 MiB), or else an orphan directory, the only
// problems a walk reports.
func walkWarningKind(problem *tree.Error) WarningKind {
	switch {
	case errors.Is(problem, tree.ErrInvalidID):
		return WarningInvalidID
	case errors.Is(problem, tree.ErrNotRegular), errors.Is(problem, tree.ErrTooLarge):
		return WarningNotRegular
	}
	return WarningOrphanDirectory
}

// worktreeRelative names a path below worktreeRoot with forward slashes, for rows and warnings.
func worktreeRelative(worktreeRoot, path string) string {
	relative, err := filepath.Rel(worktreeRoot, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}
