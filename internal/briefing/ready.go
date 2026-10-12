// Package briefing composes what capcom tells an agent about the work in front of it: the
// Ready-list that capcom ready and prime show.
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
}

// ReadyList judges the todo issues of x against the ready rules and returns the ones options
// select, most urgent first and in tree order within a priority, at most options.Limit of them.
// The rules judge every issue, Closed ones and those the filter leaves out included, so a filter
// never changes a verdict; the label and plan filter is the index's own. It fails only when the
// index cannot be read.
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

	nodes := make([]issue.Node, 0, len(entries))
	byID := make(map[string]index.Entry, len(entries))
	for _, entry := range entries {
		nodes = append(nodes, entry.Node())
		byID[entry.ID] = entry
	}
	var listed []ReadyItem
	for _, verdict := range issue.Ready(nodes, cfg) {
		if selected[verdict.ID] && options.keeps(verdict) {
			listed = append(listed, ReadyItem{Entry: byID[verdict.ID], Readiness: verdict})
		}
	}

	slices.SortStableFunc(listed, func(first, second ReadyItem) int {
		return cmp.Compare(first.Entry.Priority, second.Entry.Priority)
	})
	if options.Limit > 0 && len(listed) > options.Limit {
		listed = listed[:options.Limit]
	}
	return listed, nil
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
