package ui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/airiclenz/capcom/internal/index"
)

func TestIssueHeader_RendersEachSetKeyInOrder(t *testing.T) {
	tests := []struct {
		name  string
		entry index.Entry
		want  string
	}{
		{
			name: "every key set",
			entry: index.Entry{
				ID:       "child",
				ParentID: "epic",
				Title:    "Child task",
				Status:   "done",
				Type:     "bug",
				Priority: 1,
				Labels:   []string{"ui", "tui"},
				Deps:     []string{"base", "shipped"},
				Related:  []string{"cousin"},
				Plan:     "2026-10-16 - 03 - some-plan",
				Aliases:  []string{"old-child"},
				Created:  time.Date(2026, 10, 11, 9, 0, 59, 0, time.UTC),
				Closed:   time.Date(2026, 10, 12, 11, 30, 0, 0, time.FixedZone("CEST", 2*60*60)),
				Reason:   "shipped\n  in two lines",
			},
			want: "# Child task\n\n" +
				"status: done · P1 · bug\n" +
				"labels: ui, tui\n" +
				"parent: `epic`\n" +
				"deps: `base`, `shipped`\n" +
				"related: `cousin`\n" +
				"plan: 2026-10-16 - 03 - some-plan\n" +
				"aliases: old-child\n" +
				"created: 2026-10-11 09:00\n" +
				"closed: 2026-10-12 09:30\n" +
				"reason: shipped in two lines\n" +
				"\n",
		},
		{
			name:  "standing mark after the plan",
			entry: index.Entry{ID: "area", Title: "Area", Status: "open", Priority: 2, Plan: "area-plan", IsStanding: true},
			want:  "# Area\n\nstatus: open · P2\nplan: area-plan\nstanding: yes\n\n",
		},
		{
			name:  "only title, status and priority",
			entry: index.Entry{ID: "bare", Title: "Bare task", Status: "open", Priority: 2},
			want:  "# Bare task\n\nstatus: open · P2\n\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := issueHeader(test.entry); got != test.want {
				t.Errorf("issueHeader() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDetailPane_IssueWithPlanAndRelated_ShowsBothLines(t *testing.T) {
	files := map[string]string{
		"issues/alpha.md": issueFile(
			"title: Alpha task\nstatus: open\npriority: 1\nrelated: [beta]\nplan: alpha-plan\n",
			"Alpha's body.\n",
		),
		"issues/beta.md": issueFile("title: Beta task\nstatus: open\npriority: 2\n", ""),
	}
	m := newIssuesModel(t, newTestStore(t, files), "alpha", 120, 30)

	text := detailText(m)
	for _, want := range []string{"related: beta", "plan: alpha-plan", "status: open · P1 · task", "created: 2026-10-11 09:00"} {
		if !strings.Contains(text, want) {
			t.Errorf("detail pane = %q, want a line %q", text, want)
		}
	}
}

func TestDetailPane_StandingIssue_ShowsTheMark(t *testing.T) {
	ts := newTestStore(t, standingFiles)

	text := detailText(newIssuesModel(t, ts, "area", 120, 30))
	if !strings.Contains(text, "standing: yes") {
		t.Errorf("detail pane of a standing issue = %q, want a line %q", text, "standing: yes")
	}
	if text := detailText(newIssuesModel(t, ts, "child", 120, 30)); strings.Contains(text, "standing:") {
		t.Errorf("detail pane of an unmarked issue = %q, want no standing line", text)
	}
}

// A link the detail pane wraps at a hyphen ends each of its rows with its underline closed: the
// upstream wrap carried the reset after the hyphen onto the next row, so the row ended with the
// underline open and it ran on past the link into whatever the screen painted after it.
func TestDetailPane_LinkWrappedAtAHyphen_EndsEveryRowWithNoStyleOpen(t *testing.T) {
	files := map[string]string{
		"issues/alpha.md": issueFile(
			"title: Alpha task\nstatus: open\npriority: 1\n",
			"Every issue the tracker files is named by a short id, as in [agents-md-no-block](beta).\n",
		),
		"issues/beta.md": issueFile("title: Beta task\nstatus: open\npriority: 2\n", ""),
	}

	m := newIssuesModel(t, newTestStore(t, files), "alpha", 120, 30)

	if !slices.ContainsFunc(m.detail, func(line string) bool {
		return strings.HasSuffix(strings.TrimSpace(ansi.Strip(line)), "agents-md-no-")
	}) {
		t.Fatalf("detail pane = %q; want the link broken at a hyphen", detailText(m))
	}
	for i, line := range m.detail {
		if isStyleOpenAtEnd(line) {
			t.Errorf("detail row %d %q ends with a style in effect", i, line)
		}
	}
}

// isStyleOpenAtEnd reports whether line, read from a terminal with no style in effect, leaves an
// SGR style in effect after its last byte: its last SGR sequence carries a parameter other than 0.
func isStyleOpenAtEnd(line string) bool {
	matches := sgrSequence.FindAllStringSubmatch(line, -1)
	return len(matches) > 0 && strings.Trim(matches[len(matches)-1][1], "0;") != ""
}

func TestNoteHeader_RendersEachSetKeyInOrder(t *testing.T) {
	tests := []struct {
		name  string
		entry index.NoteEntry
		want  string
	}{
		{
			name: "every key set",
			entry: index.NoteEntry{
				ID:           "old-guide",
				Title:        "Old guide",
				Labels:       []string{"howto", "cli"},
				Refs:         []string{"internal/ui/detail.go", "guide"},
				IsRetired:    true,
				SupersededBy: []string{"guide", "setup"},
				Aliases:      []string{"handbook"},
			},
			want: "# Old guide\n\n" +
				"labels: howto, cli\n" +
				"refs: `internal/ui/detail.go`, `guide`\n" +
				"retired: true\n" +
				"superseded-by: `guide`, `setup`\n" +
				"aliases: handbook\n" +
				"\n",
		},
		{
			name: "not retired, superseded or aliased",
			entry: index.NoteEntry{
				ID:     "guide",
				Title:  "Guide",
				Labels: []string{"howto"},
				Refs:   []string{"internal/ui/detail.go"},
			},
			want: "# Guide\n\n" +
				"labels: howto\n" +
				"refs: `internal/ui/detail.go`\n" +
				"\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := noteHeader(test.entry); got != test.want {
				t.Errorf("noteHeader() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestKnowledgeTab_RetiredNoteDetail_LinksItsSuccessor(t *testing.T) {
	files := map[string]string{
		"knowledge/guide.md": noteFile("title: Guide\n", "The guide.\n"),
		"knowledge/old.md": noteFile(
			"title: Old\nretired: true\nsuperseded-by: [guide]\naliases: [handbook]\n",
			"Retired.\n",
		),
	}
	m := newKnowledgeModel(t, newTestStore(t, files), "old", 120, 30)

	text := detailText(m)
	for _, want := range []string{"retired: true", "superseded-by: guide", "aliases: handbook"} {
		if !strings.Contains(text, want) {
			t.Errorf("detail pane = %q, want a line %q", text, want)
		}
	}
	targets := make([]string, 0, len(m.issue.rendered.Links))
	for _, link := range m.issue.rendered.Links {
		targets = append(targets, link.Target)
	}
	if !slices.Contains(targets, "guide") {
		t.Errorf("detail links = %v, want a link to guide", targets)
	}
}
