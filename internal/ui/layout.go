package ui

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/airiclenz/capcom/internal/outline"
)

// The layout's dimensions. Below narrowWidth columns the panes stop sitting side by side: a body
// of stackBodyMinRows rows or more stacks them, the sidebar above the detail pane, and a shorter
// one shows one pane at a time, the sidebar filling the screen until Enter swaps in the detail
// pane.
const (
	narrowWidth     = 90
	sidebarShare    = 3
	sidebarMinWidth = 28
	sidebarMaxWidth = 48
	// sidebarDragMinWidth and detailMinWidth bound a sidebar width the user sets: the sidebar
	// keeps sidebarDragMinWidth columns and the detail pane, from detailLeft to the right edge,
	// detailMinWidth. sidebarStep is the columns < and > move the divider by.
	sidebarDragMinWidth = 20
	detailMinWidth      = 40
	sidebarStep         = 4
	// frameRows are the rows around the panes: the tab bar, the rules above and under the panes
	// and the status bar.
	frameRows = 4
	// dividerMargin is the blank columns on either side of the separator between the panes.
	dividerMargin = 1
	// bodyTop is the screen row the panes start on: past the tab bar and the rule under it.
	bodyTop = 2
	// stackSidebarMinRows and stackDetailMinRows are the fewest rows the stacked layout gives the
	// sidebar and the detail pane, stackDividerRows the rule between them; stackBodyMinRows, all
	// three together, is the shortest body that stacks. Unless the user set its height, the
	// sidebar takes a stackedShare part of the body's rows. stackedStep is the rows < and > move
	// the stacked layout's divider by.
	stackSidebarMinRows = 8
	stackDetailMinRows  = 12
	stackDividerRows    = 1
	stackBodyMinRows    = stackSidebarMinRows + stackDividerRows + stackDetailMinRows
	stackedShare        = 3
	stackedStep         = 2
)

// The glyphs of the frame.
const (
	tabSeparator  = " │ "
	paneSeparator = "│"
	ruleGlyph     = "─"
	ruleJoint     = "┴"
	topRuleJoint  = "┬"
	truncateTail  = "…"
)

// The tab bar's version label, `v<version> · capcom`, led by the worktree path and
// pathLabelSeparator when it fits, and the fewest blank columns it keeps between itself and the
// last tab. homeLabel stands for the home directory in that path.
const (
	versionLabelPrefix = "v"
	versionLabelSuffix = " · capcom"
	versionLabelMinGap = 2
	pathLabelSeparator = " · "
	homeLabel          = "~"
)

// The count a tab's label ends in once its read is in: the rows the tab lists with no filter, and
// with a filter in effect the matched rows it shows over them.
const (
	tabCountFormat         = " [%d]"
	tabFilteredCountFormat = " [%d/%d]"
)

// frame lays out the tab bar, the rule under it, the panes or the help screen, the rule under
// them and the status bar over the terminal's full size. A terminal shorter than the frame shows
// its top rows.
func (m model) frame() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	rows := make([]string, 0, m.bodyHeight()+frameRows)
	rows = append(rows, fit(m.tabBar(), m.width), m.rule(topRuleJoint))
	rows = append(rows, m.body()...)
	rows = append(rows, m.rule(ruleJoint), m.statusBar())
	return strings.Join(rows[:min(len(rows), m.height)], "\n")
}

// hasFullWidthPanes reports panes as wide as the screen: a terminal too narrow for the sidebar and
// the detail pane side by side, whether it shows one pane at a time or stacks them.
func (m model) hasFullWidthPanes() bool {
	return m.width < narrowWidth
}

// canStack reports a terminal sized for the stacked layout: too narrow for the panes side by side
// and with a body of stackBodyMinRows rows or more. The help screen covering the panes leaves
// their sizes as they are, so it does not count here.
func (m model) canStack() bool {
	return m.hasFullWidthPanes() && m.bodyHeight() >= stackBodyMinRows
}

// isSinglePane reports the one-pane layout: a terminal too narrow for the panes side by side and
// too short to stack them, the sidebar filling the screen until Enter swaps in the detail pane.
func (m model) isSinglePane() bool {
	return m.hasFullWidthPanes() && !m.canStack()
}

// isStacked reports the sidebar above the detail pane: a narrow terminal tall enough to stack
// them, help closed.
func (m model) isStacked() bool {
	return m.canStack() && !m.isHelpOpen
}

// isSplit reports the sidebar and the detail pane side by side: a wide terminal, help closed.
func (m model) isSplit() bool {
	return !m.hasFullWidthPanes() && !m.isHelpOpen
}

