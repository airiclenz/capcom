package index

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/tree"
)

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
// It also returns the names the files the walk leaves out claim, as problemClaims finds them with
// parse, the tree.AliasParseFunc of the tree's kind, and
// the node files it leaves out below a repeated id, as leaveOutBelowRepeatedIDs finds them, both
// without their kind.
func walkIssues(
	worktreeRoot, issuesDir string,
	parse tree.AliasParseFunc,
	r *refresher,
) ([]foundFile, []claim, []leftOutNode, error) {
	nodes, problems, err := tree.Walk(issuesDir)
	if err != nil {
		return nil, nil, nil, relativePathError(worktreeRoot, err)
	}
	nodes, leftOut, directoryProblems, err := leaveOutBelowRepeatedIDs(worktreeRoot, nodes)
	if err != nil {
		return nil, nil, nil, err
	}
	problems = append(problems, directoryProblems...)
	var claims []claim
	for _, problem := range problems {
		// A walk reports only invalid ids, entries left out unread and orphan directories, each
		// wrapping a sentinel TreeKind knows.
		kind, _ := diagnose.TreeKind(problem)
		path, _ := store.RelativePath(worktreeRoot, problem.Path)
		r.warn(kind, path, 0, problem.Message())
		claims = append(claims, problemClaims(worktreeRoot, problem, parse)...)
	}
	claims = withoutKeptFiles(claims, worktreeRoot, nodes)

	found := make([]foundFile, 0, len(nodes))
	for _, node := range nodes {
		path, _ := store.RelativePath(worktreeRoot, node.Path)
		info, err := lstat(node.Path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%s: %w", path, err)
		}
		found = append(found, foundFile{
			path:     path,
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
// which tree.DecideMembership leaves out of the tree.
type leftOutNode struct {
	id string
	// path is relative to the working tree root, with forward slashes.
	path string
	// kind is kindIssue or kindNote, by the tree the file was found in.
	kind string
}

// leaveOutBelowRepeatedIDs splits nodes, as tree.Walk returns them, by tree.DecideMembership, the
// membership function tree.Load decides by. Each node is passed as a walk fact of its path, id,
// parent id and directory alone: its key is its absolute path, so a repeated id's files order as
// tree.Load orders them; it lists no aliases, its parse state is tree.UnreadFile and it is usable,
// for the walk reads no file. Every node the function leaves out below a repeated id is left out,
// its path relative to worktreeRoot. The files of a repeated id themselves are kept: the refresh
// leaves them out as duplicates. It returns the kept nodes in
// order, the left-out ones in order without their kind, and the problem of each repeated-id
// directory that is a directory on disk, as tree.RepeatedIDDirProblems finds them. A directory
// whose Lstat fails other than by being gone fails the walk, naming it relative to worktreeRoot.
func leaveOutBelowRepeatedIDs(worktreeRoot string, nodes []tree.Node) ([]tree.Node, []leftOutNode, tree.Errors, error) {
	facts := make([]tree.FileFact, len(nodes))
	for i, node := range nodes {
		facts[i] = tree.FileFact{
			Key:      node.Path,
			ID:       node.ID,
			ParentID: node.ParentID,
			Dir:      filepath.Dir(node.Path),
			Parse:    tree.UnreadFile,
			IsUsable: true,
			Origin:   tree.WalkOrigin,
		}
	}
	membership := tree.DecideMembership(facts)
	if len(membership.RepeatedIDDirs) == 0 {
		return nodes, nil, nil, nil
	}

	isBelow := map[int]bool{}
	for _, file := range membership.LeftOut {
		if file.Reason == tree.BelowRepeatedID {
			isBelow[file.Fact] = true
		}
	}
	kept := make([]tree.Node, 0, len(nodes))
	var leftOut []leftOutNode
	for i, node := range nodes {
		if !isBelow[i] {
			kept = append(kept, node)
			continue
		}
		path, _ := store.RelativePath(worktreeRoot, node.Path)
		leftOut = append(leftOut, leftOutNode{id: node.ID, path: path})
	}
	problems, err := tree.RepeatedIDDirProblems(membership.RepeatedIDDirs)
	if err != nil {
		return nil, nil, nil, relativePathError(worktreeRoot, err)
	}
	return kept, leftOut, problems, nil
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
		path, _ := store.RelativePath(worktreeRoot, node.Path)
		kept[path] = true
	}
	return slices.DeleteFunc(claims, func(current claim) bool {
		return kept[current.path]
	})
}

// problemClaims returns the names a file the walk leaves out claims, so a resolve refuses them
// instead of matching them as a prefix. A node file left out unread (not a regular file, or over
// 1 MiB) whose name is a valid id claims that id; a directory left out claims the names of every
// node file below it, as orphanDirectoryClaims finds them with parse; a file with an invalid id
// claims nothing. Each claim's path is relative to worktreeRoot and its kind is left for the caller
// to set.
func problemClaims(worktreeRoot string, problem *tree.Error, parse tree.AliasParseFunc) []claim {
	switch {
	case errors.Is(problem, tree.ErrOrphanDirectory):
		return orphanDirectoryClaims(worktreeRoot, problem.Path, parse)
	case errors.Is(problem, tree.ErrNotRegular), errors.Is(problem, tree.ErrTooLarge):
		id, isNode := nodeFileID(problem.Path)
		if !isNode {
			return nil
		}
		path, _ := store.RelativePath(worktreeRoot, problem.Path)
		return []claim{{name: id, path: path}}
	}
	return nil
}

// orphanDirectoryClaims returns the names claimed by every node file with a valid id that
// tree.ListOrphanFiles lists below dir, a directory the walk left out: each file's id, plus the
// Aliases tree.ReadAliases reads with parse from the bytes of a regular file read through
// nodefile.ReadRegularFile. A file that is not a regular file, or that the guarded reader refuses
// unread, claims its id only.
//
// The walk has already warned about dir itself, so an entry below it that cannot be read (an
// unreadable directory, dir or a file removed since the walk, a failed read) only leaves its names
// unclaimed, never those of the files listed beside it; it never fails the refresh.
func orphanDirectoryClaims(worktreeRoot, dir string, parse tree.AliasParseFunc) []claim {
	files, _ := tree.ListOrphanFiles(dir)
	var claims []claim
	for _, file := range files {
		id, isNode := nodeFileID(file.Path)
		if !isNode {
			continue
		}
		names := []string{id}
		if file.IsRegular {
			data, err := nodefile.ReadRegularFile(file.Path)
			switch {
			case diagnose.ReadRefusal(err) != nil:
			case err != nil:
				continue
			default:
				aliases, _ := tree.ReadAliases(data, parse)
				names = append(names, aliases...)
			}
		}
		path, _ := store.RelativePath(worktreeRoot, file.Path)
		for _, name := range names {
			claims = append(claims, claim{name: name, path: path})
		}
	}
	return claims
}

// errUnparsed is the error an alias parse of the index returns for a file that does not parse.
var errUnparsed = errors.New("frontmatter does not parse")

// aliasParser returns the tree.AliasParseFunc of the files of kind, kindIssue or kindNote: the
// Aliases of the issue issue.Parse, or the Note knowledge.Parse, parses from the bytes.
func aliasParser(kind string) tree.AliasParseFunc {
	if kind == kindNote {
		return func(data []byte) ([]string, error) {
			parsed, err := knowledge.Parse("", data)
			if err != nil {
				return nil, errUnparsed
			}
			return parsed.Aliases, nil
		}
	}
	return func(data []byte) ([]string, error) {
		parsed, err := issue.Parse("", data)
		if err != nil {
			return nil, errUnparsed
		}
		return parsed.Aliases, nil
	}
}

// nodeFileID returns the id a node file at path is named by, and whether its name is a node file's
// with a valid id.
func nodeFileID(path string) (string, bool) {
	name := filepath.Base(path)
	id := tree.IDOf(name)
	return id, id != name && tree.ValidateID(id) == nil
}

// relativePathError returns err naming its path relative to worktreeRoot when it is an
// *fs.PathError, and err unchanged otherwise.
func relativePathError(worktreeRoot string, err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		path, _ := store.RelativePath(worktreeRoot, pathErr.Path)
		return fmt.Errorf("%s: %w", path, err)
	}
	return err
}
