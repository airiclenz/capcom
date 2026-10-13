package tree_test

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/airiclenz/capcom/internal/tree"
)

// resolveFixture loads a tree with nested nodes and legacy aliases.
func resolveFixture(t *testing.T) *tree.Tree {
	t.Helper()
	dir := t.TempDir()
	writeNodes(t, dir,
		"login-flow.md",
		"login-flow/login-timeout-race.md",
		"login-fix.md",
		"export-csv.md",
	)
	return mustLoad(t, dir, aliasesByFile(map[string][]string{
		"login-timeout-race.md": {"apogee-kk0.4", "old-timeout"},
		"export-csv.md":         {"apogee-qi3"},
	}))
}

func TestResolve_Matches(t *testing.T) {
	t.Parallel()
	loaded := resolveFixture(t)
	cases := []struct {
		name string
		arg  string
		want string
	}{
		{"exact id", "login-flow", "login-flow"},
		{"nested exact id", "login-timeout-race", "login-timeout-race"},
		{"legacy alias", "apogee-kk0.4", "login-timeout-race"},
		{"alias", "apogee-qi3", "export-csv"},
		{"unique prefix", "exp", "export-csv"},
		{"unique prefix of nested id", "login-t", "login-timeout-race"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			node, err := loaded.Resolve(testCase.arg)

			if err != nil || node.ID != testCase.want {
				t.Fatalf("Resolve(%q) = %q, %v; want %q", testCase.arg, node.ID, err, testCase.want)
			}
		})
	}
}

func TestResolve_ExactIDBeatsPrefix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "login-flow.md", "login-flow-v2.md")
	loaded := mustLoad(t, dir, nil)

	node, err := loaded.Resolve("login-flow")

	if err != nil || node.ID != "login-flow" {
		t.Fatalf("Resolve = %q, %v; want login-flow", node.ID, err)
	}
}

func TestResolve_AmbiguousPrefix_ListsCandidates(t *testing.T) {
	t.Parallel()
	loaded := resolveFixture(t)

	_, err := loaded.Resolve("login")

	var ambiguous *tree.AmbiguousError
	if !errors.As(err, &ambiguous) || !errors.Is(err, tree.ErrAmbiguous) {
		t.Fatalf("Resolve = %v, want *AmbiguousError", err)
	}
	want := []string{"login-fix", "login-flow", "login-timeout-race"}
	if !reflect.DeepEqual(ambiguous.Candidates, want) {
		t.Fatalf("Candidates = %v, want %v", ambiguous.Candidates, want)
	}
}

func TestResolve_NoMatch_NotFound(t *testing.T) {
	t.Parallel()
	loaded := resolveFixture(t)
	for _, arg := range []string{"nothing-here", "", "apogee", "timeout-race"} {
		t.Run(arg, func(t *testing.T) {
			_, err := loaded.Resolve(arg)

			if !errors.Is(err, tree.ErrNotFound) {
				t.Fatalf("Resolve(%q) = %v, want ErrNotFound", arg, err)
			}
		})
	}
}

func TestNormalizeArg_CitationSpellings(t *testing.T) {
	t.Parallel()
	cases := []struct {
		arg  string
		want string
	}{
		{"ADR 0076", "adr-0076"},
		{"  Floor   guard ", "floor-guard"},
		{"apogee-kk0.4", "apogee-kk0.4"},
		{"Login\tFlow", "login-flow"},
		{" \t ", ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.arg, func(t *testing.T) {
			if got := tree.NormalizeArg(testCase.arg); got != testCase.want {
				t.Fatalf("NormalizeArg(%q) = %q, want %q", testCase.arg, got, testCase.want)
			}
		})
	}
}

func TestResolve_CitationSpelling_Matches(t *testing.T) {
	t.Parallel()
	loaded := resolveFixture(t)
	cases := []struct {
		arg  string
		want string
	}{
		{"Login Timeout Race", "login-timeout-race"},
		{"  login   flow ", "login-flow"},
		{"APOGEE-KK0.4", "login-timeout-race"},
		{"Export", "export-csv"},
	}
	for _, testCase := range cases {
		t.Run(testCase.arg, func(t *testing.T) {
			node, err := loaded.Resolve(testCase.arg)

			if err != nil || node.ID != testCase.want {
				t.Fatalf("Resolve(%q) = %q, %v; want %q", testCase.arg, node.ID, err, testCase.want)
			}
		})
	}
}

