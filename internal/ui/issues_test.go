package ui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/outline"
)

// treeFiles is a store whose issue outline is epic (with alpha, holding leaf, and the closed
// shipped, holding the closed part), solo (aliased solo-alias) and wrapped, whose two children are
// both closed.
var treeFiles = map[string]string{
	"issues/epic.md":              issueFile("title: Epic\nstatus: open\npriority: 1\n", "The epic.\n"),
	"issues/epic/alpha.md":        issueFile("title: Alpha\nstatus: active\npriority: 2\n", "Alpha's body.\n"),
	"issues/epic/alpha/leaf.md":   issueFile("title: Leaf\nstatus: open\npriority: 3\n", ""),
	"issues/epic/shipped.md":      closedIssueFile("Shipped"),
	"issues/epic/shipped/part.md": closedIssueFile("Part"),
	"issues/solo.md":              issueFile("title: Solo\nstatus: open\npriority: 2\naliases: [solo-alias]\n", ""),
	"issues/wrapped.md":           issueFile("title: Wrapped\nstatus: open\npriority: 2\n", ""),
	"issues/wrapped/one.md":       closedIssueFile("One"),
	"issues/wrapped/two.md":       closedIssueFile("Two"),
}

// closedIssueFile returns the file of a done issue titled title.
func closedIssueFile(title string) string {
	return issueFile(
		"title: "+title+"\nstatus: done\npriority: 2\nclosed: 2026-10-12T09:00:00Z\nreason: shipped\n",
		"",
	)
}

// newIssuesModel returns a started model over the test store on the Issues tab, selecting id
// when it is not empty, sized width by height.
func newIssuesModel(t *testing.T, ts *testStore, id string, width, height int) model {
	t.Helper()
	options := newTestOptions(t, ts)
	options.Tab = TabIssues
	options.ID = id
	return startModel(t, options, width, height)
}

// visibleIDs returns the ids of the Issues tree's visible rows, top to bottom.
func visibleIDs(m model) []string {
	ids := make([]string, 0, len(m.issues.tree.rows))
	for _, row := range m.issues.tree.rows {
		ids = append(ids, row.ID)
	}
	return ids
}

// mutedIDs returns the ids of the Issues tree's muted rows, top to bottom.
func mutedIDs(m model) []string {
	var ids []string
	for _, row := range m.issues.tree.rows {
		if row.isMuted {
			ids = append(ids, row.ID)
		}
	}
	return ids
}

// selectedIssueID returns the id of the Issues tree's selected issue, "" when there is none.
func selectedIssueID(m model) string {
	entry, _ := m.issuesSelection()
	return entry.ID
}

// pressKeys hands keys to the model one after another, driving each one's command.
func pressKeys(t *testing.T, m model, keys ...tea.KeyPressMsg) model {
	t.Helper()
	for _, key := range keys {
		m = press(t, m, key)
	}
	return m
}

func TestIssuesTab_OpensCollapsed_MatchesGolden(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, treeFiles), "", 120, 30)

	golden(t, "issues-collapsed-120x30", m.View())
}

func TestIssuesTab_NestedExpanded_MatchesGolden(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, treeFiles), "", 120, 30)

	m = pressKeys(t, m, runeKey('l'), runeKey('j'), specialKey(tea.KeyRight))

	golden(t, "issues-expanded-120x30", m.View())
}

func TestIssuesTab_ParentWithEveryChildClosed_MatchesGolden(t *testing.T) {
	files := map[string]string{
		"issues/wrapped.md":     treeFiles["issues/wrapped.md"],
		"issues/wrapped/one.md": treeFiles["issues/wrapped/one.md"],
		"issues/wrapped/two.md": treeFiles["issues/wrapped/two.md"],
	}

	m := newIssuesModel(t, newTestStore(t, files), "", 150, 30)

	golden(t, "issues-children-closed-150x30", m.View())
}

func TestIssuesTab_FullyExpanded_RowOrderEqualsTheListTreeOutline(t *testing.T) {
	ts := newTestStore(t, treeFiles)
	entries, err := ts.index.List(index.Filter{IsClosedIncluded: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var want []string
	for _, row := range outline.Issues(entries, false).Rows(nil, outline.UnlimitedDepth) {
		want = append(want, row.ID)
	}
	m := newIssuesModel(t, ts, "", 120, 30)

	for position := 0; position < len(m.issues.tree.rows); position++ {
		m.issues.tree.cursor.seat(position, len(m.issues.tree.rows))
		m = press(t, m, runeKey('l'))
	}

	if got := visibleIDs(m); !slices.Equal(got, want) {
		t.Errorf("fully expanded tree = %v, want the outline of capcom list --tree %v", got, want)
	}
}

func TestIssuesTab_TreeKeys_ExpandCollapseAndMoveToTheParent(t *testing.T) {
	cases := []struct {
		name         string
		keys         []tea.KeyPressMsg
		wantVisible  []string
		wantSelected string
	}{
		{
			name:         "l expands",
			keys:         []tea.KeyPressMsg{runeKey('l')},
			wantVisible:  []string{"epic", "alpha", "solo", "wrapped"},
			wantSelected: "epic",
		},
		{
			name:         "right expands",
			keys:         []tea.KeyPressMsg{specialKey(tea.KeyRight)},
			wantVisible:  []string{"epic", "alpha", "solo", "wrapped"},
			wantSelected: "epic",
		},
		{
			name:         "h collapses",
			keys:         []tea.KeyPressMsg{runeKey('l'), runeKey('h')},
			wantVisible:  []string{"epic", "solo", "wrapped"},
			wantSelected: "epic",
		},
		{
			name:         "left on a child moves to its parent",
			keys:         []tea.KeyPressMsg{runeKey('l'), runeKey('j'), specialKey(tea.KeyLeft)},
			wantVisible:  []string{"epic", "alpha", "solo", "wrapped"},
			wantSelected: "epic",
		},
		{
			name:         "h on a top-level leaf stays",
			keys:         []tea.KeyPressMsg{runeKey('j'), runeKey('h')},
			wantVisible:  []string{"epic", "solo", "wrapped"},
			wantSelected: "solo",
		},
		{
			name:         "l on a parent whose children are all closed does nothing",
			keys:         []tea.KeyPressMsg{runeKey('j'), runeKey('j'), runeKey('l')},
			wantVisible:  []string{"epic", "solo", "wrapped"},
			wantSelected: "wrapped",
		},
	}
	ts := newTestStore(t, treeFiles)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newIssuesModel(t, ts, "", 120, 30)

			m = pressKeys(t, m, testCase.keys...)

			if got := visibleIDs(m); !slices.Equal(got, testCase.wantVisible) {
				t.Errorf("visible rows = %v, want %v", got, testCase.wantVisible)
			}
			if got := selectedIssueID(m); got != testCase.wantSelected {
				t.Errorf("selected = %q, want %q", got, testCase.wantSelected)
			}
			if text := detailText(m); !strings.Contains(text, "status: ") {
				t.Errorf("detail = %q, want the selected issue's header", text)
			}
		})
	}
}

