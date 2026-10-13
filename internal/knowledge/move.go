package knowledge

import (
	"slices"

	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/tree"
)

// moveKind is the noun phrase, with its article, that the errors of a Note move name a Note by.
// It is lowercase, as the other Note errors of the CLI are.
const moveKind = "a note"

// Move is a rename and re-parent of one Note: the tree move that carries its file and subtree,
// with the Note files whose content the move changes added as rewrites.
type Move struct {
	*tree.Move
	// Original is the Note file's content before the move, as read from disk.
	Original []byte
	// Content is the Note file's content after the move.
	Content []byte
	// Unread are the files of other Notes that do not parse, or that nodefile.ReadNodeFile refuses
	// unread, so their superseded-by entries could not be checked for the old id and were left as
	// they are.
	Unread []string
}

// PlanMove works out the move of the Note with exact id id in notes to the destination to, reading
// the Note files but changing nothing. A rename adds the old id to the Note's Aliases (or, when the
// new id is one of its own Aliases, swaps the two) and rewrites every other Note's superseded-by
// entry that names the old id or one of its Aliases to the new id; body text is never rewritten.
// The Note's children directory, with its whole subtree, moves along with it.
//
// PlanMove checks the destination before it reads any file, failing as tree.PlanMove does: with
// ErrInvalidID (tree) for an invalid new id, a *TakenError (tree) for a new id another Note holds,
// ErrNotFound (tree) for an unknown Note or parent and ErrMoveIntoSubtree (tree) for a parent at
// or below the Note itself. It then fails with the Note's own *Error when its file does not parse,
// and with the *fs.PathError of another Note file it cannot read.
func PlanMove(notes *tree.Tree, id string, to tree.Destination) (*Move, error) {
	planned, err := notes.PlanMove(id, to, moveKind)
	if err != nil {
		return nil, err
	}
	path := planned.Node.Path
	data, err := nodefile.ReadNodeFile(path)
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
	rewrites, err := move.planSuccessorRewrites(notes)
	if err != nil {
		return nil, err
	}
	for _, rewrite := range rewrites {
		planned.AddRewrite(rewrite)
	}
	return move, nil
}

// planSuccessorRewrites reads every other Note file of notes and returns, in tree order, the files
// whose superseded-by entries name the renamed Note, by its id or one of its Aliases, rewritten to
// name its new id. A file that does not parse, or that nodefile.ReadNodeFile refuses unread, joins
// m.Unread.
func (m *Move) planSuccessorRewrites(notes *tree.Tree) ([]tree.Rewrite, error) {
	names := append([]string{m.Node.ID}, m.Node.Aliases...)
	var rewrites []tree.Rewrite
	for _, other := range notes.Nodes() {
		if other.ID == m.Node.ID {
			continue
		}
		data, err := nodefile.ReadNodeFile(other.Path)
		if isRefusedRead(err) {
			m.Unread = append(m.Unread, other.Path)
			continue
		}
		if err != nil {
			return nil, err
		}
		parsed, err := Parse(other.Path, data)
		if err != nil {
			m.Unread = append(m.Unread, other.Path)
			continue
		}
		if !slices.ContainsFunc(parsed.SupersededBy, isOneOf(names)) {
			continue
		}

		parsed.SupersededBy = renameSuccessors(parsed.SupersededBy, names, m.NewID)
		rewrites = append(rewrites, tree.Rewrite{Path: other.Path, Original: data, Content: parsed.Serialize()})
	}
	return rewrites, nil
}

// renameSuccessors returns successors, a superseded-by list, with every entry that is one of names
// replaced by newID, keeping only the first entry that ends up as newID, so [x, y] renamed from x
// to y is [y].
func renameSuccessors(successors []string, names []string, newID string) []string {
	renamed := make([]string, 0, len(successors))
	for _, successor := range successors {
		if slices.Contains(names, successor) {
			successor = newID
		}
		if successor == newID && slices.Contains(renamed, newID) {
			continue
		}
		renamed = append(renamed, successor)
	}
	return renamed
}

// isOneOf returns the predicate that matches an entry that is one of names.
func isOneOf(names []string) func(string) bool {
	return func(entry string) bool { return slices.Contains(names, entry) }
}
