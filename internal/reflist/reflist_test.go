package reflist_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/reflist"
	"github.com/airiclenz/capcom/internal/tree"
)

// errIndexUnreadable stands for a resolver failure that refuses no argument: the index cannot be
// read.
var errIndexUnreadable = errors.New("index unreadable")

// The names of the fake store the tests resolve in: ids, the Aliases of alpha, a name two files
// claim, a name only an unusable file claims, and an argument whose lookup fails outright.
var (
	testIDs     = []string{"alpha", "beta", "build-pipeline", "build-plan", "gamma"}
	testAliases = map[string]string{"a1": "alpha", "old-alpha": "alpha"}
)

const (
	duplicateName = "dup"
	unusableName  = "broken"
	failingName   = "unreadable"
)

// resolveTest resolves arg in the fake store as tree.ResolveIn would: an exact id, then an Alias,
// then the one id it is a prefix of.
func resolveTest(arg string) (string, error) {
	name := tree.NormalizeArg(arg)
	switch name {
	case duplicateName:
		return "", &tree.Error{Path: "dup.md", Other: "x/dup.md", Err: tree.ErrDuplicate}
	case unusableName:
		return "", &tree.UnusableError{Arg: arg, Path: "broken.md"}
	case failingName:
		return "", errIndexUnreadable
	}
	if id, isFound := namesTest(name); isFound {
		return id, nil
	}
	var candidates []string
	for _, id := range testIDs {
		if strings.HasPrefix(id, name) {
			candidates = append(candidates, id)
		}
	}
	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("id %q %w", arg, tree.ErrNotFound)
	case 1:
		return candidates[0], nil
	default:
		return "", &tree.AmbiguousError{Arg: arg, Candidates: candidates}
	}
}

// namesTest returns the id an entry of the fake store names by exact id or Alias.
func namesTest(entry string) (string, bool) {
	if slices.Contains(testIDs, entry) {
		return entry, true
	}
	id, isFound := testAliases[entry]
	return id, isFound
}

// cycleTest closes a cycle for any edge into beta from gamma: gamma -> beta -> gamma.
func cycleTest(from string, to string) []string {
	if from == "gamma" && to == "beta" {
		return []string{"gamma", "beta", "gamma"}
	}
	return nil
}

// editOp is the edit a table row makes.
type editOp int

const (
	opAdd editOp = iota
	opRemove
)

