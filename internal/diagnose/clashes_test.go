package diagnose_test

import (
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
					Message: `duplicate id "alpha": also the alias of b.md; both are left out`,
				},
				{
					Kind: diagnose.Duplicate, Path: "a.md",
					Message: `duplicate id "alpha": also the alias of c.md; both are left out`,
				},
				{
					Kind: diagnose.Duplicate, Path: "b.md",
					Message: `duplicate alias "alpha": also the id of a.md; both are left out`,
				},
				{
					Kind: diagnose.Duplicate, Path: "b.md",
					Message: `duplicate alias "alpha": also the alias of c.md; both are left out`,
				},
				{
					Kind: diagnose.Duplicate, Path: "c.md",
					Message: `duplicate alias "alpha": also the id of a.md; both are left out`,
				},
				{
					Kind: diagnose.Duplicate, Path: "c.md",
					Message: `duplicate alias "alpha": also the alias of b.md; both are left out`,
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
