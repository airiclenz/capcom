package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/tree"
)

// Flags of capcom update; --status, --type, --priority and --plan are shared with list.
const (
	flagTitle          = "title"
	flagAddLabel       = "add-label"
	flagRemoveLabel    = "remove-label"
	flagBodyAppend     = "body-append"
	flagBodyAppendFile = "body-append-file"
	flagBodyReplaceOld = "body-replace-old"
	flagBodyReplaceNew = "body-replace-new"
	flagIfMatch        = "if-match"
	stdinPath          = "-"
	kindIssueWrite     = "issue-write"
	problemSeparator   = "; "
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
				return UsageErrorf("update needs at least one flag that changes the issue")
			}
			appendFile := ""
			if cmd.Flags().Changed(flagBodyAppendFile) {
				text, err := readTextInput(cmd, options.bodyAppendFile)
				if err != nil {
					return err
				}
				appendFile = text
			}
			result, err := a.editIssue(args[0], options.ifMatch, func(target issue.Issue) (issue.Issue, error) {
				return a.applyUpdate(cmd, options, appendFile, target)
			})
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
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
		if err := checkStatusChange(target.Status, options.status, a.store.Config); err != nil {
			return issue.Issue{}, err
		}
		target.Status = options.status
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
	target = target.WithLabels(options.addLabels, options.removeLabels)
	if flags.Changed(flagBodyReplaceOld) {
		replaced, err := target.ReplaceBody(options.replaceOld, options.replaceNew)
		if err != nil {
			return issue.Issue{}, Errorf(CodeInvalid, "%s", err.Error())
		}
		target = replaced
	}
	if flags.Changed(flagBodyAppend) {
		target = target.AppendBody(options.bodyAppend)
	}
	if flags.Changed(flagBodyAppendFile) {
		target = target.AppendBody(appendFile)
	}
	return target, nil
}

// checkStatusChange refuses a move into or out of a closed category (done or dropped, ADR 0003):
// close and reopen own those, with the closed time, reason and Log entry they bring.
func checkStatusChange(from string, to string, cfg config.Config) error {
	if from == to {
		return nil
	}
	fromCategory, _ := cfg.CategoryOf(from)
	toCategory, _ := cfg.CategoryOf(to)
	if isClosedCategory(fromCategory) {
		return Errorf(CodeInvalid, "status %q is closed; use reopen to move the issue out of it", from)
	}
	if isClosedCategory(toCategory) {
		return Errorf(CodeInvalid, "status %q is in the %s category; use close", to, toCategory)
	}
	return nil
}

// isClosedCategory reports whether category is one a closed issue is in.
func isClosedCategory(category config.Category) bool {
	return category == config.CategoryDone || category == config.CategoryDropped
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

// readTextInput reads the file at path, or stdin when path is "-". Errors name the file.
func readTextInput(cmd *cobra.Command, path string) (string, error) {
	if path == stdinPath {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", Errorf(CodeInvalid, "read stdin: %v", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", &Error{Code: CodeNotFound, Path: path, Message: "no such file"}
	}
	if err != nil {
		return "", &Error{Code: CodeInvalid, Path: path, Message: err.Error()}
	}
	return string(data), nil
}

// editIssue is the read-modify-write cycle every command that changes an existing issue goes
// through: it resolves arg, reads the issue file, refuses with conflict when ifMatch is set and
// is not the file's etag, applies change to the parsed issue, then validates and writes the result
// atomically. The result names the issue and its new etag.
func (a *app) editIssue(
	arg string,
	ifMatch string,
	change func(issue.Issue) (issue.Issue, error),
) (Result, error) {
	entry, err := a.resolveIssue(arg)
	if err != nil {
		return Result{}, err
	}
	data, err := a.readIssueFile(entry.Path)
	if err != nil {
		return Result{}, err
	}
	if current := edit.ETag(data); ifMatch != "" && ifMatch != current {
		return Result{}, &Error{
			Code:    CodeConflict,
			Path:    entry.Path,
			Message: fmt.Sprintf("etag %s does not match the file's etag %s; show the issue again", ifMatch, current),
		}
	}
	parsed, err := issue.Parse(entry.Path, data)
	if err != nil {
		return Result{}, a.issueFailure(err)
	}
	edited, err := change(parsed)
	if err != nil {
		failure := asError(err)
		if failure.Path == "" {
			failure.Path = entry.Path
		}
		return Result{}, failure
	}
	etag, err := a.writeIssueFile(entry.Path, edited)
	if err != nil {
		return Result{}, err
	}
	return writeResult(entry.ID, entry.Path, etag), nil
}

// readIssueFile reads the issue file at path, relative to the working tree root with forward
// slashes (an index Entry's Path).
func (a *app) readIssueFile(path string) ([]byte, error) {
	data, err := os.ReadFile(a.absoluteIssuePath(path))
	if err != nil {
		return nil, &Error{Code: CodeInvalid, Path: path, Message: err.Error()}
	}
	return data, nil
}

// writeIssueFile validates target against the config and writes it atomically to path, relative
// to the working tree root with forward slashes, returning the new etag. A target that does not
// validate fails invalid, citing its first problem's line, and is not written.
func (a *app) writeIssueFile(path string, target issue.Issue) (string, error) {
	if problems := issue.Validate(path, target, a.store.Config); len(problems) > 0 {
		messages := make([]string, 0, len(problems))
		for _, problem := range problems {
			messages = append(messages, problem.Message)
		}
		return "", &Error{
			Code:    CodeInvalid,
			Path:    path,
			Line:    problems[0].Line,
			Message: strings.Join(messages, problemSeparator),
		}
	}
	data := bytes.ReplaceAll(target.Serialize(), []byte("\r\n"), []byte("\n"))
	if err := tree.WriteFileAtomic(a.absoluteIssuePath(path), bytes.NewReader(data)); err != nil {
		return "", asError(err)
	}
	return edit.ETag(data), nil
}

// absoluteIssuePath turns path, relative to the working tree root with forward slashes, into a
// native absolute path.
func (a *app) absoluteIssuePath(path string) string {
	return filepath.Join(a.store.WorktreeRoot, filepath.FromSlash(path))
}

// issueFailure types an issue file that cannot be parsed: invalid, at its file and line, both
// the path and the message naming the file relative to the working tree root.
func (a *app) issueFailure(err error) error {
	var failure *issue.Error
	if errors.As(err, &failure) {
		return &Error{
			Code:    CodeInvalid,
			Path:    a.worktreeRelative(failure.Path),
			Line:    failure.Line,
			Message: a.worktreeRelativeText(failure.Message),
		}
	}
	return asError(err)
}

// writeResult is the result of a command that wrote an issue: its id, path and new etag.
func writeResult(id string, path string, etag string) Result {
	return Result{
		Kind: kindIssueWrite,
		Data: map[string]string{"id": id, "path": path, "etag": etag},
		Text: fmt.Sprintf("id: %s\npath: %s\netag: %s\n", id, path, etag),
		KV:   []KVRecord{{Pairs: []KVPair{{"ID", id}, {"PATH", path}, {"ETAG", etag}}}},
	}
}
