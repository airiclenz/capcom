// Package reflist decides the edits to a reference list: the names a node's file writes under deps,
// related or superseded-by, each the id or an Alias of another node of its kind, or text that names
// no node any more. Given the holder, its entries, the argument, a resolver and an entry-names
// lookup, Add returns the canonical id to write and whether it is already present, and Remove
// returns the entries to drop; either refuses with a *Refusal. The package reads and writes no file
// and holds no state; the caller resolves through its own index and applies the decision itself.
package reflist

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/tree"
)

// cycleArrow joins the ids of a refused cycle in Refusal.Error.
const cycleArrow = " -> "

// Reason is why a Refusal refuses an edit.
type Reason int

// The reasons a Refusal gives. Self and Cycle refuse an addition only; Ambiguous and Duplicate
// refuse the argument itself, whichever the edit; NotFound is an argument that names no node, on
// an addition, or a removal that matches no entry.
const (
	// ReasonSelf refuses adding the holder to its own list.
	ReasonSelf Reason = iota + 1
	// ReasonCycle refuses adding an entry that would close a cycle.
	ReasonCycle
	// ReasonNotFound refuses an argument that names nothing the edit can use.
	ReasonNotFound
	// ReasonAmbiguous refuses an id prefix that matches several ids.
	ReasonAmbiguous
	// ReasonDuplicate refuses a name two or more files claim.
	ReasonDuplicate
)

// Refusal is an edit Add or Remove refuses. It wraps the resolver's error, when the resolver
// refused the argument, so errors.Is and errors.As still reach the tree sentinel errors and
// problem types beneath it.
type Refusal struct {
	// Reason is why the edit is refused.
	Reason Reason
	// Arg is the argument as given.
	Arg string
	// Holder is the List's Holder; "" when it has none.
	Holder string
	// ID is the canonical id the argument resolves to; "" when it resolves to none.
	ID string
	// Cycle is the cycle a ReasonCycle refusal would close, from Holder back to Holder; nil for
	// every other reason.
	Cycle []string
	// Err is the resolver's error behind the refusal; nil when the resolver did not fail.
	Err error
}

// Error renders the refusal: the resolver's own error when there is one, otherwise the holder or
// the argument and, for a cycle, the ids along it. A caller words its own message from the fields.
func (r *Refusal) Error() string {
	switch {
	case r.Err != nil:
		return r.Err.Error()
	case r.Reason == ReasonSelf:
		return fmt.Sprintf("%s cannot refer to itself", r.Holder)
	case r.Reason == ReasonCycle:
		return fmt.Sprintf(
			"%s cannot refer to %s: that would create the cycle %s",
			r.Holder,
			r.ID,
			strings.Join(r.Cycle, cycleArrow),
		)
	default:
		return fmt.Sprintf("no entry names %s", r.Arg)
	}
}

// Unwrap returns the resolver's error behind the refusal, or nil.
func (r *Refusal) Unwrap() error {
	return r.Err
}

// ResolveFunc resolves an argument to the canonical id of the node it names, by exact id, Alias or
// unique id prefix, as tree.ResolveIn does. It fails with errors matching tree.ErrNotFound,
// tree.ErrAmbiguous or tree.ErrDuplicate under errors.Is for the arguments those refuse; any other
// error stops the edit unchanged.
type ResolveFunc func(arg string) (id string, err error)

// NamesFunc returns the canonical id of the node an entry names, as the file writes it, by exact id
// or Alias; isFound is false when it names none.
type NamesFunc func(entry string) (id string, isFound bool)

// CycleFunc returns the cycle an edge from -> to would close, from, to, ..., from, or nil when it
// closes none, as issue.DepGraph.FindCycle does.
type CycleFunc func(from string, to string) []string

// List is one reference list and the lookups that decide edits to it.
type List struct {
	// Holder is the canonical id of the node whose list it is; "" for a node not yet written,
	// which no self check applies to.
	Holder string
	// Entries are the list's entries, as the file writes them.
	Entries []string
	// Resolve resolves an argument; it must be set.
	Resolve ResolveFunc
	// Names resolves an entry; it must be set.
	Names NamesFunc
	// FindCycle checks an addition for a cycle; nil skips the check.
	FindCycle CycleFunc
}

// Addition is what Add decided for an argument.
type Addition struct {
	// ID is the canonical id the argument resolves to: the entry to append.
	ID string
	// IsPresent is set when an entry already names ID, by its id or by any Alias, so the addition
	// leaves the list as it is.
	IsPresent bool
}

