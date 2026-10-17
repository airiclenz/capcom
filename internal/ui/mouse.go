package ui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/index"
)

// detailWheelRows is how many rows one wheel notch scrolls the detail pane.
const detailWheelRows = 3

// doubleClickWindow is the longest time between two presses on the divider's hit area that makes
// them a double-click.
const doubleClickWindow = 500 * time.Millisecond

// isMouseIgnored reports an open help screen, filter line, status picker or prompt: each takes
// every key while it is open, so a click or a wheel notch cannot move what it acts on.
func (m model) isMouseIgnored() bool {
	return m.isHelpOpen || m.isFilterOpen || m.picker.isOpen || m.prompt.isOpen || m.textPrompt.isOpen
}

// dividerDrag is a drag of the divider between the panes, from a left press on its hit area to the
// release of the button. isStacked records the layout pressed: the stacked layout's divider,
// which moves along the rows, or the split layout's, which moves along the columns. grabOffset is
// the column pressed counted from the separator, or the row counted from the stacked rule, so the
// divider keeps that distance from the pointer and never jumps to it. startColumns is the
// sidebar's width at the press, startRows its height in the stacked layout, and startRatio is the
// pressed divider's ratio then: a motion back onto the start restores startRatio, and the release
// compares the ratio with it to tell whether the divider moved.
type dividerDrag struct {
	isActive     bool
	isStacked    bool
	grabOffset   int
	startColumns int
	startRows    int
	startRatio   float64
}

// mouseClick handles a left click: on the divider's hit area it starts a drag of the divider, or
// resets the sidebar's width, or its height in the stacked layout, when the click completes a
// double-click there, on a tab's label it opens that tab, on a sidebar row it selects that row, expanding or collapsing a
// tree node when the click lands on its arrow, and in the detail pane it selects a row of the
// dependency view or anchors the detail selection. Every press that is not ignored first clears
// the detail selection. Every other click, and every click while isMouseIgnored, does nothing.
func (m model) mouseClick(msg tea.MouseClickMsg) (model, tea.Cmd) {
	if m.isMouseIgnored() {
		return m, nil
	}
	m.sel = detailSel{}
	if msg.Button != tea.MouseLeft {
		return m, nil
	}
	if m.isOnDivider(msg.X, msg.Y) {
		return m.pressDivider(msg.X, msg.Y)
	}
	m.lastDividerPress = time.Time{}
	point := m.pointAt(msg.X, msg.Y)
	switch point.pane {
	case paneTabBar:
		tab, isOnTab := tabAt(point.col)
		if !isOnTab {
			return m, nil
		}
		return m.closeDepsIfOpen().openTab(tab)
	case paneSidebar:
		return m.clickSidebar(point)
	case paneDetail:
		return m.clickDetail(point)
	}
	return m, nil
}

// pressDivider handles a left press on the divider's hit area at column x, row y. Within
// doubleClickWindow of the last press there it resets the sidebar to its default size, its width
// in the split layout or its height in the stacked one, and returns the command that clears that
// divider's ratio in the state file, starting no drag; otherwise it records the press and starts
// a drag.
func (m model) pressDivider(x, y int) (model, tea.Cmd) {
	now := m.now()
	isDoubleClick := !m.lastDividerPress.IsZero() && now.Sub(m.lastDividerPress) <= doubleClickWindow
	if !isDoubleClick {
		m.lastDividerPress = now
		return m.startDrag(x, y), nil
	}
	m.lastDividerPress = time.Time{}
	m.drag = dividerDrag{}
	m = m.withDividerRatio(m.isStacked(), 0)
	return m.relayout(), m.saveState()
}

// startDrag starts a drag of the divider pressed at column x, row y: the stacked layout's when the
// panes stack, the split layout's otherwise.
func (m model) startDrag(x, y int) model {
	isStacked := m.isStacked()
	m.drag = dividerDrag{
		isActive:     true,
		isStacked:    isStacked,
		grabOffset:   x - m.dividerColumn(),
		startColumns: m.sidebarWidth(),
		startRows:    m.stackedSidebarRows(),
		startRatio:   m.dividerRatio(isStacked),
	}
	if isStacked {
		m.drag.grabOffset = y - m.stackedDividerRow()
	}
	return m
}

