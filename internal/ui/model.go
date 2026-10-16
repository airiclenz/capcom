package ui

import (
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/ui/markdown"
	"github.com/airiclenz/capcom/internal/userconfig"
)

// userConfigNoticePrefix opens a status-bar notice about the user config file.
const userConfigNoticePrefix = "user config: "

// defaultsNotice closes a user config or state file notice whose error left the UI on the
// defaults.
const defaultsNotice = " (running on defaults)"

// stateNoticePrefix opens a status-bar notice about a state file that could not be loaded, and
// stateSaveNoticePrefix one about a sidebar width that could not be saved there.
const (
	stateNoticePrefix     = "ui state: "
	stateSaveNoticePrefix = "cannot save the sidebar width: "
)

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
	// execFn returns the command that runs the editor command argv and hands its exit to done:
	// execEditor, which tests replace with a runner of their own.
	execFn func(argv []string, done tea.ExecCallback) tea.Cmd
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
	// theme is every style the UI paints with, from the user config's theme and colours over the
	// terminal background; isLightBackground reports the terminal answered a light background
	// (until it answers, the background counts as dark), and isNoColor that NO_COLOR is set, so
	// the theme has no colours.
	theme             theme
	isLightBackground bool
	isNoColor         bool
	// markdownTheme is what issue and Note bodies render with: the theme's markdown styles. Its
	// Measure follows the painter: Update folds every tea.ModeReportMsg into it.
	markdownTheme markdown.Theme

	width  int
	height int
	// sidebarColumns is the sidebar's width in columns the user set in the split layout, 0 for
	// the default; sidebarWidth clamps it to the terminal without changing it. It starts on the
	// width the state file holds.
	sidebarColumns int
	// stateSaver writes sidebarColumns to the state file, shared by every copy of the model.
	// stateNotice reports a state file that could not be loaded or saved, empty when there is
	// nothing to report; stateSavedRequest is the request of the last save it reported on, so an
	// older save's report never overrides it.
	stateSaver        *stateSaver
	stateNotice       string
	stateSavedRequest int
	// drag is the drag of the divider under way, its zero value when there is none.
	drag dividerDrag

	// isHelpOpen reports the help screen covering the panes.
	isHelpOpen bool
	// isDetailOpen reports, in the narrow layout, the detail pane filling the screen in place of
	// the sidebar.
	isDetailOpen bool
	// sidebar and detail are the rows of the two panes, top to bottom; detailOffset is the first
	// detail row on screen. sidebar holds only the list's rows on screen: sidebarTop is the first
	// of them and sidebarTotal all of the list's rows, which the sidebar's scrollbar stands for.
	sidebar      []string
	sidebarTop   int
	sidebarTotal int
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

	// prompt is the confirmation a write or an edit asks in the status bar, taking every key while
	// it is open; textPrompt is the reason or Log line c, R and L ask for there, taking every key
	// while it is open; picker is the status picker s opens over the detail pane, taking every key
	// while it is open; writeNotice is the status bar's report of the last write or edit, empty
	// when there is none.
	prompt      confirmPrompt
	textPrompt  textPrompt
	picker      statusPicker
	writeNotice string
}

