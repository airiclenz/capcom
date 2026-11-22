package cli

import (
	"errors"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/ops"
	"github.com/airiclenz/capcom/internal/outline"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/tree"
)

// Flags of capcom list.
const (
	flagCategory = "category"
	flagTree     = "tree"
)

// contextMarker ends the text line of a list --tree row that prints only as the start of the
// outline or as an ancestor of an issue the row filters match.
const contextMarker = "  [context]"

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
		"levels deep. With --tree the filters but --limit and --parent (which starts the outline) keep " +
		"the matching Issues and their ancestors, an ancestor that does not match ending in [context]; " +
		"--status and --category decide by themselves whether closed Issues match, the other " +
		"filters follow --all.\n\n" +
		"Given ids, it prints one line per id, in the order given: the Issue whose exact id or Alias it is, " +
		"closed ones included, or the id marked not found, so one call answers each id's status and whether " +
		"an id is still free; the exit code is 0 either way.\n\n" +
		"A prefix of an id is not found here; an id only an invalid file claims is marked unusable with " +
		"that file's path, and one that two files claim duplicate. Ids take no flag but --no-elsewhere.\n\n" +
		"Changes to Issues on other branches are reported as Elsewhere unless --no-elsewhere is given."
	listExample = "  capcom list                     # every issue that is not closed\n" +
		"  capcom list login-flow new-idea # each id's status, or not found\n" +
		"  capcom list --label owner-run   # open issues carrying a label\n" +
		"  capcom list --all --type bug    # every bug, closed ones included\n" +
		"  capcom list --tree --depth 2    # the outline, two levels deep\n" +
		"  capcom list --tree --status open  # open issues, ancestors marked [context]"
)

// newListCommand lists issues in tree order, one line each. Without flags it lists every issue
// that is not Closed. With --tree it prints the issue outline instead (see issueTreeResult).
// Issue changes on other local and remote-tracking branches are reported apart from the list
// (Elsewhere).
func (a *app) newListCommand() *cobra.Command {
	options := &listOptions{depth: unlimitedDepth}
	cmd := &cobra.Command{
		Use:               "list [<id>...]",
		Short:             "List issues; without flags, every issue that is not closed",
		Long:              listLong,
		Example:           listExample,
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeIssueIDs(everyArgument),
		Annotations:       map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			var result output.Result
			var err error
			if len(args) > 0 {
				result, err = a.listNamedResult(cmd, args)
			} else {
				result, err = a.listFlaggedResult(cmd, options)
			}
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
		"print the issue outline: each parent with its closed/total children, closed issues left out unless --all; "+
			"with row filters, the matches and their ancestors")
	flags.IntVar(&options.depth, flagDepth, unlimitedDepth,
		"with --tree, levels below the top (or below --parent) to print; 0 prints every level")
	cmd.MarkFlagsMutuallyExclusive(flagPlanned, flagUnplanned)
	return cmd
}

// listFlaggedResult lists the issues the flags select, after checkTreeFlags.
func (a *app) listFlaggedResult(cmd *cobra.Command, options *listOptions) (output.Result, error) {
	if err := checkTreeFlags(cmd, options); err != nil {
		return output.Result{}, err
	}
	return a.listResult(options)
}

