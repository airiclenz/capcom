package cli

import (
	"cmp"
	"reflect"
	"testing"
)

// outlineAdd is one call walkOutline made to add.
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
			maxDepth: unlimitedDepth,
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
			maxDepth: unlimitedDepth,
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

// treeEntry is an entry of a visibleTree under test: its id, its parent's id and whether the
// visibility rule hides it.
type treeEntry struct {
	id       string
	parentID string
	isHidden bool
}

// newTestTree returns a visibleTree of entries that hides the hidden ones, its groups in the order
// compare gives (entries' own order when nil).
func newTestTree(entries []treeEntry, compare func(left, right treeEntry) int) *visibleTree[treeEntry] {
	return newVisibleTree(
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

			all, shown := idsOf(tree.all(tc.parentID)), idsOf(tree.shown(tc.parentID))

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
		{"top, every level", nil, unlimitedDepth, []outlineAdd{{"a", 0, 0, 0}, {"a1", 1, 0, 0}, {"a11", 2, 0, 0}}},
		{"top, depth 1", nil, 1, []outlineAdd{{"a", 0, 1, 2}}},
		{"start a, depth 1", &parent, 1, []outlineAdd{{"a", 0, 0, 0}, {"a1", 1, 1, 2}}},
		{"start hidden h", &hidden, unlimitedDepth, []outlineAdd{{"h", 0, 0, 0}, {"h1", 1, 0, 0}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newTestTree(entries, nil)
			var got []outlineAdd

			tree.walk(tc.start, tc.maxDepth, func(entry treeEntry, indent int, more int, moreDepth int) {
				got = append(got, outlineAdd{entry.id, indent, more, moreDepth})
			})

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("adds = %v, want %v", got, tc.want)
			}
		})
	}
}