func TestResolve_CitationSpelling_ErrorsQuoteTheArgumentAsTyped(t *testing.T) {
	t.Parallel()
	loaded := resolveFixture(t)

	_, notFound := loaded.Resolve("ADR 9999")
	_, ambiguous := loaded.Resolve("Login")
	_, blank := loaded.Resolve("   ")

	if !errors.Is(notFound, tree.ErrNotFound) || notFound.Error() != `id "ADR 9999" not found` {
		t.Errorf("Resolve(ADR 9999) = %v", notFound)
	}
	var ambiguousErr *tree.AmbiguousError
	if !errors.As(ambiguous, &ambiguousErr) || ambiguousErr.Arg != "Login" {
		t.Errorf("Resolve(Login) = %v, want an *AmbiguousError quoting Login", ambiguous)
	}
	if !errors.Is(blank, tree.ErrNotFound) || blank.Error() != `id "   " not found` {
		t.Errorf("Resolve(blank) = %v", blank)
	}
}

func TestLookup_IgnoresPrefixes(t *testing.T) {
	t.Parallel()
	loaded := resolveFixture(t)
	cases := []struct {
		name    string
		isFound bool
	}{
		{"export-csv", true},
		{"old-timeout", true},
		{"exp", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, found := loaded.Lookup(testCase.name)

			if found != testCase.isFound {
				t.Fatalf("Lookup(%q) found = %v, want %v", testCase.name, found, testCase.isFound)
			}
		})
	}
}

// loadWithDuplicates loads dir, which holds at least one duplicated id or alias, and fails the test
// unless Load reports ErrDuplicate together with the tree of what it could place.
func loadWithDuplicates(t *testing.T, dir string, aliases tree.AliasFunc) *tree.Tree {
	t.Helper()
	loaded, err := tree.Load(dir, aliases)
	if !errors.Is(err, tree.ErrDuplicate) || loaded == nil {
		t.Fatalf("Load = %v, %v; want a tree and ErrDuplicate", loaded, err)
	}
	return loaded
}

func TestResolve_DuplicatedName_FailsNamingTwoClaimants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		files   []string
		aliases map[string][]string
		arg     string
		want    [2]string
	}{
		{
			name:  "id in two files",
			files: []string{"foo.md", "sub.md", "sub/foo.md", "foo-bar.md"},
			arg:   "foo",
			want:  [2]string{"foo.md", "sub/foo.md"},
		},
		{
			name:  "citation spelling of an id in two files",
			files: []string{"foo.md", "sub.md", "sub/foo.md", "foo-bar.md"},
			arg:   " FOO ",
			want:  [2]string{"foo.md", "sub/foo.md"},
		},
		{
			name:    "id another file aliases",
			files:   []string{"foo.md", "baz.md", "foo-bar.md"},
			aliases: map[string][]string{"baz.md": {"foo"}},
			arg:     "foo",
			want:    [2]string{"foo.md", "baz.md"},
		},
		{
			name:    "alias of two files",
			files:   []string{"bar.md", "baz.md"},
			aliases: map[string][]string{"bar.md": {"legacy-1"}, "baz.md": {"legacy-1"}},
			arg:     "legacy-1",
			want:    [2]string{"bar.md", "baz.md"},
		},
		{
			name:    "id of a kept file a dropped duplicate-id file aliases",
			files:   []string{"foo.md", "bar.md", "sub.md", "sub/foo.md"},
			aliases: map[string][]string{"sub/foo.md": {"bar"}},
			arg:     "bar",
			want:    [2]string{"bar.md", "sub/foo.md"},
		},
		{
			name:    "alias of a kept file a dropped duplicate-id file aliases",
			files:   []string{"foo.md", "bar.md", "sub.md", "sub/foo.md"},
			aliases: map[string][]string{"bar.md": {"legacy-1"}, "sub/foo.md": {"legacy-1"}},
			arg:     "legacy-1",
			want:    [2]string{"bar.md", "sub/foo.md"},
		},
		{
			name:  "alias of two dropped duplicate-id files",
			files: []string{"foo.md", "bar.md", "sub.md", "sub/foo.md", "sub/bar.md"},
			aliases: map[string][]string{
				"sub/bar.md": {"legacy-1"},
				"sub/foo.md": {"legacy-1"},
			},
			arg:  "legacy-1",
			want: [2]string{"sub/bar.md", "sub/foo.md"},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeNodes(t, dir, testCase.files...)
			loaded := loadWithDuplicates(t, dir, aliasesByPath(dir, testCase.aliases))

			node, err := loaded.Resolve(testCase.arg)

			var problem *tree.Error
			if !errors.As(err, &problem) || !errors.Is(err, tree.ErrDuplicate) || errors.Is(err, tree.ErrNotFound) {
				t.Fatalf("Resolve(%q) = %q, %v; want an *Error wrapping ErrDuplicate", testCase.arg, node.ID, err)
			}
			for _, file := range testCase.want {
				path := filepath.Join(dir, filepath.FromSlash(file))
				if problem.Path != path && problem.Other != path {
					t.Errorf("Resolve(%q) = %v, want it to name %s", testCase.arg, err, path)
				}
			}
		})
	}
}

