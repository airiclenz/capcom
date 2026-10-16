package ui

import (
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
)

// handleKey applies one key press. q and ctrl+c quit from anywhere. While the help screen is open
// only ? and Esc, which close it, do anything else. Otherwise the keys switch tabs, open and leave
// the narrow detail view, open the filter line, open the help screen, and scroll the detail pane:
// the page keys from anywhere, the arrows and j k only where the detail pane fills a narrow
// screen. Esc from the sidebar clears the open tab's filter. On the Ready and Issues tabs a claims
// the shown issue and s opens the status picker over it. Everywhere else
// the up and down arrows and j k move the sidebar's selection, and on a tree the left and right
// arrows and h l collapse and expand it.
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

	switch key {
	case keyHelp:
		m.isHelpOpen = true
	case keyNextTab:
		return m.openTab(nextTab(m.tab))
	case keyOpen:
		m.isDetailOpen = m.isNarrow()
	case keyBack:
		isNarrowDetail := m.isNarrow() && m.isDetailOpen
		m.isDetailOpen = false
		if !isNarrowDetail && m.filters[m.tab].isActive() {
			return m.clearFilter(m.tab).showSelection()
		}
	case keyFilter:
		return m.openFilter(), nil
	case keyClaim:
		return m.startWrite(claimAction())
	case keyStatus:
		return m.openStatusPicker()
	default:
		return m.sidebarOrScrollKey(msg)
	}
	return m, nil
}

// sidebarOrScrollKey gives a key to the sidebar's selection while the sidebar is on screen, and
// to scrollForKey when the sidebar does not take it.
func (m model) sidebarOrScrollKey(msg tea.KeyPressMsg) (model, tea.Cmd) {
	isSidebarShown := !m.isNarrow() || !m.isDetailOpen
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
	}
	return m, nil, false
}

// scrollForKey scrolls the detail pane for a scroll key: a page or half a page for the page keys,
// a row for the arrows and j k where the detail pane fills a narrow screen. Any other key leaves
// the model as it is.
func (m model) scrollForKey(key string) model {
	page := m.bodyHeight()
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

// tabForKey returns the tab a digit key opens: 1, 2 and 3 name the tabs in tabOrder.
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

// halfPage returns the rows ctrl+u and ctrl+d scroll: half of page, at least one.
func halfPage(page int) int {
	return max(page/2, 1)
}

// openTab switches to tab, back on its sidebar with the detail pane at its top, and returns the
// command that loads the tab's selection into the detail pane. Each tab keeps its own selection,
// and the Issues and Knowledge trees their expanded nodes, across the switch.
func (m model) openTab(tab Tab) (model, tea.Cmd) {
	m.tab = tab
	m.isDetailOpen = false
	m.detailOffset = 0
	m.sidebar = nil
	m.detail = nil
	m.issue = issueDetail{}
	return m.showSelection()
}

// scrollNarrowDetail scrolls the detail pane by rows when it fills a narrow screen, and leaves
// the model as it is otherwise.
func (m model) scrollNarrowDetail(rows int) model {
	if !m.isNarrow() || !m.isDetailOpen {
		return m
	}
	return m.scrollDetail(rows)
}

// scrollDetail moves the detail pane by rows, down when positive, keeping its last row no higher
// than the bottom of the pane and its first row no lower than the top.
func (m model) scrollDetail(rows int) model {
	lastOffset := max(len(m.detail)-m.bodyHeight(), 0)
	m.detailOffset = min(max(m.detailOffset+rows, 0), lastOffset)
	return m
}
