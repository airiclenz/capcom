package ui

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/airiclenz/capcom/internal/ops"
)

// The screen sizes the mouse tests run at: wide for the split layout, small for the narrow one.
const (
	wideWidth   = 120
	wideHeight  = 30
	smallWidth  = 80
	smallHeight = 24
)

// fillerLines is how many paragraphs open the body of linkFiles' long issue, enough to push its
// link below the detail pane's fold at either size.
const fillerLines = 40

// linkFiles is a store whose Ready queue opens on long, a long body ending in a link to beta, then
// beta, then pointer, whose body links the Note setup under guide.
var linkFiles = map[string]string{
	"issues/long.md":           issueFile("title: Long\nstatus: open\npriority: 0\n", longBody()),
	"issues/beta.md":           issueFile("title: Beta\nstatus: open\npriority: 1\n", "Beta's body.\n"),
	"issues/pointer.md":        issueFile("title: Pointer\nstatus: open\npriority: 2\n", "Read `setup` first.\n"),
	"knowledge/guide.md":       noteFile("title: Guide\n", "The guide.\n"),
	"knowledge/guide/setup.md": noteFile("title: Setup\n", "Set it up.\n"),
}

// longBody returns fillerLines paragraphs, then one naming beta.
func longBody() string {
	var body strings.Builder
	for line := range fillerLines {
		fmt.Fprintf(&body, "Filler paragraph %d.\n\n", line)
	}
	body.WriteString("Then see `beta`.\n")
	return body.String()
}

// click is a left click at column x, row y.
func click(x, y int) tea.MouseClickMsg {
	return tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft}
}

// wheel is a notch of the wheel button at column x, row y.
func wheel(button tea.MouseButton, x, y int) tea.MouseWheelMsg {
	return tea.MouseWheelMsg{X: x, Y: y, Button: button}
}

// detailLeft returns the screen column the detail pane starts at: past the sidebar and the
// divider " │ " in the split layout.
func detailLeft(m model) int {
	if m.isNarrow() {
		return 0
	}
	return m.sidebarWidth() + 3
}

// linkCell returns the screen cell of the first cell of the detail pane's link to target as the
// pane is scrolled now.
func linkCell(t *testing.T, m model, target string) (int, int) {
	t.Helper()
	for _, link := range m.issue.rendered.Links {
		if link.Target == target {
			return detailLeft(m) + link.Col, bodyTop + link.Line - m.detailOffset
		}
	}
	t.Fatalf("no link to %s in the detail pane: %v", target, m.issue.rendered.Links)
	return 0, 0
}

// openDetail opens the narrow detail view on a narrow screen, and leaves a wide one as it is.
func openDetail(t *testing.T, m model) model {
	t.Helper()
	if !m.isNarrow() {
		return m
	}
	return press(t, m, specialKey(tea.KeyEnter))
}

func TestMouse_View_ReportsClicksAndTheWheel(t *testing.T) {
	m := newTestModel(t, newTestStore(t, readyFiles), wideWidth, wideHeight)

	got := m.View().MouseMode

	if got != tea.MouseModeCellMotion {
		t.Errorf("MouseMode = %v, want MouseModeCellMotion", got)
	}
}

func TestMouse_ClickOnTabBar_OpensTheTabUnderThePointer(t *testing.T) {
	cases := []struct {
		name    string
		width   int
		height  int
		x       int
		wantTab Tab
	}{
		{name: "Issues at 120x30", width: wideWidth, height: wideHeight, x: 2, wantTab: TabIssues},
		{name: "Knowledge at 120x30", width: wideWidth, height: wideHeight, x: 25, wantTab: TabKnowledge},
		{name: "separator at 120x30", width: wideWidth, height: wideHeight, x: 9, wantTab: TabReady},
		{name: "Knowledge at 80x24", width: smallWidth, height: smallHeight, x: 31, wantTab: TabKnowledge},
		{name: "past the labels at 80x24", width: smallWidth, height: smallHeight, x: 32, wantTab: TabReady},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(t, newTestStore(t, readyFiles), tc.width, tc.height)

			m = press(t, m, click(tc.x, 0))

			if m.tab != tc.wantTab {
				t.Errorf("tab = %v, want %v", m.tab, tc.wantTab)
			}
		})
	}
}

