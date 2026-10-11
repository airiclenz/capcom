package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/tree"
)

// rootParentArg is the --parent value of capcom mv that moves an issue to the top of the tree.
const rootParentArg = "."

// newMvCommand renames an issue, moves it below another, or both. A rename keeps the old id as an
// Alias and rewrites every deps entry that named the issue; the issue's subtree moves with it.
// capcom moves files with a plain file-system rename and never runs git.
func (a *app) newMvCommand() *cobra.Command {
	var parent string
	cmd := &cobra.Command{
		Use:         "mv <id|alias|prefix> [<new-id>] [--parent <id|alias|prefix|.>]",
		Short:       "Rename an issue or move it below another",
		Args:        cobra.RangeArgs(1, 2),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			isReparented := cmd.Flags().Changed(flagParent)
			if len(args) == 1 && !isReparented {
				return UsageErrorf("mv needs a new id, --parent, or both")
			}
			result, err := a.move(args, parent, isReparented)
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	cmd.Flags().StringVar(&parent, flagParent, "", "move the issue below this one; . moves it to the top of the tree")
	return cmd
}

// move resolves the issue args[0] names and moves it to the new id args[1], when given, and below
// parent, when isReparented. The result names the issue's new id, path and etag, with a warning
// for every issue file whose deps could not be checked.
func (a *app) move(args []string, parent string, isReparented bool) (Result, error) {
	issues, err := tree.Load(a.store.IssuesDir(), readAliases)
	if err != nil {
		return Result{}, a.treeLoadFailure(err)
	}
	node, err := resolveNode(issues, args[0])
	if err != nil {
		return Result{}, err
	}
	destination := tree.Destination{ID: node.ID, ParentID: node.ParentID}
	if len(args) == 2 {
		destination.ID = args[1]
	}
	if isReparented {
		if destination.ParentID, err = resolveParent(issues, parent); err != nil {
			return Result{}, err
		}
	}

	move, err := issue.PlanMove(issues, node.ID, destination)
	if err != nil {
		return Result{}, a.moveFailure(err, node)
	}
	if err := move.Apply(); err != nil {
		return Result{}, asError(err)
	}
	path := relativeSlashPath(a.store.WorktreeRoot, move.NewPath)
	result := writeResult(move.NewID, path, edit.ETag(move.Content))
	for _, unread := range move.Unread {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"%s: deps not checked for %s: the file does not parse",
			relativeSlashPath(a.store.WorktreeRoot, unread),
			move.Node.ID,
		))
	}
	return result, nil
}

// resolveNode resolves arg to its node in issues, typing the failure as resolveFailure does.
func resolveNode(issues *tree.Tree, arg string) (tree.Node, error) {
	node, err := issues.Resolve(arg)
	if err != nil {
		failure, _ := resolveFailure(err) // either way the command stops here
		return tree.Node{}, failure
	}
	return node, nil
}

// resolveParent returns the id of the issue the --parent value names, or "" for rootParentArg.
func resolveParent(issues *tree.Tree, parent string) (string, error) {
	if parent == rootParentArg {
		return "", nil
	}
	node, err := resolveNode(issues, parent)
	if err != nil {
		return "", err
	}
	return node.ID, nil
}

// moveFailure types a move PlanMove refused: a taken id is exists, naming the file that holds it;
// an unknown issue is not_found; everything else, an invalid id and a parent inside the issue's
// own subtree among them, is invalid at the issue's file.
func (a *app) moveFailure(err error, node tree.Node) error {
	var taken *tree.TakenError
	if errors.As(err, &taken) {
		return existsFailure(taken.ID, taken.Owner, a.store.WorktreeRoot)
	}
	if errors.Is(err, tree.ErrNotFound) {
		return Errorf(CodeNotFound, "%s", err.Error())
	}
	var parseFailure *issue.Error
	if errors.As(err, &parseFailure) {
		return issueFailure(err)
	}
	return &Error{
		Code:    CodeInvalid,
		Path:    relativeSlashPath(a.store.WorktreeRoot, node.Path),
		Message: err.Error(),
	}
}
