// Package markdown renders the markdown subset capcom's issue and Note bodies use into styled
// terminal lines: # headings, ``` fenced code blocks ```, bullet and numbered lists, GFM pipe
// tables, `inline code`, **bold** and <u>underline</u>. It is a spare, lipgloss-only renderer with
// no syntax highlighting, ported from apogee's TUI.
//
// Every width is a display width in terminal cells, taken in the Theme's Measure: ANSI escapes
// count for nothing, wide glyphs count for two, and the measure follows the painter (see
// Measure). The styling is baked into the text as ANSI before wrapping, and the wrap is SGR-aware,
// so it re-emits a style across a soft-wrap boundary. Unterminated markup (a ` or ** or <u> with
// no closing partner, a fence with no closing fence) degrades to literal text.
//
// Files: markdown.go (Render and the block walk), table.go (pipe tables), wrap.go (wrapping
// primitives), width.go (Measure), theme.go (Theme and glyphs).
package markdown

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// codeBlockIndent leads every line of a fenced code block.
const codeBlockIndent = "  "

// Render renders markdown text into styled physical lines, none wider than width cells in
// th.Measure (a width below 1 is taken as 1). The one line that may exceed it is a single
// grapheme wider than the whole width, which no break can divide.
//
// It walks the text block by block: fenced code blocks, ATX headings, pipe tables, bullet and
// numbered list items, then plain paragraphs. The table matcher is offered a line before the list
// matcher, which would otherwise read a delimiter row written without outer pipes ("- | -") as a
// bullet; a block that is no table, or one the width cannot fit, falls through to the paragraph
// path. Runs of two or more blank lines outside code blocks are collapsed to one first. Empty text
// yields one empty line.
func Render(th Theme, text string, width int) []string {
	if width < 1 {
		width = 1
	}
	lines := collapseBlankRuns(strings.Split(text, "\n"))
	var out []string
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case isFence(line):
			body, end := fenceBody(lines, i)
			out = append(out, renderCodeBlock(th, body, width)...)
			i = end
		case isHeading(line):
			out = append(out, renderHeadingLine(th, headingText(line), width)...)
		default:
			if rendered, span, ok := renderTableAt(th, lines, i, width); ok {
				out = append(out, rendered...)
				i += span - 1 // the loop's own i++ steps past the block's last line
				continue
			}
			if item, ok := matchListItem(line); ok {
				out = append(out, renderListItem(th, item, width)...)
				continue
			}
			out = append(out, renderParagraphLine(th, line, width)...)
		}
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

// fenceBody returns the body of the fenced code block opening at lines[start], and the index of
// its closing fence, or of its last line when the fence is never closed. The fence lines
// themselves are not part of the body.
func fenceBody(lines []string, start int) ([]string, int) {
	var body []string
	end := start + 1
	for ; end < len(lines) && !isFence(lines[end]); end++ {
		body = append(body, lines[end])
	}
	if end == len(lines) {
		end-- // no closing fence to skip
	}
	return body, end
}

// renderTableAt renders the pipe table opening at lines[start], reporting how many source lines
// it spans; ok is false when no table opens there or the width cannot fit it.
func renderTableAt(th Theme, lines []string, start, width int) (rendered []string, span int, ok bool) {
	table, span, ok := matchTableBlock(lines, start)
	if !ok {
		return nil, 0, false
	}
	rendered, fits := renderTable(th, table, width)
	if !fits {
		return nil, 0, false
	}
	return rendered, span, true
}

// collapseBlankRuns returns lines with every run of two or more blank lines reduced to the first
// of them. Fenced code blocks are exempt: a blank line inside one is part of the code and
// survives, so the walk toggles on each fence line and copies the interior untouched.
func collapseBlankRuns(lines []string) []string {
	out := make([]string, 0, len(lines))
	isInFence, isPreviousBlank := false, false
	for _, line := range lines {
		switch {
		case isFence(line):
			isInFence = !isInFence
			isPreviousBlank = false
		case isInFence:
			// Code: copied as is, and it never arms the collapse for the line after the fence.
		case blankLine(line):
			if isPreviousBlank {
				continue
			}
			isPreviousBlank = true
		default:
			isPreviousBlank = false
		}
		out = append(out, line)
	}
	return out
}

// renderParagraphLine styles one ordinary line's inline spans and word-wraps it to width,
// keeping the line's own break. A blank line stays one blank line.
func renderParagraphLine(th Theme, line string, width int) []string {
	return wrapText(th, renderInline(th, line), width)
}

// renderHeadingLine renders a heading's text in the Heading style, word-wrapped to width. Inline
// markup inside a heading is not parsed: a nested span's reset would cut the heading's own style
// short.
func renderHeadingLine(th Theme, text string, width int) []string {
	wrapped := wrapText(th, text, width)
	out := make([]string, len(wrapped))
	for i, segment := range wrapped {
		out[i] = th.Heading.Render(segment)
	}
	return out
}

