package ui

import (
	"cmp"
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/store"
)

// The Closed tab's texts and the layout of its rows' date.
const (
	// closedEmptyText is the sidebar's one row when no issue is closed.
	closedEmptyText = "No issue is closed."
	// closedFailurePrefix opens the sidebar's one row when the closed issues could not be read.
	closedFailurePrefix = "closed: "
	// notOnClosedFormat is the notice for an id capcom ui closed names that the tab does not hold.
	notOnClosedFormat = "%s is not on the Closed tab"
	// closedDateLayout is how a row shows its issue's Closed time, in UTC.
	closedDateLayout = "2006-01-02"
	// closedColumnGap separates the columns of a closed issue's sidebar row: date, id, title.
	closedColumnGap = "  "
)

// closedMsg carries one read of the closed issues: those in a closed category, newest Closed time
// first, or the error that kept them from being read. selectID is the issue the read was asked to
// select, resolved to its id, and notice reports an id that resolved to no issue; both are empty
// when no id was asked for. isStored marks a selectID the worktree state file stored, which the
// tab may not hold, without a notice. isReload marks a live refresh's read, and filter is the
// Closed filter it matched again, nil when it matched none.
type closedMsg struct {
	entries  []index.Entry
	selectID string
	notice   string
	isStored bool
	err      error
	isReload bool
	filter   *sidebarFilter
}

// closedTab is the Closed tab's state: the closed issues from the last read, as an entry list over
// the issues the Closed filter keeps.
type closedTab struct {
	entryList
}

// loadClosed returns a command that reads the issues in a closed category through the session,
// newest Closed time first and, on a tie, by id, matches read's filter again when it names one,
// and resolves read's wanted id, aliases included, when it is not empty; a stored id that resolves
// to nothing is dropped without a notice.
func loadClosed(s *session, read tabRead) tea.Cmd {
	wantedID := read.wantedID
	return func() tea.Msg {
		msg := closedMsg{isReload: read.isReload, isStored: read.isStored}
		msg.err = s.do(func(opened *store.Store, issueIndex *index.Index) error {
			entries, err := issueIndex.List(index.Filter{
				Categories:       config.ClosedCategories(),
				IsClosedIncluded: true,
			})
			if err != nil {
				return err
			}
			slices.SortFunc(entries, newestClosedFirst)
			msg.filter = read.rematch(opened.Config, issueIndex, TabClosed)
			msg.entries = entries
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

// newestClosedFirst orders two closed issues by their Closed time, the later first, and two
// closed at the same time by id.
func newestClosedFirst(a, b index.Entry) int {
	if order := b.Closed.Compare(a.Closed); order != 0 {
		return order
	}
	return cmp.Compare(a.ID, b.ID)
}

// applyClosed takes in a read of the closed issues: it narrows them to the Closed filter, the one
// the read matched again when it did, keeps the cursor on the issue it was on or, when the read
// dropped that, on its replacement, seats it on the issue the read resolved, when the tab holds
// it, reports the id in the status bar when it does not and the worktree state file did not store
// it, and shows the selection; a reload's read also reads the shown issue again.
func (m model) applyClosed(msg closedMsg) (model, tea.Cmd) {
	m = m.takeRematched(TabClosed, msg.filter)
	m.closed.take(msg.entries, msg.err, m.filters[TabClosed])
	if msg.notice != "" {
		m.notice = msg.notice
	}
	if msg.selectID != "" && !m.closed.seatID(msg.selectID) && !msg.isStored {
		m.notice = fmt.Sprintf(notOnClosedFormat, msg.selectID)
	}
	next, cmd := m.showClosedSelection()
	return next.withShownReloaded(TabClosed, msg.isReload, cmd)
}

// closedKey moves the Closed tab's selection for a cursor key and shows the issue it lands on. It
// reports false for any other key, which it leaves to the model's own keys.
func (m model) closedKey(msg tea.KeyPressMsg) (model, tea.Cmd, bool) {
	if !m.closed.key(msg) {
		return m, nil, false
	}
	next, cmd := m.showClosedSelection()
	return next, cmd, true
}

// showClosedSelection paints the Closed sidebar and, when the selected issue is not the one the
// detail pane shows, returns the command that loads it; an empty list clears the detail pane. It
// does nothing on another tab.
func (m model) showClosedSelection() (model, tea.Cmd) {
	if m.tab != TabClosed {
		return m, nil
	}
	m = m.paintClosedSidebar()
	selected, isSelected := m.closedSelection()
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

// closedSelection returns the selected issue of the Closed tab, and false when it lists none.
func (m model) closedSelection() (index.Entry, bool) {
	return m.closed.selection()
}

// paintClosedSidebar fills the sidebar with the closed issues the Closed filter keeps, one
// `<closed date>  id  title` row per issue, the selected one marked, scrolled so the selection
// stays on screen. The id column is measured over the whole list, on screen or not, so scrolling
// never shifts the titles. No closed issue, a list that could not be read and a filter that keeps
// nothing each take a single row saying so. It does nothing on another tab.
func (m model) paintClosedSidebar() model {
	if m.tab != TabClosed {
		return m
	}
	switch {
	case m.closed.err != nil:
		return m.withSidebar([]string{closedFailurePrefix + m.closed.err.Error()}, 0, 1)
	case len(m.closed.entries) == 0:
		return m.withSidebar([]string{m.emptySidebarText(TabClosed, closedEmptyText)}, 0, 1)
	}
	idWidth := 0
	for _, entry := range m.closed.entries {
		idWidth = max(idWidth, ansi.StringWidth(entry.ID))
	}
	start, end, selected := m.closed.window(max(m.sidebarHeight(), 1))
	rows := make([]string, 0, end-start)
	for row := start; row < end; row++ {
		mark := m.theme.markOf(false, row == selected)
		rows = append(rows, mark.render(closedRow(m.closed.entries[row], idWidth)))
	}
	return m.withSidebar(rows, start, len(m.closed.entries))
}

// closedRow is the span of the sidebar row of one closed issue, `<closed date>  id  title`, its
// date the Closed time's day in UTC and its id padded to idWidth.
func closedRow(entry index.Entry, idWidth int) span {
	date := entry.Closed.UTC().Format(closedDateLayout)
	return span{
		text: date + closedColumnGap + entry.ID + padding(entry.ID, idWidth) + closedColumnGap + oneLine(entry.Title),
	}
}
