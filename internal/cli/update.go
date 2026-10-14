package cli

import (
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/frontmatter"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodestore"
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

// The help text of capcom update.
const (
	updateLong = "Change an Issue's fields, labels or body in place; at least one flag that changes the Issue " +
		"is required. --status moves it between statuses that are not closed: close and reopen cross into " +
		"and out of the closed categories. --body-append and --body-append-file (- reads stdin) add to " +
		"the end of the body, and --body-replace-old with --body-replace-new replaces text that must " +
		"occur exactly once. --if-match refuses the write when the file no longer has the etag show " +
		"printed. It prints the Issue's id, path and new etag."
	updateExample = "  capcom update login-timeout-race --priority 1 --add-label owner-run\n" +
		"  capcom update login-timeout-race --status active   # claim it: a status in the doing category\n" +
		"  capcom update login-timeout-race --body-append \"Seen on Safari only.\"\n" +
		"  capcom update login-timeout-race --body-replace-old \"Safari only\" --body-replace-new \"Safari and Firefox\""
)

// newUpdateCommand changes the fields and body of one issue. Only the flags given change
// anything; a status change into or out of a closed category is refused, since close and reopen
// own those moves.
func (a *app) newUpdateCommand() *cobra.Command {
	options := &updateOptions{}
	cmd := &cobra.Command{
		Use:         "update <id|alias|prefix>",
		Short:       "Change an issue's fields or body",
		Long:        updateLong,
		Example:     updateExample,
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
	target.Labels = edit.List(target.Labels, options.addLabels, options.removeLabels)
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

// refusedReadOp is the Op of the *fs.PathError nodefile.ReadRegularFile refuses a node file with,
// the words a refused read fails in.
const refusedReadOp = "read"

// readNodeFile reads the node file at path, relative to the working tree root with forward slashes
// (an index entry's Path), through reader, the Kind reader of its kind. A file that cannot be read,
// refused unread (not a regular file, or larger than 1 MiB) or not, is an invalid-input error at
// path, worded as nodefile.ReadRegularFile's *fs.PathError naming the file. A file that does not
// parse fails nothing here: its Reading carries the finding.
func readNodeFile[T any](a *app, reader nodestore.Reader[T], path string) (nodestore.Reading[T], error) {
	absolute := a.absoluteIssuePath(path)
	reading, err := reader.Read(absolute, path)
	if err == nil && !reading.IsRead() {
		refusal := errors.New(reading.Refusal[0].Message)
		err = &fs.PathError{Op: refusedReadOp, Path: absolute, Err: refusal}
	}
	if err != nil {
		return nodestore.Reading[T]{}, &output.Error{Code: output.CodeInvalid, Path: path, Message: err.Error()}
	}
	return reading, nil
}

// parseNodeFile reads the node file at path through reader as readNodeFile does, and fails as
// invalid at its file and line when the file does not parse. A field or alias finding fails
// nothing: the Reading keeps it, and the index already leaves such a file out.
func parseNodeFile[T any](a *app, reader nodestore.Reader[T], path string) (nodestore.Reading[T], error) {
	reading, err := readNodeFile(a, reader, path)
	if err != nil {
		return nodestore.Reading[T]{}, err
	}
	if !reading.IsParsed() {
		return nodestore.Reading[T]{}, a.parseFailure(reading.Parse[0])
	}
	return reading, nil
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
			Path:    a.store.Relative(failure.Path),
			Line:    failure.Line,
			Message: a.worktreeRelativeText(failure.Message),
		}
	}
	return output.AsError(err)
}

// parseFailure types finding, the parse finding of a node file that does not parse, as
// frontmatterFailure types the parser's error: invalid, at its file and line, both the path and
// the message naming the file relative to the working tree root.
func (a *app) parseFailure(finding diagnose.Finding) error {
	return &output.Error{
		Code:    output.CodeInvalid,
		Path:    a.store.Relative(finding.Path),
		Line:    finding.Line,
		Message: a.worktreeRelativeText(finding.Message),
	}
}
