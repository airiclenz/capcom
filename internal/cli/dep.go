package cli

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/tree"
)

// Text of capcom dep list.
const (
	depsHeading       = "deps:"
	dependentsHeading = "dependents:"
	depMissingLabel   = "missing"
	depSeeAboveLabel  = "(see above)"
	cycleArrow        = " -> "
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

// defaultDepListDepth is the --depth of dep list when none is given: the direct deps and
// dependents only.
const defaultDepListDepth = 1

// depListData is the --json data of dep list: the issue, the lines of its deps section that name
// an issue, the deps that name none (as the file writes them), and the lines of its dependents
// section.
type depListData struct {
	ID         string        `json:"id"`
	Deps       []depListItem `json:"deps"`
	Missing    []string      `json:"missing"`
	Dependents []depListItem `json:"dependents"`
}

// depListItem is one issue line of a dep list section in --json: the issue, how many edges it lies
// from the listed issue (depth) and the id of the issue it hangs under (via, "" at depth 1). An
// issue printed again as (see above) is an item again.
type depListItem struct {
	issueData
	Depth int    `json:"depth"`
	Via   string `json:"via"`
}

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

// newDepCommand groups the commands that add, remove, list and graph Dependency edges. dep add and dep
// rm change an existing issue's deps and create --dep writes a new issue's; every edge is stored
// on the dependent, under the canonical id of the issue it depends on.
func (a *app) newDepCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dep",
		Short: "Manage dependencies between issues",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(a.newDepAddCommand())
	cmd.AddCommand(a.newDepGraphCommand())
	cmd.AddCommand(a.newDepListCommand())
	cmd.AddCommand(a.newDepRmCommand())
	return cmd
}

// newDepAddCommand makes one issue depend on another. A self-dependency and an edge that would
// close a cycle are refused; an edge that is already there is left as it is.
func (a *app) newDepAddCommand() *cobra.Command {
	return a.withKV(&cobra.Command{
		Use:         "add <id|alias|prefix> <dep-id|alias|prefix>",
		Short:       "Make an issue depend on another",
		Args:        cobra.ExactArgs(2),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			dependent, dependency, err := a.resolveEdge(args)
			if err != nil {
				return err
			}
			if dependent.ID == dependency.ID {
				return &Error{
					Code:    CodeInvalid,
					Path:    dependent.Path,
					Message: fmt.Sprintf("%s cannot depend on itself", dependent.ID),
				}
			}
			graph, err := a.loadDepGraph()
			if err != nil {
				return err
			}
			isPresent := slices.ContainsFunc(dependent.Deps, graph.names(dependency.ID))
			if cycle := graph.graph.FindCycle(dependent.ID, dependency.ID); !isPresent && cycle != nil {
				return &Error{
					Code: CodeInvalid,
					Path: dependent.Path,
					Message: fmt.Sprintf(
						"%s cannot depend on %s: that would create the cycle %s",
						dependent.ID,
						dependency.ID,
						strings.Join(cycle, cycleArrow),
					),
				}
			}
			written, err := a.editIssue(dependent.ID, "", func(target issue.Issue) (issue.Issue, error) {
				if !isPresent {
					target.Deps = append(slices.Clone(target.Deps), dependency.ID)
				}
				return target, nil
			})
			if err != nil {
				return err
			}
			return a.emit(cmd, writeResult(written.ID, written.Path, written.ETag))
		}),
	})
}

