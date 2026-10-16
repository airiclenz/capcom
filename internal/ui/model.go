package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/ui/markdown"
	"github.com/airiclenz/capcom/internal/userconfig"
)

// userConfigNoticePrefix opens a status-bar notice about the user config file.
const userConfigNoticePrefix = "user config: "

// defaultsNotice closes a user config notice whose error left the UI on the default config.
const defaultsNotice = " (running on defaults)"

// model is the Bubble Tea model of the UI. It holds the session every store and index access goes
// through, the open tab, the id to select there, the user config it runs on, the terminal size
// from the last tea.WindowSizeMsg, live refresh's state, and the state of the panes and the
// status bar.
type model struct {
	session   *session
	tab       Tab
	initialID string

	// tick returns the command that asks for the next poll of the index: scheduleTick, which
	// tests replace so no real clock runs.
	tick func() tea.Cmd
	// isPollingStopped reports polling ended for good, once another capcom version rebuilt the
	// index.
	isPollingStopped bool
	// storeNotice reports a changed config.yaml that could not be used, and refreshNotice a poll
	// that failed or stopped polling; each is empty when there is nothing to report.
	storeNotice   string
	refreshNotice string

	// userConfig is the user config the UI runs on: Default when the file could not be used.
	userConfig userconfig.Config
	// configNotice reports the user config's warnings or error in the status bar; empty when the
	// file loaded cleanly.
	configNotice string
	// lockText is the status bar's account of this worktree's Tree lock, from the last read.
	lockText string
	// notice is the status bar's report of an id to select that could not be selected; empty when
	// there is none.
	notice string
	// markdownTheme is what issue and Note bodies render with. Its Measure follows the painter:
	// Update folds every tea.ModeReportMsg into it.
	markdownTheme markdown.Theme

	width  int
	height int

	// isHelpOpen reports the help screen covering the panes.
	isHelpOpen bool
	// isDetailOpen reports, in the narrow layout, the detail pane filling the screen in place of
	// the sidebar.
	isDetailOpen bool
	// sidebar and detail are the rows of the two panes, top to bottom; detailOffset is the first
	// detail row on screen.
	sidebar      []string
	detail       []string
	detailOffset int

	// ready is the Ready tab's queue and selection, issues the Issues tab's tree, knowledge the
	// Knowledge tab's tree, issue the issue or Note the detail pane shows, and deps the dependency
	// view that covers it while d has it open.
	ready     readyTab
	issues    issuesTab
	knowledge knowledgeTab
	issue     issueDetail
	deps      depView

	// filters are the filters in effect over each tab's sidebar, and filterAsked the filter line
	// text each tab was last kept with, "" once cleared, both indexed by Tab: a matched filter
	// applies only while it is still the one asked for.
	filters     [len(tabNames)]sidebarFilter
	filterAsked [len(tabNames)]string
	// isFilterOpen reports the filter line open over the sidebar, taking every key, and
	// filterLine is that line.
	isFilterOpen bool
	filterLine   lineInput

	// prompt is the confirmation a write asks in the status bar, taking every key while it is
	// open; picker is the status picker s opens over the detail pane, taking every key while it is
	// open; writeNotice is the status bar's report of the last write, empty when there is none.
	prompt      confirmPrompt
	picker      statusPicker
	writeNotice string
}

// newModel returns the model of a UI over options' store and index, opening on options.Tab and
// selecting options.ID there when it is set. It loads the user config at options.UserConfigPath,
// or at userconfig.Path when that is empty, creating the file when it is missing; a config that
// cannot be used leaves the model on userconfig.Default with a notice in the status bar.
func newModel(options Options) model {
	config, notice := loadUserConfig(options.UserConfigPath)
	return model{
		session:      newSession(options.Store, options.Index),
		tab:          options.Tab,
		initialID:    options.ID,
		userConfig:   config,
		configNotice: notice,
		lockText:     lockReadingText,
		tick:         scheduleTick,
	}
}

// loadUserConfig loads the user config file at path, resolving the default path when path is
// empty. It returns the config to run on and the status-bar notice of its warnings or error, ""
// when there are none. A path that cannot be resolved and a file that cannot be used both leave
// the config at userconfig.Default.
func loadUserConfig(path string) (userconfig.Config, string) {
	if path == "" {
		resolved, err := userconfig.Path()
		if err != nil {
			return userconfig.Default(), userConfigNoticePrefix + err.Error() + defaultsNotice
		}
		path = resolved
	}
	config, warnings, err := userconfig.Load(path)
	if err != nil {
		return config, userConfigNoticePrefix + err.Error() + defaultsNotice
	}
	if len(warnings) == 0 {
		return config, ""
	}
	texts := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		texts = append(texts, warning.String())
	}
	return config, userConfigNoticePrefix + strings.Join(texts, "; ")
}

