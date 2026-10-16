package ui

import (
	"maps"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/airiclenz/capcom/internal/outline"
)

// The glyphs of a tree row: the arrow of a node whose children are shown, of one whose children
// are hidden, the blank of a leaf, and the indent of one level.
const (
	arrowExpanded  = "▾ "
	arrowCollapsed = "▸ "
	arrowLeaf      = "  "
	treeIndent     = "  "
)

// The keys a tree takes beyond the up and down of its listCursor.
const (
	keyLeft     = "left"
	keyRight    = "right"
	keyLeftVim  = "h"
	keyRightVim = "l"
)

// mutedRowStyle marks a revealed row, and mutedSelectedRowStyle one that is also selected.
var (
	mutedRowStyle         = lipgloss.NewStyle().Faint(true)
	mutedSelectedRowStyle = lipgloss.NewStyle().Faint(true).Reverse(true)
)

// treeRow is one visible row of a treeWidget: its outline row, whether its children are shown
// beneath it, and whether it is a node the outline hides, revealed only because it or a node
// below it is selected.
type treeRow[E any] struct {
	outline.Row[E]
	isExpanded bool
	isMuted    bool
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
		w.expanded = maps.Clone(w.expanded)
		delete(w.expanded, selected.ID)
		w.rebuild(selected.ID)
		return
	}
	parentRow := slices.IndexFunc(w.rows, func(row treeRow[E]) bool { return row.ID == selected.ParentID })
	if selected.ParentID == "" || parentRow < 0 {
		return
	}
	w.cursor.seat(parentRow, len(w.rows))
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
	w.addRows(&rows, revealPath, "", 0)
	w.rows = rows
	w.cursor.clampSelection(len(rows))
	if row := slices.IndexFunc(rows, func(row treeRow[E]) bool { return row.ID == selectedID }); row >= 0 {
		w.cursor.seat(row, len(rows))
	}
}

// addRows appends the visible rows below parentID at depth: each child the outline shows or the
// reveal path holds, in outline order, followed by its own visible rows when it is expanded or
// an ancestor of the revealed node.
func (w *treeWidget[E]) addRows(rows *[]treeRow[E], revealPath map[string]bool, parentID string, depth int) {
	for _, child := range w.children(parentID) {
		id := w.idOf(child)
		isMuted := !w.isShown[id]
		if isMuted && !revealPath[id] {
			continue
		}
		row := treeRow[E]{
			Row:     outline.Row[E]{Entry: child, ID: id, ParentID: parentID, Depth: depth},
			isMuted: isMuted,
		}
		row.HasChildren = !isMuted && len(w.outline.Shown(id)) > 0
		*rows = append(*rows, row)
		position := len(*rows) - 1
		isOpen := (row.HasChildren && w.expanded[id]) || (revealPath[id] && id != w.revealed)
		if !isOpen {
			continue
		}
		w.addRows(rows, revealPath, id, depth+1)
		(*rows)[position].isExpanded = len(*rows) > position+1
	}
}

// paint returns the rows of the tree that fit in height rows, scrolled so the selection stays on
// screen: each indented by its depth, with its arrow and the text rowText gives, the selected one
// and the muted ones marked. A height under one paints nothing and leaves the scroll as it is.
func (w *treeWidget[E]) paint(height int, rowText func(outline.Row[E]) string) []string {
	if height < 1 || len(w.rows) == 0 {
		return nil
	}
	selected := w.cursor.highlight(len(w.rows))
	w.top = clampInt(w.top, selected-height+1, selected)
	end := min(w.top+height, len(w.rows))
	painted := make([]string, 0, end-w.top)
	for position := w.top; position < end; position++ {
		row := w.rows[position]
		text := strings.Repeat(treeIndent, row.Depth) + treeArrow(row) + rowText(row.Row)
		painted = append(painted, rowStyle(row.isMuted, position == selected).Render(text))
	}
	return painted
}

// treeArrow returns the arrow of row: expanded, collapsed over hidden children, or none.
func treeArrow[E any](row treeRow[E]) string {
	switch {
	case row.isExpanded:
		return arrowExpanded
	case row.HasChildren:
		return arrowCollapsed
	}
	return arrowLeaf
}

// rowStyle returns the style of a tree row that is muted or selected, or both.
func rowStyle(isMuted, isSelected bool) lipgloss.Style {
	switch {
	case isMuted && isSelected:
		return mutedSelectedRowStyle
	case isMuted:
		return mutedRowStyle
	case isSelected:
		return selectedRowStyle
	}
	return lipgloss.NewStyle()
}
