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
	// lockWord opens the Tree lock's segment, padded with a space each side, and is the only part
	// of it the lock badge paints; one plain space separates it from the lock's state.
	lockWord   = " lock "
	lockPrefix = lockWord + " "
	// lockReadingText stands in for the Tree lock until its first read returns.
	lockReadingText = lockPrefix + "reading"
	hintsPanes      = "? help · q quit"
	hintsDetail     = "esc back · ? help · q quit"
	hintsHelp       = "esc close · q quit"
	// layoutButtonToStacked is the layout button's label while the split layout is on screen, and
	// layoutButtonToSplit its label while the stacked one is: the divider of the layout a toggle
	// brings, padded with a space each side.
	layoutButtonToStacked = " ─ "
	layoutButtonToSplit   = " │ "
)

// lockMsg carries the status bar's account of this worktree's Tree lock from a read, and the
// badge its leading word wears.
type lockMsg struct {
	text  string
	badge lockBadge
}

// lockBadge is how the status bar paints the word lock: not at all, as held, or as free. The
// zero value paints nothing.
type lockBadge int

// The lock badges. lockBadgeHeld covers a lock that is held or stale: either way another session
// holds it, as the UI holds no token.
const (
	lockBadgeNone lockBadge = iota
	lockBadgeHeld
	lockBadgeFree
)

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
		return lockMsg{text: lockText(overview, err, time.Now()), badge: lockBadgeOf(overview, err)}
	}
}

// lockBadgeOf returns the badge of this worktree's state from a Lock overview: held for a lock
// that is held or stale, free for a free one, and none for an invalid or unreadable one or an
// overview that could not be read at all.
func lockBadgeOf(overview lock.Overview, err error) lockBadge {
	if err != nil {
		return lockBadgeNone
	}
	switch overview.State {
	case lock.StateHeld, lock.StateStale:
		return lockBadgeHeld
	case lock.StateFree:
		return lockBadgeFree
	default:
		return lockBadgeNone
	}
}

// lockText renders this worktree's state from a Lock overview read at now: ` lock  free`,
// ` lock  held by <holder>: <purpose> (heartbeat <age> ago)`, ` lock  stale: <purpose> (heartbeat
// <age> ago)`, ` lock  invalid` or ` lock  unreadable`. An overview that could not be read at all
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
// line in place of everything else, or the Tree lock, its leading word in its badge, the open
// tab's filter, the copy notice, the user config notice, the state file notice, the worktree state
// file notice, the notices of a config.yaml that could not be reloaded and of a poll that failed
// or stopped, the notice of an id that could not be selected and the report of the last write
// when there are any, the key hints, muted, and the layout button when it is drawn, at the row's
// right edge with blanks between it and the hints. A row too long for the terminal is cut before
// the hints and the button, so they stay visible while there is room for them.
func (m model) statusBar() string {
	if m.prompt.isOpen {
		return fit(m.prompt.text, m.width)
	}
	if m.textPrompt.isOpen {
		return fit(m.textPrompt.line.view(), m.width)
	}
	text := m.paintedLock()
	if filter := m.filterStatus(); filter != "" {
		text += statusSeparator + filter
	}
	for _, notice := range []string{m.flash, m.configNotice, m.stateNotice, m.worktreeStateNotice, m.storeNotice, m.refreshNotice, m.notice, m.writeNotice} {
		if notice != "" {
			text += statusSeparator + notice
		}
	}
	hints := statusSeparator + m.keyHints()
	button := m.layoutButton()
	room := m.width - ansi.StringWidth(hints) - ansi.StringWidth(button)
	painted := m.theme.hints.Render(hints)
	paintedButton := ""
	if button != "" {
		paintedButton = m.theme.layoutButton.Render(button)
	}
	if room <= 0 {
		return fit(text+painted+paintedButton, m.width)
	}
	cut := fit(text, room)
	if button == "" {
		return cut + painted
	}
	padding := strings.Repeat(" ", room-ansi.StringWidth(cut))
	return cut + painted + padding + paintedButton
}

// layoutButton returns the status bar's layout button, unstyled: the divider of the layout a
// toggle brings, on a terminal that fits both layouts while help is closed and no prompt or text
// prompt is open, and "" anywhere else.
func (m model) layoutButton() string {
	if !m.canChooseLayout() || m.isHelpOpen || m.prompt.isOpen || m.textPrompt.isOpen {
		return ""
	}
	if m.isStacked() {
		return layoutButtonToSplit
	}
	return layoutButtonToStacked
}

// paintedLock returns the status bar's account of the Tree lock with its leading padded word painted
// in the lock badge, the rest of it unstyled; with no badge, or a text not led by the word, it is
// the text alone.
func (m model) paintedLock() string {
	rest, isLed := strings.CutPrefix(m.lockText, lockWord)
	if m.lockBadge == lockBadgeNone || !isLed {
		return m.lockText
	}
	return m.theme.lockBadge(m.lockBadge).Render(lockWord) + rest
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
	case m.isSinglePane() && m.isDetailOpen:
		return hintsDetail
	default:
		return hintsPanes
	}
}
