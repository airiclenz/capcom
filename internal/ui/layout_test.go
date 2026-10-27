package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/airiclenz/capcom/internal/lock"
	"github.com/airiclenz/capcom/internal/userconfig"
)

// wideStatusWidth returns a terminal width that fits a status bar naming path in full: temp-dir
// paths run past 150 columns on macOS.
func wideStatusWidth(path string) int {
	return 200 + len(path)
}

func TestModel_View_EmptyLayout_MatchesGolden(t *testing.T) {
	cases := []struct {
		name   string
		width  int
		height int
	}{
		{name: "layout-120x30", width: 120, height: 30},
		{name: "layout-80x24", width: 80, height: 24},
		{name: "layout-80x40", width: 80, height: 40},
	}
	ts := newTestStore(t, map[string]string{"issues/valid-issue.md": validIssue})
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newTestModel(t, ts, testCase.width, testCase.height)

			view := m.View()

			if !view.AltScreen {
				t.Errorf("View: AltScreen is off, want the alternate screen")
			}
			golden(t, testCase.name, view)
		})
	}
}

func TestModel_View_HelpScreen_MatchesGolden(t *testing.T) {
	m := newTestModel(t, newTestStore(t, nil), 120, 30)

	m = send(t, m, runeKey('?'))

	golden(t, "help-120x30", m.View())
}

func TestModel_View_HelpScreen_FitsTheSmallTerminal(t *testing.T) {
	m := newTestModel(t, newTestStore(t, nil), smallWidth, smallHeight)

	m = send(t, m, runeKey('?'))

	if len(helpLines) > m.bodyHeight() {
		t.Fatalf("helpLines holds %d rows, want at most the %d body rows at %dx%d",
			len(helpLines), m.bodyHeight(), smallWidth, smallHeight)
	}
	view := ansi.Strip(m.View().Content)
	for _, line := range helpLines {
		if !strings.Contains(view, line) {
			t.Errorf("help screen at %dx%d lacks the row %q", smallWidth, smallHeight, line)
		}
	}
}

func TestModel_HelpKeys_ToggleTheHelpScreen(t *testing.T) {
	cases := []struct {
		name  string
		close tea.KeyPressMsg
	}{
		{name: "question mark", close: runeKey('?')},
		{name: "esc", close: specialKey(tea.KeyEscape)},
	}
	ts := newTestStore(t, nil)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := send(t, newTestModel(t, ts, 120, 30), runeKey('?'))
			opened := m
			m = send(t, m, specialKey(tea.KeyTab))
			isTabIgnored := m.tab == TabReady

			m = send(t, m, testCase.close)

			if !opened.isHelpOpen || !strings.Contains(statusRow(opened), hintsHelp) {
				t.Errorf("? did not open the help screen: status bar %q", statusRow(opened))
			}
			if !isTabIgnored {
				t.Errorf("Tab under the help screen switched to %v, want it ignored", m.tab)
			}
			if m.isHelpOpen {
				t.Errorf("%s left the help screen open", testCase.name)
			}
		})
	}
}

func TestModel_TabKeys_SwitchTabs(t *testing.T) {
	cases := []struct {
		name string
		keys []tea.KeyPressMsg
		want Tab
	}{
		{name: "tab once", keys: []tea.KeyPressMsg{specialKey(tea.KeyTab)}, want: TabKnowledge},
		{
			name: "tab wraps",
			keys: []tea.KeyPressMsg{specialKey(tea.KeyTab), specialKey(tea.KeyTab), specialKey(tea.KeyTab)},
			want: TabIssues,
		},
		{name: "1", keys: []tea.KeyPressMsg{runeKey('1')}, want: TabIssues},
		{name: "2", keys: []tea.KeyPressMsg{runeKey('3'), runeKey('2')}, want: TabReady},
		{name: "3", keys: []tea.KeyPressMsg{runeKey('3')}, want: TabKnowledge},
	}
	ts := newTestStore(t, nil)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newTestModel(t, ts, 120, 30)

			for _, key := range testCase.keys {
				m = send(t, m, key)
			}

			if m.tab != testCase.want {
				t.Errorf("tab = %v, want %v", m.tab, testCase.want)
			}
		})
	}
}

func TestModel_NarrowLayout_EnterOpensDetailAndEscReturns(t *testing.T) {
	m := newTestModel(t, newTestStore(t, nil), 80, 24)

	opened := send(t, m, specialKey(tea.KeyEnter))
	returned := send(t, opened, specialKey(tea.KeyEscape))

	if !opened.isDetailOpen || !strings.Contains(statusRow(opened), hintsDetail) {
		t.Errorf("Enter on a narrow terminal: status bar %q, want the detail view's hints", statusRow(opened))
	}
	if returned.isDetailOpen || !strings.Contains(statusRow(returned), hintsPanes) {
		t.Errorf("Esc in the detail view: status bar %q, want the sidebar back", statusRow(returned))
	}
}

func TestModel_WideLayout_EnterKeepsBothPanes(t *testing.T) {
	m := newTestModel(t, newTestStore(t, nil), 120, 30)

	m = send(t, m, specialKey(tea.KeyEnter))

	if m.isDetailOpen {
		t.Errorf("Enter on a wide terminal opened the narrow detail view")
	}
}

func TestModel_SplitLayout_PadsTheDividerWithAMarginEachSide(t *testing.T) {
	m := newTestModel(t, newTestStore(t, readyFiles), 120, 30)
	sidebarWidth := m.sidebarWidth()

	rows := strings.Split(ansi.Strip(m.View().Content), "\n")

	for row := bodyTop; row < bodyTop+m.bodyHeight(); row++ {
		if got := ansi.Cut(rows[row], sidebarWidth, sidebarWidth+3); got != " │ " {
			t.Errorf("row %d holds %q at columns %d..%d, want \" │ \"", row, got, sidebarWidth, sidebarWidth+2)
		}
	}
	rule := rows[len(rows)-2]
	if got := ansi.Cut(rule, sidebarWidth+1, sidebarWidth+2); got != ruleJoint {
		t.Errorf("the rule holds %q at column %d, want %q under the separator", got, sidebarWidth+1, ruleJoint)
	}
	if got, want := m.detailWidth(), 120-sidebarWidth-3-scrollbarColumns; got != want {
		t.Errorf("detailWidth() = %d, want %d: the width right of the divider, less the scrollbar", got, want)
	}
}

