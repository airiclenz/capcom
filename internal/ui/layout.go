package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
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
)

// The glyphs of the frame.
const (
	tabSeparator  = " │ "
	paneSeparator = "│"
	ruleGlyph     = "─"
	ruleJoint     = "┴"
	truncateTail  = "…"
)

// activeTabStyle marks the open tab on the tab bar.
var activeTabStyle = lipgloss.NewStyle().Bold(true).Reverse(true)

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
// it, the detail pane.
func (m model) body() []string {
	height := m.bodyHeight()
	switch {
	case m.isHelpOpen:
		return paneRows(helpLines, 0, height, m.width)
	case m.isNarrow() && m.isDetailOpen:
		return paneRows(m.detail, m.detailOffset, height, m.width)
	case m.isNarrow():
		return paneRows(m.sidebar, 0, height, m.width)
	}

	sidebarWidth := m.sidebarWidth()
	sidebar := paneRows(m.sidebar, 0, height, sidebarWidth)
	detail := paneRows(m.detail, m.detailOffset, height, m.width-sidebarWidth-1)
	rows := make([]string, 0, height)
	for row := range height {
		rows = append(rows, pad(sidebar[row], sidebarWidth)+paneSeparator+detail[row])
	}
	return rows
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
// layout.
func (m model) rule() string {
	if !m.isSplit() {
		return strings.Repeat(ruleGlyph, m.width)
	}
	sidebarWidth := m.sidebarWidth()
	return strings.Repeat(ruleGlyph, sidebarWidth) + ruleJoint +
		strings.Repeat(ruleGlyph, max(m.width-sidebarWidth-1, 0))
}

// tabBar lists the tabs in tabOrder, the open one marked by activeTabStyle.
func (m model) tabBar() string {
	labels := make([]string, 0, len(tabOrder))
	for _, tab := range tabOrder {
		label := " " + tabLabels[tab] + " "
		if tab == m.tab {
			label = activeTabStyle.Render(label)
		}
		labels = append(labels, label)
	}
	return strings.Join(labels, tabSeparator)
}

// fit cuts text to width columns, ending a cut line with an ellipsis.
func fit(text string, width int) string {
	return ansi.Truncate(text, max(width, 0), truncateTail)
}

// pad fills text with spaces to width columns; text already that wide is returned as is.
func pad(text string, width int) string {
	return text + strings.Repeat(" ", max(width-ansi.StringWidth(text), 0))
}
