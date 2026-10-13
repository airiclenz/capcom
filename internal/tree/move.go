package tree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// movedDirPermissions are the permissions of a children directory a move creates.
const movedDirPermissions = 0o755

// ErrMoveIntoSubtree reports a move that would place a node below itself or one of its own
// descendants. PlanMove wraps it in a message that names the kind of node.
var ErrMoveIntoSubtree = errors.New("below itself or its own descendants")

// ErrMoveTarget reports a path a move would rename onto that is already there on disk.
var ErrMoveTarget = errors.New("the move's target path already exists")

// TakenError refuses a new id that another node already holds as its id or as an Alias. It
// matches ErrDuplicate under errors.Is.
type TakenError struct {
	// ID is the refused new id.
	ID string
	// Owner is the node that holds it.
	Owner Node
	// Kind is the kind of node, as a noun phrase with its article: "an issue", "a note".
	Kind string
}

// Error names the id and the node that holds it, by the bare noun of its Kind.
func (e *TakenError) Error() string {
	noun := bareNoun(e.Kind)
	if e.Owner.ID == e.ID {
		return fmt.Sprintf("%v id %q: already taken by %s %s", ErrDuplicate, e.ID, noun, e.Owner.ID)
	}
	return fmt.Sprintf("%v id %q: already an alias of %s %s", ErrDuplicate, e.ID, noun, e.Owner.ID)
}

// Unwrap returns ErrDuplicate.
func (e *TakenError) Unwrap() error {
	return ErrDuplicate
}

// bareNoun returns the noun phrase kind without its leading article: "issue" for "an issue".
func bareNoun(kind string) string {
	if _, noun, hasArticle := strings.Cut(kind, " "); hasArticle {
		return noun
	}
	return kind
}

// Destination is where a move puts a node.
type Destination struct {
	// ID is the node's id after the move; its current id leaves it unrenamed.
	ID string
	// ParentID is the id of the node it is to sit directly below, "" for the top of the tree; its
	// current parent leaves it in place.
	ParentID string
}

// Rewrite is one file whose content a Move replaces.
type Rewrite struct {
	// Path is the file, at its path before the move.
	Path string
	// Original is the file's content before the move: Apply writes the rewrite only over a file
	// that still holds it, and writes it back to take the rewrite back.
	Original []byte
	// Content is the file's content after the move.
	Content []byte
}

// Move is a rename and re-parent of one node, worked out by PlanMove before anything on disk
// changes, then carried out by Apply. It knows nothing of the node files' format: a caller that
// changes file contents adds them with AddRewrite. Its renames are the steps of its Plan.
type Move struct {
	// Node is the moved node, as it was before the move.
	Node Node
	// NewID is the node's id after the move.
	NewID string
	// NewPath is the node file after the move.
	NewPath string

	Plan
}

// PlanMove works out the move of the node with exact id id to the destination to, changing
// nothing on disk. kind names the kind of node as a noun phrase with its article ("an issue",
// "a note") for the errors below. The node's children directory, with its whole subtree, moves
// along with it. Every check runs here, before a caller reads any file for its rewrites.
//
// PlanMove fails with ErrInvalidID for an invalid new id, a *TakenError for a new id another node
// holds, ErrNotFound for an unknown node or parent, and ErrMoveIntoSubtree for a parent at or
// below the node itself.
func (t *Tree) PlanMove(id string, to Destination, kind string) (*Move, error) {
	node, err := t.lookupExact(id)
	if err != nil {
		return nil, err
	}
	if err := t.checkDestination(node, to, kind); err != nil {
		return nil, err
	}
	newPath, err := t.PathFor(to.ID, to.ParentID)
	if err != nil {
		return nil, err
	}

	move := &Move{Node: node, NewID: to.ID, NewPath: newPath}
	if newPath != node.Path {
		move.planRenames(t)
	}
	return move, nil
}

