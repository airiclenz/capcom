package ui

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

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
