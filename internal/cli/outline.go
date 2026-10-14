package cli

import (
	"fmt"
	"slices"
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

// visibleTree is a hierarchy of entries grouped under their parents, with the visibility rule of
// the command that prints it. Grouping covers every entry, hidden ones included, so a hidden
// entry's children stay in its group and are hidden with it, never promoted to the top level.
type visibleTree[E any] struct {
	children map[string][]E
	idOf     func(E) string
	isShown  func(E) bool
}

// newVisibleTree groups entries by the id parentOf gives, under "" for the top level. An entry
// whose parent is not among entries (its file unusable or left out) joins the top level. Each
// group is in the order compare gives, or in the order of entries when compare is nil. isShown
// decides which entries the outline prints; nil shows every entry.
func newVisibleTree[E any](
	entries []E,
	idOf func(E) string,
	parentOf func(E) string,
	compare func(left E, right E) int,
	isShown func(E) bool,
) *visibleTree[E] {
	isListed := make(map[string]bool, len(entries))
	for _, entry := range entries {
		isListed[idOf(entry)] = true
	}
	children := map[string][]E{}
	for _, entry := range entries {
		parentID := parentOf(entry)
		if !isListed[parentID] {
			parentID = ""
		}
		children[parentID] = append(children[parentID], entry)
	}
	if compare != nil {
		for _, group := range children {
			slices.SortStableFunc(group, compare)
		}
	}
	return &visibleTree[E]{children: children, idOf: idOf, isShown: isShown}
}

// all returns every direct child of parentID, hidden ones included, in group order.
func (t *visibleTree[E]) all(parentID string) []E {
	return t.children[parentID]
}

// shown returns the direct children of parentID the outline prints, in group order.
func (t *visibleTree[E]) shown(parentID string) []E {
	if t.isShown == nil {
		return t.children[parentID]
	}
	var shown []E
	for _, child := range t.children[parentID] {
		if t.isShown(child) {
			shown = append(shown, child)
		}
	}
	return shown
}

// walk adds the outline to add, cutting it maxDepth levels below its start (unlimitedDepth for
// none), as walkOutline does. With a nil start the outline is every shown top-level entry and its
// shown subtree. Otherwise start comes first at indent 0, cutting nothing, whether or not it is
// shown, then its shown subtree one indent deeper.
func (t *visibleTree[E]) walk(start *E, maxDepth int, add func(entry E, indent int, more int, moreDepth int)) {
	if start == nil {
		walkOutline(t.shown, t.idOf, "", 1, 0, maxDepth, add)
		return
	}
	add(*start, 0, 0, 0)
	walkOutline(t.shown, t.idOf, t.idOf(*start), 1, 1, maxDepth, add)
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
