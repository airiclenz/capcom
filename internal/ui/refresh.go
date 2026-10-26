package ui

import (
	"fmt"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/lock"
)

// refreshInterval is how long the UI waits after one poll of the index before the next.
const refreshInterval = time.Second

// The status bar's texts of live refresh.
const (
	// foreignSchemaText replaces polling once another capcom version has rebuilt the index: the
	// UI never rebuilds it back.
	foreignSchemaText = "index changed by another capcom version: restart capcom ui"
	// refreshFailurePrefix opens the notice of a poll that could not refresh the index.
	refreshFailurePrefix = "refresh: "
	// reloadFailureFormat is the notice of a changed config.yaml that could not be used.
	reloadFailureFormat = "reload config: %v (kept the previous config)"
)

// tickMsg asks for one poll of the index.
type tickMsg struct{}

// scheduleTick returns the command that sends the next tickMsg once refreshInterval has passed.
func scheduleTick() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

// refreshedMsg carries one poll of the index: what it did about config.yaml and the error of a
// re-open that failed, whether another capcom version's schema stopped it, whether any issue or
// Note changed, the error of a refresh that failed, and the status bar's account of the Tree lock
// with the badge its leading word wears.
type refreshedMsg struct {
	reload    reloadOutcome
	reloadErr error
	isForeign bool
	isChanged bool
	err       error
	lockText  string
	lockBadge lockBadge
}

// tabRead is what one read of a tab's sidebar asks for besides its rows: the id to select, and,
// on a reload, the filter in effect, matched again in the same read so rows and filter agree.
// isStored marks an id the worktree state file stored, selected quietly: one that resolves to no
// issue or Note, or that the tab does not hold, leaves the first row selected without a notice.
type tabRead struct {
	wantedID   string
	isStored   bool
	isReload   bool
	filterText string
}

// pollIndex returns the command that polls the index once through the session.
func pollIndex(s *session) tea.Cmd {
	return func() tea.Msg {
		return s.poll()
	}
}

// poll re-opens the store and index on a changed config.yaml, stops at an index another capcom
// version rebuilt, otherwise refreshes the index, and reads this worktree's Tree lock, all while no
// other access runs.
func (s *session) poll() refreshedMsg {
	s.mu.Lock()
	defer s.mu.Unlock()
	var msg refreshedMsg
	msg.reload, msg.reloadErr = s.reloadConfig()
	msg.isChanged = msg.reload == reloadDone
	msg.isForeign, msg.err = s.index.IsSchemaForeign()
	if msg.err == nil && !msg.isForeign {
		var isChanged bool
		isChanged, msg.err = s.index.Refresh()
		msg.isChanged = msg.isChanged || isChanged
	}
	overview, err := lock.ForWorktree(s.store).Overview()
	msg.lockText = lockText(overview, err, time.Now())
	msg.lockBadge = lockBadgeOf(overview, err)
	return msg
}

// tickUpdate polls the index, unless polling has stopped.
func (m model) tickUpdate() (model, tea.Cmd) {
	if m.isPollingStopped {
		return m, nil
	}
	return m, pollIndex(m.session)
}

// applyRefreshed takes in one poll: the Tree lock, the config.yaml and refresh notices, and, when
// anything changed, a reload of every tab, the shown issue or Note and the dependency view. An
// index another capcom version rebuilt stops polling with a notice; any other poll schedules the
// next one.
func (m model) applyRefreshed(msg refreshedMsg) (model, tea.Cmd) {
	m.lockText = msg.lockText
	m.lockBadge = msg.lockBadge
	switch {
	case msg.reload == reloadFailed:
		m.storeNotice = fmt.Sprintf(reloadFailureFormat, msg.reloadErr)
	case msg.reload == reloadDone && msg.reloadErr != nil:
		m.storeNotice = refreshFailurePrefix + msg.reloadErr.Error()
	case msg.reload == reloadDone:
		m.storeNotice = ""
	}
	if msg.isForeign {
		m.isPollingStopped = true
		m.refreshNotice = foreignSchemaText
		return m, nil
	}
	m.refreshNotice = ""
	if msg.err != nil {
		m.refreshNotice = refreshFailurePrefix + msg.err.Error()
	}
	if !msg.isChanged {
		return m, m.tick()
	}
	return m, tea.Batch(m.reloadTabs(), m.tick())
}

