// Package termsafe makes untrusted text safe to write to a terminal. Repository files are
// untrusted, and a raw escape sequence in a title or body could clear the screen, set the window
// title or write to the clipboard; Escape rewrites every byte a terminal could act on as visible
// text, so the CLI's text output and the UI draw the same bytes by the same rule.
package termsafe

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// C0 and C1 control ranges escaped by Escape.
const (
	firstPrintableRune = 0x20
	deleteRune         = 0x7f
	firstC1Rune        = 0x80
	lastC1Rune         = 0x9f
)

// Escape returns text with every byte a terminal could act on written as a visible \xHH escape:
// C0 controls other than TAB and LF, DEL, C1 controls (U+0080 to U+009F) and each byte of
// invalid UTF-8. Everything else is returned unchanged.
func Escape(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	for position := 0; position < len(text); {
		character, width := utf8.DecodeRuneInString(text[position:])
		switch {
		case character == utf8.RuneError && width == 1:
			fmt.Fprintf(&out, "\\x%02x", text[position])
		case isTerminalControl(character):
			fmt.Fprintf(&out, "\\x%02x", character)
		default:
			out.WriteString(text[position : position+width])
		}
		position += width
	}
	return out.String()
}

// isTerminalControl reports whether character is a control Escape escapes.
func isTerminalControl(character rune) bool {
	if character == '\t' || character == '\n' {
		return false
	}
	return character < firstPrintableRune || character == deleteRune ||
		(character >= firstC1Rune && character <= lastC1Rune)
}