// newDepRmCommand removes every dep of an issue that names the given one, whether the file writes
// it as the id or as an Alias. An argument that is not exactly an issue's id or Alias removes the
// dep that is its text, before it is matched as a prefix; one naming no issue at all is matched
// only by its text.
func (a *app) newDepRmCommand() *cobra.Command {
	return a.withKV(&cobra.Command{
		Use:         "rm <id|alias|prefix> <dep-id|alias|prefix>",
		Short:       "Remove a dependency of an issue",
		Args:        cobra.ExactArgs(2),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			dependent, err := a.resolveIssue(args[0])
			if err != nil {
				return err
			}
			graph, err := a.loadDepGraph()
			if err != nil {
				return err
			}
			isRemoved, err := a.depRemoval(graph, dependent.Deps, args[1])
			if err != nil {
				return err
			}
			written, err := a.editIssue(dependent.ID, "", func(target issue.Issue) (issue.Issue, error) {
				kept := slices.DeleteFunc(slices.Clone(target.Deps), isRemoved)
				if len(kept) == len(target.Deps) {
					return issue.Issue{}, Errorf(CodeNotFound, "%s does not depend on %s", dependent.ID, args[1])
				}
				if len(kept) == 0 {
					kept = nil
				}
				target.Deps = kept
				return target, nil
			})
			if err != nil {
				return err
			}
			return a.emit(cmd, writeResult(written.ID, written.Path, written.ETag))
		}),
	})
}

// depRemoval returns the test a dep of the dependent must pass to be removed by dep rm: it names
// the issue arg resolves to, or it is arg's text (isEntryText) when isRemovedByText says so.
func (a *app) depRemoval(graph depGraph, deps []string, arg string) (func(string) bool, error) {
	dependency, err := a.issueIndex.Resolve(arg)
	if graph.isRemovedByText(deps, arg, err) {
		return isEntryText(arg), nil
	}
	if err != nil {
		failure, _ := resolveFailure(err) // either way the command stops here
		return nil, failure
	}
	return graph.names(dependency.ID), nil
}

// newDepListCommand lists an issue's dependencies and the issues that depend on it, each as an
// indented tree --depth levels deep (0 for every level). A given --depth also prints a
// `... <n> more (--depth <d>)` line under each issue whose branch it cut; without one the output
// is the direct deps and dependents alone.
func (a *app) newDepListCommand() *cobra.Command {
	depth := defaultDepListDepth
	cmd := &cobra.Command{
		Use:         "list <id|alias|prefix>",
		Short:       "List an issue's dependencies and dependents",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			if depth < unlimitedDepth {
				return UsageErrorf("--%s must be %d (every level) or more", flagDepth, unlimitedDepth)
			}
			entry, err := a.resolveIssue(args[0])
			if err != nil {
				return err
			}
			graph, err := a.loadDepGraph()
			if err != nil {
				return err
			}
			return a.emit(cmd, graph.listResult(entry, depth, cmd.Flags().Changed(flagDepth)))
		}),
	}
	cmd.Flags().IntVar(&depth, flagDepth, defaultDepListDepth,
		"levels of deps and dependents to print; 0 prints every level")
	return cmd
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

