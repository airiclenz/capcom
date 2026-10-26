package ui

import (
	"maps"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// closedFiles is readyFiles, whose finished is done since 2026-10-12, with three more closed
// issues: scrapped, dropped at a time that is 2026-10-15 in UTC but 2026-10-14 where it was
// written, and zz and aa, both done at one time on 2026-10-13, which their ids order.
func closedFiles() map[string]string {
	files := maps.Clone(readyFiles)
	files["issues/scrapped.md"] = issueFile(
		"title: Scrapped task\nstatus: dropped\npriority: 2\nclosed: 2026-10-14T23:30:00-05:00\nreason: not needed\n",
		"Scrapped's body.\n",
	)
	files["issues/zz.md"] = issueFile(
		"title: ZZ task\nstatus: done\npriority: 2\nclosed: 2026-10-13T10:00:00Z\nreason: shipped\n",
		"",
	)
	files["issues/epic/aa.md"] = issueFile(
		"title: AA task\nstatus: done\npriority: 2\nclosed: 2026-10-13T10:00:00Z\nreason: shipped\n",
		"",
	)
	return files
}

// closedIDs returns the ids of the Closed tab's list, top to bottom.
func closedIDs(m model) []string {
	ids := make([]string, 0, len(m.closed.entries))
	for _, entry := range m.closed.entries {
		ids = append(ids, entry.ID)
	}
	return ids
}

// selectedClosedID returns the id of the Closed tab's selected issue, "" when it selects none.
func selectedClosedID(m model) string {
	entry, _ := m.closedSelection()
	return entry.ID
}

// newClosedModel returns a model over ts opened on the Closed tab, selecting id when it is not
// empty, as capcom ui closed [id] opens it.
func newClosedModel(t *testing.T, ts *testStore, id string) model {
	t.Helper()
	options := newTestOptions(t, ts)
	options.Tab = TabClosed
	options.IsTabGiven = true
	options.ID = id
	return startModel(t, options, wideWidth, wideHeight)
}

func TestClosedTab_Sidebar_ListsTheClosedIssuesNewestFirst(t *testing.T) {
	m := newClosedModel(t, newTestStore(t, closedFiles()), "")

	got := make([]string, 0, len(m.sidebar))
	for _, row := range m.sidebar {
		got = append(got, ansi.Strip(row))
	}

	want := []string{
		"2026-10-15  scrapped  Scrapped task",
		"2026-10-13  aa        AA task",
		"2026-10-13  zz        ZZ task",
		"2026-10-12  finished  Finished",
	}
	if !slices.Equal(got, want) {
		t.Errorf("Closed sidebar = %q, want %q", got, want)
	}
}

func TestClosedTab_Keys_OpenIt(t *testing.T) {
	cases := []struct {
		name string
		keys []tea.KeyPressMsg
	}{
		{name: "4", keys: []tea.KeyPressMsg{runeKey('4')}},
		{name: "tab twice from ready", keys: []tea.KeyPressMsg{specialKey(tea.KeyTab), specialKey(tea.KeyTab)}},
		{name: "1 then tab thrice", keys: []tea.KeyPressMsg{
			runeKey('1'), specialKey(tea.KeyTab), specialKey(tea.KeyTab), specialKey(tea.KeyTab),
		}},
	}
	ts := newTestStore(t, closedFiles())
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newTestModel(t, ts, wideWidth, wideHeight)

			m = pressKeys(t, m, testCase.keys...)

			if m.tab != TabClosed || selectedClosedID(m) != "scrapped" {
				t.Errorf("tab %v selecting %q, want closed selecting scrapped", m.tab, selectedClosedID(m))
			}
		})
	}
}

func TestClosedTab_TabKeyOnClosed_WrapsToIssues(t *testing.T) {
	m := newClosedModel(t, newTestStore(t, closedFiles()), "")

	m = press(t, m, specialKey(tea.KeyTab))

	if m.tab != TabIssues {
		t.Errorf("tab after tab on closed = %v, want issues", m.tab)
	}
}

