package tree

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// nodeExtension is the file extension of every node file.
const nodeExtension = ".md"

// hiddenPrefix marks file and directory names Load skips, including nodefile.WriteFileAtomic's
// temp files.
const hiddenPrefix = "."

// rootParentID is the ParentID of a top-level node.
const rootParentID = ""

// AliasFunc returns the Aliases recorded in the node file at path. Its error is returned from
// Load unchanged, so it should name the file itself.
type AliasFunc func(path string) ([]string, error)

// Node is one node of a Tree. Its fields are read-only.
type Node struct {
	// ID is the node's id: its file name without the .md extension.
	ID string
	// Path is the node file, joined onto the directory passed to Load.
	Path string
	// ParentID is the id of the node directly above it, or "" for a top-level node.
	ParentID string
	// Aliases are the node's former ids, as the AliasFunc returned them.
	Aliases []string
}

// Tree is one loaded store directory. It is a snapshot: later changes on disk are not seen.
type Tree struct {
	dir      string
	nodes    []Node
	byID     map[string]int
	byAlias  map[string]int
	children map[string][]int
	// leftOut are the node files Load left out for a repeated id, ordered by id, then path.
	leftOut []Node
	// duplicates maps every id or alias two or more files claim to the first problem Load reported
	// for it, which names two of those files. It also maps each id and alias a node file left out
	// below a repeated id claims, and each alias the first file of a repeated id lists, to the
	// problem of that repeated id.
	duplicates map[string]*Error
}

// Load reads the tree stored in dir. A missing dir loads as an empty tree. Names starting with a
// dot and files not ending in .md are skipped. aliases may be nil when the kind has none.
//
// Load reports, as an *Error naming the file, an invalid id, an alias CheckAliases rejects (its
// message naming the "node"), an id or alias two files share (naming the other file in Other), and
// every entry Walk leaves out (an entry that is not a regular file, a node file over 1 MiB, a child
// directory without its node file), as Walk words it. It finds every such problem in one pass and
// returns them all as Errors, together with the tree of what it could place: a file with an invalid
// id, a directory without its node file (with its whole subtree), every file of a repeated id and
// an invalid or repeated alias are left out.
//
// A repeated id is one problem per file after the first, naming the first in Other. Every node file
// under a directory beside a file of a repeated id is left out too, and each such directory is an
// *Error wrapping ErrOrphanDirectory, its message naming the node file and its duplicate id. LeftOut
// returns the node files left out for a repeated id. A directory that cannot be read or Lstat and
// an AliasFunc error stop the load and return no tree; the AliasFunc error comes back unchanged. A
// child directory gone since its parent was read walks as empty.
//
// aliases is called for every node file, one left out for a repeated id or below one included. Such
// a file's aliases are checked as any other file's and never mapped; one that a kept file also
// claims as its id or an alias, or that an earlier such file lists, is a duplicate, which Resolve
// refuses.
func Load(dir string, aliases AliasFunc) (*Tree, error) {
	tree := &Tree{
		dir:        dir,
		byID:       map[string]int{},
		byAlias:    map[string]int{},
		children:   map[string][]int{},
		duplicates: map[string]*Error{},
	}
	nodes, problems, err := Walk(dir)
	if err != nil {
		return nil, err
	}
	tree.nodes = nodes
	sort.Slice(tree.nodes, func(i, j int) bool {
		if tree.nodes[i].ID != tree.nodes[j].ID {
			return tree.nodes[i].ID < tree.nodes[j].ID
		}
		return tree.nodes[i].Path < tree.nodes[j].Path
	})
	dropped, err := tree.indexIDs(&problems)
	if err != nil {
		return nil, err
	}
	if err := tree.indexAliases(aliases, &problems); err != nil {
		return nil, err
	}
	if err := tree.checkDroppedAliases(aliases, dropped, &problems); err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		sort.SliceStable(problems, func(i, j int) bool { return problems[i].Path < problems[j].Path })
		return tree, problems
	}
	return tree, nil
}

// The reasons a node file is left out, as the problem of the directory beside it words them.
const (
	invalidIDReason  = "has an invalid id"
	notRegularReason = "is not a regular file"
	tooLargeReason   = "is larger than 1 MiB"
	duplicateReason  = "has a duplicate id"
)

// Dir returns the directory the tree was loaded from.
func (t *Tree) Dir() string {
	return t.dir
}

// Nodes returns every node, ordered by id.
func (t *Tree) Nodes() []Node {
	return append([]Node(nil), t.nodes...)
}

// LeftOut returns the node files Load left out for a repeated id, ordered by id, then path: every
// file of the id, and every node file under a directory beside one of them. Their Aliases are as
// the AliasFunc returned them. They are not nodes of the tree: Nodes, Lookup, ChildrenOf and
// PathFor never see them, and Resolve refuses their names as duplicates.
func (t *Tree) LeftOut() []Node {
	return append([]Node(nil), t.leftOut...)
}

// ChildrenOf returns the nodes directly below the node id, ordered by id; id "" returns the
// top-level nodes. An unknown id has no children.
func (t *Tree) ChildrenOf(id string) []Node {
	indexes := t.children[id]
	children := make([]Node, 0, len(indexes))
	for _, i := range indexes {
		children = append(children, t.nodes[i])
	}
	return children
}

// PathFor returns the file a node named id stores at when placed below parentID ("" for the top
// level), whether or not it exists yet. It fails with ErrInvalidID for an invalid id and with
// ErrNotFound when parentID is not a node of the tree.
func (t *Tree) PathFor(id string, parentID string) (string, error) {
	if err := ValidateID(id); err != nil {
		return "", err
	}
	if parentID == rootParentID {
		return filepath.Join(t.dir, id+nodeExtension), nil
	}
	parent, found := t.byID[parentID]
	if !found {
		return "", fmt.Errorf("parent id %q %w", parentID, ErrNotFound)
	}
	parentDir := strings.TrimSuffix(t.nodes[parent].Path, nodeExtension)
	return filepath.Join(parentDir, id+nodeExtension), nil
}