// bodyHeight returns the rows between the rule under the tab bar and the rule under the panes.
func (m model) bodyHeight() int {
	return max(m.height-frameRows, 0)
}

// stackedSidebarRows returns the sidebar's rows in the stacked layout: the ratio the user set
// times the body's rows, rounded, or else a stackedShare part of them, rounded down, either
// clamped by clampStackedRows.
func (m model) stackedSidebarRows() int {
	desired := m.bodyHeight() / stackedShare
	if m.stackedRatio > 0 {
		desired = int(math.Round(m.stackedRatio * float64(m.bodyHeight())))
	}
	return m.clampStackedRows(desired)
}

// clampStackedRows returns rows within the stacked sidebar's limits at the body's height: at least
// stackSidebarMinRows, and at most what leaves the divider and stackDetailMinRows rows under it.
func (m model) clampStackedRows(rows int) int {
	upper := m.bodyHeight() - stackDividerRows - stackDetailMinRows
	return max(min(rows, upper), stackSidebarMinRows)
}

// resizeStacked moves the stacked layout's divider by delta rows, down when positive, within the
// sidebar's and the detail pane's minimums, and repaints the panes at the new heights. The height
// is kept as the user's ratio of the body's rows, so it holds when the terminal resizes. It does
// nothing outside the stacked layout, and leaves the ratio as it was when the limits keep the
// height where it is.
func (m model) resizeStacked(delta int) model {
	if !m.isStacked() {
		return m
	}
	rows := m.clampStackedRows(m.stackedSidebarRows() + delta)
	if rows == m.stackedSidebarRows() {
		return m
	}
	m.stackedRatio = m.rowRatioOf(rows)
	return m.relayout()
}

// rowRatioOf returns rows as a fraction of the body's rows, the form stackedRatio keeps a height
// in. Only the stacked layout calls it, so the body is never empty.
func (m model) rowRatioOf(rows int) float64 {
	return float64(rows) / float64(m.bodyHeight())
}

// dividerRatio returns the ratio the divider of one layout keeps: stackedRatio for the stacked
// layout's when isStacked, sidebarRatio for the split layout's otherwise.
func (m model) dividerRatio(isStacked bool) float64 {
	if isStacked {
		return m.stackedRatio
	}
	return m.sidebarRatio
}

// withDividerRatio returns the model with ratio as the ratio the divider of one layout keeps:
// stackedRatio when isStacked, sidebarRatio otherwise.
func (m model) withDividerRatio(isStacked bool, ratio float64) model {
	if isStacked {
		m.stackedRatio = ratio
		return m
	}
	m.sidebarRatio = ratio
	return m
}

// hasDivider reports the divider of one layout on screen: the stacked layout's when isStacked,
// the split layout's otherwise.
func (m model) hasDivider(isStacked bool) bool {
	if isStacked {
		return m.isStacked()
	}
	return m.isSplit()
}

// sidebarPaneRows returns the sidebar pane's rows: its share of the body when the panes stack,
// the whole body otherwise.
func (m model) sidebarPaneRows() int {
	if m.canStack() {
		return m.stackedSidebarRows()
	}
	return m.bodyHeight()
}

// detailPaneRows returns the detail pane's rows: what the sidebar and the divider leave of the
// body when the panes stack, the whole body otherwise.
func (m model) detailPaneRows() int {
	if m.canStack() {
		return m.bodyHeight() - m.stackedSidebarRows() - stackDividerRows
	}
	return m.bodyHeight()
}

// sidebarWidth returns the sidebar's columns in the split layout: the ratio the user set times the
// terminal's width, rounded, or else a third of the terminal within sidebarMinWidth and
// sidebarMaxWidth, either clamped by clampSidebarWidth to the terminal's width.
func (m model) sidebarWidth() int {
	desired := min(max(m.width/sidebarShare, sidebarMinWidth), sidebarMaxWidth)
	if m.sidebarRatio > 0 {
		desired = int(math.Round(m.sidebarRatio * float64(m.width)))
	}
	return m.clampSidebarWidth(desired)
}

// clampSidebarWidth returns columns within the sidebar's limits at the terminal's width: at least
// sidebarDragMinWidth, and at most what leaves detailMinWidth columns right of the divider and its
// margins. The lower limit wins on a terminal too narrow for both.
func (m model) clampSidebarWidth(columns int) int {
	upper := m.width - 2*dividerMargin - ansi.StringWidth(paneSeparator) - detailMinWidth
	return max(min(columns, upper), sidebarDragMinWidth)
}