func TestResolve_AliasOnlyADroppedFileLists_IsNotADuplicate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "foo.md", "sub.md", "sub/foo.md")
	loaded := loadWithDuplicates(t, dir, aliasesByPath(dir, map[string][]string{
		"sub/foo.md": {"legacy-1"},
	}))

	node, err := loaded.Resolve("legacy-1")

	if !errors.Is(err, tree.ErrNotFound) || errors.Is(err, tree.ErrDuplicate) {
		t.Fatalf("Resolve(legacy-1) = %q, %v; want ErrNotFound, as before the file was dropped", node.ID, err)
	}
}

func TestResolve_DuplicatedName_LeavesOtherNamesAndLookupAlone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "foo.md", "sub.md", "sub/foo.md", "foo-bar.md")
	loaded := loadWithDuplicates(t, dir, nil)
	cases := []struct {
		arg  string
		want string
	}{
		{"foo-b", "foo-bar"},
		{"foo-bar", "foo-bar"},
		{"sub", "sub"},
	}
	for _, testCase := range cases {
		node, err := loaded.Resolve(testCase.arg)

		if err != nil || node.ID != testCase.want {
			t.Errorf("Resolve(%q) = %q, %v; want %q", testCase.arg, node.ID, err, testCase.want)
		}
	}

	node, found := loaded.Lookup("foo")

	if found {
		t.Errorf("Lookup(foo) = %+v, want nothing: both files of the id are left out", node)
	}
}

func TestResolve_NodeBelowDuplicatedID_FailsNamingBothFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "a.md", "a/x.md", "b.md", "b/x.md", "b/x/c2.md")
	loaded := loadWithDuplicates(t, dir, aliasesByPath(dir, map[string][]string{
		"b/x/c2.md": {"legacy-c2"},
	}))

	for _, arg := range []string{"c2", "c", "legacy-c2"} {
		node, err := loaded.Resolve(arg)

		var problem *tree.Error
		if !errors.As(err, &problem) || !errors.Is(err, tree.ErrDuplicate) {
			t.Fatalf("Resolve(%q) = %q, %v; want an *Error wrapping ErrDuplicate", arg, node.ID, err)
		}
		for _, file := range []string{"a/x.md", "b/x.md"} {
			path := filepath.Join(dir, filepath.FromSlash(file))
			if problem.Path != path && problem.Other != path {
				t.Errorf("Resolve(%q) = %v, want it to name %s", arg, err, path)
			}
		}
	}
}

func TestResolve_AliasOfTheFirstFileOfADuplicatedID_FailsInsteadOfPrefixMatching(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "foo.md", "sub.md", "sub/foo.md", "auth-tests.md")
	loaded := loadWithDuplicates(t, dir, aliasesByPath(dir, map[string][]string{
		"foo.md": {"auth"},
	}))

	node, err := loaded.Resolve("auth")

	var problem *tree.Error
	if !errors.As(err, &problem) || !errors.Is(err, tree.ErrDuplicate) {
		t.Fatalf("Resolve(auth) = %q, %v; want the duplicate problem of foo, not auth-tests", node.ID, err)
	}
	if problem.Other != filepath.Join(dir, "foo.md") {
		t.Errorf("Resolve(auth) = %v, want it to name foo.md", err)
	}
}

func TestResolve_PrefixOfADuplicatedIDAndAKeptID_StaysAmbiguous(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "foo.md", "sub-issue.md", "sub-issue/foo.md", "foo-bar.md")
	loaded := loadWithDuplicates(t, dir, nil)

	node, err := loaded.Resolve("fo")

	var ambiguous *tree.AmbiguousError
	if !errors.As(err, &ambiguous) || !reflect.DeepEqual(ambiguous.Candidates, []string{"foo", "foo-bar"}) {
		t.Errorf("Resolve(fo) = %q, %v; want ambiguous between foo and foo-bar", node.ID, err)
	}
}
