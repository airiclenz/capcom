package issue

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// LogHeading is the line that opens an issue's Log section.
const LogHeading = "## Log"

// logContinuationIndent indents the second and later lines of a multi-line Log entry, keeping them
// inside the entry's list item.
const logContinuationIndent = "  "

// ErrReplaceMatch reports body replacement text that does not occur exactly once in the body.
var ErrReplaceMatch = errors.New("the text to replace must occur exactly once in the body")

// WithLabels returns a copy of the issue with each label of add appended unless it is already
// there, then each label of remove taken out. Labels keep their order; a label to remove that the
// issue does not carry is ignored.
func (i Issue) WithLabels(add []string, remove []string) Issue {
	labels := slices.Clone(i.Labels)
	for _, label := range add {
		if !slices.Contains(labels, label) {
			labels = append(labels, label)
		}
	}
	labels = slices.DeleteFunc(labels, func(label string) bool {
		return slices.Contains(remove, label)
	})
	if len(labels) == 0 {
		labels = nil
	}
	i.Labels = labels
	return i
}

// AppendBody returns a copy of the issue with text appended to its body as a new block, separated
// from what is there by a blank line, and ending in a line break. text is taken literally; CRLF
// line endings in it are read as LF. An empty body gets the leading blank line a new issue file
// has after its frontmatter.
func (i Issue) AppendBody(text string) Issue {
	i.Body = appendBlock(i.Body, withFinalNewline(normalizeNewlines(text)))
	return i
}

// ReplaceBody returns a copy of the issue with the one occurrence of old in its body replaced by
// replacement (CRLF line endings in either read as LF). It fails with an error wrapping
// ErrReplaceMatch when old is empty or occurs in the body other than exactly once.
func (i Issue) ReplaceBody(old string, replacement string) (Issue, error) {
	old = normalizeNewlines(old)
	if old == "" {
		return Issue{}, fmt.Errorf("%w; the text to replace is empty", ErrReplaceMatch)
	}
	if count := strings.Count(i.Body, old); count != 1 {
		return Issue{}, fmt.Errorf("%w; %q occurs %d times", ErrReplaceMatch, old, count)
	}
	i.Body = strings.Replace(i.Body, old, normalizeNewlines(replacement), 1)
	return i, nil
}

// AppendLog returns a copy of the issue with the entry `- YYYY-MM-DD <author>: <text>` appended
// as the last item of its Log section, the section that opens with a LogHeading line and runs to
// the next level-one or level-two heading. A body without that section gets one at its end. date
// is written as its calendar date in its own location; the second and later lines of a multi-line
// text are indented to stay inside the entry. This is the single writer of Log entries: capcom log
// and reopen both append through it.
func (i Issue) AppendLog(date time.Time, author string, text string) Issue {
	lines := strings.Split(strings.TrimSpace(normalizeNewlines(text)), "\n")
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
		return appendBlock(body, LogHeading+"\n\n"+entry)
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

	before := withFinalNewline(strings.Join(lines[:last+1], ""))
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

// appendBlock appends block to body after a blank line. A blank body becomes a blank line and the
// block, the shape of a body right after the frontmatter.
func appendBlock(body string, block string) string {
	if strings.TrimSpace(body) == "" {
		return "\n" + block
	}
	body = withFinalNewline(body)
	if !strings.HasSuffix(body, "\n\n") {
		body += "\n"
	}
	return body + block
}

// withFinalNewline returns text ending in a line break; empty text stays empty.
func withFinalNewline(text string) string {
	if text == "" || strings.HasSuffix(text, "\n") {
		return text
	}
	return text + "\n"
}

// normalizeNewlines turns CRLF line endings into LF, the line ending issue bodies keep.
func normalizeNewlines(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}
