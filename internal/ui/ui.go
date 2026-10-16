// Package ui is capcom's terminal UI, the program behind capcom ui (docs/design/tui.md): a Bubble
// Tea v2 model over a store and its index that internal/cli has already opened. This package never
// checks for a terminal and never closes the index it is handed; the caller does both.
//
// Every store and index access from a tea.Cmd goes through the session, which runs one access at
// a time; nothing reaches the index outside it. About once a second the session polls: it
// refreshes the index and re-reads the Tree lock, and the UI reloads its tabs when anything
// changed. A changed config.yaml makes the session re-open the store and an index of its own on
// it, which it closes itself.
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
)

// tabOrder is the order of the tabs on the tab bar.
var tabOrder = []Tab{TabIssues, TabReady, TabKnowledge}

// tabNames are the names capcom ui takes on its command line, and tabLabels the tab bar labels,
// both indexed by Tab.
var (
	tabNames  = [...]string{TabReady: "ready", TabIssues: "issues", TabKnowledge: "knowledge"}
	tabLabels = [...]string{TabReady: "Ready", TabIssues: "Issues", TabKnowledge: "Knowledge"}
)

// String returns the tab's command-line name.
func (t Tab) String() string {
	return tabNames[t]
}

// ParseTab returns the tab a command-line name names: issues, ready or knowledge. Any other name
// is an error that lists the valid ones.
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
// index once Run returns; an index Run re-opens on a changed config.yaml it closes itself. ID, when set, names the Issue or Note to select in Tab. In and Out are
// the terminal the program reads keys from and draws on. UserConfigPath names the user config
// file; empty, the default `<os.UserConfigDir>/capcom/config.yaml` is used. NoColor, set when the
// NO_COLOR environment variable is non-empty, draws without any colour, whatever the user config
// file says.
type Options struct {
	Store          *store.Store
	Index          *index.Index
	Tab            Tab
	ID             string
	In             io.Reader
	Out            io.Writer
	UserConfigPath string
	NoColor        bool
}

// Run runs the terminal UI until the user quits, with q or ctrl+c. It first loads the user config
// file, creating it when it is missing; a file it cannot use is reported in the status bar and the
// UI runs on the default config.
func Run(options Options) error {
	initial := newModel(options)
	program := tea.NewProgram(
		initial,
		tea.WithInput(options.In),
		tea.WithOutput(options.Out),
	)
	_, err := program.Run()
	if err != nil {
		err = fmt.Errorf("run the terminal UI: %w", err)
	}
	if closeErr := initial.session.close(); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("close the re-opened index: %w", closeErr))
	}
	return err
}
