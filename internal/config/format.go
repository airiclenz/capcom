package config

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
)

// realFormatLine matches a top-level format key and captures its value; commentedFormatLine
// matches a commented-out one such as the format-1 template's `# format: 1`; documentStartLine
// matches a `---` document start marker, trailing blanks or a comment allowed.
var (
	realFormatLine      = regexp.MustCompile(`^format:[ \t]*([^#\s]*)`)
	commentedFormatLine = regexp.MustCompile(`^#[ \t]*format:[ \t]*[0-9]+[ \t]*$`)
	documentStartLine   = regexp.MustCompile(`^---(?:[ \t]+(?:#.*)?)?$`)
)

// RaiseFormat returns config.yaml content that declares at least the given store format, every
// other byte kept, and whether it changed anything. A top-level format line is replaced by
// `format: <version>`; failing that, a commented-out one inside the document is, and the format-1
// template's comment line directly above it becomes the current template's. Content with neither
// gains `format: <version>` as a line of its own: directly after the `---` line that opens the
// document, when one does, otherwise as the first line. The key is never written twice, nor
// outside the document. Content already at version or above is returned unchanged.
func RaiseFormat(data []byte, version int) ([]byte, bool) {
	lines := bytes.SplitAfter(data, []byte("\n"))
	start := documentStart(lines)
	index, declared := findFormatLine(lines, start+1)
	if declared >= version {
		return data, false
	}

	formatKey := fmt.Sprintf("%s: %d", keyFormat, version)
	if index < 0 {
		return insertLine(lines, start+1, formatKey), true
	}
	isCommented := commentedFormatLine.Match(trimLineEnding(lines[index]))
	lines[index] = []byte(formatKey + lineEnding(lines[index], ""))
	if isCommented && index > 0 && string(trimLineEnding(lines[index-1])) == keylessFormatComment {
		lines[index-1] = []byte(formatComment + lineEnding(lines[index-1], ""))
	}
	return bytes.Join(lines, nil), true
}

// documentStart returns the index of the `---` line that opens the document, or -1 when the
// content does not open with one: only blank lines, comments and `%` directives may precede it.
func documentStart(lines [][]byte) int {
	for index, line := range lines {
		content := trimLineEnding(line)
		trimmed := bytes.TrimLeft(content, " \t")
		switch {
		case documentStartLine.Match(content):
			return index
		case len(trimmed) == 0, trimmed[0] == '#', content[0] == '%':
			continue
		default:
			return -1
		}
	}
	return -1
}

// insertLine returns lines joined with text inserted as a line of its own before lines[at]. The
// new line ends like the line above it (the first line, when at is 0), with LF failing that; a
// last line without an ending gains one so the new line stays separate.
func insertLine(lines [][]byte, at int, text string) []byte {
	above := lines[max(at-1, 0)]
	ending := lineEnding(above, "\n")
	var joined bytes.Buffer
	for _, line := range lines[:at] {
		joined.Write(line)
	}
	if at > 0 && lineEnding(above, "") == "" {
		joined.WriteString(ending)
	}
	joined.WriteString(text + ending)
	for _, line := range lines[at:] {
		joined.Write(line)
	}
	return joined.Bytes()
}

// findFormatLine returns the index of the line holding the top-level format key and the format it
// declares, or, without one, the first commented-out format line from the line index from on and
// MissingFormat. With neither the index is -1. A value that is not a whole number declares format
// 0, below every version.
func findFormatLine(lines [][]byte, from int) (index int, declared int) {
	commented := -1
	for lineIndex, line := range lines {
		content := trimLineEnding(line)
		if match := realFormatLine.FindSubmatch(content); match != nil {
			version, err := strconv.Atoi(string(match[1]))
			if err != nil {
				version = 0
			}
			return lineIndex, version
		}
		if commented < 0 && lineIndex >= from && commentedFormatLine.Match(content) {
			commented = lineIndex
		}
	}
	return commented, MissingFormat
}

// trimLineEnding returns line without its trailing LF or CRLF.
func trimLineEnding(line []byte) []byte {
	return bytes.TrimSuffix(bytes.TrimSuffix(line, []byte("\n")), []byte("\r"))
}

// lineEnding returns the LF or CRLF line ends with, or fallback when it has none.
func lineEnding(line []byte, fallback string) string {
	switch {
	case bytes.HasSuffix(line, []byte("\r\n")):
		return "\r\n"
	case bytes.HasSuffix(line, []byte("\n")):
		return "\n"
	default:
		return fallback
	}
}
