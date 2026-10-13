package tree

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/airiclenz/capcom/internal/nodefile"
)

// ChildrenError refuses the removal of a node that has children in the tree.
type ChildrenError struct {
	// Node is the refused node.
	Node Node
	// Count is the number of its children.
	Count int
	// Kind is the kind of node, as a noun phrase with its article: "an issue", "a note".
	Kind string
}

// Error names the node and counts its children, by the bare noun of its Kind.
func (e *ChildrenError) Error() string {
	noun := bareNoun(e.Kind)
	return fmt.Sprintf("%s %s has %s; remove or move them first", noun, e.Node.ID, childCount(e.Count, noun))
}

// ChildrenDirError refuses the removal of a node whose children directory holds anything, files
// the tree skips such as dotfiles and non-node files among them, so the removal never orphans it.
type ChildrenDirError struct {
	// Dir is the node's children directory.
	Dir string
	// Entries is the number of entries it holds.
	Entries int
	// Kind is the kind of node, as a noun phrase with its article: "an issue", "a note".
	Kind string
}

// Error counts the entries the children directory holds, naming the node by the bare noun of Kind.
func (e *ChildrenDirError) Error() string {
	return fmt.Sprintf(
		"the %s's children directory still holds %s; remove them first",
		bareNoun(e.Kind),
		entryCount(e.Entries),
	)
}

// CleanupError is a best-effort cleanup of a Plan (a removal or a move) that failed: Path is the
// directory it left in place and Err why.
type CleanupError struct {
	// Path is the directory the cleanup left in place.
	Path string
	// Err is why it was left.
	Err error
}

// Error names the directory and why it was left.
func (e *CleanupError) Error() string {
	return e.Path + ": " + e.Err.Error()
}

// Unwrap returns Err.
func (e *CleanupError) Unwrap() error {
	return e.Err
}

// Removal is the removal of one node file, worked out by PlanRemove before anything on disk
// changes, then carried out by Apply of its Plan. It knows nothing of the node files' format: a
// caller that changes other files adds them with AddRewrite, and Apply writes those before it
// removes anything, each only over a file that still holds the rewrite's Original (ErrChanged
// otherwise), and takes one back only over a file that still holds the bytes it wrote. Its steps remove the node's empty children directory, when it has one, and
// then the node file, so no directory is ever left without its node file; its cleanup removes the
// parent's children directory when the removal emptied it.
type Removal struct {
	// Node is the removed node.
	Node Node
	// ChildrenDir is the node's empty children directory, removed before the node file; "" when
	// the node has none.
	ChildrenDir string

	Plan
}

// PlanRemove works out the removal of the node with exact id id, changing nothing on disk. kind
// names the kind of node as a noun phrase with its article ("an issue", "a note") for the errors
// below. Every check runs here, before a caller reads any file for its rewrites; PlanRemove never
// reads the node file, so a node whose file does not parse is removed like any other.
//
// PlanRemove fails with ErrNotFound for an unknown node, a *ChildrenError for a node with
// children, a *ChildrenDirError for a children directory that holds anything, and the
// *fs.PathError of a children directory that cannot be read.
//
// Apply takes back the removals already made when a later one fails: a children directory comes
// back with its permissions, and a node file byte for byte with its permissions. Failing to remove
// the emptied parent directory fails nothing: Apply returns it as a *CleanupError warning.
func (t *Tree) PlanRemove(id string, kind string) (*Removal, error) {
	node, err := t.lookupExact(id)
	if err != nil {
		return nil, err
	}
	if children := t.ChildrenOf(node.ID); len(children) > 0 {
		return nil, &ChildrenError{Node: node, Count: len(children), Kind: kind}
	}
	childrenDir := strings.TrimSuffix(node.Path, nodeExtension)
	hasChildrenDir, err := checkChildrenDir(childrenDir, kind)
	if err != nil {
		return nil, err
	}

	removal := &Removal{Node: node}
	if hasChildrenDir {
		removal.ChildrenDir = childrenDir
		removal.steps = append(removal.steps, removal.removeDirStep(childrenDir))
	}
	removal.steps = append(removal.steps, removal.removeFileStep(node.Path))
	if node.ParentID != "" {
		parentDir := filepath.Dir(node.Path)
		removal.AddCleanup(func() error { return removeEmptyDir(parentDir, removal.doRemove) })
	}
	return removal, nil
}

// checkChildrenDir reports whether dir, the children directory of a node of the kind kind, is
// there as a directory. One that holds anything is refused with a *ChildrenDirError.
func checkChildrenDir(dir string, kind string) (bool, error) {
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	if len(entries) > 0 {
		return false, &ChildrenDirError{Dir: dir, Entries: len(entries), Kind: kind}
	}
	return true, nil
}

// removeDirStep removes the empty directory dir, and creates it again with its permissions to undo
// that.
func (r *Removal) removeDirStep(dir string) step {
	var mode fs.FileMode
	return step{
		apply: func() error {
			info, err := os.Stat(dir)
			if err != nil {
				return err
			}
			mode = info.Mode().Perm()
			return r.doRemove(dir)
		},
		undo: func() error {
			if err := os.Mkdir(dir, mode); err != nil {
				return err
			}
			// Mkdir applies the umask; Chmod gives the directory back exactly the permissions it had.
			return os.Chmod(dir, mode)
		},
	}
}

// removeFileStep removes the file at path, read through nodefile.ReadNodeFile, and writes its bytes
// back with its permissions to undo that.
func (r *Removal) removeFileStep(path string) step {
	var original []byte
	var mode fs.FileMode
	return step{
		apply: func() error {
			info, err := os.Stat(path)
			if err != nil {
				return err
			}
			data, err := nodefile.ReadNodeFile(path)
			if err != nil {
				return err
			}
			original, mode = data, info.Mode()
			return r.doRemove(path)
		},
		undo: func() error {
			return nodefile.WriteFileAtomicVerbatim(path, bytes.NewReader(original), mode)
		},
	}
}

// removeEmptyDir removes dir with remove when nothing is left in it and leaves it when anything is.
// A dir that cannot be read or removed fails with a *CleanupError naming it.
func removeEmptyDir(dir string, remove func(path string) error) error {
	entries, err := os.ReadDir(dir)
	if err == nil && len(entries) > 0 {
		return nil
	}
	if err == nil {
		err = remove(dir)
	}
	if err != nil {
		return &CleanupError{Path: dir, Err: err}
	}
	return nil
}

// childCount phrases count children of the kind noun names: "1 child note", "2 child issues".
func childCount(count int, noun string) string {
	if count == 1 {
		return "1 child " + noun
	}
	return fmt.Sprintf("%d child %ss", count, noun)
}

// entryCount phrases count directory entries: "1 entry", "2 entries".
func entryCount(count int) string {
	if count == 1 {
		return "1 entry"
	}
	return fmt.Sprintf("%d entries", count)
}
