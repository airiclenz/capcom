package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/outline"
	"github.com/airiclenz/capcom/internal/userconfig"
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

// glyphEntries is an outline of main, holding first, which holds only, then last, which holds
// one, two and the closed gone.
var glyphEntries = []index.Entry{
	{ID: "main", Category: config.CategoryTodo},
	{ID: "first", ParentID: "main", Category: config.CategoryTodo},
	{ID: "only", ParentID: "first", Category: config.CategoryTodo},
	{ID: "last", ParentID: "main", Category: config.CategoryTodo},
	{ID: "one", ParentID: "last", Category: config.CategoryTodo},
	{ID: "two", ParentID: "last", Category: config.CategoryTodo},
	{ID: "gone", ParentID: "last", Category: config.CategoryDone},
}

// paintIDs returns the rows w paints with each row's id as its text, stripped of styling.
func paintIDs(t *testing.T, w treeWidget[index.Entry]) []string {
	t.Helper()
	th := newTheme(userconfig.Default().UI, false, false)
	painted := w.paint(len(w.rows), th, func(row outline.Row[index.Entry], _ bool) string { return row.ID })
	stripped := make([]string, 0, len(painted))
	for _, row := range painted {
		stripped = append(stripped, ansi.Strip(row))
	}
	return stripped
}

func TestTreeWidget_Paint_DrawsGlyphsAndConnectors(t *testing.T) {
	tests := []struct {
		name     string
		expanded []string
		reveal   string
		want     []string
	}{
		{
			name:     "a collapsed child, then an expanded last child",
			expanded: []string{"main", "last"},
			want:     []string{"▼ main", "┝ ▶ first", "┕ ▼ last", "  ┝ · one", "  ┕ · two"},
		},
		{
			name:     "an expanded child a sibling follows continues as a line",
			expanded: []string{"main", "first", "last"},
			want:     []string{"▼ main", "┝ ▼ first", "│ ┕ · only", "┕ ▼ last", "  ┝ · one", "  ┕ · two"},
		},
		{
			name:     "a revealed last child takes the elbow",
			expanded: []string{"main", "last"},
			reveal:   "gone",
			want:     []string{"▼ main", "┝ ▶ first", "┕ ▼ last", "  ┝ · one", "  ┝ · two", "  ┕ · gone"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expanded := map[string]bool{}
			for _, id := range tt.expanded {
				expanded[id] = true
			}
			w := newTreeWidget(
				outline.Issues(glyphEntries, false),
				func(entry index.Entry) string { return entry.ID },
				expanded,
			)
			if tt.reveal != "" {
				w.selectID(tt.reveal)
			}

			got := paintIDs(t, w)

			if !slices.Equal(got, tt.want) {
				t.Errorf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

func TestTreeWidget_Paint_MarksTheSelectionAfterTheConnector(t *testing.T) {
	w := newTreeWidget(
		outline.Issues(glyphEntries, false),
		func(entry index.Entry) string { return entry.ID },
		map[string]bool{"main": true, "last": true},
	)
	w.selectID("two")
	th := newTheme(userconfig.Default().UI, false, false)

	painted := w.paint(len(w.rows), th, func(row outline.Row[index.Entry], _ bool) string { return row.ID })

	want := th.border.Render("  ┕ ") + th.selectedRow.Render("· two")
	if got := painted[len(painted)-1]; got != want {
		t.Errorf("selected row = %q, want %q", got, want)
	}
	if got := painted[0]; got != "▼ main" {
		t.Errorf("top-level row = %q, want %q with no connector styling", got, "▼ main")
	}
}
