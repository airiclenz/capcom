package ui

import (
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/airiclenz/capcom/internal/ui/markdown"
)

// contentCell is a position in the detail pane's rendered lines: line indexes the lines and col is
// the display column within that line, in the painter's measure. A selection anchors in these
// content coordinates rather than screen ones so it survives a scroll mid-drag: the scroll moves
// what is on screen, not the line the anchor names.
type contentCell struct{ line, col int }

// detailSel is a drag selection over the detail pane's rendered lines, ported from apogee's
// transcript selection. anchor is the drag's fixed end, set on the press; head is its moving end,
// following the drag. The zero value selects nothing.
//
// The copied text is cut from the rendered lines, not from the markdown source: what is selected
// is what is on screen. Every column is measured and cut through markdown.Measure, the painter's
// measure (ADR 0030), so the cells shaded are the cells the pointer ran over and the cells copied.
type detailSel struct {
	isActive     bool
	anchor, head contentCell
}

// isCollapsed reports whether the selection spans no cell: a press not yet dragged. It paints
// nothing and copies nothing.
func (s detailSel) isCollapsed() bool {
	return s.anchor == s.head
}

// ordered returns the selection's two ends in reading order: top before bottom, on an earlier line
// or further left on the same one. A drag upward or leftward selects what the reverse drag does.
func (s detailSel) ordered() (top, bottom contentCell) {
	top, bottom = s.anchor, s.head
	if bottom.line < top.line || (bottom.line == top.line && bottom.col < top.col) {
		top, bottom = bottom, top
	}
	return top, bottom
}

// spanUnchanged reports whether the selection's ground held still across a re-render: every line
// it spans is the same in oldLines and newLines. True keeps the selection, false drops it, so a
// highlight never stands over text that has since been rewritten.
//
// An inactive selection has nothing to keep and reports false. A collapsed one reports true
// whatever changed: it shades nothing, so nothing under it can go stale, and it is the press a
// release still needs to read. A span reaching past either slice reports false. Only the line
// range is checked: identical lines have identical widths, so a column that was valid still is.
func (s detailSel) spanUnchanged(oldLines, newLines []string) bool {
	if !s.isActive {
		return false
	}
	if s.isCollapsed() {
		return true
	}
	top, bottom := s.ordered()
	if top.line < 0 || bottom.line >= len(oldLines) || bottom.line >= len(newLines) {
		return false
	}
	return slices.Equal(oldLines[top.line:bottom.line+1], newLines[top.line:bottom.line+1])
}

// columns returns the display columns [c0, c1) the selection covers on line, one of its spanned
// lines, in a pane width columns wide: the first spanned line from the top's column, the last to
// the bottom's, and every line between them whole.
func (s detailSel) columns(line, width int) (c0, c1 int) {
	top, bottom := s.ordered()
	c0, c1 = 0, width
	if line == top.line {
		c0 = top.col
	}
	if line == bottom.line {
		c1 = bottom.col
	}
	return c0, c1
}

// paint returns a copy of lines with the selection shaded in style on the rows the pane shows: the
// rows lines[offset:offset+rows] of a pane width columns wide. The copy is as long as lines, so a
// caller still measures the whole for its scrollbar and slices the window from offset itself; lines
// is never changed. An inactive or collapsed selection shades nothing.
func (s detailSel) paint(
	measure markdown.Measure,
	lines []string,
	offset, rows, width int,
	style lipgloss.Style,
) []string {
	painted := slices.Clone(lines)
	if !s.isActive || s.isCollapsed() {
		return painted
	}
	top, bottom := s.ordered()
	first := max(offset, top.line, 0)
	last := min(offset+rows, bottom.line+1, len(lines))
	for line := first; line < last; line++ {
		c0, c1 := s.columns(line, width)
		painted[line] = shade(measure, lines[line], c0, c1, style)
	}
	return painted
}

// text returns the plain text the selection covers in lines, a pane width columns wide: each
// spanned line cut to the columns paint shades, its styling stripped and its trailing spaces
// trimmed, the lines joined with newlines. A line the span covers no cell of copies as a blank
// line. An inactive selection copies nothing.
func (s detailSel) text(measure markdown.Measure, lines []string, width int) string {
	if !s.isActive {
		return ""
	}
	top, bottom := s.ordered()
	copied := make([]string, 0, max(0, bottom.line-top.line+1))
	for line := max(top.line, 0); line <= bottom.line && line < len(lines); line++ {
		c0, c1 := s.columns(line, width)
		cut := ""
		if c0 < c1 {
			cut = ansi.Strip(measure.Cut(lines[line], c0, c1))
		}
		copied = append(copied, strings.TrimRight(cut, " "))
	}
	return strings.Join(copied, "\n")
}

// shade returns line with its display columns [c0, c1) re-rendered in style. The cells either side
// keep their own styling; the shaded span is stripped first so the selection's style wins over the
// one it covers. The span stops at the line's last cell, and a span of no cells of the line returns
// it unchanged.
func shade(measure markdown.Measure, line string, c0, c1 int, style lipgloss.Style) string {
	width := measure.Width(line)
	c1 = min(c1, width)
	if c1 <= c0 {
		return line
	}
	left := measure.Cut(line, 0, c0)
	middle := measure.Cut(line, c0, c1)
	right := measure.Cut(line, c1, width)
	return left + style.Render(ansi.Strip(middle)) + right
}
