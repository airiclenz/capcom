package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
)

// Text of capcom dep list.
const (
	depsHeading       = "deps:"
	dependentsHeading = "dependents:"
	depMissingLabel   = "missing"
	depSeeAboveLabel  = "(see above)"
)

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
				return output.UsageErrorf("--%s must be %d (every level) or more", flagDepth, unlimitedDepth)
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

// listResult is the result of dep list for entry, each section walked maxDepth levels deep
// (issue.UnlimitedDepth for all), every level indented one step further. The deps section starts
// with the file's deps verbatim, in its order: a dep naming no issue is marked missing, and only
// the first of a repeated dep carries its branch. The dependents section follows in tree order.
// An issue already printed in its section prints again as `<id>  (see above)` without its branch;
// entry itself, met again on a cycle, prints in full but without a branch. With isCutShown each
// issue whose branch maxDepth cut is followed by the outline's `... <n> more` line.
func (g depGraph) listResult(entry index.Entry, maxDepth int, isCutShown bool) output.Result {
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
	return output.Result{Kind: "deps", Data: data, Text: text}
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
