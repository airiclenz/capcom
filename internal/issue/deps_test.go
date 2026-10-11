package issue_test

import (
	"reflect"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/issue"
)

func TestFindCycle_Edges_ReportTheCycleTheNewEdgeCloses(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		edges    map[string][]string
		from, to string
		want     []string
	}{
		{
			name: "no edges",
			from: "a", to: "b",
			want: nil,
		},
		{
			name:  "self dependency",
			edges: map[string][]string{"a": {"b"}},
			from:  "a", to: "a",
			want: []string{"a", "a"},
		},
		{
			name:  "length two",
			edges: map[string][]string{"b": {"a"}},
			from:  "a", to: "b",
			want: []string{"a", "b", "a"},
		},
		{
			name:  "length three",
			edges: map[string][]string{"b": {"c"}, "c": {"a"}},
			from:  "a", to: "b",
			want: []string{"a", "b", "c", "a"},
		},
		{
			name:  "shortest way back is taken",
			edges: map[string][]string{"b": {"c", "d"}, "c": {"e"}, "e": {"a"}, "d": {"a"}},
			from:  "a", to: "b",
			want: []string{"a", "b", "d", "a"},
		},
		{
			name:  "diamond without a way back",
			edges: map[string][]string{"a": {"c"}, "b": {"c"}, "c": {"d"}},
			from:  "a", to: "b",
			want: nil,
		},
		{
			name:  "existing cycle elsewhere is ignored",
			edges: map[string][]string{"b": {"c"}, "c": {"b"}, "a": {"x"}},
			from:  "a", to: "b",
			want: nil,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := issue.FindCycle(testCase.edges, testCase.from, testCase.to)

			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("FindCycle(%s -> %s) = %v, want %v", testCase.from, testCase.to, got, testCase.want)
			}
		})
	}
}

// depsNodes is a tree whose dependencies are written by id, by alias, by an alias two issues
// claim and by a name no issue has.
func depsNodes() []issue.Node {
	schema := readyNode("db-schema", config.CategoryDone)
	schema.Aliases = []string{"apogee-kk0.4"}
	first := readyNode("first-claim", config.CategoryTodo)
	first.Aliases = []string{"shared-name"}
	second := readyNode("second-claim", config.CategoryTodo)
	second.Aliases = []string{"shared-name"}
	return []issue.Node{
		schema,
		withDeps(readyNode("by-alias", config.CategoryTodo), "apogee-kk0.4"),
		withDeps(readyNode("by-id", config.CategoryTodo), "db-schema", "apogee-kk0.4"),
		withDeps(readyNode("dangling", config.CategoryTodo), "nothing-here", "shared-name"),
		first,
		second,
	}
}

func TestDepsGraph_Dependents_ResolveIDsAndAliasesOnce(t *testing.T) {
	t.Parallel()
	graph := issue.NewDepGraph(depsNodes())

	got := graph.Dependents("db-schema")

	if want := []string{"by-alias", "by-id"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Dependents(db-schema) = %v, want %v", got, want)
	}
}

func TestDepsGraph_Resolve_ExactIDThenUniqueAliasNeverPrefix(t *testing.T) {
	t.Parallel()
	graph := issue.NewDepGraph(depsNodes())
	cases := map[string]string{
		"db-schema":    "db-schema",
		"apogee-kk0.4": "db-schema",
		"db-sch":       "",
		"shared-name":  "",
		"nothing-here": "",
	}
	for dep, want := range cases {
		got, isFound := graph.Resolve(dep)

		if got != want || isFound != (want != "") {
			t.Errorf("Resolve(%q) = %q, %v, want %q", dep, got, isFound, want)
		}
	}
}

func TestDepsGraph_FindCycle_FollowsEdgesWrittenAsAliases(t *testing.T) {
	t.Parallel()
	nodes := depsNodes()
	nodes[0] = withDeps(nodes[0], "by-id")
	graph := issue.NewDepGraph(nodes)

	got := graph.FindCycle("by-id", "by-alias")

	if want := []string{"by-id", "by-alias", "db-schema", "by-id"}; !reflect.DeepEqual(got, want) {
		t.Errorf("FindCycle(by-id -> by-alias) = %v, want %v", got, want)
	}
}
