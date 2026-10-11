package cli

import (
	"slices"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
)

// Flags of capcom list.
const (
	flagStatus    = "status"
	flagCategory  = "category"
	flagLabel     = "label"
	flagNoLabel   = "no-label"
	flagType      = "type"
	flagPriority  = "priority"
	flagParent    = "parent"
	flagPlan      = "plan"
	flagPlanned   = "planned"
	flagUnplanned = "unplanned"
	flagAll       = "all"
	flagLimit     = "limit"
)

// statusCategories are the categories --category accepts (ADR 0003).
var statusCategories = []config.Category{
	config.CategoryTodo, config.CategoryDoing, config.CategoryWaiting, config.CategoryDone, config.CategoryDropped,
}

// listOptions holds the flags of one capcom list.
type listOptions struct {
	statuses    []string
	categories  []string
	labels      []string
	noLabels    []string
	types       []string
	priorities  []int
	parent      string
	plan        string
	isPlanned   bool
	isUnplanned bool
	isAll       bool
	limit       int
}

// newListCommand lists issues in tree order, one line each. Without flags it lists every issue
// that is not Closed.
func (a *app) newListCommand() *cobra.Command {
	options := &listOptions{}
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List issues; without flags, every issue that is not closed",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			filter, err := a.listFilter(options)
			if err != nil {
				return err
			}
			entries, err := a.issueIndex.List(filter)
			if err != nil {
				return err
			}
			return a.emit(cmd, Result{
				Kind: "issue-list",
				Data: issueListData(entries),
				Text: issueLines(entries, relativeSlashPath(a.store.WorktreeRoot, a.store.IssuesDir())),
			})
		}),
	}
	flags := cmd.Flags()
	flags.StringSliceVar(&options.statuses, flagStatus, nil, "only issues with one of these statuses (includes closed ones)")
	flags.StringSliceVar(&options.categories, flagCategory, nil, "only issues in one of these categories (includes closed ones)")
	flags.StringSliceVar(&options.labels, flagLabel, nil, "only issues carrying every one of these labels")
	flags.StringSliceVar(&options.noLabels, flagNoLabel, nil, "only issues carrying none of these labels")
	flags.StringSliceVar(&options.types, flagType, nil, "only issues of one of these types")
	flags.IntSliceVar(&options.priorities, flagPriority, nil, "only issues with one of these priorities")
	flags.StringVar(&options.parent, flagParent, "", "only the direct children of this issue (id, alias or prefix)")
	flags.StringVar(&options.plan, flagPlan, "", "only issues whose plan contains this text")
	flags.BoolVar(&options.isPlanned, flagPlanned, false, "only issues with a plan")
	flags.BoolVar(&options.isUnplanned, flagUnplanned, false, "only issues without a plan")
	flags.BoolVar(&options.isAll, flagAll, false, "include closed issues")
	flags.IntVar(&options.limit, flagLimit, 0, "list at most this many issues (0: all)")
	cmd.MarkFlagsMutuallyExclusive(flagPlanned, flagUnplanned)
	return cmd
}

// listFilter checks the flags against the config and turns them into an index filter. An explicit
// --status or --category decides by itself whether Closed issues are listed.
func (a *app) listFilter(options *listOptions) (index.Filter, error) {
	if err := a.checkListValues(options); err != nil {
		return index.Filter{}, err
	}
	filter := index.Filter{
		Statuses:         options.statuses,
		Labels:           options.labels,
		NoLabels:         options.noLabels,
		Types:            options.types,
		Priorities:       options.priorities,
		PlanContains:     options.plan,
		IsClosedIncluded: options.isAll || len(options.statuses) > 0 || len(options.categories) > 0,
		Limit:            options.limit,
	}
	for _, category := range options.categories {
		filter.Categories = append(filter.Categories, config.Category(category))
	}
	switch {
	case options.isPlanned:
		filter.Plan = index.PlanSet
	case options.isUnplanned:
		filter.Plan = index.PlanUnset
	}
	if options.parent != "" {
		parent, err := a.resolveIssue(options.parent)
		if err != nil {
			return index.Filter{}, err
		}
		filter.ParentID = parent.ID
	}
	return filter, nil
}

// checkListValues rejects flag values that could never match: an undeclared status or type, an
// unknown category, a priority out of range, a negative limit.
func (a *app) checkListValues(options *listOptions) error {
	cfg := a.store.Config
	for _, status := range options.statuses {
		if _, isDeclared := cfg.CategoryOf(status); !isDeclared {
			return UsageErrorf("--%s %q: not a status declared in config.yaml", flagStatus, status)
		}
	}
	for _, category := range options.categories {
		if !slices.Contains(statusCategories, config.Category(category)) {
			return UsageErrorf("--%s %q: must be one of %v", flagCategory, category, statusCategories)
		}
	}
	for _, issueType := range options.types {
		if !slices.Contains(cfg.Types, issueType) {
			return UsageErrorf("--%s %q: not a type declared in config.yaml", flagType, issueType)
		}
	}
	for _, priority := range options.priorities {
		if priority < issue.MinPriority || priority > issue.MaxPriority {
			return UsageErrorf("--%s %d: must be %d to %d", flagPriority, priority, issue.MinPriority, issue.MaxPriority)
		}
	}
	if options.limit < 0 {
		return UsageErrorf("--%s %d: must not be negative", flagLimit, options.limit)
	}
	return nil
}
