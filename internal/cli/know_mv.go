package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/tree"
)

// knowMvOptions holds the flags of one capcom know mv.
type knowMvOptions struct {
	parent       string
	isReparented bool
	ifMatch      string
}

// newKnowMvCommand renames a Note, moves it below another, or both. A rename keeps the old id as
// an Alias; the Note's subtree moves with it. capcom moves files with a plain file-system rename
// and never runs git.
func (a *app) newKnowMvCommand() *cobra.Command {
	options := &knowMvOptions{}
	cmd := &cobra.Command{
		Use:         "mv <id|alias|prefix> [<new-id>] [--parent <id|alias|prefix|.>] [--if-match <etag>]",
		Short:       "Rename a Note or move it below another",
		Args:        cobra.RangeArgs(1, 2),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			options.isReparented = cmd.Flags().Changed(flagParent)
			if len(args) == 1 && !options.isReparented {
				return UsageErrorf("know mv needs a new id, --parent, or both")
			}
			result, err := a.moveNote(args, options)
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&options.parent, flagParent, "", "move the note below this one; . moves it to the top of the tree")
	flags.StringVar(&options.ifMatch, flagIfMatch, "", "only move when the file's etag is this one (see know show)")
	return cmd
}

// moveNote resolves the Note args[0] names and moves it to the new id args[1], when given, and
// below options.parent, when reparented. It refuses with conflict, before anything moves, when
// --if-match is set and is not the Note file's etag. The result names the Note's new id, path and
// etag, with a warning for every tree problem that concerns neither the Note, its new path nor the
// new parent; a stale capcom block of AGENTS.md is refreshed afterwards.
func (a *app) moveNote(args []string, options *knowMvOptions) (Result, error) {
	notes, err := tree.Load(a.store.KnowledgeDir(), readNoteAliases)
	problems, err := a.treeProblems(err)
	if err != nil {
		return Result{}, err
	}
	node, err := resolveNode(notes, args[0])
	if err != nil {
		return Result{}, err
	}
	destination := tree.Destination{ID: node.ID, ParentID: node.ParentID}
	if len(args) == 2 {
		destination.ID = args[1]
	}
	if options.isReparented {
		if destination.ParentID, err = resolveParent(notes, options.parent); err != nil {
			return Result{}, err
		}
	}

	move, err := knowledge.PlanMove(notes, node.ID, destination)
	if err != nil {
		return Result{}, a.noteMoveFailure(err, node)
	}
	resolvedParentID := ""
	if options.isReparented {
		resolvedParentID = destination.ParentID
	}
	warned, err := a.triageMoveProblems(notes, problems, move.Move, resolvedParentID, readNoteAliases)
	if err != nil {
		return Result{}, err
	}
	if move.NewID != move.Node.ID {
		// PlanMove has parsed the moved Note's file, so the holder found is another Note's.
		err := a.unparsedAliasFailure(notes, problems, move.NewID, noteNoun, recoverNoteAliases)
		if err != nil {
			return Result{}, err
		}
	}
	if current := edit.ETag(move.Original); options.ifMatch != "" && options.ifMatch != current {
		return Result{}, &Error{
			Code:    CodeConflict,
			Path:    relativeSlashPath(a.store.WorktreeRoot, node.Path),
			Message: fmt.Sprintf("etag %s does not match the file's etag %s; show the note again", options.ifMatch, current),
		}
	}
	if err := applyMove(move.Move); err != nil {
		return Result{}, a.moveApplyFailure(err, move.Node)
	}
	path := relativeSlashPath(a.store.WorktreeRoot, move.NewPath)
	result := noteWriteResult(move.NewID, path, edit.ETag(move.Content))
	result.Warnings = append(result.Warnings, a.movedWarnings(move.Move, warned)...)
	a.refreshAgentsAfterWrite(&result)
	return result, nil
}

// noteMoveFailure types a move knowledge.PlanMove refused: a taken id is exists, naming the Note
// file that holds it; a Note file that does not parse is invalid at its file and line; everything
// else, an invalid id and a parent inside the Note's own subtree among them, is invalid at the
// Note's file.
func (a *app) noteMoveFailure(err error, node tree.Node) error {
	var taken *tree.TakenError
	if errors.As(err, &taken) {
		return noteExistsFailure(taken.ID, taken.Owner, a.store.WorktreeRoot)
	}
	var parseFailure *knowledge.Error
	if errors.As(err, &parseFailure) {
		return a.noteFailure(err)
	}
	return &Error{
		Code:    CodeInvalid,
		Path:    relativeSlashPath(a.store.WorktreeRoot, node.Path),
		Message: err.Error(),
	}
}