func TestModel_TerminalSize_PicksTheLayout(t *testing.T) {
	cases := []struct {
		name        string
		width       int
		height      int
		wantStacked bool
		wantSplit   bool
	}{
		{name: "80x24 one pane", width: 80, height: 24},
		{name: "80x25 stacked", width: 80, height: 25, wantStacked: true},
		{name: "80x40 stacked", width: 80, height: 40, wantStacked: true},
		{name: "120x40 split", width: 120, height: 40, wantSplit: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newTestModel(t, newTestStore(t, readyFiles), testCase.width, testCase.height)

			rows := strings.Split(ansi.Strip(m.View().Content), "\n")

			stackedRule := rows[bodyTop+m.stackedSidebarRows()]
			isStackedRule := stackedRule == strings.Repeat(ruleGlyph, testCase.width)
			if isStackedRule != testCase.wantStacked {
				t.Errorf("row %d is the stacked divider %v, want %v: %q",
					bodyTop+m.stackedSidebarRows(), isStackedRule, testCase.wantStacked, stackedRule)
			}
			column := m.sidebarWidth()
			isSplitRow := ansi.Cut(rows[bodyTop], column, column+3) == " │ "
			if isSplitRow != testCase.wantSplit {
				t.Errorf("row %d holds the split separator %v, want %v", bodyTop, isSplitRow, testCase.wantSplit)
			}
		})
	}
}

func TestModel_TerminalResize_SwitchesBetweenOnePaneAndStacked(t *testing.T) {
	ts := newTestStore(t, readyFiles)
	m := newTestModel(t, ts, 80, 24)
	onePane := m.View().Content
	stacked := newTestModel(t, ts, 80, 40).View().Content

	m = send(t, m, tea.WindowSizeMsg{Width: 80, Height: 40})
	gotStacked := m.View().Content
	m = send(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})

	if gotStacked != stacked {
		t.Errorf("80x24 resized to 80x40:\n%s\nwant the stacked layout\n%s", gotStacked, stacked)
	}
	if got := m.View().Content; got != onePane {
		t.Errorf("resized back to 80x24:\n%s\nwant the one-pane layout\n%s", got, onePane)
	}
}

func TestModel_StoredLayoutChoice_PicksTheLayoutOnlyWhenBothFit(t *testing.T) {
	cases := []struct {
		name        string
		choice      string
		width       int
		height      int
		wantStacked bool
		wantSplit   bool
	}{
		{name: "120x30 stacked", choice: userconfig.LayoutStacked, width: 120, height: 30, wantStacked: true},
		{name: "120x24 stacked, body too short", choice: userconfig.LayoutStacked, width: 120, height: 24, wantSplit: true},
		{name: "120x30 split", choice: userconfig.LayoutSplit, width: 120, height: 30, wantSplit: true},
		{name: "80x40 split, too narrow", choice: userconfig.LayoutSplit, width: 80, height: 40, wantStacked: true},
		{name: "80x24 stacked, one pane", choice: userconfig.LayoutStacked, width: 80, height: 24},
		{name: "80x24 split, one pane", choice: userconfig.LayoutSplit, width: 80, height: 24},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			options := newTestOptions(t, newTestStore(t, readyFiles))
			writeTestFile(t, options.UserStatePath, layoutState(testCase.choice))

			m := startModel(t, options, testCase.width, testCase.height)

			if got := hasStackedDivider(m); got != testCase.wantStacked {
				t.Errorf("stacked divider on screen = %v, want %v", got, testCase.wantStacked)
			}
			if got := hasSplitSeparator(m); got != testCase.wantSplit {
				t.Errorf("split separator on screen = %v, want %v", got, testCase.wantSplit)
			}
			isFullWidth := m.detailWidth() == testCase.width-scrollbarColumns
			if isFullWidth == testCase.wantSplit {
				t.Errorf("detailWidth() = %d at %d columns: full width %v, want %v",
					m.detailWidth(), testCase.width, isFullWidth, !testCase.wantSplit)
			}
		})
	}
}

func TestModel_TerminalResize_KeepsTheStoredLayoutChoiceUnsaved(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, readyFiles))
	writeTestFile(t, options.UserStatePath, layoutState(userconfig.LayoutStacked))
	m := startModel(t, options, 120, 30)
	stacked := m.View().Content
	if err := os.Remove(options.UserStatePath); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	m = press(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	isOnePane := m.isSinglePane()
	m = press(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})

	if !isOnePane {
		t.Errorf("resized to 80x24: want the one-pane layout")
	}
	if got := m.View().Content; got != stacked {
		t.Errorf("resized back to 120x30:\n%s\nwant the stacked layout\n%s", got, stacked)
	}
	if _, err := os.Stat(options.UserStatePath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("state file after the resizes: Stat error %v, want it not written", err)
	}
}

func TestModel_DividerSave_WritesTheLayoutChoiceHeld(t *testing.T) {
	cases := []struct {
		name   string
		choice string
	}{
		{name: "stacked", choice: userconfig.LayoutStacked},
		{name: "none", choice: ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			options := newTestOptions(t, newTestStore(t, readyFiles))
			if testCase.choice != "" {
				writeTestFile(t, options.UserStatePath, layoutState(testCase.choice))
			}
			m := startModel(t, options, 120, 30)

			press(t, m, runeKey('>'))

			content, err := os.ReadFile(options.UserStatePath)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			wantLine := "layout: " + testCase.choice
			hasLayoutKey := strings.Contains(string(content), "layout:")
			if testCase.choice == "" && hasLayoutKey {
				t.Errorf("state file = %q, want no layout key", content)
			}
			if testCase.choice != "" && !strings.Contains(string(content), wantLine) {
				t.Errorf("state file = %q, want %q", content, wantLine)
			}
		})
	}
}

