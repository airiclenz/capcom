package nodestore

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/tree"
)

// The keys of the reference fields of both Tree kinds, as Descriptor.RefKeys and Removed.Dropped
// report them.
const (
	// RefDeps is an issue's deps: the issues it waits for.
	RefDeps = "deps"
	// RefRelated is an issue's related: the issues it is linked to without waiting for them.
	RefRelated = "related"
	// RefSupersededBy is a Note's superseded-by: the Notes that replace it.
	RefSupersededBy = "superseded-by"
)

// refField is one frontmatter field of a Tree kind whose entries name other nodes of the tree by
// id: an issue's deps and related, a Note's superseded-by. The kind's Descriptor lists its fields,
// and the Codec takes them from there.
type refField[T any] struct {
	// key is the field's frontmatter key.
	key string
	// get returns the field's entries of node.
	get func(node T) []string
	// set returns node with the field's entries replaced by entries.
	set func(node T, entries []string) T
	// blocksRemoval reports whether a node holding an entry of the field naming another node
	// depends on it: the removal of that node is refused unless the entries are dropped.
	blocksRemoval bool
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
	// unread are the files of other nodes that do not parse, or that nodefile.ReadRegularFile
	// refuses unread, so the references in them to the moved node could not be checked and were
	// left as they are.
	unread []string
	// unreadNotes are the Note files that do not parse or cannot be read, so their Refs to the moved
	// node file or its children directory could not be checked and were left as they are, and the
	// Knowledge directory itself when it could not be walked. A Note file unread already lists is
	// left out.
	unreadNotes []string
	// refsRewritten are the Note files whose Refs the move rewrites, in walk order.
	refsRewritten []string
}

// planNodeMove works out the move of node, a node of nodes, to the destination to, reading the
// node files but changing nothing. The node's children directory, with its whole subtree, moves
// along with it. Every rewrite keeps the bytes it read as its Original, so the move's Apply refuses
// a file changed since, and a file both a rename and a changed path rewrite is rewritten once with
// both changes.
//
// A rename adds the old id to the node's Aliases (or, when the new id is one of its own Aliases,
// swaps the two) and, in every node file of the tree, the node's own among them, replaces each
// entry of the kind's reference fields that names the old id or one of its Aliases by the new id,
// keeping only the first entry equal to it; a file none of whose entries names them keeps its
// entries as they are. On the node's own self-refused fields, an entry naming the new id is
// dropped. Body text is never rewritten.
//
// A move that changes the node file's path, a rename or a re-parent, rewrites, in every Note file
// of the store's Knowledge tree, the moved Note's own and those inside its subtree among them, each
// Ref naming the node file, its children directory or a path below it to its path after the move,
// as knowledge.RewriteRefs does; a Note none of whose Refs names them is left as it is.
//
// It checks the destination before it reads any file, failing as tree.PlanMove does, then fails
// with the error of the node's own file when nodefile.ReadRegularFile refuses it or it does not
// parse, and, on a rename, with the error of another node file it cannot read; another node file
// that does not parse, or that nodefile.ReadRegularFile refuses unread, joins unread instead. A
// Note whose Refs cannot be checked never fails the move: it joins unreadNotes.
func (k *Kind[T]) planNodeMove(nodes *tree.Tree, node tree.Node, to tree.Destination) (*nodeMove, error) {
	planned, err := nodes.PlanMove(node.ID, to, k.codec.nounPhrase)
	if err != nil {
		return nil, err
	}
	data, err := nodefile.ReadRegularFile(node.Path)
	if err != nil {
		return nil, err
	}
	own, err := k.codec.Parse(node.Path, data)
	if err != nil {
		return nil, err
	}

	move := &nodeMove{Move: planned, original: data, content: data}
	rewrites := &rewriteSet{}
	if planned.NewID != node.ID {
		if err := k.planRename(nodes, own, move, rewrites); err != nil {
			return nil, err
		}
	}
	if planned.NewPath != node.Path {
		k.planNoteRefs(move, rewrites)
	}
	for _, rewrite := range rewrites.rewrites {
		planned.AddRewrite(rewrite)
	}
	return move, nil
}

// planRename adds to rewrites the rewrites of a rename of move's node, own its parsed file: the
// node file with its Aliases and reference fields renamed, setting move.content, and every other
// node file of nodes whose entries name the node. A node file that cannot be checked joins
// move.unread; one that cannot be read otherwise fails the rename.
func (k *Kind[T]) planRename(nodes *tree.Tree, own T, move *nodeMove, rewrites *rewriteSet) error {
	node := move.Node
	names := slices.Concat([]string{node.ID}, node.Aliases)
	own = k.codec.setAliases(own, tree.RenameAliases(k.codec.Aliases(own), node.ID, move.NewID))
	own = k.renameOwnRefs(own, names, move.NewID)
	move.content = k.codec.Serialize(own)
	rewrites.put(tree.Rewrite{Path: node.Path, Original: move.original, Content: move.content})

	for _, other := range nodes.Nodes() {
		if other.ID == node.ID {
			continue
		}
		rewrite, isUnread, err := k.planRefRewrite(other.Path, names, move.NewID)
		if err != nil {
			return err
		}
		if isUnread {
			move.unread = append(move.unread, other.Path)
		}
		if rewrite != nil {
			rewrites.put(*rewrite)
		}
	}
	return nil
}

