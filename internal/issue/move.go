package issue

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/tree"
)

// nodeFileExtension is the extension of an issue file; its children directory is the file's path
// without it.
const nodeFileExtension = ".md"

// movedDirPermissions are the permissions of a children directory a move creates.
const movedDirPermissions = 0o755

// ErrMoveIntoSubtree reports a move that would place an issue below itself or one of its own
// descendants.
var ErrMoveIntoSubtree = errors.New("cannot move an issue below itself or its own descendants")

// ErrMoveTarget reports a path a move would rename onto that is already there on disk.
var ErrMoveTarget = errors.New("the move's target path already exists")

// TakenError refuses a new id that another issue already holds as its id or as an Alias. It
// matches tree.ErrDuplicate under errors.Is.
type TakenError struct {
	// ID is the refused new id.
	ID string
	// Owner is the issue that holds it.
	Owner tree.Node
}

// Error names the id and the issue that holds it.
func (e *TakenError) Error() string {
	if e.Owner.ID == e.ID {
		return fmt.Sprintf("%v id %q: already taken by issue %s", tree.ErrDuplicate, e.ID, e.Owner.ID)
	}
	return fmt.Sprintf("%v id %q: already an alias of issue %s", tree.ErrDuplicate, e.ID, e.Owner.ID)
}

// Unwrap returns tree.ErrDuplicate.
func (e *TakenError) Unwrap() error {
	return tree.ErrDuplicate
}

// Destination is where a move puts an issue.
type Destination struct {
	// ID is the issue's id after the move; its current id leaves it unrenamed.
	ID string
	// ParentID is the id of the issue it is to sit directly below, "" for the top of the tree; its
	// current parent leaves it in place.
	ParentID string
}

// Move is a rename and re-parent of one issue, worked out in full by PlanMove before anything on
// disk changes, then carried out by Apply.
type Move struct {
	// ID is the issue's id before the move.
	ID string
	// NewID is the issue's id after it.
	NewID string
	// NewPath is the issue file after the move.
	NewPath string
	// Content is the issue file's content after the move.
	Content []byte
	// Unread are the files of other issues that do not parse, so their deps could not be checked
	// for the old id and were left as they are.
	Unread []string

	steps []moveStep
	// rename renames a file or directory; tests swap it to inject a failure.
	rename func(oldPath string, newPath string) error
}

// moveStep is one change a Move makes on disk and the change that takes it back.
type moveStep struct {
	apply func() error
	undo  func() error
}

// fileRewrite is one issue file whose content a Move replaces.
type fileRewrite struct {
	path     string
	original []byte
	content  []byte
}

// PlanMove works out the move of the issue with exact id id in issues to the destination to,
// reading the issue files but changing nothing. A rename adds the old id to the issue's Aliases
// (or, when the new id is one of its own Aliases, swaps the two) and rewrites every deps entry in
// the tree that names the old id or one of its Aliases to the new id; body text is never
// rewritten. The issue's children directory, with its whole subtree, moves along with it.
//
// PlanMove fails with ErrInvalidID (tree) for an invalid new id, a *TakenError for a new id
// another issue holds, ErrNotFound (tree) for an unknown issue or parent, ErrMoveIntoSubtree for a
// parent at or below the issue itself, and the issue's own *Error when its file does not parse.
func PlanMove(issues *tree.Tree, id string, to Destination) (*Move, error) {
	node, err := lookupExact(issues, id)
	if err != nil {
		return nil, err
	}
	if err := checkDestination(issues, node, to); err != nil {
		return nil, err
	}
	newPath, err := issues.PathFor(to.ID, to.ParentID)
	if err != nil {
		return nil, err
	}

	move := &Move{ID: node.ID, NewID: to.ID, NewPath: newPath, rename: os.Rename}
	rewrites, err := move.planRewrites(issues, node)
	if err != nil {
		return nil, err
	}
	for _, rewrite := range rewrites {
		move.steps = append(move.steps, rewriteStep(rewrite))
	}
	if newPath != node.Path {
		move.planRenames(node, issues)
	}
	return move, nil
}

// lookupExact returns the node whose id is exactly id.
func lookupExact(issues *tree.Tree, id string) (tree.Node, error) {
	node, isFound := issues.Lookup(id)
	if !isFound || node.ID != id {
		return tree.Node{}, fmt.Errorf("id %q %w", id, tree.ErrNotFound)
	}
	return node, nil
}

// checkDestination refuses a new id that is invalid or held by another issue, and a parent that
// is unknown or lies at or below node.
func checkDestination(issues *tree.Tree, node tree.Node, to Destination) error {
	if to.ID != node.ID {
		if err := tree.ValidateID(to.ID); err != nil {
			return err
		}
		if owner, isTaken := issues.Lookup(to.ID); isTaken && owner.ID != node.ID {
			return &TakenError{ID: to.ID, Owner: owner}
		}
	}
	for ancestor := to.ParentID; ancestor != ""; {
		if ancestor == node.ID {
			return fmt.Errorf("%w: %s cannot go below %s", ErrMoveIntoSubtree, node.ID, to.ParentID)
		}
		parent, err := lookupExact(issues, ancestor)
		if err != nil {
			return fmt.Errorf("parent %w", err)
		}
		ancestor = parent.ParentID
	}
	return nil
}

