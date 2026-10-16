package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// The layout's dimensions. Below narrowWidth columns the panes stop sharing the screen: the
// sidebar fills it, and Enter swaps in the detail pane.
const (
	narrowWidth     = 90
	sidebarShare    = 3
	sidebarMinWidth = 28
	sidebarMaxWidth = 48
	// frameRows are the rows around the panes: the tab bar, the rule under the panes and the
	// status bar.
	frameRows = 3
	// dividerMargin is the blank columns on either side of the separator between the panes.
	dividerMargin = 1
)

// The glyphs of the frame.
const (
	tabSeparator  = " │ "
	paneSeparator = "│"
	ruleGlyph     = "─"
	ruleJoint     = "┴"
	truncateTail  = "…"
)

// frame lays out the tab bar, the panes or the help screen, the rule and the status bar over the
// terminal's full size. A terminal shorter than the frame shows its top rows.
func (m model) frame() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	rows := make([]string, 0, m.bodyHeight()+frameRows)
	rows = append(rows, fit(m.tabBar(), m.width))
	rows = append(rows, m.body()...)
	rows = append(rows, m.rule(), m.statusBar())
	return strings.Join(rows[:min(len(rows), m.height)], "\n")
}

// isNarrow reports a terminal too narrow for the sidebar and the detail pane side by side.
func (m model) isNarrow() bool {
	return m.width < narrowWidth
}

// isSplit reports the sidebar and the detail pane side by side: a wide terminal, help closed.
func (m model) isSplit() bool {
	return !m.isNarrow() && !m.isHelpOpen
}

// bodyHeight returns the rows between the tab bar and the rule.
func (m model) bodyHeight() int {
	return max(m.height-frameRows, 0)
}

// sidebarWidth returns the sidebar's columns in the split layout: a third of the terminal, within
// sidebarMinWidth and sidebarMaxWidth.
func (m model) sidebarWidth() int {
	return min(max(m.width/sidebarShare, sidebarMinWidth), sidebarMaxWidth)
}

// body draws the bodyHeight rows under the tab bar: the help screen when it is open, the two
// panes side by side on a wide terminal, and on a narrow one the sidebar or, once Enter opened
// it or while the status picker is open, the detail pane.
func (m model) body() []string {
	height := m.bodyHeight()
	switch {
	case m.isHelpOpen:
		return paneRows(helpLines, 0, height, m.width)
	case m.isNarrow() && (m.isDetailOpen || m.picker.isOpen):
		lines, offset := m.detailRows()
		return paneRows(lines, offset, height, m.width)
	case m.isNarrow():
		return paneRows(m.sidebarLines(), 0, height, m.width)
	}

	sidebarWidth := m.sidebarWidth()
	sidebar := paneRows(m.sidebarLines(), 0, height, sidebarWidth)
	lines, offset := m.detailRows()
	detail := paneRows(lines, offset, height, m.detailWidth())
	margin := strings.Repeat(" ", dividerMargin)
	divider := margin + m.theme.border.Render(paneSeparator) + margin
	rows := make([]string, 0, height)
	for row := range height {
		rows = append(rows, pad(sidebar[row], sidebarWidth)+divider+detail[row])
	}
	return rows
}

// dividerColumn returns the screen column of the separator between the panes in the split
// layout: past the sidebar and the margin left of it.
func (m model) dividerColumn() int {
	return m.sidebarWidth() + dividerMargin
}

// detailLeft returns the screen column the detail pane starts at: the first on a narrow terminal,
// past the sidebar, the separator and the margins either side of it on a wide one.
func (m model) detailLeft() int {
	if m.isNarrow() {
		return 0
	}
	return m.dividerColumn() + ansi.StringWidth(paneSeparator) + dividerMargin
}

// detailRows returns the rows the detail pane draws and the first of them on screen: the status
// picker's while it is open, the shown issue's, Note's or dependency view's otherwise.
func (m model) detailRows() ([]string, int) {
	if m.picker.isOpen {
		return m.picker.lines(m.theme.selectedRow), 0
	}
	return m.detail, m.detailOffset
}

