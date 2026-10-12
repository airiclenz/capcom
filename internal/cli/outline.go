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
