package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/userconfig"
)

// userConfigNoticePrefix opens a status-bar notice about the user config file.
const userConfigNoticePrefix = "user config: "

// defaultsNotice closes a user config notice whose error left the UI on the default config.
const defaultsNotice = " (running on defaults)"

// model is the Bubble Tea model of the UI. It holds the session every store and index access goes
// through, the open tab, the id to select there, the user config it runs on, the terminal size
// from the last tea.WindowSizeMsg, and the state of the panes and the status bar.
type model struct {
	session   *session
	tab       Tab
	initialID string

	// userConfig is the user config the UI runs on: Default when the file could not be used.
	userConfig userconfig.Config
	// configNotice reports the user config's warnings or error in the status bar; empty when the
	// file loaded cleanly.
	configNotice string
	// lockText is the status bar's account of this worktree's Tree lock, from the last read.
	lockText string

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

// Init reads this worktree's Tree lock for the status bar.
func (m model) Init() tea.Cmd {
	return readLock(m.session)
}

// Update records a new terminal size and the Tree lock's state, and handles the keys.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.scrollDetail(0)
	case lockMsg:
		m.lockText = msg.text
	case tea.KeyPressMsg:
		return m.handleKey(msg)
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