// layoutState returns a state file holding only the layout choice.
func layoutState(choice string) string {
	return "ui:\n  layout: " + choice + "\n"
}

// hasStackedDivider reports the stacked layout's divider on the model's screen: a rule across the
// whole row under the stacked sidebar.
func hasStackedDivider(m model) bool {
	rows := strings.Split(ansi.Strip(m.View().Content), "\n")
	return rows[bodyTop+m.stackedSidebarRows()] == strings.Repeat(ruleGlyph, m.width)
}

// hasSplitSeparator reports the split layout's separator, with its margins, on the model's first
// body row.
func hasSplitSeparator(m model) bool {
	rows := strings.Split(ansi.Strip(m.View().Content), "\n")
	column := m.sidebarWidth()
	return ansi.Cut(rows[bodyTop], column, column+3) == " │ "
}

func TestModel_StackedLayout_EnterKeepsBothPanes(t *testing.T) {
	m := newTestModel(t, newTestStore(t, readyFiles), 80, 40)
	before := m.View().Content

	m = send(t, m, specialKey(tea.KeyEnter))

	if m.isDetailOpen {
		t.Errorf("Enter at 80x40 opened the one-pane detail view")
	}
	if got := m.View().Content; got != before {
		t.Errorf("Enter at 80x40 changed the screen:\n%s\nwant\n%s", got, before)
	}
}

func TestModel_ResizeKeys_LeaveTheSidebarRatioUnwrittenInTheStackedLayout(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, readyFiles))
	m := startModel(t, options, 80, 40)

	for _, key := range []rune{'>', '<'} {
		m = press(t, m, runeKey(key))
	}

	if m.sidebarRatio != 0 {
		t.Errorf("sidebarRatio = %v, want 0: no width set in the stacked layout", m.sidebarRatio)
	}
	if got := readStoredRatio(t, options.UserStatePath); got != 0 {
		t.Errorf("stored sidebar ratio = %v, want none written", got)
	}
}

func TestModel_TabRule_JoinsTheDividerOnlyInTheSplitLayout(t *testing.T) {
	cases := []struct {
		name      string
		width     int
		height    int
		keys      []tea.KeyPressMsg
		wantJoint bool
	}{
		{name: "split", width: 120, height: 30, wantJoint: true},
		{name: "one pane", width: 80, height: 24},
		{name: "stacked", width: 80, height: 40},
		{name: "help", width: 120, height: 30, keys: []tea.KeyPressMsg{runeKey('?')}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newTestModel(t, newTestStore(t, readyFiles), testCase.width, testCase.height)
			for _, key := range testCase.keys {
				m = send(t, m, key)
			}

			rule := strings.Split(ansi.Strip(m.View().Content), "\n")[1]

			want := strings.Repeat(ruleGlyph, testCase.width)
			if testCase.wantJoint {
				column := m.dividerColumn()
				want = strings.Repeat(ruleGlyph, column) + topRuleJoint + strings.Repeat(ruleGlyph, testCase.width-column-1)
			}
			if rule != want {
				t.Errorf("row 1 = %q, want %q", rule, want)
			}
		})
	}
}

func TestModel_TabBar_EndsInTheWidestLabelThatFits(t *testing.T) {
	tabs := strippedTabs(newTestModel(t, newTestStore(t, readyFiles), wideWidth, wideHeight))
	versionLabel := versionLabelPrefix + testVersion + versionLabelSuffix
	pathLabel := homeLabel + pathLabelSeparator + versionLabel
	pathFitWidth := ansi.StringWidth(tabs) + versionLabelMinGap + ansi.StringWidth(pathLabel)
	versionFitWidth := ansi.StringWidth(tabs) + versionLabelMinGap + ansi.StringWidth(versionLabel)
	cases := []struct {
		name      string
		width     int
		version   string
		wantLabel string
	}{
		{name: "path label at 120", width: wideWidth, version: testVersion, wantLabel: pathLabel},
		{name: "path label with the least gap", width: pathFitWidth, version: testVersion, wantLabel: pathLabel},
		{name: "one column short of the path label", width: pathFitWidth - 1, version: testVersion, wantLabel: versionLabel},
		{name: "version label with the least gap", width: versionFitWidth, version: testVersion, wantLabel: versionLabel},
		{name: "one column short of the version label", width: versionFitWidth - 1, version: testVersion},
		{name: "no version", width: wideWidth},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			options := newTestOptions(t, newTestStore(t, readyFiles))
			options.Version = testCase.version
			options.Home = options.Store.WorktreeRoot
			m := startModel(t, options, testCase.width, wideHeight)

			tabBar := strings.Split(ansi.Strip(m.View().Content), "\n")[0]

			want := tabs
			if testCase.wantLabel != "" {
				gap := testCase.width - ansi.StringWidth(tabs) - ansi.StringWidth(testCase.wantLabel)
				want = tabs + strings.Repeat(" ", gap) + testCase.wantLabel
			}
			if tabBar != want {
				t.Errorf("row 0 = %q, want %q", tabBar, want)
			}
			if testCase.wantLabel != "" && ansi.StringWidth(tabBar) != testCase.width {
				t.Errorf("row 0 is %d columns wide, want %d", ansi.StringWidth(tabBar), testCase.width)
			}
		})
	}
}

func TestDisplayPath_ShortensOnlyAPathUnderHome(t *testing.T) {
	home := filepath.FromSlash("/home/user")
	cases := []struct {
		name string
		root string
		home string
		want string
	}{
		{name: "under home", root: filepath.FromSlash("/home/user/repos/capcom"), home: home, want: filepath.FromSlash("~/repos/capcom")},
		{name: "equal to home", root: home, home: home, want: homeLabel},
		{name: "sibling sharing the prefix", root: filepath.FromSlash("/home/userx/capcom"), home: home, want: filepath.FromSlash("/home/userx/capcom")},
		{name: "outside home", root: filepath.FromSlash("/srv/capcom"), home: home, want: filepath.FromSlash("/srv/capcom")},
		{name: "empty home", root: filepath.FromSlash("/home/user/capcom"), home: "", want: filepath.FromSlash("/home/user/capcom")},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := displayPath(testCase.root, testCase.home)

			if got != testCase.want {
				t.Errorf("displayPath(%q, %q) = %q, want %q", testCase.root, testCase.home, got, testCase.want)
			}
		})
	}
}

