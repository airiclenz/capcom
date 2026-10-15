package tree_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/tree"
)

// loadAllowingProblems loads dir, failing the test unless Load returns a tree with no error or
// with tree.Errors.
func loadAllowingProblems(t *testing.T, dir string, aliases tree.AliasFunc) *tree.Tree {
	t.Helper()
	loaded, err := tree.Load(dir, "issue", aliases)
	var problems tree.Errors
	if loaded == nil || (err != nil && !errors.As(err, &problems)) {
		t.Fatalf("Load = %v, %v; want a tree", loaded, err)
	}
	return loaded
}

// claimantCase is one Claimant call on a tree of files, each "x\n", and its expected Claim.
type claimantCase struct {
	name      string
	files     []string
	aliases   map[string][]string
	recovered map[string][]string
	claimed   string
	except    string
	want      tree.Claim
	wantFound bool
}

func TestClaimant(t *testing.T) {
	t.Parallel()
	cases := []claimantCase{
		{
			name:      "live node by its id",
			files:     []string{"auth-flow.md"},
			claimed:   "auth-flow",
			want:      tree.Claim{Path: "auth-flow.md", Holder: "auth-flow", Kind: tree.LiveClaim},
			wantFound: true,
		},
		{
			name:      "live node by an alias",
			files:     []string{"auth-flow.md"},
			aliases:   map[string][]string{"auth-flow.md": {"login"}},
			claimed:   "login",
			want:      tree.Claim{Path: "auth-flow.md", Holder: "auth-flow", IsAlias: true, Kind: tree.LiveClaim},
			wantFound: true,
		},
		{
			name:    "orphan by its file name",
			files:   []string{"stray/auth-flow.md"},
			claimed: "auth-flow",
			want: tree.Claim{
				Path: "stray/auth-flow.md", Holder: "auth-flow", Kind: tree.OrphanClaim, Dir: "stray",
			},
			wantFound: true,
		},
		{
			name:    "orphan by an alias",
			files:   []string{"stray/auth-flow.md"},
			aliases: map[string][]string{"stray/auth-flow.md": {"login"}},
			claimed: "login",
			want: tree.Claim{
				Path: "stray/auth-flow.md", Holder: "auth-flow", IsAlias: true, Kind: tree.OrphanClaim, Dir: "stray",
			},
			wantFound: true,
		},
		{
			name:    "nested orphan names the reported orphan directory",
			files:   []string{"stray/sub.md", "stray/sub/auth-flow.md"},
			claimed: "auth-flow",
			want: tree.Claim{
				Path: "stray/sub/auth-flow.md", Holder: "auth-flow", Kind: tree.OrphanClaim, Dir: "stray",
			},
			wantFound: true,
		},
		{
			name:    "file below a repeated id's directory is an orphan of that directory",
			files:   []string{"a.md", "a/dup.md", "a/dup/child.md", "a/dup/lost/deep.md", "b.md", "b/dup.md"},
			claimed: "deep",
			want: tree.Claim{
				Path: "a/dup/lost/deep.md", Holder: "deep", Kind: tree.OrphanClaim, Dir: "a/dup",
			},
			wantFound: true,
		},
		{
			name:      "duplicate by its file name",
			files:     []string{"a.md", "a/dup.md", "b.md", "b/dup.md"},
			claimed:   "dup",
			want:      tree.Claim{Path: "a/dup.md", Holder: "dup", Kind: tree.DuplicateClaim},
			wantFound: true,
		},
		{
			name:      "duplicate by an alias",
			files:     []string{"a.md", "a/dup.md", "b.md", "b/dup.md"},
			aliases:   map[string][]string{"b/dup.md": {"older"}},
			claimed:   "older",
			want:      tree.Claim{Path: "b/dup.md", Holder: "dup", IsAlias: true, Kind: tree.DuplicateClaim},
			wantFound: true,
		},
		{
			name:      "unparsed live node",
			files:     []string{"auth-flow.md"},
			recovered: map[string][]string{"auth-flow.md": {"login"}},
			claimed:   "login",
			want:      tree.Claim{Path: "auth-flow.md", Holder: "auth-flow", IsAlias: true, Kind: tree.UnparsedClaim},
			wantFound: true,
		},
		{
			name:      "unparsed orphan",
			files:     []string{"stray/auth-flow.md"},
			recovered: map[string][]string{"stray/auth-flow.md": {"login"}},
			claimed:   "login",
			want: tree.Claim{
				Path: "stray/auth-flow.md", Holder: "auth-flow", IsAlias: true, Kind: tree.UnparsedClaim,
			},
			wantFound: true,
		},
		{
			name:      "unparsed duplicate",
			files:     []string{"a.md", "a/dup.md", "b.md", "b/dup.md"},
			recovered: map[string][]string{"b/dup.md": {"login"}},
			claimed:   "login",
			want:      tree.Claim{Path: "b/dup.md", Holder: "dup", IsAlias: true, Kind: tree.UnparsedClaim},
			wantFound: true,
		},
		{
			name:      "live before orphan",
			files:     []string{"auth-flow.md", "stray/auth-flow.md"},
			claimed:   "auth-flow",
			want:      tree.Claim{Path: "auth-flow.md", Holder: "auth-flow", Kind: tree.LiveClaim},
			wantFound: true,
		},
		{
			name:    "orphan before duplicate",
			files:   []string{"a.md", "a/dup.md", "b.md", "b/dup.md", "stray/dup.md"},
			claimed: "dup",
			want: tree.Claim{
				Path: "stray/dup.md", Holder: "dup", Kind: tree.OrphanClaim, Dir: "stray",
			},
			wantFound: true,
		},
		{
			name:      "duplicate before unparsed",
			files:     []string{"a.md", "a/dup.md", "b.md", "b/dup.md"},
			aliases:   map[string][]string{"a/dup.md": {"login"}},
			recovered: map[string][]string{"a.md": {"login"}},
			claimed:   "login",
			want:      tree.Claim{Path: "a/dup.md", Holder: "dup", IsAlias: true, Kind: tree.DuplicateClaim},
			wantFound: true,
		},
		{
			name:    "except skips the moved node's own alias",
			files:   []string{"auth-flow.md"},
			aliases: map[string][]string{"auth-flow.md": {"login"}},
			claimed: "login",
			except:  "auth-flow.md",
		},
		{
			name:    "except skips only the moved node's file",
			files:   []string{"auth-flow.md", "stray/other.md"},
			aliases: map[string][]string{"auth-flow.md": {"login"}, "stray/other.md": {"login"}},
			claimed: "login",
			except:  "auth-flow.md",
			want: tree.Claim{
				Path: "stray/other.md", Holder: "other", IsAlias: true, Kind: tree.OrphanClaim, Dir: "stray",
			},
			wantFound: true,
		},
		{
			name:    "orphan with the excepted id still claims",
			files:   []string{"auth-flow.md", "stray/auth-flow.md"},
			claimed: "auth-flow",
			except:  "auth-flow.md",
			want: tree.Claim{
				Path: "stray/auth-flow.md", Holder: "auth-flow", Kind: tree.OrphanClaim, Dir: "stray",
			},
			wantFound: true,
		},
		{
			name:      "unparsed file with the excepted id still claims",
			files:     []string{"auth-flow.md", "stray/auth-flow.md"},
			recovered: map[string][]string{"stray/auth-flow.md": {"login"}},
			claimed:   "login",
			except:    "auth-flow.md",
			want: tree.Claim{
				Path: "stray/auth-flow.md", Holder: "auth-flow", IsAlias: true, Kind: tree.UnparsedClaim,
			},
			wantFound: true,
		},
		{
			name:    "free name",
			files:   []string{"auth-flow.md", "stray/other.md", "a.md", "a/dup.md", "b.md", "b/dup.md"},
			aliases: map[string][]string{"auth-flow.md": {"login"}},
			claimed: "free",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeNodes(t, dir, tc.files...)
			loaded := loadAllowingProblems(t, dir, aliasesByPath(dir, tc.aliases))
			except := ""
			if tc.except != "" {
				except = filepath.Join(dir, filepath.FromSlash(tc.except))
			}

			got, found, err := loaded.Claimant(tc.claimed, except, aliasesByPath(dir, tc.recovered))

			if err != nil {
				t.Fatalf("Claimant(%q) error = %v", tc.claimed, err)
			}
			want := tc.want
			if want.Path != "" {
				want.Path = filepath.Join(dir, filepath.FromSlash(want.Path))
			}
			if want.Dir != "" {
				want.Dir = filepath.Join(dir, filepath.FromSlash(want.Dir))
			}
			if found != tc.wantFound || got != want {
				t.Errorf("Claimant(%q) = %+v, %t; want %+v, %t", tc.claimed, got, found, want, tc.wantFound)
			}
		})
	}
}