func TestMouse_ClickOnTheVersionLabel_ChangesNoTab(t *testing.T) {
	versionLabel := versionLabelPrefix + testVersion + versionLabelSuffix
	firstColumn := wideWidth - ansi.StringWidth(versionLabel)
	for _, x := range []int{firstColumn, wideWidth - 1} {
		t.Run(fmt.Sprintf("column %d", x), func(t *testing.T) {
			m := newIssuesModel(t, newTestStore(t, treeFiles), "solo", wideWidth, wideHeight)
			if !strings.HasSuffix(strings.Split(ansi.Strip(m.View().Content), "\n")[0], versionLabel) {
				t.Fatalf("tab bar lacks the version label %q", versionLabel)
			}

			m = press(t, m, click(x, 0))

			if m.tab != TabIssues {
				t.Errorf("tab = %v, want %v", m.tab, TabIssues)
			}
		})
	}
}

func TestMouse_ClickOnSidebarRow_SelectsAndShowsIt(t *testing.T) {
	for _, size := range [][2]int{{wideWidth, wideHeight}, {smallWidth, smallHeight}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := newTestModel(t, newTestStore(t, readyFiles), size[0], size[1])

			m = press(t, m, click(3, 3))

			if got, isSelected := m.readySelection(); !isSelected || got.ID != "beta" {
				t.Fatalf("selected %q, want beta", got.ID)
			}
			if !m.isNarrow() && !strings.Contains(detailText(m), "Beta's body.") {
				t.Errorf("detail pane = %q, want beta's body", detailText(m))
			}
		})
	}
}

func TestMouse_ClickBelowTheLastSidebarRow_KeepsTheSelection(t *testing.T) {
	m := newTestModel(t, newTestStore(t, readyFiles), wideWidth, wideHeight)

	m = press(t, m, click(3, 11))

	if got, _ := m.readySelection(); got.ID != "alpha" {
		t.Errorf("selected %q, want alpha", got.ID)
	}
}

func TestMouse_ClickOrWheelOnTheTabRule_DoesNothing(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, treeFiles), "solo", wideWidth, wideHeight)

	// Row 1 is the rule under the tab bar; row 2 would hold the epic's arrow.
	clicked := press(t, m, click(0, 1))
	wheeled := press(t, m, wheel(tea.MouseWheelDown, 3, 1))

	if got := selectedIssueID(clicked); got != selectedIssueID(m) || slices.Contains(visibleIDs(clicked), "alpha") {
		t.Errorf("a click on the rule selected %q, visible %v; want the selection and the tree as they were",
			got, visibleIDs(clicked))
	}
	if got := selectedIssueID(wheeled); got != selectedIssueID(m) {
		t.Errorf("a wheel on the rule selected %q, want %q", got, selectedIssueID(m))
	}
}

func TestMouse_NarrowDetailView_OnlyTheDetailPaneResponds(t *testing.T) {
	m := openDetail(t, newTestModel(t, newTestStore(t, readyFiles), smallWidth, smallHeight))

	m = press(t, m, click(3, 3))
	m = press(t, m, wheel(tea.MouseWheelDown, 3, 3))

	if got, _ := m.readySelection(); got.ID != "alpha" || !m.isDetailOpen {
		t.Errorf("selected %q with the detail view open %v; want alpha, still open", got.ID, m.isDetailOpen)
	}
}

func TestMouse_ClickOnTreeArrow_ExpandsAndCollapses(t *testing.T) {
	for _, size := range [][2]int{{wideWidth, wideHeight}, {smallWidth, smallHeight}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := newIssuesModel(t, newTestStore(t, treeFiles), "solo", size[0], size[1])

			expanded := press(t, m, click(0, 2))
			collapsed := press(t, expanded, click(1, 2))

			if got := selectedIssueID(expanded); got != "epic" || !slices.Contains(visibleIDs(expanded), "alpha") {
				t.Errorf("after the first click: selected %q, visible %v; want epic expanded", got, visibleIDs(expanded))
			}
			if slices.Contains(visibleIDs(collapsed), "alpha") {
				t.Errorf("after the second click: visible %v, want epic collapsed", visibleIDs(collapsed))
			}
		})
	}
}

