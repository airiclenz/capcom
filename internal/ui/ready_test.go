package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/airiclenz/capcom/internal/briefing"
	"github.com/airiclenz/capcom/internal/index"
)

// readyFiles is a store whose Ready queue is alpha (P1), beta (P2, unblocking gamma) and delta
// (P2), in that order: gamma waits on beta, epic waits on its open child delta, and finished is
// done.
var readyFiles = map[string]string{
	"issues/alpha.md": issueFile(
		"title: Alpha task\nstatus: open\npriority: 1\nlabels: [ui, tui]\ndeps: [finished]\n",
		"Alpha names `beta` and `no-such-id`.\n\n- first point\n- second point\n",
	),
	"issues/beta.md":       issueFile("title: Beta task\nstatus: open\npriority: 2\naliases: [beta-alias]\n", "Beta's body.\n"),
	"issues/gamma.md":      issueFile("title: Gamma task\nstatus: open\npriority: 0\ndeps: [beta]\n", ""),
	"issues/epic.md":       issueFile("title: Epic\nstatus: open\npriority: 3\n", ""),
	"issues/epic/delta.md": issueFile("title: Delta task\nstatus: open\npriority: 2\n", "Child of the epic.\n"),
	"issues/finished.md": issueFile(
		"title: Finished\nstatus: done\npriority: 2\nclosed: 2026-10-12T09:00:00Z\nreason: shipped\n",
		"",
	),
}

// issueFile returns an issue file with fields in its frontmatter, then a type and a created
// time, and body after it.
func issueFile(fields, body string) string {
	return "---\n" + fields + "type: task\ncreated: 2026-10-11T09:00:00Z\n---\n" + body
}

// detailText returns the detail pane's rows, their styling stripped, joined by newlines.
func detailText(m model) string {
	return ansi.Strip(strings.Join(m.detail, "\n"))
}

// readyIDs returns the ids of the Ready tab's queue, top to bottom.
func readyIDs(m model) []string {
	ids := make([]string, 0, len(m.ready.entries))
	for _, entry := range m.ready.entries {
		ids = append(ids, entry.ID)
	}
	return ids
}

func TestReadyTab_Sidebar_ListsTheReadyListInItsOrder(t *testing.T) {
	ts := newTestStore(t, readyFiles)
	listed, err := briefing.ReadyList(ts.index, ts.store.Config, briefing.ReadyOptions{Plan: index.PlanAny})
	if err != nil {
		t.Fatalf("ReadyList: %v", err)
	}
	want := make([]string, 0, len(listed))
	for _, item := range listed {
		want = append(want, item.Entry.ID)
	}

	m := newTestModel(t, ts, 120, 30)

	if got := readyIDs(m); !slices.Equal(got, want) || !slices.Equal(want, []string{"alpha", "beta", "delta"}) {
		t.Errorf("Ready queue = %v, want ReadyList's %v (alpha, beta, delta)", got, want)
	}
	if got := ansi.Strip(m.sidebar[1]); got != "beta  open  P2  Beta task" {
		t.Errorf("second sidebar row = %q, want %q", got, "beta  open  P2  Beta task")
	}
}

func TestReadyTab_PopulatedQueue_MatchesGolden(t *testing.T) {
	m := newTestModel(t, newTestStore(t, readyFiles), 120, 30)

	golden(t, "ready-120x30", m.View())
}

func TestReadyTab_EmptyQueue_MatchesGolden(t *testing.T) {
	m := newTestModel(t, newTestStore(t, nil), 120, 30)

	golden(t, "ready-empty-120x30", m.View())
}

func TestReadyTab_DownKeys_SelectTheNextIssueAndShowIt(t *testing.T) {
	m := newTestModel(t, newTestStore(t, readyFiles), 120, 30)

	m = press(t, m, runeKey('j'))
	m = press(t, m, specialKey(tea.KeyDown))
	m = press(t, m, runeKey('j'))

	text := detailText(m)
	if !strings.HasPrefix(text, "Delta task") || !strings.Contains(text, "parent: epic") ||
		!strings.Contains(text, "Child of the epic.") {
		t.Errorf("detail after j, down, j (stopping at the last row) = %q, want delta with parent epic", text)
	}
}

