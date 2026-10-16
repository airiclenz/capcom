package ui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// depFiles is a store where alpha, under epic, waits on base, on the done shipped under the done
// archive and on ghost, which names no issue, and blocks user; base blocks alpha; lonely has
// neither side.
var depFiles = map[string]string{
	"issues/epic.md": issueFile("title: Epic\nstatus: open\npriority: 1\n", "The epic.\n"),
	"issues/epic/alpha.md": issueFile(
		"title: Alpha\nstatus: active\npriority: 2\ndeps: [base, shipped, ghost]\n",
		"Alpha's body.\n",
	),
	"issues/base.md":            issueFile("title: Base\nstatus: open\npriority: 1\n", "Base's body.\n"),
	"issues/user.md":            issueFile("title: User\nstatus: open\npriority: 3\ndeps: [alpha]\n", ""),
	"issues/archive.md":         closedIssueFile("Archive"),
	"issues/archive/shipped.md": closedIssueFile("Shipped"),
	"issues/lonely.md":          issueFile("title: Lonely\nstatus: open\npriority: 2\n", "Lonely's body.\n"),
}

func TestDepView_IssueWithBothSides_MatchesGolden(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, depFiles), "alpha", 120, 30)

	m = press(t, m, runeKey('d'))

	golden(t, "deps-both-120x30", m.View())
}

func TestDepView_SelectedRow_FillsTheBackgroundKeepingItsColours(t *testing.T) {
	// dark's selection is #3a5fcd and its status-todo #b2b2b2.
	const (
		fill     = "48;2;58;95;205"
		todoText = "38;2;178;178;178"
	)
	m := newThemeModel(t, depFiles, "ui:\n  theme: dark\n", false, func(options *Options) {
		options.Tab = TabIssues
		options.ID = "alpha"
	})

	m = press(t, m, runeKey('d'))

	// base, open (todo), is alpha's first dependency and so the selected row once the view opens.
	if got := m.deps.cursor.highlight(len(m.deps.rows())); got != 0 {
		t.Fatalf("selected dependency row = %d, want 0 (base)", got)
	}
	index := slices.IndexFunc(m.detail, func(line string) bool { return strings.Contains(line, "Base") })
	if index < 0 {
		t.Fatalf("detail pane %q has no row for base", m.detail)
	}
	row := m.detail[index]
	for _, segment := range []struct{ text, want string }{
		{text: "base", want: fill},
		{text: "open", want: todoText},
		{text: "open", want: fill},
		{text: "Base", want: fill},
	} {
		if got := sgrBefore(t, row, segment.text); !strings.Contains(got, segment.want) {
			t.Errorf("%q in row %q is drawn in %q, want it to carry %s", segment.text, row, got, segment.want)
		}
	}
}

func TestDepView_IssueWithNeitherSide_SaysSo(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, depFiles), "lonely", 120, 30)

	m = press(t, m, runeKey('d'))

	if got, want := detailText(m), "lonely waits on no issue and blocks none."; got != want {
		t.Errorf("detail pane = %q, want %q", got, want)
	}
}

func TestDepView_EnterOnDependent_SelectsItInTheIssuesTabWithAncestorsExpanded(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, depFiles))
	options.ID = "base"
	m := startModel(t, options, 120, 30)

	m = pressKeys(t, m, runeKey('d'), specialKey(tea.KeyEnter))

	if m.tab != TabIssues || selectedIssueID(m) != "alpha" {
		t.Fatalf("after Enter: tab %v selecting %q, want the Issues tab selecting alpha", m.tab, selectedIssueID(m))
	}
	if !slices.Contains(visibleIDs(m), "epic") || m.deps.isOpen {
		t.Errorf("visible %v, dep view open %v; want epic shown and the view closed", visibleIDs(m), m.deps.isOpen)
	}
	if !strings.Contains(detailText(m), "Alpha's body.") {
		t.Errorf("detail pane = %q, want alpha's body", detailText(m))
	}
}

func TestDepView_EnterOnDoneDep_RevealsAndSelectsIt(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, depFiles), "alpha", 120, 30)

	m = pressKeys(t, m, runeKey('d'), runeKey('j'), specialKey(tea.KeyEnter))

	if got := selectedIssueID(m); got != "shipped" {
		t.Fatalf("selected %q, want shipped", got)
	}
	if got, want := mutedIDs(m), []string{"archive", "shipped"}; !slices.Equal(got, want) {
		t.Errorf("muted rows = %v, want %v", got, want)
	}
}

func TestDepView_EnterOnMissingDep_ShowsTheMessageAndKeepsTheSelection(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, depFiles), "alpha", 120, 30)

	m = pressKeys(t, m, runeKey('d'), runeKey('j'), runeKey('j'), specialKey(tea.KeyEnter))

	if !strings.Contains(statusRow(m), "ghost is missing") {
		t.Errorf("status bar = %q, want it to say ghost is missing", statusRow(m))
	}
	if m.tab != TabIssues || selectedIssueID(m) != "alpha" || !m.deps.isOpen {
		t.Errorf("tab %v selecting %q, dep view open %v; want the Issues tab on alpha, the view open",
			m.tab, selectedIssueID(m), m.deps.isOpen)
	}
}

func TestDepView_CloseKeys_ReturnToTheBody(t *testing.T) {
	cases := []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{name: "d", key: runeKey('d')},
		{name: "esc", key: specialKey(tea.KeyEscape)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newIssuesModel(t, newTestStore(t, depFiles), "alpha", 120, 30)

			m = pressKeys(t, m, runeKey('d'), tc.key)

			if m.deps.isOpen || !strings.Contains(detailText(m), "Alpha's body.") {
				t.Errorf("dep view open %v, detail pane %q; want it closed on alpha's body", m.deps.isOpen, detailText(m))
			}
		})
	}
}

func TestDepView_SidebarKey_ClosesTheViewBeforeMovingTheSelection(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, depFiles), "alpha", 120, 30)

	m = pressKeys(t, m, runeKey('d'), runeKey('h'))

	if got := selectedIssueID(m); got != "epic" {
		t.Fatalf("selected %q, want epic", got)
	}
	if m.deps.isOpen || strings.Contains(detailText(m), depsWaitsOnHeading) {
		t.Errorf("dep view open %v, detail pane %q; want it closed with no dependencies listed",
			m.deps.isOpen, detailText(m))
	}
	if !strings.Contains(detailText(m), "The epic.") {
		t.Errorf("detail pane = %q, want epic's body", detailText(m))
	}
}
