package markdown

import (
	"strings"
)

// GFM pipe tables. The first half of this file is pure data, with no theme, no width and no ANSI:
// does a table start on this line, how far does it run, and what are its cells? The second half
// lays those cells out as columns. Cell text comes out of the parser as markdown source, because
// the column widths that matter are those of the rendered cells.
//
// Detection is the two-line lookahead GFM describes: a line carrying at least one unescaped pipe,
// immediately followed by a delimiter row of the same cell count. It is prefix and byte-scan work
// with no regex, since the question is asked of every line the renderer walks.

// mdAlign is a table column's horizontal alignment, as named by its delimiter-row cell.
type mdAlign int

const (
	mdAlignLeft   mdAlign = iota // "---" or ":--"
	mdAlignCenter                // ":-:"
	mdAlignRight                 // "--:"
)

// mdTable is one parsed pipe table: its header cells, one alignment per column, and its body rows,
// each normalised to the header's column count, so len(align) == len(header) == len(row).
type mdTable struct {
	header []string
	align  []mdAlign
	rows   [][]string
}

// matchTableBlock reports whether a pipe table begins at lines[start] and, when it does, returns
// the parsed table and the number of source lines it spans (header, delimiter and body). The
// block ends at the first line with no unescaped pipe, a blank line included. A delimiter-shaped
// line with no header above it never opens a table.
func matchTableBlock(lines []string, start int) (mdTable, int, bool) {
	if start < 0 || start+1 >= len(lines) || !hasUnescapedPipe(lines[start]) {
		return mdTable{}, 0, false
	}
	align, ok := parseDelimiterRow(lines[start+1])
	if !ok {
		return mdTable{}, 0, false
	}
	header := splitTableRow(lines[start])
	if len(align) != len(header) {
		return mdTable{}, 0, false
	}

	end := start + 2
	var rows [][]string
	for ; end < len(lines) && hasUnescapedPipe(lines[end]); end++ {
		rows = append(rows, fitRow(splitTableRow(lines[end]), len(header)))
	}
	return mdTable{header: header, align: align, rows: rows}, end - start, true
}

// parseDelimiterRow reports whether line is a table's delimiter row and returns the alignment it
// names for each column. A delimiter row carries nothing but pipes, hyphens, colons and spaces,
// and every one of its cells is a run of hyphens with an optional colon at either end.
func parseDelimiterRow(line string) ([]mdAlign, bool) {
	source := strings.TrimSpace(line)
	if source == "" {
		return nil, false
	}
	if strings.Trim(source, "|-: ") != "" {
		return nil, false
	}

	cells := splitTableRow(source)
	align := make([]mdAlign, 0, len(cells))
	for _, cell := range cells {
		cellAlign, ok := delimiterCellAlign(cell)
		if !ok {
			return nil, false
		}
		align = append(align, cellAlign)
	}
	return align, true
}

// delimiterCellAlign parses one delimiter cell ("-+", ":-+", "-+:" or ":-+:") into the alignment
// it names. Anything else is not a delimiter cell: an empty cell, a bare colon, or a hyphen run
// broken by anything at all, which keeps prose that contains a dash out of the table path.
func delimiterCellAlign(cell string) (mdAlign, bool) {
	isLeading := strings.HasPrefix(cell, ":")
	isTrailing := len(cell) > 1 && strings.HasSuffix(cell, ":")
	hyphens := cell
	if isLeading {
		hyphens = hyphens[1:]
	}
	if isTrailing {
		hyphens = hyphens[:len(hyphens)-1]
	}
	if hyphens == "" || strings.Trim(hyphens, "-") != "" {
		return mdAlignLeft, false
	}

	switch {
	case isLeading && isTrailing:
		return mdAlignCenter, true
	case isTrailing:
		return mdAlignRight, true
	default:
		return mdAlignLeft, true
	}
}

// splitTableRow splits one table row into its cells on unescaped pipes. The row's optional
// leading and trailing pipes are dropped, "\|" becomes a literal pipe inside its cell, and every
// cell is trimmed of the spaces that padded the source.
func splitTableRow(line string) []string {
	source := strings.TrimSpace(line)
	var cells []string
	var cell strings.Builder
	isEndOnPipe := false
	for i := 0; i < len(source); i++ {
		switch {
		case source[i] == '\\' && i+1 < len(source) && source[i+1] == '|':
			cell.WriteByte('|')
			i++
			isEndOnPipe = false
		case source[i] == '|':
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
			isEndOnPipe = i == len(source)-1
		default:
			cell.WriteByte(source[i])
			isEndOnPipe = false
		}
	}
	cells = append(cells, strings.TrimSpace(cell.String()))

	// The pipes on either end of a row are decoration, not an empty first or last cell.
	if len(cells) > 1 && strings.HasPrefix(source, "|") {
		cells = cells[1:]
	}
	if len(cells) > 1 && isEndOnPipe {
		cells = cells[:len(cells)-1]
	}
	return cells
}

