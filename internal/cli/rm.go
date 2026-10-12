package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/nodestore"
)

// Flags and output of capcom rm.
const (
	kindIssueRemove = "issue-remove"
	flagDropDeps    = "drop-deps"
	issueNoun       = "issue"
)

// rmOptions holds the flags of one capcom rm.
type rmOptions struct {
	ifMatch        string
	shouldDropDeps bool
}

// issueRemoveData is the --json data of rm: the removed issue's id and path, and the ids of the
// issues whose deps on it and whose related entries naming it were dropped.
type issueRemoveData struct {
	ID             string   `json:"id"`
	Path           string   `json:"path"`
	DepsDropped    []string `json:"deps_dropped"`
	RelatedDropped []string `json:"related_dropped"`
}

// newRmCommand deletes one issue file. An issue with children, whose children directory holds
// anything, or that other issues depend on is refused; --drop-deps removes those deps entries
// instead. Other issues' related entries naming it are always removed. Its Aliases go with the
// file. capcom deletes the file itself and never runs git.
func (a *app) newRmCommand() *cobra.Command {
	options := &rmOptions{}
	cmd := &cobra.Command{
		Use:         "rm <id|alias|prefix> [--if-match <etag>] [--drop-deps]",
		Short:       "Remove an issue that has no children",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			result, err := a.removeIssue(args[0], options)
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	cmd.Flags().StringVar(&options.ifMatch, flagIfMatch, "", "only remove when the file's etag is this one (see show)")
	cmd.Flags().BoolVar(
		&options.shouldDropDeps,
		flagDropDeps,
		false,
		"remove the deps entries of the issues that depend on it instead of refusing",
	)
	return a.withKV(cmd)
}

// removeIssue removes the issue arg names through the issue node store, which deletes its file
// together with the issue's own children directory when that is empty, then the parent's children
// directory when the removal emptied it. Every refusal comes before anything changes; see
// nodestore.Kind.Remove. The result names the removed file, the dependents whose deps were dropped
// and the issues whose related entries were dropped, with a warning for every tree problem that
// concerns neither the issue nor its parent, for every issue file whose deps were not checked and
// for an emptied parent directory left in place.
func (a *app) removeIssue(arg string, options *rmOptions) (Result, error) {
	removed, problems, err := nodestore.Issues(a.store, nil).Remove(arg, options.ifMatch, options.shouldDropDeps)
	if err != nil {
		return Result{}, issueRemovalFailure(err)
	}

	result := issueRemoveResult(removed.ID, removed.Path, removed.DepsDropped, removed.RelatedDropped)
	result.Warnings = append(result.Warnings, a.unwarnedTreeProblems(problems)...)
	for _, unread := range removed.Unread {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"%s: deps not checked for %s: the file does not parse",
			unread,
			removed.ID,
		))
	}
	result.Warnings = append(result.Warnings, cleanupWarnings(removed.LeftInPlace)...)
	return result, nil
}

// issueRemovalFailure types err, a failure of the removal of an issue: dependents are invalid at
// the issue file, with the flag that drops their deps entries named, and the rest are typed as
// kindFailure types them.
func issueRemovalFailure(err error) error {
	var dependents *nodestore.DependentsError
	if errors.As(err, &dependents) {
		return &Error{
			Code:    CodeInvalid,
			Path:    dependents.Path,
			Message: fmt.Sprintf("%s; pass --%s to remove those deps entries", dependents.Message, flagDropDeps),
		}
	}
	return kindFailure(err)
}

// issueRemoveResult is the result of rm: the id and path of the removed issue file and the ids of
// the issues whose deps on it and whose related entries naming it were dropped.
func issueRemoveResult(id string, path string, dropped []string, relatedDropped []string) Result {
	data := issueRemoveData{
		ID:             id,
		Path:           path,
		DepsDropped:    append([]string{}, dropped...),
		RelatedDropped: append([]string{}, relatedDropped...),
	}
	text := fmt.Sprintf("id: %s\npath: %s\n", id, path)
	if len(dropped) > 0 {
		text += "deps_dropped: " + flowList(dropped) + "\n"
	}
	if len(relatedDropped) > 0 {
		text += "related_dropped: " + flowList(relatedDropped) + "\n"
	}
	return Result{
		Kind: kindIssueRemove,
		Data: data,
		Text: text,
		KV: []KVRecord{{Pairs: []KVPair{
			{"ID", id},
			{"PATH", path},
			{"DEPS_DROPPED", flowList(dropped)},
			{"RELATED_DROPPED", flowList(relatedDropped)},
		}}},
	}
}
