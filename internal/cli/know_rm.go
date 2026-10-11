package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/tree"
)

// kindNoteRemove is the --json kind of know rm: the id and path of the removed Note file.
const kindNoteRemove = "note-remove"

// noteFileExtension ends every Note file; the Note's children directory is its path without it.
const noteFileExtension = ".md"

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
// files, and an --if-match that is not the file's etag. The result names the removed file, with a
// warning for every tree problem that concerns neither the Note nor its parent; a stale capcom
// block of AGENTS.md is refreshed afterwards.
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
	path := relativeSlashPath(a.store.WorktreeRoot, node.Path)
	if children := notes.ChildrenOf(node.ID); len(children) > 0 {
		return Result{}, &Error{
			Code:    CodeInvalid,
			Path:    path,
			Message: fmt.Sprintf("note %s has %s; remove or move them first", node.ID, childNoteCount(len(children))),
		}
	}
	childrenDir := strings.TrimSuffix(node.Path, noteFileExtension)
	hasChildrenDir, err := a.checkChildrenDir(childrenDir)
	if err != nil {
		return Result{}, err
	}
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

	// The empty children directory goes first: should the file then fail to go, the tree is
	// unchanged, and no directory is ever left without its Note file.
	if hasChildrenDir {
		if err := os.Remove(childrenDir); err != nil {
			return Result{}, &Error{Code: CodeInvalid, Path: relativeSlashPath(a.store.WorktreeRoot, childrenDir), Message: err.Error()}
		}
	}
	if err := os.Remove(node.Path); err != nil {
		return Result{}, &Error{Code: CodeInvalid, Path: path, Message: err.Error()}
	}

	result := noteRemoveResult(node.ID, path)
	result.Warnings = append(result.Warnings, warnings...)
	if node.ParentID != "" {
		a.removeEmptyParentDir(filepath.Dir(node.Path), &result)
	}
	a.refreshAgentsAfterWrite(&result)
	return result, nil
}

// checkChildrenDir reports whether the Note's children directory dir exists. A directory that
// holds anything, files the tree skips such as dotfiles and non-Note files among them, is refused
// invalid naming it, so removing the Note never orphans it.
func (a *app) checkChildrenDir(dir string) (bool, error) {
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	path := relativeSlashPath(a.store.WorktreeRoot, dir)
	if err != nil {
		return false, &Error{Code: CodeInvalid, Path: path, Message: err.Error()}
	}
	if !info.IsDir() {
		return false, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, &Error{Code: CodeInvalid, Path: path, Message: err.Error()}
	}
	if len(entries) > 0 {
		return false, &Error{
			Code:    CodeInvalid,
			Path:    path,
			Message: fmt.Sprintf("the note's children directory still holds %s; remove them first", entryCount(len(entries))),
		}
	}
	return true, nil
}

// removeEmptyParentDir removes the parent's children directory dir when the removal left it
// empty. The Note is already gone, so a failure is a warning on result, not a failed command.
func (a *app) removeEmptyParentDir(dir string, result *Result) {
	entries, err := os.ReadDir(dir)
	if err == nil && len(entries) > 0 {
		return
	}
	if err == nil {
		err = os.Remove(dir)
	}
	if err != nil {
		result.Warnings = append(result.Warnings,
			relativeSlashPath(a.store.WorktreeRoot, dir)+": "+flatten(err.Error())+"; the empty directory was not removed")
	}
}

// noteRemoveResult is the result of know rm: the id and path of the removed Note file.
func noteRemoveResult(id string, path string) Result {
	return Result{
		Kind: kindNoteRemove,
		Data: map[string]string{"id": id, "path": path},
		Text: fmt.Sprintf("id: %s\npath: %s\n", id, path),
	}
}

// childNoteCount phrases count child Notes: "1 child note", "2 child notes".
func childNoteCount(count int) string {
	if count == 1 {
		return "1 child note"
	}
	return fmt.Sprintf("%d child notes", count)
}

// entryCount phrases count directory entries: "1 entry", "2 entries".
func entryCount(count int) string {
	if count == 1 {
		return "1 entry"
	}
	return fmt.Sprintf("%d entries", count)
}
