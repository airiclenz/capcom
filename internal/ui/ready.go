package ui

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/airiclenz/capcom/internal/briefing"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/store"
)

// The Ready tab's texts.
const (
	// readyEmptyText is the sidebar's one row when no issue is ready.
	readyEmptyText = "No issues are ready."
	// readyFailurePrefix opens the sidebar's one row when the queue could not be read.
	readyFailurePrefix = "ready: "
	// notInReadyFormat is the notice for an id capcom ui ready names that the queue does not hold.
	notInReadyFormat = "%s is not in the Ready queue"
	// selectFailureFormat is the notice for an id capcom ui ready, issues or knowledge names that
	// resolves to no issue or Note.
	selectFailureFormat = "select %s: %v"
)

// readyOptions are the options capcom ready builds from no flags: every Ready issue, planned or
// not, hidden ones left out, no limit.
var readyOptions = briefing.ReadyOptions{Plan: index.PlanAny}

// selectedRowStyle marks the selected sidebar row.
var selectedRowStyle = lipgloss.NewStyle().Reverse(true)

// readyMsg carries one read of the Ready queue: its issues, most urgent first, or the error that
// kept it from being read. selectID is the issue the read was asked to select, resolved to its id,
// and notice reports an id that resolved to no issue; both are empty when no id was asked for.
type readyMsg struct {
	entries  []index.Entry
	selectID string
	notice   string
	err      error
}

// readyTab is the Ready tab's state: the queue from the last read, the cursor over it and the
// first queue row on screen.
type readyTab struct {
	entries []index.Entry
	err     error
	cursor  listCursor
	top     int
}

// loadReady returns a command that reads the Ready queue through the session, as capcom ready
// with no flags lists it, and resolves wantedID, aliases included, when it is not empty.
func loadReady(s *session, wantedID string) tea.Cmd {
	return func() tea.Msg {
		var msg readyMsg
		msg.err = s.do(func(opened *store.Store, issueIndex *index.Index) error {
			listed, err := briefing.ReadyList(issueIndex, opened.Config, readyOptions)
			if err != nil {
				return err
			}
			msg.entries = make([]index.Entry, 0, len(listed))
			for _, item := range listed {
				msg.entries = append(msg.entries, item.Entry)
			}
			if wantedID == "" {
				return nil
			}
			entry, err := issueIndex.Resolve(wantedID)
			if err != nil {
				msg.notice = fmt.Sprintf(selectFailureFormat, wantedID, err)
				return nil
			}
			msg.selectID = entry.ID
			return nil
		})
		return msg
	}
}

// applyReady takes in a read of the Ready queue: it seats the cursor on the issue the read
// resolved, when the queue holds it, reports the id in the status bar when it does not, and shows
// the selection.
func (m model) applyReady(msg readyMsg) (model, tea.Cmd) {
	m.ready.entries = msg.entries
	m.ready.err = msg.err
	count := len(msg.entries)
	m.ready.cursor.clampSelection(count)
	if msg.notice != "" {
		m.notice = msg.notice
	}
	if msg.selectID != "" {
		row := slices.IndexFunc(msg.entries, func(entry index.Entry) bool { return entry.ID == msg.selectID })
		if row < 0 {
			m.notice = fmt.Sprintf(notInReadyFormat, msg.selectID)
		} else {
			m.ready.cursor.seat(row, count)
		}
	}
	return m.showReadySelection()
}

// readyKey moves the Ready tab's selection for a cursor key and shows the issue it lands on. It
// reports false for any other key, which it leaves to the model's own keys.
func (m model) readyKey(msg tea.KeyPressMsg) (model, tea.Cmd, bool) {
	if m.ready.cursor.key(msg, len(m.ready.entries), listStopsAtEnds) != listSwallowed {
		return m, nil, false
	}
	next, cmd := m.showReadySelection()
	return next, cmd, true
}

// showReadySelection paints the Ready sidebar and, when the selected issue is not the one the
// detail pane shows, returns the command that loads it; an empty queue clears the detail pane.
// It does nothing on another tab.
func (m model) showReadySelection() (model, tea.Cmd) {
	if m.tab != TabReady {
		return m, nil
	}
	m = m.paintReadySidebar()
	selected, isSelected := m.readySelection()
	switch {
	case !isSelected:
		m.issue = issueDetail{}
		m.detail = nil
		m.detailOffset = 0
		return m, nil
	case selected.ID == m.issue.id:
		return m, nil
	}
	return m, loadDetail(m.session, selected)
}

// readySelection returns the selected issue of the Ready queue, and false when the queue is
// empty.
func (m model) readySelection() (index.Entry, bool) {
	row := m.ready.cursor.highlight(len(m.ready.entries))
	if row < 0 {
		return index.Entry{}, false
	}
	return m.ready.entries[row], true
}

// paintReadySidebar fills the sidebar with the Ready queue, one `id  status  P<n>  title` row per
// issue, the selected one marked, scrolled so the selection stays on screen. An empty queue and
// one that could not be read each take a single row saying so. It does nothing on another tab.
func (m model) paintReadySidebar() model {
	if m.tab != TabReady {
		return m
	}
	switch {
	case m.ready.err != nil:
		m.sidebar = []string{readyFailurePrefix + m.ready.err.Error()}
		return m
	case len(m.ready.entries) == 0:
		m.sidebar = []string{readyEmptyText}
		return m
	}
	height := max(m.bodyHeight(), 1)
	selected := m.ready.cursor.highlight(len(m.ready.entries))
	m.ready.top = clampInt(m.ready.top, selected-height+1, selected)
	end := min(m.ready.top+height, len(m.ready.entries))
	rows := make([]string, 0, end-m.ready.top)
	for row := m.ready.top; row < end; row++ {
		text := readyRow(m.ready.entries[row])
		if row == selected {
			text = selectedRowStyle.Render(text)
		}
		rows = append(rows, text)
	}
	m.sidebar = rows
	return m
}

// readyRow is the sidebar row of one queued issue, as capcom ready prints it.
func readyRow(entry index.Entry) string {
	return fmt.Sprintf("%s  %s  P%d  %s", entry.ID, entry.Status, entry.Priority, oneLine(entry.Title))
}
