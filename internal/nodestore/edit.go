package nodestore

import (
	"errors"
	"path/filepath"

	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/tree"
)

// errNoResolver is the failure of Edit on a Kind built without a Resolver.
var errNoResolver = errors.New("nodestore: the kind has no resolver")

// Edit changes the existing node ref names (its id, one of its Aliases or a unique id prefix) and
// writes it back, returning the node written. Once ref has resolved, the Written names the node's
// id and file even when Edit fails, its ETag empty, so the caller can locate a failure of change.
//
// It runs, in this order, stopping at the first failure:
//  1. the resolution of ref through the Kind's Resolver, whose failure is returned as it is.
//  2. the read of the node file: a file that cannot be read, or one that is not a regular file or
//     is larger than 1 MiB (tree.ReadNodeFile refuses it unread), fails with a *FileError at it.
//  3. the etag check: when ifMatch is set and is not the file's etag, a *ConflictError.
//  4. the parse: a file that does not parse fails with a *FileError at its line when known.
//  5. change, applied to the parsed node, whose failure is returned unwrapped.
//  6. validation: a node that does not validate fails with a *FileError at the file, listing every
//     problem and citing the first problem's line.
//  7. the atomic write, whose failure is returned as it is.
func (k *Kind[T]) Edit(ref string, ifMatch string, change func(T) (T, error)) (Written, error) {
	if k.resolver == nil {
		return Written{}, errNoResolver
	}
	target, err := k.resolver.Resolve(ref)
	if err != nil {
		return Written{}, err
	}
	located := Written{ID: target.ID, Path: target.Path}
	path := filepath.Join(k.worktreeRoot, filepath.FromSlash(target.Path))

	data, err := tree.ReadNodeFile(path)
	if err != nil {
		return located, k.fileError(path, err)
	}
	if current := edit.ETag(data); ifMatch != "" && ifMatch != current {
		return located, k.conflictError(path, ifMatch, current)
	}
	node, err := k.codec.Parse(target.Path, data)
	if err != nil {
		return located, k.fileError(path, err)
	}
	edited, err := change(node)
	if err != nil {
		return located, err
	}
	written, err := k.write(path, target.ID, edited)
	if err != nil {
		return located, err
	}
	return written, nil
}
