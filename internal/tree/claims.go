package tree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/nodefile"
)

// The claims a duplicate alias problem names its Other file by.
const (
	idClaim    = "the id of"
	aliasClaim = "also an alias of"
)

// DuplicateIDError returns the problem of path, a node file whose id other, an earlier node file,
// also has: an *Error wrapping ErrDuplicate, as Load reports a repeated id.
func DuplicateIDError(path, other, id string) *Error {
	return &Error{Path: path, Other: other, Err: fmt.Errorf("%w id %q, also at", ErrDuplicate, id)}
}

// DuplicateIDDirectoryError returns the problem of dir, the directory beside a node file of the
// repeated id id: an *Error wrapping ErrOrphanDirectory, worded as Load reports the directory left
// out with its whole subtree.
func DuplicateIDDirectoryError(dir, id string) *Error {
	return &Error{Path: dir, Err: &invalidIDDirectoryError{name: id, reason: duplicateReason}}
}

// droppedNode is a node file Load leaves out for a repeated id, with the problem of the repeated id
// its aliases stand for: set for the first file of the id and for a node file below one of its
// files, and nil for a file of the id after the first, whose aliases stay its own.
type droppedNode struct {
	node     Node
	repeated *Error
}

// repeatedID is an id two or more node files share: the first of its files, by path, and the
// problem of its second file, which names both.
type repeatedID struct {
	first   string
	problem *Error
}

// repeatedIDDir is a directory beside a file of a repeated id, the id, and that id's problem.
type repeatedIDDir struct {
	path     string
	id       string
	repeated *Error
}

// indexIDs maps every id to its node and every parent to its children. Every file of a repeated id
// is dropped, and so is every node file under a directory beside one of them, whose id is recorded
// as a duplicate with the repeated id's problem. Each directory beside a file of a repeated id that
// exists is added to problems as left out. It returns the dropped nodes, in order, and fails only
// when such a directory cannot be Lstat.
func (t *Tree) indexIDs(problems *Errors) ([]droppedNode, error) {
	repeated := t.repeatedIDs(problems)
	var dirs []repeatedIDDir
	for _, node := range t.nodes {
		if id := repeated[node.ID]; id != nil {
			dirs = append(dirs, repeatedIDDir{
				path:     ChildrenDir(node.Path),
				id:       node.ID,
				repeated: id.problem,
			})
		}
	}

	var dropped []droppedNode
	kept := t.nodes[:0]
	for _, node := range t.nodes {
		if id := repeated[node.ID]; id != nil {
			entry := droppedNode{node: node}
			if node.Path == id.first {
				entry.repeated = id.problem
			}
			t.repeatedFiles = append(t.repeatedFiles, len(dropped))
			dropped = append(dropped, entry)
			continue
		}
		if dir := enclosingDir(dirs, node.Path); dir != nil {
			t.recordDuplicate(node.ID, dir.repeated)
			dropped = append(dropped, droppedNode{node: node, repeated: dir.repeated})
			continue
		}
		t.byID[node.ID] = len(kept)
		t.children[node.ParentID] = append(t.children[node.ParentID], len(kept))
		kept = append(kept, node)
	}
	t.nodes = kept
	for _, entry := range dropped {
		t.leftOut = append(t.leftOut, entry.node)
	}
	return dropped, addRepeatedIDDirs(dirs, problems)
}

// repeatedIDs finds the ids two or more of the tree's nodes share, which are ordered by id, then
// path. It adds one problem per file of such an id after the first, naming the first in Other, and
// records the id as a duplicate with the first such problem.
func (t *Tree) repeatedIDs(problems *Errors) map[string]*repeatedID {
	repeated := map[string]*repeatedID{}
	firstPaths := map[string]string{}
	for _, node := range t.nodes {
		first, isSeen := firstPaths[node.ID]
		if !isSeen {
			firstPaths[node.ID] = node.Path
			continue
		}
		problem := DuplicateIDError(node.Path, first, node.ID)
		*problems = append(*problems, problem)
		t.recordDuplicate(node.ID, problem)
		if repeated[node.ID] == nil {
			repeated[node.ID] = &repeatedID{first: first, problem: problem}
		}
	}
	return repeated
}

