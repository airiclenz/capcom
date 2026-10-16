package ui

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/store"
)

// keyStatus opens the status picker over the selected issue.
const keyStatus = "s"

// The status picker's texts.
const (
	// pickerHeadingFormat heads the picker with the issue it sets the status of.
	pickerHeadingFormat = "status of %s:"
	// hintsPicker are the status bar's key hints while the picker is open.
	hintsPicker = "↑↓ move · enter set · esc cancel"
	// statusesFailurePrefix opens the notice of statuses that could not be read.
	statusesFailurePrefix = "status: "
)

// statusesMsg carries the statuses the picker offers for the issue id names, read from the
// store's config, or the error that kept them from being read.
type statusesMsg struct {
	id       string
	statuses []string
	err      error
}

// statusPicker is the list of statuses s opens over the detail pane: the issue it sets the status
// of, the configured statuses outside the closed categories, in config order, and the cursor over
// them.
type statusPicker struct {
	isOpen   bool
	id       string
	statuses []string
	cursor   listCursor
}

// loadStatuses returns the command that reads, through the session, the statuses the picker
// offers for the issue id names.
func loadStatuses(s *session, id string) tea.Cmd {
	return func() tea.Msg {
		msg := statusesMsg{id: id}
		msg.err = s.do(func(opened *store.Store, _ *index.Index) error {
			msg.statuses = openStatuses(opened.Config)
			return nil
		})
		return msg
	}
}

// openStatuses returns the names of cfg's statuses outside the closed categories, in config
// order: the statuses capcom update --status can move an open issue to.
func openStatuses(cfg config.Config) []string {
	names := make([]string, 0, len(cfg.Statuses))
	for _, status := range cfg.Statuses {
		if !status.Category.IsClosed() {
			names = append(names, status.Name)
		}
	}
	return names
}

// openStatusPicker returns the command that reads the statuses for the picker over the issue the
// detail pane shows, refusing as startWrite does: nothing on the Knowledge tab or before the
// selected issue has been read, and a notice for an issue whose file does not parse.
func (m model) openStatusPicker() (model, tea.Cmd) {
	write, notice, isReady := m.shownWrite(writeAction{})
	m.writeNotice = notice
	if !isReady {
		return m, nil
	}
	return m, loadStatuses(m.session, write.id)
}

// applyStatuses opens the picker over the issue the statuses were read for, its cursor on the
// issue's status, while the detail pane still shows that issue; statuses that could not be read
// are a notice instead.
func (m model) applyStatuses(msg statusesMsg) model {
	if msg.id != m.issue.id {
		return m
	}
	if msg.err != nil {
		m.writeNotice = statusesFailurePrefix + oneLine(msg.err.Error())
		return m
	}
	m.picker = statusPicker{isOpen: true, id: msg.id, statuses: msg.statuses}
	if row := slices.Index(msg.statuses, m.issue.node.Status); row >= 0 {
		m.picker.cursor.seat(row, len(msg.statuses))
	}
	return m
}

// pickerKey gives one key to the open status picker, which takes every key: the up and down
// arrows and j k move its cursor, Enter closes it and sets the selected status on the issue the
// detail pane shows, when that is still the issue it was opened over, Esc closes it with nothing
// written, ctrl+c quits, and every other key does nothing.
func (m model) pickerKey(msg tea.KeyPressMsg) (model, tea.Cmd) {
	switch msg.String() {
	case keyInterrupt:
		return m, tea.Quit
	case keyBack:
		m.picker = statusPicker{}
		return m, nil
	case keyOpen:
		picker := m.picker
		m.picker = statusPicker{}
		row := picker.cursor.highlight(len(picker.statuses))
		if row < 0 || picker.id != m.issue.id {
			return m, nil
		}
		return m.startWrite(statusAction(picker.statuses[row]))
	}
	m.picker.cursor.key(msg, len(m.picker.statuses), listStopsAtEnds)
	return m, nil
}

// lines returns the picker's rows: a heading naming the issue, then one row per status, the
// selected one marked in selectedStyle.
func (p statusPicker) lines(selectedStyle lipgloss.Style) []string {
	lines := make([]string, 0, len(p.statuses)+1)
	lines = append(lines, fmt.Sprintf(pickerHeadingFormat, p.id))
	selected := p.cursor.highlight(len(p.statuses))
	for row, status := range p.statuses {
		text := depsRowIndent + status
		if row == selected {
			text = selectedStyle.Render(text)
		}
		lines = append(lines, text)
	}
	return lines
}
