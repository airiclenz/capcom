package tree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/airiclenz/capcom/internal/nodefile"
)

// readDir lists a directory for the walk; a test swaps it to change the tree mid-walk.
var readDir = os.ReadDir

// Walk lists the node files of the tree stored in dir: the one directory walk of ADR 0001, which
// Load and the index both build on. A missing dir walks as an empty tree. Names starting with a dot
// and files not ending in .md are skipped. Each Node's Path is joined onto dir and its Aliases are
// nil; the nodes come in walk order, each directory's files by name before its child directories.
//
// Walk reports, as an *Error naming the file or directory, a node file whose name is not a valid id
// (wrapping ErrInvalidID), an entry that is neither a regular file nor a directory, a symlink
// included, whatever its name (wrapping ErrNotRegular), a node file larger than 1 MiB by its Lstat
// (wrapping ErrTooLarge) and a child directory that has no node file beside it that the walk kept
// (wrapping ErrOrphanDirectory), in walk order. Such a file, and such a directory with its whole
// subtree, are left out; no file is opened, so a named pipe never blocks the walk. A directory that
// cannot be read stops the walk, and so does a node file whose Lstat fails other than by being
// gone: its os error comes back unchanged, with no nodes and no problems. A child directory gone
// since its parent was read walks as empty.
func Walk(dir string) ([]Node, Errors, error) {
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	var walker treeWalker
	if err := walker.walk(dir, rootParentID); err != nil {
		return nil, nil, err
	}
	return walker.nodes, walker.problems, nil
}

// treeWalker collects the nodes and the problems of one Walk.
type treeWalker struct {
	nodes    []Node
	problems Errors
}

// walk collects the nodes of dir, all children of parentID, and recurses into the child
// directories that have their node file, adding every entry left out — an invalid id, an entry
// that is not a regular file, a node file over 1 MiB, an orphan directory — to problems. A child
// directory gone since its parent was read walks as empty; any other failure to read dir is
// returned unchanged.
func (w *treeWalker) walk(dir string, parentID string) error {
	entries, err := readDir(dir)
	if errors.Is(err, fs.ErrNotExist) && parentID != rootParentID {
		return nil
	}
	if err != nil {
		return err
	}

	validIDs := map[string]bool{}
	leftOutIDs := map[string]string{}
	var childDirs []string
	for _, entry := range entries {
		name := entry.Name()
		if isSkippedEntry(entry) {
			continue
		}
		if entry.IsDir() {
			childDirs = append(childDirs, name)
			continue
		}
		id, isNode := strings.CutSuffix(name, nodeExtension)
		path := filepath.Join(dir, name)
		leftOut, err := leftOutFileOf(entry, id)
		if err != nil {
			return err
		}
		if leftOut != nil {
			w.problems = append(w.problems, &Error{Path: path, Err: leftOut.problem})
			if isNode {
				leftOutIDs[id] = leftOut.reason
			}
			continue
		}
		validIDs[id] = true
		w.nodes = append(w.nodes, Node{ID: id, Path: path, ParentID: parentID})
	}

	for _, name := range childDirs {
		path := filepath.Join(dir, name)
		switch {
		case validIDs[name]:
			if err := w.walk(path, name); err != nil {
				return err
			}
		case leftOutIDs[name] != "":
			w.problems = append(w.problems, &Error{
				Path: path,
				Err:  &invalidIDDirectoryError{name: name, reason: leftOutIDs[name]},
			})
		default:
			w.problems = append(w.problems, &Error{
				Path: path,
				Err:  fmt.Errorf("%w: no %s%s beside it; left out", ErrOrphanDirectory, name, nodeExtension),
			})
		}
	}
	return nil
}

// isSkippedEntry reports whether a walk passes over entry without a word: an entry whose name
// starts with a dot, or a regular file whose name does not end in .md. A directory, a node file by
// its name and an entry that is not a regular file are not skipped.
func isSkippedEntry(entry fs.DirEntry) bool {
	name := entry.Name()
	if strings.HasPrefix(name, hiddenPrefix) {
		return true
	}
	if entry.IsDir() {
		return false
	}
	return !strings.HasSuffix(name, nodeExtension) && !nodefile.IsNotRegular(entry.Type())
}

// leftOutFile is why a walk leaves a file out: the problem reported at the file, and the reason
// the directory beside it is left out too.
type leftOutFile struct {
	problem error
	reason  string
}

// leftOutFileOf returns why the walk leaves out entry, a file that is a node file by its name (id
// is its name without .md) or is not a regular file: ErrNotRegular, an invalid id, or ErrTooLarge
// by its Lstat size. It returns nil for a node file the walk keeps, and for one gone since its
// directory was read. A failed Lstat is returned unchanged.
func leftOutFileOf(entry fs.DirEntry, id string) (*leftOutFile, error) {
	if nodefile.IsNotRegular(entry.Type()) {
		return &leftOutFile{problem: ErrNotRegular, reason: notRegularReason}, nil
	}
	if err := ValidateID(id); err != nil {
		return &leftOutFile{problem: err, reason: invalidIDReason}, nil
	}
	info, err := entry.Info()
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Size() > nodefile.MaxNodeFileSize {
		return &leftOutFile{problem: ErrTooLarge, reason: tooLargeReason}, nil
	}
	return nil, nil
}
