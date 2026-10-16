package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/outline"
	"github.com/airiclenz/capcom/internal/store"
)

// The Issues tab's texts.
const (
	// issuesEmptyText is the sidebar's one row when the outline holds no issue.
	issuesEmptyText = "No open issues."
	// issuesFailurePrefix opens the sidebar's one row when the issues could not be read.
	issuesFailurePrefix = "issues: "
	// notInTreeFormat is the notice for an id capcom ui issues names that the tree does not hold.
	notInTreeFormat = "%s is not in the issue tree"
	// progressFormat closes the row of an issue with children: how many of them are Closed.
	progressFormat = "  (%d/%d closed)"
)

// issuesMsg carries one read of the issues for the Issues tab: every usable issue, Closed ones
// included, in tree order, or the error that kept them from being read. selectID is the issue
// the read was asked to select, resolved to its id, and notice reports an id that resolved to no
// issue; both are empty when no id was asked for. isReload marks a live refresh's read, and
// filter is the Issues filter it matched again, nil when it matched none.
type issuesMsg struct {
	entries  []index.Entry
	selectID string
	notice   string
	err      error
	isReload bool
	filter   *sidebarFilter
}

// issuesTab is the Issues tab's state: the issues of the last read, the tree over them, the error
// of a read that failed, and, while a filter is in effect, the nodes expanded before it.
type issuesTab struct {
	entries            []index.Entry
	tree               treeWidget[index.Entry]
	err                error
	unfilteredExpanded map[string]bool
}

// loadIssues returns a command that reads every issue through the session, Closed ones included,
// as capcom list --tree does, matches read's filter again when it names one, and resolves read's
// wanted id, aliases included, when it is not empty.
func loadIssues(s *session, read tabRead) tea.Cmd {
	wantedID := read.wantedID
	return func() tea.Msg {
		msg := issuesMsg{isReload: read.isReload}
		msg.err = s.do(func(opened *store.Store, issueIndex *index.Index) error {
			entries, err := issueIndex.List(index.Filter{IsClosedIncluded: true})
			if err != nil {
				return err
			}
			msg.filter = read.rematch(opened.Config, issueIndex, TabIssues)
			msg.entries = entries
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

// applyIssues takes in a read of the issues: it builds the outline capcom list --tree prints, or
// what the Issues filter leaves of it, the one the read matched again when it did, keeping the
// nodes expanded before it, the scroll and the selection or, when the read dropped the selected
// issue, its replacement, selects the issue the read resolved, reporting the id in the status bar
// when the tree does not hold it, and shows the selection; a reload's read also reads the shown
// issue again.
func (m model) applyIssues(msg issuesMsg) (model, tea.Cmd) {
	m = m.takeRematched(TabIssues, msg.filter)
	m.issues.err = msg.err
	m.issues.entries = msg.entries
	m.issues.tree = m.issues.tree.carryInto(m.issuesTree(m.issues.tree.expanded))
	if msg.notice != "" {
		m.notice = msg.notice
	}
	if msg.selectID != "" && !m.issues.tree.selectID(msg.selectID) {
		m.notice = fmt.Sprintf(notInTreeFormat, msg.selectID)
	}
	next, cmd := m.showIssuesSelection()
	return next.withShownReloaded(TabIssues, msg.isReload, cmd)
}

// issuesKey moves, expands or collapses the Issues tree for a tree key and shows the issue the
// selection lands on. It reports false for any other key, which it leaves to the model's own
// keys.
func (m model) issuesKey(msg tea.KeyPressMsg) (model, tea.Cmd, bool) {
	if m.issues.tree.key(msg) != listSwallowed {
		return m, nil, false
	}
	next, cmd := m.showIssuesSelection()
	return next, cmd, true
}

// showIssuesSelection paints the Issues sidebar and, when the selected issue is not the one the
// detail pane shows, returns the command that loads it; an empty tree clears the detail pane. It
// does nothing on another tab.
func (m model) showIssuesSelection() (model, tea.Cmd) {
	if m.tab != TabIssues {
		return m, nil
	}
	m = m.paintIssuesSidebar()
	selected, isSelected := m.issuesSelection()
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

// issuesSelection returns the selected issue of the Issues tree, and false when it has no rows.
func (m model) issuesSelection() (index.Entry, bool) {
	row, isSelected := m.issues.tree.selection()
	return row.Entry, isSelected
}

// paintIssuesSidebar fills the sidebar with the Issues tree, one `id  status  P<n>  title` row
// per visible issue with its children's progress, scrolled so the selection stays on screen. An
// empty tree, issues that could not be read and a filter that leaves nothing each take a single
// row saying so. It does nothing on another tab.
func (m model) paintIssuesSidebar() model {
	if m.tab != TabIssues {
		return m
	}
	switch {
	case m.issues.err != nil:
		m.sidebar = []string{issuesFailurePrefix + m.issues.err.Error()}
		return m
	case len(m.issues.tree.rows) == 0:
		m.sidebar = []string{m.emptySidebarText(TabIssues, issuesEmptyText)}
		return m
	}
	m.sidebar = m.issues.tree.paint(m.sidebarHeight(), m.theme, m.issueTreeRow)
	return m
}

// issueTreeRow is the text of one issue in the tree, as capcom list --tree prints it: `id  status
// P<n>  title`, then `(<closed>/<total> closed)` over all its children, hidden ones included,
// when it has any. Its status is in its category's colour unless the row is marked.
func (m model) issueTreeRow(row outline.Row[index.Entry], isMarked bool) string {
	status := m.theme.status(row.Entry.Category)
	if isMarked {
		status = lipgloss.NewStyle()
	}
	text := readyRow(row.Entry, status)
	children := m.issues.tree.children(row.ID)
	if len(children) == 0 {
		return text
	}
	closed := 0
	for _, child := range children {
		if child.Category.IsClosed() {
			closed++
		}
	}
	return text + fmt.Sprintf(progressFormat, closed, len(children))
}
