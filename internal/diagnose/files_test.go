package diagnose_test

import (
	"slices"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/diagnose"
)

func TestParseIssue_FileDoesNotParse_ReturnsOneFrontmatterFinding(t *testing.T) {
	tests := []struct {
		name string
		data string
		want diagnose.Finding
	}{
		{
			name: "with a line",
			data: "---\ntitle: a\nbogus: 1\n---\n",
			want: diagnose.Finding{
				Kind: diagnose.Frontmatter,
				Path: "alpha.md",
				Line: 3,
				Message: "unknown key \"bogus\"; an issue has title, status, type, priority, labels, deps, " +
					"related, plan, aliases, created, closed and reason",
			},
		},
		{
			name: "without a line",
			data: "---\n\ttitle: a\n---\n",
			want: diagnose.Finding{
				Kind:    diagnose.Frontmatter,
				Path:    "alpha.md",
				Message: "invalid YAML frontmatter: yaml: found character that cannot start any token",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, findings := diagnose.ParseIssue("alpha.md", []byte(test.data))

			if !slices.Equal(findings, []diagnose.Finding{test.want}) {
				t.Errorf("findings = %#v, want %#v", findings, test.want)
			}
			if parsed.Title != "" {
				t.Errorf("parsed title = %q, want the zero issue", parsed.Title)
			}
		})
	}
}

func TestParseNote_FileDoesNotParse_ReturnsOneFrontmatterFinding(t *testing.T) {
	_, findings := diagnose.ParseNote("alpha.md", []byte("---\ntitle: a\nbogus: 1\n---\n"))

	want := []diagnose.Finding{{
		Kind:    diagnose.Frontmatter,
		Path:    "alpha.md",
		Line:    3,
		Message: "unknown key \"bogus\"; a note has title, labels, refs, retired, superseded-by and aliases",
	}}
	if !slices.Equal(findings, want) {
		t.Errorf("findings = %#v, want %#v", findings, want)
	}
}

func TestValidateIssue_TwoBadFields_ReturnsAFieldFindingForEach(t *testing.T) {
	data := "---\ntitle: \"\"\nstatus: nope\ntype: task\ncreated: 2026-10-13T00:00:00Z\n---\n"
	parsed, parseFindings := diagnose.ParseIssue("alpha.md", []byte(data))
	if parseFindings != nil {
		t.Fatalf("ParseIssue findings = %v, want none", parseFindings)
	}

	findings := diagnose.ValidateIssue("alpha.md", parsed, config.Default())

	want := []diagnose.Finding{
		{Kind: diagnose.Field, Path: "alpha.md", Line: 2, Message: "title is required"},
		{
			Kind:    diagnose.Field,
			Path:    "alpha.md",
			Line:    3,
			Message: "status \"nope\" is not declared in config; use one of [open active parked done dropped]",
		},
	}
	if !slices.Equal(findings, want) {
		t.Errorf("findings = %#v, want %#v", findings, want)
	}
}

func TestValidateNote_MissingTitle_ReturnsAFieldFinding(t *testing.T) {
	parsed, parseFindings := diagnose.ParseNote("alpha.md", []byte("---\nlabels: [x]\n---\n"))
	if parseFindings != nil {
		t.Fatalf("ParseNote findings = %v, want none", parseFindings)
	}

	findings := diagnose.ValidateNote("alpha.md", parsed)

	if len(findings) != 1 || findings[0].Kind != diagnose.Field || findings[0].Path != "alpha.md" {
		t.Errorf("findings = %#v, want one Field finding at alpha.md", findings)
	}
}

func TestAliasFindings_RejectedAliases_ReturnsTheirKinds(t *testing.T) {
	findings := diagnose.AliasFindings("alpha.md", "issue", "alpha", []string{"alpha", "Not Valid", "ok"})

	want := []diagnose.Finding{
		{Kind: diagnose.RepeatedAlias, Path: "alpha.md", Message: "alias \"alpha\" repeats the issue's own id"},
		{
			Kind:    diagnose.InvalidAlias,
			Path:    "alpha.md",
			Message: "invalid alias \"Not Valid\": must use only a-z, 0-9, dots and hyphens",
		},
	}
	if !slices.Equal(findings, want) {
		t.Errorf("findings = %#v, want %#v", findings, want)
	}
}

func TestFindings_CleanFile_ReturnsNone(t *testing.T) {
	data := "---\ntitle: Alpha\nstatus: open\ntype: task\naliases: [first]\ncreated: 2026-10-13T00:00:00Z\n---\n"
	parsed, parseFindings := diagnose.ParseIssue("alpha.md", []byte(data))

	findings := slices.Concat(
		parseFindings,
		diagnose.ValidateIssue("alpha.md", parsed, config.Default()),
		diagnose.AliasFindings("alpha.md", "issue", "alpha", parsed.Aliases),
	)

	if findings != nil {
		t.Errorf("findings = %#v, want none", findings)
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
