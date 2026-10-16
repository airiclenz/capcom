package ui

import (
	"fmt"
	"slices"
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
	// selectFailureFormat is the notice for an id capcom ui ready, issues or knowledge names that
	// resolves to no issue or Note.
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
// isReload marks a live refresh's read, and filter is the Ready filter it matched again, nil when
// it matched none.
type readyMsg struct {
	entries  []index.Entry
	selectID string
	notice   string
	err      error
	isReload bool
	filter   *sidebarFilter
}

// readyTab is the Ready tab's state: the queue from the last read, its issues the Ready filter
// keeps, the cursor over those and the first of them on screen.
type readyTab struct {
	all     []index.Entry
	entries []index.Entry
	err     error
	cursor  listCursor
	top     int
}

// loadReady returns a command that reads the Ready queue through the session, as capcom ready
// with no flags lists it, matches read's filter again when it names one, and resolves read's
// wanted id, aliases included, when it is not empty.
func loadReady(s *session, read tabRead) tea.Cmd {
	wantedID := read.wantedID
	return func() tea.Msg {
		msg := readyMsg{isReload: read.isReload}
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
				msg.notice = fmt.Sprintf(selectFailureFormat, wantedID, err)
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
// holds it, reports the id in the status bar when it does not, and shows the selection; a
// reload's read also reads the shown issue again.
func (m model) applyReady(msg readyMsg) (model, tea.Cmd) {
	previous, wasSelected := m.readySelection()
	previousIDs := make([]string, 0, len(m.ready.entries))
	for _, entry := range m.ready.entries {
		previousIDs = append(previousIDs, entry.ID)
	}
	m = m.takeRematched(TabReady, msg.filter)
	m.ready.all = msg.entries
	m.ready.entries = keepMatched(msg.entries, entryID, m.filters[TabReady])
	m.ready.err = msg.err
	count := len(m.ready.entries)
	m.ready.cursor.clampSelection(count)
	if wasSelected {
		m = m.keepReadySelection(previous, previousIDs)
	}
	if msg.notice != "" {
		m.notice = msg.notice
	}
	if msg.selectID != "" {
		row := slices.IndexFunc(m.ready.entries, func(entry index.Entry) bool { return entry.ID == msg.selectID })
		if row < 0 {
			m.notice = fmt.Sprintf(notInReadyFormat, msg.selectID)
		} else {
			m.ready.cursor.seat(row, count)
		}
	}
	next, cmd := m.showReadySelection()
	return next.withShownReloaded(TabReady, msg.isReload, cmd)
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
	height := max(m.sidebarHeight(), 1)
	selected := m.ready.cursor.highlight(len(m.ready.entries))
	m.ready.top = clampInt(m.ready.top, selected-height+1, selected)
	end := min(m.ready.top+height, len(m.ready.entries))
	rows := make([]string, 0, end-m.ready.top)
	for row := m.ready.top; row < end; row++ {
		entry := m.ready.entries[row]
		mark := m.theme.markOf(false, row == selected)
		rows = append(rows, mark.render(readyRow(entry, m.theme.status(entry.Category), columns)...))
	}
	return m.withSidebar(rows, m.ready.top, len(m.ready.entries))
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
