package ui

import (
	"maps"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/airiclenz/capcom/internal/outline"
)

// The glyphs of a tree row: the arrow of a node whose children are shown, of one whose children
// are hidden, and the dot of a leaf.
const (
	arrowExpanded  = "▼ "
	arrowCollapsed = "▶ "
	arrowLeaf      = "· "
)

// The connector cells that lead a row below the top level, one per level: the tee of a row a
// later visible sibling follows, the elbow of a parent's last visible child, and the cells of an
// ancestor beneath it that continue as a line when a later visible sibling follows that ancestor,
// and are blank when it was the last. Every cell is connectorWidth columns wide, so a row's arrow
// starts at column connectorWidth*depth.
const (
	connectorTee   = "┝ "
	connectorElbow = "┕ "
	connectorLine  = "│ "
	connectorBlank = "  "
	connectorWidth = 2
)

// The keys a tree takes beyond the up and down of its listCursor.
const (
	keyLeft     = "left"
	keyRight    = "right"
	keyLeftVim  = "h"
	keyRightVim = "l"
)

// treeRow is one visible row of a treeWidget: its outline row, whether its children are shown
// beneath it, whether it is a node the outline hides, revealed only because it or a node below
// it is selected, and the unstyled connector cells that lead it, "" at the top level.
type treeRow[E any] struct {
	outline.Row[E]
	isExpanded bool
	isMuted    bool
	connector  string
}

// treeWidget is a collapsible sidebar tree over an outline: the rows of the outline that the
// expanded nodes leave visible, in outline order, and a listCursor over them. It opens collapsed
// to the top level.
//
// A selected node the outline hides (a closed issue, a background Note) is revealed: it and its
// ancestors show as extra rows, the hidden ones muted, for as long as it stays selected. A
// revealed node shows no children of its own. Once the selection moves to a node the outline
// shows, the revealed rows hide again.
//
// It is a plain value the value-copied model holds inline: every change replaces its rows and
// its expanded set rather than editing them, so an older copy never sees a newer one's state.
type treeWidget[E any] struct {
	// outline is the tree the widget shows, and idOf names its entries.
	outline *outline.Tree[E]
	idOf    func(E) string
	// parentOf maps every entry's id, hidden ones included, to its parent's in the outline, ""
	// at the top level; isShown holds the ids of the rows the fully expanded outline prints.
	parentOf map[string]string
	isShown  map[string]bool
	// expanded holds the ids of the nodes whose children show, and revealed the selected node
	// the outline hides, "" when the selection is a shown node.
	expanded map[string]bool
	revealed string

	rows   []treeRow[E]
	cursor listCursor
	top    int
}

// newTreeWidget returns the widget of tree, whose entries idOf names, with the nodes of expanded
// that are still in tree expanded and the first row selected.
func newTreeWidget[E any](tree *outline.Tree[E], idOf func(E) string, expanded map[string]bool) treeWidget[E] {
	w := treeWidget[E]{
		outline:  tree,
		idOf:     idOf,
		parentOf: map[string]string{},
		isShown:  map[string]bool{},
		expanded: map[string]bool{},
	}
	w.indexParents("")
	for _, row := range tree.Rows(nil, outline.UnlimitedDepth) {
		w.isShown[row.ID] = true
	}
	for id := range expanded {
		if _, isInTree := w.parentOf[id]; isInTree {
			w.expanded[id] = true
		}
	}
	w.rebuild("")
	return w
}

// indexParents records the parent of every entry below parentID, hidden ones included.
func (w *treeWidget[E]) indexParents(parentID string) {
	for _, child := range w.outline.Children(parentID) {
		id := w.idOf(child)
		w.parentOf[id] = parentID
		w.indexParents(id)
	}
}

// children returns every direct child of id in the outline, hidden ones included.
func (w treeWidget[E]) children(id string) []E {
	return w.outline.Children(id)
}

// selection returns the selected row, and false when the tree has no rows.
func (w treeWidget[E]) selection() (treeRow[E], bool) {
	row := w.cursor.highlight(len(w.rows))
	if row < 0 {
		return treeRow[E]{}, false
	}
	return w.rows[row], true
}

