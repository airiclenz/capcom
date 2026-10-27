package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/outline"
	"github.com/airiclenz/capcom/internal/store"
)

// Flags of capcom list.
const (
	flagCategory = "category"
	flagTree     = "tree"
)

// treeRowFilters are the flags of capcom list that pick rows and so cannot be combined with
// --tree: the outline keeps every issue below its start, --all deciding only about Closed ones.
var treeRowFilters = []string{
	flagStatus, flagCategory, flagLabel, flagNoLabel, flagType, flagPriority,
	flagPlan, flagPlanned, flagUnplanned, flagLimit,
}

// statusCategories are the categories --category accepts (ADR 0003).
var statusCategories = []config.Category{
	config.CategoryTodo, config.CategoryDoing, config.CategoryWaiting, config.CategoryDone, config.CategoryDropped,
}

// listOptions holds the flags of one capcom list.
type listOptions struct {
	statuses           []string
	categories         []string
	labels             []string
	noLabels           []string
	types              []string
	priorities         []int
	parent             string
	plan               string
	isPlanned          bool
	isUnplanned        bool
	isAll              bool
	isElsewhereSkipped bool
	limit              int
	isTree             bool
	depth              int
}

// The help text of capcom list.
const (
	listLong = "List Issues one per line with id, status, priority and title, each child indented below its " +
		"Parent.\n\n" +
		"Without flags it lists every Issue that is not closed; --all includes closed ones, and the " +
		"filters (--status, --category, --label, --type, --priority, --parent, --plan and more) narrow " +
		"the list.\n\n" +
		"--tree prints the Issue outline instead: each Parent with its closed/total children, --depth " +
		"levels deep.\n\n" +
		"Changes to Issues on other branches are reported as Elsewhere unless --no-elsewhere is given."
	listExample = "  capcom list                     # every issue that is not closed\n" +
		"  capcom list --label owner-run   # open issues carrying a label\n" +
		"  capcom list --all --type bug    # every bug, closed ones included\n" +
		"  capcom list --tree --depth 2    # the outline, two levels deep"
)

// newListCommand lists issues in tree order, one line each. Without flags it lists every issue
// that is not Closed. With --tree it prints the issue outline instead (see issueTreeResult).
// Issue changes on other local and remote-tracking branches are reported apart from the list
// (Elsewhere).
func (a *app) newListCommand() *cobra.Command {
	options := &listOptions{depth: unlimitedDepth}
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List issues; without flags, every issue that is not closed",
		Long:        listLong,
		Example:     listExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			if err := checkTreeFlags(cmd, options); err != nil {
				return err
			}
			result, err := a.listResult(options)
			if err != nil {
				return err
			}
			return a.emit(cmd, a.addElsewhere(result, options.isElsewhereSkipped, a.store.IssuesPathspec()))
		}),
	}
	flags := cmd.Flags()
	flags.StringSliceVar(&options.statuses, flagStatus, nil, "only issues with one of these statuses (includes closed ones)")
	a.completeFlag(cmd, flagStatus, commaSeparated(completeStatuses(isAnyCategory)))
	flags.StringSliceVar(&options.categories, flagCategory, nil, "only issues in one of these categories (includes closed ones)")
	a.completeFlag(cmd, flagCategory, commaSeparated(completeCategories))
	flags.StringSliceVar(&options.labels, flagLabel, nil, "only issues carrying every one of these labels")
	flags.StringSliceVar(&options.noLabels, flagNoLabel, nil, "only issues carrying none of these labels")
	flags.StringSliceVar(&options.types, flagType, nil, "only issues of one of these types")
	a.completeFlag(cmd, flagType, commaSeparated(completeTypes))
	flags.IntSliceVar(&options.priorities, flagPriority, nil, "only issues with one of these priorities")
	flags.StringVar(&options.parent, flagParent, "",
		"only the direct children of this issue (id, alias or prefix); with --tree, this issue first, then its subtree")
	a.completeFlag(cmd, flagParent, completeIDValue(issueIDs))
	flags.StringVar(&options.plan, flagPlan, "", "only issues whose plan contains this text")
	flags.BoolVar(&options.isPlanned, flagPlanned, false, "only issues whose plan field is set")
	flags.BoolVar(&options.isUnplanned, flagUnplanned, false, "only issues whose plan field is not set")
	flags.BoolVar(&options.isAll, flagAll, false, "include closed issues")
	flags.IntVar(&options.limit, flagLimit, 0, "list at most this many issues (0: all)")
	flags.BoolVar(&options.isElsewhereSkipped, flagNoElsewhere, false, "skip the report of issue changes on other branches")
	flags.BoolVar(&options.isTree, flagTree, false,
		"print the issue outline: each parent with its closed/total children, closed issues left out unless --all")
	flags.IntVar(&options.depth, flagDepth, unlimitedDepth,
		"with --tree, levels below the top (or below --parent) to print; 0 prints every level")
	cmd.MarkFlagsMutuallyExclusive(flagPlanned, flagUnplanned)
	return cmd
}