// tabBarRow returns the view's top row, its styling stripped: the tab bar.
func tabBarRow(m model) string {
	return strings.Split(ansi.Strip(m.View().Content), "\n")[0]
}

func TestModel_TabBar_CountsWhatEachTabLists(t *testing.T) {
	// filterFiles lists six open issues (shipped and one are done), three Ready ones and four
	// Notes outside the background zeta.
	m := newTestModel(t, newTestStore(t, filterFiles), wideWidth, wideHeight)

	got := tabBarRow(m)

	want := " Issues [6]  │  Ready [3]  │  Knowledge [4] "
	if !strings.HasPrefix(got, want) {
		t.Errorf("row 0 = %q, want it to start %q", got, want)
	}
}

func TestModel_TabBar_NoCountWithoutAGoodRead(t *testing.T) {
	cases := []struct {
		name string
		read func(m model) model
	}{
		{name: "before the first read", read: func(m model) model { return m }},
		{
			name: "after a failed read",
			read: func(m model) model {
				failure := errors.New("index unreadable")
				m = send(t, m, readyMsg{err: failure})
				m = send(t, m, issuesMsg{err: failure})
				return send(t, m, knowledgeMsg{err: failure})
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newModel(newTestOptions(t, newTestStore(t, filterFiles)))
			m = send(t, m, tea.WindowSizeMsg{Width: wideWidth, Height: wideHeight})

			m = testCase.read(m)

			want := " Issues  │  Ready  │  Knowledge "
			if got := tabBarRow(m); !strings.HasPrefix(got, want) {
				t.Errorf("row 0 = %q, want it to start %q", got, want)
			}
		})
	}
}

func TestModel_TabBar_CountsFollowCloseAndCreate(t *testing.T) {
	ts := newTestStore(t, readyFiles)
	m := newTestModel(t, ts, wideWidth, wideHeight)
	if got, want := strippedTabs(m), " Issues [5]  │  Ready [3]  │  Knowledge [0]  │  Closed [1] "; got != want {
		t.Fatalf("tabs at start = %q, want %q", got, want)
	}

	m = submitPrompt(t, m, 'c', "fixed in 3f2a1c9")

	if got, want := strippedTabs(m), " Issues [4]  │  Ready [2]  │  Knowledge [0]  │  Closed [2] "; got != want {
		t.Errorf("tabs after closing alpha = %q, want %q", got, want)
	}

	writeStoreFile(t, ts, "issues/fresh.md", issueFile("title: Fresh task\nstatus: open\npriority: 2\n", ""))
	m = tick(t, m)

	if got, want := strippedTabs(m), " Issues [5]  │  Ready [3]  │  Knowledge [0]  │  Closed [2] "; got != want {
		t.Errorf("tabs after creating fresh = %q, want %q", got, want)
	}
}

// strippedTabs returns m's tab bar tabs as they read without styling, their counts included,
// before any version label.
func strippedTabs(m model) string {
	labels := make([]string, 0, len(tabOrder))
	for _, tab := range tabOrder {
		labels = append(labels, m.tabBarLabel(tab))
	}
	return strings.Join(labels, tabSeparator)
}

func TestModel_BodyHeight_LeavesTheFrameRows(t *testing.T) {
	m := newTestModel(t, newTestStore(t, nil), 120, 30)

	got := m.bodyHeight()

	if want := 30 - 4; got != want {
		t.Errorf("bodyHeight() = %d, want %d: the screen less the tab bar, two rules and the status bar", got, want)
	}
}

func TestModel_SidebarWidth_DefaultsToAThirdWithinItsBounds(t *testing.T) {
	cases := []struct {
		width int
		want  int
	}{
		{width: 90, want: 30},
		{width: 120, want: 40},
		{width: 200, want: sidebarMaxWidth},
	}
	ts := newTestStore(t, nil)
	for _, testCase := range cases {
		t.Run(fmt.Sprintf("width %d", testCase.width), func(t *testing.T) {
			m := newTestModel(t, ts, testCase.width, 30)

			if got := m.sidebarWidth(); got != testCase.want {
				t.Errorf("sidebarWidth() = %d, want %d", got, testCase.want)
			}
		})
	}
}

func TestModel_ResizeKeys_MoveTheDividerByAStep(t *testing.T) {
	cases := []struct {
		name  string
		key   rune
		delta int
	}{
		{name: "grow", key: '>', delta: sidebarStep},
		{name: "shrink", key: '<', delta: -sidebarStep},
	}
	ts := newTestStore(t, readyFiles)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newTestModel(t, ts, 120, 30)
			before := m.dividerColumn()

			m = send(t, m, runeKey(testCase.key))

			if got, want := m.dividerColumn(), before+testCase.delta; got != want {
				t.Errorf("dividerColumn() = %d, want %d", got, want)
			}
			assertDividerAt(t, m)
		})
	}
}

func TestModel_ResizeKeys_StopAtTheLimits(t *testing.T) {
	const width = 120
	cases := []struct {
		name string
		key  rune
		want int
	}{
		{name: "shrink", key: '<', want: sidebarDragMinWidth},
		{name: "grow", key: '>', want: width - 2*dividerMargin - 1 - detailMinWidth},
	}
	ts := newTestStore(t, readyFiles)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newTestModel(t, ts, width, 30)

			for _, key := range repeatKey(runeKey(testCase.key), 20) {
				m = send(t, m, key)
			}

			if got := m.sidebarWidth(); got != testCase.want {
				t.Errorf("sidebarWidth() = %d, want %d", got, testCase.want)
			}
			if got := m.detailLeft() + m.detailWidth() + scrollbarColumns; got != width {
				t.Errorf("the detail pane ends at column %d, want the right edge %d", got, width)
			}
			assertDividerAt(t, m)
		})
	}
}

