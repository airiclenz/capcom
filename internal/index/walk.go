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
	"github.com/airiclenz/capcom/internal/nodestore"
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
// It also returns the names the files the walk leaves out claim, as tree.ClaimOnlyFacts, the one
// builder tree.Load claims through, finds them with content, the ContentFunc of the tree's kind,
// over the walk problems and the directories beside the files of a repeated id, and the node files
// it leaves out below a repeated id, as leaveOutBelowRepeatedIDs finds them, both without their
// kind.
func walkIssues(
	worktreeRoot, issuesDir string,
	content tree.ContentFunc,
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
	// The directories beside the files of a repeated id are passed too: their claims are the only
	// aliases the node files left out below a repeated id get.
	problems = append(problems, directoryProblems...)
	for _, problem := range problems {
		// A walk reports only invalid ids, entries left out unread and orphan directories, each
		// wrapping a sentinel TreeKind knows.
		kind, _ := diagnose.TreeKind(problem)
		path, _ := store.RelativePath(worktreeRoot, problem.Path)
		r.warn(kind, path, 0, problem.Message())
	}
	// The noun only words the alias problems ClaimOnlyFacts drops.
	claims := factClaims(worktreeRoot, tree.ClaimOnlyFacts(problems, "node", content))
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

// factClaims returns the claims of facts, as tree.ClaimOnlyFacts builds them: each fact's id and
// its Aliases, at its Key relative to worktreeRoot and with its parse state, its kind left for the
// caller to set.
func factClaims(worktreeRoot string, facts []tree.FileFact) []claim {
	var claims []claim
	for _, fact := range facts {
		path, _ := store.RelativePath(worktreeRoot, fact.Key)
		claims = append(claims, claim{name: fact.ID, path: path, parse: fact.Parse})
		for _, alias := range fact.Aliases {
			claims = append(claims, claim{name: alias, path: path, parse: fact.Parse})
		}
	}
	return claims
}

// contentThrough returns the tree.ContentFunc that reads through reader, the Kind reader of one
// tree, naming each file relative to worktreeRoot: a file that parses gives its Aliases, one that
// does not gives the ones its raw frontmatter still recovers, and one the reader refuses unread
// gives none. Any other read failure is returned.
func contentThrough[T any](worktreeRoot string, reader nodestore.Reader[T]) tree.ContentFunc {
	return func(path string) (tree.Content, error) {
		relPath, _ := store.RelativePath(worktreeRoot, path)
		reading, err := reader.Read(path, relPath)
		switch {
		case err != nil:
			return tree.Content{}, err
		case !reading.IsRead():
			return tree.Content{Parse: tree.UnreadFile}, nil
		case !reading.IsParsed():
			return tree.Content{Aliases: reading.Aliases, Parse: tree.UnparsedFile}, nil
		}
		return tree.Content{Aliases: reading.Aliases, Parse: tree.ParsedFile, IsUsable: reading.IsUsable()}, nil
	}
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