// Add decides adding arg to the list. It resolves arg, then refuses, in this order: an argument
// that resolves to the Holder (ReasonSelf); an argument the list already names is no refusal but
// an Addition with IsPresent set, even when its edge sits in a cycle; and an edge FindCycle says
// closes a cycle (ReasonCycle). A resolver failure is a Refusal of reason NotFound, Ambiguous or
// Duplicate wrapping it, and any other resolver error comes back unchanged.
func (l List) Add(arg string) (Addition, error) {
	id, err := l.Resolve(arg)
	if err != nil {
		return Addition{}, l.resolveRefusal(arg, err)
	}
	if l.Holder != "" && id == l.Holder {
		return Addition{}, &Refusal{Reason: ReasonSelf, Arg: arg, Holder: l.Holder, ID: id}
	}
	if slices.ContainsFunc(l.Entries, namesID(l.Names, id)) {
		return Addition{ID: id, IsPresent: true}, nil
	}
	if l.FindCycle != nil {
		if cycle := l.FindCycle(l.Holder, id); cycle != nil {
			return Addition{}, &Refusal{Reason: ReasonCycle, Arg: arg, Holder: l.Holder, ID: id, Cycle: cycle}
		}
	}
	return Addition{ID: id}, nil
}

// Removal is what Remove decided for an argument: which entries it drops. Apply it with Drop.
type Removal struct {
	// ID is the canonical id whose entries are dropped, by its id or by any Alias; "" when the
	// entries are matched by the argument's text alone.
	ID string

	arg    string
	holder string
	names  NamesFunc
}

// Remove decides removing arg from the list, against the list's Entries. An argument that is
// exactly a node's id or Alias drops every entry naming that node, by id or by any Alias. One that
// is not drops the entries that are its text, as given or as tree.NormalizeArg spells it, when one
// of the Entries is: so the text beats a prefix, and an ambiguous prefix or a name that resolves to
// no node is matched only by its text. A prefix of one id no entry is the text of drops the entries
// naming that node.
//
// It refuses an ambiguous prefix none of the Entries is the text of (ReasonAmbiguous) and a name
// two files claim (ReasonDuplicate), wrapping the resolver's error; any other resolver error comes
// back unchanged. A removal never refuses the Holder itself, so a hand-written entry naming the
// holder can still be dropped.
func (l List) Remove(arg string) (Removal, error) {
	removal := Removal{arg: arg, holder: l.Holder, names: l.Names}
	id, err := l.Resolve(arg)
	switch {
	case errors.Is(err, tree.ErrNotFound):
		return removal, nil
	case errors.Is(err, tree.ErrAmbiguous) && l.hasText(arg):
		return removal, nil
	case err != nil:
		return Removal{}, l.resolveRefusal(arg, err)
	}
	if _, isExact := l.Names(tree.NormalizeArg(arg)); !isExact && l.hasText(arg) {
		return removal, nil
	}
	removal.ID = id
	return removal, nil
}

// Drop returns the entries of entries the removal drops, in their order. entries is the list
// Remove decided against, or the same list as an edit reads it again. It fails with a Refusal of
// ReasonNotFound when the removal drops none; a caller may ignore that for a no-op removal.
func (r Removal) Drop(entries []string) ([]string, error) {
	isDropped := isText(r.arg)
	if r.ID != "" {
		isDropped = namesID(r.names, r.ID)
	}
	var dropped []string
	for _, entry := range entries {
		if isDropped(entry) {
			dropped = append(dropped, entry)
		}
	}
	if dropped == nil {
		return nil, &Refusal{Reason: ReasonNotFound, Arg: r.arg, Holder: r.holder, ID: r.ID}
	}
	return dropped, nil
}

// resolveRefusal types err, the resolver's failure on arg: a Refusal wrapping it when it is a
// not-found, ambiguous or duplicate refusal, and err itself otherwise.
func (l List) resolveRefusal(arg string, err error) error {
	var reason Reason
	switch {
	case errors.Is(err, tree.ErrNotFound):
		reason = ReasonNotFound
	case errors.Is(err, tree.ErrAmbiguous):
		reason = ReasonAmbiguous
	case errors.Is(err, tree.ErrDuplicate):
		reason = ReasonDuplicate
	default:
		return err
	}
	return &Refusal{Reason: reason, Arg: arg, Holder: l.Holder, Err: err}
}

// hasText reports whether one of the Entries is arg's text (isText).
func (l List) hasText(arg string) bool {
	return slices.ContainsFunc(l.Entries, isText(arg))
}

// namesID returns a test of whether an entry names the node with id, by id or by an Alias, through
// names.
func namesID(names NamesFunc, id string) func(string) bool {
	return func(entry string) bool {
		named, isFound := names(entry)
		return isFound && named == id
	}
}

// isText returns a test of whether an entry is arg's text: arg as given or as tree.NormalizeArg
// spells it.
func isText(arg string) func(string) bool {
	name := tree.NormalizeArg(arg)
	return func(entry string) bool { return entry == arg || entry == name }
}