func TestReadyTab_OpenedOnAnID_SelectsThatIssue(t *testing.T) {
	cases := []struct {
		name       string
		id         string
		wantTitle  string
		wantNotice string
	}{
		{name: "id", id: "delta", wantTitle: "Delta task"},
		{name: "alias", id: "beta-alias", wantTitle: "Beta task"},
		{name: "not ready", id: "gamma", wantTitle: "Alpha task", wantNotice: "gamma is not in the Ready queue"},
		{name: "unknown", id: "no-such-id", wantTitle: "Alpha task", wantNotice: "select no-such-id: "},
	}
	ts := newTestStore(t, readyFiles)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			options := newTestOptions(t, ts)
			options.ID = testCase.id

			m := startModel(t, options, 160, 30)

			if text := detailText(m); !strings.HasPrefix(text, testCase.wantTitle) {
				t.Errorf("capcom ui ready %s: detail = %q, want %s", testCase.id, text, testCase.wantTitle)
			}
			if !strings.Contains(statusRow(m), testCase.wantNotice) {
				t.Errorf("capcom ui ready %s: status bar %q, want %q", testCase.id, statusRow(m), testCase.wantNotice)
			}
		})
	}
}

func TestReadyTab_BodyID_RendersAsALinkFromTheIDSnapshot(t *testing.T) {
	ts := newTestStore(t, readyFiles)
	m := newTestModel(t, ts, 120, 30)
	if err := os.Remove(filepath.Join(ts.store.Root, "issues", "beta.md")); err != nil {
		t.Fatalf("remove beta: %v", err)
	}
	if _, err := ts.index.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if _, isFound, err := ts.index.Issue("beta"); err != nil || isFound {
		t.Fatalf("Issue(beta) after its removal: found %t, error %v; want gone", isFound, err)
	}

	m = press(t, m, tea.WindowSizeMsg{Width: 130, Height: 30})

	targets := make([]string, 0, len(m.issue.rendered.Links))
	for _, link := range m.issue.rendered.Links {
		targets = append(targets, link.Target)
	}
	if !slices.Contains(targets, "beta") || !slices.Contains(targets, "finished") ||
		slices.Contains(targets, "no-such-id") {
		t.Errorf("links rendered at the new width = %v, want beta and finished from the snapshot, no no-such-id", targets)
	}
	if m.issue.renderedWidth != m.detailWidth() {
		t.Errorf("detail rendered at %d columns, want the new pane width %d", m.issue.renderedWidth, m.detailWidth())
	}
}

func TestReadyTab_UnparsableIssueFile_ShowsItsPathAndError(t *testing.T) {
	ts := newTestStore(t, map[string]string{"issues/alpha.md": issueFile("title: Alpha task\nstatus: open\n", "")})
	broken := "---\ntitle: [unclosed\n---\n"
	if err := os.WriteFile(filepath.Join(ts.store.Root, "issues", "alpha.md"), []byte(broken), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m := newTestModel(t, ts, 200, 30)

	text := detailText(m)
	if !strings.HasPrefix(text, "Alpha task") || !strings.Contains(text, ".capcom/issues/alpha.md:") {
		t.Errorf("detail of an unparsable file = %q, want the index header, then .capcom/issues/alpha.md: <error>", text)
	}
	if m.issue.etag != "" {
		t.Errorf("ETag kept for an unparsable file = %q, want none", m.issue.etag)
	}
}

func TestListCursor_MovesAndWheel(t *testing.T) {
	cases := []struct {
		name  string
		start int
		step  func(*listCursor)
		want  int
	}{
		{name: "down stops at the end", start: 2, step: func(l *listCursor) { l.move(1, 3, listStopsAtEnds) }, want: 2},
		{name: "down wraps to the first", start: 2, step: func(l *listCursor) { l.move(1, 3, listWrapsAround) }, want: 0},
		{name: "up wraps to the last", start: 0, step: func(l *listCursor) { l.move(-1, 3, listWrapsAround) }, want: 2},
		{name: "wheel down", start: 0, step: func(l *listCursor) { l.wheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown}, 3) }, want: 1},
		{name: "wheel up stops at the top", start: 0, step: func(l *listCursor) { l.wheel(tea.MouseWheelMsg{Button: tea.MouseWheelUp}, 3) }, want: 0},
		{name: "sideways wheel", start: 1, step: func(l *listCursor) { l.wheel(tea.MouseWheelMsg{Button: tea.MouseWheelLeft}, 3) }, want: 1},
		{name: "seat clamps", start: 0, step: func(l *listCursor) { l.seat(9, 3) }, want: 2},
		{name: "clamp after the rows shrank", start: 5, step: func(l *listCursor) { l.clampSelection(2) }, want: 1},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			cursor := listCursor{selected: testCase.start}

			testCase.step(&cursor)

			if cursor.selected != testCase.want {
				t.Errorf("selection = %d, want %d", cursor.selected, testCase.want)
			}
		})
	}
}

func TestListCursor_Highlight_EmptyListMarksNoRow(t *testing.T) {
	cursor := listCursor{selected: 3}

	if got := cursor.highlight(0); got != -1 {
		t.Errorf("highlight over no rows = %d, want -1", got)
	}
}
