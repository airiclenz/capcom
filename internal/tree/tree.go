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
	idPattern    = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+){0,3}$`)
	aliasPattern = regexp.MustCompile(`^[a-z0-9.-]+$`)
)

// Sentinel errors, reachable through errors.Is on every error this package returns.
var (
	// ErrInvalidID reports an id that is not a 1-4 word kebab slug of at most 40 characters.
	ErrInvalidID = errors.New("invalid id")
	// ErrInvalidAlias reports an alias outside [a-z0-9.-]+.
	ErrInvalidAlias = errors.New("invalid alias")
	// ErrDuplicate reports an id or alias used twice anywhere in one tree.
	ErrDuplicate = errors.New("duplicate")
	// ErrRepeatedAlias reports an alias that repeats its own node's id or is listed twice in its
	// own file. Every error that wraps it also wraps ErrDuplicate.
	ErrRepeatedAlias = errors.New("repeated alias")
	// ErrOrphanDirectory reports a child directory without the node file of the same name, or
	// whose node file has an invalid id.
	ErrOrphanDirectory = errors.New("directory without its node file")
	// ErrNotFound reports an argument that matches no id, alias or id prefix.
	ErrNotFound = errors.New("not found")
	// ErrAmbiguous reports an id prefix that matches more than one node; see AmbiguousError.
	ErrAmbiguous = errors.New("ambiguous id prefix")
	// ErrNotRegular reports an entry of a store directory that is neither a regular file nor a
	// directory: a symlink, a named pipe, a socket or a device. It is never read.
	ErrNotRegular = errors.New("not a regular file")
	// ErrTooLarge reports a node file larger than maxNodeFileSize. It is never read.
	ErrTooLarge = errors.New("file is larger than 1 MiB")
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
	// duplicates maps every id or alias two or more files claim to the first problem Load reported
	// for it, which names two of those files.
	duplicates map[string]*Error
}

// ValidateID checks that id is a kebab slug of one to four lowercase words or numbers, at most 40
// characters long. The error wraps ErrInvalidID.
func ValidateID(id string) error {
	if len(id) > maxIDLength {
		return fmt.Errorf("%w %q: longer than %d characters", ErrInvalidID, id, maxIDLength)
	}
	if !idPattern.MatchString(id) {
		return fmt.Errorf(
			"%w %q: must be 1 to 4 words of a-z and 0-9 joined by single hyphens",
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
// Load reports, as an *Error naming the file, an invalid id, an alias CheckAliases rejects (its
// message naming the "node"), an id or alias two files share (naming the other file in Other), and
// every entry Walk leaves out (an entry that is not a regular file, a node file over 1 MiB, a child
// directory without its node file), as Walk words it. It finds every such problem in one pass and returns them all as Errors,
// together with the tree of what it could place: a file with an invalid id, a directory without its
// node file (with its whole subtree), a repeated id after its first file and an invalid or repeated
// alias are left out. A
// directory that cannot be read and an AliasFunc error stop the load and return no tree; the
// AliasFunc error comes back unchanged.
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

// Walk lists the node files of the tree stored in dir: the one directory walk of ADR 0001, which
// Load and the index both build on. A missing dir walks as an empty tree. Names starting with a dot
// and files not ending in .md are skipped. Each Node's Path is joined onto dir and its Aliases are
// nil; the nodes come in walk order, each directory's files by name before its child directories.
//
// Walk reports, as an *Error naming the file or directory, a node file whose name is not a valid id
// (wrapping ErrInvalidID), an entry that is neither a regular file nor a directory, a symlink
// included, whatever its name (wrapping ErrNotRegular), a node file larger than 1 MiB by its Lstat
// (wrapping ErrTooLarge) and a child directory that has no node file beside it that the walk kept
// (wrapping ErrOrphanDirectory), in walk order. Such a file, and such a directory with its whole
// subtree, are left out; no file is opened, so a named pipe never blocks the walk. A directory that
// cannot be read stops the walk, and so does a node file whose Lstat fails other than by being
// gone: its os error comes back unchanged, with no nodes and no problems.
func Walk(dir string) ([]Node, Errors, error) {
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	var walker treeWalker
	if err := walker.walk(dir, rootParentID); err != nil {
		return nil, nil, err
	}
	return walker.nodes, walker.problems, nil
}

// The reasons a node file is left out, as the problem of the directory beside it words them.
const (
	invalidIDReason  = "has an invalid id"
	notRegularReason = "is not a regular file"
	tooLargeReason   = "is larger than 1 MiB"
)

// invalidIDDirectoryError is the problem of a child directory whose node file is there but left
// out: named by an invalid id, not a regular file, or larger than 1 MiB. It wraps
// ErrOrphanDirectory, since the directory is left out like one without its node file, while its
// text names the node file and why it was left out.
type invalidIDDirectoryError struct {
	name   string
	reason string
}

// Error renders the problem without its path.
func (e *invalidIDDirectoryError) Error() string {
	return fmt.Sprintf("directory left out: its node file %s%s %s", e.name, nodeExtension, e.reason)
}

// Unwrap returns ErrOrphanDirectory.
func (e *invalidIDDirectoryError) Unwrap() error {
	return ErrOrphanDirectory
}

// treeWalker collects the nodes and the problems of one Walk.
type treeWalker struct {
	nodes    []Node
	problems Errors
}

// walk collects the nodes of dir, all children of parentID, and recurses into the child
// directories that have their node file, adding every entry left out — an invalid id, an entry
// that is not a regular file, a node file over 1 MiB, an orphan directory — to problems.
func (w *treeWalker) walk(dir string, parentID string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	validIDs := map[string]bool{}
	leftOutIDs := map[string]string{}
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
		if !isNode && !isNotRegular(entry.Type()) {
			continue
		}
		path := filepath.Join(dir, name)
		leftOut, err := leftOutFileOf(entry, id)
		if err != nil {
			return err
		}
		if leftOut != nil {
			w.problems = append(w.problems, &Error{Path: path, Err: leftOut.problem})
			if isNode {
				leftOutIDs[id] = leftOut.reason
			}
			continue
		}
		validIDs[id] = true
		w.nodes = append(w.nodes, Node{ID: id, Path: path, ParentID: parentID})
	}

	for _, name := range childDirs {
		path := filepath.Join(dir, name)
		switch {
		case validIDs[name]:
			if err := w.walk(path, name); err != nil {
				return err
			}
		case leftOutIDs[name] != "":
			w.problems = append(w.problems, &Error{
				Path: path,
				Err:  &invalidIDDirectoryError{name: name, reason: leftOutIDs[name]},
			})
		default:
			w.problems = append(w.problems, &Error{
				Path: path,
				Err:  fmt.Errorf("%w: no %s%s beside it; left out", ErrOrphanDirectory, name, nodeExtension),
			})
		}
	}
	return nil
}

// leftOutFile is why a walk leaves a file out: the problem reported at the file, and the reason
// the directory beside it is left out too.
type leftOutFile struct {
	problem error
	reason  string
}

// leftOutFileOf returns why the walk leaves out entry, a file that is a node file by its name (id
// is its name without .md) or is not a regular file: ErrNotRegular, an invalid id, or ErrTooLarge
// by its Lstat size. It returns nil for a node file the walk keeps, and for one gone since its
// directory was read. A failed Lstat is returned unchanged.
func leftOutFileOf(entry fs.DirEntry, id string) (*leftOutFile, error) {
	if isNotRegular(entry.Type()) {
		return &leftOutFile{problem: ErrNotRegular, reason: notRegularReason}, nil
	}
	if err := ValidateID(id); err != nil {
		return &leftOutFile{problem: err, reason: invalidIDReason}, nil
	}
	info, err := entry.Info()
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Size() > maxNodeFileSize {
		return &leftOutFile{problem: ErrTooLarge, reason: tooLargeReason}, nil
	}
	return nil, nil
}

// indexIDs maps every id to its node and every parent to its children. A repeated id is added to
// problems and recorded as a duplicate, and its node dropped, so the first file of the id keeps it.
func (t *Tree) indexIDs(problems *Errors) {
	kept := t.nodes[:0]
	for _, node := range t.nodes {
		if first, taken := t.byID[node.ID]; taken {
			problem := &Error{
				Path:  node.Path,
				Other: kept[first].Path,
				Err:   fmt.Errorf("%w id %q, also at", ErrDuplicate, node.ID),
			}
			*problems = append(*problems, problem)
			t.recordDuplicate(node.ID, problem)
			continue
		}
		t.byID[node.ID] = len(kept)
		t.children[node.ParentID] = append(t.children[node.ParentID], len(kept))
		kept = append(kept, node)
	}
	t.nodes = kept
}

// CheckAliases checks the aliases of the one node file at path on their own: each must be a valid
// alias, must not repeat the node's own id and must not be listed twice. kind names the node in a
// message: alias "x" repeats the <kind>'s own id. It returns the aliases that pass, in order, and
// an *Error naming path and the alias for each one that does not; an invalid alias wraps
// ErrInvalidAlias, a repeated one ErrRepeatedAlias and ErrDuplicate. The first of an alias listed
// twice passes.
func CheckAliases(path, kind, id string, aliases []string) ([]string, Errors) {
	var kept []string
	var problems Errors
	seen := map[string]bool{}
	for _, alias := range aliases {
		err := checkAlias(kind, id, alias, seen[alias])
		seen[alias] = true
		if err != nil {
			problems = append(problems, &Error{Path: path, Alias: alias, Err: err})
			continue
		}
		kept = append(kept, alias)
	}
	return kept, problems
}

// checkAlias checks one alias of a node file, isSeen telling whether the file lists it earlier.
func checkAlias(kind, id, alias string, isSeen bool) error {
	if err := ValidateAlias(alias); err != nil {
		return err
	}
	if alias == id {
		return &repeatedAliasError{message: fmt.Sprintf("alias %q repeats the %s's own id", alias, kind)}
	}
	if isSeen {
		return &repeatedAliasError{message: fmt.Sprintf("alias %q is listed twice", alias)}
	}
	return nil
}

// repeatedAliasError is an alias that repeats its own node's id or is listed twice. It renders as
// its message alone and wraps both ErrRepeatedAlias and ErrDuplicate.
type repeatedAliasError struct {
	message string
}

// Error returns the message.
func (e *repeatedAliasError) Error() string {
	return e.message
}

// Unwrap returns the sentinels, so errors.Is matches ErrRepeatedAlias and ErrDuplicate.
func (e *repeatedAliasError) Unwrap() []error {
	return []error{ErrRepeatedAlias, ErrDuplicate}
}

// indexAliases reads every node's Aliases and maps each to its node. An alias CheckAliases rejects,
// or one that repeats another node's id or alias, is added to problems and not mapped; the latter
// is also recorded as a duplicate. Load has no kind, so a message names the node. It fails only
// when aliases does.
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
		kept, rejected := CheckAliases(node.Path, "node", node.ID, names)
		*problems = append(*problems, rejected...)
		for _, alias := range kept {
			if problem := t.indexAlias(i, alias); problem != nil {
				*problems = append(*problems, problem)
				t.recordDuplicate(alias, problem)
			}
		}
	}
	return nil
}

// indexAlias maps one alias of node i, which CheckAliases passed, after checking no other node has
// it as its id or an alias.
func (t *Tree) indexAlias(i int, alias string) *Error {
	path := t.nodes[i].Path
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

// recordDuplicate records problem, which names two files claiming name, as the duplicate of name,
// unless an earlier problem already is.
func (t *Tree) recordDuplicate(name string, problem *Error) {
	if _, isRecorded := t.duplicates[name]; !isRecorded {
		t.duplicates[name] = problem
	}
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
