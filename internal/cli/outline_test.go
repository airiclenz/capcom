package cli

import (
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
