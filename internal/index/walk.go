package index

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/rawfm"
	"github.com/airiclenz/capcom/internal/tree"
)

// hiddenNamePrefix starts the name of every file or directory the walk skips.
const hiddenNamePrefix = "."

// lstat stats a node file the walk found. It is a variable so a test can remove the file between
// the walk and its stat.
var lstat = os.Lstat

// walkIssues lists the node files of the tree in issuesDir (the issue tree, or the Knowledge
// tree), in path order, with paths relative to worktreeRoot. It adapts tree.Walk, the one directory
// walk: each node file is stat'ed for the refresh to compare, and each walk problem — a file whose
// name is not a valid id, an entry that is not a regular file, a node file over 1 MiB, a directory
// without a valid node file, left out with its whole subtree — is warned about through r. A missing issuesDir is an empty tree. A file gone before its stat is
// skipped. A walk or stat failure names its path relative to worktreeRoot.
// It also returns the names the files the walk leaves out claim, as problemClaims finds them,
// without their kind.
func walkIssues(worktreeRoot, issuesDir string, r *refresher) ([]foundFile, []claim, error) {
	nodes, problems, err := tree.Walk(issuesDir)
	if err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			return nil, nil, fmt.Errorf("%s: %w", worktreeRelative(worktreeRoot, pathErr.Path), err)
		}
		return nil, nil, err
	}
	var claims []claim
	for _, problem := range problems {
		r.warn(walkWarningKind(problem), worktreeRelative(worktreeRoot, problem.Path), 0, problem.Message())
		claims = append(claims, problemClaims(worktreeRoot, problem)...)
	}

	found := make([]foundFile, 0, len(nodes))
	for _, node := range nodes {
		info, err := lstat(node.Path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", worktreeRelative(worktreeRoot, node.Path), err)
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
	return found, claims, nil
}

// problemClaims returns the names a file the walk leaves out claims, so a resolve refuses them
// instead of matching them as a prefix. A node file left out unread (not a regular file, or over
// 1 MiB) whose name is a valid id claims that id; a directory left out claims the names of every
// node file below it, as orphanDirectoryClaims finds them; a file with an invalid id claims nothing.
// Each claim's path is relative to worktreeRoot and its kind is left for the caller to set.
func problemClaims(worktreeRoot string, problem *tree.Error) []claim {
	switch {
	case errors.Is(problem, tree.ErrOrphanDirectory):
		return orphanDirectoryClaims(worktreeRoot, problem.Path)
	case errors.Is(problem, tree.ErrNotRegular), errors.Is(problem, tree.ErrTooLarge):
		id, isNode := nodeFileID(problem.Path)
		if !isNode {
			return nil
		}
		return []claim{{name: id, path: worktreeRelative(worktreeRoot, problem.Path)}}
	}
	return nil
}

// orphanDirectoryClaims returns the names claimed by every node file at any depth below dir, a
// directory the walk left out: each file's id, plus the Aliases rawfm.Aliases reads from its bytes
// through tree.ReadNodeFile. A file the guarded reader refuses unread claims its id only. Hidden
// names are skipped at every depth.
//
// The walk has already warned about dir itself, so an entry below it that cannot be read (an
// unreadable directory, dir or a file removed since the walk, a failed read) only leaves its names
// unclaimed; it never fails the refresh.
func orphanDirectoryClaims(worktreeRoot, dir string) []claim {
	var claims []claim
	visit := func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if current != dir && strings.HasPrefix(entry.Name(), hiddenNamePrefix) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		id, isNode := nodeFileID(current)
		if entry.IsDir() || !isNode {
			return nil
		}
		names := []string{id}
		data, err := tree.ReadNodeFile(current)
		switch {
		case readRefusal(err) != nil:
		case err != nil:
			return nil
		default:
			names = append(names, rawfm.Aliases(data)...)
		}
		path := worktreeRelative(worktreeRoot, current)
		for _, name := range names {
			claims = append(claims, claim{name: name, path: path})
		}
		return nil
	}
	// visit never returns an error other than filepath.SkipDir, which WalkDir consumes, so the walk
	// itself never fails.
	if err := filepath.WalkDir(dir, visit); err != nil {
		return nil
	}
	return claims
}

// nodeFileID returns the id a node file at path is named by, and whether its name is a node file's
// with a valid id.
func nodeFileID(path string) (string, bool) {
	id, isNode := strings.CutSuffix(filepath.Base(path), nodeFileExtension)
	return id, isNode && tree.ValidateID(id) == nil
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