// Init reads this worktree's Tree lock for the status bar, the Ready queue, the issues of the
// Issues tree and the Notes of the Knowledge tree, and schedules the first poll of the index; the
// tab the UI opened on selects the id it was opened with.
func (m model) Init() tea.Cmd {
	return tea.Batch(
		readLock(m.session),
		loadReady(m.session, tabRead{wantedID: m.initialIDFor(TabReady)}),
		loadIssues(m.session, tabRead{wantedID: m.initialIDFor(TabIssues)}),
		loadKnowledge(m.session, tabRead{wantedID: m.initialIDFor(TabKnowledge)}),
		m.tick(),
	)
}

// initialIDFor returns the id the UI was opened with when it opened on tab, and "" otherwise.
func (m model) initialIDFor(tab Tab) string {
	if m.tab != tab {
		return ""
	}
	return m.initialID
}

// Update records a new terminal size, the Tree lock's state, the Ready queue, the issues, the
// Notes, the shown issue or Note, the dependency view's read, a matched filter, a poll of the
// index, a write's run, the status picker's statuses and the painter's width measure, polls the
// index on a tick, and handles the keys: an open confirmation prompt takes every key, then an
// open status picker, then the open filter line, ahead of the dependency view's, then the model's
// own. While the dependency view is open it keeps the detail
// pane over whatever else repaints it. Any other message reaches the open filter line, such as a
// paste.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.filterLine.setWidth(m.sidebarPaneWidth())
		m = m.paintSidebar().renderDetail().scrollDetail(0).paintDeps()
	case lockMsg:
		m.lockText = msg.text
	case readyMsg:
		return m.applyReady(msg)
	case issuesMsg:
		return m.applyIssues(msg)
	case knowledgeMsg:
		return m.applyKnowledge(msg)
	case detailMsg:
		m = m.applyDetail(msg).paintDeps()
	case depsMsg:
		m = m.applyDeps(msg)
	case filterMsg:
		return m.applyFilter(msg)
	case tickMsg:
		return m.tickUpdate()
	case refreshedMsg:
		return m.applyRefreshed(msg)
	case writtenMsg:
		return m.applyWritten(msg)
	case statusesMsg:
		m = m.applyStatuses(msg)
	case tea.ModeReportMsg:
		m.markdownTheme.Measure = m.markdownTheme.Measure.Observe(msg)
		// A new measure can wrap the shown issue differently, so its cached rendering goes.
		m.issue.renderedWidth = 0
		m = m.renderDetail().paintDeps()
	case tea.KeyPressMsg:
		if m.prompt.isOpen {
			return m.promptKey(msg)
		}
		if m.picker.isOpen {
			return m.pickerKey(msg)
		}
		if m.isFilterOpen {
			return m.filterLineKey(msg)
		}
		next, cmd, isTaken := m.depViewKey(msg)
		if isTaken {
			return next, cmd
		}
		return next.handleKey(msg)
	default:
		if m.isFilterOpen {
			var cmd tea.Cmd
			m.filterLine, cmd = m.filterLine.update(msg)
			return m, cmd
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

// showSelection paints the open tab's sidebar and returns the command that loads its selection
// into the detail pane, when it is not the issue or Note shown already.
func (m model) showSelection() (model, tea.Cmd) {
	switch m.tab {
	case TabReady:
		return m.showReadySelection()
	case TabIssues:
		return m.showIssuesSelection()
	case TabKnowledge:
		return m.showKnowledgeSelection()
	}
	return m, nil
}

// paintSidebar fills the sidebar with the open tab's rows.
func (m model) paintSidebar() model {
	return m.paintReadySidebar().paintIssuesSidebar().paintKnowledgeSidebar()
}

// sidebarSelection returns the id of the issue or Note the open tab's sidebar selects, and false
// when it selects none.
func (m model) sidebarSelection() (string, bool) {
	switch m.tab {
	case TabReady:
		entry, isSelected := m.readySelection()
		return entry.ID, isSelected
	case TabIssues:
		entry, isSelected := m.issuesSelection()
		return entry.ID, isSelected
	case TabKnowledge:
		note, isSelected := m.knowledgeSelection()
		return note.ID, isSelected
	}
	return "", false
}
