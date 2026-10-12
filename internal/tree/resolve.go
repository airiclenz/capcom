package tree

import (
	"fmt"
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
// *Error wrapping ErrDuplicate that names two of those files, the problem Load reported for it:
// Load keeps the first file of such a name, but a write must not act on a node another file also
// answers to.
func (t *Tree) Resolve(arg string) (Node, error) {
	name := NormalizeArg(arg)
	if problem, isDuplicate := t.duplicates[name]; isDuplicate {
		duplicate := *problem
		return Node{}, &duplicate
	}
	if node, found := t.Lookup(name); found {
		return node, nil
	}

	var candidates []string
	if name != "" {
		for _, node := range t.nodes {
			if strings.HasPrefix(node.ID, name) {
				candidates = append(candidates, node.ID)
			}
		}
	}

	switch len(candidates) {
	case 0:
		return Node{}, fmt.Errorf("id %q %w", arg, ErrNotFound)
	case 1:
		return t.nodes[t.byID[candidates[0]]], nil
	default:
		return Node{}, &AmbiguousError{Arg: arg, Candidates: candidates}
	}
}
