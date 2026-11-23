package index

import (
	"maps"
	"slices"
	"testing"

	"github.com/airiclenz/capcom/internal/issue"
)

// entryGraphFixture indexes the issues of the dep graph tests: build-pipeline depends on db-schema
// by its alias, db-schema on the Closed release-notes, and login-flow stands alone. It returns
// their EntryGraph.
func entryGraphFixture(t *testing.T) EntryGraph {
	t.Helper()
	s := newStore(t)
	writeIssue(t, s, "build-pipeline.md", issueWith("Build pipeline", "open", "deps: [apogee-kk0.4]\n"))
	writeIssue(t, s, "db-schema.md", issueWith("Schema", "open", "deps: [release-notes]\naliases: [apogee-kk0.4]\n"))
	writeIssue(t, s, "login-flow.md", issueWith("Login flow", "open", ""))
	writeIssue(t, s, "release-notes.md", issueWith("Release notes", "done", ""))
	x := openIndex(t, s)
	refresh(t, x)
	graph, err := x.EntryGraph()
	if err != nil {
		t.Fatalf("EntryGraph: %v", err)
	}
	return graph
}

// shownIDs returns the ids of the graph's entries that isShown keeps, in tree order.
func shownIDs(graph EntryGraph, isShown func(Entry) bool) []string {
	var shown []string
	for _, entry := range graph.Entries() {
		if isShown(entry) {
			shown = append(shown, entry.ID)
		}
	}
	return shown
}

func TestEntryGraph_EveryUsableIssue_ByIDAndInTreeOrder(t *testing.T) {
	t.Parallel()
	graph := entryGraphFixture(t)

	got := ids(graph.Entries())

	want := []string{"build-pipeline", "db-schema", "login-flow", "release-notes"}
	if !slices.Equal(got, want) {
		t.Errorf("Entries = %v, want %v", got, want)
	}
	if entry := graph.Entry("release-notes"); entry.Title != "Release notes" {
		t.Errorf("Entry(release-notes).Title = %q, want the Closed issue's title", entry.Title)
	}
	if entry := graph.Entry("no-such-issue"); entry.ID != "" {
		t.Errorf("Entry(no-such-issue) = %+v, want the zero Entry", entry)
	}
}

func TestEntryGraph_Names(t *testing.T) {
	t.Parallel()
	graph := entryGraphFixture(t)
	cases := []struct {
		name string
		dep  string
		want bool
	}{
		{"the id itself", "db-schema", true},
		{"an alias of the issue", "apogee-kk0.4", true},
		{"another issue's id", "login-flow", false},
		{"a dep naming no issue", "no-such-issue", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := graph.Names("db-schema")(testCase.dep)

			if got != testCase.want {
				t.Errorf("Names(db-schema)(%q) = %v, want %v", testCase.dep, got, testCase.want)
			}
		})
	}
}

func TestEntryGraph_ShownBy(t *testing.T) {
	t.Parallel()
	graph := entryGraphFixture(t)
	cases := []struct {
		name  string
		start string
		isAll bool
		want  []string
	}{
		{"no start keeps every issue not closed", "", false, []string{"build-pipeline", "db-schema", "login-flow"}},
		{"no start with all keeps every issue", "", true,
			[]string{"build-pipeline", "db-schema", "login-flow", "release-notes"}},
		{"a start keeps what it reaches, closed ones left out", "build-pipeline", false,
			[]string{"build-pipeline", "db-schema"}},
		{"a start with all adds the closed issues it reaches", "db-schema", true,
			[]string{"build-pipeline", "db-schema", "release-notes"}},
		{"a closed start is itself shown", "release-notes", false,
			[]string{"build-pipeline", "db-schema", "release-notes"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := shownIDs(graph, graph.ShownBy(testCase.start, testCase.isAll))

			if !slices.Equal(got, testCase.want) {
				t.Errorf("ShownBy(%q, %v) keeps %v, want %v", testCase.start, testCase.isAll, got, testCase.want)
			}
		})
	}
}

func TestDepBranches_SplitsAtTheDepthOneSteps(t *testing.T) {
	t.Parallel()
	steps := []issue.WalkStep{
		{ID: "db-schema", Via: "build-pipeline", Depth: 1},
		{ID: "release-notes", Via: "db-schema", Depth: 2},
		{ID: "login-flow", Via: "build-pipeline", Depth: 1},
	}

	got := DepBranches(steps)

	want := map[string][]issue.WalkStep{
		"db-schema":  {steps[0], steps[1]},
		"login-flow": {steps[2]},
	}
	if !maps.EqualFunc(got, want, slices.Equal[[]issue.WalkStep]) {
		t.Errorf("DepBranches = %v, want %v", got, want)
	}
}

func TestDepBranches_NoSteps_IsEmpty(t *testing.T) {
	t.Parallel()

	got := DepBranches(nil)

	if len(got) != 0 {
		t.Errorf("DepBranches(nil) = %v, want empty", got)
	}
}
