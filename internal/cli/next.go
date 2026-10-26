package cli

import (
	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/briefing"
	"github.com/airiclenz/capcom/internal/cli/output"
)

// nothingReadyMessage is what next prints when no Issue is Ready.
const nothingReadyMessage = "nothing is ready to work on"

// nextLimit is how many Issues of the Ready list next takes: the first.
const nextLimit = 1

// The help text of capcom next.
const (
	nextLong = "Print the Issue on the first line of capcom ready in full, as capcom show prints it: the " +
		"most urgent Ready Issue, the one that unblocks more Issues first among equals.\n\n" +
		"--label, --no-label, --planned, --unplanned and --show-hidden select as they do for ready.\n\n" +
		"--json gives show's envelope for that Issue.\n\n" +
		"With nothing Ready it prints \"" + nothingReadyMessage + "\" and exits 4, a code used for " +
		"nothing else; --json then gives kind next with data null.\n\n" +
		"Changes on other branches are reported as Elsewhere unless --no-elsewhere is given."
	nextExample = "  capcom next                    # the Issue to work on now, in full\n" +
		"  capcom next --label auth       # the top Ready Issue labelled auth\n" +
		"  capcom next --unplanned        # the top Ready Issue without a plan\n" +
		"  capcom next --json             # show's JSON envelope for it"
)

// newNextCommand prints the Issue on the first line of ready, given the same filters, as show
// prints it. With nothing Ready it prints so and exits with output.ExitNothingReady.
func (a *app) newNextCommand() *cobra.Command {
	options := &readyOptions{limit: nextLimit}
	cmd := &cobra.Command{
		Use:         "next",
		Short:       "Show the top Ready Issue in full",
		Long:        nextLong,
		Example:     nextExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			listed, err := briefing.ReadyList(a.issueIndex, a.store.Config, options.listOptions())
			if err != nil {
				return err
			}
			if len(listed) == 0 {
				return a.emitNothingReady(cmd, options.isElsewhereSkipped)
			}
			return a.emitShown(cmd, []string{listed[0].Entry.ID}, options.isElsewhereSkipped)
		}),
	}
	flags := cmd.Flags()
	flags.StringSliceVar(&options.labels, flagLabel, nil, "only issues carrying every one of these labels")
	flags.StringSliceVar(&options.noLabels, flagNoLabel, nil, "only issues carrying none of these labels")
	flags.BoolVar(&options.isPlanned, flagPlanned, false, "only issues with a plan")
	flags.BoolVar(&options.isUnplanned, flagUnplanned, false, "only issues without a plan")
	flags.BoolVar(&options.isShowHidden, flagShowHidden, false, "include issues carrying a ready.hide label")
	flags.BoolVar(&options.isElsewhereSkipped, flagNoElsewhere, false, "skip the report of issue changes on other branches")
	cmd.MarkFlagsMutuallyExclusive(flagPlanned, flagUnplanned)
	return a.withKV(cmd)
}

// emitNothingReady renders the result of a next that found no Ready Issue: the message as text, a
// null --json data and no --kv records, with the Elsewhere report of the whole issue tree as ready
// gives it. It then returns the failure that exits with output.ExitNothingReady, marked rendered.
func (a *app) emitNothingReady(cmd *cobra.Command, isElsewhereSkipped bool) error {
	result := output.Result{
		Kind: "next",
		Data: []showData(nil), // a typed nil, so the envelope carries "data": null
		Text: nothingReadyMessage + "\n",
		KV:   []output.KVRecord{},
	}
	if err := a.emit(cmd, a.addElsewhere(result, isElsewhereSkipped, a.store.IssuesPathspec())); err != nil {
		return err
	}
	return output.Rendered(output.NothingReadyErrorf("%s", nothingReadyMessage))
}
