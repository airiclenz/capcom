package cli

import (
	"errors"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/frontmatter"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodefile"
)

// updateOptions holds the flags of one capcom update.
type updateOptions struct {
	title          string
	status         string
	issueType      string
	priority       int
	plan           string
	addLabels      []string
	removeLabels   []string
	bodyAppend     string
	bodyAppendFile string
	replaceOld     string
	replaceNew     string
	ifMatch        string
}

// newUpdateCommand changes the fields and body of one issue. Only the flags given change
// anything; a status change into or out of a closed category is refused, since close and reopen
// own those moves.
func (a *app) newUpdateCommand() *cobra.Command {
	options := &updateOptions{}
	cmd := &cobra.Command{
		Use:         "update <id|alias|prefix>",
		Short:       "Change an issue's fields or body",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			if !hasAnyFlag(cmd, flagTitle, flagStatus, flagType, flagPriority, flagPlan, flagAddLabel,
				flagRemoveLabel, flagBodyAppend, flagBodyAppendFile, flagBodyReplaceOld) {
				return output.UsageErrorf("update needs at least one flag that changes the issue")
			}
			appendFile := ""
			if cmd.Flags().Changed(flagBodyAppendFile) {
				text, err := readTextInput(cmd, options.bodyAppendFile)
				if err != nil {
					return err
				}
				appendFile = text
			}
			written, err := a.editIssue(args[0], options.ifMatch, func(target issue.Issue) (issue.Issue, error) {
				return a.applyUpdate(cmd, options, appendFile, target)
			})
			if err != nil {
				return err
			}
			return a.emit(cmd, writeResult(written.ID, written.Path, written.ETag))
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&options.title, flagTitle, "", "new title")
	flags.StringVar(&options.status, flagStatus, "", "new status, not in a closed category (use close or reopen)")
	flags.StringVar(&options.issueType, flagType, "", "new type")
	flags.IntVar(&options.priority, flagPriority, 0, "new priority, 0 (most urgent) to 4")
	flags.StringVar(&options.plan, flagPlan, "", "new plan basename; empty clears it")
	flags.StringSliceVar(&options.addLabels, flagAddLabel, nil, "labels to add")
	flags.StringSliceVar(&options.removeLabels, flagRemoveLabel, nil, "labels to remove")
	flags.StringVar(&options.bodyAppend, flagBodyAppend, "", "text to append to the body, taken literally")
	flags.StringVar(&options.bodyAppendFile, flagBodyAppendFile, "", "file to append to the body; - reads stdin")
	flags.StringVar(&options.replaceOld, flagBodyReplaceOld, "", "body text to replace; must occur exactly once")
	flags.StringVar(&options.replaceNew, flagBodyReplaceNew, "", "text that replaces --body-replace-old")
	flags.StringVar(&options.ifMatch, flagIfMatch, "", "only write when the file's etag is this one (see show)")
	cmd.MarkFlagsMutuallyExclusive(flagBodyAppend, flagBodyAppendFile)
	cmd.MarkFlagsRequiredTogether(flagBodyReplaceOld, flagBodyReplaceNew)
	return a.withKV(cmd)
}

// applyUpdate applies the changed flags to target: fields first, then labels, then the body
// replacement, then the append.
func (a *app) applyUpdate(
	cmd *cobra.Command,
	options *updateOptions,
	appendFile string,
	target issue.Issue,
) (issue.Issue, error) {
	flags := cmd.Flags()
	if flags.Changed(flagStatus) {
		changed, err := issue.SetStatus(target, a.store.Config, options.status)
		if err != nil {
			return issue.Issue{}, statusChangeFailure(err)
		}
		target = changed
	}
	if flags.Changed(flagTitle) {
		target.Title = options.title
	}
	if flags.Changed(flagType) {
		target.Type = options.issueType
	}
	if flags.Changed(flagPriority) {
		target.Priority = options.priority
	}
	if flags.Changed(flagPlan) {
		target.Plan = options.plan
	}
	target.Labels = edit.Labels(target.Labels, options.addLabels, options.removeLabels)
	if flags.Changed(flagBodyReplaceOld) {
		replaced, err := edit.ReplaceBody(target.Body, options.replaceOld, options.replaceNew)
		if err != nil {
			return issue.Issue{}, output.Errorf(output.CodeInvalid, "%s", err.Error())
		}
		target.Body = replaced
	}
	if flags.Changed(flagBodyAppend) {
		target.Body = edit.AppendBody(target.Body, options.bodyAppend)
	}
	if flags.Changed(flagBodyAppendFile) {
		target.Body = edit.AppendBody(target.Body, appendFile)
	}
	return target, nil
}

// statusChangeFailure types a refusal from issue.SetStatus or issue.Reopen as invalid, worded for
// the commands: the wording stays here because it names the update, close and reopen verbs. Any
// other error, such as a failure to name the Log entry's author, is invalid with its own message.
func statusChangeFailure(err error) error {
	var refusal *issue.StatusError
	if !errors.As(err, &refusal) {
		return output.Errorf(output.CodeInvalid, "%s", err.Error())
	}
	switch {
	case errors.Is(refusal, issue.ErrLeavesClosed):
		return output.Errorf(
			output.CodeInvalid,
			"status %q is closed; use reopen to move the issue out of it",
			refusal.Status,
		)
	case errors.Is(refusal, issue.ErrEntersClosed):
		return output.Errorf(
			output.CodeInvalid,
			"status %q is in the %s category; use close",
			refusal.Status,
			refusal.Category,
		)
	case errors.Is(refusal, issue.ErrNotClosed):
		return output.Errorf(
			output.CodeInvalid,
			"status %q is in the %s category, so the issue is not closed; only a closed issue can be reopened",
			refusal.Status,
			refusal.Category,
		)
	}
	return output.Errorf(output.CodeInvalid, "%s", err.Error())
}

// hasAnyFlag reports whether any of the named flags was given.
func hasAnyFlag(cmd *cobra.Command, names ...string) bool {
	for _, name := range names {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return false
}

// readIssueFile reads the issue file at path, relative to the working tree root with forward
// slashes (an index Entry's Path), through nodefile.ReadNodeFile: a file that is not a regular file or
// is larger than 1 MiB is refused unread, an invalid-input error at path like any failed read.
func (a *app) readIssueFile(path string) ([]byte, error) {
	data, err := nodefile.ReadNodeFile(a.absoluteIssuePath(path))
	if err != nil {
		return nil, &output.Error{Code: output.CodeInvalid, Path: path, Message: err.Error()}
	}
	return data, nil
}

// absoluteIssuePath turns path, relative to the working tree root with forward slashes, into a
// native absolute path.
func (a *app) absoluteIssuePath(path string) string {
	return filepath.Join(a.store.WorktreeRoot, filepath.FromSlash(path))
}

// frontmatterFailure types an issue or Note file that cannot be parsed: invalid, at its file and
// line, both the path and the message naming the file relative to the working tree root. A path
// the parser was given relative, an index entry's, is kept as it is.
func (a *app) frontmatterFailure(err error) error {
	var failure *frontmatter.Error
	if errors.As(err, &failure) {
		return &output.Error{
			Code:    output.CodeInvalid,
			Path:    a.worktreeRelative(failure.Path),
			Line:    failure.Line,
			Message: a.worktreeRelativeText(failure.Message),
		}
	}
	return output.AsError(err)
}
