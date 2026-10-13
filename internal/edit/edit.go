// Package edit holds the text edits that issue and Note writes share: label merging, appending a
// block to a markdown body and replacing one exact passage of it. Every function is pure; none
// reads or writes a file.
package edit

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrReplaceMatch reports body replacement text that does not occur exactly once in the body.
var ErrReplaceMatch = errors.New("the text to replace must occur exactly once in the body")

// Labels returns a new slice holding labels with each label of add appended unless it is empty or
// already there, then each label of remove taken out. Labels keep their order; a label to remove
// that is not there is ignored. The result is nil when no label is left; labels itself is not
// modified.
func Labels(labels []string, add []string, remove []string) []string {
	merged := slices.Clone(labels)
	for _, label := range add {
		if label != "" && !slices.Contains(merged, label) {
			merged = append(merged, label)
		}
	}
	merged = slices.DeleteFunc(merged, func(label string) bool {
		return slices.Contains(remove, label)
	})
	if len(merged) == 0 {
		return nil
	}
	return merged
}

// AppendBody returns body with text appended as a new block, separated from what is there by a
// blank line, and ending in a line break. text is taken literally; CRLF line endings in it are read
// as LF. An empty body gets the leading blank line a new file has after its frontmatter.
func AppendBody(body string, text string) string {
	return AppendBlock(body, WithFinalNewline(NormalizeNewlines(text)))
}

// ReplaceBody returns body with the one occurrence of old replaced by replacement (CRLF line
// endings in either read as LF). It fails with an error wrapping ErrReplaceMatch when old is empty
// or occurs in body other than exactly once.
func ReplaceBody(body string, old string, replacement string) (string, error) {
	old = NormalizeNewlines(old)
	if old == "" {
		return "", fmt.Errorf("%w; the text to replace is empty", ErrReplaceMatch)
	}
	if count := strings.Count(body, old); count != 1 {
		return "", fmt.Errorf("%w; %q occurs %d times", ErrReplaceMatch, old, count)
	}
	return strings.Replace(body, old, NormalizeNewlines(replacement), 1), nil
}

// AppendBlock appends block to body after a blank line. A blank body becomes a blank line and the
// block, the shape of a body right after the frontmatter. block is appended as given.
func AppendBlock(body string, block string) string {
	if strings.TrimSpace(body) == "" {
		return "\n" + block
	}
	body = WithFinalNewline(body)
	if !strings.HasSuffix(body, "\n\n") {
		body += "\n"
	}
	return body + block
}

// WithFinalNewline returns text ending in a line break; empty text stays empty.
func WithFinalNewline(text string) string {
	if text == "" || strings.HasSuffix(text, "\n") {
		return text
	}
	return text + "\n"
}

// NormalizeNewlines turns CRLF line endings into LF, the line ending stored bodies keep.
func NormalizeNewlines(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}
