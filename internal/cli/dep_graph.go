package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
)

// Text of capcom dep graph, a Mermaid flowchart.
const (
	mermaidHeading    = "flowchart LR"
	mermaidIndent     = "    "
	mermaidNodePrefix = "n"
	mermaidArrow      = " --> "
)

// directDepth is the Walk depth that reaches an issue's own deps and dependents alone.
const directDepth = 1

// depGraphData is the --json data of dep graph: its nodes in node order, and its edges.
type depGraphData struct {
	Nodes []depGraphNode `json:"nodes"`
	Edges []depGraphEdge `json:"edges"`
}

// depGraphNode is one issue of dep graph in --json.
type depGraphNode struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Category string `json:"category"`
	Title    string `json:"title"`
}

// depGraphEdge is one Dependency edge of dep graph in --json: dependent depends on dependency.
type depGraphEdge struct {
	Dependent  string `json:"dependent"`
	Dependency string `json:"dependency"`
}

// depGraph is the Dependency graph of every usable issue, with each issue's index entry by id and
// in tree order.
type depGraph struct {
	graph   issue.DepGraph
	entries map[string]index.Entry
	ordered []index.Entry
}

// newDepGraphCommand prints the Dependency graph as a Mermaid flowchart: every issue that is not
// closed, or with an id that issue and its transitive deps and dependents. --all adds closed
// issues; the named issue itself is always a node. A dep naming no issue has no edge.
func (a *app) newDepGraphCommand() *cobra.Command {
	isAll := false
	cmd := &cobra.Command{
		Use:         "graph [id|alias|prefix]",
		Short:       "Print the dependencies as a Mermaid flowchart",
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			var start string
			if len(args) == 1 {
				entry, err := a.resolveIssue(args[0])
				if err != nil {
					return err
				}
				start = entry.ID
			}
			graph, err := a.loadDepGraph()
			if err != nil {
				return err
			}
			return a.emit(cmd, graph.flowchartResult(graph.shownBy(start, isAll)))
		}),
	}
	cmd.Flags().BoolVar(&isAll, flagAll, false, "include closed issues")
	return cmd
}

// loadDepGraph builds the Dependency graph of every usable issue in the index, Closed ones
// included.
func (a *app) loadDepGraph() (depGraph, error) {
	entries, err := a.issueIndex.List(index.Filter{IsClosedIncluded: true})
	if err != nil {
		return depGraph{}, err
	}
	nodes := make([]issue.Node, 0, len(entries))
	byID := make(map[string]index.Entry, len(entries))
	for _, entry := range entries {
		nodes = append(nodes, entry.Node())
		byID[entry.ID] = entry
	}
	return depGraph{graph: issue.NewDepGraph(nodes), entries: byID, ordered: entries}, nil
}

// names returns a test of whether a dep, as a file writes it, names the issue with id.
func (g depGraph) names(id string) func(string) bool {
	return func(dep string) bool {
		resolved, isFound := g.graph.Resolve(dep)
		return isFound && resolved == id
	}
}

// shownBy returns the test of whether an issue is a node of dep graph. With start "" every issue
// that is not closed is; otherwise only start and the issues its deps and dependents reach, walked
// through closed issues too. Closed issues are nodes only with isAll, start always is.
func (g depGraph) shownBy(start string, isAll bool) func(index.Entry) bool {
	isOpenOrAll := func(entry index.Entry) bool { return isAll || !isClosedCategory(entry.Category) }
	if start == "" {
		return isOpenOrAll
	}

	isReached := map[string]bool{start: true}
	for _, direction := range []issue.WalkDirection{issue.WalkDeps, issue.WalkDependents} {
		for _, step := range g.graph.Walk(start, direction, issue.UnlimitedDepth) {
			isReached[step.ID] = true
		}
	}
	return func(entry index.Entry) bool {
		return entry.ID == start || (isReached[entry.ID] && isOpenOrAll(entry))
	}
}

// flowchartResult is the result of dep graph over the issues isShown keeps: `flowchart LR`, a node
// `n<k>["<id> (<status>)"]` per issue in tree order, then an edge `n<a> --> n<b>` from each
// dependency to its dependent, by dependent in tree order and then in the file's order of its
// deps, each edge once. An edge whose either end is not shown is left out.
func (g depGraph) flowchartResult(isShown func(index.Entry) bool) Result {
	data := depGraphData{Nodes: []depGraphNode{}, Edges: []depGraphEdge{}}
	var text strings.Builder
	text.WriteString(mermaidHeading + "\n")

	nodeNames := map[string]string{}
	for _, entry := range g.ordered {
		if !isShown(entry) {
			continue
		}
		name := fmt.Sprintf("%s%d", mermaidNodePrefix, len(data.Nodes)+1)
		nodeNames[entry.ID] = name
		data.Nodes = append(data.Nodes, depGraphNode{
			ID:       entry.ID,
			Status:   entry.Status,
			Category: string(entry.Category),
			Title:    entry.Title,
		})
		fmt.Fprintf(&text, "%s%s[\"%s (%s)\"]\n", mermaidIndent, name, entry.ID, entry.Status)
	}

	for _, node := range data.Nodes {
		// The direct walk yields each resolved dep of the node once, in the file's order.
		for _, step := range g.graph.Walk(node.ID, issue.WalkDeps, directDepth) {
			dependencyName, isNode := nodeNames[step.ID]
			if !isNode {
				continue
			}
			data.Edges = append(data.Edges, depGraphEdge{Dependent: node.ID, Dependency: step.ID})
			text.WriteString(mermaidIndent + dependencyName + mermaidArrow + nodeNames[node.ID] + "\n")
		}
	}
	return Result{Kind: "dep-graph", Data: data, Text: text.String()}
}
