package cli

import (
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/issue"
)

// reopenLogPrefix opens the Log entry reopen writes, ahead of the reason.
const reopenLogPrefix = "reopened: "

// newReopenCommand moves a Closed issue back to a status in the todo category (or the one --status
// names), clears its closed time and reason, and appends the reason to its Log.
func (a *app) newReopenCommand() *cobra.Command {
	options := &statusOptions{}
	cmd := &cobra.Command{
		Use:         "reopen <id|alias|prefix> --reason <text>",
		Short:       "Reopen a closed issue, logging why",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
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
			written, err := a.editIssue(args[0], "", func(target issue.Issue) (issue.Issue, error) {
				return a.reopen(target, status, options.reason)
			})
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

// reopen moves target out of its closed category to status and logs reason. An issue that is not
// Closed is refused.
func (a *app) reopen(target issue.Issue, status string, reason string) (issue.Issue, error) {
	category, _ := a.store.Config.CategoryOf(target.Status)
	if !isClosedCategory(category) {
		return issue.Issue{}, Errorf(
			CodeInvalid,
			"status %q is in the %s category, so the issue is not closed; only a closed issue can be reopened",
			target.Status,
			category,
		)
	}
	target.Status = status
	target.Closed = time.Time{}
	target.Reason = ""
	return a.appendLog(target, reopenLogPrefix+reason)
}

// targetStatus returns the status close or reopen moves issues to: the --status given, which must
// be declared and in one of the categories, or else the first status listed in the first category
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
	category, isDeclared := cfg.CategoryOf(given)
	if !isDeclared {
		return "", Errorf(CodeInvalid, "status %q is not declared in config", given)
	}
	if !slices.Contains(categories, category) {
		return "", Errorf(
			CodeInvalid,
			"status %q is in the %s category; %s takes a status in the %v categories",
			given,
			category,
			cmd.Name(),
			categories,
		)
	}
	return given, nil
}
