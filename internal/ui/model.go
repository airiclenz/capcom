package ui

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/ui/markdown"
	"github.com/airiclenz/capcom/internal/userconfig"
)

// userConfigNoticePrefix opens a status-bar notice about the user config file.
const userConfigNoticePrefix = "user config: "

// defaultsNotice closes a user config, state file or worktree state file notice whose error left
// the UI on the defaults.
const defaultsNotice = " (running on defaults)"

// stateNoticePrefix opens a status-bar notice about a state file that could not be loaded, and
// stateSaveNoticePrefix one about a divider position, the split layout's or the stacked
// layout's, that could not be saved there.
const (
	stateNoticePrefix     = "ui state: "
	stateSaveNoticePrefix = "cannot save the divider position: "
)

// worktreeStateFileName is the worktree state file's name in the index's directory, and
// worktreeStateNoticePrefix opens a status-bar notice about one that could not be loaded.
const (
	worktreeStateFileName     = "ui-state.yaml"
	worktreeStateNoticePrefix = "ui worktree state: "
)

// model is the Bubble Tea model of the UI. It holds the session every store and index access goes
// through, the open tab, the ids to select at start, the user config it runs on, the terminal
// size from the last tea.WindowSizeMsg, live refresh's state, and the state of the panes and the
// status bar.
type model struct {
	session *session
	tab     Tab
	// initialID is the id the command line named to select in initialTab, "" when it named none;
	// storedIDs are the ids the worktree state file stores to select in each tab, indexed by Tab,
	// "" for none. Each tab selects its stored id unless the command line named one there.
	initialID  string
	initialTab Tab
	storedIDs  [len(tabNames)]string
	// version is the release version the tab bar's version label names; empty, it draws none.
	version string

	// tick returns the command that asks for the next poll of the index: scheduleTick, which
	// tests replace so no real clock runs.
	tick func() tea.Cmd
	// now returns the current time, which tells a double-click on the divider from two presses:
	// time.Now, which tests replace with a clock of their own.
	now func() time.Time
	// execFn returns the command that runs the editor command argv and hands its exit to done:
	// execEditor, which tests replace with a runner of their own.
	execFn func(argv []string, done tea.ExecCallback) tea.Cmd
	// setClipboard returns the command that writes text to the terminal's clipboard through OSC 52:
	// tea.SetClipboard. writeNative and writeTmux write text to the native clipboard and to tmux's
	// paste buffer: a clipboard.Writer's WriteNative and WriteTmux, fenced to the store's working
	// tree. flashTick schedules the flash's clear and saveTick a selection's debounced save to the
	// worktree state file: tea.Tick both. Tests replace all five, so no test touches a real
	// clipboard, runs tmux or runs a real clock.
	setClipboard func(text string) tea.Cmd
	writeNative  func(text string) error
	writeTmux    func(text string) error
	flashTick    tickFunc
	saveTick     tickFunc
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
	// lockBadge is how the status bar paints the word lock leading lockText; none until the first
	// read returns.
	lockBadge lockBadge
	// notice is the status bar's report of an id to select that could not be selected; empty when
	// there is none.
	notice string
	// flash is the status bar's first notice for flashDuration, the copy notice, empty when there
	// is none; flashGen numbers the flash shown last, so only its own flashClearMsg clears it.
	flash    string
	flashGen int
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
	// sidebarRatio is the sidebar's width the user set in the split layout, as a fraction of the
	// terminal's width, 0 for the default; sidebarWidth scales it to the terminal and clamps it
	// without changing it, so the split holds when the terminal resizes. It starts on the ratio
	// the state file holds.
	sidebarRatio float64
	// stackedRatio is the sidebar's height the user set in the stacked layout, as a fraction of
	// the body's rows, 0 for the default. It starts on the ratio the state file holds.
	stackedRatio float64
	// stateSaver writes sidebarRatio and stackedRatio, together, to the state file, shared by
	// every copy of the model.
	// stateNotice reports a state file that could not be loaded or saved, empty when there is
	// nothing to report; stateSavedRequest is the request of the last save it reported on, so an
	// older save's report never overrides it.
	stateSaver        *stateSaver
	stateNotice       string
	stateSavedRequest int
	// worktreeStateNotice reports a worktree state file that could not be loaded, or the first
	// that could not be saved, empty when there is nothing to report. worktree saves the open tab
	// and each tab's selection to that file as they change.
	worktreeStateNotice string
	worktree            worktreeTracking
	// drag is the drag of the divider under way, its zero value when there is none.
	drag dividerDrag
	// lastDividerPress is when the divider's hit area was last pressed, the zero time when the
	// next press there cannot complete a double-click.
	lastDividerPress time.Time

	// isHelpOpen reports the help screen covering the panes.
	isHelpOpen bool
	// isDetailOpen reports, in the one-pane layout of a narrow, short terminal, the detail pane
	// filling the screen in place of the sidebar. The stacked layout shows both panes and ignores it.
	isDetailOpen bool
	// sidebar and detail are the rows of the two panes, top to bottom; detailOffset is the first
	// detail row on screen. sidebar holds only the list's rows on screen: sidebarTop is the first
	// of them and sidebarTotal all of the list's rows, which the sidebar's scrollbar stands for.
	sidebar      []string
	sidebarTop   int
	sidebarTotal int
	detail       []string
	detailOffset int
	// sel is the drag selection over the shown issue's or Note's rendered lines, its zero value
	// when there is none: a left press on the detail pane's text anchors it, a drag extends it, and
	// a key, a press elsewhere, another issue or Note, the dependency view, a tab switch or a
	// re-render that rewrites a line it spans clears it.
	sel detailSel

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

// newModel returns the model of a UI over options' store and index. It loads the worktree state
// file at options.UIStatePath, or ui-state.yaml in the index's directory when that is empty, and
// opens on options.Tab when options.IsTabGiven or the file stores no tab, otherwise on the stored
// tab, Ready when no tab has that name; options.ID is selected in options.Tab when it is set, and
// each other tab selects the id the file stores for it. A worktree state file that cannot be used
// leaves no tab and no ids stored, with a notice in the status bar. It loads the user config at options.UserConfigPath,
// or at userconfig.Path when that is empty, creating the file when it is missing; a config that
// cannot be used leaves the model on userconfig.Default with a notice in the status bar. It
// starts both divider ratios, the split layout's and the stacked layout's, on the ones the state
// file at options.UserStatePath, or at userconfig.StatePath when that is empty, holds; a state
// file that cannot be used leaves both at their defaults with a notice in the status bar. The theme starts on a dark background, and
// without colours when options.NoColor is set. A copy writes through a clipboard.Writer fenced to
// options.Store's working tree.
func newModel(options Options) model {
	config, notice := loadUserConfig(options.UserConfigPath)
	userState, saver, stateNotice := loadUserState(options.UserStatePath)
	worktreeState, worktreeSaver, worktreeStateNotice := loadWorktreeState(options.UIStatePath, options.Store)
	writer := clipboardWriter(options.Store.WorktreeRoot)
	m := model{
		session:    newSession(options.Store, options.Index),
		tab:        openedTab(options.Tab, options.IsTabGiven, worktreeState.Tab),
		initialID:  options.ID,
		initialTab: options.Tab,
		storedIDs: [len(tabNames)]string{
			TabReady:     worktreeState.Ready,
			TabIssues:    worktreeState.Issues,
			TabKnowledge: worktreeState.Knowledge,
		},
		version:             options.Version,
		userConfig:          config,
		configNotice:        notice,
		sidebarRatio:        userState.SidebarRatio,
		stackedRatio:        userState.StackedRatio,
		stateSaver:          saver,
		stateNotice:         stateNotice,
		worktreeStateNotice: worktreeStateNotice,
		worktree:            worktreeTracking{saver: worktreeSaver},
		isNoColor:           options.NoColor,
		lockText:            lockReadingText,
		tick:                scheduleTick,
		now:                 time.Now,
		execFn:              execEditor,
		setClipboard:        tea.SetClipboard,
		writeNative:         writer.WriteNative,
		writeTmux:           writer.WriteTmux,
		flashTick:           tea.Tick,
		saveTick:            tea.Tick,
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
// returns the layout stored there, both divider ratios, each 0 for none, the saver that writes the
// state file at that path, and the status-bar notice of an error, "" when there is none. A path
// that cannot be resolved and a file that cannot be used both leave both ratios at 0; the saver of
// a path that cannot be resolved reports that error on every save and writes nothing.
func loadUserState(path string) (userconfig.UIState, *stateSaver, string) {
	if path == "" {
		resolved, err := userconfig.StatePath()
		if err != nil {
			notice := stateNoticePrefix + err.Error() + defaultsNotice
			return userconfig.UIState{}, &stateSaver{pathErr: err}, notice
		}
		path = resolved
	}
	saver := &stateSaver{path: path}
	state, err := userconfig.LoadState(path)
	if err != nil {
		return userconfig.UIState{}, saver, stateNoticePrefix + err.Error() + defaultsNotice
	}
	return state.UI, saver, ""
}

// loadWorktreeState loads the worktree state file at path, ui-state.yaml in the directory of s's
// index when path is empty, never creating it. It returns what the file stores, the saver that
// writes the file at that path, and the status-bar notice of an error, "" when there is none; a
// file that cannot be used stores nothing.
func loadWorktreeState(path string, s *store.Store) (userconfig.WorktreeState, *worktreeStateSaver, string) {
	if path == "" {
		path = filepath.Join(index.Dir(s), worktreeStateFileName)
	}
	saver := &worktreeStateSaver{path: path}
	state, err := userconfig.LoadWorktreeState(path)
	if err != nil {
		return state, saver, worktreeStateNoticePrefix + err.Error() + defaultsNotice
	}
	return state, saver, ""
}

// openedTab returns the tab the UI opens on: given when the command line named it or the worktree
// state file stores no tab, otherwise the stored tab, Ready when no tab has that name.
func openedTab(given Tab, isTabGiven bool, stored string) Tab {
	if isTabGiven || stored == "" {
		return given
	}
	tab, err := ParseTab(stored)
	if err != nil {
		// A name an older or newer capcom wrote opens the default tab without a word.
		return TabReady
	}
	return tab
}

// stateSavedMsg reports a write of the state file: request numbers the save it wrote, and err is
// the write's error, nil when it succeeded.
type stateSavedMsg struct {
	request int
	err     error
}

// stateSaver writes the state file. The model holds it by pointer, so every save command built
// from any copy of the model shares it. Bubble Tea runs each command in its own goroutine, so save
// commands can run out of order: each writes the latest layout asked for, both divider ratios
// together, under the mutex, and one that finds that layout written already writes nothing. pathErr, when set, is why no path could
// be resolved; every save then reports it.
type stateSaver struct {
	path    string
	pathErr error

	mutex sync.Mutex
	// layout is the latest layout asked for, both divider ratios, asked the number of saves asked
	// for so far and written the number of the last one written.
	layout  userconfig.UIState
	asked   int
	written int
}

// save makes layout, both divider ratios, the latest layout asked for and returns the command that
// writes it, reported as a stateSavedMsg.
func (s *stateSaver) save(layout userconfig.UIState) tea.Cmd {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.layout = layout
	s.asked++
	return s.write
}

// write writes the latest layout asked for, both its ratios, a 0 ratio left out, and reports it as
// a stateSavedMsg numbered by that save. It reports nothing when that save is written already.
func (s *stateSaver) write() tea.Msg {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.written == s.asked {
		return nil
	}
	s.written = s.asked
	err := s.pathErr
	if err == nil {
		err = userconfig.SaveState(s.path, userconfig.State{UI: s.layout})
	}
	return stateSavedMsg{request: s.written, err: err}
}

// saveState returns the command that writes both divider ratios the model holds, the sidebar's
// width in the split layout and its height in the stacked layout as the user set them, to the
// state file, reported as a stateSavedMsg; saving one ratio so never drops the other.
func (m model) saveState() tea.Cmd {
	return m.stateSaver.save(userconfig.UIState{
		SidebarRatio: m.sidebarRatio,
		StackedRatio: m.stackedRatio,
	})
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
// Issues tree and the Notes of the Knowledge tree, and schedules the first poll of the index; each
// tab selects the id initialIDFor returns for it. Under theme auto, with colours on, it
// also asks the terminal for its background colour, which picks the preset once it answers.
func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		readLock(m.session),
		loadReady(m.session, m.initialRead(TabReady)),
		loadIssues(m.session, m.initialRead(TabIssues)),
		loadKnowledge(m.session, m.initialRead(TabKnowledge)),
		m.tick(),
	}
	if m.userConfig.UI.Theme == userconfig.ThemeAuto && !m.isNoColor {
		cmds = append(cmds, tea.RequestBackgroundColor)
	}
	return tea.Batch(cmds...)
}

// initialIDFor returns the id tab selects at start and whether the worktree state file stored it:
// the id the command line named when it named one for tab, otherwise the id stored for tab, ""
// when none is.
func (m model) initialIDFor(tab Tab) (string, bool) {
	if m.initialID != "" && tab == m.initialTab {
		return m.initialID, false
	}
	return m.storedIDs[tab], true
}

// initialRead returns tab's first read: it selects the id initialIDFor returns, quietly when the
// worktree state file stored it.
func (m model) initialRead(tab Tab) tabRead {
	id, isStored := m.initialIDFor(tab)
	return tabRead{wantedID: id, isStored: isStored}
}

// Update records a new terminal size, abandoning a drag of the divider whose layout it takes off
// screen, the Tree lock's state, the Ready queue, the issues, the
// Notes, the shown issue or Note, the dependency view's read, a matched filter, a poll of the
// index, a write's run, the status picker's statuses, an edit's steps, a save of the state file or
// the worktree state file, a selection's debounced save, the clear of the copy notice, the
// painter's width measure and the terminal's background colour, polls the index on a tick, and
// handles the keys: an open confirmation prompt takes
// every key, then an open text prompt, then an open status picker, then the open filter line,
// ahead of the dependency view's, then the model's own; every key first clears the detail
// selection. While the dependency view is open it keeps the detail
// pane over whatever else repaints it. Clicks and wheel notches go to the mouse handlers, which
// ignore them while a prompt, the status picker, the filter line or the help screen is open, and
// the pointer's motion and the button's release to the divider's drag, which a click on the
// divider starts, or to the detail selection, which a click on the detail pane's text starts. Any
// other message, such as a paste, reaches the open text prompt or the open filter line. After every
// message it saves a changed tab to the worktree state file at once and a changed selection a
// second after the last change.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	before := m.worktreeSnapshot()
	next, cmd := m.update(msg)
	return next.trackWorktreeState(msg, before, cmd)
}

// update is Update before the worktree state file follows the message.
func (m model) update(msg tea.Msg) (model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textPrompt.line.setWidth(m.width)
		m = m.abandonDragOffLayout().relayout()
	case lockMsg:
		m.lockText = msg.text
		m.lockBadge = msg.badge
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
	case selectionSaveMsg:
		return m.applySelectionSave(msg)
	case worktreeStateSavedMsg:
		m = m.applyWorktreeStateSaved(msg)
	case flashClearMsg:
		m = m.applyFlashClear(msg)
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
		return m.mouseRelease(msg)
	case tea.KeyPressMsg:
		m.sel = detailSel{}
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
