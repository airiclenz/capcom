package config

import (
	"bytes"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// realFormatLine matches the start of a line holding a top-level format key, plain, single- or
// double-quoted, blanks allowed before its colon; commentedFormatLine matches a commented-out one
// such as the format-1 template's `# format: 1`; documentStartLine matches a `---` document start
// marker, trailing blanks or a comment allowed.
var (
	realFormatLine      = regexp.MustCompile(`^(?:format|"format"|'format')[ \t]*:`)
	commentedFormatLine = regexp.MustCompile(`^#[ \t]*format:[ \t]*[0-9]+[ \t]*$`)
	documentStartLine   = regexp.MustCompile(`^---(?:[ \t]+(?:#.*)?)?$`)
)

// byteOrderMark is the UTF-8 byte order mark a config.yaml may open with.
const byteOrderMark = "\xef\xbb\xbf"

// RaiseFormat returns config.yaml content that declares at least the given store format, every
// other byte kept, and whether it changed anything. path names the file in every error, as Parse
// does. The content is parsed first: content that fails to parse returns Parse's error, and content
// already at version or above is returned unchanged. A top-level format key, plain or quoted and
// with or without blanks before its colon, has its line replaced by `format: <version>`; a key
// written any other way (inside a flow mapping, indented, as an explicit `?` key, with its value on
// a later line or as a literal or folded block scalar) fails with an *Error at the key's line and
// nothing is rewritten. Without a key, a commented-out one inside the document is replaced, and the
// format-1 template's comment line directly above it becomes the current template's. Content with
// neither gains `format: <version>` as a line of its own: directly after the `---` line that opens
// the document, when one does, otherwise as the first line. A leading UTF-8 byte order mark stays
// the first bytes. The key is never written twice, nor outside the document.
func RaiseFormat(path string, data []byte, version int) ([]byte, bool, error) {
	parsed, root, err := parse(path, data)
	if err != nil {
		return nil, false, err
	}
	if parsed.Format >= version {
		return data, false, nil
	}

	body, hasByteOrderMark := bytes.CutPrefix(data, []byte(byteOrderMark))
	raised, err := raiseLines(path, bytes.SplitAfter(body, []byte("\n")), root, version)
	if err != nil {
		return nil, false, err
	}
	if hasByteOrderMark {
		raised = append([]byte(byteOrderMark), raised...)
	}
	return raised, true, nil
}

// raiseLines returns lines joined with the format key raised to version, as RaiseFormat describes;
// root is their parsed top-level mapping, nil for an empty document.
func raiseLines(path string, lines [][]byte, root *yaml.Node, version int) ([]byte, error) {
	formatKey := fmt.Sprintf("%s: %d", keyFormat, version)
	if root != nil {
		if key, value := formatEntry(root); key != nil {
			if !isPlaceable(lines, root, key, value) {
				return nil, &Error{Path: path, Line: key.Line, Message: unplaceableFormatMessage}
			}
			index := key.Line - 1
			lines[index] = []byte(formatKey + lineEnding(lines[index], ""))
			return bytes.Join(lines, nil), nil
		}
	}

	start := documentStart(lines)
	index := findCommentedFormatLine(lines, start+1)
	if index < 0 {
		return insertLine(lines, start+1, formatKey), nil
	}
	lines[index] = []byte(formatKey + lineEnding(lines[index], ""))
	if index > 0 && string(trimLineEnding(lines[index-1])) == keylessFormatComment {
		lines[index-1] = []byte(formatComment + lineEnding(lines[index-1], ""))
	}
	return bytes.Join(lines, nil), nil
}

// unplaceableFormatMessage is the failure of a format key RaiseFormat cannot rewrite in place.
const unplaceableFormatMessage = "format: cannot raise the store format; " +
	"write the key as `format: <number>` at the start of a line of its own"

// isPlaceable reports whether the format key and value of root sit together on one line of lines
// that holds nothing else, so that replacing that line rewrites the key and nothing more: root is
// a block mapping, the key starts its line and is spelled as realFormatLine matches, and the value
// follows on the same line, not as a literal or folded block scalar whose text sits below it.
func isPlaceable(lines [][]byte, root *yaml.Node, key *yaml.Node, value *yaml.Node) bool {
	index := key.Line - 1
	return root.Style&yaml.FlowStyle == 0 &&
		key.Column == 1 &&
		value.Line == key.Line &&
		value.Style&(yaml.LiteralStyle|yaml.FoldedStyle) == 0 &&
		index >= 0 && index < len(lines) &&
		realFormatLine.Match(lines[index])
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

// findCommentedFormatLine returns the index of the first commented-out format line from the line
// index from on, or -1 without one.
func findCommentedFormatLine(lines [][]byte, from int) int {
	for index := max(from, 0); index < len(lines); index++ {
		if commentedFormatLine.Match(trimLineEnding(lines[index])) {
			return index
		}
	}
	return -1
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