func TestClaimant_NilRecover_SkipsUnparsed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "auth-flow.md")
	loaded := mustLoad(t, dir, nil)

	_, found, err := loaded.Claimant("login", "", nil)

	if err != nil || found {
		t.Errorf("Claimant(login) found = %t, err = %v; want none", found, err)
	}
}

func TestClaimant_AliasFuncError_ReturnedUnchanged(t *testing.T) {
	t.Parallel()
	failure := errors.New("cannot read aliases")
	failOnPath := func(failing string) tree.AliasFunc {
		return func(path string) ([]string, error) {
			if path == failing {
				return nil, failure
			}
			return nil, nil
		}
	}
	dir := t.TempDir()
	writeNodes(t, dir, "auth-flow.md", "stray/other.md")
	orphan := filepath.Join(dir, "stray", "other.md")

	t.Run("Load's AliasFunc on an orphan", func(t *testing.T) {
		t.Parallel()
		loaded := loadAllowingProblems(t, dir, failOnPath(orphan))

		_, found, err := loaded.Claimant("login", "", nil)

		if err != failure || found {
			t.Errorf("Claimant(login) found = %t, err = %v; want %v unchanged", found, err, failure)
		}
	})
	t.Run("the recovering AliasFunc", func(t *testing.T) {
		t.Parallel()
		loaded := loadAllowingProblems(t, dir, nil)

		_, found, err := loaded.Claimant("login", "", failOnPath(filepath.Join(dir, "auth-flow.md")))

		if err != failure || found {
			t.Errorf("Claimant(login) found = %t, err = %v; want %v unchanged", found, err, failure)
		}
	})
}

