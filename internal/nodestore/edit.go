package nodestore

import (
	"errors"
	"path/filepath"

	"github.com/airiclenz/capcom/internal/nodefile"
)

// errNoResolver is the failure of Edit on a Kind built without a Resolver.
var errNoResolver = errors.New("nodestore: the kind has no resolver")

// beforeRecheck runs between Edit's validation of the changed node and its re-read of the node
// file, and between Format's check of a file's canonical form and its re-read before the rewrite.
// It does nothing in production; tests set it to change the file at that moment.
var beforeRecheck = func(path string) {}

// Edit changes the existing node ref names (its id, one of its Aliases or a unique id prefix) and
// writes it back, returning the node written. Once ref has resolved, the Written names the node's
// id and file even when Edit fails, its ETag empty, so the caller can locate a failure of change.
//
// It runs, in this order, stopping at the first failure:
//  1. the resolution of ref through the Kind's Resolver, whose failure is returned as it is.
//  2. the read of the node file: a file that cannot be read, or one that is not a regular file or
//     is larger than 1 MiB (nodefile.ReadNodeFile refuses it unread), fails with a *FileError at it.
//  3. the etag check: when ifMatch is set and is not the file's etag, a *ConflictError.
//  4. the parse: a file that does not parse fails with a *FileError at its line when known.
//  5. change, applied to the parsed node, whose failure is returned unwrapped.
//  6. validation: a node that does not validate fails with a *FileError at the file, listing every
//     problem and citing the first problem's line.
//  7. the re-check, whether or not ifMatch is set: the node file is read again, a read failure
//     being a *FileError at it as in step 2, and when its etag is no longer the one step 2 read,
//     a *ConflictError naming that first-read etag as stale; the newer file is left as it is.
//  8. the atomic write: a node file removed or moved since the re-check fails with a *FileError at
//     it and is not recreated; any other failure is returned as it is. An edit landing between the
//     re-check and this write is still overwritten: the store takes no lock.
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

	data, err := nodefile.ReadNodeFile(path)
	if err != nil {
		return located, k.fileError(path, err)
	}
	firstETag := nodefile.ETag(data)
	if ifMatch != "" && ifMatch != firstETag {
		return located, k.conflictError(path, ifMatch, firstETag)
	}
	node, err := k.codec.Parse(target.Path, data)
	if err != nil {
		return located, k.fileError(path, err)
	}
	edited, err := change(node)
	if err != nil {
		return located, err
	}
	encoded, err := k.encode(path, edited)
	if err != nil {
		return located, err
	}
	if err := k.recheck(path, firstETag); err != nil {
		return located, err
	}
	written, err := k.place(path, target.ID, encoded, replaceFile)
	if err != nil {
		return located, err
	}
	return written, nil
}

// recheck reads the node file at path again, failing as Edit's step 7 documents when it cannot be
// read or its etag is no longer firstETag, the etag Edit first read.
func (k *Kind[T]) recheck(path string, firstETag string) error {
	beforeRecheck(path)
	data, err := nodefile.ReadNodeFile(path)
	if err != nil {
		return k.fileError(path, err)
	}
	if current := nodefile.ETag(data); current != firstETag {
		return k.conflictError(path, firstETag, current)
	}
	return nil
}