// fitRow normalises one body row to the table's column count: a short row is padded with empty
// cells and a long one loses its excess, so a ragged row never reshapes the table.
func fitRow(cells []string, columns int) []string {
	if len(cells) > columns {
		return cells[:columns]
	}
	for len(cells) < columns {
		cells = append(cells, "")
	}
	return cells
}

// hasUnescapedPipe reports whether s carries at least one pipe not escaped as "\|": the one test
// that both opens a table block and, failing, closes it.
func hasUnescapedPipe(s string) bool {
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s) && s[i+1] == '|':
			i++
		case s[i] == '|':
			return true
		}
	}
	return false
}

// A table is drawn as ruled columns: a │ between adjacent columns with one space either side, a
// rule under the header and another between each pair of adjacent body rows, every rule crossing
// every divider at a ┼, and nothing else (no outer frame, no rule above the first body row or
// below the last). The header is told from the body by its bold cells. Every width is a display
// width in th.Measure over the rendered cell, so markup characters and escape bytes never push a
// column open. Nothing is cut: a cell too wide for its column wraps inside it, so a row is as many
// lines as its tallest cell needs, and every line is padded to the table's full width so the
// block shows one straight right edge.

// tableDivider separates two adjacent columns; tableDividerWidth is its display width, a constant
// because the glyph is one cell under both width methods.
const (
	tableDivider      = " " + glyphTableColumn + " "
	tableDividerWidth = 3
)

// minTableColumnWidth is the narrowest a column may be squeezed to and still read as a column.
// Below it a wrapped cell comes apart into a letter or two per line, and the plain paragraphs the
// block falls back to read better. It is a floor on the shrink, not a width every column is
// given: a naturally narrower column keeps its own width.
const minTableColumnWidth = 4

// renderTable lays a parsed table out as styled lines at width: the header, a rule, then the body
// rows with a rule between each adjacent pair, each cell inline-rendered, wrapped to its column
// and padded on the side its alignment names. It reports false when the width cannot give every
// column minTableColumnWidth cells once the dividers are paid for; the caller then renders the
// block as paragraphs.
func renderTable(th Theme, table mdTable, width int) ([]string, bool) {
	if len(table.header) == 0 {
		return nil, false
	}

	// Tabs are settled before anything measures a cell (expandTabs): an unstyled body cell would
	// otherwise keep its tab through the column measure and paint four cells wider than counted.
	header := make([]string, len(table.header))
	for i, cell := range table.header {
		header[i] = th.Bold.Render(renderInline(th, expandTabs(cell)))
	}
	rows := make([][]string, len(table.rows))
	for i, row := range table.rows {
		cells := make([]string, len(row))
		for j, cell := range row {
			cells[j] = renderInline(th, expandTabs(cell))
		}
		rows[i] = cells
	}

	widths := tableColumnWidths(th, header, rows)
	if !fitColumns(widths, width-tableDividerWidth*(len(widths)-1), minTableColumnWidth) {
		return nil, false
	}

	out := make([]string, 0, 2*len(rows)+2)
	out = append(out, layoutTableRows(th, header, widths, table.align)...)
	out = append(out, tableRuleRow(th, widths))
	for i, row := range rows {
		if i > 0 {
			out = append(out, tableRuleRow(th, widths))
		}
		out = append(out, layoutTableRows(th, row, widths, table.align)...)
	}
	return out, true
}

// tableColumnWidths measures each column's natural width: the widest rendered cell in it, header
// included, floored at one cell so a column of empty cells still holds a place between its
// dividers.
func tableColumnWidths(th Theme, header []string, rows [][]string) []int {
	widths := make([]int, len(header))
	for i, cell := range header {
		widths[i] = max(1, th.Measure.Width(cell))
	}
	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				break
			}
			widths[i] = max(widths[i], th.Measure.Width(cell))
		}
	}
	return widths
}