// newModel returns the model of a UI over options' store and index, opening on options.Tab and
// selecting options.ID there when it is set. It loads the user config at options.UserConfigPath,
// or at userconfig.Path when that is empty, creating the file when it is missing; a config that
// cannot be used leaves the model on userconfig.Default with a notice in the status bar. It
// starts the sidebar on the width the state file at options.UserStatePath, or at
// userconfig.StatePath when that is empty, holds; a state file that cannot be used leaves the
// default width with a notice in the status bar. The theme starts on a dark background, and
// without colours when options.NoColor is set.
func newModel(options Options) model {
	config, notice := loadUserConfig(options.UserConfigPath)
	sidebarColumns, saver, stateNotice := loadUserState(options.UserStatePath)
	m := model{
		session:        newSession(options.Store, options.Index),
		tab:            options.Tab,
		initialID:      options.ID,
		userConfig:     config,
		configNotice:   notice,
		sidebarColumns: sidebarColumns,
		stateSaver:     saver,
		stateNotice:    stateNotice,
		isNoColor:      options.NoColor,
		lockText:       lockReadingText,
		tick:           scheduleTick,
		execFn:         execEditor,
	}
	m.theme = m.themeOf()
	m.markdownTheme = m.theme.markdown
	return m
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

// loadUserState loads the state file at path, resolving the default path when path is empty. It
// returns the sidebar width stored there, 0 for none, the saver that writes the state file at that
// path, and the status-bar notice of an error, "" when there is none. A path that cannot be
// resolved and a file that cannot be used both leave the width at 0; the saver of a path that
// cannot be resolved reports that error on every save and writes nothing.
func loadUserState(path string) (int, *stateSaver, string) {
	if path == "" {
		resolved, err := userconfig.StatePath()
		if err != nil {
			return 0, &stateSaver{pathErr: err}, stateNoticePrefix + err.Error() + defaultsNotice
		}
		path = resolved
	}
	saver := &stateSaver{path: path}
	state, err := userconfig.LoadState(path)
	if err != nil {
		return 0, saver, stateNoticePrefix + err.Error() + defaultsNotice
	}
	return state.UI.SidebarWidth, saver, ""
}

// stateSavedMsg reports a write of the state file: request numbers the save it wrote, and err is
// the write's error, nil when it succeeded.
type stateSavedMsg struct {
	request int
	err     error
}

// stateSaver writes the state file. The model holds it by pointer, so every save command built
// from any copy of the model shares it. Bubble Tea runs each command in its own goroutine, so save
// commands can run out of order: each writes the latest width asked for, under the mutex, and one
// that finds that width written already writes nothing. pathErr, when set, is why no path could
// be resolved; every save then reports it.
type stateSaver struct {
	path    string
	pathErr error

	mutex sync.Mutex
	// width is the latest width asked for, asked the number of saves asked for so far and written
	// the number of the last one written.
	width   int
	asked   int
	written int
}

// save makes width the latest width asked for and returns the command that writes it, reported
// as a stateSavedMsg.
func (s *stateSaver) save(width int) tea.Cmd {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.width = width
	s.asked++
	return s.write
}

// write writes the latest width asked for, 0 leaving the width out, and reports it as a
// stateSavedMsg numbered by that save. It reports nothing when that save is written already.
func (s *stateSaver) write() tea.Msg {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.written == s.asked {
		return nil
	}
	s.written = s.asked
	err := s.pathErr
	if err == nil {
		err = userconfig.SaveState(s.path, userconfig.State{UI: userconfig.UIState{SidebarWidth: s.width}})
	}
	return stateSavedMsg{request: s.written, err: err}
}

// saveState returns the command that writes the sidebar's width as the user set it to the state
// file, reported as a stateSavedMsg.
func (m model) saveState() tea.Cmd {
	return m.stateSaver.save(m.sidebarColumns)
}

// applyStateSaved shows a failed write of the state file in the status bar and clears the state
// file notice after a successful one; the report of a save older than the last one reported on
// changes nothing.
func (m model) applyStateSaved(msg stateSavedMsg) model {
	if msg.request < m.stateSavedRequest {
		return m
	}
	m.stateSavedRequest = msg.request
	m.stateNotice = ""
	if msg.err != nil {
		m.stateNotice = stateSaveNoticePrefix + msg.err.Error()
	}
	return m
}

// Init reads this worktree's Tree lock for the status bar, the Ready queue, the issues of the
// Issues tree and the Notes of the Knowledge tree, and schedules the first poll of the index; the
// tab the UI opened on selects the id it was opened with. Under theme auto, with colours on, it
// also asks the terminal for its background colour, which picks the preset once it answers.
func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		readLock(m.session),
		loadReady(m.session, tabRead{wantedID: m.initialIDFor(TabReady)}),
		loadIssues(m.session, tabRead{wantedID: m.initialIDFor(TabIssues)}),
		loadKnowledge(m.session, tabRead{wantedID: m.initialIDFor(TabKnowledge)}),
		m.tick(),
	}
	if m.userConfig.UI.Theme == userconfig.ThemeAuto && !m.isNoColor {
		cmds = append(cmds, tea.RequestBackgroundColor)
	}
	return tea.Batch(cmds...)
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
// index, a write's run, the status picker's statuses, an edit's steps, a save of the state file,
// the painter's width measure and the terminal's background colour, polls the index on a tick, and handles the keys:
// an open confirmation prompt takes
// every key, then an open text prompt, then an open status picker, then the open filter line,
// ahead of the dependency view's, then the model's own. While the dependency view is open it keeps the detail
// pane over whatever else repaints it. Clicks and wheel notches go to the mouse handlers, which
// ignore them while a prompt, the status picker, the filter line or the help screen is open, and
// the pointer's motion and the button's release to the divider's drag, which a click on the
// divider starts. Any
// other message, such as a paste, reaches the open text prompt or the open filter line.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textPrompt.line.setWidth(m.width)
		m = m.relayout()
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
	case editPreparedMsg:
		return m.applyEditPrepared(msg)
	case editedMsg:
		return m.applyEdited(msg)
	case checkedMsg:
		return m.applyChecked(msg)
	case stateSavedMsg:
		m = m.applyStateSaved(msg)
	case tea.ModeReportMsg:
		m.markdownTheme.Measure = m.markdownTheme.Measure.Observe(msg)
		// A new measure can wrap the shown issue differently, so its cached rendering goes.
		m.issue.renderedWidth = 0
		m = m.renderDetail().paintDeps()
	case tea.BackgroundColorMsg:
		m = m.applyBackground(msg)
	case tea.MouseClickMsg:
		return m.mouseClick(msg)
	case tea.MouseWheelMsg:
		return m.mouseWheel(msg)
	case tea.MouseMotionMsg:
		m = m.mouseMotion(msg)
	case tea.MouseReleaseMsg:
		return m.mouseRelease()
	case tea.KeyPressMsg:
		if m.prompt.isOpen {
			return m.promptKey(msg)
		}
		if m.textPrompt.isOpen {
			return m.textPromptKey(msg)
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
		if m.textPrompt.isOpen {
			var cmd tea.Cmd
			m.textPrompt.line, cmd = m.textPrompt.line.update(msg)
			return m, cmd
		}
		if m.isFilterOpen {
			var cmd tea.Cmd
			m.filterLine, cmd = m.filterLine.update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// View draws the frame on the alternate screen, with the terminal reporting clicks and the wheel.
// Until the first tea.WindowSizeMsg the size is unknown and the frame is empty.
func (m model) View() tea.View {
	view := tea.NewView(m.frame())
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
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

// relayout repaints the panes at the layout's widths, after the terminal or the sidebar changed
// size: the filter line, the sidebar's rows and the detail pane's, kept within its lines.
func (m model) relayout() model {
	m.filterLine.setWidth(m.sidebarPaneWidth())
	return m.paintSidebar().renderDetail().scrollDetail(0).paintDeps()
}

// paintSidebar fills the sidebar with the open tab's rows.
func (m model) paintSidebar() model {
	return m.paintReadySidebar().paintIssuesSidebar().paintKnowledgeSidebar()
}

// withSidebar fills the sidebar with rows, the list's rows on screen from its row top on among
// total rows, so the scrollbar never measures a list other than the one shown.
func (m model) withSidebar(rows []string, top, total int) model {
	m.sidebar = rows
	m.sidebarTop = top
	m.sidebarTotal = total
	return m
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