// planNoteRefs adds to rewrites the rewrite of every Note file of the store's Knowledge tree whose
// Refs name move's node file, its children directory or a path below it, recording it in
// move.refsRewritten. A Note already among rewrites is rewritten from the content planned for it,
// the moved Note's own file from move.content, which follows the rewrite. A Note that does not
// parse or cannot be read, and the Knowledge directory when it cannot be walked, join
// move.unreadNotes; a Note move.unread already lists is skipped.
func (k *Kind[T]) planNoteRefs(move *nodeMove, rewrites *rewriteSet) {
	oldFile, newFile := k.relative(move.Node.Path), k.relative(move.NewPath)
	oldDir, newDir := tree.ChildrenDir(oldFile), tree.ChildrenDir(newFile)
	// The walk's problems are files it leaves out of the tree, no Notes; the index warns about them.
	notes, _, err := tree.Walk(k.knowledgeDir)
	if err != nil {
		move.unreadNotes = append(move.unreadNotes, k.knowledgeDir)
		return
	}
	for _, note := range notes {
		if slices.Contains(move.unread, note.Path) {
			continue
		}
		original, current, err := move.noteContent(note.Path, rewrites)
		if err != nil {
			move.unreadNotes = append(move.unreadNotes, note.Path)
			continue
		}
		parsed, err := knowledge.Parse(note.Path, current)
		if err != nil {
			move.unreadNotes = append(move.unreadNotes, note.Path)
			continue
		}
		refs, isRewritten := knowledge.RewriteRefs(parsed.Refs, oldFile, newFile, oldDir, newDir)
		if !isRewritten {
			continue
		}
		parsed.Refs = refs
		content := parsed.Serialize()
		rewrites.put(tree.Rewrite{Path: note.Path, Original: original, Content: content})
		if note.Path == move.Node.Path {
			move.content = content
		}
		move.refsRewritten = append(move.refsRewritten, note.Path)
	}
}

// noteContent returns the Note file at path as read from disk and as the move has planned it so
// far: from rewrites when they hold it, from the move itself for the moved node's own file, and
// read from disk otherwise, failing with the read's error.
func (m *nodeMove) noteContent(path string, rewrites *rewriteSet) ([]byte, []byte, error) {
	if planned, isPlanned := rewrites.find(path); isPlanned {
		return planned.Original, planned.Content, nil
	}
	if path == m.Node.Path {
		return m.original, m.content, nil
	}
	data, err := nodefile.ReadRegularFile(path)
	if err != nil {
		return nil, nil, err
	}
	return data, data, nil
}

// rewriteSet collects the rewrites of one move, one per file in the order each file was first put,
// so a file two passes change is written once with both changes.
type rewriteSet struct {
	rewrites []tree.Rewrite
}

// find returns the rewrite of the file at path, and whether the set holds one.
func (s *rewriteSet) find(path string) (tree.Rewrite, bool) {
	index := slices.IndexFunc(s.rewrites, func(rewrite tree.Rewrite) bool { return rewrite.Path == path })
	if index < 0 {
		return tree.Rewrite{}, false
	}
	return s.rewrites[index], true
}