func TestMouse_ClickOnANestedTreeArrow_ExpandsAndCollapsesThatRow(t *testing.T) {
	for _, size := range [][2]int{{wideWidth, wideHeight}, {smallWidth, smallHeight}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := newIssuesModel(t, newTestStore(t, treeFiles), "solo", size[0], size[1])
			// Row 3 is alpha under the expanded epic: its connector in columns 0 and 1, its arrow
			// in column 2.
			m = press(t, m, click(0, 2))

			onConnector := press(t, m, click(0, 3))
			expanded := press(t, m, click(2, 3))
			collapsed := press(t, expanded, click(2, 3))

			if got := selectedIssueID(onConnector); got != "alpha" || slices.Contains(visibleIDs(onConnector), "leaf") {
				t.Errorf("after a click on the connector: selected %q, visible %v; want alpha, still collapsed",
					got, visibleIDs(onConnector))
			}
			if got := selectedIssueID(expanded); got != "alpha" || !slices.Contains(visibleIDs(expanded), "leaf") {
				t.Errorf("after the first arrow click: selected %q, visible %v; want alpha expanded", got, visibleIDs(expanded))
			}
			if slices.Contains(visibleIDs(collapsed), "leaf") || !slices.Contains(visibleIDs(collapsed), "alpha") {
				t.Errorf("after the second arrow click: visible %v, want alpha collapsed under epic", visibleIDs(collapsed))
			}
		})
	}
}

func TestMouse_ClickOnTreeRowText_SelectsWithoutExpanding(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, treeFiles), "solo", wideWidth, wideHeight)

	m = press(t, m, click(6, 2))

	if got := selectedIssueID(m); got != "epic" || slices.Contains(visibleIDs(m), "alpha") {
		t.Errorf("selected %q, visible %v; want epic selected and still collapsed", got, visibleIDs(m))
	}
}

func TestMouse_ClickOnDepViewRow_SelectsItForEnter(t *testing.T) {
	for _, size := range [][2]int{{wideWidth, wideHeight}, {smallWidth, smallHeight}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := newIssuesModel(t, newTestStore(t, depFiles), "alpha", size[0], size[1])
			m = press(t, m, runeKey('d'))

			// Row 7 of the screen is line 5 of the view: user, under the blocks heading.
			m = press(t, m, click(detailLeft(m)+4, 7))
			m = press(t, m, specialKey(tea.KeyEnter))

			if got := selectedIssueID(m); got != "user" {
				t.Errorf("selected %q after the click and Enter, want user", got)
			}
		})
	}
}

func TestMouse_PointAtDetailLeft_IsTheDetailPanesFirstColumn(t *testing.T) {
	for _, size := range [][2]int{{wideWidth, wideHeight}, {smallWidth, smallHeight}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := openDetail(t, newTestModel(t, newTestStore(t, readyFiles), size[0], size[1]))

			got := m.pointAt(detailLeft(m), 2)

			if want := (panePoint{pane: paneDetail, col: 0, row: 0}); got != want {
				t.Errorf("pointAt(%d, 2) = %+v, want %+v", detailLeft(m), got, want)
			}
		})
	}
}

func TestMouse_ClickOnTheDividerOrItsMargins_SelectsNothing(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, depFiles), "alpha", wideWidth, wideHeight)
	m = press(t, m, runeKey('d'))
	sidebarWidth := m.sidebarWidth()

	for x := sidebarWidth; x <= sidebarWidth+2; x++ {
		// Row 7 of the screen is line 5 of the view: user, under the blocks heading.
		clicked := press(t, m, click(x, 7))

		if clicked.deps.cursor.selected != m.deps.cursor.selected {
			t.Errorf("a click on column %d moved the dep view's cursor to row %d, want it left on %d",
				x, clicked.deps.cursor.selected, m.deps.cursor.selected)
		}
	}
}

func TestMouse_ClickOnLink_SelectsTheIssueInTheIssuesTab(t *testing.T) {
	for _, size := range [][2]int{{wideWidth, wideHeight}, {smallWidth, smallHeight}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := openDetail(t, newTestModel(t, newTestStore(t, readyFiles), size[0], size[1]))

			m = press(t, m, click(linkCell(t, m, "beta")))

			if m.tab != TabIssues || selectedIssueID(m) != "beta" {
				t.Errorf("tab %v selecting %q, want the Issues tab selecting beta", m.tab, selectedIssueID(m))
			}
		})
	}
}

