package nodestore

import (
	"errors"
	"io/fs"
	"os"

	"github.com/airiclenz/capcom/internal/tree"
)

// Create writes a new node of the kind, named id, below the node parent names (its id, one of its
// Aliases or a unique id prefix; "" places it at the top of the tree), with the content build
// returns. It returns the node written and the tree problems that do not concern it, their paths
// relative to the working tree root with forward slashes, for the caller to report as warnings.
//
// It runs, in this order, stopping at the first failure:
//  1. id validation: an invalid id fails with an error wrapping tree.ErrInvalidID.
//  2. the Tree load: a tree that cannot be loaded at all fails with its *tree.Error or tree.Errors.
//  3. the claim: a file that holds id, as tree.Tree.Claimant finds it with RecoverAliases, is an
//     *ExistsError at that file worded by how it holds id: a node of the tree answering to id, as
//     its id or one of its Aliases; a node file inside an orphan directory, which the tree leaves
//     out; a file left out for its repeated id; or a node file that does not parse but whose raw
//     frontmatter still lists id as an alias.
//  4. the --parent resolution: tree.ErrNotFound or tree.ErrAmbiguous, unwrapped, or, for a name
//     two files claim, a *tree.Error wrapping tree.ErrDuplicate that names both files.
//  5. triage: a tree problem naming the new file, the parent's file or the parent's children
//     directory blocks the write and comes back as the failure, a tree.Errors.
//  6. build, whose failure is returned as it is.
//  7. validation: a node that does not validate fails with a *FileError at the new file, listing
//     every problem and citing the first problem's line.
//  8. the exclusive write, which never replaces a file: right before it the parent's node file is
//     checked again, and a parent file another session removed or cannot be checked fails with a
//     *FileError at it, creating neither the parent's children directory nor the new file; a file
//     that appeared at the new node's path since the claim fails with the *ExistsError naming it;
//     any other failure is returned as it is.
func (k *Kind[T]) Create(
	id string,
	parent string,
	build func() (T, error),
) (Written, tree.Errors, error) {
	if err := tree.ValidateID(id); err != nil {
		return Written{}, nil, err
	}
	nodes, problems, err := k.load()
	if err != nil {
		return Written{}, nil, err
	}
	claim, isTaken, err := nodes.Claimant(id, "", k.RecoverAliases)
	if err != nil {
		return Written{}, nil, err
	}
	if isTaken {
		return Written{}, nil, k.claimRefusal(id, claim)
	}
	path, parentPath, err := newNodePath(nodes, id, parent)
	if err != nil {
		return Written{}, nil, k.resolveFailure(err)
	}
	warnings, err := k.triage(problems, k.relative(path), k.relative(parentPath))
	if err != nil {
		return Written{}, nil, err
	}

	node, err := build()
	if err != nil {
		return Written{}, nil, err
	}
	if parentPath != "" {
		// Writing below a parent whose node file is gone would recreate its children directory
		// with no node file.
		if _, err := os.Lstat(parentPath); err != nil {
			return Written{}, nil, k.fileError(parentPath, err)
		}
	}
	written, err := k.write(path, id, node, createFile)
	if errors.Is(err, fs.ErrExist) {
		// Another writer placed the file after the claim passed.
		return Written{}, nil, k.existsError(id, tree.Node{ID: id, Path: path})
	}
	if err != nil {
		return Written{}, nil, err
	}
	return written, warnings, nil
}

// newNodePath returns the file of the new node id, below the node parent resolves to or at the top
// of the tree when parent is "", and the file of that parent node, "" at the top. A parent that
// does not resolve fails as tree.Resolve does.
func newNodePath(nodes *tree.Tree, id string, parent string) (string, string, error) {
	parentNode := tree.Node{}
	if parent != "" {
		resolved, err := nodes.Resolve(parent)
		if err != nil {
			return "", "", err
		}
		parentNode = resolved
	}
	path, err := nodes.PathFor(id, parentNode.ID)
	if err != nil {
		return "", "", err
	}
	return path, parentNode.Path, nil
}
