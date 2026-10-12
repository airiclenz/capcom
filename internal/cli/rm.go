package cli

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/edit"
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

// removeFile removes the issue file and its empty children directory; tests swap it to inject a
// failure.
var removeFile = os.Remove

// issueRemoval is one rm worked out before anything on disk changes: the issue file, its empty
// children directory when it has one, and the files whose deps on it or related entries naming it
// are dropped, each rewritten once.
type issueRemoval struct {
	node tree.Node
	// path is the issue file relative to the working tree root, with forward slashes.
	path           string
	childrenDir    string
	hasChildrenDir bool
	rewrites       []depRewrite
	// dropped are the ids of the issues whose deps are rewritten, in tree order.
	dropped []string
	// relatedDropped are the ids of the issues whose related entries are rewritten, in tree order.
	relatedDropped []string
	// unread are the files of other issues that do not parse, so their deps and related entries
	// were not checked.
	unread []string
}

// depRewrite is one file rm rewrites to drop deps or related entries, with the permissions the
// file had when rm read it: taking the rewrite back writes Original with them.
type depRewrite struct {
	tree.Rewrite
	mode fs.FileMode
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
// Every refusal comes before anything changes; see planIssueRemoval. The result names the removed
// file, the dependents whose deps were dropped and the issues whose related entries were dropped, with a warning for every tree problem that
// concerns neither the issue nor its parent, and for every issue file whose deps were not checked.
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
	removal, err := a.planIssueRemoval(issues, node, options)
	if err != nil {
		return Result{}, err
	}
	if err := a.applyIssueRemoval(removal); err != nil {
		return Result{}, err
	}

	result := issueRemoveResult(node.ID, removal.path, removal.dropped, removal.relatedDropped)
	result.Warnings = append(result.Warnings, warnings...)
	for _, unread := range removal.unread {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"%s: deps not checked for %s: the file does not parse",
			relativeSlashPath(a.store.WorktreeRoot, unread),
			node.ID,
		))
	}
	if node.ParentID != "" {
		a.removeEmptyParentDir(filepath.Dir(node.Path), &result)
	}
	return result, nil
}

// planIssueRemoval works out the removal of node, changing nothing. It refuses, invalid at the
// file it names, an issue with children, a children directory holding anything and an issue other
// issues depend on unless options drop those deps; an --if-match that is not the file's etag is a
// conflict. A dependency of the issue on itself is no dependent. Related entries naming the issue
// never refuse it: they are dropped.
func (a *app) planIssueRemoval(issues *tree.Tree, node tree.Node, options *rmOptions) (*issueRemoval, error) {
	removal := &issueRemoval{
		node:        node,
		path:        relativeSlashPath(a.store.WorktreeRoot, node.Path),
		childrenDir: strings.TrimSuffix(node.Path, nodeFileExtension),
	}
	if children := issues.ChildrenOf(node.ID); len(children) > 0 {
		return nil, &Error{
			Code:    CodeInvalid,
			Path:    removal.path,
			Message: fmt.Sprintf("issue %s has %s; remove or move them first", node.ID, childCount(len(children), issueNoun)),
		}
	}
	var err error
	if removal.hasChildrenDir, err = a.checkChildrenDir(removal.childrenDir, issueNoun); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(node.Path)
	if err != nil {
		return nil, &Error{Code: CodeInvalid, Path: removal.path, Message: err.Error()}
	}
	if current := edit.ETag(data); options.ifMatch != "" && options.ifMatch != current {
		return nil, &Error{
			Code:    CodeConflict,
			Path:    removal.path,
			Message: fmt.Sprintf("etag %s does not match the file's etag %s; show the issue again", options.ifMatch, current),
		}
	}

	graph, err := a.loadDepGraph()
	if err != nil {
		return nil, err
	}
	dependents := slices.DeleteFunc(graph.graph.Dependents(node.ID), func(id string) bool { return id == node.ID })
	if len(dependents) > 0 && !options.shouldDropDeps {
		return nil, &Error{
			Code: CodeInvalid,
			Path: removal.path,
			Message: fmt.Sprintf(
				"issue %s is a dependency of %s; pass --%s to remove those deps entries",
				node.ID,
				strings.Join(dependents, ", "),
				flagDropDeps,
			),
		}
	}
	if err := a.planDepDrops(removal, issues, dependents, graph.names(node.ID)); err != nil {
		return nil, err
	}
	return removal, nil
}