func TestMouse_ClickOnLinkAfterScrolling_FollowsIt(t *testing.T) {
	for _, size := range [][2]int{{wideWidth, wideHeight}, {smallWidth, smallHeight}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := openDetail(t, newTestModel(t, newTestStore(t, linkFiles), size[0], size[1]))
			x := detailLeft(m) + 2
			for range fillerLines {
				m = press(t, m, wheel(tea.MouseWheelDown, x, 3))
			}
			if m.detailOffset == 0 {
				t.Fatal("the wheel left the detail pane unscrolled")
			}

			m = press(t, m, click(linkCell(t, m, "beta")))

			if m.tab != TabIssues || selectedIssueID(m) != "beta" {
				t.Errorf("tab %v selecting %q, want the Issues tab selecting beta", m.tab, selectedIssueID(m))
			}
		})
	}
}

func TestMouse_ClickOnLinkToClosedIssue_RevealsAndSelectsIt(t *testing.T) {
	m := newTestModel(t, newTestStore(t, readyFiles), wideWidth, wideHeight)

	m = press(t, m, click(linkCell(t, m, "finished")))

	if m.tab != TabIssues || selectedIssueID(m) != "finished" {
		t.Fatalf("tab %v selecting %q, want the Issues tab selecting finished", m.tab, selectedIssueID(m))
	}
	if got := mutedIDs(m); !slices.Equal(got, []string{"finished"}) {
		t.Errorf("muted rows = %v, want [finished]", got)
	}
}

func TestMouse_ClickOnNoteLink_SelectsTheNoteInTheKnowledgeTab(t *testing.T) {
	m := newTestModel(t, newTestStore(t, linkFiles), wideWidth, wideHeight)
	m = press(t, m, click(3, 4))

	m = press(t, m, click(linkCell(t, m, "setup")))

	if m.tab != TabKnowledge || selectedNoteID(m) != "setup" {
		t.Errorf("tab %v selecting %q, want the Knowledge tab selecting setup", m.tab, selectedNoteID(m))
	}
}

func TestMouse_WheelOverSidebar_MovesTheSelectionOneRow(t *testing.T) {
	cases := []struct {
		name   string
		button tea.MouseButton
		want   string
	}{
		{name: "down", button: tea.MouseWheelDown, want: "beta"},
		{name: "up at the first row", button: tea.MouseWheelUp, want: "alpha"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(t, newTestStore(t, readyFiles), wideWidth, wideHeight)

			m = press(t, m, wheel(tc.button, 3, 6))

			if got, _ := m.readySelection(); got.ID != tc.want {
				t.Errorf("selected %q, want %q", got.ID, tc.want)
			}
		})
	}
}

func TestMouse_WheelOverDetail_ScrollsThePane(t *testing.T) {
	m := newTestModel(t, newTestStore(t, linkFiles), wideWidth, wideHeight)

	m = press(t, m, wheel(tea.MouseWheelDown, detailLeft(m)+2, 6))

	if got, _ := m.readySelection(); m.detailOffset != detailWheelRows || got.ID != "long" {
		t.Errorf("detail offset %d selecting %q, want %d selecting long", m.detailOffset, got.ID, detailWheelRows)
	}
}

func TestMouse_ClickWithHelpOpen_DoesNothing(t *testing.T) {
	m := press(t, newTestModel(t, newTestStore(t, readyFiles), wideWidth, wideHeight), runeKey('?'))

	m = press(t, m, click(25, 0))

	if m.tab != TabReady || !m.isHelpOpen {
		t.Errorf("tab %v, help open %v; want the Ready tab under the open help screen", m.tab, m.isHelpOpen)
	}
}

