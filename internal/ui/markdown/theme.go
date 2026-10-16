package markdown

import (
	"charm.land/lipgloss/v2"
)

// Glyphs the renderer draws. Each is one cell wide under both width methods, which is what lets
// the table layout treat its divider as a constant (tableDividerWidth).
const (
	// glyphBullet leads a bullet-list item (- * +).
	glyphBullet = "•"
	// glyphTableRule is one cell of a table's horizontal rule, under the header and between
	// adjacent body rows alike.
	glyphTableRule = "─"
	// glyphTableColumn is U+2502 LIGHT VERTICAL, the rule between two table columns.
	glyphTableColumn = "│"
	// glyphTableCross is U+253C LIGHT VERTICAL AND HORIZONTAL, where a horizontal rule crosses a
	// column divider.
	glyphTableCross = "┼"
	// glyphQuoteBar is U+2502 LIGHT VERTICAL, the bar a blockquote shows per nesting level.
	glyphQuoteBar = "│"
)

// Theme is what Render paints with: the width measure every line is wrapped and padded in, and
// one style per markdown role. The zero Theme is valid: it measures in the painter's starting
// method (see Measure) and renders every role unstyled.
type Theme struct {
	// Measure is the display-width measure; it must be the one the painter is using, so a caller
	// folds every tea.ModeReportMsg into it with Measure.Observe.
	Measure Measure
	// Bold paints **bold** spans and a table's header cells.
	Bold lipgloss.Style
	// Underline paints <u>underline</u> spans.
	Underline lipgloss.Style
	// Heading paints an ATX heading's text.
	Heading lipgloss.Style
	// Code paints `inline code` spans.
	Code lipgloss.Style
	// CodeBlock paints each line of a fenced code block.
	CodeBlock lipgloss.Style
	// Rule paints a table's dividers and horizontal rules: the frame, which must not read as
	// loudly as the cells it separates.
	Rule lipgloss.Style
	// Link paints a followable link: a markdown link or inline-code span naming an issue or Note
	// id.
	Link lipgloss.Style
	// Inert paints a link that cannot be followed: a URL, a file path or any other non-id target.
	Inert lipgloss.Style
	// Quote paints a blockquote's bars; the quoted text keeps its own styles.
	Quote lipgloss.Style
}
