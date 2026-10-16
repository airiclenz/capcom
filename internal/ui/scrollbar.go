package ui

import (
	"strings"

	"github.com/airiclenz/capcom/internal/ui/markdown"
)

// The glyphs of a pane's scrollbar, ported from apogee's: a heavy thumb marking the shown window
// on a light track.
const (
	scrollThumbGlyph = "┃"
	scrollTrackGlyph = "│"
	// scrollBlankCell is the bar's cell while the pane's lines fit and there is nothing to scroll.
	scrollBlankCell = " "
)

// The columns a scrollbar takes at a pane's right edge: a blank gutter between the pane's text and
// the bar, then the bar.
const (
	scrollbarGutter  = 1
	scrollbarBar     = 1
	scrollbarColumns = scrollbarGutter + scrollbarBar
)

// scrollbarThumb returns the thumb's first row and its rows on a bar height rows tall, for a
// window of window lines from start on over total lines: 0, 0 when the window shows every line
// or the bar has no rows. The thumb is at least one row, flush with the top at start 0 and with
// the bottom at the last start; a start past the last one counts as the last.
func scrollbarThumb(start, window, total, height int) (int, int) {
	if height <= 0 || window <= 0 || window >= total {
		return 0, 0
	}
	size := clampInt(height*window/total, 1, height)
	position := clampInt(start, 0, total-window) * (height - size) / (total - window)
	return position, size
}

// scrollbarCells returns the height cells of a scrollbar over total lines shown from offset on,
// one per row: the thumb in th's scrollThumb style over the shown window and the track in its
// scrollTrack style elsewhere, or blank cells when the lines fit in height rows.
func scrollbarCells(th theme, offset, total, height int) []string {
	cells := make([]string, 0, max(height, 0))
	position, size := scrollbarThumb(offset, height, total, height)
	for row := range height {
		switch {
		case size == 0:
			cells = append(cells, scrollBlankCell)
		case row >= position && row < position+size:
			cells = append(cells, th.scrollThumb.Render(scrollThumbGlyph))
		default:
			cells = append(cells, th.scrollTrack.Render(scrollTrackGlyph))
		}
	}
	return cells
}

// scrolledPaneRows returns height rows of lines from offset on, each cut and padded to width
// columns, then the blank gutter and the row's scrollbar cell over all the lines.
func (m model) scrolledPaneRows(lines []string, offset, height, width int) []string {
	cells := scrollbarCells(m.theme, offset, len(lines), height)
	return m.barredRows(lines[clampInt(offset, 0, len(lines)):], cells, width)
}

// barredRows returns one row per cell: the line on that row cut and padded to width columns, then
// the blank gutter and the cell. The rows are squared in the painter's measure, not the layout's,
// so the bar paints as one straight column even on rows holding a grapheme the two measures
// disagree on; blank rows fill the pane below the last line.
func (m model) barredRows(lines, cells []string, width int) []string {
	measure := m.markdownTheme.Measure
	gutter := strings.Repeat(" ", scrollbarGutter)
	rows := make([]string, 0, len(cells))
	for row, cell := range cells {
		line := ""
		if row < len(lines) {
			line = lines[row]
		}
		rows = append(rows, squareRow(measure, line, width)+gutter+cell)
	}
	return rows
}

// squareRow cuts text to width columns of measure, ending a cut line with an ellipsis, and pads
// it with spaces to exactly width columns.
func squareRow(measure markdown.Measure, text string, width int) string {
	width = max(width, 0)
	cut := measure.Truncate(text, width, truncateTail)
	return cut + strings.Repeat(" ", max(width-measure.Width(cut), 0))
}
