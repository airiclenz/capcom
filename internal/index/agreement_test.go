package index

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/tree"
)

// fileForm is how a fixture file is written.
type fileForm int

const (
	// formValid parses and passes validation.
	formValid fileForm = iota
	// formInvalid parses but fails validation.
	formInvalid
	// formBroken does not parse: its frontmatter is broken YAML.
	formBroken
	// formSymlink is not a regular file: a symlink to /dev/zero.
	formSymlink
)

// agreementFile is one file of a membership fixture, at a slash path below the kind's tree.
type agreementFile struct {
	path    string
	form    fileForm
	aliases []string
}

// agreementQuery is one name asked of a fixture and the answers both sides give for it. resolve is
// the answer tree.Resolve and the index's resolve share unless the fixture's divergence table
// lists the name; claimant is tree.Claimant's answer.
type agreementQuery struct {
	name     string
	resolve  string
	claimant string
}

// agreementDivergence is a name tree and index answer differently today, each answer as
// agreementAnswer renders it, and the ratified call (plan header) that will reconcile them. A row
// goes when that call lands; a row whose answers agree fails the test.
type agreementDivergence struct {
	tree  string
	index string
	call  string
}

// agreementFixture is one membership fixture: the files it writes in each kind's tree, the names
// asked of it, and the names tree and index answer differently today.
type agreementFixture struct {
	name        string
	files       []agreementFile
	queries     []agreementQuery
	divergences map[string]agreementDivergence
}

// agreementKind is one node kind as the tree and the index both see it.
type agreementKind struct {
	name string
	dir  func(s *store.Store) string
	// valid and invalid are the frontmatter fields, aliases aside, of a file that passes
	// validation and of one that parses but fails it.
	valid   string
	invalid string
	// aliasFuncs return the ContentFunc commands load the tree with and the recovering AliasFunc
	// they hand Claimant.
	aliasFuncs func(s *store.Store) (read tree.ContentFunc, recover tree.AliasFunc)
	resolve    func(x *Index, arg string) (string, error)
}

// agreementKinds are the issue and the Note kind.
var agreementKinds = []agreementKind{
	{
		name:    "issue",
		dir:     (*store.Store).IssuesDir,
		valid:   "title: Fixture\nstatus: open\ntype: task\npriority: 2\ncreated: 2026-10-11T09:00:00Z\n",
		invalid: "title: Fixture\nstatus: bogus\ntype: task\npriority: 2\ncreated: 2026-10-11T09:00:00Z\n",
		aliasFuncs: func(s *store.Store) (tree.ContentFunc, tree.AliasFunc) {
			kind := nodestore.Issues(s, nil)
			return kind.ReadContent, kind.RecoverAliases
		},
		resolve: func(x *Index, arg string) (string, error) {
			entry, err := x.Resolve(arg)
			return entry.ID, err
		},
	},
	{
		name:    "note",
		dir:     (*store.Store).KnowledgeDir,
		valid:   "title: Fixture\n",
		invalid: "title: \"\"\n",
		aliasFuncs: func(s *store.Store) (tree.ContentFunc, tree.AliasFunc) {
			kind := nodestore.Notes(s, nil)
			return kind.ReadContent, kind.RecoverAliases
		},
		resolve: func(x *Index, arg string) (string, error) {
			entry, err := x.ResolveNote(arg)
			return entry.ID, err
		},
	},
}

// content is the text of file in the kind's tree; a formSymlink file has none.
func (k agreementKind) content(file agreementFile) string {
	fields := k.valid
	switch file.form {
	case formInvalid:
		fields = k.invalid
	case formBroken:
		fields = "title: [unclosed\n"
	}
	if len(file.aliases) > 0 {
		fields += "aliases: [" + strings.Join(file.aliases, ", ") + "]\n"
	}
	return "---\n" + fields + "---\nBody.\n"
}