// enclosingDir returns the innermost of dirs that path lies under, or nil when it lies under none.
func enclosingDir(dirs []repeatedIDDir, path string) *repeatedIDDir {
	var enclosing *repeatedIDDir
	for i := range dirs {
		dir := &dirs[i]
		if !strings.HasPrefix(path, dir.path+string(filepath.Separator)) {
			continue
		}
		if enclosing == nil || len(dir.path) > len(enclosing.path) {
			enclosing = dir
		}
	}
	return enclosing
}

// addRepeatedIDDirs adds to problems each of dirs that is a directory on disk, as left out for the
// duplicate id of its node file. A dir that is gone, or is no directory, is no problem; any other
// failed Lstat is returned unchanged.
func addRepeatedIDDirs(dirs []repeatedIDDir, problems *Errors) error {
	for _, dir := range dirs {
		info, err := os.Lstat(dir.path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			continue
		}
		*problems = append(*problems, DuplicateIDDirectoryError(dir.path, dir.id))
	}
	return nil
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
	if problem := t.claimedAlias(t.nodes[i].Path, alias); problem != nil {
		return problem
	}
	t.byAlias[alias] = i
	return nil
}

// checkDroppedAliases reads the Aliases of every node indexIDs dropped for a repeated id, once the
// kept nodes' aliases are mapped, and checks them without mapping any; LeftOut returns them with
// their Aliases. An alias CheckAliases rejects is added to problems; one a kept node has as its id
// or an alias, or an earlier dropped node lists, is added to problems and recorded as a duplicate,
// so Resolve refuses it. Any other alias of a dropped node that stands for a repeated id (the first
// file of the id, or a node file below one of its files) is recorded as a duplicate with that id's
// problem, so it never falls to prefix matching; one only a later file of the id lists is left
// alone. It fails only when aliases does.
func (t *Tree) checkDroppedAliases(aliases AliasFunc, dropped []droppedNode, problems *Errors) error {
	if aliases == nil {
		return nil
	}
	droppedOwners := map[string]string{}
	for i, entry := range dropped {
		node := entry.node
		names, err := aliases(node.Path)
		if err != nil {
			return err
		}
		t.leftOut[i].Aliases = names
		kept, rejected := CheckAliases(node.Path, "node", node.ID, names)
		*problems = append(*problems, rejected...)
		for _, alias := range kept {
			problem := t.claimedAlias(node.Path, alias)
			if other, taken := droppedOwners[alias]; problem == nil && taken {
				problem = newDuplicateAliasError(node.Path, other, alias, aliasClaim)
			}
			if problem == nil {
				droppedOwners[alias] = node.Path
				if entry.repeated != nil {
					t.recordDuplicate(alias, entry.repeated)
				}
				continue
			}
			*problems = append(*problems, problem)
			t.recordDuplicate(alias, problem)
		}
	}
	return nil
}

// claimedAlias returns the problem of the file at path listing alias when a kept node already has
// it as its id or a mapped alias, naming that node's file in Other, and nil otherwise.
func (t *Tree) claimedAlias(path, alias string) *Error {
	if owner, taken := t.byID[alias]; taken {
		return newDuplicateAliasError(path, t.nodes[owner].Path, alias, idClaim)
	}
	if owner, taken := t.byAlias[alias]; taken {
		return newDuplicateAliasError(path, t.nodes[owner].Path, alias, aliasClaim)
	}
	return nil
}

// newDuplicateAliasError is the problem of the file at path listing alias, which the file other
// also claims, claim wording how.
func newDuplicateAliasError(path, other, alias, claim string) *Error {
	return &Error{
		Path:  path,
		Other: other,
		Alias: alias,
		Err:   fmt.Errorf("%w alias %q: %s", ErrDuplicate, alias, claim),
	}
}

// recordDuplicate records problem, which names two files claiming name, as the duplicate of name,
// unless an earlier problem already is.
func (t *Tree) recordDuplicate(name string, problem *Error) {
	if _, isRecorded := t.duplicates[name]; !isRecorded {
		t.duplicates[name] = problem
	}
}

// ClaimKind is how the file a Claim names stands in a Tree.
type ClaimKind int

