// Package briefing composes what capcom tells an agent about the work in front of it: the
// Briefing capcom prime renders (Compose) and the Ready-list that capcom ready and prime show
// (ReadyList).
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
	// Plan keeps only issues with a plan, only issues without one, or both.
	Plan index.PlanState
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
// verdict; the label and plan filter is the index's own. It fails only when the index cannot be
// read.
func ReadyList(x *index.Index, cfg config.Config, options ReadyOptions) ([]ReadyItem, error) {
	entries, err := x.List(index.Filter{IsClosedIncluded: true})
	if err != nil {
		return nil, err
	}
	selectedIDs, err := x.ListIDs(options.filter())
	if err != nil {
		return nil, err
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
	verdicts := issue.Ready(nodes, cfg)
	unblocks := unblocksByCloser(verdicts, issue.NewDepGraph(nodes))
	var listed []ReadyItem
	for _, verdict := range verdicts {
		if !selected[verdict.ID] || !options.keeps(verdict) {
			continue
		}
		item := ReadyItem{Entry: byID[verdict.ID], Readiness: verdict}
		if verdict.IsReady {
			item.Unblocks = unblocks[verdict.ID]
		}
		listed = append(listed, item)
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
	return listed, nil
}

// unblocksByCloser maps the id of each issue to the ids of the todo issues that become Ready when
// it alone is closed as done, in the order of verdicts. It re-judges from the verdicts alone: a
// close as done can only satisfy dependencies on the closed issue and remove it from its parent's
// open children, never lift a hold, so a todo issue that is not Ready becomes Ready exactly when
// no ancestor holds it and every dependency that blocks it and every open child is that one
// issue. A ready.hide label leaves an issue out, as issue.NewlyReady does.
func unblocksByCloser(verdicts []issue.Readiness, graph issue.DepGraph) map[string][]string {
	unblocks := map[string][]string{}
	for _, verdict := range verdicts {
		if verdict.IsReady || verdict.IsHidden || verdict.HeldBy != "" {
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

// filter is the index filter for the label and plan options. It includes Closed issues so that
// only the ready rules decide which categories a list holds.
func (o ReadyOptions) filter() index.Filter {
	return index.Filter{
		Labels:           o.Labels,
		NoLabels:         o.NoLabels,
		Plan:             o.Plan,
		IsClosedIncluded: true,
	}
}

// keeps reports whether the readiness and hidden options keep the issue verdict judges.
func (o ReadyOptions) keeps(verdict issue.Readiness) bool {
	return (verdict.IsReady || o.IsNotReadyIncluded) && (!verdict.IsHidden || o.IsHiddenIncluded)
}