// write writes every file of fixture into the kind's tree of s.
func (k agreementKind) write(t *testing.T, s *store.Store, fixture agreementFixture) {
	t.Helper()
	for _, file := range fixture.files {
		path := filepath.Join(k.dir(s), filepath.FromSlash(file.path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if file.form == formSymlink {
			makeSymlink(t, "/dev/zero", path)
			continue
		}
		if err := os.WriteFile(path, []byte(k.content(file)), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
}

// agreementAnswer renders a resolve's outcome: the canonical id it found, or the sentinel its
// error matches, with the candidates of an ambiguous prefix.
func agreementAnswer(id string, err error) string {
	var ambiguous *tree.AmbiguousError
	switch {
	case err == nil:
		return "id " + id
	case errors.As(err, &ambiguous):
		return "ambiguous " + strings.Join(ambiguous.Candidates, ",")
	case errors.Is(err, tree.ErrDuplicate):
		return "duplicate"
	case errors.Is(err, tree.ErrNotFound):
		return "not found"
	default:
		return "error " + err.Error()
	}
}

// claimKindNames name each tree.ClaimKind in a claimant answer.
var claimKindNames = map[tree.ClaimKind]string{
	tree.LiveClaim:      "live",
	tree.OrphanClaim:    "orphan",
	tree.DuplicateClaim: "duplicate",
	tree.UnparsedClaim:  "unparsed",
}

// claimantAnswer renders tree.Claimant's outcome: the claim's kind, its file as a slash path
// below dir, and "alias" when the file holds the name as an alias; "none" when no file holds it.
func claimantAnswer(t *testing.T, dir string, claim tree.Claim, isFound bool, err error) string {
	t.Helper()
	if err != nil {
		return "error " + err.Error()
	}
	if !isFound {
		return "none"
	}
	relative, err := filepath.Rel(dir, claim.Path)
	if err != nil {
		t.Fatalf("Rel(%s): %v", claim.Path, err)
	}
	answer := claimKindNames[claim.Kind] + " " + filepath.ToSlash(relative)
	if claim.IsAlias {
		answer += " alias"
	}
	return answer
}

// loadAgreementTree loads the kind's tree of s as commands do, keeping the tree LoadContent
// returns beside the problems it reports.
func loadAgreementTree(t *testing.T, dir string, content tree.ContentFunc) *tree.Tree {
	t.Helper()
	loaded, err := tree.LoadContent(dir, content)
	var problems tree.Errors
	if err != nil && !errors.As(err, &problems) {
		t.Fatalf("LoadContent: %v", err)
	}
	return loaded
}

// The ratified calls (plan 2026-10-14 - 00, header) behind the divergences below.
const (
	// callUnparsedRemoval: the one exception to "Unusable claims": the exact id of an unparsed live
	// file still resolves in the tree, so rm and know rm can remove it. This row stays.
	callUnparsedRemoval = "Unusable claims, the rm exception"
)

// agreementFixtures are the membership fixtures. Every path is below the kind's tree, and the
// issue and the Note kind answer every name alike.
var agreementFixtures = []agreementFixture{
	{
		name: "repeated id with subtree",
		files: []agreementFile{
			{path: "a.md"}, {path: "b.md"},
			{path: "a/dup.md"}, {path: "b/dup.md"}, {path: "a/dup/kid.md", aliases: []string{"kid-alias"}},
		},
		queries: []agreementQuery{
			{name: "dup", resolve: "duplicate", claimant: "duplicate a/dup.md"},
			{name: "du", resolve: "duplicate", claimant: "none"},
			{name: "kid", resolve: "duplicate", claimant: "orphan a/dup/kid.md"},
			{name: "ki", resolve: "duplicate", claimant: "none"},
			{name: "kid-alias", resolve: "duplicate", claimant: "orphan a/dup/kid.md alias"},
			{name: "a", resolve: "id a", claimant: "live a.md"},
		},
	},
	{
		name: "alias clash between two files",
		files: []agreementFile{
			{path: "one.md", aliases: []string{"shared"}}, {path: "two.md", aliases: []string{"shared"}},
		},
		queries: []agreementQuery{
			{name: "shared", resolve: "duplicate", claimant: "live one.md alias"},
			{name: "sha", resolve: "not found", claimant: "none"},
			{name: "one", resolve: "id one", claimant: "live one.md"},
			{name: "two", resolve: "id two", claimant: "live two.md"},
		},
	},
	{
		name: "unparsed live file claims an alias",
		files: []agreementFile{
			{path: "zzz.md", form: formBroken, aliases: []string{"foo"}}, {path: "foo-bar.md"},
		},
		queries: []agreementQuery{
			{name: "foo", resolve: "not found", claimant: "unparsed zzz.md alias"},
			{name: "fo", resolve: "id foo-bar", claimant: "none"},
			{name: "zzz", claimant: "live zzz.md"},
			{name: "zz", resolve: "not found", claimant: "none"},
			{name: "foo-bar", resolve: "id foo-bar", claimant: "live foo-bar.md"},
		},
		divergences: map[string]agreementDivergence{
			"zzz": {tree: "id zzz", index: "not found", call: callUnparsedRemoval},
		},
	},
	{
		name: "unparsed file in an orphan directory",
		files: []agreementFile{
			{path: "ghost/zzz.md", form: formBroken, aliases: []string{"foo"}}, {path: "foo-bar.md"},
		},
		queries: []agreementQuery{
			{name: "foo", resolve: "not found", claimant: "unparsed ghost/zzz.md alias"},
			{name: "fo", resolve: "id foo-bar", claimant: "none"},
			{name: "zzz", resolve: "not found", claimant: "orphan ghost/zzz.md"},
			{name: "zz", resolve: "not found", claimant: "none"},
			{name: "ghost", resolve: "not found", claimant: "none"},
		},
	},
	{
		name: "file that parses but fails validation",
		files: []agreementFile{
			{path: "bad.md", form: formInvalid, aliases: []string{"worse"}}, {path: "bad-luck.md"},
		},
		queries: []agreementQuery{
			{name: "bad", resolve: "not found", claimant: "live bad.md"},
			{name: "ba", resolve: "ambiguous bad,bad-luck", claimant: "none"},
			{name: "worse", resolve: "not found", claimant: "live bad.md alias"},
			{name: "bad-luck", resolve: "id bad-luck", claimant: "live bad-luck.md"},
		},
	},
	{
		name: "alias listed only by the later file of a repeated id",
		files: []agreementFile{
			{path: "a.md"}, {path: "b.md"},
			{path: "a/dup.md"}, {path: "b/dup.md", aliases: []string{"later"}},
		},
		queries: []agreementQuery{
			{name: "later", resolve: "not found", claimant: "duplicate b/dup.md alias"},
			{name: "lat", resolve: "not found", claimant: "none"},
			{name: "dup", resolve: "duplicate", claimant: "duplicate a/dup.md"},
		},
	},
	{
		name: "prefix over a repeated id and a usable id",
		files: []agreementFile{
			{path: "a.md"}, {path: "b.md"},
			{path: "a/dup-x.md"}, {path: "b/dup-x.md"}, {path: "dup-y.md"},
		},
		queries: []agreementQuery{
			{name: "dup", resolve: "ambiguous dup-x,dup-y", claimant: "none"},
			{name: "dup-", resolve: "ambiguous dup-x,dup-y", claimant: "none"},
			{name: "dup-x", resolve: "duplicate", claimant: "duplicate a/dup-x.md"},
			{name: "dup-y", resolve: "id dup-y", claimant: "live dup-y.md"},
		},
	},
	{
		name: "id an unusable file also claims",
		files: []agreementFile{
			{path: "foo.md"}, {path: "bad.md", form: formInvalid, aliases: []string{"foo"}},
			{path: "bar.md"}, {path: "ghost/bar.md"},
			{path: "baz.md"}, {path: "zzz.md", form: formBroken, aliases: []string{"baz"}},
		},
		queries: []agreementQuery{
			{name: "foo", resolve: "duplicate", claimant: "live foo.md"},
			{name: "bar", resolve: "duplicate", claimant: "live bar.md"},
			{name: "baz", resolve: "duplicate", claimant: "live baz.md"},
			{name: "fo", resolve: "id foo", claimant: "none"},
		},
	},
	{
		name: "non-regular file",
		files: []agreementFile{
			{path: "zero.md", form: formSymlink}, {path: "zero-day.md"},
		},
		queries: []agreementQuery{
			{name: "zero", resolve: "not found", claimant: "none"},
			{name: "zer", resolve: "id zero-day", claimant: "none"},
			{name: "zero-day", resolve: "id zero-day", claimant: "live zero-day.md"},
		},
	},
}

func TestAgreement_MembershipFixtures_TreeAndIndexAnswerAlikeOrAsListed(t *testing.T) {
	t.Parallel()
	for _, fixture := range agreementFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			for _, kind := range agreementKinds {
				t.Run(kind.name, func(t *testing.T) {
					s := newStore(t)
					kind.write(t, s, fixture)
					read, recover := kind.aliasFuncs(s)
					loaded := loadAgreementTree(t, kind.dir(s), read)
					x := openIndex(t, s)
					refresh(t, x)

					for _, query := range fixture.queries {
						node, treeErr := loaded.Resolve(query.name)
						treeAnswer := agreementAnswer(node.ID, treeErr)
						indexAnswer := agreementAnswer(kind.resolve(x, query.name))
						claim, isFound, claimErr := loaded.Claimant(tree.NormalizeArg(query.name), "", recover)
						claimant := claimantAnswer(t, kind.dir(s), claim, isFound, claimErr)

						wantTree, wantIndex := query.resolve, query.resolve
						divergence, isDivergent := fixture.divergences[query.name]
						if isDivergent {
							wantTree, wantIndex = divergence.tree, divergence.index
						}
						if treeAnswer != wantTree || indexAnswer != wantIndex {
							t.Errorf("%q: tree.Resolve = %q, index = %q; want %q, %q",
								query.name, treeAnswer, indexAnswer, wantTree, wantIndex)
						}
						if isDivergent && divergence.tree == divergence.index {
							t.Errorf("%q: divergence row answers alike (%q); drop it once %s lands",
								query.name, divergence.tree, divergence.call)
						}
						if claimant != query.claimant {
							t.Errorf("%q: tree.Claimant = %q, want %q", query.name, claimant, query.claimant)
						}
					}
				})
			}
		})
	}
}