func TestClaimant_HiddenAndNonNodeEntriesInOrphan_NeverClaim(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "stray/.target.md", "stray/.hidden/target.md", "stray/target.txt", "stray/target")
	everyFileHolds := func(string) ([]string, error) { return []string{"target"}, nil }
	loaded := loadAllowingProblems(t, dir, everyFileHolds)

	claim, found, err := loaded.Claimant("target", "", everyFileHolds)

	if err != nil || found {
		t.Errorf("Claimant(target) = %+v, %t, %v; want none", claim, found, err)
	}
}

func TestClaimant_NonRegularFileInOrphan_ClaimsItsIDNeverAnAlias(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "target.md")
	writeSizedFile(t, outside, 2)
	if err := os.MkdirAll(filepath.Join(dir, "stray"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	makeSymlink(t, outside, filepath.Join(dir, "stray", "target.md"))
	makeFIFO(t, filepath.Join(dir, "stray", "pipe.md"))
	everyFileHolds := func(string) ([]string, error) { return []string{"held"}, nil }
	loaded := loadAllowingProblems(t, dir, everyFileHolds)

	for _, id := range []string{"target", "pipe"} {
		claim, found, err := loaded.Claimant(id, "", everyFileHolds)

		want := tree.Claim{
			Path:   filepath.Join(dir, "stray", id+".md"),
			Holder: id,
			Kind:   tree.OrphanClaim,
			Dir:    filepath.Join(dir, "stray"),
		}
		if err != nil || !found || claim != want {
			t.Errorf("Claimant(%s) = %+v, %t, %v; want %+v", id, claim, found, err, want)
		}
	}
	claim, found, err := loaded.Claimant("held", "", everyFileHolds)

	if err != nil || found {
		t.Errorf("Claimant(held) = %+v, %t, %v; want none", claim, found, err)
	}
}

func TestClaimOnlyFacts_NestedOrphanDirsOutOfOrder_ListEachFileOnceAndKeepFailedReadsID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{"a/dup/kid.md", "a/dup/deep/far.md", "a/dup-x/near.md", "a/dup/Bad_ID.md"} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	outer := filepath.Join(dir, "a", "dup")
	inner := filepath.Join(outer, "deep")
	sibling := filepath.Join(dir, "a", "dup-x")
	link := filepath.Join(dir, "link.md")
	failing := filepath.Join(outer, "kid.md")
	problems := tree.Errors{
		tree.DuplicateIDDirectoryError(inner, "deep"),
		tree.DuplicateIDDirectoryError(sibling, "dup-x"),
		tree.DuplicateIDDirectoryError(outer, "dup"),
		{Path: link, Err: tree.ErrNotRegular},
	}
	content := func(path string) (tree.Content, error) {
		if path == failing {
			return tree.Content{}, errors.New("read failed")
		}
		return tree.Content{Aliases: []string{tree.IDOf(path) + "-alias"}, Parse: tree.ParsedFile, IsUsable: true}, nil
	}

	facts := tree.ClaimOnlyFacts(problems, "issue", content)

	type claimed struct {
		key     string
		id      string
		aliases string
	}
	var got []claimed
	for _, fact := range facts {
		if fact.Origin != tree.ClaimOnlyOrigin {
			t.Errorf("%s: Origin = %v, want ClaimOnlyOrigin", fact.Key, fact.Origin)
		}
		got = append(got, claimed{fact.Key, fact.ID, strings.Join(fact.Aliases, ",")})
	}
	want := []claimed{
		{link, "link", ""},
		{failing, "kid", ""},
		{filepath.Join(inner, "far.md"), "far", "far-alias"},
		{filepath.Join(sibling, "near.md"), "near", "near-alias"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("ClaimOnlyFacts = %+v\nwant %+v", got, want)
	}
}
