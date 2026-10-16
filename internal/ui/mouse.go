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

// mouseClick handles a left click: on a tab's label it opens that tab, on a sidebar row it
// selects that row, expanding or collapsing a tree node when the click lands on its arrow, and in
// the detail pane it selects a row of the dependency view or follows a link. Every other click,
// and every click while isMouseIgnored, does nothing.
func (m model) mouseClick(msg tea.MouseClickMsg) (model, tea.Cmd) {
	if msg.Button != tea.MouseLeft || m.isMouseIgnored() {
		return m, nil
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

// clickSidebar selects the sidebar row under point and shows it, as the up and down keys do,
// closing an open dependency view first. A click below the last row selects nothing.
func (m model) clickSidebar(point panePoint) (model, tea.Cmd) {
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
// the link under point while the detail pane shows an issue or a Note.
func (m model) clickDetail(point panePoint) (model, tea.Cmd) {
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
