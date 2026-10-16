package ui

import (
	"cmp"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/outline"
	"github.com/airiclenz/capcom/internal/store"
)

// The Knowledge tab's texts.
const (
	// knowledgeEmptyText is the sidebar's one row when the outline holds no Note.
	knowledgeEmptyText = "No Notes."
	// knowledgeFailurePrefix opens the sidebar's one row when the Notes could not be read.
	knowledgeFailurePrefix = "knowledge: "
	// notInKnowledgeFormat is the notice for an id capcom ui knowledge names that the tree does
	// not hold.
	notInKnowledgeFormat = "%s is not in the Knowledge tree"
	// noteRowSeparator sits between a Note's id and its title in a tree row, as know tree prints it.
	noteRowSeparator = "  "
)

// knowledgeMsg carries one read of the Notes for the Knowledge tab: every usable Note, retired
// ones included, in tree order, or the error that kept them from being read. selectID is the Note
// the read was asked to select, resolved to its id, and notice reports an id that resolved to no
// Note; both are empty when no id was asked for.
type knowledgeMsg struct {
	notes    []index.NoteEntry
	selectID string
	notice   string
	err      error
}

// knowledgeTab is the Knowledge tab's state: the tree of the last read and the error of a read
// that failed.
type knowledgeTab struct {
	tree treeWidget[index.NoteEntry]
	err  error
}

// loadKnowledge returns a command that reads every Note through the session, retired ones
// included so a selected one can be revealed, and resolves wantedID, aliases included, when it
// is not empty.
func loadKnowledge(s *session, wantedID string) tea.Cmd {
	return func() tea.Msg {
		var msg knowledgeMsg
		msg.err = s.do(func(_ *store.Store, issueIndex *index.Index) error {
			notes, err := issueIndex.Notes(true)
			if err != nil {
				return err
			}
			msg.notes = notes
			if wantedID == "" {
				return nil
			}
			entry, err := issueIndex.ResolveNote(wantedID)
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

// noteOutline returns the outline capcom know tree prints with no flags, over notes that include
// the retired ones: outline.Notes' grouping and id order, with retired subtrees hidden as well as
// background ones. Its shown rows are outline.Notes over the Notes without the retired ones; the
// hidden ones stay in the tree so the widget can reveal a selected one.
func noteOutline(notes []index.NoteEntry) *outline.Tree[index.NoteEntry] {
	return outline.NewTree(
		notes,
		func(entry index.NoteEntry) string { return entry.ID },
		func(entry index.NoteEntry) string { return entry.ParentID },
		func(left, right index.NoteEntry) int { return cmp.Compare(left.ID, right.ID) },
		func(entry index.NoteEntry) bool {
			return !entry.IsRetired && !knowledge.IsBackground(entry.Labels)
		},
	)
}

// applyKnowledge takes in a read of the Notes: it builds the outline capcom know tree prints,
// keeping the nodes expanded before it, selects the Note the read resolved, reporting the id in
// the status bar when the tree does not hold it, and shows the selection.
func (m model) applyKnowledge(msg knowledgeMsg) (model, tea.Cmd) {
	m.knowledge.err = msg.err
	m.knowledge.tree = newTreeWidget(
		noteOutline(msg.notes),
		func(entry index.NoteEntry) string { return entry.ID },
		m.knowledge.tree.expanded,
	)
	if msg.notice != "" {
		m.notice = msg.notice
	}
	if msg.selectID != "" && !m.knowledge.tree.selectID(msg.selectID) {
		m.notice = fmt.Sprintf(notInKnowledgeFormat, msg.selectID)
	}
	return m.showKnowledgeSelection()
}

// knowledgeKey moves, expands or collapses the Knowledge tree for a tree key and shows the Note
// the selection lands on. It reports false for any other key, which it leaves to the model's own
// keys.
func (m model) knowledgeKey(msg tea.KeyPressMsg) (model, tea.Cmd, bool) {
	if m.knowledge.tree.key(msg) != listSwallowed {
		return m, nil, false
	}
	next, cmd := m.showKnowledgeSelection()
	return next, cmd, true
}

// showKnowledgeSelection paints the Knowledge sidebar and, when the selected Note is not the one
// the detail pane shows, returns the command that loads it; an empty tree clears the detail pane.
// It does nothing on another tab.
func (m model) showKnowledgeSelection() (model, tea.Cmd) {
	if m.tab != TabKnowledge {
		return m, nil
	}
	m = m.paintKnowledgeSidebar()
	selected, isSelected := m.knowledgeSelection()
	switch {
	case !isSelected:
		m.issue = issueDetail{}
		m.detail = nil
		m.detailOffset = 0
		return m, nil
	case selected.ID == m.issue.id:
		return m, nil
	}
	return m, loadNoteDetail(m.session, selected)
}

// knowledgeSelection returns the selected Note of the Knowledge tree, and false when it has no
// rows.
func (m model) knowledgeSelection() (index.NoteEntry, bool) {
	row, isSelected := m.knowledge.tree.selection()
	return row.Entry, isSelected
}

// paintKnowledgeSidebar fills the sidebar with the Knowledge tree, one `id  title` row per
// visible Note, scrolled so the selection stays on screen. An empty tree and Notes that could not
// be read each take a single row saying so. It does nothing on another tab.
func (m model) paintKnowledgeSidebar() model {
	if m.tab != TabKnowledge {
		return m
	}
	switch {
	case m.knowledge.err != nil:
		m.sidebar = []string{knowledgeFailurePrefix + m.knowledge.err.Error()}
		return m
	case len(m.knowledge.tree.rows) == 0:
		m.sidebar = []string{knowledgeEmptyText}
		return m
	}
	m.sidebar = m.knowledge.tree.paint(m.bodyHeight(), noteTreeRow)
	return m
}

// noteTreeRow is the text of one Note in the tree, as capcom know tree prints it: `id  title`.
func noteTreeRow(row outline.Row[index.NoteEntry]) string {
	return row.ID + noteRowSeparator + oneLine(row.Entry.Title)
}
