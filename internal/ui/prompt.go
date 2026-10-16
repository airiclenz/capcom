package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/ops"
)

// The texts of the prompts that ask for a reason or a Log entry.
const (
	// closePromptFormat asks for the reason of closing an issue, reopenPromptFormat for the reason
	// of reopening one, and logPromptFormat for the line to append to its Log.
	closePromptFormat  = "close %s, reason: "
	reopenPromptFormat = "reopen %s, reason: "
	logPromptFormat    = "log %s: "
	// blankReasonFormat refuses a blank reason, as capcom close and reopen refuse one.
	blankReasonFormat = "%s; say why"
)

// The keys that answer a confirmation prompt besides Enter and Esc.
const (
	keyYes = "y"
	keyNo  = "n"
)

// confirmPrompt is a yes-or-no question the status bar asks before a write or an edit goes ahead:
// whether to write or edit while another session holds the Tree lock, or whether to run a write
// again after its issue changed on disk. yes is the command a yes runs: the write, or the editor.
type confirmPrompt struct {
	isOpen bool
	text   string
	yes    tea.Cmd
}

// newConfirmPrompt returns an open prompt asking text, whose yes runs the command yes.
func newConfirmPrompt(text string, yes tea.Cmd) confirmPrompt {
	return confirmPrompt{isOpen: true, text: text, yes: yes}
}

// promptKey gives one key to the open confirmation prompt, which takes every key: y or Enter
// closes it and runs its command, n or Esc closes it with nothing run, ctrl+c quits, and every
// other key does nothing.
func (m model) promptKey(msg tea.KeyPressMsg) (model, tea.Cmd) {
	switch msg.String() {
	case keyInterrupt:
		return m, tea.Quit
	case keyYes, keyOpen:
		yes := m.prompt.yes
		m.prompt = confirmPrompt{}
		return m, yes
	case keyNo, keyBack:
		m.prompt = confirmPrompt{}
	}
	return m, nil
}

// textWrite is a write that asks for a line of text first: the prompt that asks for it, with the
// issue id for its verb, the notice that refuses a blank line, and the action the line makes.
type textWrite struct {
	promptFormat string
	blankNotice  string
	action       func(text string) writeAction
}

// closeWrite is the close c makes, asking for its reason.
func closeWrite() textWrite {
	return textWrite{
		promptFormat: closePromptFormat,
		blankNotice:  fmt.Sprintf(blankReasonFormat, ops.ErrBlankReason),
		action:       closeAction,
	}
}

// reopenWrite is the reopen R makes, asking for its reason.
func reopenWrite() textWrite {
	return textWrite{
		promptFormat: reopenPromptFormat,
		blankNotice:  fmt.Sprintf(blankReasonFormat, ops.ErrBlankReason),
		action:       reopenAction,
	}
}

// logWrite is the Log entry L appends, asking for its line.
func logWrite() textWrite {
	return textWrite{
		promptFormat: logPromptFormat,
		blankNotice:  ops.ErrBlankLog.Error(),
		action:       logAction,
	}
}

// textPrompt is the line the status bar asks for before a write that needs text: the write, the
// issue it was opened over, and the line being typed.
type textPrompt struct {
	isOpen bool
	write  textWrite
	id     string
	line   lineInput
}

// openTextPrompt opens the prompt of write over the issue the detail pane shows, refusing as
// startWrite does: nothing on the Knowledge tab or before the selected issue has been read, and a
// notice for an issue whose file does not parse.
func (m model) openTextPrompt(write textWrite) model {
	shown, notice, isReady := m.shownWrite(writeAction{})
	m.writeNotice = notice
	if !isReady {
		return m
	}
	m.textPrompt = textPrompt{
		isOpen: true,
		write:  write,
		id:     shown.id,
		line:   newLineInput(fmt.Sprintf(write.promptFormat, shown.id), "", m.width, m.theme.input),
	}
	return m
}

// textPromptKey gives one key to the open text prompt, which takes every key: ctrl+c quits, Esc
// closes it with nothing written, and Enter closes it and makes its write with the line on the
// issue the detail pane shows, when that is still the issue it was opened over. A blank line is
// refused with the write's notice before anything is read. Every other key edits the line.
func (m model) textPromptKey(msg tea.KeyPressMsg) (model, tea.Cmd) {
	if msg.String() == keyInterrupt {
		return m, tea.Quit
	}
	line, cmd, verdict := m.textPrompt.line.key(msg)
	m.textPrompt.line = line
	switch verdict {
	case lineCancelled:
		m.textPrompt = textPrompt{}
		return m, nil
	case lineSubmitted:
		asked := m.textPrompt
		m.textPrompt = textPrompt{}
		text := asked.line.value()
		if strings.TrimSpace(text) == "" {
			m.writeNotice = writeFailurePrefix + asked.write.blankNotice
			return m, nil
		}
		if asked.id != m.issue.id {
			return m, nil
		}
		return m.startWrite(asked.write.action(text))
	}
	return m, cmd
}
