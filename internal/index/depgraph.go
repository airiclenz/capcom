package index

import "github.com/airiclenz/capcom/internal/issue"

// EntryGraph is the Dependency graph of every usable indexed issue, Closed ones included, with
// each issue's index entry by id and in tree order. It is what dep graph, dep list, related and
// context read; dep add and dep rm decide their edits on the graph of ops.DepGraph instead.
type EntryGraph struct {
	graph   issue.DepGraph
	entries map[string]Entry
	ordered []Entry
}

// EntryGraph builds the Dependency graph of every usable issue in the index, Closed ones
// included, with each issue's index entry.
func (x *Index) EntryGraph() (EntryGraph, error) {
	entries, err := x.List(Filter{IsClosedIncluded: true})
	if err != nil {
		return EntryGraph{}, err
	}
	byID := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = entry
	}
	return EntryGraph{graph: issue.NewDepGraph(Nodes(entries)), entries: byID, ordered: entries}, nil
}

// DepGraph returns the Dependency graph itself, for walking it and resolving deps.
func (g EntryGraph) DepGraph() issue.DepGraph {
	return g.graph
}

// Entry returns the index entry of the issue with id, the zero Entry when the graph has none.
func (g EntryGraph) Entry(id string) Entry {
	return g.entries[id]
}

// Entries returns the index entry of every issue in the graph, in tree order.
func (g EntryGraph) Entries() []Entry {
	return g.ordered
}

// Names returns a test of whether a dep, as a file writes it, names the issue with id.
func (g EntryGraph) Names(id string) func(string) bool {
	return func(dep string) bool {
		resolved, isFound := g.graph.Resolve(dep)
		return isFound && resolved == id
	}
}

// ShownBy returns the test of whether an issue is a node of dep graph. With start "" every issue
// that is not closed is; otherwise only start and the issues its deps and dependents reach, walked
// through closed issues too. Closed issues are nodes only with isAll, start always is.
func (g EntryGraph) ShownBy(start string, isAll bool) func(Entry) bool {
	isOpenOrAll := func(entry Entry) bool { return isAll || !entry.Category.IsClosed() }
	if start == "" {
		return isOpenOrAll
	}

	isReached := map[string]bool{start: true}
	for _, direction := range []issue.WalkDirection{issue.WalkDeps, issue.WalkDependents} {
		for _, step := range g.graph.Walk(start, direction, issue.UnlimitedDepth) {
			isReached[step.ID] = true
		}
	}
	return func(entry Entry) bool {
		return entry.ID == start || (isReached[entry.ID] && isOpenOrAll(entry))
	}
}

// DepBranches splits the steps of a walk at its depth-1 steps: each depth-1 issue's id maps to
// its step followed by the steps of its branch, in walk order.
func DepBranches(steps []issue.WalkStep) map[string][]issue.WalkStep {
	branches := map[string][]issue.WalkStep{}
	var current string
	for _, step := range steps {
		if step.Depth == 1 {
			current = step.ID
		}
		branches[current] = append(branches[current], step)
	}
	return branches
}