// resizeSidebar moves the divider by delta columns, right when positive, within the sidebar's
// limits, and repaints the panes at the new width. The width is kept as the user's ratio of the
// terminal's width, so it holds when the terminal resizes. It does nothing outside the split
// layout, the one-pane and the stacked ones alike, and leaves the ratio as it was when the limits
// keep the width where it is.
func (m model) resizeSidebar(delta int) model {
	if !m.isSplit() {
		return m
	}
	columns := m.clampSidebarWidth(m.sidebarWidth() + delta)
	if columns == m.sidebarWidth() {
		return m
	}
	m.sidebarRatio = m.ratioOf(columns)
	return m.relayout()
}

// ratioOf returns columns as a fraction of the terminal's width, the form sidebarRatio keeps a
// width in. Only the split layout calls it, so the width is never 0.
func (m model) ratioOf(columns int) float64 {
	return float64(columns) / float64(m.width)
}

// body draws the bodyHeight rows under the tab bar's rule: the help screen when it is open, the two
// panes side by side on a wide terminal, on a narrow one tall enough the sidebar above the
// detail pane, and on a narrow, short one the sidebar or, once Enter opened it or while the
// status picker is open, the detail pane. Each pane ends in its scrollbar.
func (m model) body() []string {
	height := m.bodyHeight()
	switch {
	case m.isHelpOpen:
		return paneRows(helpLines, 0, height, m.width)
	case m.isSinglePane() && (m.isDetailOpen || m.picker.isOpen):
		lines, offset := m.detailRows()
		return m.scrolledPaneRows(lines, offset, height, m.detailWidth())
	case m.isSinglePane():
		return m.sidebarRows(height)
	case m.isStacked():
		return m.stackedBody()
	}

	sidebar := m.sidebarRows(height)
	lines, offset := m.detailRows()
	detail := m.scrolledPaneRows(lines, offset, height, m.detailWidth())
	margin := strings.Repeat(" ", dividerMargin)
	divider := margin + m.dividerStyle().Render(paneSeparator) + margin
	rows := make([]string, 0, height)
	for row := range height {
		rows = append(rows, sidebar[row]+divider+detail[row])
	}
	return rows
}

// stackedBody returns the stacked layout's body: the sidebar's stackedSidebarRows rows, the
// divider, a full-width rule in the divider's style, then the detail pane's detailPaneRows rows.
func (m model) stackedBody() []string {
	rows := make([]string, 0, m.bodyHeight())
	rows = append(rows, m.sidebarRows(m.stackedSidebarRows())...)
	rows = append(rows, m.dividerStyle().Render(strings.Repeat(ruleGlyph, m.width)))
	lines, offset := m.detailRows()
	return append(rows, m.scrolledPaneRows(lines, offset, m.detailPaneRows(), m.detailWidth())...)
}

// stackedDividerRow returns the screen row of the stacked layout's divider: under the tab bar's
// rule and the sidebar.
func (m model) stackedDividerRow() int {
	return bodyTop + m.stackedSidebarRows()
}

// dividerColumn returns the screen column of the separator between the panes in the split
// layout: past the sidebar and the margin left of it.
func (m model) dividerColumn() int {
	return m.sidebarWidth() + dividerMargin
}

// detailLeft returns the screen column the detail pane starts at: the first on a narrow terminal,
// one pane or stacked, past the sidebar, the separator and the margins either side of it on a
// wide one.
func (m model) detailLeft() int {
	if m.hasFullWidthPanes() {
		return 0
	}
	return m.dividerColumn() + ansi.StringWidth(paneSeparator) + dividerMargin
}

// detailRows returns the rows the detail pane draws and the first of them on screen: the status
// picker's while it is open, the shown issue's, Note's or dependency view's otherwise, the detail
// selection shaded in the selected row's style over the rows on screen.
func (m model) detailRows() ([]string, int) {
	if m.picker.isOpen {
		return m.picker.lines(m.theme.selectedRow), 0
	}
	lines := m.sel.paint(
		m.markdownTheme.Measure,
		m.detail,
		m.detailOffset,
		m.detailPaneRows(),
		m.detailWidth(),
		m.theme.selectedRow,
	)
	return lines, m.detailOffset
}

