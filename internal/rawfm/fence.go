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
// tildes indented by up to three spaces, unless the run is of backticks and another backtick
// follows it on the line, and closes on a run of the same character at least as long followed by
// nothing but whitespace.
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

// ClosedCodeLines reports, for each of lines without its line ending, whether it opens, lies
// inside or closes a fenced code block that a later line closes, following blocks from line from
// on; lines before from are never code. A block opens and closes as IsCode has it, except that an
// opening line no later line closes is plain text, so a block left open hides nothing. It runs in
// time linear in the number of lines.
func ClosedCodeLines(lines []string, from int) []bool {
	longestClosers := longestClosersFrom(lines)
	isCode := make([]bool, len(lines))
	var fence CodeFence
	for index := from; index < len(lines); index++ {
		line := lines[index]
		if fence.open == "" {
			opener := codeFenceOpener(line)
			if opener == "" || longestClosers[opener[0]][index+1] < len(opener) {
				continue
			}
		}
		isCode[index] = fence.IsCode(line)
	}
	return isCode
}

// longestClosersFrom returns, for backticks and for tildes, the longest run of the character any
// line of lines from index i on could close a fenced code block with, at index i. Each slice holds
// len(lines)+1 entries, the last 0.
func longestClosersFrom(lines []string) map[byte][]int {
	longest := make(map[byte][]int, 2)
	for _, fence := range []string{backtickFence, tildeFence} {
		char := fence[0]
		runs := make([]int, len(lines)+1)
		for index := len(lines) - 1; index >= 0; index-- {
			runs[index] = max(runs[index+1], closingRunLength(lines[index], char))
		}
		longest[char] = runs
	}
	return longest
}

// codeFenceOpener returns the run of backticks or tildes that opens a fenced code block on line,
// or "" when line opens none. As in CommonMark, a backtick run followed by text holding another
// backtick, such as "```inline``` quoted", is inline code rather than a fence and opens none.
func codeFenceOpener(line string) string {
	trimmed := trimFenceIndent(line)
	for _, fence := range []string{backtickFence, tildeFence} {
		if !strings.HasPrefix(trimmed, fence) {
			continue
		}
		info := strings.TrimLeft(trimmed, fence[:1])
		if fence == backtickFence && strings.Contains(info, backtickFence[:1]) {
			return ""
		}
		return trimmed[:len(trimmed)-len(info)]
	}
	return ""
}

// closesCodeFence reports whether line closes the fenced code block opened by fence: a run of the
// same character at least as long, followed by nothing but whitespace.
func closesCodeFence(line string, fence string) bool {
	return closingRunLength(line, fence[0]) >= len(fence)
}

// closingRunLength returns the length of the run of char that opens line after its fence indent
// when nothing but whitespace follows the run, and 0 otherwise: the longest fence of char that
// line closes.
func closingRunLength(line string, char byte) int {
	trimmed := trimFenceIndent(line)
	rest := strings.TrimLeft(trimmed, string(char))
	if strings.TrimSpace(rest) != "" {
		return 0
	}
	return len(trimmed) - len(rest)
}

// trimFenceIndent removes the up to three leading spaces a code fence may be indented by.
func trimFenceIndent(line string) string {
	for range maxFenceIndent {
		line = strings.TrimPrefix(line, " ")
	}
	return line
}
