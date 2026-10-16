package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// The keys the model handles.
const (
	keyQuit      = "q"
	keyInterrupt = "ctrl+c"
)

// The fixed rows of the frame: the tab bar on top and the status bar at the bottom.
const (
	tabSeparator = " │ "
	statusHints  = "q quit"
	frameRows    = 2
)

// activeTabStyle marks the open tab on the tab bar.
var activeTabStyle = lipgloss.NewStyle().Bold(true).Reverse(true)

// model is the Bubble Tea model of the UI. It holds the session every store and index access goes
// through, the open tab, the id to select there, and the terminal size from the last
// tea.WindowSizeMsg.
type model struct {
	session   *session
	tab       Tab
	initialID string
	width     int
	height    int
}

// newModel returns the model of a UI that opens on tab, selecting id when it is set.
func newModel(s *session, tab Tab, id string) model {
	return model{session: s, tab: tab, initialID: id}
}

// Init starts the program; the model needs no command until the first message.
func (m model) Init() tea.Cmd {
	return nil
}

// Update records a new terminal size and quits on q or ctrl+c.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyPressMsg:
		switch msg.String() {
		case keyQuit, keyInterrupt:
			return m, tea.Quit
		}
	}
	return m, nil
}

// View draws the frame on the alternate screen. Until the first tea.WindowSizeMsg the size is
// unknown and the frame is empty.
func (m model) View() tea.View {
	view := tea.NewView(m.frame())
	view.AltScreen = true
	return view
}

// frame lays out the tab bar, the empty body and the status bar over the terminal's full height.
func (m model) frame() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	rows := make([]string, 0, max(m.height, frameRows))
	rows = append(rows, m.tabBar())
	for range m.height - frameRows {
		rows = append(rows, "")
	}
	rows = append(rows, statusHints)
	return strings.Join(rows, "\n")
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
