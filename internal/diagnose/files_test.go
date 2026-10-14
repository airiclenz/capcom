package diagnose_test

import (
	"slices"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/diagnose"
)

func TestIssueFile_Verdict(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		wantTitle   string
		wantParse   []diagnose.Finding
		wantFields  []diagnose.Finding
		wantAliases []diagnose.Finding
	}{
		{
			name: "does not parse, with a line",
			data: "---\ntitle: a\nbogus: 1\n---\n",
			wantParse: []diagnose.Finding{{
				Code: diagnose.Frontmatter,
				Path: "alpha.md",
				Line: 3,
				Message: "unknown key \"bogus\"; an issue has title, status, type, priority, labels, deps, " +
					"related, plan, aliases, created, closed and reason",
			}},
		},
		{
			name: "does not parse, without a line",
			data: "---\n\ttitle: a\n---\n",
			wantParse: []diagnose.Finding{{
				Code:    diagnose.Frontmatter,
				Path:    "alpha.md",
				Message: "invalid YAML frontmatter: yaml: found character that cannot start any token",
			}},
		},
		{
			name: "two bad fields",
			data: "---\ntitle: \"\"\nstatus: nope\ntype: task\ncreated: 2026-10-13T00:00:00Z\n---\n",
			wantFields: []diagnose.Finding{
				{Code: diagnose.Field, Path: "alpha.md", Line: 2, Message: "title is required"},
				{
					Code:    diagnose.Field,
					Path:    "alpha.md",
					Line:    3,
					Message: "status \"nope\" is not declared in config; use one of [open active parked done dropped]",
				},
			},
		},
		{
			name:      "an alias repeats the id",
			data:      "---\ntitle: Alpha\nstatus: open\ntype: task\naliases: [alpha]\ncreated: 2026-10-13T00:00:00Z\n---\n",
			wantTitle: "Alpha",
			wantAliases: []diagnose.Finding{{
				Code:    diagnose.RepeatedAlias,
				Path:    "alpha.md",
				Line:    5,
				Message: "alias \"alpha\" repeats the issue's own id",
			}},
		},
		{
			name:      "clean",
			data:      "---\ntitle: Alpha\nstatus: open\ntype: task\naliases: [first]\ncreated: 2026-10-13T00:00:00Z\n---\n",
			wantTitle: "Alpha",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verdict := diagnose.IssueFile("alpha.md", "alpha", []byte(test.data), config.Default())

			if verdict.Parsed.Title != test.wantTitle {
				t.Errorf("parsed title = %q, want %q", verdict.Parsed.Title, test.wantTitle)
			}
			if verdict.IsParsed() != (test.wantParse == nil) {
				t.Errorf("IsParsed() = %v, want %v", verdict.IsParsed(), test.wantParse == nil)
			}
			assertVerdictFindings(t, verdict.Parse, verdict.Fields, verdict.Aliases, verdict.Findings(),
				test.wantParse, test.wantFields, test.wantAliases)
		})
	}
}

func TestNoteFile_Verdict(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		wantTitle   string
		wantParse   []diagnose.Finding
		wantFields  []diagnose.Finding
		wantAliases []diagnose.Finding
	}{
		{
			name: "does not parse",
			data: "---\ntitle: a\nbogus: 1\n---\n",
			wantParse: []diagnose.Finding{{
				Code:    diagnose.Frontmatter,
				Path:    "alpha.md",
				Line:    3,
				Message: "unknown key \"bogus\"; a note has title, labels, refs, retired, superseded-by and aliases",
			}},
		},
		{
			name: "missing title",
			data: "---\nlabels: [x]\n---\n",
			wantFields: []diagnose.Finding{
				{Code: diagnose.Field, Path: "alpha.md", Line: 0, Message: "title is required"},
			},
		},
		{
			name:      "an alias is no valid name",
			data:      "---\ntitle: Alpha\naliases: [Not Valid]\n---\n",
			wantTitle: "Alpha",
			wantAliases: []diagnose.Finding{{
				Code:    diagnose.InvalidAlias,
				Path:    "alpha.md",
				Line:    3,
				Message: "invalid alias \"Not Valid\": must use only a-z, 0-9, dots and hyphens",
			}},
		},
		{
			name:      "clean",
			data:      "---\ntitle: Alpha\naliases: [first]\n---\n",
			wantTitle: "Alpha",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verdict := diagnose.NoteFile("alpha.md", "alpha", []byte(test.data))

			if verdict.Parsed.Title != test.wantTitle {
				t.Errorf("parsed title = %q, want %q", verdict.Parsed.Title, test.wantTitle)
			}
			if verdict.IsParsed() != (test.wantParse == nil) {
				t.Errorf("IsParsed() = %v, want %v", verdict.IsParsed(), test.wantParse == nil)
			}
			assertVerdictFindings(t, verdict.Parse, verdict.Fields, verdict.Aliases, verdict.Findings(),
				test.wantParse, test.wantFields, test.wantAliases)
		})
	}
}

// assertVerdictFindings fails t unless each part of a verdict is its want, and all is the three
// wants in order, nil when every part is.
func assertVerdictFindings(
	t *testing.T,
	parse, fields, aliases, all []diagnose.Finding,
	wantParse, wantFields, wantAliases []diagnose.Finding,
) {
	t.Helper()
	if !slices.Equal(parse, wantParse) {
		t.Errorf("Parse = %#v, want %#v", parse, wantParse)
	}
	if !slices.Equal(fields, wantFields) {
		t.Errorf("Fields = %#v, want %#v", fields, wantFields)
	}
	if !slices.Equal(aliases, wantAliases) {
		t.Errorf("Aliases = %#v, want %#v", aliases, wantAliases)
	}
	wantAll := slices.Concat(wantParse, wantFields, wantAliases)
	if !slices.Equal(all, wantAll) || (all == nil) != (wantAll == nil) {
		t.Errorf("Findings() = %#v, want %#v", all, wantAll)
	}
}

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
