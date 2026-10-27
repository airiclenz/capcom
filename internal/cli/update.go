package cli

import (
	"errors"
	"fmt"
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

// flagNoStanding clears the standing mark of the issue capcom update changes.
const flagNoStanding = "no-standing"

// idPlaceholder stands in a hint for an issue id the failure does not know.
const idPlaceholder = "<id>"

// updateOptions holds the flags of one capcom update.
type updateOptions struct {
	title          string
	status         string
	issueType      string
	priority       int
	plan           string
	addLabels      []string
	removeLabels   []string
	bodyAppend     []string
	bodyAppendFile string
	replaceOld     []string
	replaceNew     []string
	ifMatch        string
	isStanding     bool
	isNotStanding  bool
}

// The help text of capcom update.
const (
	updateLong = "Change an Issue's fields, labels or body in place; at least one flag that changes the Issue " +
		"is required.\n\n" +
		"--status moves it between statuses that are not closed: close and reopen cross into and out " +
		"of the closed categories.\n\n" +
		"--body-append and --body-append-file (- reads stdin) add to the end of the body, and " +
		"--body-replace-old with --body-replace-new replaces text that must occur exactly once.\n\n" +
		"--body-append and the --body-replace-old/--body-replace-new pair repeat: every pair applies " +
		"in order, each against the body the pairs before it left, then every append in order, and " +
		"when one pair fails nothing is written.\n\n" +
		"--standing marks the Issue a standing Parent that only groups its children: it is never " +
		"Ready, and while it is marked a --status in the doing category is refused, as is close.\n\n" +
		"An Issue that is closed or in the doing category cannot be marked.\n\n" +
		"--no-standing clears the mark, and is applied before --status, so one call can clear it and " +
		"claim the Issue.\n\n" +
		"The first standing Issue in a store below format 5 raises the store format to 5, with a " +
		"warning that older capcom releases can no longer read the store.\n\n" +
		"--if-match refuses the write when the file no longer has the etag show printed.\n\n" +
		"It prints the Issue's id, path and new etag."
	updateExample = "  capcom update login-timeout-race --priority 1 --add-label owner-run\n" +
		"  capcom update login-timeout-race --status active   # claim it: a status in the doing category\n" +
		"  capcom update login-timeout-race --body-append \"Seen on Safari only.\"\n" +
		"  capcom update login-timeout-race --body-replace-old \"Safari only\" --body-replace-new \"Safari and Firefox\"\n" +
		"  capcom update login-timeout-race --body-replace-old \"P2\" --body-replace-new \"P1\" --body-replace-old \"todo\" --body-replace-new \"done\"   # pairs apply in order\n" +
		"  capcom update feature-requests --standing   # only groups its children: never Ready\n" +
		"  capcom update feature-requests --no-standing --status active   # clear the mark and claim it"
)

// newUpdateCommand changes the fields and body of one issue. Only the flags given change
// anything; a status change into or out of a closed category is refused, since close and reopen
// own those moves.
func (a *app) newUpdateCommand() *cobra.Command {
	options := &updateOptions{}
	cmd := &cobra.Command{
		Use:               "update <id|alias|prefix>",
		Short:             "Change an issue's fields or body",
		Long:              updateLong,
		Example:           updateExample,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeIssueIDs(1),
		Annotations:       map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			if !hasAnyFlag(cmd, flagTitle, flagStatus, flagType, flagPriority, flagPlan, flagAddLabel,
				flagRemoveLabel, flagBodyAppend, flagBodyAppendFile, flagBodyReplaceOld, flagStanding,
				flagNoStanding) {
				return output.UsageErrorf("update needs at least one flag that changes the issue")
			}
			if err := options.bodyEdits().checkPairs(); err != nil {
				return err
			}
			if err := a.checkUpdateValues(cmd, options); err != nil {
				return err
			}
			appendFile := ""
			if cmd.Flags().Changed(flagBodyAppendFile) {
				text, err := readTextInput(cmd, flagBodyAppendFile, options.bodyAppendFile)
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
			result := writeResult(written.ID, written.Path, written.ETag)
			if cmd.Flags().Changed(flagPlan) {
				result.Warnings = a.missingPlanWarnings(options.plan, written.Path)
			}
			return a.emit(cmd, result)
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&options.title, flagTitle, "", "new title")
	flags.StringVar(&options.status, flagStatus, "", "new status, not in a closed category (use close or reopen)")
	a.completeFlag(cmd, flagStatus, completeStatuses(isOpenCategory))
	flags.StringVar(&options.issueType, flagType, "", "new type")
	a.completeFlag(cmd, flagType, completeTypes)
	flags.IntVar(&options.priority, flagPriority, 0, "new priority, 0 (most urgent) to 4")
	flags.StringVar(&options.plan, flagPlan, "", "new plan basename; empty clears it")
	flags.StringSliceVar(&options.addLabels, flagAddLabel, nil, "labels to add")
	flags.StringSliceVar(&options.removeLabels, flagRemoveLabel, nil, "labels to remove")
	flags.StringArrayVar(&options.bodyAppend, flagBodyAppend, nil,
		"text to append to the body, taken literally; repeat to append several in order")
	flags.StringVar(&options.bodyAppendFile, flagBodyAppendFile, "", "file to append to the body; - reads stdin")
	flags.StringArrayVar(&options.replaceOld, flagBodyReplaceOld, nil,
		"body text to replace; must occur exactly once; repeat with --body-replace-new for several pairs")
	flags.StringArrayVar(&options.replaceNew, flagBodyReplaceNew, nil,
		"text that replaces the --body-replace-old in the same position")
	flags.StringVar(&options.ifMatch, flagIfMatch, "", "only write when the file's etag is this one (see show)")
	flags.BoolVar(&options.isStanding, flagStanding, false, "mark the issue standing: it only groups its children")
	flags.BoolVar(&options.isNotStanding, flagNoStanding, false, "clear the issue's standing mark")
	cmd.MarkFlagsMutuallyExclusive(flagBodyAppend, flagBodyAppendFile)
	cmd.MarkFlagsMutuallyExclusive(flagStanding, flagNoStanding)
	cmd.MarkFlagsRequiredTogether(flagBodyReplaceOld, flagBodyReplaceNew)
	return a.withKV(cmd)
}

// checkUpdateValues refuses an undeclared --status or --type, a --priority out of range, listing
// the values the flag accepts, and a --plan that is not a bare basename, before the issue is read
// or written. A declared status is judged by the status rules in applyUpdate.
func (a *app) checkUpdateValues(cmd *cobra.Command, options *updateOptions) error {
	flags := cmd.Flags()
	if flags.Changed(flagPlan) {
		if err := a.checkPlanValue(options.plan); err != nil {
			return err
		}
	}
	values := invalidValues(a.store.Config)
	if flags.Changed(flagStatus) {
		if err := values.status(options.status); err != nil {
			return err
		}
	}
	if flags.Changed(flagType) {
		if err := values.issueType(options.issueType); err != nil {
			return err
		}
	}
	if flags.Changed(flagPriority) {
		return values.priority(options.priority)
	}
	return nil
}

// applyUpdate applies the changed flags to target: the standing mark first, so a status change is
// judged on the issue as marked or unmarked, then the other fields, then labels, then the body
// replacements and appends in the order bodyEdits.apply gives them, then the --body-append-file
// text. A status rule's refusal is returned as the raw *issue.StatusError,
// for writeFailure to word with the issue's id.
func (a *app) applyUpdate(
	cmd *cobra.Command,
	options *updateOptions,
	appendFile string,
	target issue.Issue,
) (issue.Issue, error) {
	flags := cmd.Flags()
	if isStanding, isChanged := standingChange(cmd, options); isChanged {
		changed, err := issue.SetStanding(target, a.store.Config, isStanding)
		if err != nil {
			return issue.Issue{}, err
		}
		target = changed
	}
	if flags.Changed(flagStatus) {
		changed, err := issue.SetStatus(target, a.store.Config, options.status)
		if err != nil {
			return issue.Issue{}, err
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
	body, err := options.bodyEdits().apply(target.Body)
	if err != nil {
		return issue.Issue{}, err
	}
	target.Body = body
	if flags.Changed(flagBodyAppendFile) {
		target.Body = edit.AppendBody(target.Body, appendFile)
	}
	return target, nil
}

// bodyEdits returns the --body-replace-old/--body-replace-new pairs and the --body-append texts
// of the update, in the order given.
func (o *updateOptions) bodyEdits() bodyEdits {
	return bodyEdits{olds: o.replaceOld, news: o.replaceNew, appends: o.bodyAppend}
}

// standingChange returns the standing mark --standing or --no-standing sets, and whether either
// changes it; the two are mutually exclusive. --no-standing=false changes nothing.
func standingChange(cmd *cobra.Command, options *updateOptions) (isStanding bool, isChanged bool) {
	flags := cmd.Flags()
	switch {
	case flags.Changed(flagStanding):
		return options.isStanding, true
	case flags.Changed(flagNoStanding) && options.isNotStanding:
		return false, true
	}
	return false, false
}

// statusChangeFailure types a refusal from issue.SetStatus, issue.SetStanding, issue.Close or
// issue.Reopen as invalid, worded for the commands: the wording stays here because the hints name
// the update, close and reopen verbs. id names the issue in the hint, such as the update that
// clears a standing issue's mark; an empty id stays the <id> placeholder. Any other error, such as
// a failure to name the Log entry's author, is invalid with its own message.
func statusChangeFailure(id string, err error) error {
	var refusal *issue.StatusError
	if !errors.As(err, &refusal) {
		return output.Errorf(output.CodeInvalid, "%s", err.Error())
	}
	if id == "" {
		id = idPlaceholder
	}
	switch {
	case errors.Is(refusal, issue.ErrLeavesClosed):
		return &output.Error{
			Code:    output.CodeInvalid,
			Message: fmt.Sprintf("status %q is closed; only a reopen moves the issue out of it", refusal.Status),
			Hint:    fmt.Sprintf("capcom reopen %s --%s <text>", id, flagReason),
		}
	case errors.Is(refusal, issue.ErrEntersClosed):
		return &output.Error{
			Code: output.CodeInvalid,
			Message: fmt.Sprintf(
				"status %q is in the %s category; only a close moves the issue into it",
				refusal.Status,
				refusal.Category,
			),
			Hint: fmt.Sprintf("capcom close %s --%s <text>", id, flagReason),
		}
	case errors.Is(refusal, issue.ErrNotClosed):
		return output.Errorf(
			output.CodeInvalid,
			"status %q is in the %s category, so the issue is not closed; only a closed issue can be reopened",
			refusal.Status,
			refusal.Category,
		)
	case errors.Is(refusal, issue.ErrStanding):
		return &output.Error{
			Code:    output.CodeInvalid,
			Message: "the issue is standing: it only groups its children and is never claimed or closed",
			Hint:    fmt.Sprintf("capcom update %s --%s", id, flagNoStanding),
		}
	case errors.Is(refusal, issue.ErrStandingNotOpen):
		return output.Errorf(
			output.CodeInvalid,
			"status %q is in the %s category; only an issue that is neither closed nor in the doing category can be marked standing",
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