// planRewrites reads the moved issue's file, and on a rename every other issue file, and returns
// the files whose content the move changes, the moved issue's first. It sets m.Content and
// m.Unread.
func (m *Move) planRewrites(issues *tree.Tree, node tree.Node) ([]fileRewrite, error) {
	names := append([]string{node.ID}, node.Aliases...)
	var rewrites []fileRewrite
	for _, other := range issues.Nodes() {
		isMoved := other.ID == node.ID
		if !isMoved && m.NewID == m.ID {
			continue
		}
		rewrite, err := m.planRewrite(other.Path, names, isMoved)
		if err != nil {
			return nil, err
		}
		switch {
		case rewrite == nil:
		case isMoved:
			rewrites = slices.Insert(rewrites, 0, *rewrite)
		default:
			rewrites = append(rewrites, *rewrite)
		}
	}
	return rewrites, nil
}

// planRewrite reads the issue file at path and returns its rewrite, or nil when the move leaves
// it as it is: deps naming one of names become m.NewID and, for the moved issue (isMoved), the
// Aliases record the old id. A file of another issue that does not parse joins m.Unread; the
// moved issue's own content goes to m.Content.
func (m *Move) planRewrite(path string, names []string, isMoved bool) (*fileRewrite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	parsed, err := Parse(path, data)
	if err != nil && !isMoved {
		m.Unread = append(m.Unread, path)
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	edited := parsed
	edited.Deps = renameDeps(parsed.Deps, names, m.NewID)
	if isMoved {
		edited.Aliases = renameAliases(parsed.Aliases, m.ID, m.NewID)
		m.Content = data
	}
	if slices.Equal(edited.Deps, parsed.Deps) && slices.Equal(edited.Aliases, parsed.Aliases) {
		return nil, nil
	}
	rewrite := &fileRewrite{path: path, original: data, content: edited.Serialize()}
	if isMoved {
		m.Content = rewrite.content
	}
	return rewrite, nil
}

// renameDeps returns deps with every entry that is one of names replaced by newID, keeping only
// the first entry that ends up as newID. deps is returned unchanged when names and newID agree.
func renameDeps(deps []string, names []string, newID string) []string {
	if names[0] == newID {
		return deps
	}
	var renamed []string
	for _, dep := range deps {
		if slices.Contains(names, dep) {
			if slices.Contains(renamed, newID) {
				continue
			}
			dep = newID
		}
		renamed = append(renamed, dep)
	}
	return renamed
}

// renameAliases returns the Aliases of an issue renamed from oldID to newID: newID leaves them
// (it may have been a former id) and oldID joins them at the end.
func renameAliases(aliases []string, oldID string, newID string) []string {
	if oldID == newID {
		return aliases
	}
	kept := slices.DeleteFunc(slices.Clone(aliases), func(alias string) bool {
		return alias == newID
	})
	return append(kept, oldID)
}

// rewriteStep writes rewrite's content atomically, and its original content back to undo it.
// The undo writes the original bytes with LF line endings, as every atomic write does.
func rewriteStep(rewrite fileRewrite) moveStep {
	return moveStep{
		apply: func() error {
			return tree.WriteFileAtomic(rewrite.path, bytes.NewReader(rewrite.content))
		},
		undo: func() error {
			return tree.WriteFileAtomic(rewrite.path, bytes.NewReader(rewrite.original))
		},
	}
}

// planRenames adds the renames that carry node to m.NewPath, in this fixed order: create the
// destination directory when the new parent has no children yet, rename the issue file, rename
// its children directory when it has one, and remove the old parent's directory when the move
// left it empty.
func (m *Move) planRenames(node tree.Node, issues *tree.Tree) {
	newDir := filepath.Dir(m.NewPath)
	if _, err := os.Stat(newDir); errors.Is(err, fs.ErrNotExist) {
		m.steps = append(m.steps, moveStep{
			apply: func() error { return os.Mkdir(newDir, movedDirPermissions) },
			undo:  func() error { return os.Remove(newDir) },
		})
	}

	m.steps = append(m.steps, m.renameStep(node.Path, m.NewPath))
	childrenDir := strings.TrimSuffix(node.Path, nodeFileExtension)
	if info, err := os.Stat(childrenDir); err == nil && info.IsDir() {
		m.steps = append(m.steps, m.renameStep(childrenDir, strings.TrimSuffix(m.NewPath, nodeFileExtension)))
	}

	oldDir := filepath.Dir(node.Path)
	if node.ParentID != "" && oldDir != newDir && len(issues.ChildrenOf(node.ParentID)) == 1 {
		m.steps = append(m.steps, removeEmptyDirStep(oldDir))
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

// Apply carries out the move: it writes every changed issue file atomically at its current path,
// then makes the renames in PlanMove's fixed order. When any step fails it takes back the steps
// already made, newest first, so the tree is as it was; the error names the failing path and any
// step that could not be taken back. Apply must run at most once.
func (m *Move) Apply() error {
	for applied, step := range m.steps {
		if err := step.apply(); err != nil {
			return m.rollBack(applied, err)
		}
	}
	return nil
}

// rollBack undoes the first applied steps, newest first, and returns cause with every failure to
// undo one appended.
func (m *Move) rollBack(applied int, cause error) error {
	failures := []error{cause}
	for index := applied - 1; index >= 0; index-- {
		if err := m.steps[index].undo(); err != nil {
			failures = append(failures, fmt.Errorf("rolling back: %w", err))
		}
	}
	return errors.Join(failures...)
}
