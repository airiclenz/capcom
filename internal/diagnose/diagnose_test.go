package diagnose_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/tree"
)

// aliasProblem returns the one problem tree.CheckAliases reports for aliases of the issue alpha.
func aliasProblem(t *testing.T, aliases ...string) *tree.Error {
	t.Helper()
	_, problems := tree.CheckAliases("alpha.md", "issue", "alpha", aliases)
	if len(problems) != 1 {
		t.Fatalf("CheckAliases(%q) = %v, want one problem", aliases, problems)
	}
	return problems[0]
}

func TestTreeKind_EachSentinel_ReturnsItsKind(t *testing.T) {
	tests := []struct {
		name    string
		problem *tree.Error
		want    diagnose.Kind
	}{
		{"invalid id", &tree.Error{Path: "Bad.md", Err: fmt.Errorf("%w: Bad", tree.ErrInvalidID)}, diagnose.InvalidID},
		{"invalid alias", aliasProblem(t, "Not Valid"), diagnose.InvalidAlias},
		{"alias repeating its own id", aliasProblem(t, "alpha"), diagnose.RepeatedAlias},
		{"alias listed twice", aliasProblem(t, "beta", "beta"), diagnose.RepeatedAlias},
		{"duplicate id", tree.DuplicateIDError("b/alpha.md", "a/alpha.md", "alpha"), diagnose.Duplicate},
		{"orphan directory", &tree.Error{Path: "beta", Err: tree.ErrOrphanDirectory}, diagnose.OrphanDirectory},
		{"repeated id's directory", tree.DuplicateIDDirectoryError("alpha", "alpha"), diagnose.OrphanDirectory},
		{"not regular", &tree.Error{Path: "link.md", Err: tree.ErrNotRegular}, diagnose.NotRegular},
		{"too large", &tree.Error{Path: "big.md", Err: tree.ErrTooLarge}, diagnose.NotRegular},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, isKnown := diagnose.TreeKind(test.problem)

			if got != test.want || !isKnown {
				t.Errorf("TreeKind() = %q, %v, want %q, true", got, isKnown, test.want)
			}
		})
	}
}

func TestTreeKind_NoKnownSentinel_ReturnsFalse(t *testing.T) {
	problem := &tree.Error{Path: "alpha.md", Err: errors.New("disk on fire")}

	got, isKnown := diagnose.TreeKind(problem)

	if got != "" || isKnown {
		t.Errorf("TreeKind() = %q, %v, want \"\", false", got, isKnown)
	}
}

func TestReadRefusal_EachError_ReturnsTheRefusalSentinel(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{"not regular", fmt.Errorf("alpha.md: %w", nodefile.ErrNotRegular), tree.ErrNotRegular},
		{"too large", fmt.Errorf("alpha.md: %w", nodefile.ErrTooLarge), tree.ErrTooLarge},
		{"plain error", errors.New("permission denied"), nil},
		{"no error", nil, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := diagnose.ReadRefusal(test.err)

			if got != test.want {
				t.Errorf("ReadRefusal(%v) = %v, want %v", test.err, got, test.want)
			}
		})
	}
}

// A Kind's string is the code check --json reports and the kind the index stores with a warning,
// so every value is pinned here.
func TestKind_EachKind_PinsItsString(t *testing.T) {
	tests := map[diagnose.Kind]string{
		diagnose.InvalidID:           "invalid-id",
		diagnose.InvalidAlias:        "invalid-alias",
		diagnose.Duplicate:           "duplicate",
		diagnose.RepeatedAlias:       "repeated-alias",
		diagnose.OrphanDirectory:     "orphan-directory",
		diagnose.NotRegular:          "not-regular",
		diagnose.Frontmatter:         "frontmatter",
		diagnose.Field:               "field",
		diagnose.Config:              "config",
		diagnose.Conflict:            "conflict",
		diagnose.MissingDep:          "missing-dep",
		diagnose.MissingRelated:      "missing-related",
		diagnose.MissingSupersededBy: "missing-superseded-by",
		diagnose.FormatTooOld:        "format-too-old",
		diagnose.Cycle:               "cycle",
		diagnose.MissingPlan:         "missing-plan",
		diagnose.Standing:            "standing",
		diagnose.BrokenRef:           "broken-ref",
		diagnose.UnsharedRef:         "unshared-ref",
		diagnose.AgentsBlock:         "agents-block",
		diagnose.AgentsGuide:         "agents-guide",
	}

	for kind, want := range tests {
		t.Run(want, func(t *testing.T) {
			if string(kind) != want {
				t.Errorf("Kind = %q, want %q", kind, want)
			}
		})
	}
}