// renderListItem renders a list item: its marker (the bullet glyph or the source number) and its
// inline-styled text, word-wrapped with a hanging indent so continuation lines align under the
// text.
func renderListItem(th Theme, item listItem, width int) []string {
	return hangingPrefixes(th, item.indent+item.marker, renderInline(th, item.text), width)
}

// renderCodeBlock renders a fenced code block's body: each source line indented two columns,
// styled CodeBlock, and hard-wrapped rather than reflowed so the code's own line structure
// survives. A blank source line stays blank.
//
// Tabs are expanded before the wrap measures them (expandTabs): the style would otherwise turn
// each tab into four spaces after the wrap counted it as nothing, and a tab-indented line would
// paint wider than the width it was wrapped to.
func renderCodeBlock(th Theme, code []string, width int) []string {
	codeWidth := max(1, width-len(codeBlockIndent))
	var out []string
	for _, line := range code {
		wrapped := th.Measure.Hardwrap(expandTabs(line), codeWidth, true)
		for _, segment := range strings.Split(wrapped, "\n") {
			out = append(out, codeBlockIndent+th.CodeBlock.Render(segment))
		}
	}
	return out
}

// renderInline styles the inline spans in one logical line: `code`, **bold** and
// <u>underline</u>. Code spans win over both, so no bold and no <u> is parsed inside one, as in
// CommonMark. An unterminated ` or ** or <u> is emitted literally.
//
// <u> is the one HTML tag the renderer knows, since markdown has no underline of its own, and it
// is matched as exact lowercase bytes: <U>, <u > and every other tag stay literal.
func renderInline(th Theme, s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if styled, next, ok := matchInlineSpan(th, s, i); ok {
			b.WriteString(styled)
			i = next
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// inlineSpan is one kind of inline markup: its opening and closing delimiters and its style.
type inlineSpan struct {
	open  string
	close string
	style func(Theme) lipgloss.Style
}

// inlineSpans are the inline markups renderInline knows, in precedence order: a code span is
// tried first, so its content is never parsed for the others.
var inlineSpans = []inlineSpan{
	{open: "`", close: "`", style: func(th Theme) lipgloss.Style { return th.Code }},
	{open: "**", close: "**", style: func(th Theme) lipgloss.Style { return th.Bold }},
	{open: "<u>", close: "</u>", style: func(th Theme) lipgloss.Style { return th.Underline }},
}

// matchInlineSpan reports whether a terminated inline span opens at s[i], returning its styled
// text and the index just past its closing delimiter. An opening delimiter with no closing one is
// not a span: the caller then copies it literally, one byte at a time.
func matchInlineSpan(th Theme, s string, i int) (styled string, next int, ok bool) {
	for _, span := range inlineSpans {
		if !strings.HasPrefix(s[i:], span.open) {
			continue
		}
		start := i + len(span.open)
		end := strings.Index(s[start:], span.close)
		if end < 0 {
			return "", 0, false
		}
		return span.style(th).Render(s[start : start+end]), start + end + len(span.close), true
	}
	return "", 0, false
}

// isFence reports whether line opens or closes a fenced code block: ``` (optionally followed by a
// language tag) after any leading spaces.
func isFence(line string) bool {
	return strings.HasPrefix(strings.TrimLeft(line, " "), "```")
}

// isHeading reports whether line is an ATX heading: one to six # then a space.
func isHeading(line string) bool {
	trimmed := strings.TrimLeft(line, " ")
	n := 0
	for n < len(trimmed) && trimmed[n] == '#' {
		n++
	}
	return n >= 1 && n <= 6 && n < len(trimmed) && trimmed[n] == ' '
}

// headingText returns a heading line's text with the # markers and surrounding space stripped.
func headingText(line string) string {
	return strings.TrimSpace(strings.TrimLeft(line, " #"))
}

// listItem is a parsed list line: its leading indentation, the marker to show ("• " for a bullet,
// or the source number with its "." or ")" for an ordered item), and the item's text.
type listItem struct {
	indent string
	marker string
	text   string
}

// matchListItem reports whether line is a bullet (- * +) or numbered (1. or 1)) item and returns
// its marker and text. Leading spaces are kept so nested items keep their indent.
func matchListItem(line string) (listItem, bool) {
	rest := strings.TrimLeft(line, " ")
	indent := strings.Repeat(" ", len(line)-len(rest))
	if len(rest) >= 2 && strings.ContainsRune("-*+", rune(rest[0])) && rest[1] == ' ' {
		return listItem{indent: indent, marker: glyphBullet + " ", text: strings.TrimLeft(rest[2:], " ")}, true
	}
	digits := 0
	for digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '9' {
		digits++
	}
	isNumbered := digits > 0 && digits+1 < len(rest) &&
		(rest[digits] == '.' || rest[digits] == ')') && rest[digits+1] == ' '
	if isNumbered {
		return listItem{indent: indent, marker: rest[:digits+1] + " ", text: strings.TrimLeft(rest[digits+2:], " ")}, true
	}
	return listItem{}, false
}