func TestMouse_ClickDuringReasonPrompt_EnterClosesOnlyThePromptedIssue(t *testing.T) {
	want := wantWrite(t, alphaPath, func(writer *ops.Ops) error {
		_, err := writer.Close("alpha", "", "done", "fixed", time.Now())
		return err
	})
	ts := newTestStore(t, readyFiles)
	m := typeKeys(t, press(t, newTestModel(t, ts, wideWidth, wideHeight), runeKey('c')), "fixed")

	m = press(t, m, click(3, 3))
	m = press(t, m, wheel(tea.MouseWheelDown, 3, 3))
	m = press(t, m, specialKey(tea.KeyEnter))

	if got := readStoreFile(t, ts, alphaPath); !bytes.Equal(withoutClosedTime(got), withoutClosedTime(want)) {
		t.Errorf("alpha after the close:\n%s\nwant:\n%s", got, want)
	}
	if got := readStoreFile(t, ts, betaPath); !bytes.Equal(got, []byte(readyFiles[betaPath])) {
		t.Errorf("beta changed:\n%s", got)
	}
}

func TestMouse_WheelOverTheScrollbar_ScrollsTheDetail(t *testing.T) {
	for _, size := range [][2]int{{wideWidth, wideHeight}, {smallWidth, smallHeight}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := openDetail(t, newTestModel(t, newTestStore(t, linkFiles), size[0], size[1]))

			m = press(t, m, wheel(tea.MouseWheelDown, m.width-1, 6))

			if m.detailOffset != detailWheelRows {
				t.Errorf("detail offset %d after a wheel notch on the bar, want %d", m.detailOffset, detailWheelRows)
			}
		})
	}
}

func TestMouse_ClickOnTheScrollbarOrItsGutter_SelectsNothing(t *testing.T) {
	for _, size := range [][2]int{{wideWidth, wideHeight}, {smallWidth, smallHeight}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := newIssuesModel(t, newTestStore(t, depFiles), "alpha", size[0], size[1])
			m = press(t, m, runeKey('d'))

			for x := m.width - scrollbarColumns; x < m.width; x++ {
				// Row 7 of the screen is line 5 of the view: user, under the blocks heading.
				clicked := press(t, m, click(x, 7))

				if clicked.deps.cursor.selected != m.deps.cursor.selected {
					t.Errorf("a click on column %d moved the dep view's cursor to row %d, want it left on %d",
						x, clicked.deps.cursor.selected, m.deps.cursor.selected)
				}
			}
		})
	}
}

func TestMouse_ClickOnTheSidebarsScrollbar_SelectsNothing(t *testing.T) {
	for _, size := range [][2]int{{wideWidth, wideHeight}, {smallWidth, smallHeight}} {
		m := newTestModel(t, newTestStore(t, readyFiles), size[0], size[1])
		barColumn := m.sidebarPaneWidth() + scrollbarColumns - 1
		for _, x := range []int{barColumn - 1, barColumn} {
			t.Run(fmt.Sprintf("%dx%d column %d", size[0], size[1], x), func(t *testing.T) {
				// Row 3 holds beta, the second entry.
				clicked := press(t, m, click(x, 3))

				if got, _ := clicked.readySelection(); got.ID != "alpha" {
					t.Errorf("a click on the sidebar's scrollbar selected %q, want alpha as it was", got.ID)
				}
			})
		}
	}
}

func TestMouse_WheelOnTheSidebarsScrollbar_MovesTheSelection(t *testing.T) {
	for _, size := range [][2]int{{wideWidth, wideHeight}, {smallWidth, smallHeight}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := newTestModel(t, newTestStore(t, readyFiles), size[0], size[1])
			barColumn := m.sidebarPaneWidth() + scrollbarColumns - 1

			m = press(t, m, wheel(tea.MouseWheelDown, barColumn, 3))

			if got, _ := m.readySelection(); got.ID != "beta" {
				t.Errorf("a wheel on the sidebar's scrollbar selected %q, want beta", got.ID)
			}
		})
	}
}

// dragRow is the screen row the drag tests press, move and release on: a row of the panes.
const dragRow = 5

// motion is the pointer moving to column x, row y with the left button held.
func motion(x, y int) tea.MouseMotionMsg {
	return tea.MouseMotionMsg{X: x, Y: y, Button: tea.MouseLeft}
}

// release is the left button let go at column x, row y.
func release(x, y int) tea.MouseReleaseMsg {
	return tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft}
}

// dragDivider presses the left button at column from, moves the pointer to column to and lets
// the button go there, all on dragRow.
func dragDivider(t *testing.T, m model, from, to int) model {
	t.Helper()
	m = press(t, m, click(from, dragRow))
	m = press(t, m, motion(to, dragRow))
	return press(t, m, release(to, dragRow))
}

