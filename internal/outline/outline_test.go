package outline

import (
	"cmp"
	"reflect"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/knowledge"
)

// outlineAdd is one call walkOutline made to add, or the same fields of a Row.
type outlineAdd struct {
	entry     string
	indent    int
	more      int
	moreDepth int
}

func TestWalkOutline_DepthCutsAndCountsShownChildren(t *testing.T) {
	// a has children a1 and a2, a1 has a11; b has none. Entries are their own ids.
	children := map[string][]string{
		"":   {"a", "b"},
		"a":  {"a1", "a2"},
		"a1": {"a11"},
	}
	cases := []struct {
		name     string
		parentID string
		level    int
		indent   int
		maxDepth int
		want     []outlineAdd
	}{
		{
			name:     "whole tree, every level",
			parentID: "",
			level:    1,
			indent:   0,
			maxDepth: UnlimitedDepth,
			want: []outlineAdd{
				{"a", 0, 0, 0}, {"a1", 1, 0, 0}, {"a11", 2, 0, 0}, {"a2", 1, 0, 0}, {"b", 0, 0, 0},
			},
		},
		{
			name:     "whole tree, depth 1",
			parentID: "",
			level:    1,
			indent:   0,
			maxDepth: 1,
			want:     []outlineAdd{{"a", 0, 2, 2}, {"b", 0, 0, 2}},
		},
		{
			name:     "whole tree, depth 2",
			parentID: "",
			level:    1,
			indent:   0,
			maxDepth: 2,
			want:     []outlineAdd{{"a", 0, 0, 0}, {"a1", 1, 1, 3}, {"a2", 1, 0, 3}, {"b", 0, 0, 0}},
		},
		{
			name:     "below a, every level",
			parentID: "a",
			level:    1,
			indent:   1,
			maxDepth: UnlimitedDepth,
			want:     []outlineAdd{{"a1", 1, 0, 0}, {"a11", 2, 0, 0}, {"a2", 1, 0, 0}},
		},
		{
			name:     "below a, depth 1",
			parentID: "a",
			level:    1,
			indent:   1,
			maxDepth: 1,
			want:     []outlineAdd{{"a1", 1, 1, 2}, {"a2", 1, 0, 2}},
		},
		{
			name:     "below a, depth 2",
			parentID: "a",
			level:    1,
			indent:   1,
			maxDepth: 2,
			want:     []outlineAdd{{"a1", 1, 0, 0}, {"a11", 2, 0, 3}, {"a2", 1, 0, 0}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []outlineAdd

			walkOutline(
				func(parentID string) []string { return children[parentID] },
				func(entry string) string { return entry },
				tc.parentID,
				tc.level,
				tc.indent,
				tc.maxDepth,
				func(entry string, indent int, more int, moreDepth int) {
					got = append(got, outlineAdd{entry, indent, more, moreDepth})
				},
			)

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("adds = %v, want %v", got, tc.want)
			}
		})
	}
}

// treeEntry is an entry of a Tree under test: its id, its parent's id and whether the
// visibility rule hides it.
type treeEntry struct {
	id       string
	parentID string
	isHidden bool
}

// newTestTree returns a Tree of entries that hides the hidden ones, its groups in the order
// compare gives (entries' own order when nil).
func newTestTree(entries []treeEntry, compare func(left, right treeEntry) int) *Tree[treeEntry] {
	return NewTree(
		entries,
		func(entry treeEntry) string { return entry.id },
		func(entry treeEntry) string { return entry.parentID },
		compare,
		func(entry treeEntry) bool { return !entry.isHidden },
	)
}

// idsOf returns the ids of entries, in order.
func idsOf(entries []treeEntry) []string {
	ids := []string{}
	for _, entry := range entries {
		ids = append(ids, entry.id)
	}
	return ids
}

