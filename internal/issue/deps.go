package issue

import (
	"slices"
	"strings"
)

// DepGraph is the Dependency edges between issues, each dependency resolved to the id of the issue
// it names. A file stores only the dependent's side of an edge (an issue's deps), so the graph is
// how the other side, an issue's dependents, is found. A dependency resolves as the ready rules
// resolve it: by exact id, then by an Alias exactly one issue claims, never by prefix (ADR 0002).
// A dependency that names no issue has no edge.
type DepGraph struct {
	resolver readyTree
	// edges maps each id to the ids of the issues it depends on, in the file's order.
	edges map[string][]string
	// dependents maps each id to the ids of the issues that depend on it, each once, in node order.
	dependents map[string][]string
}

// NewDepGraph builds the graph of every node's dependencies. It needs every issue of the tree,
// Closed ones included, since a dependency may name any of them.
func NewDepGraph(nodes []Node) DepGraph {
	graph := DepGraph{
		resolver:   newReadyTree(nodes),
		edges:      make(map[string][]string, len(nodes)),
		dependents: make(map[string][]string, len(nodes)),
	}
	for _, node := range nodes {
		for _, dep := range node.Deps {
			id, isFound := graph.Resolve(dep)
			if !isFound {
				continue
			}
			graph.edges[node.ID] = append(graph.edges[node.ID], id)
			// A dependent names its dependency twice when the file writes it by id and by alias.
			if !slices.Contains(graph.dependents[id], node.ID) {
				graph.dependents[id] = append(graph.dependents[id], node.ID)
			}
		}
	}
	return graph
}

// Resolve returns the id of the issue dep names, and false when it names none.
func (g DepGraph) Resolve(dep string) (string, bool) {
	node, isFound := g.resolver.resolve(dep)
	return node.ID, isFound
}

// Dependents returns the ids of the issues that depend on id, each once, in node order.
func (g DepGraph) Dependents(id string) []string {
	return slices.Clone(g.dependents[id])
}

// UnlimitedDepth is the maxDepth that lets Walk go as deep as the graph reaches.
const UnlimitedDepth = 0

// WalkDirection picks the edges Walk follows from an issue.
type WalkDirection int

const (
	// WalkDeps follows an issue to the issues it depends on, in the file's order.
	WalkDeps WalkDirection = iota
	// WalkDependents follows an issue to the issues that depend on it, in node order.
	WalkDependents
)

// WalkStep is one issue Walk reaches.
type WalkStep struct {
	ID string
	// Via is the id of the issue this step hangs under: the walk's start, or an earlier step.
	Via string
	// Depth is how many edges this step lies from the start: 1 for the start's own neighbours.
	Depth int
	// Cut is how many neighbours of this issue the depth cap left out; 0 below the cap.
	Cut int
	// IsRepeat is true when the walk reached this issue before (the start included): the step
	// stands for it without walking its neighbours again, and the cap cuts nothing below it.
	IsRepeat bool
}

// Walk follows the edges of direction from the issue with id, pre-order and depth first, and
// returns a step for every issue it reaches; the start itself has no step. Each issue's
// neighbours are taken once each, however many times its file names one, in the direction's
// order. Each issue is walked once: reaching it again yields a repeat step, so the walk ends on a
// cycle. An issue at depth maxDepth is not walked further and its step counts the neighbours left
// out; a maxDepth of UnlimitedDepth (or below) sets no cap. An id that is no issue yields no step.
func (g DepGraph) Walk(id string, direction WalkDirection, maxDepth int) []WalkStep {
	walk := depWalk{
		graph:     g,
		direction: direction,
		maxDepth:  maxDepth,
		isReached: map[string]bool{id: true},
	}
	walk.expand(id, 1)
	return walk.steps
}

// depWalk is the state of one Walk.
type depWalk struct {
	graph     DepGraph
	direction WalkDirection
	maxDepth  int
	// isReached holds every id the walk has a step for, and its start.
	isReached map[string]bool
	steps     []WalkStep
}

// expand appends a step for each neighbour of id, which lies at depth-1, walking each neighbour
// reached for the first time before the next.
func (w *depWalk) expand(id string, depth int) {
	for _, next := range w.graph.neighbours(id, w.direction) {
		step := WalkStep{ID: next, Via: id, Depth: depth}
		if w.isReached[next] {
			step.IsRepeat = true
			w.steps = append(w.steps, step)
			continue
		}
		w.isReached[next] = true
		isCapped := w.maxDepth > UnlimitedDepth && depth >= w.maxDepth
		if isCapped {
			step.Cut = len(w.graph.neighbours(next, w.direction))
		}
		w.steps = append(w.steps, step)
		if !isCapped {
			w.expand(next, depth+1)
		}
	}
}