func TestMouse_DragTheDivider_MovesItAndSavesTheWidth(t *testing.T) {
	for grab := range 3 {
		t.Run(fmt.Sprintf("column %d of the hit area", grab), func(t *testing.T) {
			options := newTestOptions(t, newTestStore(t, readyFiles))
			m := startModel(t, options, wideWidth, wideHeight)
			before := m.sidebarWidth()
			from := before + grab

			m = dragDivider(t, m, from, from+10)

			if got := m.sidebarWidth(); got != before+10 {
				t.Errorf("sidebarWidth() = %d, want %d", got, before+10)
			}
			if got, want := readStoredRatio(t, options.UserStatePath), widthRatio(before+10, wideWidth); got != want {
				t.Errorf("stored sidebar ratio = %v, want %v", got, want)
			}
		})
	}
}

func TestMouse_DragTheDividerPastALimit_Clamps(t *testing.T) {
	cases := []struct {
		name string
		to   int
		want int
	}{
		{name: "past the sidebar's limit", to: 0, want: sidebarDragMinWidth},
		{name: "past the detail pane's limit", to: wideWidth - 1, want: wideWidth - 3 - detailMinWidth},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(t, newTestStore(t, readyFiles), wideWidth, wideHeight)
			from := m.dividerColumn()
			m = press(t, m, click(from, dragRow))

			m = press(t, m, motion(tc.to, dragRow))

			if got := m.sidebarWidth(); got != tc.want {
				t.Errorf("sidebarWidth() = %d, want %d", got, tc.want)
			}
			assertDividerAt(t, m)
		})
	}
}

func TestMouse_PressAndReleaseOnTheDivider_WritesNothing(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, readyFiles))
	m := startModel(t, options, wideWidth, wideHeight)
	before := m.sidebarWidth()

	m = press(t, m, click(m.dividerColumn(), dragRow))
	m = press(t, m, release(m.dividerColumn(), dragRow))

	if got := m.sidebarWidth(); got != before {
		t.Errorf("sidebarWidth() = %d, want it left at %d", got, before)
	}
	if got := readStoredRatio(t, options.UserStatePath); got != 0 {
		t.Errorf("stored sidebar ratio = %v, want none written", got)
	}
}

func TestMouse_PressOffThePanesRows_StartsNoDrag(t *testing.T) {
	cases := []struct {
		name string
		y    int
	}{
		{name: "tab bar", y: 0},
		{name: "rule above the panes", y: 1},
		{name: "rule under the panes", y: wideHeight - 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(t, newTestStore(t, readyFiles), wideWidth, wideHeight)
			before := m.sidebarWidth()
			x := m.dividerColumn()

			m = press(t, m, click(x, tc.y))
			m = press(t, m, motion(x+10, tc.y))

			if got := m.sidebarWidth(); got != before {
				t.Errorf("sidebarWidth() = %d, want it left at %d", got, before)
			}
		})
	}
}

func TestMouse_MotionAndReleaseWithoutADrag_ChangeNothing(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, readyFiles))
	m := startModel(t, options, wideWidth, wideHeight)
	before := m.sidebarWidth()

	m = press(t, m, motion(m.dividerColumn()+10, dragRow))
	m = press(t, m, release(m.dividerColumn()+10, dragRow))

	if got := m.sidebarWidth(); got != before {
		t.Errorf("sidebarWidth() = %d, want it left at %d", got, before)
	}
	if got := readStoredRatio(t, options.UserStatePath); got != 0 {
		t.Errorf("stored sidebar ratio = %v, want none written", got)
	}
}

func TestMouse_DragOnANarrowTerminalOrUnderHelp_DoesNothing(t *testing.T) {
	cases := []struct {
		name   string
		width  int
		height int
		keys   []tea.KeyPressMsg
	}{
		{name: "narrow terminal", width: smallWidth, height: smallHeight},
		{name: "help open", width: wideWidth, height: wideHeight, keys: []tea.KeyPressMsg{runeKey('?')}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			options := newTestOptions(t, newTestStore(t, readyFiles))
			m := startModel(t, options, tc.width, tc.height)
			for _, key := range tc.keys {
				m = press(t, m, key)
			}
			from := m.dividerColumn()

			m = dragDivider(t, m, from, from+10)

			if m.sidebarRatio != 0 {
				t.Errorf("sidebarRatio = %v, want 0: no drag", m.sidebarRatio)
			}
			if got := readStoredRatio(t, options.UserStatePath); got != 0 {
				t.Errorf("stored sidebar ratio = %v, want none written", got)
			}
		})
	}
}