// sidebarRows returns the sidebar pane's height rows, each its text cut and padded to
// sidebarPaneWidth columns, then the scrollbar: the filter line on top while it is open, a blank
// cell beside it, then the list, the bar beside it standing for the list's top row among its
// total rows.
func (m model) sidebarRows(height int) []string {
	cells := make([]string, 0, height)
	if m.isFilterOpen {
		cells = append(cells, scrollBlankCell)
	}
	cells = append(cells, scrollbarCells(m.theme, m.sidebarTop, m.sidebarTotal, m.sidebarHeight())...)
	return m.barredRows(m.sidebarLines(), cells[:min(len(cells), height)], m.sidebarPaneWidth())
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

// rule draws a line across the screen in the border style: above the panes with topRuleJoint or
// under them with ruleJoint as joint, the joint meeting the separator between the panes in the
// split layout, drawn in the divider's style while it is dragged. Outside it the line has no
// joint.
func (m model) rule(joint string) string {
	if !m.isSplit() {
		return m.theme.border.Render(strings.Repeat(ruleGlyph, m.width))
	}
	jointColumn := m.dividerColumn()
	left := strings.Repeat(ruleGlyph, jointColumn)
	right := strings.Repeat(ruleGlyph, max(m.width-jointColumn-ansi.StringWidth(joint), 0))
	if !m.drag.isActive {
		// One styled run, so the frame's bytes stay as they were before dragging existed.
		return m.theme.border.Render(left + joint + right)
	}
	return m.theme.border.Render(left) + m.theme.dividerActive.Render(joint) + m.theme.border.Render(right)
}

// dividerStyle returns the style of the separator between the panes: the theme's dividerActive
// while the divider is dragged, its border style otherwise.
func (m model) dividerStyle() lipgloss.Style {
	if m.drag.isActive {
		return m.theme.dividerActive
	}
	return m.theme.border
}

// tabBar lists the tabs in tabOrder, each labelled as tabBarLabel draws it with its count, the
// open one marked by the theme's active tab style, the separators in its border style, and ends
// in the version label in the theme's hints style, its last character in the frame's last column: `<path> · v<version> · capcom`, the worktree path as
// displayPath names it, or `v<version> · capcom` when the path does not fit. The label is left
// out when the model has no version or when even the shorter one would leave fewer than
// versionLabelMinGap columns after the last tab, so the tabs are never cut for it.
func (m model) tabBar() string {
	labels := make([]string, 0, len(tabOrder))
	for _, tab := range tabOrder {
		label := m.tabBarLabel(tab)
		if tab == m.tab {
			label = m.theme.activeTab.Render(label)
		}
		labels = append(labels, label)
	}
	tabs := strings.Join(labels, m.theme.border.Render(tabSeparator))
	if m.version == "" {
		return tabs
	}
	versionLabel := versionLabelPrefix + m.version + versionLabelSuffix
	for _, label := range []string{m.worktreePath + pathLabelSeparator + versionLabel, versionLabel} {
		gap := m.width - ansi.StringWidth(tabs) - ansi.StringWidth(label)
		if gap >= versionLabelMinGap {
			return tabs + strings.Repeat(" ", gap) + m.theme.hints.Render(label)
		}
	}
	return tabs
}

// displayPath names root as the tab bar shows it: homeLabel when root is home, homeLabel and the
// rest of root after an OS separator when root lies under home, and root unchanged otherwise.
// Both are compared component by component, so a sibling such as /home/userx is never under
// /home/user; an empty home shortens nothing.
func displayPath(root, home string) string {
	if home == "" {
		return root
	}
	relative, err := filepath.Rel(home, root)
	if err != nil || !filepath.IsLocal(relative) {
		return root
	}
	if relative == "." {
		return homeLabel
	}
	return homeLabel + string(filepath.Separator) + relative
}

// tabBarLabel returns tab's label as the tab bar draws it, unstyled, padded by a space on either
// side: its name, then `[n]` once tabCount counts it, or `[m/n]` while a filter is in effect.
func (m model) tabBarLabel(tab Tab) string {
	label := tabLabels[tab]
	matched, total, isCounted := m.tabCount(tab)
	switch {
	case !isCounted:
	case m.filters[tab].isActive():
		label += fmt.Sprintf(tabFilteredCountFormat, matched, total)
	default:
		label += fmt.Sprintf(tabCountFormat, total)
	}
	return " " + label + " "
}

// tabCount returns the counts of tab's label: total, the rows the tab lists with no filter, and
// matched, the issues or Notes the filter in effect matches that the tab shows, 0 when the filter
// has a problem or an error. Ready counts its queue; Issues the issues outside a closed category,
// or every issue while a status term names a closed status, since the tab then shows them;
// Knowledge the Notes its tree shows, outside retired subtrees and not background; and Closed the
// issues in a closed category. It reports
// false for a tab whose first read is not in or whose last read failed.
func (m model) tabCount(tab Tab) (matched int, total int, isCounted bool) {
	filter := m.filters[tab]
	switch tab {
	case TabReady:
		return m.ready.count()
	case TabClosed:
		return m.closed.count()
	case TabIssues:
		if !m.issues.isRead || m.issues.err != nil {
			return 0, 0, false
		}
		if !filter.isActive() {
			return 0, len(m.issues.tree.isShown), true
		}
		isClosedShown := filter.isClosedShown && filter.problem == "" && filter.err == nil
		visible := outline.Issues(m.issues.entries, isClosedShown)
		return countMatched(m.issues.tree.isShown, filter.matched), shownCount(visible), true
	case TabKnowledge:
		if !m.knowledge.isRead || m.knowledge.err != nil {
			return 0, 0, false
		}
		if !filter.isActive() {
			return 0, len(m.knowledge.tree.isShown), true
		}
		visible := noteOutline(m.knowledge.notes)
		return countMatched(m.knowledge.tree.isShown, filter.matched), shownCount(visible), true
	}
	return 0, 0, false
}

// shownCount returns the rows tree prints fully expanded.
func shownCount[E any](tree *outline.Tree[E]) int {
	return len(tree.Rows(nil, outline.UnlimitedDepth))
}

// countMatched returns how many of the ids isShown holds matched holds too.
func countMatched(isShown map[string]bool, matched map[string]bool) int {
	count := 0
	for id := range isShown {
		if matched[id] {
			count++
		}
	}
	return count
}

// tabAt returns the tab whose label covers column col of the tab bar, and false for a column on a
// separator or past the last label.
func (m model) tabAt(col int) (Tab, bool) {
	start := 0
	for _, tab := range tabOrder {
		end := start + ansi.StringWidth(m.tabBarLabel(tab))
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
// panes and its margins, the rules and the status bar, is paneNone.
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
// the top row, no pane on the rule under it, then on a wide terminal the sidebar left of the
// separator's margin and the detail pane right of it, on a narrow one tall enough the sidebar
// above the divider and the detail pane below it, and on a narrow, short one whichever of the two
// fills the screen. With the help screen open no pane is on screen.
func (m model) pointAt(x, y int) panePoint {
	if x < 0 || y < 0 || x >= m.width {
		return panePoint{}
	}
	if y == 0 {
		return panePoint{pane: paneTabBar, col: x}
	}
	row := y - bodyTop
	switch {
	case row < 0 || row >= m.bodyHeight() || m.isHelpOpen:
		return panePoint{}
	case m.isSinglePane() && (m.isDetailOpen || m.picker.isOpen):
		return panePoint{pane: paneDetail, col: x, row: row}
	case m.isSinglePane():
		return panePoint{pane: paneSidebar, col: x, row: row}
	case m.isStacked():
		return m.stackedPointAt(x, row)
	case x < m.sidebarWidth():
		return panePoint{pane: paneSidebar, col: x, row: row}
	case x < m.detailLeft():
		return panePoint{}
	}
	return panePoint{pane: paneDetail, col: x - m.detailLeft(), row: row}
}

// stackedPointAt locates the cell at column x on row of the stacked layout's body: the sidebar
// above the divider, no pane on the divider, and the detail pane under it, its row counted from
// the pane's top.
func (m model) stackedPointAt(x, row int) panePoint {
	sidebarRows := m.stackedSidebarRows()
	switch {
	case row < sidebarRows:
		return panePoint{pane: paneSidebar, col: x, row: row}
	case row < sidebarRows+stackDividerRows:
		return panePoint{}
	}
	return panePoint{pane: paneDetail, col: x, row: row - sidebarRows - stackDividerRows}
}

// isOnDivider reports the screen cell at column x and row y on the divider's hit area: in the
// split layout the separator between the panes or the margin either side of it, on a row of the
// panes; in the stacked layout the rule between the panes, across the screen.
func (m model) isOnDivider(x, y int) bool {
	if m.isStacked() {
		row := y - m.stackedDividerRow()
		return x >= 0 && x < m.width && row >= 0 && row < stackDividerRows
	}
	row := y - bodyTop
	return m.isSplit() && row >= 0 && row < m.bodyHeight() && x >= m.sidebarWidth() && x < m.detailLeft()
}

// fit cuts text to width columns, ending a cut line with an ellipsis.
func fit(text string, width int) string {
	return ansi.Truncate(text, max(width, 0), truncateTail)
}
