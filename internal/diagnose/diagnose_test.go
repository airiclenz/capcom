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
