package ui

import (
	"errors"
	"fmt"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/store"
)

// Tab is one of the UI's top-level views. The zero value is TabReady, the tab capcom ui opens on.
type Tab int

// The tabs of the UI.
const (
	TabReady Tab = iota
	TabIssues
	TabKnowledge
	TabClosed
)

// tabOrder is the order of the tabs on the tab bar.
var tabOrder = []Tab{TabIssues, TabReady, TabKnowledge, TabClosed}

// tabNames are the names capcom ui takes on its command line, and tabLabels the tab bar labels,
// both indexed by Tab.
var (
	tabNames = [...]string{
		TabReady:     "ready",
		TabIssues:    "issues",
		TabKnowledge: "knowledge",
		TabClosed:    "closed",
	}
	tabLabels = [...]string{
		TabReady:     "Ready",
		TabIssues:    "Issues",
		TabKnowledge: "Knowledge",
		TabClosed:    "Closed",
	}
)

// String returns the tab's command-line name.
func (t Tab) String() string {
	return tabNames[t]
}

// ParseTab returns the tab a command-line name names: issues, ready, knowledge or closed. Any
// other name is an error that lists the valid ones.
func ParseTab(name string) (Tab, error) {
	names := make([]string, 0, len(tabOrder))
	for _, tab := range tabOrder {
		if tab.String() == name {
			return tab, nil
		}
		names = append(names, tab.String())
	}
	return TabReady, fmt.Errorf("unknown tab %q: want %s", name, strings.Join(names, ", "))
}

// Options configure Run. Store and Index are open and stay owned by the caller, who closes the
// index once Run returns; an index Run re-opens on a changed config.yaml it closes itself. ID,
// when set, names the Issue or Note to select in Tab. IsTabGiven is true when the command line
// named Tab; false, no tab was given and the tab the worktree state file stores opens in its
// place, Tab only when it stores none. In and Out are the terminal the program reads keys from
// and draws on. UserConfigPath names the user config file; empty, the default
// `<os.UserConfigDir>/capcom/config.yaml` is used. UserStatePath names the state file the
// sidebar's width is kept in; empty, the default `<os.UserConfigDir>/capcom/state.yaml` is used.
// UIStatePath names the worktree state file the tab and each tab's selection are kept in; empty,
// the default `<git-dir>/capcom/ui-state.yaml` beside the index is used. NoColor, set when the
// NO_COLOR environment variable is non-empty, draws without any colour, whatever the user config
// file says. Version is the release version the tab bar's right end names, as `<path> ·
// v<Version> · capcom`, where path is Store.WorktreeRoot with Home shown as `~`, or as
// `v<Version> · capcom` when the terminal is too narrow for the path; empty, the tab bar draws no
// version label. Home is the home directory the path is shortened by; empty, the user's home
// directory with symlinks resolved is used, and none when it cannot be found.
type Options struct {
	Store          *store.Store
	Index          *index.Index
	Tab            Tab
	ID             string
	IsTabGiven     bool
	In             io.Reader
	Out            io.Writer
	UserConfigPath string
	UserStatePath  string
	UIStatePath    string
	NoColor        bool
	Version        string
	Home           string
}

// Run runs the terminal UI until the user quits, with q or ctrl+c. It first loads the user config
// file, creating it when it is missing; a file it cannot use is reported in the status bar and the
// UI runs on the default config. It then loads the state file, never creating it, and starts the
// sidebar on the width stored there; every width change a key makes is saved to it. A state file
// it cannot load or save is reported in the status bar and the UI runs on. It also loads the
// worktree state file, ui-state.yaml, never creating it: with no tab given it opens on the tab
// stored there, and each tab selects the id stored for it unless the command line named one
// there. A stored tab or id that no longer applies falls back to Ready or the first row without
// a word; a worktree state file it cannot load is reported in the status bar and the UI runs on.
// A tab change is saved to that file at once and a selection change a second after the last one;
// quitting writes a change still waiting. The first save that fails this session is reported in
// the status bar, later ones are not, and the UI runs on.
func Run(options Options) error {
	initial := newModel(options)
	program := tea.NewProgram(
		initial,
		tea.WithInput(options.In),
		tea.WithOutput(options.Out),
	)
	final, err := program.Run()
	if err != nil {
		err = fmt.Errorf("run the terminal UI: %w", err)
	}
	if finalModel, isModel := final.(model); isModel {
		// The status bar is gone, and a file that only remembers where the UI was must not fail
		// capcom ui's exit, so a failed last save goes unreported, as later failures do in a session.
		_ = finalModel.flushWorktreeState()
	}
	if closeErr := initial.session.close(); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("close the re-opened index: %w", closeErr))
	}
	return err
}
