package nodestore

import (
	"errors"

	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/tree"
)

// Destination is where Move takes a node: a new id, a new parent, or both.
type Destination struct {
	// ID is the node's new id; "" keeps its id.
	ID string
	// Parent names the node to move it below (its id, one of its Aliases or a unique id prefix);
	// "" is the top of the tree. It is used only when IsReparented.
	Parent string
	// IsReparented reports whether the node moves below Parent; otherwise it keeps its parent.
	IsReparented bool
}

// Moved is the outcome of a Move: the node written at its new id and file, with its new etag, and
// where it was before. Every path is relative to the working tree root with forward slashes.
type Moved struct {
	Written
	// OldID is the node's id before the move.
	OldID string
	// OldPath is the node file before the move.
	OldPath string
	// Unread are the files of other nodes that do not parse, so the references in them to the
	// moved node could not be checked and were left as they are.
	Unread []string
}

// Move renames the node ref names (its id, one of its Aliases or a unique id prefix), moves it
// below another, or both, as to says, carrying its subtree along. A rename keeps the old id as an
// Alias; the kind's planner rewrites the files that reference it. It returns the node moved and
// the tree problems that do not concern it, their paths relative to the working tree root with
// forward slashes, for the caller to report as warnings once it has renamed the paths inside the
// moved children directory.
//
// It resolves ref and to.Parent against the Tree it loads, never through the Resolver, and runs,
// in this order, stopping at the first failure:
//  1. the Tree load: a tree that cannot be loaded at all fails with its *tree.Error or tree.Errors.
//  2. the resolution of ref, then of to.Parent: tree.ErrNotFound or tree.ErrAmbiguous, unwrapped,
//     or, for a name two files claim, a *tree.Error wrapping tree.ErrDuplicate naming both files.
//  3. the plan: an id another node holds is an *ExistsError; the node file that does not parse, or
//     any other refusal (an invalid id, a parent inside the node's own subtree), a *FileError.
//  4. on a rename, a node file inside an orphan directory named the new id or recording it among
//     its Aliases, an *ExistsError.
//  5. on a rename, a file a repeated-id problem of the load names, named the new id or recording
//     it among its Aliases, an *ExistsError.
//  6. triage: a tree problem naming the node's file, its new file, the new parent's file or one of
//     their children directories blocks the move and comes back as the failure, a tree.Errors.
//  7. on a rename, a node file that does not parse but whose raw frontmatter still lists the new
//     id as an alias, an *ExistsError.
//  8. the etag check: when ifMatch is set and is not the node file's etag, a *ConflictError.
//  9. the renames and rewrites, taken back on failure: a *FileError at the node's file naming the
//     failing path and any step that could not be taken back.
func (k *Kind[T]) Move(ref string, to Destination, ifMatch string) (Moved, tree.Errors, error) {
	nodes, problems, err := k.load()
	if err != nil {
		return Moved{}, nil, err
	}
	node, err := nodes.Resolve(ref)
	if err != nil {
		return Moved{}, nil, k.resolveFailure(err)
	}
	destination := tree.Destination{ID: node.ID, ParentID: node.ParentID}
	if to.ID != "" {
		destination.ID = to.ID
	}
	parentPath := ""
	if to.IsReparented {
		parent, err := resolveParent(nodes, to.Parent)
		if err != nil {
			return Moved{}, nil, k.resolveFailure(err)
		}
		destination.ParentID, parentPath = parent.ID, parent.Path
	}

	move, err := k.codec.planMove(nodes, node.ID, destination)
	if err != nil {
		return Moved{}, nil, k.planFailure(err, node)
	}
	isRenamed := move.NewID != node.ID
	if isRenamed {
		if err := k.orphanClaimFailure(problems, move.NewID); err != nil {
			return Moved{}, nil, err
		}
		if err := k.duplicateClaimFailure(problems, move.NewID); err != nil {
			return Moved{}, nil, err
		}
	}
	warnings, err := k.triage(problems, k.relative(node.Path), k.relative(move.NewPath), k.relative(parentPath))
	if err != nil {
		return Moved{}, nil, err
	}
	if isRenamed {
		// The plan has parsed the moved node's file, so the holder found is another node's.
		if err := k.unparsedClaimFailure(nodes, problems, move.NewID); err != nil {
			return Moved{}, nil, err
		}
	}
	if current := edit.ETag(move.original); ifMatch != "" && ifMatch != current {
		return Moved{}, nil, k.conflictError(node.Path, ifMatch, current)
	}
	if err := k.applyMove(move.Move); err != nil {
		return Moved{}, nil, k.fileError(node.Path, err)
	}

	moved := Moved{
		Written: Written{ID: move.NewID, Path: k.relative(move.NewPath), ETag: edit.ETag(move.content)},
		OldID:   node.ID,
		OldPath: k.relative(node.Path),
	}
	for _, unread := range move.unread {
		moved.Unread = append(moved.Unread, k.relative(unread))
	}
	return moved, warnings, nil
}

// resolveParent returns the node parent resolves to in nodes, or the zero node, the top of the
// tree, for "". A parent that does not resolve fails as tree.Resolve does.
func resolveParent(nodes *tree.Tree, parent string) (tree.Node, error) {
	if parent == "" {
		return tree.Node{}, nil
	}
	return nodes.Resolve(parent)
}

// planFailure types err, the failure of the plan of a move of node: a new id another node holds
// is an *ExistsError naming that node's file; an unknown node or parent stays tree.ErrNotFound;
// everything else, the node file that does not parse among it, is a *FileError at the node's file,
// at its line when known.
func (k *Kind[T]) planFailure(err error, node tree.Node) error {
	var taken *tree.TakenError
	if errors.As(err, &taken) {
		return k.existsError(taken.ID, taken.Owner)
	}
	if errors.Is(err, tree.ErrNotFound) {
		return err
	}
	return k.fileError(node.Path, err)
}