// lookupExact returns the node whose id is exactly id.
func (t *Tree) lookupExact(id string) (Node, error) {
	node, isFound := t.Lookup(id)
	if !isFound || node.ID != id {
		return Node{}, fmt.Errorf("id %q %w", id, ErrNotFound)
	}
	return node, nil
}

// checkDestination refuses a new id that is invalid or held by another node, and a parent that is
// unknown or lies at or below node.
func (t *Tree) checkDestination(node Node, to Destination, kind string) error {
	if to.ID != node.ID {
		if err := ValidateID(to.ID); err != nil {
			return err
		}
		if owner, isTaken := t.Lookup(to.ID); isTaken && owner.ID != node.ID {
			return &TakenError{ID: to.ID, Owner: owner, Kind: kind}
		}
	}
	for ancestor := to.ParentID; ancestor != ""; {
		if ancestor == node.ID {
			return fmt.Errorf(
				"cannot move %s %w: %s cannot go below %s",
				kind,
				ErrMoveIntoSubtree,
				node.ID,
				to.ParentID,
			)
		}
		parent, err := t.lookupExact(ancestor)
		if err != nil {
			return fmt.Errorf("parent %w", err)
		}
		ancestor = parent.ParentID
	}
	return nil
}

// RenameAliases returns the Aliases of a node renamed from oldID to newID: newID leaves them (it
// may have been a former id) and oldID joins them at the end. aliases is returned unchanged when
// the two ids agree.
func RenameAliases(aliases []string, oldID string, newID string) []string {
	if oldID == newID {
		return aliases
	}
	kept := slices.DeleteFunc(slices.Clone(aliases), func(alias string) bool {
		return alias == newID
	})
	return append(kept, oldID)
}

// planRenames adds the steps that carry m.Node to m.NewPath, in this fixed order: create the
// destination directory when the new parent has no children yet, rename the node file, rename its
// children directory when it has one, and remove the old parent's directory when the move left it
// empty.
func (m *Move) planRenames(nodes *Tree) {
	node := m.Node
	newDir := filepath.Dir(m.NewPath)
	if _, err := os.Stat(newDir); errors.Is(err, fs.ErrNotExist) {
		m.steps = append(m.steps, step{
			apply: func() error { return os.Mkdir(newDir, movedDirPermissions) },
			undo:  func() error { return os.Remove(newDir) },
		})
	}

	m.steps = append(m.steps, m.renameStep(node.Path, m.NewPath))
	childrenDir := strings.TrimSuffix(node.Path, nodeExtension)
	if info, err := os.Stat(childrenDir); err == nil && info.IsDir() {
		newChildrenDir := strings.TrimSuffix(m.NewPath, nodeExtension)
		m.steps = append(m.steps, m.renameStep(childrenDir, newChildrenDir))
	}

	oldDir := filepath.Dir(node.Path)
	if node.ParentID != "" && oldDir != newDir && len(nodes.ChildrenOf(node.ParentID)) == 1 {
		m.steps = append(m.steps, removeEmptyDirStep(oldDir))
	}
}

// removeEmptyDirStep removes dir when nothing is left in it, and creates it again to undo that.
func removeEmptyDirStep(dir string) step {
	isRemoved := false
	return step{
		apply: func() error {
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) > 0 {
				return err
			}
			isRemoved = true
			return os.Remove(dir)
		},
		undo: func() error {
			if !isRemoved {
				return nil
			}
			return os.Mkdir(dir, movedDirPermissions)
		},
	}
}

// Apply carries out the move through its Plan: it writes every rewrite atomically at its file's
// current path, failing with ErrChanged on a file that no longer holds the rewrite's Original, then
// makes the renames in PlanMove's fixed order. When any step fails it takes back the steps already
// made, newest first, so the tree is as it was; the error names the failing path and any step that
// could not be taken back, a rewritten file changed since its rewrite among them, left as it is. PlanMove adds no cleanups, so Apply returns no
// warnings; a caller that adds one calls Plan.Apply to have them. Apply must run at most once.
func (m *Move) Apply() error {
	_, err := m.Plan.Apply()
	return err
}
