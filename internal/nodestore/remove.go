package nodestore

import (
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/tree"
)

// Removed is the outcome of a Remove: the removed node's id and former file, the entries of other
// nodes the removal dropped, and what it could not check or clean up. Every path is relative to
// the working tree root with forward slashes.
type Removed struct {
	ID   string
	Path string
	// Dropped maps the key of each reference field (RefDeps, RefRelated, RefSupersededBy) to the
	// ids of the nodes whose entries of it naming the removed node were dropped, in tree order; a
	// field nothing was dropped from is absent.
	Dropped map[string][]string
	// Unread are the files of other nodes that do not parse, so their entries could not be checked
	// for the removed node and were left as they are.
	Unread []string
	// LeftInPlace are the emptied directories the removal's cleanup could not remove, each a
	// *FileError at the directory saying why. The removal itself has landed.
	LeftInPlace []*FileError
}

// DependentsError refuses the removal of a node other nodes depend on. Path is its file relative
// to the working tree root with forward slashes, ID its id, Dependents the ids of the nodes whose
// deps name it, and Message names the node and its dependents.
type DependentsError struct {
	Path       string
	ID         string
	Dependents []string
	Message    string
}

// Error renders "path: message".
func (e *DependentsError) Error() string {
	return locatedMessage(e.Path, 0, e.Message)
}

// Remove deletes the node file of the node ref names (its id, one of its Aliases or a unique id
// prefix) together with its own children directory when that is empty, then the parent's children
// directory when the removal emptied it. The node's file is never parsed, so a node whose file
// does not parse is removed like any other. Every other node's entries of the kind's reference
// fields naming the node are dropped, those of a field that blocks a removal (an issue's deps)
// only when shouldDropDeps. It returns the node removed and the tree problems that concern neither
// the node nor its parent, their paths relative to the working tree root with forward slashes, for
// the caller to report as warnings.
//
// It resolves ref against the Tree it loads, never through the Resolver, and runs, in this order,
// stopping at the first failure:
//  1. the Tree load: a tree that cannot be loaded at all fails with its *tree.Error or tree.Errors.
//  2. the resolution of ref: tree.ErrNotFound or tree.ErrAmbiguous, unwrapped, or, for a name two
//     files claim, a *tree.Error wrapping tree.ErrDuplicate that names both files.
//  3. triage: a tree problem naming the node's file, its parent's file or one of their children
//     directories blocks the removal and comes back as the failure, a tree.Errors.
//  4. the children refusals: a node with children is a *FileError at its file, a children
//     directory that holds anything a *FileError at that directory.
//  5. the etag check: when ifMatch is set and is not the node file's etag, a *ConflictError.
//  6. other nodes depending on the node, through a field that blocks a removal, while not
//     shouldDropDeps: a *DependentsError. A file that cannot be read on the way is a *FileError at
//     it.
//  7. the rewrites and removals, taken back on failure: a file the removal rewrites that changed
//     since the plan read it is a *ConflictError at that file naming any step that could not be
//     taken back, with no etags; any other failure a *FileError at the first file or directory of
//     the removal its cause names, else at the node's file, naming any step that could not be
//     taken back.
func (k *Kind[T]) Remove(ref string, ifMatch string, shouldDropDeps bool) (Removed, tree.Errors, error) {
	nodes, problems, err := k.load()
	if err != nil {
		return Removed{}, nil, err
	}
	node, err := nodes.Resolve(ref)
	if err != nil {
		return Removed{}, nil, k.resolveFailure(err)
	}
	parentPath := ""
	if parent, isFound := nodes.Lookup(node.ParentID); isFound {
		parentPath = k.relative(parent.Path)
	}
	warnings, err := k.triage(problems, k.relative(node.Path), parentPath)
	if err != nil {
		return Removed{}, nil, err
	}

	removal, err := k.planNodeRemoval(nodes, node, ifMatch, shouldDropDeps)
	if err != nil {
		return Removed{}, nil, k.removalFailure(err, node)
	}
	leftInPlace, err := k.applyRemoval(removal.Removal)
	if err != nil {
		if changed := k.applyChangedFailure(err); changed != nil {
			return Removed{}, nil, changed
		}
		return Removed{}, nil, k.removalApplyFailure(err, removal)
	}

	removed := Removed{ID: node.ID, Path: k.relative(node.Path), Dropped: removal.dropped}
	for _, unread := range removal.unread {
		removed.Unread = append(removed.Unread, k.relative(unread))
	}
	for _, failure := range leftInPlace {
		removed.LeftInPlace = append(removed.LeftInPlace, k.cleanupFailure(failure, node))
	}
	return removed, warnings, nil
}