func TestModel_StoredSidebarRatio_ClampsWithoutChangingIt(t *testing.T) {
	cases := []struct {
		width int
		want  int
	}{
		{width: 100, want: 57},
		{width: 300, want: 240},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d columns", tc.width), func(t *testing.T) {
			m := newTestModel(t, newTestStore(t, nil), 120, 30)
			m.sidebarRatio = 0.8

			m = send(t, m, tea.WindowSizeMsg{Width: tc.width, Height: 30})

			if got := m.sidebarWidth(); got != tc.want {
				t.Errorf("sidebarWidth() = %d, want %d: a stored 0.8 at %d columns", got, tc.want, tc.width)
			}
			if m.sidebarRatio != 0.8 {
				t.Errorf("sidebarRatio = %v, want the stored 0.8 unchanged by the clamp", m.sidebarRatio)
			}
		})
	}
}

func TestModel_TerminalWidthChanges_KeepTheSidebarRatio(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, nil))
	m := startModel(t, options, 120, 30)
	m = press(t, m, runeKey('>'))
	ratio := m.sidebarRatio
	asked := m.stateSaver.asked

	for _, step := range []struct{ width, want int }{{240, 88}, {90, 33}, {120, 44}} {
		m = press(t, m, tea.WindowSizeMsg{Width: step.width, Height: 30})

		if got := m.sidebarWidth(); got != step.want {
			t.Errorf("sidebarWidth() at %d columns = %d, want %d", step.width, got, step.want)
		}
		assertDividerAt(t, m)
	}
	if m.sidebarRatio != ratio {
		t.Errorf("sidebarRatio = %v, want %v unchanged by the resizes", m.sidebarRatio, ratio)
	}
	if m.stateSaver.asked != asked {
		t.Errorf("saves asked = %d, want %d: a resize saves nothing", m.stateSaver.asked, asked)
	}
}

func TestModel_TerminalHeightChanges_LeaveTheDivider(t *testing.T) {
	m := newTestModel(t, newTestStore(t, nil), 120, 30)
	m = press(t, m, runeKey('>'))
	before := m.dividerColumn()

	m = press(t, m, tea.WindowSizeMsg{Width: 120, Height: 50})

	if got := m.dividerColumn(); got != before {
		t.Errorf("dividerColumn() = %d, want it left at %d", got, before)
	}
}

func TestModel_ResizeKeys_DoNothingOnANarrowTerminal(t *testing.T) {
	m := newTestModel(t, newTestStore(t, readyFiles), 80, 24)
	before := m.View().Content

	for _, key := range []rune{'>', '<', '<'} {
		m = send(t, m, runeKey(key))
	}

	if m.sidebarRatio != 0 {
		t.Errorf("sidebarRatio = %v, want 0: no width set on a narrow terminal", m.sidebarRatio)
	}
	if got := m.View().Content; got != before {
		t.Errorf("the screen changed on a narrow terminal:\n%s\nwant\n%s", got, before)
	}
}

func TestModel_ResizeKeys_ResizeTheFilterLine(t *testing.T) {
	m := newTestModel(t, newTestStore(t, readyFiles), 120, 30)

	m = send(t, m, runeKey('>'))

	want := m.filterLine
	want.setWidth(m.sidebarPaneWidth())
	if got := m.filterLine.input.Width(); got != want.input.Width() {
		t.Errorf("filter line width = %d, want %d: the grown sidebar's", got, want.input.Width())
	}
}

func TestModel_ResizeKeys_MouseFollowsTheNewWidth(t *testing.T) {
	m := newTestModel(t, newTestStore(t, readyFiles), 120, 30)
	oldDetailLeft := m.detailLeft()
	m = pressKeys(t, m, runeKey('>'), runeKey('>'))

	if got := m.pointAt(oldDetailLeft, 3).pane; got != paneSidebar {
		t.Errorf("pointAt(%d, 3) is pane %v, want the grown sidebar", oldDetailLeft, got)
	}
	m = press(t, m, click(oldDetailLeft, 3))

	if got, isSelected := m.readySelection(); !isSelected || got.ID != "beta" {
		t.Errorf("selected %q, want beta under the click", got.ID)
	}
	if got := m.pointAt(m.detailLeft(), 3); got.pane != paneDetail || got.col != 0 {
		t.Errorf("pointAt(detailLeft) = %+v, want the detail pane's first column", got)
	}
}

// assertDividerAt checks the split layout's separator " │ " on every body row right of the
// sidebar's width, and both rules' joints under it.
func assertDividerAt(t *testing.T, m model) {
	t.Helper()
	sidebarWidth := m.sidebarWidth()
	rows := strings.Split(ansi.Strip(m.View().Content), "\n")
	for row := bodyTop; row < bodyTop+m.bodyHeight(); row++ {
		if got := ansi.Cut(rows[row], sidebarWidth, sidebarWidth+3); got != " │ " {
			t.Errorf("row %d holds %q at columns %d..%d, want \" │ \"", row, got, sidebarWidth, sidebarWidth+2)
		}
	}
	column := m.dividerColumn()
	for _, rule := range []struct {
		row   int
		joint string
	}{{row: 1, joint: topRuleJoint}, {row: len(rows) - 2, joint: ruleJoint}} {
		if got := ansi.Cut(rows[rule.row], column, column+1); got != rule.joint {
			t.Errorf("row %d holds %q at column %d, want %q", rule.row, got, column, rule.joint)
		}
	}
}

