package cli

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/issue"
)

// Flags of close and reopen; --status is shared with list and update.
const flagReason = "reason"

// Outcomes of closing one issue, the STATUS of its --kv record.
const (
	outcomeClosed        = "closed"
	outcomeAlreadyClosed = "already_closed"
)

// reopenLogPrefix opens the Log entry reopen writes, ahead of the reason.
const reopenLogPrefix = "reopened: "

// statusOptions holds the flags of one close or reopen.
type statusOptions struct {
	reason string
	status string
}

// closeData is one argument of close in the --json data: the issue it resolved to (the argument
// itself when it resolved to none) and what closing it did. Path and ETag are set when the issue
// was found; ETag is the file's etag after the command.
type closeData struct {
	ID      string `json:"id"`
	Outcome string `json:"outcome"`
	Path    string `json:"path,omitempty"`
	ETag    string `json:"etag,omitempty"`
}

// newCloseCommand closes each named issue: it moves the issue to a status in the done or dropped
// category and records the closed time and the reason. Every argument is handled on its own; an
// issue already Closed is reported and left as it is, and an argument that names no issue does not
// stop the others, though the command then exits with its code.
func (a *app) newCloseCommand() *cobra.Command {
	options := &statusOptions{}
	cmd := &cobra.Command{
		Use:         "close <id|alias|prefix>... --reason <text>",
		Short:       "Close issues, recording why",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			if err := requireReason(cmd, options.reason); err != nil {
				return err
			}
			status, err := targetStatus(
				cmd,
				options.status,
				a.store.Config,
				config.CategoryDone,
				config.CategoryDropped,
			)
			if err != nil {
				return err
			}
			result, err := a.closeResult(args, status, options.reason)
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&options.reason, flagReason, "", "why the issues are closed (required)")
	flags.StringVar(&options.status, flagStatus, "", "status in the done or dropped category (default: the first done status)")
	return cmd
}

// closeResult closes every argument's issue and builds the result: a --json item, a text line and a
// --kv record per argument, and the joined failure of the arguments that name no issue.
func (a *app) closeResult(args []string, status string, reason string) (Result, error) {
	result := Result{Kind: "close", KV: []KVRecord{}}
	data := make([]closeData, 0, len(args))
	var text strings.Builder
	var messages []string
	closedAt := time.Now().UTC().Truncate(time.Second)
	isHandled := map[string]bool{}

	for _, arg := range args {
		outcome, failure, err := a.closeOne(arg, status, reason, closedAt, isHandled)
		if err != nil {
			return Result{}, err
		}
		if failure != nil {
			if result.Failure == nil {
				result.Failure = failure
			}
			messages = append(messages, failure.Error())
			data = append(data, closeData{ID: arg, Outcome: string(failure.Code)})
			result.KV = append(result.KV, KVRecord{Pairs: []KVPair{{"ID", arg}}, Status: string(failure.Code)})
			continue
		}
		data = append(data, outcome)
		fmt.Fprintf(&text, "%s  %s\n", outcome.ID, outcome.Outcome)
		result.KV = append(result.KV, KVRecord{Pairs: []KVPair{{"ID", outcome.ID}}, Status: outcome.Outcome})
	}
	if result.Failure != nil {
		result.Failure = Errorf(result.Failure.Code, "%s", strings.Join(messages, failureSeparator))
	}
	result.Data = data
	result.Text = text.String()
	return result, nil
}

// closeOne closes the issue arg names. failure is set when arg names no usable issue or its file
// cannot be written; err is set only when the index fails, which stops the command.
func (a *app) closeOne(
	arg string,
	status string,
	reason string,
	closedAt time.Time,
	isHandled map[string]bool,
) (outcome closeData, failure *Error, err error) {
	entry, err := a.issueIndex.Resolve(arg)
	if err != nil {
		resolveErr, isArgument := resolveFailure(err)
		if !isArgument {
			return closeData{}, nil, resolveErr
		}
		return closeData{}, resolveErr, nil
	}

	outcome = closeData{ID: entry.ID, Path: entry.Path}
	if isHandled[entry.ID] || isClosedCategory(entry.Category) {
		file, err := a.readIssueFile(entry.Path)
		if err != nil {
			return closeData{}, asError(err), nil
		}
		outcome.Outcome = outcomeAlreadyClosed
		outcome.ETag = edit.ETag(file)
		return outcome, nil, nil
	}
	writeData, err := a.editIssue(entry.ID, "", func(target issue.Issue) (issue.Issue, error) {
		target.Status = status
		target.Closed = closedAt
		target.Reason = reason
		return target, nil
	})
	if err != nil {
		return closeData{}, asError(err), nil
	}
	isHandled[entry.ID] = true
	written := writeData.Data.(map[string]string)
	outcome.Outcome = outcomeClosed
	outcome.ETag = written["etag"]
	return outcome, nil, nil
}

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
			result, err := a.editIssue(args[0], "", func(target issue.Issue) (issue.Issue, error) {
				return a.reopen(target, status, options.reason)
			})
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&options.reason, flagReason, "", "why the issue is reopened (required)")
	flags.StringVar(&options.status, flagStatus, "", "status that is not closed (default: the first todo status)")
	return cmd
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

// requireReason refuses a close or reopen without a reason: a missing --reason is a usage error, a
// blank one invalid.
func requireReason(cmd *cobra.Command, reason string) error {
	if !cmd.Flags().Changed(flagReason) {
		return UsageErrorf("%s needs --%s <text>", cmd.Name(), flagReason)
	}
	if strings.TrimSpace(reason) == "" {
		return Errorf(CodeInvalid, "--%s is empty; say why", flagReason)
	}
	return nil
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