// neighbours returns the ids the edges of direction lead to from id, each once, in the
// direction's order.
func (g DepGraph) neighbours(id string, direction WalkDirection) []string {
	if direction == WalkDependents {
		return g.dependents[id]
	}
	var deps []string
	for _, dep := range g.edges[id] {
		// The file may name one dependency twice: by id and by alias.
		if !slices.Contains(deps, dep) {
			deps = append(deps, dep)
		}
	}
	return deps
}

// FindCycle reports the cycle an edge from from to to would close in this graph; see FindCycle.
func (g DepGraph) FindCycle(from string, to string) []string {
	return FindCycle(g.edges, from, to)
}

// Cycles returns the dependency cycles of this graph; see Cycles.
func (g DepGraph) Cycles() [][]string {
	return Cycles(g.edges)
}

// Cycles returns the cycles in edges, each once. edges maps each id to the ids it depends on. For
// every edge that lies on a cycle it takes the cycle FindCycle reports for that edge, so every
// such edge appears in at least one returned cycle. Each cycle comes back in its canonical
// rotation: starting and ending with its smallest id (a, b, c, a), an edge from an id to itself
// as a, a. The cycles are ordered by their ids. It returns nil when edges has no cycle.
func Cycles(edges map[string][]string) [][]string {
	froms := make([]string, 0, len(edges))
	for from := range edges {
		froms = append(froms, from)
	}
	slices.Sort(froms)

	seen := map[string]bool{}
	var cycles [][]string
	for _, from := range froms {
		for _, to := range edges[from] {
			cycle := FindCycle(edges, from, to)
			if cycle == nil {
				continue
			}
			cycle = canonicalRotation(cycle)
			key := strings.Join(cycle, cycleKeySeparator)
			if seen[key] {
				continue
			}
			seen[key] = true
			cycles = append(cycles, cycle)
		}
	}
	slices.SortFunc(cycles, slices.Compare[[]string])
	return cycles
}

// cycleKeySeparator joins a cycle's ids into the key that tells cycles apart; no id contains it.
const cycleKeySeparator = "\x00"

// canonicalRotation rotates the closed cycle a, ..., a so that it starts and ends with its smallest
// id. Two rotations of one cycle come out the same.
func canonicalRotation(cycle []string) []string {
	open := cycle[:len(cycle)-1]
	start := 0
	for index, id := range open {
		if id < open[start] {
			start = index
		}
	}
	rotated := make([]string, 0, len(cycle))
	rotated = append(rotated, open[start:]...)
	rotated = append(rotated, open[:start]...)
	return append(rotated, open[start])
}

// FindCycle reports whether adding the edge from -> to (from depends on to) to edges would close a
// cycle. edges maps each id to the ids it depends on. The cycle comes back as the ids along it,
// starting and ending with from: from, to, ..., from, taking the shortest way back from to to
// from, and visiting each id's dependencies in their order. It returns nil when there is no cycle.
// An edge from an id to itself is the cycle from, from.
func FindCycle(edges map[string][]string, from string, to string) []string {
	if from == to {
		return []string{from, from}
	}

	// A breadth-first search from to for from; reachedFrom records the id each one was reached
	// from, so the way back can be read off once from is reached.
	reachedFrom := map[string]string{to: ""}
	queue := []string{to}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range edges[current] {
			if _, isReached := reachedFrom[next]; isReached {
				continue
			}
			reachedFrom[next] = current
			if next == from {
				return cyclePath(reachedFrom, from, to)
			}
			queue = append(queue, next)
		}
	}
	return nil
}

// cyclePath reads the search's way from to to from back off reachedFrom and returns the cycle
// from, to, ..., from.
func cyclePath(reachedFrom map[string]string, from string, to string) []string {
	reversed := []string{from}
	for current := reachedFrom[from]; current != to; current = reachedFrom[current] {
		reversed = append(reversed, current)
	}
	reversed = append(reversed, to, from)

	cycle := make([]string, 0, len(reversed))
	for index := len(reversed) - 1; index >= 0; index-- {
		cycle = append(cycle, reversed[index])
	}
	return cycle
}