// removalFailure types err, a refusal of the removal of node: a stale etag's *ConflictError and
// the dependents' *DependentsError stay as they are, and so does an unknown node's
// tree.ErrNotFound. Everything else is a *FileError at the children directory a
// *tree.ChildrenDirError names or the file an *fs.PathError names, else at the node's file.
func (k *Kind[T]) removalFailure(err error, node tree.Node) error {
	var conflict *ConflictError
	var dependents *DependentsError
	if errors.As(err, &conflict) || errors.As(err, &dependents) || errors.Is(err, tree.ErrNotFound) {
		return err
	}
	path := node.Path
	var childrenDir *tree.ChildrenDirError
	var pathErr *fs.PathError
	switch {
	case errors.As(err, &childrenDir):
		path = childrenDir.Dir
	case errors.As(err, &pathErr):
		path = pathErr.Path
	}
	return &FileError{Path: k.relative(path), Message: k.relativeText(err.Error())}
}

// removalApplyFailure types err, the failure of removal's Apply, as a *FileError at the
// file or directory of the step that failed: the first of removal's paths its cause names, else
// the node's file. The message joins the cause and every change that could not be taken back,
// each on one line, with the paths inside the working tree relative to its root.
func (k *Kind[T]) removalApplyFailure(err error, removal *nodeRemoval) *FileError {
	failures := []error{err}
	if joined, isJoined := err.(interface{ Unwrap() []error }); isJoined {
		failures = joined.Unwrap()
	}
	messages := make([]string, 0, len(failures))
	for _, failure := range failures {
		messages = append(messages, strings.ReplaceAll(failure.Error(), "\n", " "))
	}

	failedPath := removal.Node.Path
	firstIndex := -1
	cause := failures[0].Error()
	for _, path := range removalPaths(removal) {
		index := strings.Index(cause, path+":")
		if index >= 0 && (firstIndex < 0 || index < firstIndex) {
			failedPath, firstIndex = path, index
		}
	}
	return &FileError{
		Path:    k.relative(failedPath),
		Message: k.relativeText(strings.Join(messages, problemSeparator)),
	}
}

// removalPaths returns the files and directory removal's Apply changes: the files of the nodes
// whose entries it drops, the children directory when the node has one, and the node file.
func removalPaths(removal *nodeRemoval) []string {
	paths := slices.Clone(removal.rewritten)
	if removal.ChildrenDir != "" {
		paths = append(paths, removal.ChildrenDir)
	}
	return append(paths, removal.Node.Path)
}

// cleanupFailure types failure, a cleanup of the removal or the move of node that left an emptied
// directory in place, as a *FileError at that directory, the *tree.CleanupError's, else the node's
// parent directory.
func (k *Kind[T]) cleanupFailure(failure error, node tree.Node) *FileError {
	var cleanup *tree.CleanupError
	if errors.As(failure, &cleanup) {
		return &FileError{Path: k.relative(cleanup.Path), Message: k.relativeText(cleanup.Err.Error())}
	}
	return &FileError{Path: k.relative(filepath.Dir(node.Path)), Message: k.relativeText(failure.Error())}
}
