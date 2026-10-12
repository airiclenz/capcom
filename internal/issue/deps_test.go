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

func TestCycles_Edges_ReportEachCycleOnceInCanonicalRotation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		edges map[string][]string
		want  [][]string
	}{
		{
			name:  "no cycle",
			edges: map[string][]string{"a": {"b"}, "b": {"c"}},
			want:  nil,
		},
		{
			name:  "self dependency",
			edges: map[string][]string{"b": {"b"}},
			want:  [][]string{{"b", "b"}},
		},
		{
			name:  "one 3-cycle reported once from its smallest id",
			edges: map[string][]string{"c": {"a"}, "a": {"b"}, "b": {"c"}},
			want:  [][]string{{"a", "b", "c", "a"}},
		},
		{
			name:  "two separate cycles",
			edges: map[string][]string{"x": {"y"}, "y": {"x"}, "b": {"a"}, "a": {"b"}},
			want:  [][]string{{"a", "b", "a"}, {"x", "y", "x"}},
		},
		{
			name:  "two cycles sharing an edge",
			edges: map[string][]string{"a": {"b"}, "b": {"a", "c"}, "c": {"b"}},
			want:  [][]string{{"a", "b", "a"}, {"b", "c", "b"}},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := issue.Cycles(testCase.edges)

			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("Cycles = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestDepsGraph_Cycles_FollowsEdgesWrittenAsAliases(t *testing.T) {
	t.Parallel()
	nodes := depsNodes()
	nodes[0] = withDeps(nodes[0], "by-alias")
	graph := issue.NewDepGraph(nodes)

	got := graph.Cycles()

	if want := [][]string{{"by-alias", "db-schema", "by-alias"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("Cycles = %v, want %v", got, want)
	}
}

func TestDepsGraph_Walk_StepsToDepth(t *testing.T) {
	t.Parallel()
	diamond := []issue.Node{
		withDeps(readyNode("app", config.CategoryTodo), "api", "ui"),
		withDeps(readyNode("api", config.CategoryTodo), "schema"),
		withDeps(readyNode("ui", config.CategoryTodo), "schema"),
		readyNode("schema", config.CategoryTodo),
	}
	cycle := []issue.Node{
		withDeps(readyNode("a", config.CategoryTodo), "b"),
		withDeps(readyNode("b", config.CategoryTodo), "c"),
		withDeps(readyNode("c", config.CategoryTodo), "a"),
	}
	chain := []issue.Node{
		withDeps(readyNode("a", config.CategoryTodo), "b"),
		withDeps(readyNode("b", config.CategoryTodo), "c", "d"),
		withDeps(readyNode("c", config.CategoryTodo), "e"),
		readyNode("d", config.CategoryTodo),
		readyNode("e", config.CategoryTodo),
	}
	topOfDeps := append(depsNodes(), withDeps(readyNode("top", config.CategoryTodo), "by-id"))
	cases := []struct {
		name      string
		nodes     []issue.Node
		start     string
		direction issue.WalkDirection
		maxDepth  int
		want      []issue.WalkStep
	}{
		{
			name:  "diamond yields one repeat step",
			nodes: diamond, start: "app", direction: issue.WalkDeps, maxDepth: issue.UnlimitedDepth,
			want: []issue.WalkStep{
				{ID: "api", Via: "app", Depth: 1},
				{ID: "schema", Via: "api", Depth: 2},
				{ID: "ui", Via: "app", Depth: 1},
				{ID: "schema", Via: "ui", Depth: 2, IsRepeat: true},
			},
		},
		{
			name:  "diamond walked from the bottom",
			nodes: diamond, start: "schema", direction: issue.WalkDependents, maxDepth: issue.UnlimitedDepth,
			want: []issue.WalkStep{
				{ID: "api", Via: "schema", Depth: 1},
				{ID: "app", Via: "api", Depth: 2},
				{ID: "ui", Via: "schema", Depth: 1},
				{ID: "app", Via: "ui", Depth: 2, IsRepeat: true},
			},
		},
		{
			name:  "cycle ends on the start",
			nodes: cycle, start: "a", direction: issue.WalkDeps, maxDepth: issue.UnlimitedDepth,
			want: []issue.WalkStep{
				{ID: "b", Via: "a", Depth: 1},
				{ID: "c", Via: "b", Depth: 2},
				{ID: "a", Via: "c", Depth: 3, IsRepeat: true},
			},
		},
		{
			name:  "depth cap counts the neighbours it leaves out",
			nodes: chain, start: "a", direction: issue.WalkDeps, maxDepth: 2,
			want: []issue.WalkStep{
				{ID: "b", Via: "a", Depth: 1},
				{ID: "c", Via: "b", Depth: 2, Cut: 1},
				{ID: "d", Via: "b", Depth: 2},
			},
		},
		{
			name:  "depth cap at the first step",
			nodes: chain, start: "a", direction: issue.WalkDeps, maxDepth: 1,
			want: []issue.WalkStep{{ID: "b", Via: "a", Depth: 1, Cut: 2}},
		},
		{
			name:  "dependency named by id and by alias is one step",
			nodes: depsNodes(), start: "by-id", direction: issue.WalkDeps, maxDepth: issue.UnlimitedDepth,
			want: []issue.WalkStep{{ID: "db-schema", Via: "by-id", Depth: 1}},
		},
		{
			name:  "dependency named by id and by alias is cut once",
			nodes: topOfDeps, start: "top", direction: issue.WalkDeps, maxDepth: 1,
			want: []issue.WalkStep{{ID: "by-id", Via: "top", Depth: 1, Cut: 1}},
		},
		{
			name:  "dependent naming by id and by alias is one step",
			nodes: depsNodes(), start: "db-schema", direction: issue.WalkDependents, maxDepth: issue.UnlimitedDepth,
			want: []issue.WalkStep{
				{ID: "by-alias", Via: "db-schema", Depth: 1},
				{ID: "by-id", Via: "db-schema", Depth: 1},
			},
		},
		{
			name:  "unknown start",
			nodes: chain, start: "nothing-here", direction: issue.WalkDeps, maxDepth: issue.UnlimitedDepth,
			want: nil,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			graph := issue.NewDepGraph(testCase.nodes)

			got := graph.Walk(testCase.start, testCase.direction, testCase.maxDepth)

			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("Walk(%s) = %+v, want %+v", testCase.start, got, testCase.want)
			}
		})
	}
}
