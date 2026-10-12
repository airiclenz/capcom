package knowledge

import "github.com/airiclenz/capcom/internal/tree"

// removeKind is the noun phrase, with its article, that the refusals of a Note removal name a Note
// by. It is lowercase, as the other Note errors of the CLI are.
const removeKind = moveKind

// PlanRemove works out the removal of the Note with exact id id in notes, changing nothing. It
// never reads the Note file, so a Note whose file does not parse is removed like any other; Notes
// have no deps, so no other file changes. It fails as tree.PlanRemove does, naming a Note.
func PlanRemove(notes *tree.Tree, id string) (*tree.Removal, error) {
	return notes.PlanRemove(id, removeKind)
}