// checkTreeFlags rejects --depth without --tree, a negative --depth, and --limit with --tree.
func checkTreeFlags(cmd *cobra.Command, options *listOptions) error {
	flags := cmd.Flags()
	if !options.isTree {
		if flags.Changed(flagDepth) {
			return output.UsageErrorf("--%s needs --%s", flagDepth, flagTree)
		}
		return nil
	}
	if flags.Changed(flagLimit) {
		return output.UsageErrorf("--%s cannot be combined with --%s", flagLimit, flagTree)
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

// listNamedResult lists the issues args name, one row per argument in argument order: the usual
// issue row, unindented, for the usable issue whose exact id or Alias the argument is, Closed ones
// included, and a missingIssueData row otherwise (see lookupNamedIssue). A name no issue answers to
// is a row, never a failure. Every flag but --no-elsewhere is refused alongside ids.
func (a *app) listNamedResult(cmd *cobra.Command, args []string) (output.Result, error) {
	if err := checkNamedFlags(cmd); err != nil {
		return output.Result{}, err
	}
	data := make([]any, 0, len(args))
	var text strings.Builder
	for _, arg := range args {
		entry, missing, err := a.lookupNamedIssue(arg)
		if err != nil {
			return output.Result{}, err
		}
		if missing != nil {
			data = append(data, *missing)
			text.WriteString(missingIssueLine(*missing))
			continue
		}
		data = append(data, newIssueData(entry))
		text.WriteString(issueLines([]index.Entry{entry}, ""))
	}
	return output.Result{Kind: "issue-list", Data: data, Text: text.String()}, nil
}

// checkNamedFlags rejects every flag of capcom list but --no-elsewhere given alongside ids.
func checkNamedFlags(cmd *cobra.Command) error {
	var refused string
	cmd.LocalNonPersistentFlags().VisitAll(func(flag *pflag.Flag) {
		if refused == "" && flag.Changed && flag.Name != flagNoElsewhere {
			refused = flag.Name
		}
	})
	if refused != "" {
		return output.UsageErrorf("--%s cannot be combined with ids", refused)
	}
	return nil
}

// lookupNamedIssue finds the usable issue whose id or one of whose Aliases is exactly arg, once
// tree.NormalizeArg has turned a citation spelling into a name; a prefix of an id does not match.
// Otherwise it returns the row that says why: duplicate when two files claim the name or it stands
// for a repeated id, unusable naming the file when only an unusable file claims it, and not found
// when nothing does.
func (a *app) lookupNamedIssue(arg string) (index.Entry, *missingIssueData, error) {
	entry, err := a.issueIndex.Resolve(arg)
	var unusable *tree.UnusableError
	switch {
	case err == nil:
		name := tree.NormalizeArg(arg)
		if entry.ID == name || slices.Contains(entry.Aliases, name) {
			return entry, nil, nil
		}
		return index.Entry{}, &missingIssueData{ID: arg, Status: missingNotFound}, nil
	case errors.As(err, &unusable):
		return index.Entry{}, &missingIssueData{ID: arg, Status: missingUnusable, Path: unusable.Path}, nil
	case errors.Is(err, tree.ErrDuplicate):
		return index.Entry{}, &missingIssueData{ID: arg, Status: missingDuplicate}, nil
	case errors.Is(err, tree.ErrNotFound), errors.Is(err, tree.ErrAmbiguous):
		return index.Entry{}, &missingIssueData{ID: arg, Status: missingNotFound}, nil
	default:
		return index.Entry{}, nil, err
	}
}

// listFilter checks the flags against the config and turns them into an index filter: the row
// filters (see rowFilter), --limit and --parent.
func (a *app) listFilter(options *listOptions) (index.Filter, error) {
	filter, err := a.rowFilter(options)
	if err != nil {
		return index.Filter{}, err
	}
	filter.Limit = options.limit
	if options.parent != "" {
		parent, err := a.resolveIssue(options.parent)
		if err != nil {
			return index.Filter{}, err
		}
		filter.ParentID = parent.ID
	}
	return filter, nil
}

// rowFilter checks the flags against the config and turns the row filters into an index filter,
// without --parent and --limit. An explicit --status or --category decides by itself whether
// Closed issues are listed; otherwise --all does.
func (a *app) rowFilter(options *listOptions) (index.Filter, error) {
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
	return filter, nil
}

// hasRowFilter reports whether any flag that picks rows by the issue's own fields is given:
// --status, --category, --label, --no-label, --type, --priority, --plan, --planned or --unplanned.
func (o *listOptions) hasRowFilter() bool {
	return len(o.statuses) > 0 || len(o.categories) > 0 || len(o.labels) > 0 || len(o.noLabels) > 0 ||
		len(o.types) > 0 || len(o.priorities) > 0 || o.plan != "" || o.isPlanned || o.isUnplanned
}

// checkListValues rejects flag values that could never match: an undeclared status or type, an
// unknown category, a priority out of range, a negative limit. Each refusal but the limit's lists the
// values its flag accepts.
func (a *app) checkListValues(options *listOptions) error {
	cfg := a.store.Config
	for _, status := range options.statuses {
		if err := ops.CheckStatus(cfg, status); err != nil {
			return valueFailure(err, output.UsageErrorf)
		}
	}
	for _, category := range options.categories {
		if err := checkCategory(category); err != nil {
			return err
		}
	}
	for _, issueType := range options.types {
		if err := ops.CheckType(cfg, issueType); err != nil {
			return valueFailure(err, output.UsageErrorf)
		}
	}
	for _, priority := range options.priorities {
		if err := ops.CheckPriority(priority); err != nil {
			return valueFailure(err, output.UsageErrorf)
		}
	}
	if options.limit < 0 {
		return output.UsageErrorf("--%s %d: must not be negative", flagLimit, options.limit)
	}
	return nil
}

// issueTreeItem is one line of list --tree in --json: the issue, its indentation depth, the count of
// its direct children the text's `(<closed>/<total> closed)` gives (left out without children),
// the count of the `... <n> more` line --depth cut under it (left out when nothing was cut) and
// whether the row prints only as context, the text's `[context]` (left out on a match).
type issueTreeItem struct {
	issueData
	Depth     int           `json:"depth"`
	Children  *childrenData `json:"children,omitempty"`
	More      int           `json:"more,omitempty"`
	IsContext bool          `json:"context,omitempty"`
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
// top (or below --parent), with a `... <n> more (--depth <d>)` line under each issue it cut. With a
// row filter the outline keeps only the issues it matches and their ancestors (see issueOutline).
func (a *app) issueTreeResult(options *listOptions) (output.Result, error) {
	entries, err := a.issueIndex.List(index.Filter{IsClosedIncluded: true})
	if err != nil {
		return output.Result{}, err
	}
	tree, err := a.issueOutline(entries, options)
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
	issues := &issueTree{tree: tree, items: []issueTreeItem{}}
	for _, row := range issues.tree.Rows(start, options.depth) {
		issues.add(row)
	}
	return output.Result{Kind: "issue-list", Data: issues.items, Text: renderOutline(issues.rows)}, nil
}

// issueOutline returns the outline list --tree prints over entries. Without a row filter Closed
// issues and their subtrees are hidden unless --all. With one, every issue the index matches and
// its ancestors are shown, Closed ones included, and a shown ancestor that does not match is a
// context row; which Closed issues match is the filter's IsClosedIncluded.
func (a *app) issueOutline(entries []index.Entry, options *listOptions) (*outline.Tree[index.Entry], error) {
	if !options.hasRowFilter() {
		return outline.Issues(entries, options.isAll), nil
	}
	filter, err := a.rowFilter(options)
	if err != nil {
		return nil, err
	}
	ids, err := a.issueIndex.ListIDs(filter)
	if err != nil {
		return nil, err
	}
	matched := make(map[string]bool, len(ids))
	for _, id := range ids {
		matched[id] = true
	}
	return outline.Issues(entries, true).Matching(matched), nil
}

// add appends the line of row's issue, cutting the children --depth cut and marking a context row.
func (t *issueTree) add(row outline.Row[index.Entry]) {
	text := strings.TrimSuffix(issueLines([]index.Entry{row.Entry}, ""), "\n")
	progress := progressOf(t.tree.Children(row.ID))
	if progress != nil {
		text += "  (" + childrenText(progress) + ")"
	}
	if row.IsContext {
		text += contextMarker
	}
	t.rows = append(t.rows, outlineRow{depth: row.Depth, text: text, more: row.More, moreDepth: row.MoreDepth})
	t.items = append(t.items, issueTreeItem{
		issueData: newIssueData(row.Entry),
		Depth:     row.Depth,
		Children:  progress,
		More:      row.More,
		IsContext: row.IsContext,
	})
}
