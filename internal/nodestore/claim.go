package nodestore

import (
	"fmt"

	"github.com/airiclenz/capcom/internal/tree"
)

// claimRefusal refuses id, the name Tree.Claimant found claim holding, as an *ExistsError at the
// holding file, worded by how the file stands in the tree: a node of the tree, a node file inside
// an orphan directory, a file left out for its repeated id, or a file that does not parse but whose
// raw frontmatter still lists id as an alias. Each of those files would make writing id a
// duplicate, the last three one that surfaces only once the file is fixed.
func (k *Kind[T]) claimRefusal(id string, claim tree.Claim) *ExistsError {
	refusal := &ExistsError{Path: k.relative(claim.Path), ID: id, Holder: claim.Holder}
	how := "taken by"
	if claim.IsAlias {
		how = "an alias of"
	}
	switch claim.Kind {
	case tree.OrphanClaim:
		refusal.Message = fmt.Sprintf(
			"id %q is already %s this file, inside the orphan directory %s",
			id,
			how,
			k.relative(claim.Dir),
		)
	case tree.DuplicateClaim:
		refusal.Message = fmt.Sprintf(
			"id %q is already %s this file, left out for the duplicate id %q",
			id,
			how,
			claim.Holder,
		)
	case tree.UnparsedClaim:
		refusal.Message = fmt.Sprintf(
			"id %q may be an alias of %s %s, whose file does not parse: fix it first",
			id,
			k.codec.Noun,
			claim.Holder,
		)
	default:
		return k.existsError(id, tree.Node{ID: claim.Holder, Path: claim.Path})
	}
	return refusal
}
