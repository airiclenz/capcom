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
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/airiclenz/capcom/internal/lock"
	"github.com/airiclenz/capcom/internal/userconfig"
)

// wideStatusWidth is a terminal wide enough for a status bar naming a temp-dir config path.
const wideStatusWidth = 240

func TestModel_View_EmptyLayout_MatchesGolden(t *testing.T) {
	cases := []struct {
		name   string
		width  int
		height int
	}{
		{name: "layout-120x30", width: 120, height: 30},
		{name: "layout-80x24", width: 80, height: 24},
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
		{name: "tab wraps", keys: []tea.KeyPressMsg{specialKey(tea.KeyTab), specialKey(tea.KeyTab)}, want: TabIssues},
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

func TestModel_TabRule_JoinsTheDividerOnlyInTheSplitLayout(t *testing.T) {
	cases := []struct {
		name      string
		width     int
		keys      []tea.KeyPressMsg
		wantJoint bool
	}{
		{name: "split", width: 120, wantJoint: true},
		{name: "narrow", width: 80},
		{name: "help", width: 120, keys: []tea.KeyPressMsg{runeKey('?')}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newTestModel(t, newTestStore(t, readyFiles), testCase.width, 30)
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

func TestModel_StoredSidebarWidth_ClampsWithoutChangingIt(t *testing.T) {
	m := newTestModel(t, newTestStore(t, nil), 120, 30)
	m.sidebarColumns = 80

	m = send(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})

	if got := m.sidebarWidth(); got != 57 {
		t.Errorf("sidebarWidth() = %d, want 57: a stored 80 clamped at 100 columns", got)
	}
	if m.sidebarColumns != 80 {
		t.Errorf("sidebarColumns = %d, want the stored 80 unchanged by the clamp", m.sidebarColumns)
	}
}

func TestModel_ResizeKeys_DoNothingOnANarrowTerminal(t *testing.T) {
	m := newTestModel(t, newTestStore(t, readyFiles), 80, 24)
	before := m.View().Content

	for _, key := range []rune{'>', '<', '<'} {
		m = send(t, m, runeKey(key))
	}

	if m.sidebarColumns != 0 {
		t.Errorf("sidebarColumns = %d, want 0: no width set on a narrow terminal", m.sidebarColumns)
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
		name  string
		width int
		keys  []tea.KeyPressMsg
		want  int
	}{
		{name: "pgdown", width: 120, keys: []tea.KeyPressMsg{specialKey(tea.KeyPgDown)}, want: 26},
		{name: "pgup stops at the top", width: 120, keys: []tea.KeyPressMsg{specialKey(tea.KeyPgUp)}, want: 0},
		{name: "ctrl+d", width: 120, keys: []tea.KeyPressMsg{ctrlKey('d')}, want: 13},
		{name: "ctrl+u", width: 120, keys: []tea.KeyPressMsg{ctrlKey('d'), ctrlKey('d'), ctrlKey('u')}, want: 13},
		{name: "pgdown stops at the end", width: 120, keys: repeatKey(specialKey(tea.KeyPgDown), 5), want: 74},
		{name: "j in the split layout", width: 120, keys: []tea.KeyPressMsg{runeKey('j')}, want: 0},
		{name: "j on the narrow sidebar", width: 80, keys: []tea.KeyPressMsg{runeKey('j')}, want: 0},
		{name: "j in the narrow detail", width: 80, keys: []tea.KeyPressMsg{specialKey(tea.KeyEnter), runeKey('j'), runeKey('j')}, want: 2},
		{name: "up in the narrow detail", width: 80, keys: []tea.KeyPressMsg{specialKey(tea.KeyEnter), runeKey('j'), specialKey(tea.KeyUp)}, want: 0},
	}
	ts := newTestStore(t, nil)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newTestModel(t, ts, testCase.width, 30)
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
			want:    regexp.MustCompile(`^lock: free · \? help · q quit$`),
		},
		{
			name:    "held",
			arrange: acquireTestLock,
			want:    regexp.MustCompile(`^lock: held by bob: refactor the parser \(heartbeat 0m ago\) · `),
		},
		{
			name: "stale",
			arrange: func(t *testing.T, ts *testStore) {
				acquireTestLock(t, ts)
				ageTestLock(t, ts, 3*time.Hour)
			},
			want: regexp.MustCompile(`^lock: stale: refactor the parser \(heartbeat 3h0[0-9]m ago\) · `),
		},
		{
			name: "invalid",
			arrange: func(t *testing.T, ts *testStore) {
				writeTestLockRecord(t, ts, []byte("not json"))
			},
			want: regexp.MustCompile(`^lock: invalid · `),
		},
		{
			name: "unreadable",
			arrange: func(t *testing.T, ts *testStore) {
				if err := os.MkdirAll(lock.ForWorktree(ts.store).RecordPath(), 0o755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
			},
			want: regexp.MustCompile(`^lock: unreadable · `),
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

	if got != "lock: unreadable" {
		t.Errorf("lockText = %q, want lock: unreadable", got)
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
	if got := statusRow(m); got != "lock: free · ? help · q quit" {
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

			m := startModel(t, options, wideStatusWidth, 30)

			got := statusRow(m)
			if !strings.HasPrefix(got, "lock: free · user config: "+options.UserConfigPath) || !strings.Contains(got, testCase.want) {
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

	m := startModel(t, options, wideStatusWidth, 30)

	got := statusRow(m)
	if !strings.HasPrefix(got, "lock: free · user config: locate the user config directory: ") || !strings.Contains(got, defaultsNotice) {
		t.Errorf("status bar = %q, want the user config directory warning", got)
	}
	if !reflect.DeepEqual(m.userConfig, userconfig.Default()) {
		t.Errorf("user config = %+v, want the defaults", m.userConfig)
	}
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
