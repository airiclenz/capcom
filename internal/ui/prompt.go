package ui

import (
	tea "charm.land/bubbletea/v2"
)

// The keys that answer a confirmation prompt besides Enter and Esc.
const (
	keyYes = "y"
	keyNo  = "n"
)

// confirmPrompt is a yes-or-no question the status bar asks before a write goes ahead: whether to
// write while another session holds the Tree lock, or whether to run a write again after its
// issue changed on disk. write is the write a yes runs.
type confirmPrompt struct {
	isOpen bool
	text   string
	write  pendingWrite
}

// newConfirmPrompt returns an open prompt asking text, whose yes runs write.
func newConfirmPrompt(text string, write pendingWrite) confirmPrompt {
	return confirmPrompt{isOpen: true, text: text, write: write}
}

// promptKey gives one key to the open confirmation prompt, which takes every key: y or Enter
// closes it and runs its write, n or Esc closes it with nothing written, ctrl+c quits, and every
// other key does nothing.
func (m model) promptKey(msg tea.KeyPressMsg) (model, tea.Cmd) {
	switch msg.String() {
	case keyInterrupt:
		return m, tea.Quit
	case keyYes, keyOpen:
		write := m.prompt.write
		m.prompt = confirmPrompt{}
		return m, runWrite(m.session, write)
	case keyNo, keyBack:
		m.prompt = confirmPrompt{}
	}
	return m, nil
}