// The kinds of file that can hold a name, in Claimant's order of precedence.
const (
	// LiveClaim is a node of the tree.
	LiveClaim ClaimKind = iota + 1
	// OrphanClaim is a node file below an orphan directory Load left out.
	OrphanClaim
	// DuplicateClaim is a file Load left out for its repeated id.
	DuplicateClaim
	// UnparsedClaim is a file from which the recovering AliasFunc reads the name.
	UnparsedClaim
)

// Claim is a file that holds a name, as Claimant found it.
type Claim struct {
	// Path is the file, joined onto the directory passed to Load.
	Path string
	// Holder is the file's id: its file name without the .md extension.
	Holder string
	// IsAlias reports whether the file holds the name as an alias rather than as its id.
	IsAlias bool
	// Kind is how the file stands in the tree.
	Kind ClaimKind
	// Dir is, for an OrphanClaim, the Path of the ErrOrphanDirectory problem the file lies under,
	// and "" otherwise.
	Dir string
}

// Claimant returns the first file that holds name, by its file name or as an alias, in this order:
//  1. a node of the tree, by its id or an alias;
//  2. a node file at any depth below an orphan directory Load reported, the directories beside the
//     files of a repeated id included, its aliases read with Load's AliasFunc;
//  3. a file Load left out for its repeated id, with the aliases Load's AliasFunc read;
//  4. a file of the above from which recover reads name, which holds it as an alias.
//
// A file whose path is except is skipped; "" skips none. Below an orphan directory, names starting
// with a dot are skipped and only regular files whose name ends in .md are read, as Walk does; a
// directory gone since Load lists as empty. recover may be nil, which skips step 4. It reports
// false when no file holds name. An AliasFunc error comes back unchanged, and so does any other
// failure to read an orphan directory.
func (t *Tree) Claimant(name string, except string, recover AliasFunc) (Claim, bool, error) {
	if claim, found := t.liveClaim(name, except); found {
		return claim, true, nil
	}
	claim, found, err := t.orphanClaim(name, except)
	if err != nil || found {
		return claim, found, err
	}
	claim, found = t.duplicateClaim(name, except)
	if found || recover == nil {
		return claim, found, nil
	}
	return t.unparsedClaim(name, except, recover)
}

// liveClaim returns the node of the tree whose id or one of whose Aliases is name, unless its path
// is except.
func (t *Tree) liveClaim(name string, except string) (Claim, bool) {
	if i, found := t.byID[name]; found && t.nodes[i].Path != except {
		return Claim{Path: t.nodes[i].Path, Holder: t.nodes[i].ID, Kind: LiveClaim}, true
	}
	if i, found := t.byAlias[name]; found && t.nodes[i].Path != except {
		return Claim{Path: t.nodes[i].Path, Holder: t.nodes[i].ID, IsAlias: true, Kind: LiveClaim}, true
	}
	return Claim{}, false
}

// orphanClaim returns the first node file below one of the tree's orphan directories, other than
// except, that is named name or from which Load's AliasFunc reads name.
func (t *Tree) orphanClaim(name string, except string) (Claim, bool, error) {
	path, dir, err := t.findOrphanFile(except, func(path string) (bool, error) {
		if IDOf(path) == name {
			return true, nil
		}
		return holdsAlias(t.aliases, path, name)
	})
	if err != nil || path == "" {
		return Claim{}, false, err
	}
	holder := IDOf(path)
	claim := Claim{Path: path, Holder: holder, IsAlias: holder != name, Kind: OrphanClaim, Dir: dir}
	return claim, true, nil
}

// findOrphanFile returns the first node file below one of the tree's orphan directories, other than
// except, that matches, and the orphan directory it lies under, or "" for both when none does.
func (t *Tree) findOrphanFile(
	except string,
	matches func(path string) (bool, error),
) (path string, dir string, err error) {
	for _, dir := range t.orphanDirs {
		files, err := listNodeFiles(dir)
		if err != nil {
			return "", "", err
		}
		for _, path := range files {
			if path == except {
				continue
			}
			isMatch, err := matches(path)
			if err != nil {
				return "", "", err
			}
			if isMatch {
				return path, dir, nil
			}
		}
	}
	return "", "", nil
}

