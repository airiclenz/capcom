package nodestore

import (
	"errors"

	"github.com/airiclenz/capcom/internal/tree"
)

// load loads the kind's tree and the problems tree.Load reports while still returning the tree of
// what it could place. A failure that returns no tree is returned with its files relative to the
// working tree root.
func (k *Kind[T]) load() (*tree.Tree, tree.Errors, error) {
	nodes, err := tree.Load(k.dir, k.ReadAliases)
	if err == nil {
		return nodes, nil, nil
	}
	var problems tree.Errors
	if errors.As(err, &problems) {
		return nodes, problems, nil
	}
	var problem *tree.Error
	if errors.As(err, &problem) {
		return nil, nil, k.relativeTreeError(problem)
	}
	return nil, nil, err
}

// triage splits problems by the files a write touches, targets, given relative to the working tree
// root with forward slashes ("" entries are ignored). A problem blocks the write when it names a
// target as its file or as the other file of a duplicate, or is an orphan directory where a
// target's children go; the blocking problems come back as the failure. Every other problem is
// returned, its paths relative to the working tree root.
func (k *Kind[T]) triage(problems tree.Errors, targets ...string) (tree.Errors, error) {
	var blocking tree.Errors
	var rest tree.Errors
	for _, problem := range problems {
		relative := k.relativeTreeError(problem)
		if isBlockingProblem(relative, targets) {
			blocking = append(blocking, relative)
			continue
		}
		rest = append(rest, relative)
	}
	if len(blocking) > 0 {
		return nil, blocking
	}
	return rest, nil
}

// isBlockingProblem reports whether problem, its paths relative like targets, concerns one of
// targets: it is the target's file, the other file of the target's duplicate, or an orphan
// directory at the target's children directory.
func isBlockingProblem(problem *tree.Error, targets []string) bool {
	for _, target := range targets {
		if target == "" {
			continue
		}
		if problem.Path == target || problem.Other == target {
			return true
		}
		childrenDir := tree.ChildrenDir(target)
		if errors.Is(problem, tree.ErrOrphanDirectory) && problem.Path == childrenDir {
			return true
		}
	}
	return false
}

// resolveFailure returns err, a failure of tree.Resolve, with the files of a duplicate's *tree.Error
// relative to the working tree root; any other failure comes back as it is.
func (k *Kind[T]) resolveFailure(err error) error {
	var problem *tree.Error
	if errors.As(err, &problem) {
		return k.relativeTreeError(problem)
	}
	return err
}

// relativeTreeError returns a copy of problem whose Path and Other are relative to the working tree
// root with forward slashes; a path outside it is kept as it is.
func (k *Kind[T]) relativeTreeError(problem *tree.Error) *tree.Error {
	relative := *problem
	relative.Path = k.relative(problem.Path)
	relative.Other = k.relative(problem.Other)
	return &relative
}
