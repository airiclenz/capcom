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
	loaded, err := tree.Load(dir, "issue", aliases)
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

func TestResolve_AliasOnlyADroppedFileLists_FailsAsAnUnusableClaim(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeNodes(t, dir, "foo.md", "sub.md", "sub/foo.md", "legacy-10.md")
	loaded := loadWithDuplicates(t, dir, aliasesByPath(dir, map[string][]string{
		"sub/foo.md": {"legacy-1"},
	}))

	node, err := loaded.Resolve("legacy-1")

	var unusable *tree.UnusableError
	if !errors.As(err, &unusable) || unusable.Path != filepath.Join(dir, "sub", "foo.md") {
		t.Fatalf("Resolve(legacy-1) = %q, %v; want an *UnusableError at sub/foo.md, never legacy-10", node.ID, err)
	}
	if !errors.Is(err, tree.ErrNotFound) || errors.Is(err, tree.ErrDuplicate) {
		t.Errorf("Resolve(legacy-1) = %v; want ErrNotFound, not ErrDuplicate", err)
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

// contentByPath returns a ContentFunc that looks a node file's content up by its slash path below
// dir; a file it does not list parses, passes validation and has no aliases.
func contentByPath(dir string, contents map[string]tree.Content) tree.ContentFunc {
	return func(path string) (tree.Content, error) {
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return tree.Content{}, err
		}
		if content, isListed := contents[filepath.ToSlash(relative)]; isListed {
			return content, nil
		}
		return tree.Content{Parse: tree.ParsedFile, IsUsable: true}, nil
	}
}

// unusableClaimsFixture loads, through LoadContent, a tree holding a file of every unusable kind
// beside usable nodes whose ids the unusable names are prefixes of.
func unusableClaimsFixture(t *testing.T) (string, *tree.Tree) {
	t.Helper()
	dir := t.TempDir()
	writeNodes(t, dir,
		"zzz.md", "foo-bar.md", "bad.md", "worse-luck.md", "odd.md", "odd-one.md",
		"ghost/orphan.md", "orphan-twin.md", "spooky-tale.md", "huge-hit.md",
	)
	writeSizedFile(t, filepath.Join(dir, "huge.md"), 1<<20+1)
	loaded, err := tree.LoadContent(dir, "issue", contentByPath(dir, map[string]tree.Content{
		"zzz.md":          {Aliases: []string{"foo"}, Parse: tree.UnparsedFile},
		"bad.md":          {Aliases: []string{"worse"}, Parse: tree.ParsedFile},
		"odd.md":          {Aliases: []string{"odd"}, Parse: tree.ParsedFile, IsUsable: true},
		"ghost/orphan.md": {Aliases: []string{"spooky"}, Parse: tree.UnparsedFile},
	}))
	var problems tree.Errors
	if !errors.As(err, &problems) || loaded == nil {
		t.Fatalf("LoadContent = %v, %v; want a tree and its problems", loaded, err)
	}
	return dir, loaded
}

func TestResolve_NameAnUnusableFileClaims_FailsNamingTheFile(t *testing.T) {
	t.Parallel()
	dir, loaded := unusableClaimsFixture(t)
	cases := []struct {
		name string
		arg  string
		file string
	}{
		{"alias an unparsed file recovers", "foo", "zzz.md"},
		{"prefix of an unparsed file's id alone", "zz", "zzz.md"},
		{"id of a file that fails validation", "bad", "bad.md"},
		{"alias of a file that fails validation", "worse", "bad.md"},
		{"id of a file with a rejected alias", "odd", "odd.md"},
		{"id of a node file below an orphan directory", "orphan", "ghost/orphan.md"},
		{"alias of a node file below an orphan directory", "spooky", "ghost/orphan.md"},
		{"id of a node file the walk left out unread", "huge", "huge.md"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			node, err := loaded.Resolve(testCase.arg)

			var unusable *tree.UnusableError
			want := filepath.Join(dir, filepath.FromSlash(testCase.file))
			if !errors.As(err, &unusable) || unusable.Path != want || !errors.Is(err, tree.ErrNotFound) {
				t.Fatalf("Resolve(%q) = %q, %v; want an *UnusableError at %s", testCase.arg, node.ID, err, want)
			}
		})
	}
}

func TestResolve_ExactIDOfAnUnparsedNode_ResolvesToIt(t *testing.T) {
	t.Parallel()
	_, loaded := unusableClaimsFixture(t)

	node, err := loaded.Resolve("zzz")

	if err != nil || node.ID != "zzz" || node.Aliases != nil {
		t.Errorf("Resolve(zzz) = %+v, %v; want the node zzz, with no Aliases", node, err)
	}
}