func TestClosedTab_LabelClick_OpensIt(t *testing.T) {
	m := newTestModel(t, newTestStore(t, closedFiles()), wideWidth, wideHeight)

	m = press(t, m, click(tabColumn(t, m, TabClosed), 0))

	if m.tab != TabClosed {
		t.Errorf("tab after a click on the Closed label = %v, want closed", m.tab)
	}
}

func TestClosedTab_Label_CountsTheClosedIssues(t *testing.T) {
	m := newTestModel(t, newTestStore(t, closedFiles()), wideWidth, wideHeight)

	if got, want := m.tabBarLabel(TabClosed), " Closed [4] "; got != want {
		t.Errorf("Closed label = %q, want %q", got, want)
	}
}

func TestClosedTab_Detail_ShowsTheSelectedClosedIssue(t *testing.T) {
	m := newClosedModel(t, newTestStore(t, closedFiles()), "")

	text := detailText(m)

	if !strings.HasPrefix(text, "Scrapped task") || !strings.Contains(text, "Scrapped's body.") ||
		!strings.Contains(text, "not needed") {
		t.Errorf("detail = %q, want scrapped in full: title, close reason and body", text)
	}
}

func TestClosedTab_WorktreeState_RestoresTheTabAndItsSelection(t *testing.T) {
	ts := newTestStore(t, closedFiles())
	content := "ui:\n  tab: \"closed\"\n  selected:\n    closed: \"zz\"\n"

	m := startWithWorktreeState(t, ts, content, keepOptions)

	if m.tab != TabClosed || selectedClosedID(m) != "zz" {
		t.Errorf("tab %v selecting %q, want closed selecting zz", m.tab, selectedClosedID(m))
	}
	if got := m.worktreeSnapshot(); got.Tab != "closed" || got.Closed != "zz" {
		t.Errorf("worktree snapshot = %+v, want tab closed and closed zz", got)
	}
}

func TestClosedTab_TabChange_SavesTheTabAndItsSelection(t *testing.T) {
	ts := newTestStore(t, closedFiles())
	options := newTestOptions(t, ts)
	m := startModel(t, options, worktreeStateWidth, 30)

	m = press(t, m, runeKey('4'))

	stored := readWorktreeState(t, options.UIStatePath)
	if stored.Tab != "closed" || stored.Closed != "scrapped" {
		t.Errorf("stored state = %+v, want tab closed and closed scrapped", stored)
	}
}

func TestClosedTab_CloseOnReady_ListsTheIssueAndBumpsTheCount(t *testing.T) {
	m := newTestModel(t, newTestStore(t, closedFiles()), wideWidth, wideHeight)

	m = submitPrompt(t, m, 'c', "fixed in 3f2a1c9")
	m = press(t, m, runeKey('4'))

	if got := closedIDs(m); !slices.Contains(got, "alpha") {
		t.Errorf("Closed list after closing alpha = %v, want alpha on it", got)
	}
	if got, want := m.tabBarLabel(TabClosed), " Closed [5] "; got != want {
		t.Errorf("Closed label after closing alpha = %q, want %q", got, want)
	}
}

func TestClosedTab_OpenIssueOnTheCommandLine_NoticesAndSelectsTheFirstRow(t *testing.T) {
	m := newClosedModel(t, newTestStore(t, closedFiles()), "beta")

	if got, want := statusRow(m), "beta is not on the Closed tab"; !strings.Contains(got, want) {
		t.Errorf("status bar = %q, want %q", got, want)
	}
	if got := selectedClosedID(m); got != "scrapped" {
		t.Errorf("selected = %q, want the first row, scrapped", got)
	}
}

func TestClosedTab_Width80_RendersEveryTabLabelInFull(t *testing.T) {
	ts := newTestStore(t, closedFiles())
	options := newTestOptions(t, ts)
	options.Version = "1.2.3"

	m := startModel(t, options, 80, 30)

	tabBar := strings.Split(ansi.Strip(m.View().Content), "\n")[0]
	for _, tab := range tabOrder {
		if label := m.tabBarLabel(tab); !strings.Contains(tabBar, label) {
			t.Errorf("tab bar at width 80 = %q, want it to hold %q in full", tabBar, label)
		}
	}
}
