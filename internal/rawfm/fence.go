package rawfm

import "strings"

// Markdown code fences, which open a block whose lines are code rather than markdown.
const (
	backtickFence  = "```"
	tildeFence     = "~~~"
	maxFenceIndent = 3
)

// CodeFence follows the fenced code blocks of a markdown text read line by line. The zero value
// is outside any block.
type CodeFence struct {
	open string
}

// IsCode reports whether line, without its line ending, opens, lies inside or closes a fenced code
// block, given the lines passed before it. A block opens on a run of three or more backticks or
// tildes indented by up to three spaces, and closes on a run of the same character at least as
// long followed by nothing but whitespace.
func (f *CodeFence) IsCode(line string) bool {
	if f.open != "" {
		if closesCodeFence(line, f.open) {
			f.open = ""
		}
		return true
	}
	f.open = codeFenceOpener(line)
	return f.open != ""
}

// IsOpen reports whether the lines passed so far leave a fenced code block open.
func (f *CodeFence) IsOpen() bool {
	return f.open != ""
}

// codeFenceOpener returns the run of backticks or tildes that opens a fenced code block on line,
// or "" when line opens none.
func codeFenceOpener(line string) string {
	trimmed := trimFenceIndent(line)
	for _, fence := range []string{backtickFence, tildeFence} {
		if strings.HasPrefix(trimmed, fence) {
			return trimmed[:len(trimmed)-len(strings.TrimLeft(trimmed, fence[:1]))]
		}
	}
	return ""
}

// closesCodeFence reports whether line closes the fenced code block opened by fence: a run of the
// same character at least as long, followed by nothing but whitespace.
func closesCodeFence(line string, fence string) bool {
	trimmed := trimFenceIndent(line)
	rest := strings.TrimLeft(trimmed, fence[:1])
	return len(trimmed)-len(rest) >= len(fence) && strings.TrimSpace(rest) == ""
}

// trimFenceIndent removes the up to three leading spaces a code fence may be indented by.
func trimFenceIndent(line string) string {
	for range maxFenceIndent {
		line = strings.TrimPrefix(line, " ")
	}
	return line
}