func TestUnusableError_ReadsAsTheIndexWordsIt(t *testing.T) {
	t.Parallel()
	err := &tree.UnusableError{Arg: "Foo", Path: ".capcom/issues/zzz.md"}

	got := err.Error()

	if want := `id "Foo" not found: .capcom/issues/zzz.md is invalid`; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestResolveIn_ExactIDOfAnUnparsedNode_FailsAsAnUnusableClaim(t *testing.T) {
	t.Parallel()
	dir, loaded := unusableClaimsFixture(t)

	id, err := tree.ResolveIn(loaded, "zzz")

	var unusable *tree.UnusableError
	want := filepath.Join(dir, "zzz.md")
	if !errors.As(err, &unusable) || unusable.Path != want || unusable.Arg != "zzz" {
		t.Errorf("ResolveIn(zzz) = %q, %v; want an *UnusableError at %s", id, err, want)
	}
}

func TestResolve_PrefixOfAnUnusableNodesIDAnotherFileClaims_FailsAsDuplicate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		file    string
		content tree.Content
		arg     string
	}{
		{"node file that does not parse", "zzz.md", tree.Content{Parse: tree.UnparsedFile}, "zz"},
		{"node file that fails validation", "bad.md", tree.Content{Parse: tree.ParsedFile}, "ba"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeNodes(t, dir, testCase.file, "ghost/"+testCase.file)
			loaded, err := tree.LoadContent(dir, "issue", contentByPath(dir, map[string]tree.Content{
				testCase.file: testCase.content,
			}))
			if loaded == nil {
				t.Fatalf("LoadContent = nil, %v; want a tree", err)
			}

			node, err := loaded.Resolve(testCase.arg)

			if !errors.Is(err, tree.ErrDuplicate) {
				t.Errorf("Resolve(%q) = %q, %v; want a failure wrapping ErrDuplicate", testCase.arg, node.ID, err)
			}
		})
	}
}

// failingNames is a NameReader whose Match fails for one name and whose PrefixCandidates fail
// outright; every other name matches nothing.
type failingNames struct {
	failingName string
	err         error
}

func (f failingNames) Match(name string) (tree.NameMatch, error) {
	if name == f.failingName {
		return tree.NameMatch{}, f.err
	}
	return tree.NameMatch{}, nil
}

func (f failingNames) PrefixCandidates(string) ([]string, error) {
	return nil, f.err
}

func TestResolveIn_NameReaderFails_ReturnsItsErrorUnchanged(t *testing.T) {
	t.Parallel()
	failure := errors.New("snapshot gone")
	cases := []struct {
		name  string
		names failingNames
	}{
		{"exact match", failingNames{failingName: "login", err: failure}},
		{"prefix candidates", failingNames{failingName: "", err: failure}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			id, err := tree.ResolveIn(testCase.names, "Login")

			if !errors.Is(err, failure) || id != "" {
				t.Errorf("ResolveIn(Login) = %q, %v; want %v", id, err, failure)
			}
		})
	}
}

// prefixNames is a NameReader holding one prefix candidate and the match of that id.
type prefixNames struct {
	candidate string
	match     tree.NameMatch
}

func (p prefixNames) Match(name string) (tree.NameMatch, error) {
	if name == p.candidate {
		return p.match, nil
	}
	return tree.NameMatch{}, nil
}

func (p prefixNames) PrefixCandidates(string) ([]string, error) {
	return []string{p.candidate}, nil
}

func TestResolveIn_OneCandidate_AnswersAsItsMatchSays(t *testing.T) {
	t.Parallel()
	duplicate := tree.Conflict{Key: "a/dup.md", Other: "b/dup.md", Name: "dup"}
	cases := []struct {
		name  string
		match tree.NameMatch
		want  string
	}{
		{"usable node", tree.NameMatch{ID: "dup"}, "id dup"},
		{"usable node beats a duplicate", tree.NameMatch{ID: "dup", Refusal: tree.DuplicateRefusal, Conflict: duplicate}, "id dup"},
		{"unusable claim beats a node", tree.NameMatch{ID: "dup", Refusal: tree.UnusableRefusal, Conflict: tree.Conflict{Key: "dup.md"}}, "unusable dup.md"},
		{"alias owner only", tree.NameMatch{ID: "other", IsAlias: true}, "not found"},
		{"alias owner and a duplicate", tree.NameMatch{ID: "other", IsAlias: true, Refusal: tree.DuplicateRefusal, Conflict: duplicate}, "duplicate a/dup.md"},
		{"nothing", tree.NameMatch{}, "not found"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			id, err := tree.ResolveIn(prefixNames{candidate: "dup", match: testCase.match}, "du")

			if got := resolveInAnswer(id, err); got != testCase.want {
				t.Errorf("ResolveIn(du) = %q, want %q", got, testCase.want)
			}
		})
	}
}

// resolveInAnswer renders ResolveIn's outcome: the id it found, or the kind of its failure with
// the file the failure names.
func resolveInAnswer(id string, err error) string {
	var unusable *tree.UnusableError
	var problem *tree.Error
	switch {
	case err == nil:
		return "id " + id
	case errors.As(err, &unusable):
		return "unusable " + unusable.Path
	case errors.As(err, &problem) && errors.Is(err, tree.ErrDuplicate):
		return "duplicate " + problem.Path
	case errors.Is(err, tree.ErrNotFound):
		return "not found"
	default:
		return "error " + err.Error()
	}
}
