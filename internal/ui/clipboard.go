package ui

import (
	"fmt"
	"os"
	"runtime"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/clipboard"
)

// flashDuration is how long the status bar shows a flash, such as the copy notice, before its
// flashClearMsg clears it.
const flashDuration = 2 * time.Second

// copiedFormat is the copy notice: the count of runes copied and its noun, char or chars.
const copiedFormat = "copied %d %s"

// The copy notice's nouns, for one rune and for any other count.
const (
	copiedNounOne   = "char"
	copiedNounOther = "chars"
)

// flashClearMsg clears the status bar's flash once flashDuration has passed. gen is the flash
// that scheduled it, model.flashGen then: a clear for a flash a later one has since replaced
// changes nothing.
type flashClearMsg struct{ gen int }

// tickFunc schedules a message after a duration: tea.Tick's shape, which tests replace so no real
// clock runs.
type tickFunc func(time.Duration, func(time.Time) tea.Msg) tea.Cmd

// clipboardWriter returns the clipboard.Writer the copy writes through: this process's OS and
// environment, with the working tree at worktreeRoot as the fence no clipboard or tmux program may
// resolve inside.
func clipboardWriter(worktreeRoot string) clipboard.Writer {
	return clipboard.Writer{GOOS: runtime.GOOS, Getenv: os.Getenv, Root: worktreeRoot}
}

// copyText copies text over every route at once: OSC 52 through setClipboard, the native
// clipboard through writeNative and, inside tmux, tmux's paste buffer through writeTmux. It shows
// the copy notice, `copied N chars` with N the runes of text, as the status bar's flash and returns
// the commands of the three writes with the tick that clears the flash after flashDuration.
//
// OSC 52 stays the primary route: it reaches the far end of an ssh session, where no local program
// can. The native and tmux writes are best-effort fallbacks for a terminal that drops the escape
// and for tmux, which drops it on its default set-clipboard external. Any of the three may land
// where the others did not, so the notice shows whatever the fallbacks return.
func (m model) copyText(text string) (model, tea.Cmd) {
	count := utf8.RuneCountInString(text)
	noun := copiedNounOther
	if count == 1 {
		noun = copiedNounOne
	}
	m.flash = fmt.Sprintf(copiedFormat, count, noun)
	m.flashGen++
	gen := m.flashGen
	clearFlash := m.flashTick(flashDuration, func(time.Time) tea.Msg { return flashClearMsg{gen: gen} })
	return m, tea.Batch(
		m.setClipboard(text),
		bestEffortWrite(m.writeNative, text),
		bestEffortWrite(m.writeTmux, text),
		clearFlash,
	)
}

// bestEffortWrite returns the command that hands text to write off the Update goroutine and
// reports nothing. The error is dropped on purpose: the write runs beside OSC 52, never instead of
// it, so a host with no clipboard program or outside tmux keeps the OSC 52 copy without a report
// of a failure for a copy that may well have landed. write is read when the copy builds its
// commands, not when the command runs, so the command never reaches a seam swapped since.
func bestEffortWrite(write func(string) error, text string) tea.Cmd {
	return func() tea.Msg {
		_ = write(text)
		return nil
	}
}

// applyFlashClear clears the status bar's flash when msg is the clear of the flash it shows; the
// clear of a flash since replaced changes nothing.
func (m model) applyFlashClear(msg flashClearMsg) model {
	if msg.gen == m.flashGen {
		m.flash = ""
	}
	return m
}
