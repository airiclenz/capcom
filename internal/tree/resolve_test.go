package tree_test

import (
	"errors"
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
