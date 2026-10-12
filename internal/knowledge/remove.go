package knowledge

import (
	"errors"
	"slices"

	"github.com/airiclenz/capcom/internal/rawfm"
	"github.com/airiclenz/capcom/internal/tree"
)

// removeKind is the noun phrase, with its article, that the refusals of a Note removal name a Note
// by. It is lowercase, as the other Note errors of the CLI are.
const removeKind = moveKind

// Removal is the removal of one Note: the tree removal of its file and its empty children
// directory, with the files of the Notes whose superseded-by entries naming it are dropped added as
// rewrites, which Apply writes before it removes anything.
type Removal struct {
	*tree.Removal
	// Original is the removed Note file's content, as read from disk.
	Original []byte
	// SupersededByDropped are the ids of the Notes whose superseded-by entries naming the removed
	// Note are dropped, in tree order.
	SupersededByDropped []string
	// Unread are the files of other Notes that do not parse, or that tree.ReadNodeFile refuses
	// unread, so their superseded-by entries could not be checked for the removed Note and were
	// left as they are.
	Unread []string
}

// PlanRemove works out the removal of the Note with exact id id in notes, reading the Note files
// but changing nothing. Every other Note's superseded-by entries that name the Note, by its id or
// one of its Aliases, are dropped. The Note's own file is read but never parsed, so a Note whose
// file does not parse is removed like any other: its Aliases are the node's, or those rawfm still
// reads from the file when the node has none.
//
// PlanRemove fails as tree.PlanRemove does, naming a Note, then with the *fs.PathError of a file
// it cannot read.
func PlanRemove(notes *tree.Tree, id string) (*Removal, error) {
	planned, err := notes.PlanRemove(id, removeKind)
	if err != nil {
		return nil, err
	}
	data, err := tree.ReadNodeFile(planned.Node.Path)
	if err != nil {
		return nil, err
	}

	removal := &Removal{Removal: planned, Original: data}
	aliases := planned.Node.Aliases
	if len(aliases) == 0 {
		aliases = rawfm.Aliases(data)
	}
	rewrites, err := removal.planDrops(notes, append([]string{planned.Node.ID}, aliases...))
	if err != nil {
		return nil, err
	}
	for _, rewrite := range rewrites {
		planned.AddRewrite(rewrite)
	}
	return removal, nil
}

// planDrops reads every other Note file of notes through tree.ReadNodeFile and returns, in tree order, the files whose
// superseded-by entries are one of names, without those entries. It sets r.SupersededByDropped and
// r.Unread.
func (r *Removal) planDrops(notes *tree.Tree, names []string) ([]tree.Rewrite, error) {
	var rewrites []tree.Rewrite
	for _, other := range notes.Nodes() {
		if other.ID == r.Node.ID {
			continue
		}
		data, err := tree.ReadNodeFile(other.Path)
		if isRefusedRead(err) {
			r.Unread = append(r.Unread, other.Path)
			continue
		}
		if err != nil {
			return nil, err
		}
		parsed, err := Parse(other.Path, data)
		if err != nil {
			r.Unread = append(r.Unread, other.Path)
			continue
		}
		kept := slices.DeleteFunc(slices.Clone(parsed.SupersededBy), isOneOf(names))
		if len(kept) == len(parsed.SupersededBy) {
			continue
		}

		parsed.SupersededBy = kept
		r.SupersededByDropped = append(r.SupersededByDropped, other.ID)
		rewrites = append(rewrites, tree.Rewrite{Path: other.Path, Original: data, Content: parsed.Serialize()})
	}
	return rewrites, nil
}

// isRefusedRead reports whether err is tree.ReadNodeFile refusing a file unread: one that is not a
// regular file, or is larger than 1 MiB.
func isRefusedRead(err error) bool {
	return errors.Is(err, tree.ErrNotRegular) || errors.Is(err, tree.ErrTooLarge)
}