// put adds rewrite to the set, or, when the set already holds a rewrite of its file, replaces that
// rewrite's Content with rewrite's, keeping the Original it was first read with.
func (s *rewriteSet) put(rewrite tree.Rewrite) {
	index := slices.IndexFunc(s.rewrites, func(held tree.Rewrite) bool { return held.Path == rewrite.Path })
	if index < 0 {
		s.rewrites = append(s.rewrites, rewrite)
		return
	}
	s.rewrites[index].Content = rewrite.Content
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
// is one of names. A file that does not parse, or that nodefile.ReadRegularFile refuses unread, is
// reported unread instead; any other read failure is returned.
func (k *Kind[T]) planRefRewrite(path string, names []string, newID string) (*tree.Rewrite, bool, error) {
	data, err := nodefile.ReadRegularFile(path)
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

// nodeRemoval is a removal of one node worked out by planNodeRemoval: the tree removal, with the
// rewrites of the files whose entries it drops added.
type nodeRemoval struct {
	*tree.Removal
	// dropped maps the key of each reference field to the ids of the nodes whose entries of it
	// naming the removed node are dropped, in tree order; a field nothing is dropped from is absent.
	dropped map[string][]string
	// rewritten are the files of the nodes whose entries are dropped, in tree order.
	rewritten []string
	// unread are the files of other nodes that do not parse, or that nodefile.ReadRegularFile
	// refuses unread, so their entries could not be checked for the removed node and were left as
	// they are.
	unread []string
}

// planNodeRemoval works out the removal of node, a node of nodes, reading the node files but
// changing nothing. Every other node's entries of the kind's reference fields that name the node,
// by its id or one of its Aliases, are dropped; every other node file of the tree is read, those
// tree.Load left out for a repeated id included. The node's own file is read once, for its etag
// and, when the node has no Aliases, for those tree.ReadAliases reads from it, recovered from the
// raw frontmatter when it does not parse, so a node whose file does not parse is removed like any
// other.
//
// It fails, in this order, as tree.PlanRemove does, with the *fs.PathError of the node's own file
// when it cannot be read, with a *ConflictError when ifMatch is set and is not the node file's
// etag, with the error of another node file it cannot read, and, unless shouldDropDeps, with a
// *DependentsError naming the holders of a blocksRemoval field's entries, in the order of their
// files' paths. Another node file that does not parse, or that nodefile.ReadRegularFile refuses
// unread, joins unread instead. A node's entry naming itself is no dependent.
func (k *Kind[T]) planNodeRemoval(nodes *tree.Tree, node tree.Node, ifMatch string, shouldDropDeps bool) (*nodeRemoval, error) {
	planned, err := nodes.PlanRemove(node.ID, k.codec.nounPhrase)
	if err != nil {
		return nil, err
	}
	data, err := nodefile.ReadRegularFile(node.Path)
	if err != nil {
		return nil, err
	}
	if current := nodefile.ETag(data); ifMatch != "" && ifMatch != current {
		return nil, k.conflictError(node.Path, ifMatch, current)
	}

	aliases := node.Aliases
	if len(aliases) == 0 {
		aliases, _ = tree.ReadAliases(data, k.aliasParser(node.Path))
	}
	names := slices.Concat([]string{node.ID}, aliases)
	removal := &nodeRemoval{Removal: planned, dropped: map[string][]string{}}
	var rewrites []tree.Rewrite
	var dependents []tree.Node
	for _, other := range slices.Concat(nodes.Nodes(), nodes.LeftOut()) {
		if other.ID == node.ID {
			continue
		}
		rewrite, droppedFrom, isUnread, err := k.planRefDrop(other.Path, names)
		if err != nil {
			return nil, err
		}
		if isUnread {
			removal.unread = append(removal.unread, other.Path)
		}
		if rewrite == nil {
			continue
		}
		for _, field := range droppedFrom {
			removal.dropped[field.key] = append(removal.dropped[field.key], other.ID)
		}
		if slices.ContainsFunc(droppedFrom, func(field refField[T]) bool { return field.blocksRemoval }) {
			dependents = append(dependents, other)
		}
		rewrites = append(rewrites, *rewrite)
		removal.rewritten = append(removal.rewritten, other.Path)
	}

	if len(dependents) > 0 && !shouldDropDeps {
		return nil, k.dependentsError(node, dependents)
	}
	for _, rewrite := range rewrites {
		planned.AddRewrite(rewrite)
	}
	return removal, nil
}

// planRefDrop reads the node file at path, another node's than the removed one, and returns its
// rewrite without the entries of its reference fields that are one of names, with the fields
// those entries were dropped from, or nil when none of its entries is one of names. A file that
// does not parse, or that nodefile.ReadRegularFile refuses unread, is reported unread instead; any
// other read failure is returned.
func (k *Kind[T]) planRefDrop(path string, names []string) (*tree.Rewrite, []refField[T], bool, error) {
	data, err := nodefile.ReadRegularFile(path)
	if isRefusedRead(err) {
		return nil, nil, true, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	node, err := k.codec.Parse(path, data)
	if err != nil {
		return nil, nil, true, nil
	}

	var droppedFrom []refField[T]
	for _, field := range k.codec.refs {
		entries := field.get(node)
		kept := dropRefs(entries, names)
		if len(kept) != len(entries) {
			node = field.set(node, kept)
			droppedFrom = append(droppedFrom, field)
		}
	}
	if len(droppedFrom) == 0 {
		return nil, nil, false, nil
	}
	return &tree.Rewrite{Path: path, Original: data, Content: k.codec.Serialize(node)}, droppedFrom, false, nil
}

// dropRefs returns a copy of entries, the entries of one reference field, without those that are
// one of names, or nil when none is left.
func dropRefs(entries []string, names []string) []string {
	kept := slices.DeleteFunc(slices.Clone(entries), func(entry string) bool { return slices.Contains(names, entry) })
	if len(kept) == 0 {
		return nil
	}
	return kept
}

// dependentsError refuses the removal of node, which the nodes dependents depend on, as a
// *DependentsError at its file naming them in the order of their files' paths.
func (k *Kind[T]) dependentsError(node tree.Node, dependents []tree.Node) *DependentsError {
	slices.SortStableFunc(dependents, func(first tree.Node, second tree.Node) int {
		return strings.Compare(filepath.ToSlash(first.Path), filepath.ToSlash(second.Path))
	})
	ids := make([]string, 0, len(dependents))
	for _, dependent := range dependents {
		ids = append(ids, dependent.ID)
	}
	return &DependentsError{
		Path:       k.relative(node.Path),
		ID:         node.ID,
		Dependents: ids,
		Message:    fmt.Sprintf("%s %s is a dependency of %s", k.codec.Noun, node.ID, strings.Join(ids, ", ")),
	}
}
