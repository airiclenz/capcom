package issue

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/airiclenz/capcom/internal/edit"
)

// LogHeading is the line that opens an issue's Log section.
const LogHeading = "## Log"

// logContinuationIndent indents the second and later lines of a multi-line Log entry, keeping them
// inside the entry's list item.
const logContinuationIndent = "  "

// AppendLog returns a copy of the issue with the entry `- YYYY-MM-DD <author>: <text>` appended
// as the last item of its Log section, the section that opens with a LogHeading line and runs to
// the next level-one or level-two heading. A body without that section gets one at its end. date
// is written as its calendar date in its own location; the second and later lines of a multi-line
// text are indented to stay inside the entry. This is the single writer of Log entries: capcom log
// and reopen both append through it.
func (i Issue) AppendLog(date time.Time, author string, text string) Issue {
	lines := strings.Split(strings.TrimSpace(edit.NormalizeNewlines(text)), "\n")
	entry := fmt.Sprintf(
		"- %s %s: %s\n",
		date.Format(time.DateOnly),
		author,
		strings.Join(lines, "\n"+logContinuationIndent),
	)
	i.Body = appendLogEntry(i.Body, entry)
	return i
}

// appendLogEntry inserts entry after the last non-blank line of body's Log section, or appends a
// new Log section holding it.
func appendLogEntry(body string, entry string) string {
	lines := strings.SplitAfter(body, "\n")
	start := slices.IndexFunc(lines, func(line string) bool {
		return strings.TrimRight(line, " \t\n") == LogHeading
	})
	if start < 0 {
		return edit.AppendBlock(body, LogHeading+"\n\n"+entry)
	}

	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		if isSectionBoundary(lines[index]) {
			end = index
			break
		}
	}
	last := end - 1
	for last > start && strings.TrimSpace(lines[last]) == "" {
		last--
	}

	before := edit.WithFinalNewline(strings.Join(lines[:last+1], ""))
	if last == start {
		// The section has only its heading; a blank line separates it from the first entry.
		before += "\n"
	}
	return before + entry + strings.Join(lines[last+1:], "")
}

// isSectionBoundary reports whether line is a level-one or level-two heading, which ends the Log
// section; deeper headings belong to it.
func isSectionBoundary(line string) bool {
	heading := strings.TrimRight(line, "\n")
	for _, prefix := range []string{"#", "##"} {
		if heading == prefix || strings.HasPrefix(heading, prefix+" ") {
			return true
		}
	}
	return false
}
