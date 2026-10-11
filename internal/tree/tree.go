// Package tree loads one store directory (`.capcom/issues/`, `.capcom/knowledge/`) as a Tree of
// nodes and writes node files back atomically. The directory tree is the hierarchy (ADR 0001): a
// node is the markdown file `<id>.md`, and its children live in the sibling directory `<id>/`.
// Ids are chosen kebab slugs, unique across the whole tree together with every node's Aliases
// (ADR 0002). The package is generic over the node kind; reading a node's Aliases out of its file
// is left to the caller through an AliasFunc.
package tree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// nodeExtension is the file extension of every node file.
const nodeExtension = ".md"

// maxIDLength is the longest id allowed, in bytes (ids are ASCII).
const maxIDLength = 40

// hiddenPrefix marks file and directory names Load skips, including WriteFileAtomic's temp files.
const hiddenPrefix = "."

// rootParentID is the ParentID of a top-level node.
const rootParentID = ""

var (
	idPattern    = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+){1,3}$`)
	aliasPattern = regexp.MustCompile(`^[a-z0-9.-]+$`)
)

// Sentinel errors, reachable through errors.Is on every error this package returns.
var (
	// ErrInvalidID reports an id that is not a 2-4 word kebab slug of at most 40 characters.
	ErrInvalidID = errors.New("invalid id")
	// ErrInvalidAlias reports an alias outside [a-z0-9.-]+.
	ErrInvalidAlias = errors.New("invalid alias")
	// ErrDuplicate reports an id or alias used twice anywhere in one tree.
	ErrDuplicate = errors.New("duplicate")
	// ErrOrphanDirectory reports a child directory without the node file of the same name.
	ErrOrphanDirectory = errors.New("directory without its node file")
	// ErrNotFound reports an argument that matches no id, alias or id prefix.
	ErrNotFound = errors.New("not found")
	// ErrAmbiguous reports an id prefix that matches more than one node; see AmbiguousError.
	ErrAmbiguous = errors.New("ambiguous id prefix")
)

// Error is a problem with one file or directory of a tree. Err wraps one of the sentinel errors.
// Other is the file a duplicate id or alias also belongs to, kept out of Err so a caller can name
// it relative to a root of its own; it is "" for every other problem. Alias is the alias an invalid
// or duplicate alias problem is about, and "" for a problem with an id or a directory.
type Error struct {
	Path  string
	Other string
	Alias string
	Err   error
}

// Error renders "path: message".
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Path, e.Message())
}

// Message renders the problem without its path: Err, followed by Other when there is one.
func (e *Error) Message() string {
	if e.Other == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%v %s", e.Err, e.Other)
}

// Unwrap returns the underlying error, so errors.Is matches the sentinels.
func (e *Error) Unwrap() error {
	return e.Err
}

// Errors is every problem Load found in one tree, ordered by path. errors.Is and errors.As reach
// each problem, and through it the sentinel it wraps.
type Errors []*Error

// Error renders one "path: message" line per problem.
func (e Errors) Error() string {
	lines := make([]string, 0, len(e))
	for _, problem := range e {
		lines = append(lines, problem.Error())
	}
	return strings.Join(lines, "\n")
}

// Unwrap returns the problems, so errors.Is and errors.As look into each.
func (e Errors) Unwrap() []error {
	unwrapped := make([]error, 0, len(e))
	for _, problem := range e {
		unwrapped = append(unwrapped, problem)
	}
	return unwrapped
}

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
}

// ValidateID checks that id is a kebab slug of two to four lowercase words or numbers, at most 40
// characters long. The error wraps ErrInvalidID.
func ValidateID(id string) error {
	if len(id) > maxIDLength {
		return fmt.Errorf("%w %q: longer than %d characters", ErrInvalidID, id, maxIDLength)
	}
	if !idPattern.MatchString(id) {
		return fmt.Errorf(
			"%w %q: must be 2 to 4 words of a-z and 0-9 joined by single hyphens",
			ErrInvalidID,
			id,
		)
	}
	return nil
}

// ValidateAlias checks that alias is non-empty and uses only a-z, 0-9, dots and hyphens, the
// wider set that legacy ids (`apogee-kk0.4`) need. The error wraps ErrInvalidAlias.
func ValidateAlias(alias string) error {
	if !aliasPattern.MatchString(alias) {
		return fmt.Errorf("%w %q: must use only a-z, 0-9, dots and hyphens", ErrInvalidAlias, alias)
	}
	return nil
}

// Load reads the tree stored in dir. A missing dir loads as an empty tree. Names starting with a
// dot and files not ending in .md are skipped. aliases may be nil when the kind has none.
//
// Load reports, as an *Error naming the file, an invalid id or alias, an id or alias used twice
// anywhere in the tree (naming the other file in Other), and a child directory without its node file. It finds
// every such problem in one pass and returns them all as Errors, together with the tree of what it
// could place: a file with an invalid id, a directory without its node file (with its whole
// subtree), a repeated id after its first file and an invalid or repeated alias are left out. A
// directory that cannot be read and an AliasFunc error stop the load and return no tree; the
// AliasFunc error comes back unchanged.
func Load(dir string, aliases AliasFunc) (*Tree, error) {
	tree := &Tree{
		dir:      dir,
		byID:     map[string]int{},
		byAlias:  map[string]int{},
		children: map[string][]int{},
	}
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return tree, nil
	}

	var problems Errors
	if err := tree.walk(dir, rootParentID, &problems); err != nil {
		return nil, err
	}
	sort.Slice(tree.nodes, func(i, j int) bool {
		if tree.nodes[i].ID != tree.nodes[j].ID {
			return tree.nodes[i].ID < tree.nodes[j].ID
		}
		return tree.nodes[i].Path < tree.nodes[j].Path
	})
	tree.indexIDs(&problems)
	if err := tree.indexAliases(aliases, &problems); err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		sort.SliceStable(problems, func(i, j int) bool { return problems[i].Path < problems[j].Path })
		return tree, problems
	}
	return tree, nil
}

// walk collects the nodes of dir, all children of parentID, and recurses into the child
// directories that have their node file, adding every invalid id and orphan directory to problems.
func (t *Tree) walk(dir string, parentID string, problems *Errors) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	nodeIDs := map[string]bool{}
	var childDirs []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, hiddenPrefix) {
			continue
		}
		if entry.IsDir() {
			childDirs = append(childDirs, name)
			continue
		}
		id, isNode := strings.CutSuffix(name, nodeExtension)
		if !isNode {
			continue
		}
		path := filepath.Join(dir, name)
		if err := ValidateID(id); err != nil {
			*problems = append(*problems, &Error{Path: path, Err: err})
			continue
		}
		nodeIDs[id] = true
		t.nodes = append(t.nodes, Node{ID: id, Path: path, ParentID: parentID})
	}

	for _, name := range childDirs {
		path := filepath.Join(dir, name)
		if !nodeIDs[name] {
			*problems = append(*problems, &Error{
				Path: path,
				Err:  fmt.Errorf("%w: no %s%s beside it", ErrOrphanDirectory, name, nodeExtension),
			})
			continue
		}
		if err := t.walk(path, name, problems); err != nil {
			return err
		}
	}
	return nil
}

// indexIDs maps every id to its node and every parent to its children. A repeated id is added to
// problems and its node dropped, so the first file of the id keeps it.
func (t *Tree) indexIDs(problems *Errors) {
	kept := t.nodes[:0]
	for _, node := range t.nodes {
		if first, taken := t.byID[node.ID]; taken {
			*problems = append(*problems, &Error{
				Path:  node.Path,
				Other: kept[first].Path,
				Err:   fmt.Errorf("%w id %q, also at", ErrDuplicate, node.ID),
			})
			continue
		}
		t.byID[node.ID] = len(kept)
		t.children[node.ParentID] = append(t.children[node.ParentID], len(kept))
		kept = append(kept, node)
	}
	t.nodes = kept
}

// indexAliases reads every node's Aliases and maps each to its node. An invalid alias, or one that
// repeats an id or another alias, is added to problems and not mapped. It fails only when aliases
// does.
func (t *Tree) indexAliases(aliases AliasFunc, problems *Errors) error {
	if aliases == nil {
		return nil
	}
	for i := range t.nodes {
		node := &t.nodes[i]
		names, err := aliases(node.Path)
		if err != nil {
			return err
		}
		node.Aliases = names
		for _, alias := range names {
			if err := t.indexAlias(i, alias); err != nil {
				*problems = append(*problems, err)
			}
		}
	}
	return nil
}

// indexAlias maps one alias of node i, after checking it is valid and unused.
func (t *Tree) indexAlias(i int, alias string) *Error {
	path := t.nodes[i].Path
	if err := ValidateAlias(alias); err != nil {
		return &Error{Path: path, Alias: alias, Err: err}
	}
	if owner, taken := t.byID[alias]; taken {
		return &Error{
			Path:  path,
			Other: t.nodes[owner].Path,
			Alias: alias,
			Err:   fmt.Errorf("%w alias %q: the id of", ErrDuplicate, alias),
		}
	}
	if owner, taken := t.byAlias[alias]; taken {
		return &Error{
			Path:  path,
			Other: t.nodes[owner].Path,
			Alias: alias,
			Err:   fmt.Errorf("%w alias %q: also an alias of", ErrDuplicate, alias),
		}
	}
	t.byAlias[alias] = i
	return nil
}

// Dir returns the directory the tree was loaded from.
func (t *Tree) Dir() string {
	return t.dir
}

// Nodes returns every node, ordered by id.
func (t *Tree) Nodes() []Node {
	return append([]Node(nil), t.nodes...)
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
