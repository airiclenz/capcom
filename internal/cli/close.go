package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodefile"
)

// Flags of close and reopen; --status is shared with list and update.
const flagReason = "reason"

// Outcomes of closing one issue, the STATUS of its --kv record.
const (
	outcomeClosed        = "closed"
	outcomeAlreadyClosed = "already_closed"
)

// statusOptions holds the flags of one close or reopen.
type statusOptions struct {
	reason string
	status string
}

// closeData is one argument of close in the --json data: the issue it resolved to (the argument
// itself when it resolved to none) and what closing it did. Path and ETag are set when the issue
// was found; ETag is the file's etag after the command. NowReady names the todo issues this
// argument's close made Ready, given the closes of the arguments before it; it is never null.
type closeData struct {
	ID       string   `json:"id"`
	Outcome  string   `json:"outcome"`
	Path     string   `json:"path,omitempty"`
	ETag     string   `json:"etag,omitempty"`
	NowReady []string `json:"now_ready"`
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
				config.ClosedCategories()...,
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
	return a.withKV(cmd)
}

// closeResult closes every argument's issue and builds the result: a --json item, a text line and a
// --kv record per argument, and the joined failure of the arguments that name no issue. The text
// ends with a "now ready:" line naming the todo issues the closes made Ready, when there are any.
func (a *app) closeResult(args []string, status string, reason string) (output.Result, error) {
	// The index is not refreshed after the writes, so the issues are read once, before them.
	nodes, err := a.issueNodes()
	if err != nil {
		return output.Result{}, err
	}
	closedCategory, _ := a.store.Config.CategoryOf(status)
	closing := map[string]config.Category{}
	isReported := map[string]bool{}
	var nowReady []string

	result := output.Result{Kind: "close", KV: []output.KVRecord{}}
	data := make([]closeData, 0, len(args))
	var text strings.Builder
	var messages []string
	closedAt := time.Now().UTC().Truncate(time.Second)
	isHandled := map[string]bool{}

	for _, arg := range args {
		outcome, failure, err := a.closeOne(arg, status, reason, closedAt, isHandled)
		if err != nil {
			return output.Result{}, err
		}
		if failure != nil {
			if result.Failure == nil {
				result.Failure = failure
			}
			messages = append(messages, failure.Error())
			data = append(data, closeData{ID: arg, Outcome: string(failure.Code), NowReady: []string{}})
			result.KV = append(result.KV, output.KVRecord{Pairs: []output.KVPair{{Key: "ID", Value: arg}}, Status: string(failure.Code)})
			continue
		}
		outcome.NowReady = []string{}
		if outcome.Outcome == outcomeClosed {
			closing[outcome.ID] = closedCategory
			for _, id := range issue.NewlyReady(nodes, a.store.Config, closing) {
				if !isReported[id] {
					isReported[id] = true
					outcome.NowReady = append(outcome.NowReady, id)
				}
			}
			nowReady = append(nowReady, outcome.NowReady...)
		}
		data = append(data, outcome)
		fmt.Fprintf(&text, "%s  %s\n", outcome.ID, outcome.Outcome)
		result.KV = append(result.KV, output.KVRecord{Pairs: []output.KVPair{{Key: "ID", Value: outcome.ID}}, Status: outcome.Outcome})
	}
	if len(nowReady) > 0 {
		fmt.Fprintf(&text, "now ready: %s\n", strings.Join(nowReady, ", "))
	}
	if result.Failure != nil {
		result.Failure = output.Errorf(result.Failure.Code, "%s", strings.Join(messages, failureSeparator))
	}
	result.Data = data
	result.Text = text.String()
	return result, nil
}

// issueNodes returns the Node of every usable issue in the index, Closed ones included.
func (a *app) issueNodes() ([]issue.Node, error) {
	entries, err := a.issueIndex.List(index.Filter{IsClosedIncluded: true})
	if err != nil {
		return nil, err
	}
	nodes := make([]issue.Node, 0, len(entries))
	for _, entry := range entries {
		nodes = append(nodes, entry.Node())
	}
	return nodes, nil
}

// closeOne closes the issue arg names. failure is set when arg names no usable issue or its file
// cannot be written; err is set only when the index fails, which stops the command.
func (a *app) closeOne(
	arg string,
	status string,
	reason string,
	closedAt time.Time,
	isHandled map[string]bool,
) (outcome closeData, failure *output.Error, err error) {
	entry, err := a.issueIndex.Resolve(arg)
	if err != nil {
		resolveErr, isArgument := resolveFailure(err)
		if !isArgument {
			return closeData{}, nil, resolveErr
		}
		return closeData{}, resolveErr, nil
	}

	outcome = closeData{ID: entry.ID, Path: entry.Path}
	if isHandled[entry.ID] || entry.Category.IsClosed() {
		file, err := a.readIssueFile(entry.Path)
		if err != nil {
			return closeData{}, output.AsError(err), nil
		}
		outcome.Outcome = outcomeAlreadyClosed
		outcome.ETag = nodefile.ETag(file)
		return outcome, nil, nil
	}
	written, err := a.editIssue(entry.ID, "", func(target issue.Issue) (issue.Issue, error) {
		target.Status = status
		target.Closed = closedAt
		target.Reason = reason
		return target, nil
	})
	if err != nil {
		return closeData{}, output.AsError(err), nil
	}
	isHandled[entry.ID] = true
	outcome.Outcome = outcomeClosed
	outcome.ETag = written.ETag
	return outcome, nil, nil
}

// requireReason refuses a close or reopen without a reason: a missing --reason is a usage error, a
// blank one invalid.
func requireReason(cmd *cobra.Command, reason string) error {
	if !cmd.Flags().Changed(flagReason) {
		return output.UsageErrorf("%s needs --%s <text>", cmd.Name(), flagReason)
	}
	if strings.TrimSpace(reason) == "" {
		return output.Errorf(output.CodeInvalid, "--%s is empty; say why", flagReason)
	}
	return nil
}
