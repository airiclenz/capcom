package stage

import (
	"github.com/airiclenz/capcom/internal/tree"
)

// Plan returns the tree.Plan that makes every staged write on disk, all of them or none. Each file
// staged over one on disk is a rewrite (tree.Plan.AddRewrite) from its Original, so Apply fails
// with tree.ErrChanged when the file no longer holds the bytes the batch found; each staged create
// is a tree.Plan.AddCreate, in the order staged, which makes the missing directories above it.
// Apply makes the rewrites before every other step, as it does in any tree.Plan. Plan changes
// nothing on disk; the Overlay stays usable, and a later Plan holds what was staged by then.
func (o *Overlay) Plan() *tree.Plan {
	var plan tree.Plan
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
