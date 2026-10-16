package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/outline"
)

// treeEntries is an outline of root, holding the open kid and the closed gone, which holds the
// closed under, then the open other.
var treeEntries = []index.Entry{
	{ID: "root", Category: config.CategoryTodo},
	{ID: "kid", ParentID: "root", Category: config.CategoryTodo},
	{ID: "gone", ParentID: "root", Category: config.CategoryDone},
	{ID: "under", ParentID: "gone", Category: config.CategoryDone},
	{ID: "other", Category: config.CategoryTodo},
}

// newEntryTree returns the widget over the issue outline of entries, nothing expanded.
func newEntryTree(entries []index.Entry) treeWidget[index.Entry] {
	return newTreeWidget(
		outline.Issues(entries, false),
		func(entry index.Entry) string { return entry.ID },
		nil,
	)
}

// treeIDs returns the ids of the widget's visible rows, top to bottom.
func treeIDs(w treeWidget[index.Entry]) []string {
	ids := make([]string, 0, len(w.rows))
	for _, row := range w.rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func TestTreeWidget_New_OpensCollapsedToTheTopLevel(t *testing.T) {
	w := newEntryTree(treeEntries)

	if got := treeIDs(w); !slices.Equal(got, []string{"root", "other"}) {
		t.Errorf("rows = %v, want the top level: root, other", got)
	}
}

func TestTreeWidget_New_KeepsOnlyExpandedNodesStillInTheTree(t *testing.T) {
	w := newTreeWidget(
		outline.Issues(treeEntries, false),
		func(entry index.Entry) string { return entry.ID },
		map[string]bool{"root": true, "vanished": true},
	)

	if got := treeIDs(w); !slices.Equal(got, []string{"root", "kid", "other"}) || w.expanded["vanished"] {
		t.Errorf("rows = %v, expanded %v; want root expanded, vanished dropped", got, w.expanded)
	}
}

func TestTreeWidget_SelectID_HiddenNodeShowsMutedWithoutChildren(t *testing.T) {
	w := newEntryTree(treeEntries)

	isSelected := w.selectID("gone")
	w.expand()

	selected, _ := w.selection()
	if !isSelected || selected.ID != "gone" || !selected.isMuted || selected.HasChildren {
		t.Errorf("selected %+v (found %t), want gone, muted, with no children to expand", selected, isSelected)
	}
	if got := treeIDs(w); !slices.Equal(got, []string{"root", "kid", "gone", "other"}) {
		t.Errorf("rows after l on the revealed node = %v, want root, kid, gone, other", got)
	}
}

func TestTreeWidget_SelectID_UnknownIDChangesNothing(t *testing.T) {
	w := newEntryTree(treeEntries)

	isSelected := w.selectID("no-such-id")

	if isSelected || !slices.Equal(treeIDs(w), []string{"root", "other"}) {
		t.Errorf("selectID(no-such-id) = %t with rows %v, want false and the top level", isSelected, treeIDs(w))
	}
}

func TestTreeWidget_Expand_LeavesAnOlderCopyAsItWas(t *testing.T) {
	older := newEntryTree(treeEntries)
	newer := older

	newer.expand()

	if older.expanded["root"] || len(older.rows) != 2 {
		t.Errorf("older copy after the newer one expanded root: expanded %v, rows %v", older.expanded, treeIDs(older))
	}
}

func TestTreeWidget_Paint_IndentsAndMarksTheArrows(t *testing.T) {
	w := newEntryTree(treeEntries)
	w.expand()

	painted := w.paint(10, func(row outline.Row[index.Entry]) string { return row.ID })

	want := []string{"▾ root", "    kid", "  other"}
	for position, text := range want {
		if !strings.Contains(painted[position], text) {
			t.Errorf("row %d = %q, want %q", position, painted[position], text)
		}
	}
}
