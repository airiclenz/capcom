package diagnose_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/tree"
)

func TestNameClashes_IDConflicts_FindsEveryFileOfARepeatedID(t *testing.T) {
	tests := []struct {
		name      string
		conflicts []tree.Conflict
		want      []diagnose.Finding
	}{
		{
			name:      "no repeated id",
			conflicts: nil,
			want:      nil,
		},
		{
			name:      "two files sharing an id",
			conflicts: []tree.Conflict{{Key: "b/alpha.md", Other: "a/alpha.md", Name: "alpha"}},
			want: []diagnose.Finding{
				{
					Kind: diagnose.Duplicate, Path: "a/alpha.md",
					Message: `duplicate id "alpha": also the id of b/alpha.md; both are left out`,
				},
				{
					Kind: diagnose.Duplicate, Path: "b/alpha.md",
					Message: `duplicate id "alpha": also the id of a/alpha.md; both are left out`,
				},
			},
		},
		{
			name: "three files sharing an id",
			conflicts: []tree.Conflict{
				{Key: "b/alpha.md", Other: "a/alpha.md", Name: "alpha"},
				{Key: "c/alpha.md", Other: "a/alpha.md", Name: "alpha"},
			},
			want: []diagnose.Finding{
				{
					Kind: diagnose.Duplicate, Path: "a/alpha.md",
					Message: `duplicate id "alpha": also the id of b/alpha.md and 1 more; all are left out`,
				},
				{
					Kind: diagnose.Duplicate, Path: "b/alpha.md",
					Message: `duplicate id "alpha": also the id of a/alpha.md and 1 more; all are left out`,
				},
				{
					Kind: diagnose.Duplicate, Path: "c/alpha.md",
					Message: `duplicate id "alpha": also the id of a/alpha.md and 1 more; all are left out`,
				},
			},
		},
		{
			name: "output order follows the conflicts",
			conflicts: []tree.Conflict{
				{Key: "z2.md", Other: "z1.md", Name: "zulu"},
				{Key: "a2.md", Other: "a1.md", Name: "alpha"},
			},
			want: []diagnose.Finding{
				{
					Kind: diagnose.Duplicate, Path: "z1.md",
					Message: `duplicate id "zulu": also the id of z2.md; both are left out`,
				},
				{
					Kind: diagnose.Duplicate, Path: "z2.md",
					Message: `duplicate id "zulu": also the id of z1.md; both are left out`,
				},
				{
					Kind: diagnose.Duplicate, Path: "a1.md",
					Message: `duplicate id "alpha": also the id of a2.md; both are left out`,
				},
				{
					Kind: diagnose.Duplicate, Path: "a2.md",
					Message: `duplicate id "alpha": also the id of a1.md; both are left out`,
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := diagnose.NameClashes(test.conflicts)
			if !slices.Equal(got, test.want) {
				t.Errorf("NameClashes() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestNameClashes_ManyFiles_FindsOnePerFile(t *testing.T) {
	const fileCount = 1000
	paths := make([]string, 0, fileCount)
	conflicts := make([]tree.Conflict, 0, fileCount-1)
	for position := range fileCount {
		path := fmt.Sprintf("%04d/alpha.md", position)
		paths = append(paths, path)
		if position > 0 {
			conflicts = append(conflicts, tree.Conflict{Key: path, Other: paths[0], Name: "alpha"})
		}
	}

	got := diagnose.NameClashes(conflicts)

	if len(got) != fileCount {
		t.Fatalf("NameClashes() returned %d findings, want %d", len(got), fileCount)
	}
	for position, finding := range got {
		other := paths[0]
		if position == 0 {
			other = paths[1]
		}
		want := diagnose.Finding{
			Kind:    diagnose.Duplicate,
			Path:    paths[position],
			Message: `duplicate id "alpha": also the id of ` + other + " and 998 more; all are left out",
		}
		if finding != want {
			t.Errorf("finding %d = %v, want %v", position, finding, want)
		}
	}
}
