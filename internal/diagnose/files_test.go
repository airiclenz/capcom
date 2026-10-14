package diagnose_test

import (
	"slices"
	"testing"

	"github.com/airiclenz/capcom/internal/diagnose"
)

func TestAliasFindings_RejectedAliases_ReturnsTheirKinds(t *testing.T) {
	findings := diagnose.AliasFindings("alpha.md", "issue", "alpha", []string{"alpha", "Not Valid", "ok"}, 5)

	want := []diagnose.Finding{
		{
			Code:    diagnose.RepeatedAlias,
			Path:    "alpha.md",
			Line:    5,
			Message: "alias \"alpha\" repeats the issue's own id",
		},
		{
			Code:    diagnose.InvalidAlias,
			Path:    "alpha.md",
			Line:    5,
			Message: "invalid alias \"Not Valid\": must use only a-z, 0-9, dots and hyphens",
		},
	}
	if !slices.Equal(findings, want) {
		t.Errorf("findings = %#v, want %#v", findings, want)
	}
}

func TestFinding_String_RendersTheLineOnlyWhenKnown(t *testing.T) {
	tests := []struct {
		name    string
		finding diagnose.Finding
		want    string
	}{
		{"with a line", diagnose.Finding{Path: "alpha.md", Line: 3, Message: "bad"}, "alpha.md:3: bad"},
		{"without a line", diagnose.Finding{Path: "alpha.md", Message: "bad"}, "alpha.md: bad"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.finding.String(); got != test.want {
				t.Errorf("String() = %q, want %q", got, test.want)
			}
		})
	}
}
