package nodestore

import (
	"slices"

	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/tree"
)

// refField is one frontmatter field of a Tree kind whose entries name other nodes of the tree by
// id: an issue's deps and related, a Note's superseded-by.
type refField[T any] struct {
	// key is the field's frontmatter key.
	key string
	// get returns the field's entries of node.
	get func(node T) []string
	// set returns node with the field's entries replaced by entries.
	set func(node T, entries []string) T
	// isSelfRefused reports whether the field may not name its own node: a rename drops the moved
	// node's entry that names its new id.
	isSelfRefused bool
}

// nodeMove is a move of one node worked out by planNodeMove: the tree move, with the rewrites of
// the files whose content it changes added, and the moved node file's content before and after it.
type nodeMove struct {
	*tree.Move
	// original is the moved node file's content before the move, as read from disk.
	original []byte
	// content is the moved node file's content after the move.
	content []byte
	// unread are the files of other nodes that do not parse, or that nodefile.ReadNodeFile refuses
	// unread, so the references in them to the moved node could not be checked and were left as
	// they are.
	unread []string
}

// planNodeMove works out the move of node, a node of nodes, to the destination to, reading the
// node files but changing nothing. The node's children directory, with its whole subtree, moves
// along with it. Every rewrite keeps the bytes it read as its Original, so the move's Apply refuses
// a file changed since.
//
// A rename adds the old id to the node's Aliases (or, when the new id is one of its own Aliases,
// swaps the two) and, in every node file of the tree, the node's own among them, replaces each
// entry of the kind's reference fields that names the old id or one of its Aliases by the new id,
// keeping only the first entry equal to it; a file none of whose entries names them keeps its
// entries as they are. On the node's own self-refused fields, an entry naming the new id is
// dropped. Body text is never rewritten, and a re-parent alone rewrites no file.
//
// It checks the destination before it reads any file, failing as tree.PlanMove does, then fails
// with the error of the node's own file when nodefile.ReadNodeFile refuses it or it does not
// parse, and with the error of another node file it cannot read; another node file that does not
// parse, or that nodefile.ReadNodeFile refuses unread, joins unread instead.
func (k *Kind[T]) planNodeMove(nodes *tree.Tree, node tree.Node, to tree.Destination) (*nodeMove, error) {
	planned, err := nodes.PlanMove(node.ID, to, k.codec.nounPhrase)
	if err != nil {
		return nil, err
	}
	data, err := nodefile.ReadNodeFile(node.Path)
	if err != nil {
		return nil, err
	}
	own, err := k.codec.Parse(node.Path, data)
	if err != nil {
		return nil, err
	}

	move := &nodeMove{Move: planned, original: data, content: data}
	if planned.NewID == node.ID {
		return move, nil
	}
	names := slices.Concat([]string{node.ID}, node.Aliases)
	own = k.codec.setAliases(own, tree.RenameAliases(k.codec.Aliases(own), node.ID, planned.NewID))
	own = k.renameOwnRefs(own, names, planned.NewID)
	move.content = k.codec.Serialize(own)
	planned.AddRewrite(tree.Rewrite{Path: node.Path, Original: data, Content: move.content})

	for _, other := range nodes.Nodes() {
		if other.ID == node.ID {
			continue
		}
		rewrite, isUnread, err := k.planRefRewrite(other.Path, names, planned.NewID)
		if err != nil {
			return nil, err
		}
		if isUnread {
			move.unread = append(move.unread, other.Path)
		}
		if rewrite != nil {
			planned.AddRewrite(*rewrite)
		}
	}
	return move, nil
}

// renameOwnRefs returns own, the renamed node, with its reference fields renamed from names to
// newID, and an entry naming newID dropped from its self-refused fields.
func (k *Kind[T]) renameOwnRefs(own T, names []string, newID string) T {
	for _, field := range k.codec.refs {
		entries, _ := renameRefs(field.get(own), names, newID)
		if field.isSelfRefused {
			entries = slices.DeleteFunc(slices.Clone(entries), func(entry string) bool { return entry == newID })
		}
		own = field.set(own, entries)
	}
	return own
}

// planRefRewrite reads the node file at path, another node's than the renamed one, and returns its
// rewrite with its reference fields renamed from names to newID, or nil when none of its entries
// is one of names. A file that does not parse, or that nodefile.ReadNodeFile refuses unread, is
// reported unread instead; any other read failure is returned.
func (k *Kind[T]) planRefRewrite(path string, names []string, newID string) (*tree.Rewrite, bool, error) {
	data, err := nodefile.ReadNodeFile(path)
	if isRefusedRead(err) {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	node, err := k.codec.Parse(path, data)
	if err != nil {
		return nil, true, nil
	}

	isChanged := false
	for _, field := range k.codec.refs {
		entries, isRenamed := renameRefs(field.get(node), names, newID)
		if isRenamed {
			node = field.set(node, entries)
			isChanged = true
		}
	}
	if !isChanged {
		return nil, false, nil
	}
	return &tree.Rewrite{Path: path, Original: data, Content: k.codec.Serialize(node)}, false, nil
}

// renameRefs returns entries, the entries of one reference field, with every entry that is one of
// names replaced by newID, keeping only the first entry equal to newID, so [x, y] renamed from x
// to y is [y]; it reports whether any entry was one of names. entries is returned unchanged when
// none is.
func renameRefs(entries []string, names []string, newID string) ([]string, bool) {
	if !slices.ContainsFunc(entries, func(entry string) bool { return slices.Contains(names, entry) }) {
		return entries, false
	}
	renamed := make([]string, 0, len(entries))
	for _, entry := range entries {
		if slices.Contains(names, entry) {
			entry = newID
		}
		if entry == newID && slices.Contains(renamed, newID) {
			continue
		}
		renamed = append(renamed, entry)
	}
	return renamed, true
}
