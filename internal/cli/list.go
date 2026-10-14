package cli

import (
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
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
		"Parent. Without flags it lists every Issue that is not closed; --all includes closed ones, and " +
		"the filters (--status, --category, --label, --type, --priority, --parent, --plan and more) " +
		"narrow the list. --tree prints the Issue outline instead: each Parent with its closed/total " +
		"children, --depth levels deep. Changes to Issues on other branches are reported as Elsewhere " +
		"unless --no-elsewhere is given."
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
	flags.StringSliceVar(&options.categories, flagCategory, nil, "only issues in one of these categories (includes closed ones)")
	flags.StringSliceVar(&options.labels, flagLabel, nil, "only issues carrying every one of these labels")
	flags.StringSliceVar(&options.noLabels, flagNoLabel, nil, "only issues carrying none of these labels")
	flags.StringSliceVar(&options.types, flagType, nil, "only issues of one of these types")
	flags.IntSliceVar(&options.priorities, flagPriority, nil, "only issues with one of these priorities")
	flags.StringVar(&options.parent, flagParent, "",
		"only the direct children of this issue (id, alias or prefix); with --tree, this issue first, then its subtree")
	flags.StringVar(&options.plan, flagPlan, "", "only issues whose plan contains this text")
	flags.BoolVar(&options.isPlanned, flagPlanned, false, "only issues with a plan")
	flags.BoolVar(&options.isUnplanned, flagUnplanned, false, "only issues without a plan")
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
// unknown category, a priority out of range, a negative limit.
func (a *app) checkListValues(options *listOptions) error {
	cfg := a.store.Config
	for _, status := range options.statuses {
		if _, isDeclared := cfg.CategoryOf(status); !isDeclared {
			return output.UsageErrorf("--%s %q: not a status declared in config.yaml", flagStatus, status)
		}
	}
	for _, category := range options.categories {
		if !slices.Contains(statusCategories, config.Category(category)) {
			return output.UsageErrorf("--%s %q: must be one of %v", flagCategory, category, statusCategories)
		}
	}
	for _, issueType := range options.types {
		if !slices.Contains(cfg.Types, issueType) {
			return output.UsageErrorf("--%s %q: not a type declared in config.yaml", flagType, issueType)
		}
	}
	for _, priority := range options.priorities {
		if priority < issue.MinPriority || priority > issue.MaxPriority {
			return output.UsageErrorf("--%s %d: must be %d to %d", flagPriority, priority, issue.MinPriority, issue.MaxPriority)
		}
	}
	if options.limit < 0 {
		return output.UsageErrorf("--%s %d: must not be negative", flagLimit, options.limit)
	}
	return nil
}

// issueTreeItem is one line of list --tree in --json: the issue and its indentation depth.
type issueTreeItem struct {
	issueData
	Depth int `json:"depth"`
}

// issueTree is the issue hierarchy list --tree walks: every usable issue grouped under its parent,
// Closed ones included so they count in their parent's progress, and the outline built so far.
type issueTree struct {
	tree  *visibleTree[index.Entry]
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
	isShown := func(entry index.Entry) bool {
		return options.isAll || !entry.Category.IsClosed()
	}
	outline := &issueTree{
		tree:  newVisibleTree(entries, issueID, issueParentID, nil, isShown),
		items: []issueTreeItem{},
	}
	outline.tree.walk(start, options.depth, outline.add)
	return output.Result{Kind: "issue-list", Data: outline.items, Text: renderOutline(outline.rows)}, nil
}

// issueID returns the id of entry, the key its children are grouped under.
func issueID(entry index.Entry) string {
	return entry.ID
}

// issueParentID returns the id of the parent of entry, "" for a top-level issue.
func issueParentID(entry index.Entry) string {
	return entry.ParentID
}

// add appends the line of entry at indent, cutting more children that --depth moreDepth would show.
func (t *issueTree) add(entry index.Entry, indent int, more int, moreDepth int) {
	text := strings.TrimSuffix(issueLines([]index.Entry{entry}, ""), "\n")
	if progress := progressOf(t.tree.all(entry.ID)); progress != nil {
		text += "  (" + childrenText(progress) + ")"
	}
	t.rows = append(t.rows, outlineRow{depth: indent, text: text, more: more, moreDepth: moreDepth})
	t.items = append(t.items, issueTreeItem{issueData: newIssueData(entry), Depth: indent})
}