// checkTreeFlags rejects --depth without --tree, a negative --depth, and any row filter with --tree.
func checkTreeFlags(cmd *cobra.Command, options *listOptions) error {
	flags := cmd.Flags()
	if !options.isTree {
		if flags.Changed(flagDepth) {
			return output.UsageErrorf("--%s needs --%s", flagDepth, flagTree)
		}
		return nil
	}
	for _, name := range treeRowFilters {
		if flags.Changed(name) {
			return output.UsageErrorf("--%s cannot be combined with --%s", name, flagTree)
		}
	}
	if options.depth < unlimitedDepth {
		return output.UsageErrorf("--%s must be %d (every level) or more", flagDepth, unlimitedDepth)
	}
	return nil
}

// listResult lists the issues options select, one line each, or with --tree their outline.
func (a *app) listResult(options *listOptions) (output.Result, error) {
	if options.isTree {
		return a.issueTreeResult(options)
	}
	filter, err := a.listFilter(options)
	if err != nil {
		return output.Result{}, err
	}
	entries, err := a.issueIndex.List(filter)
	if err != nil {
		return output.Result{}, err
	}
	issuesPrefix, isInside := store.RelativePath(a.store.WorktreeRoot, a.store.IssuesDir())
	if !isInside {
		issuesPrefix = ""
	}
	return output.Result{
		Kind: "issue-list",
		Data: issueListData(entries),
		Text: issueLines(entries, issuesPrefix),
	}, nil
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
// unknown category, a priority out of range, a negative limit. Each refusal but the limit's lists the
// values its flag accepts.
func (a *app) checkListValues(options *listOptions) error {
	values := usageValues(a.store.Config)
	for _, status := range options.statuses {
		if err := values.status(status); err != nil {
			return err
		}
	}
	for _, category := range options.categories {
		if err := values.category(category); err != nil {
			return err
		}
	}
	for _, issueType := range options.types {
		if err := values.issueType(issueType); err != nil {
			return err
		}
	}
	for _, priority := range options.priorities {
		if err := values.priority(priority); err != nil {
			return err
		}
	}
	if options.limit < 0 {
		return output.UsageErrorf("--%s %d: must not be negative", flagLimit, options.limit)
	}
	return nil
}

// issueTreeItem is one line of list --tree in --json: the issue, its indentation depth, the count of
// its direct children the text's `(<closed>/<total> closed)` gives (left out without children) and
// the count of the `... <n> more` line --depth cut under it (left out when nothing was cut).
type issueTreeItem struct {
	issueData
	Depth    int           `json:"depth"`
	Children *childrenData `json:"children,omitempty"`
	More     int           `json:"more,omitempty"`
}

// issueTree is the issue hierarchy list --tree walks: every usable issue grouped under its parent,
// Closed ones included so they count in their parent's progress, and the outline built so far.
type issueTree struct {
	tree  *outline.Tree[index.Entry]
	rows  []outlineRow
	items []issueTreeItem
}

// issueTreeResult prints the issue outline: one line per issue, `id  status  P<n>  title`,
// indented two spaces per level, a parent's line ending `  (<closed>/<total> closed)` over all its
// direct children. Without --all, Closed issues and their subtrees are left out. With --parent
// that issue prints first, at depth 0, followed by its subtree. --depth limits the levels below the
// top (or below --parent), with a `... <n> more (--depth <d>)` line under each issue it cut.
func (a *app) issueTreeResult(options *listOptions) (output.Result, error) {
	entries, err := a.issueIndex.List(index.Filter{IsClosedIncluded: true})
	if err != nil {
		return output.Result{}, err
	}
	var start *index.Entry
	if options.parent != "" {
		resolved, err := a.resolveIssue(options.parent)
		if err != nil {
			return output.Result{}, err
		}
		start = &resolved
	}
	issues := &issueTree{tree: outline.Issues(entries, options.isAll), items: []issueTreeItem{}}
	for _, row := range issues.tree.Rows(start, options.depth) {
		issues.add(row)
	}
	return output.Result{Kind: "issue-list", Data: issues.items, Text: renderOutline(issues.rows)}, nil
}

// add appends the line of row's issue, cutting the children --depth cut.
func (t *issueTree) add(row outline.Row[index.Entry]) {
	text := strings.TrimSuffix(issueLines([]index.Entry{row.Entry}, ""), "\n")
	progress := progressOf(t.tree.Children(row.ID))
	if progress != nil {
		text += "  (" + childrenText(progress) + ")"
	}
	t.rows = append(t.rows, outlineRow{depth: row.Depth, text: text, more: row.More, moreDepth: row.MoreDepth})
	t.items = append(t.items, issueTreeItem{
		issueData: newIssueData(row.Entry),
		Depth:     row.Depth,
		Children:  progress,
		More:      row.More,
	})
}