// reloadTabs returns the commands that read every tab again, each with its filter in effect, and
// the dependency view's issue while it is open.
func (m model) reloadTabs() tea.Cmd {
	cmds := []tea.Cmd{
		loadReady(m.session, m.reloadRead(TabReady)),
		loadIssues(m.session, m.reloadRead(TabIssues)),
		loadKnowledge(m.session, m.reloadRead(TabKnowledge)),
	}
	if m.deps.isOpen {
		cmds = append(cmds, loadDeps(m.session, m.deps.id))
	}
	return tea.Batch(cmds...)
}

// reloadRead is the read of tab a reload asks for: no id to select, and the tab's filter in effect.
func (m model) reloadRead(tab Tab) tabRead {
	return tabRead{isReload: true, filterText: m.filters[tab].text}
}

// rematch matches a reload's filter text again over the index, as loadFilter does; nil when the
// read asks for no filter.
func (read tabRead) rematch(cfg config.Config, issueIndex *index.Index, tab Tab) *sidebarFilter {
	if read.filterText == "" {
		return nil
	}
	filter, err := matchFilter(cfg, issueIndex, tab, parseFilter(read.filterText))
	if err != nil {
		filter = sidebarFilter{err: err}
	}
	filter.text = read.filterText
	return &filter
}

// takeRematched puts a reload's re-matched filter in effect over tab, unless the tab's filter has
// been kept with other text or cleared since the reload was asked for.
func (m model) takeRematched(tab Tab, filter *sidebarFilter) model {
	if filter == nil || m.filters[tab].text != filter.text || m.filterAsked[tab] != filter.text {
		return m
	}
	m.filters[tab] = *filter
	return m
}

// withShownReloaded adds, to the command a reload's read of tab returned, the one that reads the
// shown issue or Note again, when the selection is still on it: a moved selection already has its
// own read in cmd.
func (m model) withShownReloaded(tab Tab, isReload bool, cmd tea.Cmd) (model, tea.Cmd) {
	if !isReload || cmd != nil || m.tab != tab || m.issue.id == "" {
		return m, cmd
	}
	switch tab {
	case TabReady:
		if entry, isSelected := m.readySelection(); isSelected && entry.ID == m.issue.id {
			return m, loadDetail(m.session, entry)
		}
	case TabIssues:
		if entry, isSelected := m.issuesSelection(); isSelected && entry.ID == m.issue.id {
			return m, loadDetail(m.session, entry)
		}
	case TabKnowledge:
		if note, isSelected := m.knowledgeSelection(); isSelected && note.ID == m.issue.id {
			return m, loadNoteDetail(m.session, note)
		}
	}
	return m, nil
}

// carryInto returns next, the widget of a reload of w's tree, with w's scroll position and its
// selection: on the same node when next shows it, or still holds it revealed, and otherwise on
// that node's replacement.
func (w treeWidget[E]) carryInto(next treeWidget[E]) treeWidget[E] {
	next.top = w.top
	selected, isSelected := w.selection()
	if !isSelected {
		return next
	}
	_, isInNext := next.parentOf[selected.ID]
	if next.isShown[selected.ID] || (isInNext && w.revealed == selected.ID) {
		next.selectID(selected.ID)
		return next
	}
	if id := w.replacementFor(selected.ID, func(id string) bool { return next.isShown[id] }); id != "" {
		next.selectID(id)
	}
	return next
}

// replacementFor returns the node a selection on id moves to when a reload dropped it: the first
// of its parent, its next siblings and its previous ones that isPresent holds for. A parent the
// reload dropped as well hands the rule on to its own parent and siblings. It returns "" when no
// node qualifies.
func (w treeWidget[E]) replacementFor(id string, isPresent func(string) bool) string {
	for id != "" {
		parentID := w.parentOf[id]
		children := w.children(parentID)
		siblings := make([]string, 0, len(children))
		for _, child := range children {
			siblings = append(siblings, w.idOf(child))
		}
		if replacement := replacementID(id, parentID, siblings, isPresent); replacement != "" {
			return replacement
		}
		id = parentID
	}
	return ""
}

// replacementID returns where a selection on the vanished id moves: parentID when isPresent holds
// for it, else the nearest sibling after id in siblings that it holds for, else the nearest one
// before it; "" when none is present.
func replacementID(id, parentID string, siblings []string, isPresent func(string) bool) string {
	if parentID != "" && isPresent(parentID) {
		return parentID
	}
	position := slices.Index(siblings, id)
	if position < 0 {
		return ""
	}
	for _, sibling := range siblings[position+1:] {
		if isPresent(sibling) {
			return sibling
		}
	}
	for row := position - 1; row >= 0; row-- {
		if isPresent(siblings[row]) {
			return siblings[row]
		}
	}
	return ""
}