// planDepDrops reads every other issue file of issues. A file that does not parse joins
// removal.unread. Each file is added as one rewrite without the deps entries isDropped matches,
// when its issue is in dependents, and without the related entries isDropped matches; its id joins
// removal.dropped for the first and removal.relatedDropped for the second, and its permissions are
// kept for a rollback.
func (a *app) planDepDrops(
	removal *issueRemoval,
	issues *tree.Tree,
	dependents []string,
	isDropped func(string) bool,
) error {
	for _, other := range issues.Nodes() {
		if other.ID == removal.node.ID {
			continue
		}
		data, err := os.ReadFile(other.Path)
		if err != nil {
			return &Error{Code: CodeInvalid, Path: relativeSlashPath(a.store.WorktreeRoot, other.Path), Message: err.Error()}
		}
		parsed, err := issue.Parse(other.Path, data)
		if err != nil {
			removal.unread = append(removal.unread, other.Path)
			continue
		}
		isDependent := slices.Contains(dependents, other.ID)
		if isDependent {
			parsed.Deps = dropEntries(parsed.Deps, isDropped)
			removal.dropped = append(removal.dropped, other.ID)
		}
		related := dropEntries(parsed.Related, isDropped)
		isRelatedDropped := len(related) != len(parsed.Related)
		if isRelatedDropped {
			parsed.Related = related
			removal.relatedDropped = append(removal.relatedDropped, other.ID)
		}
		if !isDependent && !isRelatedDropped {
			continue
		}
		info, err := os.Stat(other.Path)
		if err != nil {
			return &Error{Code: CodeInvalid, Path: relativeSlashPath(a.store.WorktreeRoot, other.Path), Message: err.Error()}
		}
		removal.rewrites = append(removal.rewrites, depRewrite{
			Rewrite: tree.Rewrite{Path: other.Path, Original: data, Content: parsed.Serialize()},
			mode:    info.Mode(),
		})
	}
	return nil
}

// dropEntries returns a copy of entries, a deps or related list, without the entries isDropped
// matches, or nil when none is left.
func dropEntries(entries []string, isDropped func(string) bool) []string {
	kept := slices.DeleteFunc(slices.Clone(entries), isDropped)
	if len(kept) == 0 {
		return nil
	}
	return kept
}

// applyIssueRemoval writes every rewrite atomically, then removes the empty children directory
// and last the issue file. When any step fails the rewrites already written get their original
// bytes and permissions back, so no dependent loses a dep on an issue that is still there.
func (a *app) applyIssueRemoval(removal *issueRemoval) error {
	for written, rewrite := range removal.rewrites {
		if err := tree.WriteFileAtomic(rewrite.Path, bytes.NewReader(rewrite.Content)); err != nil {
			return a.restoreRewrites(removal.rewrites[:written], rewrite.Path, err)
		}
	}
	// The empty children directory goes before the file: should the file then fail to go, no
	// directory is ever left without its issue file.
	if removal.hasChildrenDir {
		if err := removeFile(removal.childrenDir); err != nil {
			return a.restoreRewrites(removal.rewrites, removal.childrenDir, err)
		}
	}
	if err := removeFile(removal.node.Path); err != nil {
		return a.restoreRewrites(removal.rewrites, removal.node.Path, err)
	}
	return nil
}

// restoreRewrites writes the original bytes of the applied rewrites back with their original
// permissions, newest first, and returns cause as invalid at path, with every rewrite that could
// not be restored appended.
func (a *app) restoreRewrites(applied []depRewrite, path string, cause error) error {
	failures := []string{flatten(cause.Error())}
	for index := len(applied) - 1; index >= 0; index-- {
		rewrite := applied[index]
		if err := tree.WriteFileAtomicVerbatim(rewrite.Path, bytes.NewReader(rewrite.Original), rewrite.mode); err != nil {
			failures = append(failures, "rolling back: "+flatten(err.Error()))
		}
	}
	return &Error{
		Code:    CodeInvalid,
		Path:    relativeSlashPath(a.store.WorktreeRoot, path),
		Message: strings.Join(failures, problemSeparator),
	}
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
