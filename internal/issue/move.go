package issue

import (
	"slices"

	"github.com/airiclenz/capcom/internal/tree"
)

// moveKind is the noun phrase, with its article, that the errors of an issue move name an issue by.
const moveKind = "an issue"

// Move is a rename and re-parent of one issue: the tree move that carries its file and subtree,
// with the issue files whose content the move changes added as rewrites.
type Move struct {
	*tree.Move
	// Original is the issue file's content before the move, as read from disk.
	Original []byte
	// Content is the issue file's content after the move.
	Content []byte
	// Unread are the files of other issues that do not parse, or that tree.ReadNodeFile refuses
	// unread, so their deps and related entries could not be checked for the old id and were left
	// as they are.
	Unread []string
}

// PlanMove works out the move of the issue with exact id id in issues to the destination to,
// reading the issue files but changing nothing. A rename adds the old id to the issue's Aliases
// (or, when the new id is one of its own Aliases, swaps the two) and rewrites every deps and
// related entry in the tree that names the old id or one of its Aliases to the new id; body text is
// never rewritten. The issue's children directory, with its whole subtree, moves along with it.
//
// PlanMove checks the destination before it reads any file, failing as tree.PlanMove does: with
// ErrInvalidID (tree) for an invalid new id, a *TakenError (tree) for a new id another issue
// holds, ErrNotFound (tree) for an unknown issue or parent and ErrMoveIntoSubtree (tree) for a
// parent at or below the issue itself. It then fails with the issue's own *Error when its file
// does not parse.
func PlanMove(issues *tree.Tree, id string, to tree.Destination) (*Move, error) {
	planned, err := issues.PlanMove(id, to, moveKind)
	if err != nil {
		return nil, err
	}

	move := &Move{Move: planned}
	rewrites, err := move.planRewrites(issues)
	if err != nil {
		return nil, err
	}
	for _, rewrite := range rewrites {
		planned.AddRewrite(rewrite)
	}
	return move, nil
}

// planRewrites reads the moved issue's file, and on a rename every other issue file, and returns
// the files whose content the move changes, the moved issue's first. It sets m.Original,
// m.Content and m.Unread.
func (m *Move) planRewrites(issues *tree.Tree) ([]tree.Rewrite, error) {
	names := append([]string{m.Node.ID}, m.Node.Aliases...)
	var rewrites []tree.Rewrite
	for _, other := range issues.Nodes() {
		isMoved := other.ID == m.Node.ID
		if !isMoved && m.NewID == m.Node.ID {
			continue
		}
		rewrite, err := m.planRewrite(other.Path, names, isMoved)
		if err != nil {
			return nil, err
		}
		switch {
		case rewrite == nil:
		case isMoved:
			rewrites = slices.Insert(rewrites, 0, *rewrite)
		default:
			rewrites = append(rewrites, *rewrite)
		}
	}
	return rewrites, nil
}

// planRewrite reads the issue file at path through tree.ReadNodeFile and returns its rewrite, or
// nil when the move leaves it as it is: deps and related entries naming one of names become
// m.NewID and, for the moved issue (isMoved), the Aliases record the old id. A file of another
// issue that does not parse, or that tree.ReadNodeFile refuses unread, joins m.Unread; the moved
// issue's own content goes to m.Original and m.Content.
func (m *Move) planRewrite(path string, names []string, isMoved bool) (*tree.Rewrite, error) {
	data, err := tree.ReadNodeFile(path)
	if isRefusedRead(err) && !isMoved {
		m.Unread = append(m.Unread, path)
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	parsed, err := Parse(path, data)
	if err != nil && !isMoved {
		m.Unread = append(m.Unread, path)
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	edited := parsed
	edited.Deps = renameDeps(parsed.Deps, names, m.NewID)
	edited.Related = renameDeps(parsed.Related, names, m.NewID)
	if isMoved {
		edited.Aliases = tree.RenameAliases(parsed.Aliases, m.Node.ID, m.NewID)
		m.Original = data
		m.Content = data
	}
	if slices.Equal(edited.Deps, parsed.Deps) && slices.Equal(edited.Related, parsed.Related) &&
		slices.Equal(edited.Aliases, parsed.Aliases) {
		return nil, nil
	}
	rewrite := &tree.Rewrite{Path: path, Original: data, Content: edited.Serialize()}
	if isMoved {
		m.Content = rewrite.Content
	}
	return rewrite, nil
}

// renameDeps returns deps, a deps or related list, with every entry that is one of names replaced
// by newID, keeping only the first entry that ends up as newID. deps is returned unchanged when
// names and newID agree.
func renameDeps(deps []string, names []string, newID string) []string {
	if names[0] == newID {
		return deps
	}
	var renamed []string
	for _, dep := range deps {
		if slices.Contains(names, dep) {
			if slices.Contains(renamed, newID) {
				continue
			}
			dep = newID
		}
		renamed = append(renamed, dep)
	}
	return renamed
}
