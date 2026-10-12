package tree

import (
	"bytes"
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
	// Original is the file's content before the move, written back to take the rewrite back.
	Original []byte
	// Content is the file's content after the move.
	Content []byte
}

// Move is a rename and re-parent of one node, worked out by PlanMove before anything on disk
// changes, then carried out by Apply. It knows nothing of the node files' format: a caller that
// changes file contents adds them with AddRewrite.
type Move struct {
	// Node is the moved node, as it was before the move.
	Node Node
	// NewID is the node's id after the move.
	NewID string
	// NewPath is the node file after the move.
	NewPath string

	rewrites []moveStep
	renames  []moveStep
	// rename renames a file or directory; tests swap it to inject a failure.
	rename func(oldPath string, newPath string) error
}

// moveStep is one change a Move makes on disk and the change that takes it back.
type moveStep struct {
	apply func() error
	undo  func() error
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

	move := &Move{Node: node, NewID: to.ID, NewPath: newPath, rename: os.Rename}
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

// AddRewrite adds a file whose content the move replaces. Apply writes every rewrite atomically at
// the file's current path, in the order added, before it makes any rename. Taking a rewrite back
// writes Original byte for byte, CRLF included, with the permissions the file had just before
// Apply overwrote it.
func (m *Move) AddRewrite(rewrite Rewrite) {
	var mode fs.FileMode
	m.rewrites = append(m.rewrites, moveStep{
		apply: func() error {
			info, err := os.Stat(rewrite.Path)
			if err != nil {
				return err
			}
			mode = info.Mode()
			return WriteFileAtomic(rewrite.Path, bytes.NewReader(rewrite.Content))
		},
		undo: func() error {
			return WriteFileAtomicVerbatim(rewrite.Path, bytes.NewReader(rewrite.Original), mode)
		},
	})
}

// planRenames adds the renames that carry m.Node to m.NewPath, in this fixed order: create the
// destination directory when the new parent has no children yet, rename the node file, rename its
// children directory when it has one, and remove the old parent's directory when the move left it
// empty.
func (m *Move) planRenames(nodes *Tree) {
	node := m.Node
	newDir := filepath.Dir(m.NewPath)
	if _, err := os.Stat(newDir); errors.Is(err, fs.ErrNotExist) {
		m.renames = append(m.renames, moveStep{
			apply: func() error { return os.Mkdir(newDir, movedDirPermissions) },
			undo:  func() error { return os.Remove(newDir) },
		})
	}

	m.renames = append(m.renames, m.renameStep(node.Path, m.NewPath))
	childrenDir := strings.TrimSuffix(node.Path, nodeExtension)
	if info, err := os.Stat(childrenDir); err == nil && info.IsDir() {
		newChildrenDir := strings.TrimSuffix(m.NewPath, nodeExtension)
		m.renames = append(m.renames, m.renameStep(childrenDir, newChildrenDir))
	}

	oldDir := filepath.Dir(node.Path)
	if node.ParentID != "" && oldDir != newDir && len(nodes.ChildrenOf(node.ParentID)) == 1 {
		m.renames = append(m.renames, removeEmptyDirStep(oldDir))
	}
}

// renameStep renames oldPath to newPath, refusing to replace anything already at newPath, and
// renames it back to undo it.
func (m *Move) renameStep(oldPath string, newPath string) moveStep {
	return moveStep{
		apply: func() error {
			if _, err := os.Lstat(newPath); !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("%s: %w", newPath, ErrMoveTarget)
			}
			return m.rename(oldPath, newPath)
		},
		undo: func() error { return m.rename(newPath, oldPath) },
	}
}

// removeEmptyDirStep removes dir when nothing is left in it, and creates it again to undo that.
func removeEmptyDirStep(dir string) moveStep {
	isRemoved := false
	return moveStep{
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

// Apply carries out the move: it writes every rewrite atomically at its file's current path, then
// makes the renames in PlanMove's fixed order. When any step fails it takes back the steps already
// made, newest first, so the tree is as it was; the error names the failing path and any step that
// could not be taken back. Apply must run at most once.
func (m *Move) Apply() error {
	steps := slices.Concat(m.rewrites, m.renames)
	for applied, step := range steps {
		if err := step.apply(); err != nil {
			return rollBack(steps[:applied], err)
		}
	}
	return nil
}

// rollBack undoes the applied steps, newest first, and returns cause with every failure to undo
// one appended.
func rollBack(applied []moveStep, cause error) error {
	failures := []error{cause}
	for index := len(applied) - 1; index >= 0; index-- {
		if err := applied[index].undo(); err != nil {
			failures = append(failures, fmt.Errorf("rolling back: %w", err))
		}
	}
	return errors.Join(failures...)
}
