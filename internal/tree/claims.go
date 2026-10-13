package tree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
				path:     strings.TrimSuffix(node.Path, nodeExtension),
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