// abandonDrag ends a drag of the divider without saving it, its divider back on the ratio it had
// at the press, and forgets the press, so the next one never completes a double-click. A save
// writes both dividers' ratios, so a change it abandons must not linger to be written by a save on
// the other divider.
func (m model) abandonDrag() model {
	m = m.withDividerRatio(m.drag.isStacked, m.drag.startRatio)
	m.drag = dividerDrag{}
	m.lastDividerPress = time.Time{}
	return m
}

// abandonDragOffLayout abandons a drag of the divider, as abandonDrag does, once the layout it was
// pressed in is off screen, as after a terminal resize into another layout.
func (m model) abandonDragOffLayout() model {
	if !m.drag.isActive || m.hasDivider(m.drag.isStacked) {
		return m
	}
	return m.abandonDrag()
}

// mouseMotion moves the divider with the pointer while it is dragged, as dragDivider does, and
// otherwise extends the detail selection to the pointer while the left button drags it.
func (m model) mouseMotion(msg tea.MouseMotionMsg) model {
	if m.drag.isActive {
		return m.dragDivider(msg)
	}
	if msg.Button != tea.MouseLeft {
		return m
	}
	return m.extendSelection(msg.X, msg.Y)
}

// dragDivider moves the divider with the pointer in the layout it was pressed in, and does nothing
// once that layout is off screen. It moves the split layout's divider as dragSplitDivider does and
// the stacked layout's as dragStackedDivider does.
func (m model) dragDivider(msg tea.MouseMotionMsg) model {
	if !m.hasDivider(m.drag.isStacked) {
		return m
	}
	if m.drag.isStacked {
		return m.dragStackedDivider(msg)
	}
	return m.dragSplitDivider(msg)
}

// dragSplitDivider moves the split layout's divider with the pointer, keeping the column it was
// grabbed by under the pointer within the sidebar's limits, and repaints the panes when the width
// changed. The width is kept as its ratio of the terminal's width; back on the width it had at the
// press it takes the ratio it had then, so a drag with no net move changes nothing.
func (m model) dragSplitDivider(msg tea.MouseMotionMsg) model {
	columns := m.clampSidebarWidth(msg.X - m.drag.grabOffset - dividerMargin)
	if columns == m.sidebarWidth() {
		return m
	}
	m.sidebarRatio = m.ratioOf(columns)
	if columns == m.drag.startColumns {
		m.sidebarRatio = m.drag.startRatio
	}
	return m.relayout()
}

// dragStackedDivider moves the stacked layout's divider with the pointer, keeping the row it was
// grabbed by under the pointer within the panes' minimums, and repaints the panes when the
// sidebar's height changed. The height is kept as its ratio of the body's rows; back on the
// height it had at the press it takes the ratio it had then, so a drag with no net move changes
// nothing.
func (m model) dragStackedDivider(msg tea.MouseMotionMsg) model {
	rows := m.clampStackedRows(msg.Y - m.drag.grabOffset - bodyTop)
	if rows == m.stackedSidebarRows() {
		return m
	}
	m.stackedRatio = m.rowRatioOf(rows)
	if rows == m.drag.startRows {
		m.stackedRatio = m.drag.startRatio
	}
	return m.relayout()
}

// mouseRelease ends a drag of the divider and returns the command that saves both dividers'
// ratios to the state file when the drag moved its divider; a drag that moved it also forgets its
// press, so the next press on the divider starts a drag and never completes a double-click.
// Without a drag it ends the detail selection's drag and copies what it covers, as
// releaseSelection does.
func (m model) mouseRelease(msg tea.MouseReleaseMsg) (model, tea.Cmd) {
	if !m.drag.isActive {
		return m.releaseSelection(msg.X, msg.Y)
	}
	isChanged := m.dividerRatio(m.drag.isStacked) != m.drag.startRatio
	m.drag = dividerDrag{}
	if !isChanged {
		return m, nil
	}
	m.lastDividerPress = time.Time{}
	return m, m.saveState()
}

