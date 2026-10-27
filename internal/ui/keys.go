package ui

import (
	"slices"
	"strconv"

	tea "charm.land/bubbletea/v2"
)

// The keys the model handles, as tea.KeyPressMsg.String spells them.
const (
	keyQuit      = "q"
	keyInterrupt = "ctrl+c"
	keyHelp      = "?"
	keyNextTab   = "tab"
	keyOpen      = "enter"
	keyBack      = "esc"
	keyPageUp    = "pgup"
	keyPageDown  = "pgdown"
	keyHalfUp    = "ctrl+u"
	keyHalfDown  = "ctrl+d"
	keyUp        = "up"
	keyDown      = "down"
	keyUpVim     = "k"
	keyDownVim   = "j"
	// keyShrinkSidebar and keyGrowSidebar move the divider between the panes left and right, or up
	// and down in the stacked layout.
	keyShrinkSidebar = "<"
	keyGrowSidebar   = ">"
	// keyLayout switches between the split and the stacked layout where the terminal fits both.
	keyLayout = "v"
)

// closedInertKeys are the write keys that do nothing on the Closed tab: a closed issue takes no
// claim, close, status change or Log line there, and R, which reopens it, is the tab's one write.
var closedInertKeys = []string{keyClaim, keyClose, keyStatus, keyLog}

// handleKey applies one key press. q and ctrl+c quit from anywhere. While the help screen is open
// only ? and Esc, which close it, do anything else. Otherwise the keys switch tabs, open and leave
// the one-pane layout's detail view, open the filter line, open the help screen, and scroll the
// detail pane: the page keys from anywhere, the arrows and j k only where the detail pane fills a
// narrow, short screen; in the stacked layout Enter keeps both panes, as in the split layout. Esc
// from the sidebar clears the open tab's filter. On the Ready and Issues
// tabs a claims the shown issue, s opens the status picker over it, c and R ask for a reason and
// close or reopen it, and L asks for a line and appends it to its Log; on the Closed tab R does
// the same and a, s, c and L do nothing. On every tab e opens the
// selected issue or Note file in the editor, and < and > move the divider on screen: in the split
// layout they narrow and widen the sidebar, in the stacked layout they shorten and lengthen it,
// and either saves the new position to the state file. On a terminal that fits both the split and
// the stacked layout v switches to the one not on screen and saves the layout choice.
// Everywhere else the up and down arrows and j k move the sidebar's selection, and on a tree the
// left and right arrows and h l collapse and expand it.
func (m model) handleKey(msg tea.KeyPressMsg) (model, tea.Cmd) {
	key := msg.String()
	if key == keyQuit || key == keyInterrupt {
		return m, tea.Quit
	}
	if m.isHelpOpen {
		if key == keyHelp || key == keyBack {
			m.isHelpOpen = false
		}
		return m, nil
	}
	if tab, isTabKey := tabForKey(key); isTabKey {
		return m.openTab(tab)
	}
	if m.tab == TabClosed && slices.Contains(closedInertKeys, key) {
		return m, nil
	}

	switch key {
	case keyHelp:
		m.isHelpOpen = true
	case keyNextTab:
		return m.openTab(nextTab(m.tab))
	case keyOpen:
		m.isDetailOpen = m.isSinglePane()
	case keyBack:
		isOnePaneDetail := m.isSinglePane() && m.isDetailOpen
		m.isDetailOpen = false
		if !isOnePaneDetail && m.filters[m.tab].isActive() {
			return m.clearFilter(m.tab).showSelection()
		}
	case keyFilter:
		return m.openFilter(), nil
	case keyClaim:
		return m.startWrite(claimAction())
	case keyStatus:
		return m.openStatusPicker()
	case keyClose:
		return m.openTextPrompt(closeWrite()), nil
	case keyReopen:
		return m.openTextPrompt(reopenWrite()), nil
	case keyLog:
		return m.openTextPrompt(logWrite()), nil
	case keyEdit:
		return m.startEdit()
	case keyShrinkSidebar:
		return m.resizeSidebarKey(-1)
	case keyGrowSidebar:
		return m.resizeSidebarKey(1)
	case keyLayout:
		return m.toggleLayout()
	default:
		return m.sidebarOrScrollKey(msg)
	}
	return m, nil
}

