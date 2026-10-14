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

// readContentFacts returns the FileFact of every node, in the same order, each node read with
// content. A node that parses has its Aliases set, and its fact claims the ones CheckAliases
// passes; one with an alias CheckAliases rejects is unusable. A node that does not parse keeps nil
// Aliases, and its fact claims the recovered aliases CheckAliases passes, their problems left
// unreported. It returns the problems of the aliases CheckAliases rejects in the files that parse,
// in node order, and the error content returned for each node, by index; a node whose read failed
// claims its id alone. A nil content reads none and leaves every fact UnreadFile.
func readContentFacts(nodes []Node, content ContentFunc) ([]FileFact, Errors, map[int]error) {
	facts := make([]FileFact, 0, len(nodes))
	var rejected Errors
	failures := map[int]error{}
	for i := range nodes {
		node := &nodes[i]
		fact := FileFact{
			Key:      node.Path,
			ID:       node.ID,
			ParentID: node.ParentID,
			Dir:      filepath.Dir(node.Path),
			Parse:    UnreadFile,
			IsUsable: true,
			Origin:   WalkOrigin,
		}
		if content == nil {
			facts = append(facts, fact)
			continue
		}
		read, err := content(node.Path)
		if err != nil {
			failures[i] = err
			facts = append(facts, fact)
			continue
		}
		kept, problems := CheckAliases(node.Path, "node", node.ID, read.Aliases)
		fact.Aliases, fact.Parse = kept, read.Parse
		fact.IsUsable = read.IsUsable && len(problems) == 0
		if read.Parse == ParsedFile {
			node.Aliases = read.Aliases
			rejected = append(rejected, problems...)
		}
		facts = append(facts, fact)
	}
	return facts, rejected, failures
}

// claimOnlyFacts returns the facts of the files Walk left out that still claim names, among
// problems, Walk's: each node file with a valid id it left out unread (not a regular file, or over
// 1 MiB), claiming its id, and each node file with a valid id at any depth below an orphan
// directory it reported, claiming its id and the aliases content reads, the recovered ones of a
// file that does not parse included. Walk has already reported each such directory, so one that
// cannot be listed claims nothing, and a file content fails to read claims its id alone. A nil
// content reads no aliases.
func claimOnlyFacts(problems Errors, content ContentFunc) []FileFact {
	var facts []FileFact
	for _, problem := range problems {
		isUnread := errors.Is(problem, ErrNotRegular) || errors.Is(problem, ErrTooLarge)
		if fact, isNodeFile := claimOnlyFact(problem.Path); isUnread && isNodeFile {
			facts = append(facts, fact)
		}
	}
	for _, dir := range outermostOrphanDirs(problems) {
		files, err := listNodeFiles(dir)
		if err != nil {
			continue
		}
		for _, path := range files {
			fact, isNodeFile := claimOnlyFact(path)
			if !isNodeFile {
				continue
			}
			if content != nil {
				if read, err := content(path); err == nil {
					fact.Aliases, _ = CheckAliases(path, "node", fact.ID, read.Aliases)
					fact.Parse = read.Parse
				}
			}
			facts = append(facts, fact)
		}
	}
	return facts
}

// claimOnlyFact returns the claim-only fact of the file at path, unread and claiming its id, and
// whether path is a node file with a valid id.
func claimOnlyFact(path string) (FileFact, bool) {
	id := IDOf(path)
	if id == filepath.Base(path) || ValidateID(id) != nil {
		return FileFact{}, false
	}
	fact := FileFact{Key: path, ID: id, Dir: filepath.Dir(path), Parse: UnreadFile, Origin: ClaimOnlyOrigin}
	return fact, true
}

// firstAliasFailure returns the first of failures, by fact, in the order Load reads aliases: the
// kept nodes, then the files left out, each in membership's order; nil when there is none.
func firstAliasFailure(membership Membership, failures map[int]error) error {
	for _, i := range membership.Kept {
		if err := failures[i]; err != nil {
			return err
		}
	}
	for _, file := range membership.LeftOut {
		if err := failures[file.Fact]; err != nil {
			return err
		}
	}
	return nil
}

// conflictErrors returns the problem of each of conflicts, in order.
func conflictErrors(conflicts []Conflict) Errors {
	problems := make(Errors, 0, len(conflicts))
	for _, conflict := range conflicts {
		problems = append(problems, conflictError(conflict))
	}
	return problems
}

// conflictError returns the problem of conflict, as Load reports it: a repeated id when Key claims
// the name as its id, otherwise a duplicate alias naming how Other claims it.
func conflictError(conflict Conflict) *Error {
	if !conflict.IsAlias {
		return DuplicateIDError(conflict.Key, conflict.Other, conflict.Name)
	}
	return DuplicateAliasError(conflict.Key, conflict.Other, conflict.Name, conflict.IsOtherAlias)
}

// DuplicateAliasError returns the problem of path, a node file listing alias, which other also
// claims, as its id or, when isOtherAlias, as an alias: an *Error wrapping ErrDuplicate, as Load
// reports a duplicate alias.
func DuplicateAliasError(path, other, alias string, isOtherAlias bool) *Error {
	claim := idClaim
	if isOtherAlias {
		claim = aliasClaim
	}
	return newDuplicateAliasError(path, other, alias, claim)
}

// RepeatedIDDirProblems returns the problem of each of dirs, DecideMembership's RepeatedIDDirs,
// that is a directory on disk, as left out for the duplicate id of its node file. A dir that is
// gone, or is no directory, is no problem; any other failed Lstat is returned unchanged, an
// *fs.PathError naming the dir. Load and the index's walk both Lstat through it.
func RepeatedIDDirProblems(dirs []RepeatedIDDir) (Errors, error) {
	var problems Errors
	for _, dir := range dirs {
		info, err := os.Lstat(dir.Path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			continue
		}
		problems = append(problems, DuplicateIDDirectoryError(dir.Path, dir.ID))
	}
	return problems, nil
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