// paneRows returns height rows of lines from offset on, each cut to width columns, blank rows
// filling the pane below the last line.
func paneRows(lines []string, offset, height, width int) []string {
	rows := make([]string, 0, height)
	for row := range height {
		line := ""
		if offset+row < len(lines) {
			line = fit(lines[offset+row], width)
		}
		rows = append(rows, line)
	}
	return rows
}

// rule draws the line under the panes, joined to the separator between them in the split
// layout, in the border style.
func (m model) rule() string {
	if !m.isSplit() {
		return m.theme.border.Render(strings.Repeat(ruleGlyph, m.width))
	}
	jointColumn := m.dividerColumn()
	return m.theme.border.Render(strings.Repeat(ruleGlyph, jointColumn) + ruleJoint +
		strings.Repeat(ruleGlyph, max(m.width-jointColumn-ansi.StringWidth(ruleJoint), 0)))
}

// tabBar lists the tabs in tabOrder, the open one marked by the theme's active tab style, the
// separators in its border style.
func (m model) tabBar() string {
	labels := make([]string, 0, len(tabOrder))
	for _, tab := range tabOrder {
		label := tabBarLabel(tab)
		if tab == m.tab {
			label = m.theme.activeTab.Render(label)
		}
		labels = append(labels, label)
	}
	return strings.Join(labels, m.theme.border.Render(tabSeparator))
}

// tabBarLabel returns tab's label as the tab bar draws it, unstyled: its name padded by a space
// on either side.
func tabBarLabel(tab Tab) string {
	return " " + tabLabels[tab] + " "
}

// tabAt returns the tab whose label covers column col of the tab bar, and false for a column on a
// separator or past the last label.
func tabAt(col int) (Tab, bool) {
	start := 0
	for _, tab := range tabOrder {
		end := start + ansi.StringWidth(tabBarLabel(tab))
		if col >= start && col < end {
			return tab, true
		}
		start = end + ansi.StringWidth(tabSeparator)
	}
	return TabReady, false
}

// screenPane names the part of the screen a mouse event lands on.
type screenPane int

// The parts of the screen the mouse acts on; every other cell, such as the separator between the
// panes and its margins, the rule and the status bar, is paneNone.
const (
	paneNone screenPane = iota
	paneTabBar
	paneSidebar
	paneDetail
)

// panePoint is a screen cell located in the pane that draws it: the pane, and the cell's column
// and row counted from that pane's top-left corner.
type panePoint struct {
	pane screenPane
	col  int
	row  int
}

// pointAt locates the screen cell at column x and row y in the layout body draws: the tab bar on
// the top row, then on a wide terminal the sidebar left of the separator's margin and the detail
// pane right of it, and on a narrow one whichever of the two fills the screen. With the help screen
// open no pane is on screen.
func (m model) pointAt(x, y int) panePoint {
	if x < 0 || y < 0 || x >= m.width {
		return panePoint{}
	}
	if y == 0 {
		return panePoint{pane: paneTabBar, col: x}
	}
	row := y - 1
	switch {
	case row >= m.bodyHeight() || m.isHelpOpen:
		return panePoint{}
	case m.isNarrow() && (m.isDetailOpen || m.picker.isOpen):
		return panePoint{pane: paneDetail, col: x, row: row}
	case m.isNarrow() || x < m.sidebarWidth():
		return panePoint{pane: paneSidebar, col: x, row: row}
	case x < m.detailLeft():
		return panePoint{}
	}
	return panePoint{pane: paneDetail, col: x - m.detailLeft(), row: row}
}

// fit cuts text to width columns, ending a cut line with an ellipsis.
func fit(text string, width int) string {
	return ansi.Truncate(text, max(width, 0), truncateTail)
}

// pad fills text with spaces to width columns; text already that wide is returned as is.
func pad(text string, width int) string {
	return text + strings.Repeat(" ", max(width-ansi.StringWidth(text), 0))
}