// sidebarOrScrollKey gives a key to the sidebar's selection while the sidebar is on screen, and
// to scrollForKey when the sidebar does not take it.
func (m model) sidebarOrScrollKey(msg tea.KeyPressMsg) (model, tea.Cmd) {
	isSidebarShown := !m.isSinglePane() || !m.isDetailOpen
	if isSidebarShown {
		if next, cmd, isTaken := m.sidebarKey(msg); isTaken {
			return next, cmd
		}
	}
	return m.scrollForKey(msg.String()), nil
}

// sidebarKey gives a key to the open tab's sidebar, and reports false when it does not take it.
func (m model) sidebarKey(msg tea.KeyPressMsg) (model, tea.Cmd, bool) {
	switch m.tab {
	case TabReady:
		return m.readyKey(msg)
	case TabIssues:
		return m.issuesKey(msg)
	case TabKnowledge:
		return m.knowledgeKey(msg)
	case TabClosed:
		return m.closedKey(msg)
	}
	return m, nil, false
}

// scrollForKey scrolls the detail pane for a scroll key: a page or half a page of the pane's own
// rows for the page keys, a row for the arrows and j k where the detail pane fills a narrow,
// short screen. Any other key leaves the model as it is.
func (m model) scrollForKey(key string) model {
	page := m.detailPaneRows()
	switch key {
	case keyPageUp:
		return m.scrollDetail(-page)
	case keyPageDown:
		return m.scrollDetail(page)
	case keyHalfUp:
		return m.scrollDetail(-halfPage(page))
	case keyHalfDown:
		return m.scrollDetail(halfPage(page))
	case keyUp, keyUpVim:
		return m.scrollNarrowDetail(-1)
	case keyDown, keyDownVim:
		return m.scrollNarrowDetail(1)
	}
	return m
}

// tabForKey returns the tab a digit key opens: 1 to 4 name the tabs in tabOrder.
func tabForKey(key string) (Tab, bool) {
	for position, tab := range tabOrder {
		if key == strconv.Itoa(position+1) {
			return tab, true
		}
	}
	return TabReady, false
}

// nextTab returns the tab after tab in tabOrder, wrapping from the last to the first.
func nextTab(tab Tab) Tab {
	for position, candidate := range tabOrder {
		if candidate == tab {
			return tabOrder[(position+1)%len(tabOrder)]
		}
	}
	return tabOrder[0]
}

// resizeSidebarKey moves the divider on screen one step in direction, -1 shrinking the sidebar and
// 1 growing it: sidebarStep columns in the split layout, as resizeSidebar does, or stackedStep
// rows in the stacked layout, as resizeStacked does. It returns the command that saves both
// dividers' ratios to the state file; a key that leaves the divider where it was saves nothing.
func (m model) resizeSidebarKey(direction int) (model, tea.Cmd) {
	isStacked := m.isStacked()
	before := m.dividerRatio(isStacked)
	if isStacked {
		m = m.resizeStacked(direction * stackedStep)
	} else {
		m = m.resizeSidebar(direction * sidebarStep)
	}
	if m.dividerRatio(isStacked) == before {
		return m, nil
	}
	return m, m.saveState()
}

// halfPage returns the rows ctrl+u and ctrl+d scroll: half of page, at least one.
func halfPage(page int) int {
	return max(page/2, 1)
}

// openTab switches to tab, back on its sidebar with the detail pane at its top and no detail
// selection, and returns the command that loads the tab's selection into the detail pane. Each tab
// keeps its own selection, and the Issues and Knowledge trees their expanded nodes, across the
// switch.
func (m model) openTab(tab Tab) (model, tea.Cmd) {
	m.tab = tab
	m.isDetailOpen = false
	m.detailOffset = 0
	m = m.withSidebar(nil, 0, 0)
	m.detail = nil
	m.issue = issueDetail{}
	m.sel = detailSel{}
	return m.showSelection()
}

// scrollNarrowDetail scrolls the detail pane by rows when it fills a narrow, short screen in the
// one-pane layout, and leaves the model as it is otherwise.
func (m model) scrollNarrowDetail(rows int) model {
	if !m.isSinglePane() || !m.isDetailOpen {
		return m
	}
	return m.scrollDetail(rows)
}

// scrollDetail moves the detail pane by rows, down when positive, keeping its last row no higher
// than the bottom of the pane and its first row no lower than the top.
func (m model) scrollDetail(rows int) model {
	lastOffset := max(len(m.detail)-m.detailPaneRows(), 0)
	m.detailOffset = min(max(m.detailOffset+rows, 0), lastOffset)
	return m
}
