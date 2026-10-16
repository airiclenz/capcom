package ui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// lineVerdict is what a lineInput did with one key press.
type lineVerdict int

// The verdicts of lineInput.key.
const (
	// lineEditing: the key edited the line or moved its cursor, or did nothing at all.
	lineEditing lineVerdict = iota
	// lineSubmitted: Enter, which hands the line's text to its owner.
	lineSubmitted
	// lineCancelled: Esc, which drops the line.
	lineCancelled
)

// lineInput is a one-row text entry over a bubbles textinput: the filter line, and the prompts
// that ask for a reason or a log entry. While it is open its owner gives it every key: Enter
// submits it, Esc cancels it, and every other key edits it. Its cursor never blinks, so no key
// press schedules a tick.
type lineInput struct {
	input textinput.Model
}

// newLineInput returns a focused line that shows prompt before value, with the cursor after
// value, fitting width columns, painted in styles.
func newLineInput(prompt, value string, width int, styles textinput.Styles) lineInput {
	input := textinput.New()
	input.Prompt = prompt
	line := lineInput{input: input}
	line.setStyles(styles)
	line.setWidth(width)
	line.input.SetValue(value)
	line.input.CursorEnd()
	// A cursor that does not blink has no command to start.
	line.input.Focus()
	return line
}

// setStyles paints the line in styles, its cursor kept from blinking whatever they say.
func (l *lineInput) setStyles(styles textinput.Styles) {
	styles.Cursor.Blink = false
	l.input.SetStyles(styles)
}

// setWidth fits the line, its prompt and cursor included, in width columns: a longer value
// scrolls sideways to keep the cursor in view.
func (l *lineInput) setWidth(width int) {
	l.input.SetWidth(max(width-ansi.StringWidth(l.input.Prompt)-1, 1))
}

// key applies one key press: Enter submits the line and Esc cancels it, both leaving its text as
// it is; any other key goes to the textinput, whose command it returns.
func (l lineInput) key(msg tea.KeyPressMsg) (lineInput, tea.Cmd, lineVerdict) {
	switch msg.String() {
	case keyOpen:
		return l, nil, lineSubmitted
	case keyBack:
		return l, nil, lineCancelled
	}
	var cmd tea.Cmd
	l.input, cmd = l.input.Update(msg)
	return l, cmd, lineEditing
}

// update hands the textinput a message that is not a key press, such as a paste, and returns
// its command.
func (l lineInput) update(msg tea.Msg) (lineInput, tea.Cmd) {
	var cmd tea.Cmd
	l.input, cmd = l.input.Update(msg)
	return l, cmd
}

// value returns the line's text.
func (l lineInput) value() string {
	return l.input.Value()
}

// view returns the line's row: its prompt, then its text with the cursor.
func (l lineInput) view() string {
	return l.input.View()
}