func TestOutline_VisibleTree_GroupsAllAndShown(t *testing.T) {
	// h is hidden and has a shown child h1; o's parent gone is not an entry, so o joins the top level.
	entries := []treeEntry{
		{id: "b"},
		{id: "a"},
		{id: "h", isHidden: true},
		{id: "h1", parentID: "h"},
		{id: "o", parentID: "gone"},
		{id: "a2", parentID: "a"},
		{id: "a1", parentID: "a", isHidden: true},
	}
	byID := func(left, right treeEntry) int { return cmp.Compare(left.id, right.id) }
	cases := []struct {
		name     string
		compare  func(left, right treeEntry) int
		parentID string
		wantAll  []string
		wantShow []string
	}{
		{"top level, entries' order", nil, "", []string{"b", "a", "h", "o"}, []string{"b", "a", "o"}},
		{"top level, sorted", byID, "", []string{"a", "b", "h", "o"}, []string{"a", "b", "o"}},
		{"below a, sorted", byID, "a", []string{"a1", "a2"}, []string{"a2"}},
		{"below hidden h", nil, "h", []string{"h1"}, []string{"h1"}},
		{"below a leaf", nil, "b", []string{}, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newTestTree(entries, tc.compare)

			all, shown := idsOf(tree.Children(tc.parentID)), idsOf(tree.Shown(tc.parentID))

			if !reflect.DeepEqual(all, tc.wantAll) || !reflect.DeepEqual(shown, tc.wantShow) {
				t.Errorf("all = %v, shown = %v, want %v and %v", all, shown, tc.wantAll, tc.wantShow)
			}
		})
	}
}

func TestOutline_VisibleTreeWalk_StartsAtTopOrAtStart(t *testing.T) {
	// h is hidden, so its child h1 is left out below the top, but h prints when named as the start.
	entries := []treeEntry{
		{id: "a"},
		{id: "a1", parentID: "a"},
		{id: "a11", parentID: "a1"},
		{id: "h", isHidden: true},
		{id: "h1", parentID: "h"},
	}
	hidden := entries[3]
	parent := entries[0]
	cases := []struct {
		name     string
		start    *treeEntry
		maxDepth int
		want     []outlineAdd
	}{
		{"top, every level", nil, UnlimitedDepth, []outlineAdd{{"a", 0, 0, 0}, {"a1", 1, 0, 0}, {"a11", 2, 0, 0}}},
		{"top, depth 1", nil, 1, []outlineAdd{{"a", 0, 1, 2}}},
		{"start a, depth 1", &parent, 1, []outlineAdd{{"a", 0, 0, 0}, {"a1", 1, 1, 2}}},
		{"start hidden h", &hidden, UnlimitedDepth, []outlineAdd{{"h", 0, 0, 0}, {"h1", 1, 0, 0}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newTestTree(entries, nil)
			var got []outlineAdd

			for _, row := range tree.Rows(tc.start, tc.maxDepth) {
				got = append(got, outlineAdd{row.ID, row.Depth, row.More, row.MoreDepth})
			}

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("adds = %v, want %v", got, tc.want)
			}
		})
	}
}

// rowSummary is the position of one Row: its id, parent, depth, whether it has shown children and
// what a depth cut left out.
type rowSummary struct {
	id          string
	parentID    string
	depth       int
	hasChildren bool
	more        int
	moreDepth   int
}

// summariesOf returns the position of each row, in order.
func summariesOf[E any](rows []Row[E]) []rowSummary {
	summaries := []rowSummary{}
	for _, row := range rows {
		summaries = append(summaries, rowSummary{
			row.ID, row.ParentID, row.Depth, row.HasChildren, row.More, row.MoreDepth,
		})
	}
	return summaries
}

