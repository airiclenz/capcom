package knowledge

import (
	"os"

	"github.com/airiclenz/capcom/internal/tree"
)

// moveKind is the noun phrase, with its article, that the errors of a Note move name a Note by.
// It is lowercase, as the other Note errors of the CLI are.
const moveKind = "a note"

// Move is a rename and re-parent of one Note: the tree move that carries its file and subtree,
// with the Note file's new content added as a rewrite when a rename changes its Aliases.
type Move struct {
	*tree.Move
	// Original is the Note file's content before the move, as read from disk.
	Original []byte
	// Content is the Note file's content after the move.
	Content []byte
}

// PlanMove works out the move of the Note with exact id id in notes to the destination to, reading
// the Note file but changing nothing. A rename adds the old id to the Note's Aliases (or, when the
// new id is one of its own Aliases, swaps the two); Notes have no deps, so no other file changes
// and body text is never rewritten. The Note's children directory, with its whole subtree, moves
// along with it.
//
// PlanMove checks the destination before it reads any file, failing as tree.PlanMove does: with
// ErrInvalidID (tree) for an invalid new id, a *TakenError (tree) for a new id another Note holds,
// ErrNotFound (tree) for an unknown Note or parent and ErrMoveIntoSubtree (tree) for a parent at
// or below the Note itself. It then fails with the Note's own *Error when its file does not parse.
func PlanMove(notes *tree.Tree, id string, to tree.Destination) (*Move, error) {
	planned, err := notes.PlanMove(id, to, moveKind)
	if err != nil {
		return nil, err
	}
	path := planned.Node.Path
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	note, err := Parse(path, data)
	if err != nil {
		return nil, err
	}

	move := &Move{Move: planned, Original: data, Content: data}
	if planned.NewID == planned.Node.ID {
		return move, nil
	}
	note.Aliases = tree.RenameAliases(note.Aliases, planned.Node.ID, planned.NewID)
	move.Content = note.Serialize()
	planned.AddRewrite(tree.Rewrite{Path: path, Original: data, Content: move.Content})
	return move, nil
}