// clickSidebar selects the sidebar row under point and shows it, as the up and down keys do,
// closing an open dependency view first. A click below the last row, or on the scrollbar's gutter
// or bar, selects nothing.
func (m model) clickSidebar(point panePoint) (model, tea.Cmd) {
	if point.col >= m.sidebarPaneWidth() {
		return m, nil
	}
	switch m.tab {
	case TabReady:
		position := m.ready.top + point.row
		if m.ready.err != nil || position >= len(m.ready.entries) {
			return m, nil
		}
		m = m.closeDepsIfOpen()
		m.ready.cursor.seat(position, len(m.ready.entries))
		return m.showReadySelection()
	case TabIssues:
		if m.issues.err != nil || !m.issues.tree.click(point.row, point.col) {
			return m, nil
		}
		return m.closeDepsIfOpen().showIssuesSelection()
	case TabKnowledge:
		if m.knowledge.err != nil || !m.knowledge.tree.click(point.row, point.col) {
			return m, nil
		}
		return m.showKnowledgeSelection()
	}
	return m, nil
}

// clickDetail selects the dependency view's row under point while the view is open, and anchors
// the detail selection, collapsed, on the cell under point while the detail pane shows an issue or
// a Note; a link under it is followed at the release, by releaseSelection. A click on the
// scrollbar's gutter or bar does nothing.
func (m model) clickDetail(point panePoint) (model, tea.Cmd) {
	if point.col >= m.detailWidth() {
		return m, nil
	}
	line := m.detailOffset + point.row
	if m.deps.isOpen {
		if !m.deps.isLoaded {
			return m, nil
		}
		position := m.deps.rowAtLine(line)
		if position < 0 {
			return m, nil
		}
		m.deps.cursor.seat(position, len(m.deps.rows()))
		return m.paintDeps(), nil
	}
	if m.issue.id == "" {
		return m, nil
	}
	cell := contentCell{line: line, col: point.col}
	m.sel = detailSel{isActive: true, anchor: cell, head: cell}
	return m, nil
}

// extendSelection moves the detail selection's head to the screen cell at column x and row y
// while a selection is under way and the cell is in the detail pane, its column held to the pane's
// text so the scrollbar never extends it. A cell off the pane leaves the head where it was.
func (m model) extendSelection(x, y int) model {
	if !m.sel.isActive {
		return m
	}
	point := m.pointAt(x, y)
	if point.pane != paneDetail {
		return m
	}
	m.sel.head = contentCell{
		line: m.detailOffset + point.row,
		col:  min(max(point.col, 0), m.detailWidth()),
	}
	return m
}

// releaseSelection ends the detail selection's drag at the screen cell at column x and row y,
// extending it there as a motion does. A selection that spans a cell copies the text it covers, as
// copyText does, and is kept, so its highlight shows what was copied; one whose text is blank
// copies nothing and is cleared. A bare click, one that spans none, follows the link under it and
// clears the selection. Without a selection under way it does nothing.
func (m model) releaseSelection(x, y int) (model, tea.Cmd) {
	if !m.sel.isActive {
		return m, nil
	}
	m = m.extendSelection(x, y)
	if !m.sel.isCollapsed() {
		return m.copySelection()
	}
	anchor := m.sel.anchor
	m.sel = detailSel{}
	target, isOnLink := m.issue.linkAt(anchor.line, anchor.col)
	if !isOnLink {
		return m, nil
	}
	return m.followLink(target)
}

