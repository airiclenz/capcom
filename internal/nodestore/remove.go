package nodestore

import (
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/tree"
)

// Removed is the outcome of a Remove: the removed node's id and former file, the entries of other
// nodes the removal dropped, and what it could not check or clean up. Every path is relative to
// the working tree root with forward slashes.
type Removed struct {
	ID   string
	Path string
	// DepsDropped are the ids of the nodes whose deps entries naming the removed node were
	// dropped, in tree order.
	DepsDropped []string
	// RelatedDropped are the ids of the nodes whose related entries naming the removed node were
	// dropped, in tree order.
	RelatedDropped []string
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

// plannedRemoval is a removal a Codec worked out: the tree removal, with the kind's rewrites
// added, and the entries of other nodes it drops.
type plannedRemoval struct {
	*tree.Removal
	// depsDropped and relatedDropped are the ids of the nodes whose deps or related entries naming
	// the removed node the removal drops.
	depsDropped    []string
	relatedDropped []string
	// supersededByDropped are the ids of the Notes whose superseded-by entries naming the removed
	// Note the removal drops.
	supersededByDropped []string
	// unread are the files of other nodes that do not parse, so their entries could not be checked.
	unread []string
}

// Remove deletes the node file of the node ref names (its id, one of its Aliases or a unique id
// prefix) together with its own children directory when that is empty, then the parent's children
// directory when the removal emptied it. The node's file is never parsed, so a node whose file
// does not parse is removed like any other. The issue kind drops every other issue's related
// entries naming the node, and its deps entries too when shouldDropDeps; the Knowledge tree drops
// every other Note's superseded-by entries naming it. It returns the node removed and the tree problems that concern neither the node nor its
// parent, their paths relative to the working tree root with forward slashes, for the caller to
// report as warnings.
//
// It resolves ref against the Tree it loads, never through the Resolver, and runs, in this order,
// stopping at the first failure:
//  1. the Tree load: a tree that cannot be loaded at all fails with its *tree.Error or tree.Errors.
//  2. the resolution of ref: tree.ErrNotFound or tree.ErrAmbiguous, unwrapped.
//  3. triage: a tree problem naming the node's file, its parent's file or one of their children
//     directories blocks the removal and comes back as the failure, a tree.Errors.
//  4. the children refusals: a node with children is a *FileError at its file, a children
//     directory that holds anything a *FileError at that directory.
//  5. the etag check: when ifMatch is set and is not the node file's etag, a *ConflictError.
//  6. on the issue kind, other issues depending on the node while not shouldDropDeps: a
//     *DependentsError. A file that cannot be read on the way is a *FileError at it.
//  7. the rewrites and removals, taken back on failure: a *FileError at the first file or directory
//     of the removal its cause names, else at the node's file, naming any step that could not be
//     taken back.
func (k *Kind[T]) Remove(ref string, ifMatch string, shouldDropDeps bool) (Removed, tree.Errors, error) {
	nodes, problems, err := k.load()
	if err != nil {
		return Removed{}, nil, err
	}
	node, err := nodes.Resolve(ref)
	if err != nil {
		return Removed{}, nil, err
	}
	parentPath := ""
	if parent, isFound := nodes.Lookup(node.ParentID); isFound {
		parentPath = k.relative(parent.Path)
	}
	warnings, err := k.triage(problems, k.relative(node.Path), parentPath)
	if err != nil {
		return Removed{}, nil, err
	}

	removal, err := k.codec.planRemove(nodes, node.ID, ifMatch, shouldDropDeps)
	if err != nil {
		return Removed{}, nil, k.removalFailure(err, node)
	}
	leftInPlace, err := k.applyRemoval(removal.Removal)
	if err != nil {
		return Removed{}, nil, k.removalApplyFailure(err, nodes, removal)
	}

	removed := Removed{
		ID:             node.ID,
		Path:           k.relative(node.Path),
		DepsDropped:    removal.depsDropped,
		RelatedDropped: removal.relatedDropped,
	}
	for _, unread := range removal.unread {
		removed.Unread = append(removed.Unread, k.relative(unread))
	}
	for _, failure := range leftInPlace {
		removed.LeftInPlace = append(removed.LeftInPlace, k.cleanupFailure(failure, node))
	}
	return removed, warnings, nil
}

// planIssueRemove is the removal planner of the issue tree: issue.PlanRemove, which also drops the
// deps and related entries that name the removed issue. A stale ifMatch comes back as a
// *ConflictError at the absolute file with no message, which Remove completes.
func planIssueRemove(nodes *tree.Tree, id string, ifMatch string, shouldDropDeps bool) (*plannedRemoval, error) {
	removal, err := issue.PlanRemove(nodes, id, issue.RemoveOptions{IfMatch: ifMatch, ShouldDropDeps: shouldDropDeps})
	var mismatch *issue.ETagMismatchError
	if errors.As(err, &mismatch) {
		return nil, &ConflictError{Path: mismatch.Path, IfMatch: mismatch.IfMatch, ETag: mismatch.ETag}
	}
	if err != nil {
		return nil, err
	}
	return &plannedRemoval{
		Removal:        removal.Removal,
		depsDropped:    removal.DepsDropped,
		relatedDropped: removal.RelatedDropped,
		unread:         removal.Unread,
	}, nil
}

// planNoteRemove is the removal planner of the Knowledge tree: knowledge.PlanRemove, which also
// drops the superseded-by entries that name the removed Note, then the etag check of the Note file,
// which it reads but never parses. A Note has no deps, so shouldDropDeps changes nothing. A stale
// ifMatch comes back as planIssueRemove's does.
func planNoteRemove(nodes *tree.Tree, id string, ifMatch string, _ bool) (*plannedRemoval, error) {
	removal, err := knowledge.PlanRemove(nodes, id)
	if err != nil {
		return nil, err
	}
	if current := edit.ETag(removal.Original); ifMatch != "" && ifMatch != current {
		return nil, &ConflictError{Path: removal.Node.Path, IfMatch: ifMatch, ETag: current}
	}
	return &plannedRemoval{
		Removal:             removal.Removal,
		supersededByDropped: removal.SupersededByDropped,
		unread:              removal.Unread,
	}, nil
}

// removalFailure types err, a refusal of the removal of node: a stale etag is a *ConflictError,
// dependents a *DependentsError at the node's file, and an unknown node stays tree.ErrNotFound.
// Everything else is a *FileError at the children directory a *tree.ChildrenDirError names or the
// file an *fs.PathError names, else at the node's file.
func (k *Kind[T]) removalFailure(err error, node tree.Node) error {
	var conflict *ConflictError
	if errors.As(err, &conflict) {
		return k.conflictError(conflict.Path, conflict.IfMatch, conflict.ETag)
	}
	var dependents *issue.DependentsError
	if errors.As(err, &dependents) {
		return &DependentsError{
			Path:       k.relative(node.Path),
			ID:         node.ID,
			Dependents: dependents.Dependents,
			Message:    dependents.Error(),
		}
	}
	if errors.Is(err, tree.ErrNotFound) {
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

// removalApplyFailure types err, the failure of removal's Apply in nodes, as a *FileError at the
// file or directory of the step that failed: the first of removal's paths its cause names, else
// the node's file. The message joins the cause and every change that could not be taken back,
// each on one line, with the paths inside the working tree relative to its root.
func (k *Kind[T]) removalApplyFailure(err error, nodes *tree.Tree, removal *plannedRemoval) *FileError {
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
	for _, path := range removalPaths(nodes, removal) {
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
func removalPaths(nodes *tree.Tree, removal *plannedRemoval) []string {
	var paths []string
	for _, id := range slices.Concat(removal.depsDropped, removal.relatedDropped, removal.supersededByDropped) {
		if node, isFound := nodes.Lookup(id); isFound {
			paths = append(paths, node.Path)
		}
	}
	if removal.ChildrenDir != "" {
		paths = append(paths, removal.ChildrenDir)
	}
	return append(paths, removal.Node.Path)
}

// cleanupFailure types failure, a cleanup of the removal of node that left an emptied directory in
// place, as a *FileError at that directory, the *tree.CleanupError's, else the node's parent
// directory.
func (k *Kind[T]) cleanupFailure(failure error, node tree.Node) *FileError {
	var cleanup *tree.CleanupError
	if errors.As(failure, &cleanup) {
		return &FileError{Path: k.relative(cleanup.Path), Message: k.relativeText(cleanup.Err.Error())}
	}
	return &FileError{Path: k.relative(filepath.Dir(node.Path)), Message: k.relativeText(failure.Error())}
}
