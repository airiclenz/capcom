package cli

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
)

// flagShowHidden lists the issues a ready.hide label leaves out.
const flagShowHidden = "show-hidden"

// readyOptions holds the flags of one capcom ready.
type readyOptions struct {
	labels             []string
	noLabels           []string
	isPlanned          bool
	isUnplanned        bool
	isAll              bool
	isShowHidden       bool
	isElsewhereSkipped bool
	limit              int
}

// readyEntry is one listed issue and the ready rules' verdict on it.
type readyEntry struct {
	entry   index.Entry
	verdict issue.Readiness
}

// blockerData is one unsatisfied dependency in the --json data of ready.
type blockerData struct {
	Dep    string `json:"dep"`
	Reason string `json:"reason"`
}

// readyData is one issue in the --json data of ready: its fields, whether it is Ready, and, for a
// todo issue --all lists although it is not Ready, what keeps it back.
type readyData struct {
	issueData
	Ready        bool          `json:"ready"`
	Blocking     []blockerData `json:"blocking,omitempty"`
	OpenChildren []string      `json:"open_children,omitempty"`
	HeldBy       string        `json:"held_by,omitempty"`
}

// newReadyCommand lists the Ready issues, most urgent first and in tree order within a priority.
// A dependency counted as satisfied only because it was dropped is reported as a warning. Issue
// changes on other local branches are reported apart from the list (Elsewhere).
func (a *app) newReadyCommand() *cobra.Command {
	options := &readyOptions{}
	cmd := &cobra.Command{
		Use:         "ready",
		Short:       "List the issues ready to work on, most urgent first",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			if options.limit < 0 {
				return UsageErrorf("--%s %d: must not be negative", flagLimit, options.limit)
			}
			listed, err := a.readyEntries(options)
			if err != nil {
				return err
			}
			result := Result{
				Kind:     "ready",
				Data:     readyListData(listed),
				Text:     readyLines(listed),
				Warnings: droppedDepWarnings(listed),
			}
			return a.emit(cmd, a.addElsewhere(result, options.isElsewhereSkipped, a.issuesPathspec()))
		}),
	}
	flags := cmd.Flags()
	flags.StringSliceVar(&options.labels, flagLabel, nil, "only issues carrying every one of these labels")
	flags.StringSliceVar(&options.noLabels, flagNoLabel, nil, "only issues carrying none of these labels")
	flags.BoolVar(&options.isPlanned, flagPlanned, false, "only issues with a plan")
	flags.BoolVar(&options.isUnplanned, flagUnplanned, false, "only issues without a plan")
	flags.BoolVar(&options.isAll, flagAll, false, "also list todo issues that are not ready, with what blocks them")
	flags.BoolVar(&options.isShowHidden, flagShowHidden, false, "include issues carrying a ready.hide label")
	flags.IntVar(&options.limit, flagLimit, 0, "list at most this many issues (0: all)")
	flags.BoolVar(&options.isElsewhereSkipped, flagNoElsewhere, false, "skip the report of issue changes on other branches")
	cmd.MarkFlagsMutuallyExclusive(flagPlanned, flagUnplanned)
	return cmd
}

// readyEntries judges every issue against the ready rules and returns the ones options select,
// most urgent first, in tree order within a priority, at most options.limit of them.
func (a *app) readyEntries(options *readyOptions) ([]readyEntry, error) {
	entries, err := a.issueIndex.List(index.Filter{IsClosedIncluded: true})
	if err != nil {
		return nil, err
	}
	nodes := make([]issue.Node, 0, len(entries))
	byID := make(map[string]index.Entry, len(entries))
	for _, entry := range entries {
		nodes = append(nodes, issue.Node{
			ID:       entry.ID,
			ParentID: entry.ParentID,
			Category: entry.Category,
			Labels:   entry.Labels,
			Deps:     entry.Deps,
			Aliases:  entry.Aliases,
		})
		byID[entry.ID] = entry
	}

	var listed []readyEntry
	for _, verdict := range issue.Ready(nodes, a.store.Config) {
		entry := byID[verdict.ID]
		if options.selects(entry, verdict) {
			listed = append(listed, readyEntry{entry: entry, verdict: verdict})
		}
	}
	slices.SortStableFunc(listed, func(first, second readyEntry) int {
		return cmp.Compare(first.entry.Priority, second.entry.Priority)
	})
	if options.limit > 0 && len(listed) > options.limit {
		listed = listed[:options.limit]
	}
	return listed, nil
}

// selects reports whether the flags keep entry, given the ready rules' verdict on it.
func (o *readyOptions) selects(entry index.Entry, verdict issue.Readiness) bool {
	switch {
	case !verdict.IsReady && !o.isAll,
		verdict.IsHidden && !o.isShowHidden,
		o.isPlanned && entry.Plan == "",
		o.isUnplanned && entry.Plan != "":
		return false
	}
	for _, label := range o.labels {
		if !slices.Contains(entry.Labels, label) {
			return false
		}
	}
	for _, label := range o.noLabels {
		if slices.Contains(entry.Labels, label) {
			return false
		}
	}
	return true
}

// readyListData converts the listed issues to their --json form, an empty list when there are none.
func readyListData(listed []readyEntry) []readyData {
	data := make([]readyData, 0, len(listed))
	for _, current := range listed {
		item := readyData{
			issueData:    newIssueData(current.entry),
			Ready:        current.verdict.IsReady,
			OpenChildren: current.verdict.OpenChildren,
			HeldBy:       current.verdict.HeldBy,
		}
		for _, blocker := range current.verdict.Blocking {
			item.Blocking = append(item.Blocking, blockerData{Dep: blocker.Dep, Reason: blocker.Reason})
		}
		data = append(data, item)
	}
	return data
}

// readyLines renders one issue line per listed issue, as list does but without indentation; an
// issue that is not Ready ends with what keeps it back.
func readyLines(listed []readyEntry) string {
	var text strings.Builder
	for _, current := range listed {
		line := strings.TrimSuffix(issueLines([]index.Entry{current.entry}, ""), "\n")
		text.WriteString(line)
		if !current.verdict.IsReady {
			text.WriteString("  -- not ready: " + notReadyReasons(current.verdict))
		}
		text.WriteString("\n")
	}
	return text.String()
}

// notReadyReasons describes what keeps an issue from being Ready, e.g.
// `blocked by schema (doing), gone (missing); open children: part; held by epic`.
func notReadyReasons(verdict issue.Readiness) string {
	var reasons []string
	if len(verdict.Blocking) > 0 {
		blockers := make([]string, 0, len(verdict.Blocking))
		for _, blocker := range verdict.Blocking {
			blockers = append(blockers, fmt.Sprintf("%s (%s)", blocker.Dep, blocker.Reason))
		}
		reasons = append(reasons, "blocked by "+strings.Join(blockers, ", "))
	}
	if len(verdict.OpenChildren) > 0 {
		reasons = append(reasons, "open children: "+strings.Join(verdict.OpenChildren, ", "))
	}
	if verdict.HeldBy != "" {
		reasons = append(reasons, "held by "+verdict.HeldBy)
	}
	return strings.Join(reasons, "; ")
}

// droppedDepWarnings warns, for every listed issue, about each dependency counted as satisfied
// only because it was dropped.
func droppedDepWarnings(listed []readyEntry) []string {
	var warnings []string
	for _, current := range listed {
		for _, dep := range current.verdict.DroppedDeps {
			warnings = append(warnings, fmt.Sprintf(
				"%s: dep %q is dropped; counted as satisfied", current.entry.Path, dep,
			))
		}
	}
	return warnings
}