// copySelection copies the text the detail selection covers, as copyText does, keeping the
// selection; a selection whose text is blank copies nothing and is cleared.
func (m model) copySelection() (model, tea.Cmd) {
	text := m.sel.text(m.markdownTheme.Measure, m.detail, m.detailWidth())
	if strings.TrimSpace(text) == "" {
		m.sel = detailSel{}
		return m, nil
	}
	return m.copyText(text)
}

// followLink selects the issue or Note a detail link names: an issue in the Issues tab, a Note in
// the Knowledge tab, each revealed when its tree hides it. An id that names both follows the kind
// the detail pane shows: the Note on the Knowledge tab, the issue elsewhere.
func (m model) followLink(target string) (model, tea.Cmd) {
	isIssue := slices.ContainsFunc(m.issues.entries, func(entry index.Entry) bool { return entry.ID == target })
	isNote := slices.ContainsFunc(m.knowledge.notes, func(note index.NoteEntry) bool { return note.ID == target })
	if isNote && (!isIssue || m.tab == TabKnowledge) {
		return m.revealNote(target)
	}
	return m.revealIssue(target)
}

// revealNote switches to the Knowledge tab with the Note id names selected, its ancestors
// expanded and, when the tree hides it, revealed; a Note the Knowledge filter hides clears that
// filter first. A Note the tree does not hold is reported in the status bar as capcom ui
// knowledge reports one.
func (m model) revealNote(id string) (model, tea.Cmd) {
	m.deps = depView{}
	if m.filters[TabKnowledge].isActive() && !m.knowledge.tree.isShown[id] {
		m = m.clearFilter(TabKnowledge)
	}
	if !m.knowledge.tree.selectID(id) {
		m.notice = fmt.Sprintf(notInKnowledgeFormat, id)
	}
	return m.openTab(TabKnowledge)
}

// mouseWheel handles a wheel notch: over the sidebar it moves the selection a row, as the up and
// down keys do, closing an open dependency view first; over the detail pane it moves the
// dependency view's cursor a row while the view is open, and scrolls the pane detailWheelRows rows
// otherwise. Anywhere else, and while isMouseIgnored, it does nothing.
func (m model) mouseWheel(msg tea.MouseWheelMsg) (model, tea.Cmd) {
	if m.isMouseIgnored() {
		return m, nil
	}
	switch m.pointAt(msg.X, msg.Y).pane {
	case paneSidebar:
		return m.closeDepsIfOpen().wheelSidebar(msg)
	case paneDetail:
		return m.wheelDetail(msg), nil
	}
	return m, nil
}

// wheelSidebar moves the open tab's sidebar selection one row for a wheel notch and shows the
// row it lands on.
func (m model) wheelSidebar(msg tea.MouseWheelMsg) (model, tea.Cmd) {
	switch m.tab {
	case TabReady:
		m.ready.cursor.wheel(msg, len(m.ready.entries))
		return m.showReadySelection()
	case TabIssues:
		m.issues.tree.wheel(msg)
		return m.showIssuesSelection()
	case TabKnowledge:
		m.knowledge.tree.wheel(msg)
		return m.showKnowledgeSelection()
	}
	return m, nil
}

// wheelDetail moves the dependency view's cursor one row for a wheel notch while the view is
// open, and otherwise scrolls the detail pane detailWheelRows rows.
func (m model) wheelDetail(msg tea.MouseWheelMsg) model {
	if m.deps.isOpen {
		if !m.deps.isLoaded {
			return m
		}
		m.deps.cursor.wheel(msg, len(m.deps.rows()))
		return m.paintDeps()
	}
	switch msg.Button {
	case tea.MouseWheelUp:
		return m.scrollDetail(-detailWheelRows)
	case tea.MouseWheelDown:
		return m.scrollDetail(detailWheelRows)
	}
	return m
}

// closeDepsIfOpen closes the dependency view when it is open, so the detail pane never lists one
// issue's dependencies beside another's selection.
func (m model) closeDepsIfOpen() model {
	if !m.deps.isOpen {
		return m
	}
	return m.closeDeps()
}