func TestMouse_MidDrag_MatchesANSIGolden(t *testing.T) {
	m := newThemeModel(t, themeFiles, "ui:\n  theme: dark\n", false, nil)
	from := m.dividerColumn()
	m = press(t, m, click(from, dragRow))

	m = press(t, m, motion(from+10, dragRow))

	goldenANSI(t, "drag-divider-120x30", m.View())
}

// storedWidthAt is the sidebar width the double-click tests store in the state file, as its ratio
// of wideWidth, before the model starts, wide enough that a reset to the default width shows.
const storedWidthAt = 60

// testClock is a clock a test moves on by hand, which stands in for time.Now in a model.
type testClock struct {
	current time.Time
}

// now returns the clock's time.
func (c *testClock) now() time.Time {
	return c.current
}

// advance moves the clock on by duration.
func (c *testClock) advance(duration time.Duration) {
	c.current = c.current.Add(duration)
}

// startClockModel returns the model of options started at the wide size, on a testClock the test
// moves on by hand, and that clock.
func startClockModel(t *testing.T, options Options) (model, *testClock) {
	t.Helper()
	m := startModel(t, options, wideWidth, wideHeight)
	clock := &testClock{current: time.Date(2026, time.October, 16, 12, 0, 0, 0, time.UTC)}
	m.now = clock.now
	return m, clock
}

// pressDividerOnce presses the left button on the divider and lets it go without moving.
func pressDividerOnce(t *testing.T, m model) model {
	t.Helper()
	x := m.dividerColumn()
	m = press(t, m, click(x, dragRow))
	return press(t, m, release(x, dragRow))
}

// assertNoStoredRatio fails the test unless the state file at path holds no sidebar_ratio key.
func assertNoStoredRatio(t *testing.T, path string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(content), "sidebar_ratio") {
		t.Errorf("state file = %q, want no sidebar_ratio key", content)
	}
}

func TestMouse_DoubleClickOnTheDivider_ResetsTheWidthAndClearsTheStoredOne(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, readyFiles))
	m, clock := startClockModel(t, options)
	defaultWidth := m.sidebarWidth()
	from := m.dividerColumn()
	m = dragDivider(t, m, from, from+storedWidthAt-defaultWidth)
	if got, want := readStoredRatio(t, options.UserStatePath), widthRatio(storedWidthAt, wideWidth); got != want {
		t.Fatalf("stored sidebar ratio after the drag = %v, want %v", got, want)
	}
	clock.advance(doubleClickWindow + time.Millisecond)

	m = pressDividerOnce(t, m)
	clock.advance(200 * time.Millisecond)
	m = pressDividerOnce(t, m)

	if got := m.sidebarWidth(); got != defaultWidth {
		t.Errorf("sidebarWidth() = %d, want the default %d", got, defaultWidth)
	}
	assertNoStoredRatio(t, options.UserStatePath)
}

func TestMouse_DoubleClickOnTheDivider_StartsNoDrag(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, readyFiles))
	writeTestFile(t, options.UserStatePath, ratioState(storedWidthAt, wideWidth))
	m, clock := startClockModel(t, options)
	x := m.dividerColumn()
	m = pressDividerOnce(t, m)
	clock.advance(200 * time.Millisecond)
	m = press(t, m, click(x, dragRow))
	defaultWidth := m.sidebarWidth()

	m = press(t, m, motion(x+10, dragRow))
	m = press(t, m, release(x+10, dragRow))

	if got := m.sidebarWidth(); got != defaultWidth {
		t.Errorf("sidebarWidth() = %d, want it left at the default %d: no drag", got, defaultWidth)
	}
	assertNoStoredRatio(t, options.UserStatePath)
}

