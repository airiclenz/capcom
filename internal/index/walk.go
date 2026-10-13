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

	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/nodefile"
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
// skipped. Every node file under the directory beside a file of a repeated id is left out with
// that directory, which is warned about as an orphan directory. A walk or stat failure names its
// path relative to worktreeRoot.
// It also returns the names the files the walk leaves out claim, as problemClaims finds them, and
// the node files it leaves out below a repeated id, as leaveOutRepeatedIDSubtrees finds them, both
// without their kind.
func walkIssues(worktreeRoot, issuesDir string, r *refresher) ([]foundFile, []claim, []leftOutNode, error) {
	nodes, problems, err := tree.Walk(issuesDir)
	if err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			return nil, nil, nil, fmt.Errorf("%s: %w", worktreeRelative(worktreeRoot, pathErr.Path), err)
		}
		return nil, nil, nil, err
	}
	nodes, leftOut, directoryProblems, err := leaveOutRepeatedIDSubtrees(worktreeRoot, nodes)
	if err != nil {
		return nil, nil, nil, err
	}
	problems = append(problems, directoryProblems...)
	var claims []claim
	for _, problem := range problems {
		// A walk reports only invalid ids, entries left out unread and orphan directories, each
		// wrapping a sentinel TreeKind knows.
		kind, _ := diagnose.TreeKind(problem)
		r.warn(kind, worktreeRelative(worktreeRoot, problem.Path), 0, problem.Message())
		claims = append(claims, problemClaims(worktreeRoot, problem)...)
	}
	claims = withoutKeptFiles(claims, worktreeRoot, nodes)

	found := make([]foundFile, 0, len(nodes))
	for _, node := range nodes {
		info, err := lstat(node.Path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%s: %w", worktreeRelative(worktreeRoot, node.Path), err)
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
	return found, claims, leftOut, nil
}

// leftOutNode is a node file the walk found below the directory beside a file of a repeated id,
// which the index leaves out as tree.Load does, with the duplicate problem of that repeated id.
type leftOutNode struct {
	id string
	// path is relative to the working tree root, with forward slashes.
	path string
	// kind is kindIssue or kindNote, by the tree the file was found in.
	kind    string
	problem *tree.Error
}

// repeatedIDDirectory is the directory beside a file of a repeated id, with the id and its
// duplicate problem.
type repeatedIDDirectory struct {
	path    string
	id      string
	problem *tree.Error
}

// leaveOutRepeatedIDSubtrees splits nodes, as tree.Walk returns them, the way tree.Load does for an
// id two or more of them share: every node file under the directory beside a file of such an id is
// left out, with the duplicate problem of the innermost such id it lies under, which names that
// id's second file by path and, in Other, its first, both relative to worktreeRoot. The files of a
// repeated id themselves are kept: the refresh leaves them out as duplicates. It returns the kept
// nodes in order, the left-out ones without their kind, and one problem wrapping
// tree.ErrOrphanDirectory per such directory that is a directory on disk. A directory whose Lstat
// fails other than by being gone fails the walk, naming it relative to worktreeRoot.
func leaveOutRepeatedIDSubtrees(worktreeRoot string, nodes []tree.Node) ([]tree.Node, []leftOutNode, tree.Errors, error) {
	pathsByID := map[string][]string{}
	for _, node := range nodes {
		pathsByID[node.ID] = append(pathsByID[node.ID], node.Path)
	}
	var directories []repeatedIDDirectory
	for _, node := range nodes {
		paths := pathsByID[node.ID]
		if len(paths) < 2 {
			continue
		}
		slices.Sort(paths)
		directories = append(directories, repeatedIDDirectory{
			path: tree.ChildrenDir(node.Path),
			id:   node.ID,
			problem: tree.DuplicateIDError(
				worktreeRelative(worktreeRoot, paths[1]),
				worktreeRelative(worktreeRoot, paths[0]),
				node.ID,
			),
		})
	}
	if len(directories) == 0 {
		return nodes, nil, nil, nil
	}

	kept := make([]tree.Node, 0, len(nodes))
	var leftOut []leftOutNode
	for _, node := range nodes {
		directory := enclosingRepeatedIDDirectory(directories, node.Path)
		if directory == nil || len(pathsByID[node.ID]) > 1 {
			kept = append(kept, node)
			continue
		}
		leftOut = append(leftOut, leftOutNode{
			id:      node.ID,
			path:    worktreeRelative(worktreeRoot, node.Path),
			problem: directory.problem,
		})
	}
	problems, err := repeatedIDDirectoryProblems(worktreeRoot, directories)
	return kept, leftOut, problems, err
}

// enclosingRepeatedIDDirectory returns the innermost of directories that path lies under, or nil
// when it lies under none.
func enclosingRepeatedIDDirectory(directories []repeatedIDDirectory, path string) *repeatedIDDirectory {
	var enclosing *repeatedIDDirectory
	for i := range directories {
		directory := &directories[i]
		if !strings.HasPrefix(path, directory.path+string(filepath.Separator)) {
			continue
		}
		if enclosing == nil || len(directory.path) > len(enclosing.path) {
			enclosing = directory
		}
	}
	return enclosing
}

// repeatedIDDirectoryProblems returns the problem of each of directories that is a directory on
// disk, worded as tree.Load words it. One that is gone, or is no directory, is no problem; any other
// failed Lstat is returned, naming the directory relative to worktreeRoot.
func repeatedIDDirectoryProblems(worktreeRoot string, directories []repeatedIDDirectory) (tree.Errors, error) {
	var problems tree.Errors
	for _, directory := range directories {
		info, err := os.Lstat(directory.path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", worktreeRelative(worktreeRoot, directory.path), err)
		}
		if info.IsDir() {
			problems = append(problems, tree.DuplicateIDDirectoryError(directory.path, directory.id))
		}
	}
	return problems, nil
}

// withoutKeptFiles drops from claims every claim of a file among nodes, which keeps its own rows:
// a file of a repeated id below the directory of another one, which that directory's claims reach.
// Such a claim would never be dropped, since replaceWalkClaims rebuilds only the claims of files
// without a files row.
func withoutKeptFiles(claims []claim, worktreeRoot string, nodes []tree.Node) []claim {
	if len(claims) == 0 {
		return claims
	}
	kept := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		kept[worktreeRelative(worktreeRoot, node.Path)] = true
	}
	return slices.DeleteFunc(claims, func(current claim) bool {
		return kept[current.path]
	})
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
// through nodefile.ReadRegularFile. A file the guarded reader refuses unread claims its id only.
// Hidden names are skipped at every depth.
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
		data, err := nodefile.ReadRegularFile(current)
		switch {
		case diagnose.ReadRefusal(err) != nil:
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
	name := filepath.Base(path)
	id := tree.IDOf(name)
	return id, id != name && tree.ValidateID(id) == nil
}

// worktreeRelative names a path below worktreeRoot with forward slashes, for rows and warnings.
func worktreeRelative(worktreeRoot, path string) string {
	relative, err := filepath.Rel(worktreeRoot, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}