// fitColumns shrinks widths in place until they sum to no more than budget, the width left once
// the dividers are paid for, and reports whether they fit at all. Space is taken from the widest
// column and, where several are equally wide, from the leftmost, so a table lays out the same way
// on every repaint. It steps a whole level at a time rather than a cell at a time.
//
// No column is taken below floor, and the width the table requires is the sum of min(natural,
// floor), not one floor per column: a column already narrower than the floor is never widened to
// it, so it is not charged it either.
func fitColumns(widths []int, budget, floor int) bool {
	total, required := 0, 0
	for _, w := range widths {
		total += w
		required += min(w, floor)
	}
	if required > budget {
		return false
	}

	for total > budget {
		top, next, count := shrinkLevel(widths, floor)
		if count == 0 {
			// Unreachable while required <= budget holds; it stays as the loop's termination
			// guarantee.
			return false
		}
		// Never take more than the overflow, nor more than the level allows. The last cells
		// come off the group left to right.
		take := min(total-budget, count*(top-next))
		each, extra := take/count, take%count
		for i, w := range widths {
			if w != top {
				continue
			}
			cut := each
			if extra > 0 {
				cut++
				extra--
			}
			widths[i] -= cut
		}
		total -= take
	}
	return true
}

// shrinkLevel finds, among the columns that can still give a cell up (those wider than floor),
// the widest width top, how many columns share it, and the width next just below it, floored at
// floor: the level one pass of fitColumns brings that group down to. count is 0 when no column
// can shrink.
func shrinkLevel(widths []int, floor int) (top, next, count int) {
	next = floor
	for _, w := range widths {
		switch {
		case w <= floor:
		case w > top:
			top, next, count = w, max(next, top), 1
		case w == top:
			count++
		case w > next:
			next = w
		}
	}
	return top, next, count
}

// wrapTableCell breaks one rendered cell into the lines its column holds. A cell that already
// fits is handed back as is; anything wider goes through wrapText, which carries an SGR run across
// a break and enforces the width cap.
func wrapTableCell(th Theme, cell string, width int) []string {
	if th.Measure.Width(cell) <= width {
		return []string{cell}
	}
	return wrapText(th, cell, width)
}

// layoutTableRows lays one row's rendered cells into the fitted column widths and returns the
// lines it occupies: every cell wrapped to its column, the row as tall as its tallest cell, and
// shorter cells blank-filled below their content. The columns are joined by the divider in the
// Rule style, and every column, the last included, is padded, so every line is the table's width.
func layoutTableRows(th Theme, cells []string, widths []int, align []mdAlign) []string {
	wrapped := make([][]string, len(widths))
	height := 1
	for i, w := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		wrapped[i] = wrapTableCell(th, cell, w)
		height = max(height, len(wrapped[i]))
	}

	divider := th.Rule.Render(tableDivider)
	out := make([]string, height)
	for line := range height {
		var b strings.Builder
		for i, w := range widths {
			if i > 0 {
				b.WriteString(divider)
			}
			if line >= len(wrapped[i]) {
				b.WriteString(strings.Repeat(" ", w))
				continue
			}
			cellAlign := mdAlignLeft
			if i < len(align) {
				cellAlign = align[i]
			}
			b.WriteString(padTableCell(th, wrapped[i][line], w, cellAlign))
		}
		out[line] = b.String()
	}
	return out
}

// padTableCell pads one wrapped line of a cell out to its column on the side its alignment names:
// right for a left-aligned cell, left for a right-aligned one, and split for a centred one with
// the odd cell on the right. It never cuts; a single grapheme wider than the column keeps its
// line.
func padTableCell(th Theme, cell string, width int, align mdAlign) string {
	pad := width - th.Measure.Width(cell)
	if pad <= 0 {
		return cell
	}
	switch align {
	case mdAlignRight:
		return strings.Repeat(" ", pad) + cell
	case mdAlignCenter:
		left := pad / 2
		return strings.Repeat(" ", left) + cell + strings.Repeat(" ", pad-left)
	default:
		return cell + strings.Repeat(" ", pad)
	}
}

// tableRuleRow renders one horizontal rule in the Rule style: a ─ run per column joined by ─┼─
// where a divider passes through, so it is exactly as wide as every other line of the table and
// each crossing sits in the cell the divider occupies on the rows above and below.
func tableRuleRow(th Theme, widths []int) string {
	columns := make([]string, len(widths))
	for i, w := range widths {
		columns[i] = strings.Repeat(glyphTableRule, w)
	}
	crossing := glyphTableRule + glyphTableCross + glyphTableRule
	return th.Rule.Render(strings.Join(columns, crossing))
}
