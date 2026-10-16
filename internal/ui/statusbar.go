package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/lock"
	"github.com/airiclenz/capcom/internal/store"
)

// The status bar's texts.
const (
	statusSeparator = " · "
	lockPrefix      = "lock: "
	// lockReadingText stands in for the Tree lock until its first read returns.
	lockReadingText = lockPrefix + "reading"
	hintsPanes      = "? help · q quit"
	hintsDetail     = "esc back · ? help · q quit"
	hintsHelp       = "esc close · q quit"
)

// lockMsg carries the status bar's account of this worktree's Tree lock from a read.
type lockMsg struct {
	text string
}

// readLock returns a command that reads this worktree's Lock overview through the session and
// reports it as a lockMsg. The UI holds no token, so a held lock is always another session's.
func readLock(s *session) tea.Cmd {
	return func() tea.Msg {
		var overview lock.Overview
		err := s.do(func(opened *store.Store, _ *index.Index) error {
			var readErr error
			overview, readErr = lock.ForWorktree(opened).Overview()
			return readErr
		})
		return lockMsg{text: lockText(overview, err, time.Now())}
	}
}

// lockText renders this worktree's state from a Lock overview read at now: `lock: free`,
// `lock: held by <holder>: <purpose> (heartbeat <age> ago)`, `lock: stale: <purpose> (heartbeat
// <age> ago)`, `lock: invalid` or `lock: unreadable`. An overview that could not be read at all
// is unreadable.
func lockText(overview lock.Overview, err error, now time.Time) string {
	if err != nil {
		return lockPrefix + string(lock.StateUnreadable)
	}
	record := overview.Own.Record
	heartbeat := lock.AgeText(now.Sub(record.Heartbeat))
	switch overview.State {
	case lock.StateHeld:
		return fmt.Sprintf("%sheld by %s: %s (heartbeat %s ago)", lockPrefix, record.Holder, oneLine(record.Purpose), heartbeat)
	case lock.StateStale:
		return fmt.Sprintf("%sstale: %s (heartbeat %s ago)", lockPrefix, oneLine(record.Purpose), heartbeat)
	default:
		return lockPrefix + string(overview.State)
	}
}

// oneLine joins text's lines and runs of spaces into single spaces, for the one-row status bar.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// statusBar draws the bottom row: an open confirmation prompt's question or an open text prompt's
// line in place of everything else, or the Tree lock, the open tab's filter, the user config notice, the notices of a
// config.yaml that could not be reloaded and of a poll that failed or stopped, the notice of an id
// that could not be selected and the report of the last write when there are any, and the key
// hints, muted. A row too long for the terminal is cut before the hints, so they stay visible while
// there is room for them.
func (m model) statusBar() string {
	if m.prompt.isOpen {
		return fit(m.prompt.text, m.width)
	}
	if m.textPrompt.isOpen {
		return fit(m.textPrompt.line.view(), m.width)
	}
	text := m.lockText
	if filter := m.filterStatus(); filter != "" {
		text += statusSeparator + filter
	}
	for _, notice := range []string{m.configNotice, m.storeNotice, m.refreshNotice, m.notice, m.writeNotice} {
		if notice != "" {
			text += statusSeparator + notice
		}
	}
	hints := statusSeparator + m.keyHints()
	room := m.width - ansi.StringWidth(hints)
	painted := m.theme.hints.Render(hints)
	if room <= 0 {
		return fit(text+painted, m.width)
	}
	return fit(text, room) + painted
}

// keyHints returns the hints for the keys that matter in the current view.
func (m model) keyHints() string {
	switch {
	case m.isHelpOpen:
		return hintsHelp
	case m.picker.isOpen:
		return hintsPicker
	case m.isFilterOpen:
		return hintsFilter
	case m.isNarrow() && m.isDetailOpen:
		return hintsDetail
	default:
		return hintsPanes
	}
}
