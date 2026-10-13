package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/briefing"
	"github.com/airiclenz/capcom/internal/cli/output"
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

// blockerData is one unsatisfied dependency in the --json data of ready.
type blockerData struct {
	Dep    string `json:"dep"`
	Reason string `json:"reason"`
}

// readyData is one issue in the --json data of ready: its fields, whether it is Ready, the issues
// it unblocks when it is, and, for a todo issue --all lists although it is not Ready, what keeps it
// back.
type readyData struct {
	issueData
	Ready        bool          `json:"ready"`
	Unblocks     []string      `json:"unblocks,omitempty"`
	Blocking     []blockerData `json:"blocking,omitempty"`
	OpenChildren []string      `json:"open_children,omitempty"`
	HeldBy       string        `json:"held_by,omitempty"`
}

// The help text of capcom ready.
const (
	readyLong = "List the Ready Issues, most urgent first: Issues in the todo category whose dependencies are " +
		"all closed, with no open children and no waiting or dropped ancestor. Each line gives the id, " +
		"status, priority and title, and how many Issues closing it would make Ready. --all also lists " +
		"the todo Issues that are not Ready, with what holds each one back. Issues carrying a label " +
		"listed under ready.hide in the config stay out unless --show-hidden is given. Changes to " +
		"Issues on other branches are reported as Elsewhere unless --no-elsewhere is given."
	readyExample = "  capcom ready                               # what to work on next\n" +
		"  capcom ready --limit 1                     # only the most urgent\n" +
		"  capcom ready --all                         # also the todo issues that are not ready, and why\n" +
		"  capcom ready --label owner-run --unplanned # filtered by label and plan"
)

// newReadyCommand lists the Ready issues, most urgent first, then the one that unblocks more
// issues first, then in tree order.
// A dependency counted as satisfied only because it was dropped is reported as a warning. Issue
// changes on other local and remote-tracking branches are reported apart from the list
// (Elsewhere).
func (a *app) newReadyCommand() *cobra.Command {
	options := &readyOptions{}
	cmd := &cobra.Command{
		Use:         "ready",
		Short:       "List the issues ready to work on, most urgent first",
		Long:        readyLong,
		Example:     readyExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			if options.limit < 0 {
				return output.UsageErrorf("--%s %d: must not be negative", flagLimit, options.limit)
			}
			listed, err := briefing.ReadyList(a.issueIndex, a.store.Config, options.listOptions())
			if err != nil {
				return err
			}
			result := output.Result{
				Kind:     "ready",
				Data:     readyListData(listed),
				Text:     readyLines(listed),
				Warnings: droppedDepWarnings(listed),
			}
			return a.emit(cmd, a.addElsewhere(result, options.isElsewhereSkipped, a.store.IssuesPathspec()))
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

// listOptions converts the flags to the options of the Ready-list.
func (o *readyOptions) listOptions() briefing.ReadyOptions {
	plan := index.PlanAny
	switch {
	case o.isPlanned:
		plan = index.PlanSet
	case o.isUnplanned:
		plan = index.PlanUnset
	}
	return briefing.ReadyOptions{
		Labels:             o.labels,
		NoLabels:           o.noLabels,
		Plan:               plan,
		IsNotReadyIncluded: o.isAll,
		IsHiddenIncluded:   o.isShowHidden,
		Limit:              o.limit,
	}
}

// readyListData converts the listed issues to their --json form, an empty list when there are none.
func readyListData(listed []briefing.ReadyItem) []readyData {
	data := make([]readyData, 0, len(listed))
	for _, current := range listed {
		item := readyData{
			issueData:    newIssueData(current.Entry),
			Ready:        current.Readiness.IsReady,
			Unblocks:     current.Unblocks,
			OpenChildren: current.Readiness.OpenChildren,
			HeldBy:       current.Readiness.HeldBy,
		}
		for _, blocker := range current.Readiness.Blocking {
			item.Blocking = append(item.Blocking, blockerData{Dep: blocker.Dep, Reason: blocker.Reason})
		}
		data = append(data, item)
	}
	return data
}

// readyLines renders one issue line per listed issue, as list does but without indentation; a
// Ready issue that unblocks others ends with how many, and an issue that is not Ready ends with
// what keeps it back.
func readyLines(listed []briefing.ReadyItem) string {
	var text strings.Builder
	for _, current := range listed {
		line := strings.TrimSuffix(issueLines([]index.Entry{current.Entry}, ""), "\n")
		text.WriteString(line)
		if len(current.Unblocks) > 0 {
			fmt.Fprintf(&text, "  -- unblocks %d", len(current.Unblocks))
		}
		if !current.Readiness.IsReady {
			text.WriteString("  -- not ready: " + notReadyReasons(current.Readiness))
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
func droppedDepWarnings(listed []briefing.ReadyItem) []string {
	var warnings []string
	for _, current := range listed {
		for _, dep := range current.Readiness.DroppedDeps {
			warnings = append(warnings, fmt.Sprintf(
				"%s: dep %q is dropped; counted as satisfied", current.Entry.Path, dep,
			))
		}
	}
	return warnings
}
