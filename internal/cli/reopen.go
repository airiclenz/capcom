package cli

import (
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/config"
)

// The help text of capcom reopen.
const (
	reopenLong = "Reopen a closed Issue and append the reason to its Log; --reason is required.\n\n" +
		"The Issue moves to the first todo status, or to the todo, doing or waiting status --status " +
		"names.\n\n" +
		"It prints the Issue's id, path and new etag."
	reopenExample = "  capcom reopen login-timeout-race --reason \"Came back on Safari\"\n" +
		"  capcom reopen login-timeout-race --status active --reason \"Picking it up again now\""
)

// newReopenCommand moves a Closed issue back to a status in the todo category (or the one --status
// names), clears its closed time and reason, and appends the reason to its Log.
func (a *app) newReopenCommand() *cobra.Command {
	options := &statusOptions{}
	cmd := &cobra.Command{
		Use:               "reopen <id|alias|prefix> --reason <text>",
		Short:             "Reopen a closed issue, logging why",
		Long:              reopenLong,
		Example:           reopenExample,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeIssueIDs(1),
		Annotations:       map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			if err := requireReason(cmd, options.reason); err != nil {
				return err
			}
			status, err := targetStatus(
				cmd,
				options.status,
				a.store.Config,
				config.CategoryTodo,
				config.CategoryDoing,
				config.CategoryWaiting,
			)
			if err != nil {
				return err
			}
			reopened, err := a.ops().Reopen(args[0], "", status, options.reason, time.Now())
			written, err := a.written(issueIDs, reopened, err)
			if err != nil {
				return err
			}
			return a.emit(cmd, writeResult(written.ID, written.Path, written.ETag))
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&options.reason, flagReason, "", "why the issue is reopened (required)")
	flags.StringVar(&options.status, flagStatus, "", "status that is not closed (default: the first todo status)")
	return a.withKV(cmd)
}

// targetStatus returns the status close or reopen moves issues to: the --status given, which must
// be declared (a refusal lists the declared statuses) and in one of the categories, or else the first status listed in the first category
// (config.DefaultStatus). The config always declares a todo and a done status.
func targetStatus(
	cmd *cobra.Command,
	given string,
	cfg config.Config,
	categories ...config.Category,
) (string, error) {
	if !cmd.Flags().Changed(flagStatus) {
		status, _ := cfg.DefaultStatus(categories[0])
		return status, nil
	}
	if err := invalidValues(cfg).status(given); err != nil {
		return "", err
	}
	category, _ := cfg.CategoryOf(given)
	if !slices.Contains(categories, category) {
		return "", output.Errorf(
			output.CodeInvalid,
			"status %q is in the %s category; %s takes a status in the %v categories",
			given,
			category,
			cmd.Name(),
			categories,
		)
	}
	return given, nil
}