func TestModel_ScrollKeys_MoveTheDetailPaneWithinItsLines(t *testing.T) {
	cases := []struct {
		name   string
		width  int
		height int
		keys   []tea.KeyPressMsg
		want   int
	}{
		{name: "pgdown", width: 120, height: 30, keys: []tea.KeyPressMsg{specialKey(tea.KeyPgDown)}, want: 26},
		{name: "pgup stops at the top", width: 120, height: 30, keys: []tea.KeyPressMsg{specialKey(tea.KeyPgUp)}, want: 0},
		{name: "ctrl+d", width: 120, height: 30, keys: []tea.KeyPressMsg{ctrlKey('d')}, want: 13},
		{name: "ctrl+u", width: 120, height: 30, keys: []tea.KeyPressMsg{ctrlKey('d'), ctrlKey('d'), ctrlKey('u')}, want: 13},
		{name: "pgdown stops at the end", width: 120, height: 30, keys: repeatKey(specialKey(tea.KeyPgDown), 5), want: 74},
		{name: "j in the split layout", width: 120, height: 30, keys: []tea.KeyPressMsg{runeKey('j')}, want: 0},
		{name: "j on the narrow sidebar", width: 80, height: 24, keys: []tea.KeyPressMsg{runeKey('j')}, want: 0},
		{name: "j in the narrow detail", width: 80, height: 24, keys: []tea.KeyPressMsg{specialKey(tea.KeyEnter), runeKey('j'), runeKey('j')}, want: 2},
		{name: "up in the narrow detail", width: 80, height: 24, keys: []tea.KeyPressMsg{specialKey(tea.KeyEnter), runeKey('j'), specialKey(tea.KeyUp)}, want: 0},
		// At 80x40 the stacked detail pane has 36-12-1 rows: the body's, less the sidebar's third and the divider.
		{name: "pgdown in the stacked layout", width: 80, height: 40, keys: []tea.KeyPressMsg{specialKey(tea.KeyPgDown)}, want: 23},
		{name: "j after Enter in the stacked layout", width: 80, height: 40, keys: []tea.KeyPressMsg{specialKey(tea.KeyEnter), runeKey('j')}, want: 0},
	}
	ts := newTestStore(t, nil)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newTestModel(t, ts, testCase.width, testCase.height)
			// The empty store leaves the Ready detail pane without rows, so the test gives it 100 of its own.
			m.detail = make([]string, 100)

			for _, key := range testCase.keys {
				m = send(t, m, key)
			}

			if m.detailOffset != testCase.want {
				t.Errorf("detail offset = %d, want %d", m.detailOffset, testCase.want)
			}
		})
	}
}

// repeatKey returns count presses of key.
func repeatKey(key tea.KeyPressMsg, count int) []tea.KeyPressMsg {
	keys := make([]tea.KeyPressMsg, count)
	for position := range keys {
		keys[position] = key
	}
	return keys
}

func TestModel_StatusBar_ShowsTheTreeLock(t *testing.T) {
	cases := []struct {
		name    string
		arrange func(t *testing.T, ts *testStore)
		want    *regexp.Regexp
	}{
		{
			name:    "free",
			arrange: func(*testing.T, *testStore) {},
			want:    regexp.MustCompile("^" + regexp.QuoteMeta(withLayoutButton(" lock  free · ? help · q quit", layoutButtonToStacked, 120)) + "$"),
		},
		{
			name:    "held",
			arrange: acquireTestLock,
			want:    regexp.MustCompile(`^ lock  held by bob: refactor the parser \(heartbeat 0m ago\) · `),
		},
		{
			name: "stale",
			arrange: func(t *testing.T, ts *testStore) {
				acquireTestLock(t, ts)
				ageTestLock(t, ts, 3*time.Hour)
			},
			want: regexp.MustCompile(`^ lock  stale: refactor the parser \(heartbeat 3h0[0-9]m ago\) · `),
		},
		{
			name: "invalid",
			arrange: func(t *testing.T, ts *testStore) {
				writeTestLockRecord(t, ts, []byte("not json"))
			},
			want: regexp.MustCompile(`^ lock  invalid · `),
		},
		{
			name: "unreadable",
			arrange: func(t *testing.T, ts *testStore) {
				if err := os.MkdirAll(lock.ForWorktree(ts.store).RecordPath(), 0o755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
			},
			want: regexp.MustCompile(`^ lock  unreadable · `),
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ts := newTestStore(t, nil)
			testCase.arrange(t, ts)

			m := newTestModel(t, ts, 120, 30)

			if got := statusRow(m); !testCase.want.MatchString(got) {
				t.Errorf("status bar = %q, want a match for %s", got, testCase.want)
			}
		})
	}
}

func TestLockText_OverviewError_IsUnreadable(t *testing.T) {
	got := lockText(lock.Overview{}, errors.New("cannot list the lock directory"), time.Now())

	if got != " lock  unreadable" {
		t.Errorf("lockText = %q, want \" lock  unreadable\"", got)
	}
}

// acquireTestLock takes the test store's Tree lock for bob, as another session would.
func acquireTestLock(t *testing.T, ts *testStore) {
	t.Helper()
	claim := lock.Claim{Purpose: "refactor the parser", Holder: "bob"}
	if _, _, err := lock.ForWorktree(ts.store).Acquire(claim, "", false); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
}

// ageTestLock moves the since and heartbeat of the test store's lock record age into the past.
func ageTestLock(t *testing.T, ts *testStore, age time.Duration) {
	t.Helper()
	content, err := os.ReadFile(lock.ForWorktree(ts.store).RecordPath())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var record map[string]any
	if err := json.Unmarshal(content, &record); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	past := time.Now().Add(-age).UTC().Format(time.RFC3339Nano)
	record["since"], record["heartbeat"] = past, past
	if content, err = json.Marshal(record); err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	writeTestLockRecord(t, ts, content)
}

// writeTestLockRecord writes content as the test store's lock record, creating its directory.
func writeTestLockRecord(t *testing.T, ts *testStore, content []byte) {
	t.Helper()
	path := lock.ForWorktree(ts.store).RecordPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestModel_FirstStart_CreatesTheUserConfig(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, nil))

	m := startModel(t, options, 120, 30)

	if _, err := os.Stat(options.UserConfigPath); err != nil {
		t.Errorf("user config after the first start: %v, want the file created", err)
	}
	if got := statusRow(m); got != withLayoutButton(" lock  free · ? help · q quit", layoutButtonToStacked, 120) {
		t.Errorf("status bar = %q, want no config notice", got)
	}
}

