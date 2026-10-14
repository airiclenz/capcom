// Package rawfm reads what it can from the raw text of an issue or Note file, before and without
// parsing it: the git conflict markers a merge leaves in the file, the aliases its frontmatter
// lists, for a file that does not parse, and where its body starts and its fenced code blocks lie.
// It imports no capcom package, so every layer that reports
// on a file, the parsers as well as `capcom check`, can use it.
package rawfm

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// The git conflict markers. A merge writes the first three at the start of a line, followed by a
// label; the separator is a line of its own.
const (
	conflictOpen      = "<<<<<<< "
	conflictClose     = ">>>>>>> "
	conflictBase      = "||||||| "
	conflictSeparator = "======="
)

// ConflictMarker is one git conflict marker line of a file: its 1-based line and its text, without
// the line ending.
type ConflictMarker struct {
	Line int
	Text string
}

// ConflictMarkers returns the git conflict markers in the content of an issue or Note file, in
// line order: every line that starts with "<<<<<<< ", ">>>>>>> " or "||||||| ", and a "=======" line
// only between a "<<<<<<< " line and the ">>>>>>> " line that ends it, since a bare one is a setext
// heading underline. The frontmatter is always scanned; body lines inside a fenced code block are
// skipped. One trailing "\r" is trimmed from each line, so CRLF content gives the same markers.
func ConflictMarkers(data []byte) []ConflictMarker {
	lines := strings.Split(string(data), "\n")
	bodyStart := bodyStartIndex(lines)
	var markers []ConflictMarker
	var separators []ConflictMarker
	isConflictOpen := false
	var fence CodeFence
	for index, line := range lines {
		text := strings.TrimSuffix(line, "\r")
		if index >= bodyStart && fence.IsCode(text) {
			continue
		}
		marker := ConflictMarker{Line: index + 1, Text: text}
		switch {
		case strings.HasPrefix(text, conflictOpen):
			markers = append(markers, marker)
			isConflictOpen = true
			separators = nil
		case strings.HasPrefix(text, conflictClose):
			markers = append(markers, marker)
			if isConflictOpen {
				markers = append(markers, separators...)
			}
			isConflictOpen = false
			separators = nil
		case strings.HasPrefix(text, conflictBase):
			markers = append(markers, marker)
		case isConflictOpen && text == conflictSeparator:
			separators = append(separators, marker)
		}
	}
	slices.SortFunc(markers, func(a, b ConflictMarker) int { return cmp.Compare(a.Line, b.Line) })
	return markers
}

// ConflictMessage returns the words that report marker to the user: the marker's text and what to
// do about it. It names neither the file nor the line, which the caller reports beside it.
func ConflictMessage(marker ConflictMarker) string {
	return fmt.Sprintf("git conflict marker %q; resolve the conflict and delete the markers", marker.Text)
}

// BodyStart returns the index of the first body line of an issue or Note file split into lines,
// each without its LF terminator: the line after the "---" line that closes the frontmatter, 0 for a
// file that opens with no frontmatter, and len(lines) for one whose frontmatter is never closed.
func BodyStart(lines []string) int {
	return bodyStartIndex(lines)
}

// bodyStartIndex returns the index of the first body line of a file split into lines: the line
// after the "---" line that closes the frontmatter, 0 for a file that opens with no frontmatter,
// and len(lines) for one whose frontmatter is never closed, which is then all frontmatter.
func bodyStartIndex(lines []string) int {
	if len(lines) == 0 || !IsFrontmatterFence(lines[0]) {
		return 0
	}
	for index := 1; index < len(lines); index++ {
		if IsFrontmatterFence(lines[index]) {
			return index + 1
		}
	}
	return len(lines)
}
