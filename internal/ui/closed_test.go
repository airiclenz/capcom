package ui

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
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

// scrappedPath is the file of closedFiles' scrapped issue, relative to .capcom.
const scrappedPath = "issues/scrapped.md"

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

// storeFiles returns every file under ts's .capcom directory, keyed by its path relative to it.
func storeFiles(t *testing.T, ts *testStore) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(ts.store.Root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(ts.store.Root, path)
		files[relative] = string(content)
		return err
	})
	if err != nil {
		t.Fatalf("walk %s: %v", ts.store.Root, err)
	}
	return files
}

func TestClosedTab_CursorKeys_MoveTheSelection(t *testing.T) {
	cases := []struct {
		name string
		keys []tea.KeyPressMsg
		want string
	}{
		{name: "down", keys: []tea.KeyPressMsg{specialKey(tea.KeyDown)}, want: "aa"},
		{name: "j j", keys: []tea.KeyPressMsg{runeKey('j'), runeKey('j')}, want: "zz"},
		{name: "down then k", keys: []tea.KeyPressMsg{specialKey(tea.KeyDown), runeKey('k')}, want: "scrapped"},
	}
	ts := newTestStore(t, closedFiles())
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newClosedModel(t, ts, "")

			m = pressKeys(t, m, testCase.keys...)

			if got := selectedClosedID(m); got != testCase.want {
				t.Errorf("selected = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestClosedTab_MouseClickAndWheel_MoveTheSelection(t *testing.T) {
	cases := []struct {
		name  string
		event tea.Msg
		want  string
	}{
		{name: "click on the second row", event: click(3, 3), want: "aa"},
		{name: "wheel down", event: wheel(tea.MouseWheelDown, 3, 6), want: "aa"},
	}
	ts := newTestStore(t, closedFiles())
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newClosedModel(t, ts, "")

			m = press(t, m, testCase.event)

			if got := selectedClosedID(m); got != testCase.want {
				t.Errorf("selected = %q, want %q", got, testCase.want)
			}
			if got := detailText(m); !strings.HasPrefix(got, "AA task") {
				t.Errorf("detail = %q, want aa shown", got)
			}
		})
	}
}

func TestClosedTab_R_ReopensAndMovesTheIssueToIssuesAndReady(t *testing.T) {
	m := newClosedModel(t, newTestStore(t, closedFiles()), "")

	m = submitPrompt(t, m, 'R', "not done after all")

	if got := closedIDs(m); slices.Contains(got, "scrapped") {
		t.Errorf("Closed list after R = %v, want scrapped gone", got)
	}
	if !slices.Contains(readyIDs(m), "scrapped") {
		t.Errorf("Ready queue after R = %v, want scrapped on it", readyIDs(m))
	}
	if !m.issues.tree.isShown["scrapped"] {
		t.Errorf("Issues tree after R does not show scrapped")
	}
	wantLabels := map[Tab]string{
		TabIssues: " Issues [6] ",
		TabReady:  " Ready [4] ",
		TabClosed: " Closed [3] ",
	}
	for tab, want := range wantLabels {
		if got := m.tabBarLabel(tab); got != want {
			t.Errorf("label after R = %q, want %q", got, want)
		}
	}
}

func TestClosedTab_WriteKeysOtherThanR_DoNothing(t *testing.T) {
	for _, key := range []rune{'a', 'c', 's', 'L'} {
		t.Run(string(key), func(t *testing.T) {
			ts := newTestStore(t, closedFiles())
			m := newClosedModel(t, ts, "")
			before := storeFiles(t, ts)

			m = press(t, m, runeKey(key))

			if m.textPrompt.isOpen || m.picker.isOpen {
				t.Errorf("after %c: text prompt %v, picker %v; want neither open",
					key, m.textPrompt.isOpen, m.picker.isOpen)
			}
			if got := storeFiles(t, ts); !maps.Equal(got, before) {
				t.Errorf("store after %c changed; want it unchanged", key)
			}
			if got := string(readStoreFile(t, ts, scrappedPath)); got != before[filepath.FromSlash(scrappedPath)] {
				t.Errorf("scrapped after %c:\n%s\nwant its bytes unchanged", key, got)
			}
		})
	}
}

func TestClosedTab_Filter_NarrowsTheList(t *testing.T) {
	files := closedFiles()
	files["issues/zz.md"] = issueFile(
		"title: ZZ task\nstatus: done\npriority: 2\nlabels: [ui]\nclosed: 2026-10-13T10:00:00Z\nreason: shipped\n",
		"",
	)
	cases := []struct {
		name string
		text string
		want []string
	}{
		{name: "word", text: "Scrapped", want: []string{"scrapped"}},
		{name: "label", text: "label:ui", want: []string{"zz"}},
		{name: "closed status", text: "status:done", want: []string{"aa", "zz", "finished"}},
	}
	ts := newTestStore(t, files)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newClosedModel(t, ts, "")

			m = applyFilterText(t, m, testCase.text)

			if got := closedIDs(m); !slices.Equal(got, testCase.want) {
				t.Errorf("Closed list under %q = %v, want %v", testCase.text, got, testCase.want)
			}
		})
	}
}

func TestClosedTab_OpenStatusFilter_ShowsTheProblemRow(t *testing.T) {
	m := newClosedModel(t, newTestStore(t, closedFiles()), "")

	m = applyFilterText(t, m, "status:open")

	want := []string{"status: on the Closed tab takes a closed status"}
	if got := sidebarText(m); !slices.Equal(got, want) {
		t.Errorf("Closed sidebar under status:open = %q, want %q", got, want)
	}
}

func TestClosedTab_D_OpensTheDependencyView(t *testing.T) {
	m := newClosedModel(t, newTestStore(t, closedFiles()), "")

	m = press(t, m, runeKey('d'))

	if !m.deps.isOpen || m.deps.id != "scrapped" {
		t.Errorf("deps view open %v over %q, want it open over scrapped", m.deps.isOpen, m.deps.id)
	}
}

func TestClosedTab_E_EditsTheSelectedIssuesFile(t *testing.T) {
	ts := newTestStore(t, closedFiles())
	editor := newFakeEditor(t, "Scrapped's body.", "Scrapped's edited body.")
	m := startEditModel(t, ts, TabClosed, editor)

	m = press(t, m, runeKey('e'))

	wantArgs := []string{"--flag", filepath.Join(ts.store.Root, filepath.FromSlash(scrappedPath))}
	if got := editor.args(t); !slices.Equal(got, wantArgs) {
		t.Errorf("editor args = %q, want %q", got, wantArgs)
	}
	if got := detailText(m); !strings.Contains(got, "Scrapped's edited body.") {
		t.Errorf("detail pane after e:\n%s\nwant the edited body", got)
	}
}
