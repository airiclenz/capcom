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

// Resolve finds the node an argument names: an exact id first, then an Alias, then the one id the
// argument is a prefix of. It fails with an *AmbiguousError when the prefix matches several ids,
// and with ErrNotFound when it matches none.
func (t *Tree) Resolve(arg string) (Node, error) {
	if node, found := t.Lookup(arg); found {
		return node, nil
	}

	var candidates []string
	if arg != "" {
		for _, node := range t.nodes {
			if strings.HasPrefix(node.ID, arg) {
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