// duplicateClaim returns the first file Load left out for its repeated id, other than except, that
// is named name or lists name among the Aliases Load read.
func (t *Tree) duplicateClaim(name string, except string) (Claim, bool) {
	for _, i := range t.repeatedFiles {
		node := t.leftOut[i]
		if node.Path == except {
			continue
		}
		claim := Claim{Path: node.Path, Holder: node.ID, Kind: DuplicateClaim}
		if node.ID == name {
			return claim, true
		}
		if slices.Contains(node.Aliases, name) {
			claim.IsAlias = true
			return claim, true
		}
	}
	return Claim{}, false
}

// unparsedClaim returns the first file, a node of the tree, then a node file below an orphan
// directory, then a file left out for its repeated id, other than except, from which recover reads
// name.
func (t *Tree) unparsedClaim(name string, except string, recover AliasFunc) (Claim, bool, error) {
	isHeld := func(path string) (bool, error) {
		return holdsAlias(recover, path, name)
	}
	path, err := findNode(t.nodes, except, isHeld)
	if err != nil || path != "" {
		return unparsedClaimOf(path, err)
	}
	path, _, err = t.findOrphanFile(except, isHeld)
	if err != nil || path != "" {
		return unparsedClaimOf(path, err)
	}
	repeated := make([]Node, 0, len(t.repeatedFiles))
	for _, i := range t.repeatedFiles {
		repeated = append(repeated, t.leftOut[i])
	}
	path, err = findNode(repeated, except, isHeld)
	return unparsedClaimOf(path, err)
}

// findNode returns the Path of the first of nodes, other than except, that matches, or "" when none
// does.
func findNode(
	nodes []Node,
	except string,
	matches func(path string) (bool, error),
) (string, error) {
	for _, node := range nodes {
		if node.Path == except {
			continue
		}
		isMatch, err := matches(node.Path)
		if err != nil {
			return "", err
		}
		if isMatch {
			return node.Path, nil
		}
	}
	return "", nil
}

// unparsedClaimOf returns the UnparsedClaim of the file at path, which holds a name as an alias,
// Claimant's result for it: none when path is "" or err is set, and err unchanged.
func unparsedClaimOf(path string, err error) (Claim, bool, error) {
	if err != nil || path == "" {
		return Claim{}, false, err
	}
	return Claim{Path: path, Holder: IDOf(path), IsAlias: true, Kind: UnparsedClaim}, true, nil
}

// holdsAlias reports whether aliases reads name among the Aliases of the file at path. A nil
// aliases reads none.
func holdsAlias(aliases AliasFunc, path string, name string) (bool, error) {
	if aliases == nil {
		return false, nil
	}
	names, err := aliases(path)
	if err != nil {
		return false, err
	}
	return slices.Contains(names, name), nil
}

// outermostOrphanDirs returns the Paths of the ErrOrphanDirectory problems among problems, which
// are ordered by path, that lie under no other of them: listing those reaches every node file below
// any of them.
func outermostOrphanDirs(problems Errors) []string {
	var dirs []string
	for _, problem := range problems {
		if !errors.Is(problem, ErrOrphanDirectory) {
			continue
		}
		if n := len(dirs); n > 0 && isBelow(problem.Path, dirs[n-1]) {
			continue
		}
		dirs = append(dirs, problem.Path)
	}
	return dirs
}

// isBelow reports whether path lies under the directory dir.
func isBelow(path string, dir string) bool {
	return strings.HasPrefix(path, dir+string(filepath.Separator))
}

// listNodeFiles returns the node files at any depth below dir, with the entry rules Walk applies:
// names starting with a dot are skipped, and so is every file that is not a regular file or whose
// name does not end in .md. Each directory's files come by name before its child directories. A
// directory gone since its parent was read, dir included, lists as empty; any other failure to read
// one is returned unchanged.
func listNodeFiles(dir string) ([]string, error) {
	entries, err := readDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files, childDirs []string
	for _, entry := range entries {
		if isSkippedEntry(entry) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		switch {
		case entry.IsDir():
			childDirs = append(childDirs, path)
		case !nodefile.IsNotRegular(entry.Type()):
			files = append(files, path)
		}
	}
	for _, child := range childDirs {
		below, err := listNodeFiles(child)
		if err != nil {
			return nil, err
		}
		files = append(files, below...)
	}
	return files, nil
}