func TestIssuesTab_TabSwitch_KeepsTheExpandedNodesAndTheSelection(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, treeFiles), "", 120, 30)
	m = pressKeys(t, m, runeKey('l'), runeKey('j'), runeKey('l'), runeKey('j'))

	m = pressKeys(t, m, runeKey('2'), specialKey(tea.KeyTab), specialKey(tea.KeyTab))

	if m.tab != TabIssues {
		t.Fatalf("tab after 2, tab, tab = %v, want Issues", m.tab)
	}
	if got, want := visibleIDs(m), []string{"epic", "alpha", "leaf", "solo", "wrapped"}; !slices.Equal(got, want) {
		t.Errorf("visible rows after the round trip = %v, want %v", got, want)
	}
	if got := selectedIssueID(m); got != "leaf" {
		t.Errorf("selected after the round trip = %q, want leaf", got)
	}
	if text := detailText(m); !strings.HasPrefix(text, "Leaf") {
		t.Errorf("detail after the round trip = %q, want leaf", text)
	}
}

func TestIssuesTab_OpenedOnAnID_SelectsItWithItsAncestorsExpanded(t *testing.T) {
	cases := []struct {
		name         string
		id           string
		wantVisible  []string
		wantSelected string
		wantNotice   string
	}{
		{name: "nested id", id: "leaf", wantVisible: []string{"epic", "alpha", "leaf", "solo", "wrapped"}, wantSelected: "leaf"},
		{name: "alias", id: "solo-alias", wantVisible: []string{"epic", "solo", "wrapped"}, wantSelected: "solo"},
		{
			name:         "unknown",
			id:           "no-such-id",
			wantVisible:  []string{"epic", "solo", "wrapped"},
			wantSelected: "epic",
			wantNotice:   "select no-such-id: ",
		},
	}
	ts := newTestStore(t, treeFiles)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newIssuesModel(t, ts, testCase.id, 120, 30)

			if got := visibleIDs(m); !slices.Equal(got, testCase.wantVisible) {
				t.Errorf("capcom ui issues %s: visible rows = %v, want %v", testCase.id, got, testCase.wantVisible)
			}
			if got := selectedIssueID(m); got != testCase.wantSelected {
				t.Errorf("capcom ui issues %s: selected = %q, want %q", testCase.id, got, testCase.wantSelected)
			}
			if !strings.Contains(statusRow(m), testCase.wantNotice) {
				t.Errorf("capcom ui issues %s: status bar %q, want %q", testCase.id, statusRow(m), testCase.wantNotice)
			}
		})
	}
}

func TestIssuesTab_OpenedOnAClosedID_RevealsItUntilTheSelectionMovesAway(t *testing.T) {
	ts := newTestStore(t, treeFiles)

	m := newIssuesModel(t, ts, "part", 120, 30)
	revealed := []string{"epic", "alpha", "shipped", "part", "solo", "wrapped"}
	if got := visibleIDs(m); !slices.Equal(got, revealed) {
		t.Fatalf("capcom ui issues part: visible rows = %v, want %v", got, revealed)
	}
	if got := mutedIDs(m); !slices.Equal(got, []string{"shipped", "part"}) {
		t.Errorf("capcom ui issues part: muted rows = %v, want shipped and part", got)
	}
	if text := detailText(m); selectedIssueID(m) != "part" || !strings.HasPrefix(text, "Part") {
		t.Errorf("capcom ui issues part: selected %q with detail %q, want part", selectedIssueID(m), text)
	}

	up := press(t, m, runeKey('k'))
	down := press(t, m, runeKey('j'))

	if got, want := visibleIDs(up), []string{"epic", "alpha", "shipped", "solo", "wrapped"}; !slices.Equal(got, want) ||
		selectedIssueID(up) != "shipped" {
		t.Errorf("k onto the closed parent: visible %v selecting %q, want %v selecting shipped", got, selectedIssueID(up), want)
	}
	if got, want := visibleIDs(down), []string{"epic", "alpha", "solo", "wrapped"}; !slices.Equal(got, want) ||
		selectedIssueID(down) != "solo" || len(mutedIDs(down)) != 0 {
		t.Errorf("j onto solo: visible %v selecting %q, want %v selecting solo, nothing muted", got, selectedIssueID(down), want)
	}
}
