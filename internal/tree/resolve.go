package tree

import (
	"fmt"
	"slices"
	"strings"
)

// AmbiguousError reports an id prefix that matches more than one node. It matches ErrAmbiguous
// under errors.Is.
type AmbiguousError struct {
	// Arg is the argument as given.
	Arg string
	// Candidates are the ids it is a prefix of, in order.
	Candidates []string
}

// Error renders the argument and every candidate id.
func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("%v %q: matches %s", ErrAmbiguous, e.Arg, strings.Join(e.Candidates, ", "))
}

// Is reports whether target is ErrAmbiguous.
func (e *AmbiguousError) Is(target error) bool {
	return target == ErrAmbiguous
}

// UnusableError reports an argument naming what only an unusable file claims (see Resolve). It
// matches ErrNotFound under errors.Is and reads as the index words the same refusal.
type UnusableError struct {
	// Arg is the argument as given.
	Arg string
	// Path is the file that claims the name.
	Path string
}

// Error renders the argument and the file that claims it.
func (e *UnusableError) Error() string {
	return fmt.Sprintf("id %q %v: %s is invalid", e.Arg, ErrNotFound, e.Path)
}

// Is reports whether target is ErrNotFound.
func (e *UnusableError) Is(target error) bool {
	return target == ErrNotFound
}

// Lookup returns the node whose id or one of whose Aliases is exactly name.
func (t *Tree) Lookup(name string) (Node, bool) {
	if i, found := t.byID[name]; found {
		return t.nodes[i], true
	}
	if i, found := t.byAlias[name]; found {
		return t.nodes[i], true
	}
	return Node{}, false
}

// NormalizeArg returns the name a citation spelling of an argument stands for: trimmed,
// lowercased, and with each run of whitespace turned into one hyphen, so "ADR 0076" names
// adr-0076. Dots and hyphens stay as they are, which keeps legacy aliases (apogee-kk0.4) intact.
func NormalizeArg(arg string) string {
	return strings.Join(strings.Fields(strings.ToLower(arg)), "-")
}

// Resolve finds the node an argument names, once NormalizeArg has turned a citation spelling into
// a name: an exact id first, then an Alias, then the one id the name is a prefix of. It fails with
// an *AmbiguousError when the prefix matches several ids, and with ErrNotFound when it matches
// none; both quote the argument as given.
//
// Before any of that, a name that two or more files claim as their id or an alias fails with an
// *Error wrapping ErrDuplicate that names two of those files, the problem Load reported for it, so
// a write never acts on a node another file also answers to. The id or an alias of a node file Load
// left out below a repeated id, and an alias of the first file of a repeated id, fail the same way
// with the problem of that repeated id, naming both its files. The ids of the files LeftOut returns
// count as prefix candidates, so a prefix one of them shares with another id stays ambiguous, and
// a prefix of one of them alone fails with its duplicate problem.
//
// A name that only an unusable file claims fails with an *UnusableError naming that file, and is
// never matched as a prefix of another id: the id or an alias of a file that does not parse, fails
// its kind's validation or has an alias CheckAliases rejects, an alias only a file left out for a
// repeated id lists, and the id or an alias of a node file the walk left out unread or below an
// orphan directory. A tree Load built sees only the aliases its AliasFunc reads. The exact id of a
// node whose file does not parse is the one exception: it resolves to the node, so a removal can
// still take the file away. A prefix of the id of one unusable node alone fails the same way.
func (t *Tree) Resolve(arg string) (Node, error) {
	name := NormalizeArg(arg)
	if problem, isDuplicate := t.duplicates[name]; isDuplicate {
		duplicate := *problem
		return Node{}, &duplicate
	}
	if path, isUnusable := t.unusables[name]; isUnusable && !t.unparsedIDs[name] {
		return Node{}, &UnusableError{Arg: arg, Path: path}
	}
	if node, found := t.Lookup(name); found {
		return node, nil
	}

	candidates := t.prefixCandidates(name)
	switch len(candidates) {
	case 0:
		return Node{}, fmt.Errorf("id %q %w", arg, ErrNotFound)
	case 1:
		if path, isUnusable := t.unusables[candidates[0]]; isUnusable {
			return Node{}, &UnusableError{Arg: arg, Path: path}
		}
		if i, isKept := t.byID[candidates[0]]; isKept {
			return t.nodes[i], nil
		}
		duplicate := *t.duplicates[candidates[0]]
		return Node{}, &duplicate
	default:
		return Node{}, &AmbiguousError{Arg: arg, Candidates: candidates}
	}
}

// prefixCandidates returns the ids name is a prefix of, each once and in order: the ids of the
// nodes and of the node files Load left out for a repeated id. An empty name has none.
func (t *Tree) prefixCandidates(name string) []string {
	if name == "" {
		return nil
	}
	var candidates []string
	for _, node := range slices.Concat(t.nodes, t.leftOut) {
		if strings.HasPrefix(node.ID, name) {
			candidates = append(candidates, node.ID)
		}
	}
	slices.Sort(candidates)
	return slices.Compact(candidates)
}