func TestMouse_PressRightAfterADragThatMovedTheDivider_StartsADrag(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, readyFiles))
	m, clock := startClockModel(t, options)
	before := m.sidebarWidth()
	from := m.dividerColumn()
	m = dragDivider(t, m, from, from+10)
	clock.advance(200 * time.Millisecond)

	m = dragDivider(t, m, from+10, from+15)

	if got := m.sidebarWidth(); got != before+15 {
		t.Errorf("sidebarWidth() = %d, want %d: a drag, no reset", got, before+15)
	}
	if got, want := readStoredRatio(t, options.UserStatePath), widthRatio(before+15, wideWidth); got != want {
		t.Errorf("stored sidebar ratio = %v, want %v", got, want)
	}
}

func TestMouse_TwoPressesOnTheDividerThatAreNoDoubleClick_KeepTheWidth(t *testing.T) {
	cases := []struct {
		name             string
		gap              time.Duration
		isSidebarPressed bool
	}{
		{name: "600 ms apart", gap: 600 * time.Millisecond},
		{name: "a sidebar row pressed between", gap: 100 * time.Millisecond, isSidebarPressed: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			options := newTestOptions(t, newTestStore(t, readyFiles))
			writeTestFile(t, options.UserStatePath, ratioState(storedWidthAt, wideWidth))
			m, clock := startClockModel(t, options)

			m = pressDividerOnce(t, m)
			clock.advance(tc.gap)
			if tc.isSidebarPressed {
				m = press(t, m, click(2, dragRow))
				clock.advance(tc.gap)
			}
			m = pressDividerOnce(t, m)

			if got := m.sidebarWidth(); got != storedWidthAt {
				t.Errorf("sidebarWidth() = %d, want it left at %d", got, storedWidthAt)
			}
			if got, want := readStoredRatio(t, options.UserStatePath), widthRatio(storedWidthAt, wideWidth); got != want {
				t.Errorf("stored sidebar ratio = %v, want it left at %v", got, want)
			}
		})
	}
}

func TestMouse_DragThenATerminalResize_KeepsTheDraggedRatio(t *testing.T) {
	m := newTestModel(t, newTestStore(t, readyFiles), wideWidth, wideHeight)
	from := m.dividerColumn()
	m = dragDivider(t, m, from, from+60-m.sidebarWidth())

	m = press(t, m, tea.WindowSizeMsg{Width: 240, Height: wideHeight})

	if got := m.sidebarWidth(); got != 120 {
		t.Errorf("sidebarWidth() = %d, want 120: 60 of 120 columns kept at 240", got)
	}
}

func TestMouse_DoubleClickResetThenATerminalResize_FollowsTheDefault(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, readyFiles))
	writeTestFile(t, options.UserStatePath, ratioState(storedWidthAt, wideWidth))
	m, clock := startClockModel(t, options)
	m = pressDividerOnce(t, m)
	clock.advance(200 * time.Millisecond)
	m = pressDividerOnce(t, m)

	m = press(t, m, tea.WindowSizeMsg{Width: 200, Height: wideHeight})

	if got := m.sidebarWidth(); got != 48 {
		t.Errorf("sidebarWidth() = %d, want the default 48 at 200 columns", got)
	}
}

func TestMouse_DragBackToThePressColumn_RestoresTheRatioAndSavesNothing(t *testing.T) {
	const width = 97
	options := newTestOptions(t, newTestStore(t, readyFiles))
	writeTestFile(t, options.UserStatePath, "ui:\n  sidebar_ratio: 0.3\n")
	m := startModel(t, options, width, wideHeight)
	if got := m.sidebarWidth(); got != 29 {
		t.Fatalf("sidebarWidth() = %d, want 29: 0.3 of %d columns", got, width)
	}
	asked := m.stateSaver.asked
	from := m.dividerColumn()

	m = press(t, m, click(from, dragRow))
	m = press(t, m, motion(from+1, dragRow))
	m = press(t, m, motion(from, dragRow))
	m = press(t, m, release(from, dragRow))

	if m.sidebarRatio != 0.3 {
		t.Errorf("sidebarRatio = %v, want the stored 0.3 back", m.sidebarRatio)
	}
	if m.stateSaver.asked != asked {
		t.Errorf("saves asked = %d, want %d: a drag with no net move saves nothing", m.stateSaver.asked, asked)
	}
}