func TestModel_UserConfigProblems_ShowInTheStatusBar(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		want       string
		isDefaults bool
	}{
		{name: "unknown key", content: "bogus: 1\n", want: `config.yaml:1: unknown key "bogus", ignored · `},
		{name: "invalid theme", content: "ui:\n  theme: purple\n", want: "config.yaml:2: ", isDefaults: true},
	}
	ts := newTestStore(t, nil)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			options := newTestOptions(t, ts)
			writeTestFile(t, options.UserConfigPath, testCase.content)

			m := startModel(t, options, wideStatusWidth(options.UserConfigPath), 30)

			got := statusRow(m)
			if !strings.HasPrefix(got, " lock  free · user config: "+options.UserConfigPath) || !strings.Contains(got, testCase.want) {
				t.Errorf("status bar = %q, want the config notice %q", got, testCase.want)
			}
			if testCase.isDefaults && !strings.Contains(got, defaultsNotice) {
				t.Errorf("status bar = %q, want it to say the UI runs on defaults", got)
			}
		})
	}
}

func TestModel_NoUserConfigDirectory_WarnsAndRunsOnDefaults(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		t.Skip("the user config directory comes from HOME and XDG_CONFIG_HOME on Unix only")
	}
	options := newTestOptions(t, newTestStore(t, nil))
	options.UserConfigPath = ""
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	m := startModel(t, options, wideStatusWidth(""), 30)

	got := statusRow(m)
	if !strings.HasPrefix(got, " lock  free · user config: locate the user config directory: ") || !strings.Contains(got, defaultsNotice) {
		t.Errorf("status bar = %q, want the user config directory warning", got)
	}
	if !reflect.DeepEqual(m.userConfig, userconfig.Default()) {
		t.Errorf("user config = %+v, want the defaults", m.userConfig)
	}
}

func TestModel_StoredSidebarRatio_StartsTheSidebarOnIt(t *testing.T) {
	cases := []struct {
		width int
		want  int
	}{
		{width: 120, want: 30},
		{width: 200, want: 50},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d columns", tc.width), func(t *testing.T) {
			options := newTestOptions(t, newTestStore(t, nil))
			writeTestFile(t, options.UserStatePath, "ui:\n  sidebar_ratio: 0.25\n")

			m := startModel(t, options, tc.width, 30)

			if got := m.sidebarWidth(); got != tc.want {
				t.Errorf("sidebarWidth() = %d, want %d: a stored 0.25 of %d columns", got, tc.want, tc.width)
			}
		})
	}
}

func TestModel_ResizeKey_SavesTheRatioToTheStateFile(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, nil))
	writeTestFile(t, options.UserStatePath, ratioState(36, 120))
	m := startModel(t, options, 120, 30)

	press(t, m, runeKey('>'))

	if got, want := readStoredRatio(t, options.UserStatePath), widthRatio(40, 120); got != want {
		t.Errorf("stored sidebar ratio = %v, want %v: 40 of 120 columns", got, want)
	}
	content, err := os.ReadFile(options.UserStatePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(content), "sidebar_ratio: ") {
		t.Errorf("state file = %q, want a sidebar_ratio key", content)
	}
}

func TestModel_StoredBothRatios_StartsTheModelOnBoth(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, nil))
	writeTestFile(t, options.UserStatePath, "ui:\n  sidebar_ratio: 0.25\n  stacked_ratio: 0.4\n")

	m := startModel(t, options, 120, 30)

	if m.sidebarRatio != 0.25 || m.stackedRatio != 0.4 {
		t.Errorf(
			"sidebarRatio, stackedRatio = %v, %v, want the stored 0.25, 0.4",
			m.sidebarRatio,
			m.stackedRatio,
		)
	}
}

func TestModel_ResizeKey_KeepsTheStoredStackedRatio(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, nil))
	writeTestFile(t, options.UserStatePath, ratioState(36, 120)+"  stacked_ratio: 0.4\n")
	m := startModel(t, options, 120, 30)

	press(t, m, runeKey('>'))

	state, err := userconfig.LoadState(options.UserStatePath)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	want := userconfig.UIState{SidebarRatio: widthRatio(40, 120), StackedRatio: 0.4}
	if state.UI != want {
		t.Errorf("stored ratios = %+v, want %+v: the new width beside the stacked ratio", state.UI, want)
	}
}

func TestModel_SaveCommands_RunInReverseOrder_LeaveTheLaterWidth(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, nil))
	m := startModel(t, options, 120, 30)
	m.sidebarRatio = 0.25
	earlier := m.saveState()
	m.sidebarRatio = 0.5
	later := m.saveState()

	m = drive(t, m, later)
	m = drive(t, m, earlier)

	if got := readStoredRatio(t, options.UserStatePath); got != 0.5 {
		t.Errorf("stored sidebar ratio = %v, want the later 0.5", got)
	}
	if m.stateNotice != "" {
		t.Errorf("state notice = %q, want none after the saves succeeded", m.stateNotice)
	}
}

func TestModel_MalformedStateFile_ShowsItsPathAndRunsOnTheDefaultWidth(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, nil))
	writeTestFile(t, options.UserStatePath, "ui:\n  sidebar_ratio: wide\n")

	m := startModel(t, options, wideStatusWidth(options.UserStatePath), 30)

	if got := statusRow(m); !strings.Contains(got, stateNoticePrefix+options.UserStatePath+":2: ") {
		t.Errorf("status bar = %q, want the state file notice naming %s", got, options.UserStatePath)
	}
	if m.sidebarRatio != 0 {
		t.Errorf("sidebarRatio = %v, want 0: the default width", m.sidebarRatio)
	}
}

