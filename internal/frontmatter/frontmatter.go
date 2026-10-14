// Package frontmatter reads and writes the files capcom keeps under .capcom: markdown with a YAML
// frontmatter between two `---` lines, an issue or a Note alike. Split separates the frontmatter
// from the body, Decode drives a per-kind callback over the frontmatter keys, and Error is the one
// failure both report, naming the file and the line when one is to blame. Encoder writes a file
// back in the canonical form, under the one quoting policy of docs/site/reference/file-format.md.
// The package knows no kind's keys: a Kind names them, so issues and Notes share every rule of the
// format (fences, CRLF, duplicate and unknown keys, YAML-error lines, the conflict-marker override)
// and differ only in their keys.
package frontmatter

import (
	"fmt"
	"strings"

	"github.com/airiclenz/capcom/internal/rawfm"
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
// first later line that is a fence, `---` with at most one trailing carriage return
// (rawfm.IsFrontmatterFence), so a closing `---\r` that ends the file closes it too; the body is
// everything after that line, byte for byte. path names the file in an error, and noun names one
// file of its kind with its article ("an issue", "a Note").
//
// Split fails, with an *Error citing line 1, when data does not open with a `---` line ("an issue
// file must start with a --- line") and when the frontmatter is never closed.
func Split(path string, data []byte, noun string) (front string, body string, err error) {
	opening, rest, isLine := strings.Cut(string(data), "\n")
	if !isLine || !rawfm.IsFrontmatterFence(opening) {
		return "", "", &Error{
			Path:    path,
			Line:    openingFenceLine,
			Message: noun + " file must start with a --- line",
		}
	}
	front, body, isClosed := splitAtFence(rest)
	if !isClosed {
		return "", "", &Error{
			Path:    path,
			Line:    openingFenceLine,
			Message: "the frontmatter opened here is never closed by a --- line",
		}
	}
	return crlfToLF(front), crlfToLF(body), nil
}

// splitAtFence splits rest, the raw content after the opening fence's line, at its first line that
// is a fence by rawfm.IsFrontmatterFence, the one predicate rawfm's own readers use. isClosed is
// false when there is none.
func splitAtFence(rest string) (front string, body string, isClosed bool) {
	offset := 0
	for {
		line, _, hasEnd := strings.Cut(rest[offset:], "\n")
		if rawfm.IsFrontmatterFence(line) {
			if !hasEnd {
				return rest[:offset], "", true
			}
			return rest[:offset], rest[offset+len(line)+1:], true
		}
		if !hasEnd {
			return "", "", false
		}
		offset += len(line) + 1
	}
}

// crlfToLF returns text with each CRLF line ending read as LF.
func crlfToLF(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}
