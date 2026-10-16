package ui

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/index"
)

// detailWheelRows is how many rows one wheel notch scrolls the detail pane.
const detailWheelRows = 3

// isMouseIgnored reports an open help screen, filter line, status picker or prompt: each takes
// every key while it is open, so a click or a wheel notch cannot move what it acts on.
func (m model) isMouseIgnored() bool {
	return m.isHelpOpen || m.isFilterOpen || m.picker.isOpen || m.prompt.isOpen || m.textPrompt.isOpen
}

// dividerDrag is a drag of the divider between the panes, from a left press on its hit area to the
// release of the button. grabOffset is the column pressed counted from the separator, so the
// divider keeps that distance from the pointer and never jumps to it; startColumns is
// sidebarColumns at the press, which tells the release whether the width changed.
type dividerDrag struct {
	isActive     bool
	grabOffset   int
	startColumns int
}

// mouseClick handles a left click: on the divider's hit area it starts a drag of the divider, on a
// tab's label it opens that tab, on a sidebar row it selects that row, expanding or collapsing a
// tree node when the click lands on its arrow, and in the detail pane it selects a row of the
// dependency view or follows a link. Every other click, and every click while isMouseIgnored, does
// nothing.
func (m model) mouseClick(msg tea.MouseClickMsg) (model, tea.Cmd) {
	if msg.Button != tea.MouseLeft || m.isMouseIgnored() {
		return m, nil
	}
	if m.isOnDivider(msg.X, msg.Y) {
		return m.startDrag(msg.X), nil
	}
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

// startDrag starts a drag of the divider pressed at column x.
func (m model) startDrag(x int) model {
	m.drag = dividerDrag{isActive: true, grabOffset: x - m.dividerColumn(), startColumns: m.sidebarColumns}
	return m
}

// mouseMotion moves the divider with the pointer while it is dragged in the split layout, keeping
// the column it was grabbed by under the pointer within the sidebar's limits, and repaints the
// panes when the width changed. Without a drag it does nothing.
func (m model) mouseMotion(msg tea.MouseMotionMsg) model {
	if !m.drag.isActive || !m.isSplit() {
		return m
	}
	columns := m.clampSidebarWidth(msg.X - m.drag.grabOffset - dividerMargin)
	if columns == m.sidebarWidth() {
		return m
	}
	m.sidebarColumns = columns
	return m.relayout()
}

// mouseRelease ends a drag of the divider and returns the command that saves the width to the
// state file when the drag changed it. Without a drag it does nothing.
func (m model) mouseRelease() (model, tea.Cmd) {
	if !m.drag.isActive {
		return m, nil
	}
	isChanged := m.sidebarColumns != m.drag.startColumns
	m.drag = dividerDrag{}
	if !isChanged {
		return m, nil
	}
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

// clickDetail selects the dependency view's row under point while the view is open, and follows
// the link under point while the detail pane shows an issue or a Note. A click on the scrollbar's
// gutter or bar does nothing.
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
	target, isOnLink := m.issue.linkAt(line, point.col)
	if !isOnLink {
		return m, nil
	}
	return m.followLink(target)
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