// resolveEdge resolves the two arguments of dep add: the dependent issue, then the one it is to
// depend on.
func (a *app) resolveEdge(args []string) (dependent index.Entry, dependency index.Entry, err error) {
	if dependent, err = a.resolveIssue(args[0]); err != nil {
		return index.Entry{}, index.Entry{}, err
	}
	if dependency, err = a.resolveIssue(args[1]); err != nil {
		return index.Entry{}, index.Entry{}, err
	}
	return dependent, dependency, nil
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

// isEntryText returns a test of whether an entry of a deps or related list is arg's text: arg as
// typed or as tree.NormalizeArg spells it.
func isEntryText(arg string) func(string) bool {
	name := tree.NormalizeArg(arg)
	return func(entry string) bool { return entry == arg || entry == name }
}

// isRemovedByText reports whether dep rm or related rm removes the entries of a holder's list that
// are arg's text (isEntryText), given resolveErr, what issueIndex.Resolve made of arg. It does when
// arg names no issue. When arg is a prefix, of one issue's id or of several, it does when one of
// entries is arg's text, so a dangling entry is never mistaken for the issue it prefixes. An arg
// that is exactly a usable issue's id or Alias, and a name a duplicate leaves out, never are.
func (g depGraph) isRemovedByText(entries []string, arg string, resolveErr error) bool {
	switch {
	case errors.Is(resolveErr, tree.ErrNotFound):
		return true
	case errors.Is(resolveErr, tree.ErrAmbiguous):
		return slices.ContainsFunc(entries, isEntryText(arg))
	case resolveErr != nil:
		return false
	}
	if _, isExact := g.graph.Resolve(tree.NormalizeArg(arg)); isExact {
		return false
	}
	return slices.ContainsFunc(entries, isEntryText(arg))
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

// listResult is the result of dep list for entry, each section walked maxDepth levels deep
// (issue.UnlimitedDepth for all), every level indented one step further. The deps section starts
// with the file's deps verbatim, in its order: a dep naming no issue is marked missing, and only
// the first of a repeated dep carries its branch. The dependents section follows in tree order.
// An issue already printed in its section prints again as `<id>  (see above)` without its branch;
// entry itself, met again on a cycle, prints in full but without a branch. With isCutShown each
// issue whose branch maxDepth cut is followed by the outline's `... <n> more` line.
func (g depGraph) listResult(entry index.Entry, maxDepth int, isCutShown bool) Result {
	data := depListData{ID: entry.ID, Missing: []string{}}
	deps := newDepSection(g, entry.ID, isCutShown)
	dependents := newDepSection(g, entry.ID, isCutShown)

	branches := depBranches(g.graph.Walk(entry.ID, issue.WalkDeps, maxDepth))
	for _, dep := range entry.Deps {
		id, isFound := g.graph.Resolve(dep)
		if !isFound {
			data.Missing = append(data.Missing, dep)
			deps.rows = append(deps.rows, outlineRow{depth: 1, text: flatten(dep) + "  " + depMissingLabel})
			continue
		}
		branch, isFirst := branches[id]
		if !isFirst {
			// The file names this issue again, by id or by alias; the first carried its branch.
			deps.add(issue.WalkStep{ID: id, Via: entry.ID, Depth: 1})
			continue
		}
		delete(branches, id)
		for _, step := range branch {
			deps.add(step)
		}
	}

	for _, step := range g.graph.Walk(entry.ID, issue.WalkDependents, maxDepth) {
		dependents.add(step)
	}

	data.Deps, data.Dependents = deps.items, dependents.items
	text := depsHeading + "\n" + renderOutline(deps.rows) + dependentsHeading + "\n" + renderOutline(dependents.rows)
	return Result{Kind: "deps", Data: data, Text: text}
}

// depBranches splits the steps of a walk at its depth-1 steps: each depth-1 issue's id maps to
// its step followed by the steps of its branch, in walk order.
func depBranches(steps []issue.WalkStep) map[string][]issue.WalkStep {
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

// depSection collects the outline rows and --json items of one dep list section.
type depSection struct {
	graph depGraph
	// start is the id of the listed issue.
	start      string
	isCutShown bool
	rows       []outlineRow
	items      []depListItem
}

// newDepSection returns an empty section of the dep list of start.
func newDepSection(graph depGraph, start string, isCutShown bool) *depSection {
	return &depSection{graph: graph, start: start, isCutShown: isCutShown, items: []depListItem{}}
}

// add appends the line of one walk step: the issue line at the step's depth, or `<id>  (see
// above)` for an issue the section already printed, and its cut line when the section shows them.
func (s *depSection) add(step issue.WalkStep) {
	entry := s.graph.entries[step.ID]
	via := step.Via
	if step.Depth == 1 {
		via = ""
	}
	s.items = append(s.items, depListItem{issueData: newIssueData(entry), Depth: step.Depth, Via: via})

	row := outlineRow{depth: step.Depth, text: strings.TrimSuffix(issueLines([]index.Entry{entry}, ""), "\n")}
	if step.IsRepeat && step.ID != s.start {
		row.text = step.ID + "  " + depSeeAboveLabel
	}
	if s.isCutShown && step.Cut > 0 {
		row.more, row.moreDepth = step.Cut, step.Depth+1
	}
	s.rows = append(s.rows, row)
}
