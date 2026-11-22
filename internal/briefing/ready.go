// Package briefing composes what capcom tells an agent about the work in front of it: the
// Briefing capcom prime renders (Compose) and the Ready-list that capcom ready and prime show
// (ReadyList, and ReadyListJudged with the verdicts behind it), and what ready.hide leaves out of
// it (ReadyHidden).
package briefing

import (
	"cmp"
	"slices"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
)

// ReadyOptions selects the issues ReadyList returns. The zero value keeps every Ready issue that
// carries no ready.hide label.
type ReadyOptions struct {
	// Labels must all be on the issue.
	Labels []string
	// NoLabels must all be absent from the issue.
	NoLabels []string
	// Plan keeps only Planned issues (index.PlanSet), only issues that are not Planned
	// (index.PlanUnset), or both (index.PlanAny). An issue is Planned when its plan resolves to a
	// file under Root, so a plan that names no file counts as unplanned.
	Plan index.PlanState
	// Root is the worktree root the plans directories resolve under; only a Plan other than
	// index.PlanAny reads it.
	Root string
	// IsNotReadyIncluded also keeps the todo issues the ready rules hold back (capcom ready --all).
	IsNotReadyIncluded bool
	// IsHiddenIncluded also keeps the issues carrying a label the config's ready.hide lists.
	IsHiddenIncluded bool
	// Limit caps the number of issues returned, after sorting; 0 returns all of them.
	Limit int
}

// ReadyItem is one issue ReadyList returns and the ready rules' verdict on it.
type ReadyItem struct {
	Entry     index.Entry
	Readiness issue.Readiness
	// Unblocks are the ids of the todo issues that become Ready when this issue alone is closed as
	// done, in tree order; a ready.hide label leaves an issue out. Only a Ready item carries them.
	Unblocks []string
}

// ReadyList judges the todo issues of x against the ready rules and returns the ones options
// select, most urgent first, then the item that unblocks more issues first, then in tree order,
// at most options.Limit of them; an item that is not Ready unblocks none. The rules judge every
// issue, Closed ones and those the filter leaves out included, so a filter never changes a
// verdict; the label filter is the index's own, and the plan filter resolves each plan against
// cfg's plans directories under options.Root. It fails only when the index cannot be read.
func ReadyList(x *index.Index, cfg config.Config, options ReadyOptions) ([]ReadyItem, error) {
	listed, _, err := ReadyListJudged(x, cfg, options)
	return listed, err
}

// Judgement is what the ready rules made of every issue of an index: the verdicts and the
// Dependency graph a Ready-list was judged from, for a caller that needs them for other issues
// than the listed ones and would otherwise judge the whole tree again.
type Judgement struct {
	// Verdicts are issue.Ready's verdicts on the issues of the index, Closed ones and those the
	// filters leave out included, in tree order.
	Verdicts []issue.Readiness
	// Deps is the Dependency graph of every issue of the index.
	Deps issue.DepGraph
}

// ReadyListJudged returns what ReadyList returns, given the same arguments, and the Judgement the
// list was judged from. It fails only when the index cannot be read.
func ReadyListJudged(x *index.Index, cfg config.Config, options ReadyOptions) ([]ReadyItem, Judgement, error) {
	candidates, judgement, err := judgeFiltered(x, cfg, options)
	if err != nil {
		return nil, Judgement{}, err
	}
	var listed []ReadyItem
	for _, candidate := range candidates {
		if options.keeps(candidate.Readiness) {
			listed = append(listed, candidate)
		}
	}

	slices.SortStableFunc(listed, func(first, second ReadyItem) int {
		return cmp.Or(
			cmp.Compare(first.Entry.Priority, second.Entry.Priority),
			cmp.Compare(len(second.Unblocks), len(first.Unblocks)),
		)
	})
	if options.Limit > 0 && len(listed) > options.Limit {
		listed = listed[:options.Limit]
	}
	return listed, judgement, nil
}

// Hidden is what the ready.hide rule alone leaves out of a Ready-list.
type Hidden struct {
	// Count is the number of Ready issues left out.
	Count int
	// Labels are the ready.hide labels those issues carry, sorted and each once; nil when Count
	// is 0.
	Labels []string
}

// ReadyHidden reports the Ready issues of x that ReadyList, given the same options, leaves out
// only because they carry a label cfg's ready.hide lists: the ones the label and plan filters
// keep, judged in the same pass ReadyList makes, so the two never disagree on an issue.
// options.Limit and options.IsNotReadyIncluded change nothing, and options.IsHiddenIncluded
// returns the zero Hidden without reading the index, since then nothing is left out. It fails
// only when the index cannot be read.
func ReadyHidden(x *index.Index, cfg config.Config, options ReadyOptions) (Hidden, error) {
	if options.IsHiddenIncluded {
		return Hidden{}, nil
	}
	candidates, _, err := judgeFiltered(x, cfg, options)
	if err != nil {
		return Hidden{}, err
	}
	var hidden Hidden
	for _, candidate := range candidates {
		if !candidate.Readiness.IsReady || !candidate.Readiness.IsHidden {
			continue
		}
		hidden.Count++
		for _, label := range candidate.Entry.Labels {
			if slices.Contains(cfg.Ready.Hide, label) {
				hidden.Labels = append(hidden.Labels, label)
			}
		}
	}
	slices.Sort(hidden.Labels)
	hidden.Labels = slices.Compact(hidden.Labels)
	return hidden, nil
}