func TestModel_UnwritableStateFile_ShowsTheSaveNoticeAndStillResizes(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, nil))
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	writeTestFile(t, parent, "a regular file\n")
	options.UserStatePath = filepath.Join(parent, "state.yaml")
	m := startModel(t, options, wideStatusWidth(options.UserStatePath), 30)
	before := m.sidebarWidth()

	m = press(t, m, runeKey('>'))

	if got := m.sidebarWidth(); got != before+sidebarStep {
		t.Errorf("sidebarWidth() = %d, want %d: the width changes though it cannot be saved", got, before+sidebarStep)
	}
	if got := statusRow(m); !strings.Contains(got, stateSaveNoticePrefix) {
		t.Errorf("status bar = %q, want the notice %q", got, stateSaveNoticePrefix)
	}
}

// readStoredRatio returns the sidebar ratio the state file at path holds, 0 for none.
func readStoredRatio(t *testing.T, path string) float64 {
	t.Helper()
	state, err := userconfig.LoadState(path)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	return state.UI.SidebarRatio
}

// widthRatio returns a sidebar of columns as the ratio the model keeps on a terminal width columns
// wide.
func widthRatio(columns, width int) float64 {
	return float64(columns) / float64(width)
}

// ratioState returns a state file holding the sidebar ratio of columns on a terminal width columns
// wide.
func ratioState(columns, width int) string {
	return "ui:\n  sidebar_ratio: " + strconv.FormatFloat(widthRatio(columns, width), 'f', -1, 64) + "\n"
}

// writeTestFile writes content at path, creating its directory.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestModel_ResizeKeys_MoveTheStackedDividerByAStep(t *testing.T) {
	cases := []struct {
		name  string
		key   rune
		delta int
	}{
		{name: "grow", key: '>', delta: stackedStep},
		{name: "shrink", key: '<', delta: -stackedStep},
	}
	ts := newTestStore(t, readyFiles)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newTestModel(t, ts, 80, 40)
			before := m.stackedSidebarRows()

			m = send(t, m, runeKey(testCase.key))

			if got, want := m.stackedSidebarRows(), before+testCase.delta; got != want {
				t.Errorf("stackedSidebarRows() = %d, want %d", got, want)
			}
			if got := ansi.Strip(strings.Split(m.View().Content, "\n")[m.stackedDividerRow()]); got != strings.Repeat(ruleGlyph, 80) {
				t.Errorf("row %d = %q, want the stacked divider", m.stackedDividerRow(), got)
			}
		})
	}
}

func TestModel_ResizeKeys_StopAtTheStackedMinimums(t *testing.T) {
	cases := []struct {
		name           string
		key            rune
		wantSidebar    int
		wantDetailRows int
	}{
		{name: "shrink", key: '<', wantSidebar: stackSidebarMinRows, wantDetailRows: 36 - 1 - stackSidebarMinRows},
		{name: "grow", key: '>', wantSidebar: 36 - 1 - stackDetailMinRows, wantDetailRows: stackDetailMinRows},
	}
	ts := newTestStore(t, readyFiles)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newTestModel(t, ts, 80, 40)

			for _, key := range repeatKey(runeKey(testCase.key), 20) {
				m = send(t, m, key)
			}

			if got := m.stackedSidebarRows(); got != testCase.wantSidebar {
				t.Errorf("stackedSidebarRows() = %d, want %d", got, testCase.wantSidebar)
			}
			if got := m.detailPaneRows(); got != testCase.wantDetailRows {
				t.Errorf("detailPaneRows() = %d, want %d", got, testCase.wantDetailRows)
			}
		})
	}
}

func TestModel_ResizeKey_InTheStackedLayout_SavesTheStackedRatioThatSurvivesARestart(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, nil))
	writeTestFile(t, options.UserStatePath, "ui:\n  sidebar_ratio: 0.3\n")
	m := startModel(t, options, 80, 40)
	want := m.stackedSidebarRows() + stackedStep

	press(t, m, runeKey('>'))
	restarted := startModel(t, options, 80, 40)

	state, err := userconfig.LoadState(options.UserStatePath)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	wantState := userconfig.UIState{SidebarRatio: 0.3, StackedRatio: float64(want) / 36}
	if state.UI != wantState {
		t.Errorf("stored ratios = %+v, want %+v: the new height beside the stored width", state.UI, wantState)
	}
	if got := restarted.stackedSidebarRows(); got != want {
		t.Errorf("stackedSidebarRows() after a restart = %d, want the saved %d", got, want)
	}
}

func TestModel_ShrinkKey_InTheSplitLayout_KeepsTheStoredStackedRatio(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, nil))
	writeTestFile(t, options.UserStatePath, ratioState(36, 120)+"  stacked_ratio: 0.4\n")
	m := startModel(t, options, 120, 30)

	m = press(t, m, runeKey('<'))

	if got := m.sidebarWidth(); got != 36-sidebarStep {
		t.Errorf("sidebarWidth() = %d, want %d", got, 36-sidebarStep)
	}
	state, err := userconfig.LoadState(options.UserStatePath)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	want := userconfig.UIState{SidebarRatio: widthRatio(36-sidebarStep, 120), StackedRatio: 0.4}
	if state.UI != want {
		t.Errorf("stored ratios = %+v, want %+v: the new width beside the stacked ratio", state.UI, want)
	}
}

func TestModel_UnwritableStateFile_InTheStackedLayout_ShowsTheDividerSaveNotice(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, nil))
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	writeTestFile(t, parent, "a regular file\n")
	options.UserStatePath = filepath.Join(parent, "state.yaml")
	m := startModel(t, options, 80, 40)
	before := m.stackedSidebarRows()

	m = press(t, m, runeKey('>'))

	if got := m.stackedSidebarRows(); got != before+stackedStep {
		t.Errorf("stackedSidebarRows() = %d, want %d: the height changes though it cannot be saved", got, before+stackedStep)
	}
	if !strings.HasPrefix(m.stateNotice, "cannot save the layout: ") {
		t.Errorf("state notice = %q, want it to open with %q", m.stateNotice, "cannot save the layout: ")
	}
}
