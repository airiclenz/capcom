package index

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
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
// without a valid node file, left out with its whole subtree — is warned about through r. A
// missing issuesDir is an empty tree. A file gone before its stat is skipped. Every node file the
// walk keeps is listed, the ones below a repeated id included: persistMembership leaves those out
// by the one membership verdict of the refresh. A walk or stat failure names its path relative to
// worktreeRoot.
// It also returns the names the files the walk leaves out claim, as tree.ClaimOnlyFacts, the one
// builder tree.Load claims through, finds them with content, the ContentFunc of the tree's kind,
// over the walk problems, without their kind.
func walkIssues(
	worktreeRoot, issuesDir string,
	content tree.ContentFunc,
	r *refresher,
) ([]foundFile, []claim, error) {
	nodes, problems, err := tree.Walk(issuesDir)
	if err != nil {
		return nil, nil, relativePathError(worktreeRoot, err)
	}
	for _, problem := range problems {
		// A walk reports only invalid ids, entries left out unread and orphan directories, each
		// wrapping a sentinel TreeKind knows.
		kind, _ := diagnose.TreeKind(problem)
		path, _ := store.RelativePath(worktreeRoot, problem.Path)
		r.warn(kind, path, 0, problem.Message())
	}
	// The noun only words the alias problems ClaimOnlyFacts drops.
	claims := factClaims(worktreeRoot, tree.ClaimOnlyFacts(problems, "node", content))

	found := make([]foundFile, 0, len(nodes))
	for _, node := range nodes {
		path, _ := store.RelativePath(worktreeRoot, node.Path)
		info, err := lstat(node.Path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", path, err)
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
	return found, claims, nil
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
