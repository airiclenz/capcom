package tree_test

import (
	"fmt"
	"path"
	"slices"
	"testing"

	"github.com/airiclenz/capcom/internal/tree"
)

// factOption changes one field of a test FileFact.
type factOption func(*tree.FileFact)

// withAliases makes the file claim aliases.
func withAliases(aliases ...string) factOption {
	return func(fact *tree.FileFact) { fact.Aliases = aliases }
}

// unparsed makes the file one that does not parse.
func unparsed(fact *tree.FileFact) { fact.Parse = tree.UnparsedFile }

// unusable makes the file one that fails its kind's validation.
func unusable(fact *tree.FileFact) { fact.IsUsable = false }

// claimOnly makes the file one the walk left out, unread.
func claimOnly(fact *tree.FileFact) {
	fact.Origin = tree.ClaimOnlyOrigin
	fact.Parse = tree.UnreadFile
}

// fileFact returns the fact of a usable, parsed walk file at the slash path key, its id, parent and
// directory taken from key.
func fileFact(key string, options ...factOption) tree.FileFact {
	dir := path.Dir(key)
	parentID := ""
	if dir != "." {
		parentID = path.Base(dir)
	}
	fact := tree.FileFact{
		Key:      key,
		ID:       tree.IDOf(key),
		ParentID: parentID,
		Dir:      dir,
		Parse:    tree.ParsedFile,
		IsUsable: true,
		Origin:   tree.WalkOrigin,
	}
	for _, option := range options {
		option(&fact)
	}
	return fact
}

// membershipCase is one set of facts and the verdict DecideMembership must reach on it.
type membershipCase struct {
	name    string
	facts   []tree.FileFact
	kept    []string
	leftOut []string
	// refusals maps a name to its refusal as renderRefusal renders it; "" for a name it must not
	// refuse.
	refusals map[string]string
}

// renderRefusal renders the refusal of a verdict: its reason, the reported key and, after "<", the
// other key; "" when nothing refuses the name.
func renderRefusal(verdict tree.NameVerdict) string {
	switch verdict.Refusal {
	case tree.DuplicateRefusal:
		return fmt.Sprintf("duplicate %s<%s", verdict.Conflict.Key, verdict.Conflict.Other)
	case tree.UnusableRefusal:
		return "unusable " + verdict.Conflict.Key
	default:
		return ""
	}
}

// renderLeftOut renders a left-out file as its key and reason.
func renderLeftOut(facts []tree.FileFact, file tree.LeftOutFile) string {
	reason := "below"
	if file.Reason == tree.RepeatedIDFile {
		reason = "repeated"
	}
	return facts[file.Fact].Key + " " + reason
}