// selectID selects the node id names: it expands the node's ancestors and, when the outline hides
// the node, reveals it. It reports false, changing nothing, when id is not in the tree.
func (w *treeWidget[E]) selectID(id string) bool {
	if _, isInTree := w.parentOf[id]; !isInTree {
		return false
	}
	w.expanded = maps.Clone(w.expanded)
	for parentID := w.parentOf[id]; parentID != ""; parentID = w.parentOf[parentID] {
		w.expanded[parentID] = true
	}
	w.revealed = ""
	if !w.isShown[id] {
		w.revealed = id
	}
	w.rebuild(id)
	return true
}

// key routes one key press through the tree and reports what it did: up and k, down and j move
// the selection, → and l expand the selected node, ← and h collapse it or, when it is not
// expanded, move to its parent. Every other key is listUnclaimed.
func (w *treeWidget[E]) key(msg tea.KeyPressMsg) listVerdict {
	switch msg.String() {
	case keyRight, keyRightVim:
		w.expand()
		return listSwallowed
	case keyLeft, keyLeftVim:
		w.collapseOrLeave()
		return listSwallowed
	}
	if w.cursor.key(msg, len(w.rows), listStopsAtEnds) == listUnclaimed {
		return listUnclaimed
	}
	w.settleSelection()
	return listSwallowed
}

// expand shows the selected node's children, when it has hidden ones.
func (w *treeWidget[E]) expand() {
	selected, isSelected := w.selection()
	if !isSelected || selected.isMuted || !selected.HasChildren || w.expanded[selected.ID] {
		return
	}
	w.expanded = maps.Clone(w.expanded)
	w.expanded[selected.ID] = true
	w.rebuild(selected.ID)
}

// collapseOrLeave hides the selected node's children when it is expanded, and otherwise moves
// the selection to its parent, when it has one.
func (w *treeWidget[E]) collapseOrLeave() {
	selected, isSelected := w.selection()
	if !isSelected {
		return
	}
	if w.expanded[selected.ID] && selected.isExpanded {
		w.collapse(selected.ID)
		return
	}
	parentRow := slices.IndexFunc(w.rows, func(row treeRow[E]) bool { return row.ID == selected.ParentID })
	if selected.ParentID == "" || parentRow < 0 {
		return
	}
	w.cursor.seat(parentRow, len(w.rows))
	w.settleSelection()
}

// collapse hides the children of the node id names, keeping the selection on it.
func (w *treeWidget[E]) collapse(id string) {
	w.expanded = maps.Clone(w.expanded)
	delete(w.expanded, id)
	w.rebuild(id)
}

// click selects the row painted at screen row row of the tree, counted from the top of what
// paint last drew, and reports false, changing nothing, when no row is drawn there. A click at
// column col on the arrow of a node with children also expands it, or collapses it when its
// children show.
func (w *treeWidget[E]) click(row, col int) bool {
	position := w.top + row
	if row < 0 || position >= len(w.rows) {
		return false
	}
	clicked := w.rows[position]
	arrowStart := clicked.Depth * connectorWidth
	isOnArrow := clicked.HasChildren && col >= arrowStart && col < arrowStart+ansi.StringWidth(treeArrow(clicked))

	w.cursor.seat(position, len(w.rows))
	w.settleSelection()
	if !isOnArrow {
		return true
	}
	selected, _ := w.selection()
	if w.expanded[selected.ID] && selected.isExpanded {
		w.collapse(selected.ID)
		return true
	}
	w.expand()
	return true
}

// wheel moves the selection one row per wheel notch, stopping at the ends, and applies the
// reveal rule where it lands.
func (w *treeWidget[E]) wheel(msg tea.MouseWheelMsg) {
	w.cursor.wheel(msg, len(w.rows))
	w.settleSelection()
}

// settleSelection applies the reveal rule after the selection moved: a muted row the cursor
// landed on becomes the revealed node, and a shown one ends the reveal. A changed reveal
// rebuilds the rows, keeping the selection on the same node.
func (w *treeWidget[E]) settleSelection() {
	selected, isSelected := w.selection()
	if !isSelected || selected.ID == w.revealed {
		return
	}
	w.revealed = ""
	if selected.isMuted {
		w.revealed = selected.ID
	}
	w.rebuild(selected.ID)
}

