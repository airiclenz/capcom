package tree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/airiclenz/capcom/internal/nodefile"
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

// Content is what a ContentFunc reads of one node file.
type Content struct {
	// Aliases are the Aliases the file records or, for a file that does not parse, the ones a
	// recovering read still finds.
	Aliases []string
	// Parse is ParsedFile for a file that parses, UnparsedFile for one that does not, and
	// UnreadFile for one the reader refused unread.
	Parse ParseState
	// IsUsable reports whether the file passes its kind's validation; false for one that does not
	// parse or was not read.
	IsUsable bool
}

// ContentFunc reads the node file at path for LoadContent. Its error is returned from LoadContent
// unchanged, so it should name the file itself.
type ContentFunc func(path string) (Content, error)

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
	dir string
	// fsys is the FS Load read the tree through; Claimant lists the orphan directories through it.
	fsys     nodefile.FS
	nodes    []Node
	byID     map[string]int
	byAlias  map[string]int
	children map[string][]int
	// leftOut are the node files Load left out for a repeated id, ordered by id, then path.
	leftOut []Node
	// refusals maps every name the membership verdicts refuse to its verdict: a name two or more
	// files claim, each id and alias a node file left out below a repeated id claims, and each alias
	// the first file of a repeated id lists, as duplicates; and every name only an unusable file
	// claims, one that does not parse, fails its kind's validation or has an alias CheckAliases
	// rejects, a file left out for a repeated id, or a node file the walk left out.
	refusals map[string]NameVerdict
	// unparsedIDs holds the id of every node whose file does not parse.
	unparsedIDs map[string]bool
	// unusableIDs holds the id of every node whose file is not usable: it does not parse, fails its
	// kind's validation or has an alias CheckAliases rejects.
	unusableIDs map[string]bool
	// aliases is the AliasFunc Load read the Aliases with; nil when the kind has none.
	aliases AliasFunc
	// repeatedFiles index the files of leftOut that carry a repeated id, in leftOut's order.
	repeatedFiles []int
	// orphanDirs are the Paths of the ErrOrphanDirectory problems Load reported that lie under no
	// other such directory, ordered by path.
	orphanDirs []string
}

// Load reads the tree stored in dir. A missing dir loads as an empty tree. Names starting with a
// dot and files not ending in .md are skipped. noun names one node of the kind in a message,
// "issue" or "Note". aliases may be nil when the kind has none.
//
// Load reports, as an *Error naming the file, an invalid id, an alias CheckAliases rejects (its
// message naming the node by noun), an id or alias two files share (naming the other file in Other), and
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
// refuses. It is also called for the node files below the orphan directories Walk reports, whose
// names Resolve refuses as unusable claims; an error there leaves the file claiming its id alone,
// as a node file the walk left out unread claims it. Resolve refuses as unusable claims, too, every
// name of a file with an alias CheckAliases rejects, and an alias only a file left out for a
// repeated id lists.
func Load(dir string, noun string, aliases AliasFunc) (*Tree, error) {
	return load(diskFS{}, dir, noun, aliasContent(aliases), aliases, 1)
}

// LoadParallel is Load calling aliases for the node files the walk finds on at most
// runtime.NumCPU() goroutines at once, so aliases must be safe to call concurrently. Every such
// file is read before any is placed, and the tree is built from the reads in walk order, so the
// tree, its order and every problem and error it returns are Load's. The node files below an
// orphan directory, and Claimant later, call aliases one at a time.
func LoadParallel(dir string, noun string, aliases AliasFunc) (*Tree, error) {
	return load(diskFS{}, dir, noun, aliasContent(aliases), aliases, readWorkers())
}

// aliasContent returns the ContentFunc of aliases: every file parses and is usable, with the
// Aliases aliases reads. A nil aliases gives a nil ContentFunc, which reads no file.
func aliasContent(aliases AliasFunc) ContentFunc {
	if aliases == nil {
		return nil
	}
	return func(path string) (Content, error) {
		names, err := aliases(path)
		return Content{Aliases: names, Parse: ParsedFile, IsUsable: true}, err
	}
}

// readWorkers is the most goroutines LoadParallel reads node files on at once.
func readWorkers() int {
	return runtime.NumCPU()
}

