package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

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
	// selectFailureFormat is the notice for an id capcom ui ready, issues, knowledge or closed
	// names that resolves to no issue or Note.
	selectFailureFormat = "select %s: %v"
	// readyColumnGap separates the columns of an issue's sidebar row: id, status, priority, title.
	readyColumnGap = "  "
)

// readyOptions are the options capcom ready builds from no flags: every Ready issue, planned or
// not, hidden ones left out, no limit.
var readyOptions = briefing.ReadyOptions{Plan: index.PlanAny}

// readyMsg carries one read of the Ready queue: its issues, most urgent first, or the error that
// kept it from being read. selectID is the issue the read was asked to select, resolved to its id,
// and notice reports an id that resolved to no issue; both are empty when no id was asked for.
// isStored marks a selectID the worktree state file stored, which the queue may drop without a
// notice. isReload marks a live refresh's read, and filter is the Ready filter it matched again,
// nil when it matched none.
type readyMsg struct {
	entries  []index.Entry
	selectID string
	notice   string
	isStored bool
	err      error
	isReload bool
	filter   *sidebarFilter
}

// readyTab is the Ready tab's state: the queue from the last read, as an entry list over the
// issues the Ready filter keeps.
type readyTab struct {
	entryList
}

// loadReady returns a command that reads the Ready queue through the session, as capcom ready
// with no flags lists it, matches read's filter again when it names one, and resolves read's
// wanted id, aliases included, when it is not empty; a stored id that resolves to nothing is
// dropped without a notice.
func loadReady(s *session, read tabRead) tea.Cmd {
	wantedID := read.wantedID
	return func() tea.Msg {
		msg := readyMsg{isReload: read.isReload, isStored: read.isStored}
		msg.err = s.do(func(opened *store.Store, issueIndex *index.Index) error {
			listed, err := briefing.ReadyList(issueIndex, opened.Config, readyOptions)
			if err != nil {
				return err
			}
			msg.filter = read.rematch(opened.Config, issueIndex, TabReady)
			msg.entries = make([]index.Entry, 0, len(listed))
			for _, item := range listed {
				msg.entries = append(msg.entries, item.Entry)
			}
			if wantedID == "" {
				return nil
			}
			entry, err := issueIndex.Resolve(wantedID)
			if err != nil {
				if !read.isStored {
					msg.notice = fmt.Sprintf(selectFailureFormat, wantedID, err)
				}
				return nil
			}
			msg.selectID = entry.ID
			return nil
		})
		return msg
	}
}

// applyReady takes in a read of the Ready queue: it narrows the queue to the Ready filter, the
// one the read matched again when it did, keeps the cursor on the issue it was on or, when the
// read dropped that, on its replacement, seats it on the issue the read resolved, when the queue
// holds it, reports the id in the status bar when it does not and the worktree state file did not
// store it, and shows the selection; a reload's read also reads the shown issue again.
func (m model) applyReady(msg readyMsg) (model, tea.Cmd) {
	m = m.takeRematched(TabReady, msg.filter)
	m.ready.take(msg.entries, msg.err, m.filters[TabReady])
	if msg.notice != "" {
		m.notice = msg.notice
	}
	if msg.selectID != "" && !m.ready.seatID(msg.selectID) && !msg.isStored {
		m.notice = fmt.Sprintf(notInReadyFormat, msg.selectID)
	}
	next, cmd := m.showReadySelection()
	return next.withShownReloaded(TabReady, msg.isReload, cmd)
}

// readyKey moves the Ready tab's selection for a cursor key and shows the issue it lands on. It
// reports false for any other key, which it leaves to the model's own keys.
func (m model) readyKey(msg tea.KeyPressMsg) (model, tea.Cmd, bool) {
	if !m.ready.key(msg) {
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
		m.sel = detailSel{}
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
	return m.ready.selection()
}

// paintReadySidebar fills the sidebar with the Ready queue the Ready filter keeps, one `id  status
// P<n>  title` row per issue, its status in its category's colour and the selected one marked,
// scrolled so the selection stays on screen. The columns are measured over the whole queue, on
// screen or not, so scrolling never shifts them.
// An empty queue, one that could not be read and a filter that keeps nothing each take a single
// row saying so. It does nothing on another tab.
func (m model) paintReadySidebar() model {
	if m.tab != TabReady {
		return m
	}
	switch {
	case m.ready.err != nil:
		return m.withSidebar([]string{readyFailurePrefix + m.ready.err.Error()}, 0, 1)
	case len(m.ready.entries) == 0:
		return m.withSidebar([]string{m.emptySidebarText(TabReady, readyEmptyText)}, 0, 1)
	}
	var columns columnWidths
	for _, entry := range m.ready.entries {
		columns = columns.widen(0, entry)
	}
	start, end, selected := m.ready.window(max(m.sidebarHeight(), 1))
	rows := make([]string, 0, end-start)
	for row := start; row < end; row++ {
		entry := m.ready.entries[row]
		mark := m.theme.markOf(false, row == selected)
		rows = append(rows, mark.render(readyRow(entry, m.theme.status(entry.Category), columns)...))
	}
	return m.withSidebar(rows, start, len(m.ready.entries))
}

// columnWidths are the widths a sidebar row pads its id and its status to, so the status and the
// P<n> of every row start at one column; the zero columnWidths pads nothing.
type columnWidths struct {
	id     int
	status int
}

// widen returns c widened to hold the id and the status of entry, its id led by lead columns.
func (c columnWidths) widen(lead int, entry index.Entry) columnWidths {
	return columnWidths{
		id:     max(c.id, lead+ansi.StringWidth(entry.ID)),
		status: max(c.status, ansi.StringWidth(entry.Status)),
	}
}

// after returns c for a row whose id follows lead columns: its id column narrowed by lead.
func (c columnWidths) after(lead int) columnWidths {
	c.id -= lead
	return c
}

// padding returns the spaces that fill text out to width columns, none when it is as wide.
func padding(text string, width int) string {
	return strings.Repeat(" ", max(width-ansi.StringWidth(text), 0))
}

// readyRow is the spans of the sidebar row of one issue, `id  status  P<n>  title`, its id and its
// status padded to columns and its status painted in status. The padding stays outside the status
// style, so a selected row's fill runs unbroken beneath it.
func readyRow(entry index.Entry, status lipgloss.Style, columns columnWidths) []span {
	return []span{
		{text: entry.ID + padding(entry.ID, columns.id) + readyColumnGap},
		{text: entry.Status, style: status},
		{
			text: padding(entry.Status, columns.status) + readyColumnGap +
				fmt.Sprintf("P%d", entry.Priority) + readyColumnGap + oneLine(entry.Title),
		},
	}
}
