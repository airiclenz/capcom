package diagnose_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/airiclenz/capcom/internal/diagnose"
)

func TestNameClashes_Claims_FindsEveryClaimantOfASharedName(t *testing.T) {
	tests := []struct {
		name   string
		claims []diagnose.Claim
		want   []diagnose.Finding
	}{
		{
			name:   "one claimant",
			claims: []diagnose.Claim{{Name: "alpha", Path: "a/alpha.md", Kind: "id"}},
			want:   nil,
		},
		{
			name: "two files sharing an id",
			claims: []diagnose.Claim{
				{Name: "alpha", Path: "a/alpha.md", Kind: "id"},
				{Name: "alpha", Path: "b/alpha.md", Kind: "id"},
			},
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
			name: "an id against an alias",
			claims: []diagnose.Claim{
				{Name: "alpha", Path: "alpha.md", Kind: "id"},
				{Name: "alpha", Path: "beta.md", Kind: "alias"},
			},
			want: []diagnose.Finding{
				{
					Kind: diagnose.Duplicate, Path: "alpha.md",
					Message: `duplicate id "alpha": also the alias of beta.md; both are left out`,
				},
				{
					Kind: diagnose.Duplicate, Path: "beta.md",
					Message: `duplicate alias "alpha": also the id of alpha.md; both are left out`,
				},
			},
		},
		{
			name: "three claimants",
			claims: []diagnose.Claim{
				{Name: "alpha", Path: "a.md", Kind: "id"},
				{Name: "alpha", Path: "b.md", Kind: "alias"},
				{Name: "alpha", Path: "c.md", Kind: "alias"},
			},
			want: []diagnose.Finding{
				{
					Kind: diagnose.Duplicate, Path: "a.md",
					Message: `duplicate id "alpha": also the alias of b.md and 1 more; all are left out`,
				},
				{
					Kind: diagnose.Duplicate, Path: "b.md",
					Message: `duplicate alias "alpha": also the id of a.md and 1 more; all are left out`,
				},
				{
					Kind: diagnose.Duplicate, Path: "c.md",
					Message: `duplicate alias "alpha": also the id of a.md and 1 more; all are left out`,
				},
			},
		},
		{
			name: "an identical claim repeated counts once",
			claims: []diagnose.Claim{
				{Name: "alpha", Path: "a.md", Kind: "alias"},
				{Name: "alpha", Path: "a.md", Kind: "alias"},
				{Name: "beta", Path: "b.md", Kind: "alias"},
				{Name: "beta", Path: "b.md", Kind: "alias"},
				{Name: "beta", Path: "c.md", Kind: "id"},
			},
			want: []diagnose.Finding{
				{
					Kind: diagnose.Duplicate, Path: "b.md",
					Message: `duplicate alias "beta": also the id of c.md; both are left out`,
				},
				{
					Kind: diagnose.Duplicate, Path: "c.md",
					Message: `duplicate id "beta": also the alias of b.md; both are left out`,
				},
			},
		},
		{
			name: "output order follows input",
			claims: []diagnose.Claim{
				{Name: "zulu", Path: "z1.md", Kind: "id"},
				{Name: "lone", Path: "l.md", Kind: "id"},
				{Name: "alpha", Path: "a2.md", Kind: "id"},
				{Name: "zulu", Path: "z2.md", Kind: "alias"},
				{Name: "alpha", Path: "a1.md", Kind: "alias"},
			},
			want: []diagnose.Finding{
				{
					Kind: diagnose.Duplicate, Path: "z1.md",
					Message: `duplicate id "zulu": also the alias of z2.md; both are left out`,
				},
				{
					Kind: diagnose.Duplicate, Path: "z2.md",
					Message: `duplicate alias "zulu": also the id of z1.md; both are left out`,
				},
				{
					Kind: diagnose.Duplicate, Path: "a2.md",
					Message: `duplicate id "alpha": also the alias of a1.md; both are left out`,
				},
				{
					Kind: diagnose.Duplicate, Path: "a1.md",
					Message: `duplicate alias "alpha": also the id of a2.md; both are left out`,
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := diagnose.NameClashes(test.claims)
			if !slices.Equal(got, test.want) {
				t.Errorf("NameClashes() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestNameClashes_ManyClaimants_FindsOnePerClaimant(t *testing.T) {
	const claimantCount = 1000
	claims := make([]diagnose.Claim, 0, claimantCount)
	for position := range claimantCount {
		path := fmt.Sprintf("%04d.md", position)
		claims = append(claims, diagnose.Claim{Name: "alpha", Path: path, Kind: "alias"})
	}

	got := diagnose.NameClashes(claims)

	if len(got) != claimantCount {
		t.Fatalf("NameClashes() returned %d findings, want %d", len(got), claimantCount)
	}
	for position, finding := range got {
		other := "0000.md"
		if position == 0 {
			other = "0001.md"
		}
		want := diagnose.Finding{
			Kind: diagnose.Duplicate,
			Path: claims[position].Path,
			Message: `duplicate alias "alpha": also the alias of ` + other +
				" and 998 more; all are left out",
		}
		if finding != want {
			t.Errorf("finding %d = %v, want %v", position, finding, want)
		}
	}
}