// LoadContent reads the tree stored in dir as Load does, reading every node file with content
// instead of an AliasFunc: a file that does not parse is a node whose Aliases are nil, and a file
// that fails its kind's validation is a node as any other. Resolve refuses every name such a file
// claims, its recovered aliases included, as an unusable claim, except the exact id of a node whose
// file does not parse. Nodes, ChildrenOf and LeftOut are as Load gives them.
func LoadContent(dir string, noun string, content ContentFunc) (*Tree, error) {
	return LoadContentFS(diskFS{}, dir, noun, content)
}

// LoadContentFS is LoadContent reading the tree through fsys: every directory listing, Stat and
// Lstat of the load goes through it, and so does Claimant's listing of the orphan directories
// later. content reads the node files itself, so it should read through fsys too.
func LoadContentFS(fsys nodefile.FS, dir string, noun string, content ContentFunc) (*Tree, error) {
	var aliases AliasFunc
	if content != nil {
		aliases = parsedAliases(content)
	}
	return load(fsys, dir, noun, content, aliases, 1)
}

// parsedAliases returns the AliasFunc of content: the Aliases of a file that parses, none for any
// other.
func parsedAliases(content ContentFunc) AliasFunc {
	return func(path string) ([]string, error) {
		read, err := content(path)
		if err != nil || read.Parse != ParsedFile {
			return nil, err
		}
		return read.Aliases, nil
	}
}

// load is Load, LoadParallel and LoadContentFS, reading the tree through fsys: noun names one node
// of the kind in a message, content reads every node file, nil reading none, and aliases is the
// AliasFunc the tree keeps for Claimant. workers is the most goroutines content reads the walked
// node files on at once; 1 reads them one at a time, in walk order.
func load(
	fsys nodefile.FS,
	dir string,
	noun string,
	content ContentFunc,
	aliases AliasFunc,
	workers int,
) (*Tree, error) {
	nodes, problems, err := walkTree(fsys, dir)
	if err != nil {
		return nil, err
	}
	facts, rejected, failures := readContentFacts(nodes, noun, preread(nodes, content, workers))
	facts = append(facts, claimOnlyFacts(fsys, problems, noun, content)...)
	membership := DecideMembership(facts)
	dirProblems, err := repeatedIDDirProblems(fsys, membership.RepeatedIDDirs)
	if err != nil {
		return nil, err
	}
	if err := firstAliasFailure(membership, failures); err != nil {
		return nil, err
	}

	tree := newTree(dir, aliases, nodes, facts, membership)
	tree.fsys = fsys
	// Appended in this order, a file's repeated id comes before its rejected aliases, and those
	// before its alias conflicts, once the problems are sorted by path.
	problems = append(problems, conflictErrors(membership.IDConflicts)...)
	problems = append(problems, dirProblems...)
	problems = append(problems, rejected...)
	problems = append(problems, conflictErrors(membership.AliasConflicts)...)
	if len(problems) > 0 {
		sort.SliceStable(problems, func(i, j int) bool { return problems[i].Path < problems[j].Path })
		tree.orphanDirs = outermostOrphanDirs(problems)
		return tree, problems
	}
	return tree, nil
}

// contentRead is what a ContentFunc returned for one node file.
type contentRead struct {
	content Content
	err     error
}

// preread reads every file of nodes with content on at most workers goroutines at once, and returns
// a ContentFunc that answers each of their paths with that read and reads any other path with
// content. readContentFacts, reading through it in walk order, sees what reading the files one at a
// time gives. With fewer than two workers or two nodes, or a nil content, it returns content as is.
func preread(nodes []Node, content ContentFunc, workers int) ContentFunc {
	workers = min(workers, len(nodes))
	if content == nil || workers < 2 {
		return content
	}
	reads := make([]contentRead, len(nodes))
	indexes := make(chan int)
	var readers sync.WaitGroup
	for range workers {
		readers.Go(func() {
			// Each reader writes only the slots of the indexes it receives, so no two share one.
			for index := range indexes {
				read, err := content(nodes[index].Path)
				reads[index] = contentRead{content: read, err: err}
			}
		})
	}
	for index := range nodes {
		indexes <- index
	}
	close(indexes)
	readers.Wait()

	byPath := make(map[string]int, len(nodes))
	for index, node := range nodes {
		byPath[node.Path] = index
	}
	return func(path string) (Content, error) {
		if index, isRead := byPath[path]; isRead {
			return reads[index].content, reads[index].err
		}
		return content(path)
	}
}

