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

// NameReader reads one kind's names as ResolveIn resolves them. A Tree's in-memory maps and the
// index's read snapshot both implement it, so one algorithm resolves an argument over either.
type NameReader interface {
	// Match returns what is known of exactly name.
	Match(name string) (NameMatch, error)
	// PrefixCandidates returns the ids name is a prefix of, each once and in order: the id of every
	// node file the walk found, kept or left out for a repeated id, with problems or without. A file
	// the walk leaves out unread or below an orphan directory counts as none. An empty name has
	// none.
	PrefixCandidates(name string) ([]string, error)
}

// NameMatch is what a NameReader knows of one exact name: the membership verdict that refuses it, and the
// node of the tree it names.
type NameMatch struct {
	// ID is the id of the node of the tree whose id is the name, or, with IsAlias set, of the node
	// that has the name as an Alias; "" when no node answers to it.
	ID      string
	IsAlias bool
	// Refusal is why the name must not be resolved; zero when nothing refuses it.
	Refusal RefusalReason
	// Conflict is the conflict that refuses the name, as DecideMembership gave it; zero when
	// Refusal is.
	Conflict Conflict
}

// refusalError returns the error a resolve of arg fails with for the match's refusal: an
// *UnusableError naming the file that alone claims the name, or the duplicate problem of its
// Conflict, as Load reports it.
func (m NameMatch) refusalError(arg string) error {
	if m.Refusal == UnusableRefusal {
		return &UnusableError{Arg: arg, Path: m.Conflict.Key}
	}
	return conflictError(m.Conflict)
}

// ResolveIn finds the id of the node an argument names among names, once NormalizeArg has turned a
// citation spelling into a name: an exact id first, then an Alias, then the one id the name is a
// prefix of. It fails with an *AmbiguousError when the prefix matches several ids, and with
// ErrNotFound when it matches none; both quote the argument as given. An error names returns comes
// back unchanged.
//
// Before any of that, a name the membership verdicts refuse fails: one that two or more files
// claim, or that stands for a repeated id, with an *Error wrapping ErrDuplicate naming two of those
// files, and one only an unusable file claims with an *UnusableError naming that file. A refused
// name is never matched as a prefix of another id, and the id of a node file with problems or left
// out for a repeated id still counts as a prefix candidate, so a prefix it shares with another id
// stays ambiguous. A prefix of one such id alone fails as that id refused, unless a usable node has
// the id and the refusal is a duplicate: the node wins.
func ResolveIn(names NameReader, arg string) (string, error) {
	name := NormalizeArg(arg)
	match, err := names.Match(name)
	if err != nil {
		return "", err
	}
	if match.Refusal != 0 {
		return "", match.refusalError(arg)
	}
	if match.ID != "" {
		return match.ID, nil
	}

	candidates, err := names.PrefixCandidates(name)
	if err != nil {
		return "", err
	}
	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("id %q %w", arg, ErrNotFound)
	case 1:
		return prefixMatch(names, arg, candidates[0])
	default:
		return "", &AmbiguousError{Arg: arg, Candidates: candidates}
	}
}

// prefixMatch returns id, the one candidate the prefix arg matched, when a node of the tree has it
// as its id, unless only an unusable file claims it. Otherwise it fails as the exact name id would,
// quoting arg.
func prefixMatch(names NameReader, arg, id string) (string, error) {
	match, err := names.Match(id)
	switch {
	case err != nil:
		return "", err
	case match.Refusal == UnusableRefusal:
		return "", match.refusalError(arg)
	case match.ID != "" && !match.IsAlias:
		return match.ID, nil
	case match.Refusal != 0:
		return "", match.refusalError(arg)
	default:
		return "", fmt.Errorf("id %q %w", arg, ErrNotFound)
	}
}

// Resolve finds the node an argument names, as ResolveIn finds it among the tree's names. The
// id or an alias of a node file Load left out below a repeated id, and an alias of the first file
// of a repeated id, fail with the problem of that repeated id, naming both its files. A name only
// an unusable file claims is the id or an alias of a file that does not parse, fails its kind's
// validation or has an alias CheckAliases rejects, an alias only a file left out for a repeated id
// lists, and the id or an alias of a node file the walk left out unread or below an orphan
// directory. A tree Load built sees only the aliases its AliasFunc reads.
//
// The exact id of a node whose file does not parse, which no other file claims, is the one
// exception: it resolves to the node, so a removal can still take the file away. A prefix of that
// id alone still fails as an unusable claim.
func (t *Tree) Resolve(arg string) (Node, error) {
	name := NormalizeArg(arg)
	if t.unparsedIDs[name] && t.refusals[name].Refusal == UnusableRefusal {
		return t.nodes[t.byID[name]], nil
	}
	id, err := ResolveIn(t, arg)
	if err != nil {
		return Node{}, err
	}
	return t.nodes[t.byID[id]], nil
}

// Match returns the refusal Load's membership verdicts give name, and the node whose id or alias
// it is. It never fails.
func (t *Tree) Match(name string) (NameMatch, error) {
	verdict := t.refusals[name]
	match := NameMatch{Refusal: verdict.Refusal, Conflict: verdict.Conflict}
	if i, found := t.byID[name]; found {
		match.ID = t.nodes[i].ID
	} else if i, found := t.byAlias[name]; found {
		match.ID, match.IsAlias = t.nodes[i].ID, true
	}
	return match, nil
}

// PrefixCandidates returns the ids of the nodes and of the node files Load left out for a repeated
// id that name is a prefix of, as NameReader says. It never fails.
func (t *Tree) PrefixCandidates(name string) ([]string, error) {
	if name == "" {
		return nil, nil
	}
	var candidates []string
	for _, node := range slices.Concat(t.nodes, t.leftOut) {
		if strings.HasPrefix(node.ID, name) {
			candidates = append(candidates, node.ID)
		}
	}
	slices.Sort(candidates)
	return slices.Compact(candidates), nil
}