func TestList_AddAndRemove(t *testing.T) {
	cases := []struct {
		name    string
		op      editOp
		holder  string
		entries []string
		// applyTo is the list Drop runs on; nil runs it on entries.
		applyTo   []string
		isNoCycle bool
		arg       string
		// wantID is Addition.ID or Removal.ID.
		wantID      string
		wantPresent bool
		wantDropped []string
		wantReason  reflist.Reason
		wantIs      error
		wantCycle   []string
	}{
		// Add: the argument resolves to its canonical id.
		{name: "add exact id", op: opAdd, holder: "gamma", arg: "alpha", wantID: "alpha"},
		{name: "add alias writes the id", op: opAdd, holder: "gamma", arg: "a1", wantID: "alpha"},
		{name: "add prefix writes the id", op: opAdd, holder: "beta", arg: "gam", wantID: "gamma"},
		{name: "add citation spelling", op: opAdd, holder: "gamma", arg: "Alpha", wantID: "alpha"},
		// Add: already present is names-based.
		{name: "add present by id", op: opAdd, holder: "gamma", entries: []string{"alpha"}, arg: "alpha", wantID: "alpha", wantPresent: true},
		{name: "add present by alias spelling", op: opAdd, holder: "gamma", entries: []string{"old-alpha"}, arg: "a1", wantID: "alpha", wantPresent: true},
		{name: "add text naming nothing is not present", op: opAdd, holder: "gamma", entries: []string{"gone"}, arg: "alpha", wantID: "alpha"},
		// Add: self, then present, then cycle.
		{name: "add self", op: opAdd, holder: "alpha", arg: "a1", wantReason: reflist.ReasonSelf},
		{name: "add self beats present", op: opAdd, holder: "alpha", entries: []string{"alpha"}, arg: "alpha", wantReason: reflist.ReasonSelf},
		{name: "add closing a cycle", op: opAdd, holder: "gamma", arg: "beta", wantReason: reflist.ReasonCycle, wantCycle: []string{"gamma", "beta", "gamma"}},
		{name: "add present inside a cycle", op: opAdd, holder: "gamma", entries: []string{"beta"}, arg: "beta", wantID: "beta", wantPresent: true},
		{name: "add without a cycle check", op: opAdd, holder: "gamma", arg: "beta", isNoCycle: true, wantID: "beta"},
		{name: "add with no holder", op: opAdd, entries: []string{"beta"}, arg: "alpha", isNoCycle: true, wantID: "alpha"},
		{name: "add with no holder present", op: opAdd, entries: []string{"beta"}, arg: "beta", isNoCycle: true, wantID: "beta", wantPresent: true},
		// Add: resolver refusals.
		{name: "add not found", op: opAdd, holder: "gamma", arg: "zzz", wantReason: reflist.ReasonNotFound, wantIs: tree.ErrNotFound},
		{name: "add unusable claim", op: opAdd, holder: "gamma", arg: unusableName, wantReason: reflist.ReasonNotFound, wantIs: tree.ErrNotFound},
		{name: "add ambiguous", op: opAdd, holder: "gamma", arg: "b", wantReason: reflist.ReasonAmbiguous, wantIs: tree.ErrAmbiguous},
		{name: "add duplicate", op: opAdd, holder: "gamma", arg: duplicateName, wantReason: reflist.ReasonDuplicate, wantIs: tree.ErrDuplicate},
		{name: "add resolver failure", op: opAdd, holder: "gamma", arg: failingName, wantIs: errIndexUnreadable},
		// Remove: an exact id or alias drops every entry naming the node, and beats text.
		{name: "remove exact id drops every spelling", op: opRemove, holder: "gamma", entries: []string{"alpha", "beta", "a1", "old-alpha"}, arg: "alpha", wantID: "alpha", wantDropped: []string{"alpha", "a1", "old-alpha"}},
		{name: "remove exact alias", op: opRemove, holder: "gamma", entries: []string{"beta", "alpha"}, arg: "a1", wantID: "alpha", wantDropped: []string{"alpha"}},
		{name: "remove exact beats text", op: opRemove, holder: "gamma", entries: []string{"Alpha", "alpha"}, arg: "Alpha", wantID: "alpha", wantDropped: []string{"alpha"}},
		{name: "remove self entry", op: opRemove, holder: "alpha", entries: []string{"alpha", "beta"}, arg: "alpha", wantID: "alpha", wantDropped: []string{"alpha"}},
		// Remove: text beats a prefix.
		{name: "remove text beats prefix", op: opRemove, holder: "gamma", entries: []string{"bet", "beta"}, arg: "bet", wantDropped: []string{"bet"}},
		{name: "remove prefix without text", op: opRemove, holder: "gamma", entries: []string{"beta", "alpha"}, arg: "bet", wantID: "beta", wantDropped: []string{"beta"}},
		{name: "remove not found by text", op: opRemove, holder: "gamma", entries: []string{"gone-issue"}, arg: "Gone Issue", wantDropped: []string{"gone-issue"}},
		{name: "remove not found by text as given", op: opRemove, holder: "gamma", entries: []string{"Gone Issue"}, arg: "Gone Issue", wantDropped: []string{"Gone Issue"}},
		{name: "remove unusable claim by text", op: opRemove, holder: "gamma", entries: []string{unusableName}, arg: unusableName, wantDropped: []string{unusableName}},
		{name: "remove ambiguous by text", op: opRemove, holder: "gamma", entries: []string{"b", "beta"}, arg: "b", wantDropped: []string{"b"}},
		// Remove: refusals.
		{name: "remove ambiguous without text", op: opRemove, holder: "gamma", entries: []string{"beta"}, arg: "b", wantReason: reflist.ReasonAmbiguous, wantIs: tree.ErrAmbiguous},
		{name: "remove duplicate with text", op: opRemove, holder: "gamma", entries: []string{duplicateName}, arg: duplicateName, wantReason: reflist.ReasonDuplicate, wantIs: tree.ErrDuplicate},
		{name: "remove resolver failure", op: opRemove, holder: "gamma", arg: failingName, wantIs: errIndexUnreadable},
		{name: "remove matching no entry", op: opRemove, holder: "gamma", entries: []string{"beta"}, arg: "alpha", wantReason: reflist.ReasonNotFound},
		{name: "remove text matching no entry", op: opRemove, holder: "gamma", entries: []string{"beta"}, arg: "zzz", wantReason: reflist.ReasonNotFound},
		// Remove: the decision applies to the list as an edit reads it again.
		{name: "remove applied to a re-read list", op: opRemove, holder: "gamma", entries: []string{"beta"}, applyTo: []string{"a1", "beta", "alpha"}, arg: "alpha", wantID: "alpha", wantDropped: []string{"a1", "alpha"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			list := reflist.List{Holder: tc.holder, Entries: tc.entries, Resolve: resolveTest, Names: namesTest}
			if !tc.isNoCycle {
				list.FindCycle = cycleTest
			}

			gotID, gotPresent, gotDropped, err := runEdit(list, tc.op, tc.arg, tc.applyTo)

			assertRefusal(t, err, tc.wantReason, tc.wantIs, tc.wantCycle)
			if err != nil {
				return
			}
			if gotID != tc.wantID || gotPresent != tc.wantPresent || !slices.Equal(gotDropped, tc.wantDropped) {
				t.Errorf(
					"got id %q, present %v, dropped %q; want %q, %v, %q",
					gotID, gotPresent, gotDropped, tc.wantID, tc.wantPresent, tc.wantDropped,
				)
			}
		})
	}
}

