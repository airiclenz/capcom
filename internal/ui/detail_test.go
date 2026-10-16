package ui

import (
	"strings"
	"testing"
	"time"

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
