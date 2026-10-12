package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/tree"
)

// kindNoteRemove is the --json kind of know rm: the id and path of the removed Note file.
const kindNoteRemove = "note-remove"

// noteNoun names a Note in the messages know rm shares with rm.
const noteNoun = "note"

// knowRmOptions holds the flags of one capcom know rm.
type knowRmOptions struct {
	ifMatch string
}

// newKnowRmCommand deletes one Note file. A Note with children, or whose children directory holds
// anything, is refused, so removal never leaves an orphan directory. Its Aliases go with the file.
func (a *app) newKnowRmCommand() *cobra.Command {
	options := &knowRmOptions{}
	cmd := &cobra.Command{
		Use:         "rm <id|alias|prefix> [--if-match <etag>]",
		Short:       "Remove a Note that has no children",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			result, err := a.removeNote(args[0], options)
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	cmd.Flags().StringVar(&options.ifMatch, flagIfMatch, "", "only remove when the file's etag is this one (see know show)")
	return cmd
}

// removeNote resolves arg and deletes its Note file together with the Note's own children
// directory when that is empty, then the parent's children directory when the removal emptied it.
// Before anything is deleted it refuses a Note with children, a children directory holding other
// files, and an --if-match that is not the file's etag; it never parses the Note file. The result
// names the removed file, with a warning for every tree problem that concerns neither the Note nor
// its parent and for an emptied parent directory left in place; a stale capcom block of AGENTS.md
// is refreshed afterwards.
func (a *app) removeNote(arg string, options *knowRmOptions) (Result, error) {
	notes, err := tree.Load(a.store.KnowledgeDir(), readNoteAliases)
	problems, err := a.treeProblems(err)
	if err != nil {
		return Result{}, err
	}
	node, err := resolveNode(notes, arg)
	if err != nil {
		return Result{}, err
	}
	parentPath := ""
	if parent, isFound := notes.Lookup(node.ParentID); isFound {
		parentPath = a.worktreeRelative(parent.Path)
	}
	warnings, err := a.triageTreeProblems(problems, a.worktreeRelative(node.Path), parentPath)
	if err != nil {
		return Result{}, err
	}
	removal, err := knowledge.PlanRemove(notes, node.ID)
	if err != nil {
		return Result{}, a.removalFailure(err, node)
	}
	path := relativeSlashPath(a.store.WorktreeRoot, node.Path)
	data, err := os.ReadFile(node.Path)
	if err != nil {
		return Result{}, &Error{Code: CodeInvalid, Path: path, Message: err.Error()}
	}
	if current := edit.ETag(data); options.ifMatch != "" && options.ifMatch != current {
		return Result{}, &Error{
			Code:    CodeConflict,
			Path:    path,
			Message: fmt.Sprintf("etag %s does not match the file's etag %s; show the note again", options.ifMatch, current),
		}
	}
	cleanupFailures, err := removal.Apply()
	if err != nil {
		return Result{}, a.removalFailure(err, node)
	}

	result := noteRemoveResult(node.ID, path)
	result.Warnings = append(result.Warnings, warnings...)
	for _, failure := range cleanupFailures {
		result.Warnings = append(result.Warnings, a.cleanupWarning(failure))
	}
	a.refreshAgentsAfterWrite(&result)
	return result, nil
}

// removalFailure types err, a refusal of a removal of node or the failure of its Apply: invalid at
// the children directory a *tree.ChildrenDirError or the first *fs.PathError names, else at the
// node's file.
func (a *app) removalFailure(err error, node tree.Node) error {
	path := node.Path
	var childrenDirErr *tree.ChildrenDirError
	var pathErr *fs.PathError
	switch {
	case errors.As(err, &childrenDirErr):
		path = childrenDirErr.Dir
	case errors.As(err, &pathErr):
		path = pathErr.Path
	}
	return &Error{Code: CodeInvalid, Path: relativeSlashPath(a.store.WorktreeRoot, path), Message: err.Error()}
}

// cleanupWarning renders failure, a removal's cleanup that left an emptied directory in place, as
// a warning naming that directory relative to the working tree root.
func (a *app) cleanupWarning(failure error) string {
	var cleanupErr *tree.CleanupError
	if !errors.As(failure, &cleanupErr) {
		return a.worktreeRelativeText(flatten(failure.Error()))
	}
	return relativeSlashPath(a.store.WorktreeRoot, cleanupErr.Path) + ": " + flatten(cleanupErr.Err.Error()) +
		"; the empty directory was not removed"
}

// noteRemoveResult is the result of know rm: the id and path of the removed Note file.
func noteRemoveResult(id string, path string) Result {
	return Result{
		Kind: kindNoteRemove,
		Data: map[string]string{"id": id, "path": path},
		Text: fmt.Sprintf("id: %s\npath: %s\n", id, path),
	}
}