// runEdit makes op's edit of arg on list: Add, or Remove and then Drop on applyTo, or on the list's
// entries when applyTo is nil.
func runEdit(list reflist.List, op editOp, arg string, applyTo []string) (string, bool, []string, error) {
	if op == opAdd {
		addition, err := list.Add(arg)
		return addition.ID, addition.IsPresent, nil, err
	}
	removal, err := list.Remove(arg)
	if err != nil {
		return "", false, nil, err
	}
	if applyTo == nil {
		applyTo = list.Entries
	}
	dropped, err := removal.Drop(applyTo)
	return removal.ID, false, dropped, err
}

// assertRefusal checks err against the refusal a row wants: none, a *reflist.Refusal of
// wantReason with wantCycle, and an error matching wantIs under errors.Is.
func assertRefusal(t *testing.T, err error, wantReason reflist.Reason, wantIs error, wantCycle []string) {
	t.Helper()
	if wantReason == 0 && wantIs == nil {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if wantIs != nil && !errors.Is(err, wantIs) {
		t.Errorf("error %v does not match %v", err, wantIs)
	}
	var refusal *reflist.Refusal
	isRefusal := errors.As(err, &refusal)
	if wantReason == 0 {
		if isRefusal {
			t.Errorf("error %v is a refusal; want the resolver's error unchanged", err)
		}
		return
	}
	if !isRefusal || refusal.Reason != wantReason {
		t.Fatalf("error %v is not a refusal of reason %d", err, wantReason)
	}
	if !slices.Equal(refusal.Cycle, wantCycle) {
		t.Errorf("cycle %q; want %q", refusal.Cycle, wantCycle)
	}
}

func TestRefusal_Error_KeepsTheResolversMessage(t *testing.T) {
	list := reflist.List{Holder: "gamma", Resolve: resolveTest, Names: namesTest}

	_, err := list.Add("b")

	want := `ambiguous id prefix "b": matches beta, build-pipeline, build-plan`
	if err == nil || err.Error() != want {
		t.Errorf("error %v; want %q", err, want)
	}
}
