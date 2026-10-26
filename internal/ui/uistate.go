package ui

import (
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/userconfig"
)

// selectionSaveDelay is how long after the last change of a selection the worktree state file is
// saved, so a run of moves writes it once.
const selectionSaveDelay = time.Second

// worktreeSaveNoticePrefix opens the status-bar notice about a worktree state file that could not
// be saved.
const worktreeSaveNoticePrefix = "cannot save the tab and selection: "

// worktreeTracking is the model's account of the worktree state file's saves: the saver that
// writes it, which tabs' first reads have applied, the debounced selection save and whether a
// failed save was reported.
type worktreeTracking struct {
	// saver writes the worktree state file, shared by every copy of the model.
	saver *worktreeStateSaver
	// isInitialReadIn reports, indexed by Tab, that the tab's first read arrived, and isRead that
	// a read without error applied after it; until then the tab keeps its stored id.
	isInitialReadIn [len(tabNames)]bool
	isRead          [len(tabNames)]bool
	// sequence numbers the last debounce tick scheduled, so only its own tick saves, and
	// isSavePending reports a selection change not yet saved.
	sequence      int
	isSavePending bool
	// isFailureReported reports a failed save shown in the status bar; later failures are silent.
	isFailureReported bool
}

// selectionSaveMsg is the debounce tick of a selection change: sequence numbers the tick, and only
// the last one scheduled saves.
type selectionSaveMsg struct {
	sequence int
}

// worktreeStateSavedMsg reports a write of the worktree state file: err is the write's error, nil
// when it succeeded.
type worktreeStateSavedMsg struct {
	err error
}

// worktreeStateSaver writes the worktree state file. The model holds it by pointer, so every save
// command built from any copy of the model shares it. Bubble Tea runs each command in its own
// goroutine, so save commands can run out of order: each writes the latest state asked for, under
// the mutex, and one that finds that state written already writes nothing.
type worktreeStateSaver struct {
	path string

	mutex sync.Mutex
	// state is the latest state asked for, asked the number of saves asked for so far and written
	// the number of the last one written.
	state   userconfig.WorktreeState
	asked   int
	written int
}

// save makes state the latest state asked for and returns the command that writes it, reported as
// a worktreeStateSavedMsg.
func (s *worktreeStateSaver) save(state userconfig.WorktreeState) tea.Cmd {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.state = state
	s.asked++
	return s.write
}

// write writes the latest state asked for and reports it as a worktreeStateSavedMsg. It reports
// nothing when that state is written already.
func (s *worktreeStateSaver) write() tea.Msg {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.written == s.asked {
		return nil
	}
	s.written = s.asked
	return worktreeStateSavedMsg{err: userconfig.SaveWorktreeState(s.path, s.state)}
}

// worktreeSnapshot returns what the worktree state file is to hold: the open tab and the id
// selected on each tab, the stored id for a tab whose first read has not applied, "" for a tab
// that selects nothing.
func (m model) worktreeSnapshot() userconfig.WorktreeState {
	return userconfig.WorktreeState{
		Tab:       m.tab.String(),
		Ready:     m.trackedSelection(TabReady),
		Issues:    m.trackedSelection(TabIssues),
		Knowledge: m.trackedSelection(TabKnowledge),
		Closed:    m.trackedSelection(TabClosed),
	}
}

// trackedSelection returns the id tab selects for the worktree state file: its stored id until
// its first read applies, then its selection, "" when it selects none.
func (m model) trackedSelection(tab Tab) string {
	if !m.worktree.isRead[tab] {
		return m.storedIDs[tab]
	}
	var (
		id         string
		isSelected bool
	)
	switch tab {
	case TabReady:
		entry, selected := m.readySelection()
		id, isSelected = entry.ID, selected
	case TabIssues:
		entry, selected := m.issuesSelection()
		id, isSelected = entry.ID, selected
	case TabKnowledge:
		note, selected := m.knowledgeSelection()
		id, isSelected = note.ID, selected
	case TabClosed:
		entry, selected := m.closedSelection()
		id, isSelected = entry.ID, selected
	}
	if !isSelected {
		return ""
	}
	return id
}

// trackWorktreeState follows msg, which turned the model before into m and returned cmd, into the
// worktree state file: a tab's read marks it read, a changed tab saves at once, and a changed
// selection schedules a save selectionSaveDelay later, dropping the one scheduled before. It
// returns cmd batched with the save or the tick, if any.
func (m model) trackWorktreeState(msg tea.Msg, before userconfig.WorktreeState, cmd tea.Cmd) (model, tea.Cmd) {
	m = m.markTabRead(msg)
	after := m.worktreeSnapshot()
	switch {
	case after.Tab != before.Tab:
		m.worktree.sequence++
		m.worktree.isSavePending = false
		return m, tea.Batch(cmd, m.worktree.saver.save(after))
	case after != before:
		m.worktree.sequence++
		m.worktree.isSavePending = true
		sequence := m.worktree.sequence
		tick := m.saveTick(selectionSaveDelay, func(time.Time) tea.Msg {
			return selectionSaveMsg{sequence: sequence}
		})
		return m, tea.Batch(cmd, tick)
	}
	return m, cmd
}

// markTabRead marks the tab msg reads as read when msg is a tab's read: its first read, or one
// after it, that applied without error.
func (m model) markTabRead(msg tea.Msg) model {
	switch msg := msg.(type) {
	case readyMsg:
		return m.markRead(TabReady, msg.isReload, msg.err)
	case issuesMsg:
		return m.markRead(TabIssues, msg.isReload, msg.err)
	case knowledgeMsg:
		return m.markRead(TabKnowledge, msg.isReload, msg.err)
	case closedMsg:
		return m.markRead(TabClosed, msg.isReload, msg.err)
	}
	return m
}

// markRead records a read of tab: isReload marks a live refresh's read and err the read's error.
// The tab counts as read once its first read arrived and a read without error applied; a reload
// ahead of the first read leaves its stored id standing.
func (m model) markRead(tab Tab, isReload bool, err error) model {
	if !isReload {
		m.worktree.isInitialReadIn[tab] = true
	}
	if err == nil && m.worktree.isInitialReadIn[tab] {
		m.worktree.isRead[tab] = true
	}
	return m
}

// applySelectionSave saves the worktree state file on the last scheduled debounce tick while a
// selection change is pending; an older tick does nothing.
func (m model) applySelectionSave(msg selectionSaveMsg) (model, tea.Cmd) {
	if msg.sequence != m.worktree.sequence || !m.worktree.isSavePending {
		return m, nil
	}
	m.worktree.isSavePending = false
	return m, m.worktree.saver.save(m.worktreeSnapshot())
}

// applyWorktreeStateSaved shows the first failed write of the worktree state file this session in
// the status bar; every later failure is silent.
func (m model) applyWorktreeStateSaved(msg worktreeStateSavedMsg) model {
	if msg.err == nil || m.worktree.isFailureReported {
		return m
	}
	m.worktree.isFailureReported = true
	m.worktreeStateNotice = worktreeSaveNoticePrefix + msg.err.Error()
	return m
}

// flushWorktreeState writes the worktree state file at once, when a selection change is still
// waiting for its debounce tick or a save has not run yet, and returns the write's error. Run calls
// it with the final model once the program ended, so quitting loses no change.
func (m model) flushWorktreeState() error {
	if m.worktree.isSavePending {
		m.worktree.saver.save(m.worktreeSnapshot())
	}
	saved, _ := m.worktree.saver.write().(worktreeStateSavedMsg)
	return saved.err
}
