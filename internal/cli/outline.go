package cli

import (
	"fmt"
	"strings"
)

// outlineRow is one line of an indented outline: its text at its indentation depth and, when a
// --depth limit cut the entry's children, how many direct children it cut (more) and the --depth
// that would show them (moreDepth).
type outlineRow struct {
	depth     int
	text      string
	more      int
	moreDepth int
}

// walkOutline walks the outline below parentID, whose shown children sit level levels below the
// start of the outline: it calls add for each shown child at indent, in the order shownChildren
// gives, then walks that child's subtree one level and one indent deeper. On the last level
// maxDepth allows (every level when maxDepth is unlimitedDepth) it descends no further: add gets
// the number of the child's shown children it cut (more) and the --depth that would show them
// (moreDepth). Elsewhere add gets 0 for both. The caller adds the start of the outline itself.
func walkOutline[E any](
	shownChildren func(parentID string) []E,
	idOf func(E) string,
	parentID string,
	level int,
	indent int,
	maxDepth int,
	add func(entry E, indent int, more int, moreDepth int),
) {
	isLastLevel := maxDepth != unlimitedDepth && level >= maxDepth
	for _, child := range shownChildren(parentID) {
		if isLastLevel {
			add(child, indent, len(shownChildren(idOf(child))), level+1)
			continue
		}
		add(child, indent, 0, 0)
		walkOutline(shownChildren, idOf, idOf(child), level+1, indent+1, maxDepth, add)
	}
}

// renderOutline renders rows as an indented outline, textIndent per depth, one row per line in
// the order given. Under a row whose children were cut it adds one line, indented one level
// deeper: `... <more> more (--depth <moreDepth>)`. Rows that cut nothing add no line.
func renderOutline(rows []outlineRow) string {
	var text strings.Builder
	for _, row := range rows {
		text.WriteString(strings.Repeat(textIndent, row.depth) + row.text + "\n")
		if row.more > 0 {
			fmt.Fprintf(
				&text,
				"%s... %d more (--%s %d)\n",
				strings.Repeat(textIndent, row.depth+1),
				row.more,
				flagDepth,
				row.moreDepth,
			)
		}
	}
	return text.String()
}
