// Package frontmatter reads and writes the files capcom keeps under .capcom: markdown with a YAML
// frontmatter between two `---` lines, an issue or a Note alike. Split separates the frontmatter
// from the body, Decode drives a per-kind callback over the frontmatter keys, and Error is the one
// failure both report, naming the file and the line when one is to blame. Encoder writes a file
// back in the canonical form, under the one quoting policy of docs/format.md. The package knows no
// kind's keys: a Kind names them, so issues and Notes share every rule of the format (fences, CRLF,
// duplicate and unknown keys, YAML-error lines, the conflict-marker override) and differ only in
// their keys.
package frontmatter

import (
	"bytes"
	"fmt"
	"strings"
)

// fence is the line that opens and closes the frontmatter.
const fence = "---"

// fenceLine is a fence with its line ending.
const fenceLine = fence + "\n"

// frontmatterLineOffset turns a line of the frontmatter YAML into a line of the file: the opening
// fence is line 1.
const frontmatterLineOffset = 1

// openingFenceLine is the file line of the opening fence, which the fence failures cite.
const openingFenceLine = 1

// Error is a file that cannot be used. Line is the 1-based line of the offending key or value, or
// 0 when no single line is to blame.
type Error struct {
	Path    string
	Line    int
	Message string
}

// Error renders "path:line: message", or "path: message" when the line is unknown.
func (e *Error) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", e.Path, e.Line, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// Split separates the file content data into its frontmatter YAML and its body, both with CRLF
// line endings read as LF. The frontmatter is everything between the opening `---` line and the
// first later line that is exactly `---`; the body is everything after that line, byte for byte.
// path names the file in an error, and noun names one file of its kind with its article ("an
// issue", "a note").
//
// Split fails, with an *Error citing line 1, when data does not open with a `---` line ("an issue
// file must start with a --- line") and when the frontmatter is never closed.
func Split(path string, data []byte, noun string) (front string, body string, err error) {
	text := string(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
	if !strings.HasPrefix(text, fenceLine) {
		return "", "", &Error{
			Path:    path,
			Line:    openingFenceLine,
			Message: noun + " file must start with a --- line",
		}
	}
	front, body, isClosed := splitAtFence(text[len(fenceLine):])
	if !isClosed {
		return "", "", &Error{
			Path:    path,
			Line:    openingFenceLine,
			Message: "the frontmatter opened here is never closed by a --- line",
		}
	}
	return front, body, nil
}

// splitAtFence splits rest, the content after the opening fence, at the first line that is exactly
// a fence. isClosed is false when there is none.
func splitAtFence(rest string) (front string, body string, isClosed bool) {
	offset := 0
	for {
		end := strings.IndexByte(rest[offset:], '\n')
		if end < 0 {
			if rest[offset:] == fence {
				return rest[:offset], "", true
			}
			return "", "", false
		}
		if rest[offset:offset+end] == fence {
			return rest[:offset], rest[offset+end+1:], true
		}
		offset += end + 1
	}
}