func TestDecideMembership_FixtureShapes_DecideNodesLeftOutAndRefusals(t *testing.T) {
	t.Parallel()
	cases := []membershipCase{
		{
			name: "repeated id with a subtree",
			facts: []tree.FileFact{
				fileFact("solo.md"),
				fileFact("b/dup.md"),
				fileFact("a.md"),
				fileFact("b.md"),
				fileFact("a/dup.md"),
				fileFact("a/dup/child.md", withAliases("kid")),
			},
			kept:    []string{"a.md", "b.md", "solo.md"},
			leftOut: []string{"a/dup.md repeated", "a/dup/child.md below", "b/dup.md repeated"},
			refusals: map[string]string{
				"dup":   "duplicate b/dup.md<a/dup.md",
				"child": "duplicate b/dup.md<a/dup.md",
				"kid":   "duplicate b/dup.md<a/dup.md",
				"solo":  "",
			},
		},
		{
			name: "alias clash between two files",
			facts: []tree.FileFact{
				fileFact("one.md", withAliases("foo")),
				fileFact("two.md", withAliases("foo")),
			},
			kept:     []string{"one.md", "two.md"},
			refusals: map[string]string{"foo": "duplicate two.md<one.md", "one": "", "two": ""},
		},
		{
			name: "alias that is another node's id",
			facts: []tree.FileFact{
				fileFact("one.md", withAliases("two")),
				fileFact("two.md"),
			},
			kept:     []string{"one.md", "two.md"},
			refusals: map[string]string{"two": "duplicate one.md<two.md", "one": ""},
		},
		{
			name: "unparsed live file claiming an alias",
			facts: []tree.FileFact{
				fileFact("zzz.md", unparsed, withAliases("foo")),
				fileFact("foo-bar.md"),
			},
			kept:     []string{"foo-bar.md", "zzz.md"},
			refusals: map[string]string{"foo": "unusable zzz.md", "zzz": "unusable zzz.md", "foo-bar": ""},
		},
		{
			name: "claim-only file under an orphan directory",
			facts: []tree.FileFact{
				fileFact("orphan/zzz.md", claimOnly, withAliases("foo")),
				fileFact("orphan/foo-bar.md", claimOnly),
				fileFact("foo-bar.md"),
			},
			kept: []string{"foo-bar.md"},
			refusals: map[string]string{
				"foo":     "unusable orphan/zzz.md",
				"zzz":     "unusable orphan/zzz.md",
				"foo-bar": "duplicate orphan/foo-bar.md<foo-bar.md",
			},
		},
		{
			name: "file that fails validation",
			facts: []tree.FileFact{
				fileFact("bad.md", unusable, withAliases("foo")),
				fileFact("good.md"),
			},
			kept:     []string{"bad.md", "good.md"},
			refusals: map[string]string{"foo": "unusable bad.md", "bad": "unusable bad.md", "good": ""},
		},
		{
			name: "alias only the later file of a repeated id lists",
			facts: []tree.FileFact{
				fileFact("a.md"),
				fileFact("b.md"),
				fileFact("a/dup.md", withAliases("first")),
				fileFact("b/dup.md", withAliases("later")),
			},
			kept:     []string{"a.md", "b.md"},
			leftOut:  []string{"a/dup.md repeated", "b/dup.md repeated"},
			refusals: map[string]string{"first": "duplicate b/dup.md<a/dup.md", "later": ""},
		},
		{
			name: "prefix over a repeated id",
			facts: []tree.FileFact{
				fileFact("a.md"),
				fileFact("b.md"),
				fileFact("a/dup-x.md"),
				fileFact("b/dup-x.md"),
				fileFact("dup-y.md"),
			},
			kept:     []string{"a.md", "b.md", "dup-y.md"},
			leftOut:  []string{"a/dup-x.md repeated", "b/dup-x.md repeated"},
			refusals: map[string]string{"dup-x": "duplicate b/dup-x.md<a/dup-x.md", "dup-y": ""},
		},
		{
			name: "file the walk refused to read",
			facts: []tree.FileFact{
				fileFact("link.md", claimOnly),
				fileFact("solo.md"),
			},
			kept:     []string{"solo.md"},
			refusals: map[string]string{"link": "unusable link.md", "solo": ""},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			membership := tree.DecideMembership(testCase.facts)

			var kept []string
			for _, i := range membership.Kept {
				kept = append(kept, testCase.facts[i].Key)
			}
			slices.Sort(kept)
			if !slices.Equal(kept, testCase.kept) {
				t.Errorf("Kept = %q, want %q", kept, testCase.kept)
			}
			var leftOut []string
			for _, file := range membership.LeftOut {
				leftOut = append(leftOut, renderLeftOut(testCase.facts, file))
			}
			slices.Sort(leftOut)
			if !slices.Equal(leftOut, testCase.leftOut) {
				t.Errorf("LeftOut = %q, want %q", leftOut, testCase.leftOut)
			}
			for name, want := range testCase.refusals {
				if got := renderRefusal(membership.Names[name]); got != want {
					t.Errorf("refusal of %q = %q, want %q", name, got, want)
				}
			}
		})
	}
}

func TestDecideMembership_ClaimOnlyFile_ClaimsButNeverLeavesAnotherOut(t *testing.T) {
	t.Parallel()
	facts := []tree.FileFact{
		fileFact("node.md"),
		fileFact("orphan/node.md", claimOnly),
		fileFact("node/child.md"),
	}

	membership := tree.DecideMembership(facts)

	if len(membership.Kept) != 2 || len(membership.LeftOut) != 0 || len(membership.IDConflicts) != 0 {
		t.Fatalf("Kept = %v, LeftOut = %v, IDConflicts = %v; want node and its child kept, no conflict",
			membership.Kept, membership.LeftOut, membership.IDConflicts)
	}
	claims := membership.Names["node"].Claims
	if len(claims) != 2 || claims[0].Fact != 0 || claims[1].Fact != 1 {
		t.Errorf("claims of node = %v, want node.md then orphan/node.md", claims)
	}
}

func TestDecideMembership_AliasConflicts_NameHowTheOtherFileClaims(t *testing.T) {
	t.Parallel()
	facts := []tree.FileFact{
		fileFact("one.md", withAliases("two", "shared")),
		fileFact("two.md", withAliases("shared")),
	}

	membership := tree.DecideMembership(facts)

	want := []tree.Conflict{
		{Key: "one.md", Other: "two.md", Name: "two", IsAlias: true},
		{Key: "two.md", Other: "one.md", Name: "shared", IsAlias: true, IsOtherAlias: true},
	}
	if !slices.Equal(membership.AliasConflicts, want) {
		t.Errorf("AliasConflicts = %+v, want %+v", membership.AliasConflicts, want)
	}
	if owner, isMapped := membership.Aliases["shared"]; !isMapped || owner != 0 {
		t.Errorf("Aliases[shared] = %d, %t; want one.md's fact", owner, isMapped)
	}
}