// newTree builds the tree of dir from membership, DecideMembership's verdict on facts, which hold
// the facts of nodes first, in nodes' order.
func newTree(dir string, aliases AliasFunc, nodes []Node, facts []FileFact, membership Membership) *Tree {
	tree := &Tree{
		dir:         dir,
		aliases:     aliases,
		nodes:       make([]Node, 0, len(membership.Kept)),
		byID:        map[string]int{},
		byAlias:     map[string]int{},
		children:    map[string][]int{},
		refusals:    map[string]NameVerdict{},
		unparsedIDs: map[string]bool{},
		unusableIDs: map[string]bool{},
	}
	keptAt := map[int]int{}
	for _, i := range membership.Kept {
		node := nodes[i]
		if facts[i].Parse == UnparsedFile {
			tree.unparsedIDs[node.ID] = true
		}
		if !facts[i].IsUsable {
			tree.unusableIDs[node.ID] = true
		}
		keptAt[i] = len(tree.nodes)
		tree.byID[node.ID] = len(tree.nodes)
		tree.children[node.ParentID] = append(tree.children[node.ParentID], len(tree.nodes))
		tree.nodes = append(tree.nodes, node)
	}
	for alias, i := range membership.Aliases {
		tree.byAlias[alias] = keptAt[i]
	}
	for _, file := range membership.LeftOut {
		if file.Reason == RepeatedIDFile {
			tree.repeatedFiles = append(tree.repeatedFiles, len(tree.leftOut))
		}
		tree.leftOut = append(tree.leftOut, nodes[file.Fact])
	}
	for name, verdict := range membership.Names {
		if verdict.Refusal != 0 {
			tree.refusals[name] = verdict
		}
	}
	return tree
}

// The reasons a node file is left out, as the problem of the directory beside it words them.
const (
	invalidIDReason  = "has an invalid id"
	notRegularReason = "is not a regular file"
	tooLargeReason   = "is larger than 1 MiB"
	duplicateReason  = "has a duplicate id"
)

// Nodes returns every node, ordered by id.
func (t *Tree) Nodes() []Node {
	return append([]Node(nil), t.nodes...)
}

// LeftOut returns the node files Load left out for a repeated id, ordered by id, then path: every
// file of the id, and every node file under a directory beside one of them. Their Aliases are as
// the AliasFunc returned them. They are not nodes of the tree: Nodes, Lookup, ChildrenOf and
// PathFor never see them, and Resolve refuses their names: as duplicates, and an alias only a later
// file of a repeated id lists as an unusable claim.
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
// ErrNotFound when parentID is not a node of the tree. A parent whose children directory exists
// as anything but a real directory, a symlink or a file, fails with a *Error naming that path and
// wrapping ErrNotDirectory, so nothing is written through it; a missing one is fine.
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
	childrenDir := ChildrenDir(t.nodes[parent].Path)
	if err := checkRealDir(childrenDir, parentID); err != nil {
		return "", err
	}
	return filepath.Join(childrenDir, id+nodeExtension), nil
}

// checkRealDir refuses dir, the children directory of the node parentID, when it exists as
// anything but a real directory. Lstat does not follow a symlink, so one that points at a
// directory is refused too. A dir that does not exist passes.
func checkRealDir(dir string, parentID string) error {
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return &Error{
			Path: dir,
			Err:  fmt.Errorf("children directory of %s is %w", parentID, ErrNotDirectory),
		}
	}
	return nil
}

// IDOf returns the id the node file at nodePath has by its name: the base name without the .md
// extension. nodePath may use the OS separator or forward slashes, as git and the index do.
func IDOf(nodePath string) string {
	return strings.TrimSuffix(path.Base(filepath.ToSlash(nodePath)), nodeExtension)
}

// ChildrenDir returns the directory that holds the children of the node file at nodePath: the
// path without the .md extension, in nodePath's own separators.
func ChildrenDir(nodePath string) string {
	return strings.TrimSuffix(nodePath, nodeExtension)
}