// judgeFiltered judges the todo issues of x against the ready rules and returns, in tree order,
// every one the label and plan filters of options keep, whatever its verdict and whether or not it
// is hidden; a Ready item carries what it unblocks. The rules judge every issue, Closed ones and
// those the filters leave out included, so a filter never changes a verdict; it also returns that
// Judgement. It fails only when the index cannot be read.
func judgeFiltered(x *index.Index, cfg config.Config, options ReadyOptions) ([]ReadyItem, Judgement, error) {
	entries, err := x.List(index.Filter{IsClosedIncluded: true})
	if err != nil {
		return nil, Judgement{}, err
	}
	selectedIDs, err := x.ListIDs(options.filter())
	if err != nil {
		return nil, Judgement{}, err
	}
	selected := make(map[string]bool, len(selectedIDs))
	for _, id := range selectedIDs {
		selected[id] = true
	}

	nodes := index.Nodes(entries)
	byID := make(map[string]index.Entry, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = entry
	}
	judgement := Judgement{Verdicts: issue.Ready(nodes, cfg), Deps: issue.NewDepGraph(nodes)}
	unblocks := unblocksByCloser(judgement.Verdicts, judgement.Deps)
	var candidates []ReadyItem
	for _, verdict := range judgement.Verdicts {
		entry := byID[verdict.ID]
		if !selected[verdict.ID] || !options.keepsPlan(cfg, entry) {
			continue
		}
		item := ReadyItem{Entry: entry, Readiness: verdict}
		if verdict.IsReady {
			item.Unblocks = unblocks[verdict.ID]
		}
		candidates = append(candidates, item)
	}
	return candidates, judgement, nil
}

// unblocksByCloser maps the id of each issue to the ids of the todo issues that become Ready when
// it alone is closed as done, in the order of verdicts. It re-judges from the verdicts alone: a
// close as done can only satisfy dependencies on the closed issue and remove it from its parent's
// open children, never lift a hold or a standing mark, so a todo issue that is not Ready becomes
// Ready exactly when it is not standing, no ancestor holds it and every dependency that blocks it
// and every open child is that one issue. A ready.hide label leaves an issue out, as
// issue.NewlyReady does.
func unblocksByCloser(verdicts []issue.Readiness, graph issue.DepGraph) map[string][]string {
	unblocks := map[string][]string{}
	for _, verdict := range verdicts {
		if verdict.IsReady || verdict.IsStanding || verdict.IsHidden || verdict.HeldBy != "" {
			continue
		}
		if closer, isSole := soleCloser(verdict, graph); isSole {
			unblocks[closer] = append(unblocks[closer], verdict.ID)
		}
	}
	return unblocks
}

// soleCloser returns the one issue whose close as done would make the issue verdict judges Ready,
// and false when there is no such single issue: it is blocked by a dependency that names no issue,
// or by more than one issue.
func soleCloser(verdict issue.Readiness, graph issue.DepGraph) (string, bool) {
	closer := ""
	isSole := func(id string) bool {
		if closer == "" {
			closer = id
		}
		return closer == id
	}
	for _, blocker := range verdict.Blocking {
		id, isFound := graph.Resolve(blocker.Dep)
		if !isFound || !isSole(id) {
			return "", false
		}
	}
	for _, child := range verdict.OpenChildren {
		if !isSole(child) {
			return "", false
		}
	}
	return closer, closer != ""
}

// filter is the index filter for the label options. It includes Closed issues so that only the
// ready rules decide which categories a list holds. The plan option is not the index's: whether a
// plan resolves to a file is known only on disk (keepsPlan).
func (o ReadyOptions) filter() index.Filter {
	return index.Filter{
		Labels:           o.Labels,
		NoLabels:         o.NoLabels,
		IsClosedIncluded: true,
	}
}

// keepsPlan reports whether the plan option keeps entry: index.PlanSet keeps it when it is
// Planned, index.PlanUnset when it is not, and index.PlanAny always.
func (o ReadyOptions) keepsPlan(cfg config.Config, entry index.Entry) bool {
	if o.Plan == index.PlanAny {
		return true
	}
	return isPlanned(cfg, o.Root, entry.Plan) == (o.Plan == index.PlanSet)
}

// isPlanned reports whether an issue with plan is Planned: its plan resolves to a file in one of
// cfg's plans directories under the worktree root. An empty plan, a plan that is not a bare name
// and a plan that names no file are all unplanned.
func isPlanned(cfg config.Config, root, plan string) bool {
	if plan == "" {
		return false
	}
	_, isFound := cfg.PlanPath(root, plan)
	return isFound
}

// keeps reports whether the readiness and hidden options keep the issue verdict judges.
func (o ReadyOptions) keeps(verdict issue.Readiness) bool {
	return (verdict.IsReady || o.IsNotReadyIncluded) && (!verdict.IsHidden || o.IsHiddenIncluded)
}