// rebuild recomputes the visible rows from the expanded set and the revealed node, and seats the
// selection on selectedID's row when it is visible; otherwise the selection is clamped.
func (w *treeWidget[E]) rebuild(selectedID string) {
	revealPath := map[string]bool{}
	for id := w.revealed; id != ""; id = w.parentOf[id] {
		revealPath[id] = true
	}
	var rows []treeRow[E]
	w.addRows(&rows, revealPath, "", 0, "")
	w.rows = rows
	w.cursor.clampSelection(len(rows))
	if row := slices.IndexFunc(rows, func(row treeRow[E]) bool { return row.ID == selectedID }); row >= 0 {
		w.cursor.seat(row, len(rows))
	}
}

// addRows appends the visible rows below parentID at depth: each child the outline shows or the
// reveal path holds, in outline order, followed by its own visible rows when it is expanded or
// an ancestor of the revealed node. ancestors holds the connector cells of the levels between the
// top level and parentID, which lead every row below depth 0.
func (w *treeWidget[E]) addRows(
	rows *[]treeRow[E],
	revealPath map[string]bool,
	parentID string,
	depth int,
	ancestors string,
) {
	visible := w.visibleChildren(parentID, revealPath)
	for position, child := range visible {
		id := w.idOf(child)
		isLast := position == len(visible)-1
		row := treeRow[E]{
			Row:       outline.Row[E]{Entry: child, ID: id, ParentID: parentID, Depth: depth},
			isMuted:   !w.isShown[id],
			connector: rowConnector(depth, ancestors, isLast),
		}
		row.HasChildren = !row.isMuted && len(w.outline.Shown(id)) > 0
		*rows = append(*rows, row)
		rowPosition := len(*rows) - 1
		isOpen := (row.HasChildren && w.expanded[id]) || (revealPath[id] && id != w.revealed)
		if !isOpen {
			continue
		}
		w.addRows(rows, revealPath, id, depth+1, childAncestors(depth, ancestors, isLast))
		(*rows)[rowPosition].isExpanded = len(*rows) > rowPosition+1
	}
}

// visibleChildren returns the children of parentID that show as rows: those the outline shows
// and those the reveal path holds, in outline order.
func (w *treeWidget[E]) visibleChildren(parentID string, revealPath map[string]bool) []E {
	var visible []E
	for _, child := range w.children(parentID) {
		id := w.idOf(child)
		if w.isShown[id] || revealPath[id] {
			visible = append(visible, child)
		}
	}
	return visible
}

// rowConnector returns the connector cells that lead a row at depth below ancestors: none at the
// top level, otherwise the ancestors' cells and then an elbow when the row is its parent's last
// visible child, a tee when a sibling follows.
func rowConnector(depth int, ancestors string, isLast bool) string {
	switch {
	case depth == 0:
		return ""
	case isLast:
		return ancestors + connectorElbow
	}
	return ancestors + connectorTee
}

// childAncestors returns the ancestor cells the children of a row at depth inherit: none below a
// top-level row, otherwise ancestors and then a line when a sibling follows the row, blanks when
// it was the last.
func childAncestors(depth int, ancestors string, isLast bool) string {
	switch {
	case depth == 0:
		return ""
	case isLast:
		return ancestors + connectorBlank
	}
	return ancestors + connectorLine
}

// paint returns the rows of the tree that fit in height rows, scrolled so the selection stays on
// screen: each led by its connector cells in th's border style, then its arrow and the spans
// rowText gives, the selected one and the muted (revealed) ones marked by th's markOf over the
// arrow and the spans only. A height under one paints nothing and leaves the scroll as it is.
func (w *treeWidget[E]) paint(
	height int,
	th theme,
	rowText func(row outline.Row[E]) []span,
) []string {
	if height < 1 || len(w.rows) == 0 {
		return nil
	}
	selected := w.cursor.highlight(len(w.rows))
	w.top = clampInt(w.top, selected-height+1, selected)
	end := min(w.top+height, len(w.rows))
	painted := make([]string, 0, end-w.top)
	for position := w.top; position < end; position++ {
		row := w.rows[position]
		mark := th.markOf(row.isMuted, position == selected)
		text := mark.render(append([]span{{text: treeArrow(row)}}, rowText(row.Row)...)...)
		if row.connector != "" {
			text = th.border.Render(row.connector) + text
		}
		painted = append(painted, text)
	}
	return painted
}

// treeArrow returns the arrow of row: expanded, collapsed over hidden children, or a leaf's dot.
func treeArrow[E any](row treeRow[E]) string {
	switch {
	case row.isExpanded:
		return arrowExpanded
	case row.HasChildren:
		return arrowCollapsed
	}
	return arrowLeaf
}
