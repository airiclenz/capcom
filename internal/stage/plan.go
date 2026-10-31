package stage

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/tree"
)

// rewriteOp is the Op of the *fs.PathError the config.yaml raise fails with when the file changed,
// the Op tree.Plan's rewrites fail with.
const rewriteOp = "rewrite"

// Plan returns the tree.Plan that makes every staged write on disk, all of them or none. Each file
// staged over one on disk is a rewrite (tree.Plan.AddRewrite) from its Original, so Apply fails
// with tree.ErrChanged when the file no longer holds the bytes the batch found; each staged create
// is a tree.Plan.AddCreate, in the order staged, which makes the missing directories above it.
// Apply makes the rewrites before every other step, as it does in any tree.Plan; a staged format
// raise comes first among the other steps, made as store.Store.RaiseFormat makes it: a
// config.yaml missing when the batch read it is created holding only the raised format key, and
// one that was there is rewritten byte for byte as config.RaiseFormat returned it, CRLF line
// endings and permission bits kept, only over a file that still holds the bytes the batch read.
// Plan changes nothing on disk; the Overlay stays usable, and a later Plan holds what was staged by
// then.
func (o *Overlay) Plan() *tree.Plan {
	var plan tree.Plan
	if o.format > 0 {
		if o.config.isMissing {
			plan.AddCreate(o.configPath, o.raisedConfig)
		} else {
			addVerbatimRewrite(&plan, o.configPath, o.config.data, o.raisedConfig)
		}
	}
	for _, path := range o.order {
		file := o.files[path]
		if file.isNew {
			plan.AddCreate(path, file.data)
			continue
		}
		plan.AddRewrite(tree.Rewrite{Path: path, Original: o.originals[path], Content: file.data})
	}
	return &plan
}

// addVerbatimRewrite adds to plan a step that replaces the file at path, which must still hold
// original, with content byte for byte, keeping its permission bits, and takes it back by writing
// original byte for byte with those bits, only over a file that still holds content; a file
// changed since is left as it is and named as a failure to take the step back.
func addVerbatimRewrite(plan *tree.Plan, path string, original []byte, content []byte) {
	var mode fs.FileMode
	plan.AddStep(
		func() error {
			info, err := os.Stat(path)
			if err != nil {
				return err
			}
			mode = info.Mode()
			if err := checkUnchanged(path, original); err != nil {
				return err
			}
			return nodefile.ReplaceFileVerbatim(path, bytes.NewReader(content))
		},
		func() error {
			if err := checkUnchanged(path, content); err != nil {
				if errors.Is(err, tree.ErrChanged) {
					return fmt.Errorf("%s: changed since the format raise; left as it is", path)
				}
				return err
			}
			return nodefile.WriteFileAtomicVerbatim(path, bytes.NewReader(original), mode)
		},
	)
}

// checkUnchanged reads the file at path through nodefile.ReadRegularFile and fails with a
// *fs.PathError naming path that wraps tree.ErrChanged when it does not hold expected byte for
// byte. A failed read is returned as it is.
func checkUnchanged(path string, expected []byte) error {
	current, err := nodefile.ReadRegularFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, expected) {
		return &fs.PathError{Op: rewriteOp, Path: path, Err: tree.ErrChanged}
	}
	return nil
}
