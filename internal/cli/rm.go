package cli

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/tree"
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

// removeIssue resolves arg and deletes its issue file together with the issue's own children
// directory when that is empty, then the parent's children directory when the removal emptied it.
// Every refusal comes before anything changes; see issue.PlanRemove. The result names the removed
// file, the dependents whose deps were dropped and the issues whose related entries were dropped,
// with a warning for every tree problem that concerns neither the issue nor its parent, for every
// issue file whose deps were not checked and for an emptied parent directory left in place.
func (a *app) removeIssue(arg string, options *rmOptions) (Result, error) {
	issues, err := tree.Load(a.store.IssuesDir(), readAliases)
	problems, err := a.treeProblems(err)
	if err != nil {
		return Result{}, err
	}
	node, err := resolveNode(issues, arg)
	if err != nil {
		return Result{}, err
	}
	parentPath := ""
	if parent, isFound := issues.Lookup(node.ParentID); isFound {
		parentPath = a.worktreeRelative(parent.Path)
	}
	warnings, err := a.triageTreeProblems(problems, a.worktreeRelative(node.Path), parentPath)
	if err != nil {
		return Result{}, err
	}
	removal, err := issue.PlanRemove(issues, node.ID, issue.RemoveOptions{
		IfMatch:        options.ifMatch,
		ShouldDropDeps: options.shouldDropDeps,
	})
	if err != nil {
		return Result{}, a.issueRemovalFailure(err, node)
	}
	cleanupFailures, err := removal.Apply()
	if err != nil {
		return Result{}, a.issueRemovalApplyFailure(err, issues, removal)
	}

	path := relativeSlashPath(a.store.WorktreeRoot, node.Path)
	result := issueRemoveResult(node.ID, path, removal.DepsDropped, removal.RelatedDropped)
	result.Warnings = append(result.Warnings, warnings...)
	for _, unread := range removal.Unread {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"%s: deps not checked for %s: the file does not parse",
			relativeSlashPath(a.store.WorktreeRoot, unread),
			node.ID,
		))
	}
	for _, failure := range cleanupFailures {
		result.Warnings = append(result.Warnings, a.cleanupWarning(failure))
	}
	return result, nil
}

// issueRemovalFailure types err, a refusal of the removal of node: a stale --if-match is a
// conflict at the issue file, dependents are invalid there, and the rest are typed as
// removalFailure types them.
func (a *app) issueRemovalFailure(err error, node tree.Node) error {
	path := relativeSlashPath(a.store.WorktreeRoot, node.Path)
	var mismatch *issue.ETagMismatchError
	if errors.As(err, &mismatch) {
		return &Error{Code: CodeConflict, Path: path, Message: mismatch.Error() + "; show the issue again"}
	}
	var dependents *issue.DependentsError
	if errors.As(err, &dependents) {
		return &Error{
			Code:    CodeInvalid,
			Path:    path,
			Message: fmt.Sprintf("%s; pass --%s to remove those deps entries", dependents.Error(), flagDropDeps),
		}
	}
	return a.removalFailure(err, node)
}

// issueRemovalApplyFailure types err, the failure of removal's Apply in issues, as invalid at the
// file or directory of the step that failed, the first of removal's paths its cause names, else at
// the issue file. The message joins the cause and every change that could not be taken back, each on
// one line, with the paths inside the working tree relative to its root.
func (a *app) issueRemovalApplyFailure(err error, issues *tree.Tree, removal *issue.Removal) error {
	failures := []error{err}
	if joined, isJoined := err.(interface{ Unwrap() []error }); isJoined {
		failures = joined.Unwrap()
	}
	messages := make([]string, 0, len(failures))
	for _, failure := range failures {
		messages = append(messages, flatten(failure.Error()))
	}

	failedPath := removal.Node.Path
	firstIndex := -1
	cause := failures[0].Error()
	for _, path := range removalPaths(issues, removal) {
		index := strings.Index(cause, path+":")
		if index >= 0 && (firstIndex < 0 || index < firstIndex) {
			failedPath, firstIndex = path, index
		}
	}
	return &Error{
		Code:    CodeInvalid,
		Path:    relativeSlashPath(a.store.WorktreeRoot, failedPath),
		Message: a.worktreeRelativeText(strings.Join(messages, problemSeparator)),
	}
}

// removalPaths returns the files and directory removal's Apply changes: the files of the issues
// whose entries it drops, the children directory when the issue has one, and the issue file.
func removalPaths(issues *tree.Tree, removal *issue.Removal) []string {
	var paths []string
	for _, id := range slices.Concat(removal.DepsDropped, removal.RelatedDropped) {
		if node, isFound := issues.Lookup(id); isFound {
			paths = append(paths, node.Path)
		}
	}
	if removal.ChildrenDir != "" {
		paths = append(paths, removal.ChildrenDir)
	}
	return append(paths, removal.Node.Path)
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