func TestIssues_RowsHideClosedSubtreesUnlessAll(t *testing.T) {
	// d is done with an open child d1; c is closed below open a. Entries keep their given order.
	entries := []index.Entry{
		{ID: "b", Category: config.CategoryTodo},
		{ID: "a", Category: config.CategoryDoing},
		{ID: "c", ParentID: "a", Category: config.CategoryDropped},
		{ID: "a1", ParentID: "a", Category: config.CategoryTodo},
		{ID: "d", Category: config.CategoryDone},
		{ID: "d1", ParentID: "d", Category: config.CategoryTodo},
	}
	cases := []struct {
		name  string
		isAll bool
		want  []rowSummary
	}{
		{"closed hidden with their subtrees", false, []rowSummary{
			{"b", "", 0, false, 0, 0},
			{"a", "", 0, true, 0, 0},
			{"a1", "a", 1, false, 0, 0},
		}},
		{"all shows closed issues", true, []rowSummary{
			{"b", "", 0, false, 0, 0},
			{"a", "", 0, true, 0, 0},
			{"c", "a", 1, false, 0, 0},
			{"a1", "a", 1, false, 0, 0},
			{"d", "", 0, true, 0, 0},
			{"d1", "d", 1, false, 0, 0},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := Issues(entries, tc.isAll)

			got := summariesOf(tree.Rows(nil, UnlimitedDepth))

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("rows = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIssues_ChildrenIncludeHiddenIssues(t *testing.T) {
	entries := []index.Entry{
		{ID: "a", Category: config.CategoryTodo},
		{ID: "a1", ParentID: "a", Category: config.CategoryDone},
		{ID: "a2", ParentID: "a", Category: config.CategoryTodo},
	}

	tree := Issues(entries, false)

	got := []string{}
	for _, child := range tree.Children("a") {
		got = append(got, child.ID)
	}
	if want := []string{"a1", "a2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Children(a) = %v, want %v", got, want)
	}
}

func TestIssues_RowsShowHiddenStartAndCutAtDepth(t *testing.T) {
	// d is done, so hidden, but prints when named as the start; d1 has two open children.
	entries := []index.Entry{
		{ID: "d", Category: config.CategoryDone},
		{ID: "d1", ParentID: "d", Category: config.CategoryTodo},
		{ID: "d11", ParentID: "d1", Category: config.CategoryTodo},
		{ID: "d12", ParentID: "d1", Category: config.CategoryTodo},
	}
	start := entries[0]
	cases := []struct {
		name     string
		maxDepth int
		want     []rowSummary
	}{
		{"every level", UnlimitedDepth, []rowSummary{
			{"d", "", 0, true, 0, 0},
			{"d1", "d", 1, true, 0, 0},
			{"d11", "d1", 2, false, 0, 0},
			{"d12", "d1", 2, false, 0, 0},
		}},
		{"depth 1 cuts d1's children", 1, []rowSummary{
			{"d", "", 0, true, 0, 0},
			{"d1", "d", 1, true, 2, 2},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := Issues(entries, false)

			got := summariesOf(tree.Rows(&start, tc.maxDepth))

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("rows = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNotes_RowsSortByIDAndHideBackgroundSubtrees(t *testing.T) {
	// bg is background with a plain child bg1, which stays hidden with it; o's parent is not a Note.
	background := []string{knowledge.LabelBackground}
	notes := []index.NoteEntry{
		{ID: "zeta"},
		{ID: "bg", Labels: background},
		{ID: "bg1", ParentID: "bg"},
		{ID: "alpha"},
		{ID: "alpha-2", ParentID: "alpha"},
		{ID: "alpha-1", ParentID: "alpha"},
		{ID: "o", ParentID: "gone"},
	}
	cases := []struct {
		name                 string
		isBackgroundIncluded bool
		want                 []rowSummary
	}{
		{"background hidden", false, []rowSummary{
			{"alpha", "", 0, true, 0, 0},
			{"alpha-1", "alpha", 1, false, 0, 0},
			{"alpha-2", "alpha", 1, false, 0, 0},
			{"o", "", 0, false, 0, 0},
			{"zeta", "", 0, false, 0, 0},
		}},
		{"background included", true, []rowSummary{
			{"alpha", "", 0, true, 0, 0},
			{"alpha-1", "alpha", 1, false, 0, 0},
			{"alpha-2", "alpha", 1, false, 0, 0},
			{"bg", "", 0, true, 0, 0},
			{"bg1", "bg", 1, false, 0, 0},
			{"o", "", 0, false, 0, 0},
			{"zeta", "", 0, false, 0, 0},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := Notes(notes, tc.isBackgroundIncluded)

			got := summariesOf(tree.Rows(nil, UnlimitedDepth))

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("rows = %v, want %v", got, tc.want)
			}
		})
	}
}
