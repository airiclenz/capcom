package cli

import (
	"errors"
	"fmt"
	"strings"

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
	return a.withKV(cmd)
}

// move resolves the issue args[0] names and moves it to the new id args[1], when given, and below
// parent, when isReparented. The result names the issue's new id, path and etag, with a warning
// for every tree problem that concerns neither the issue, its new path nor the new parent, and for
// every issue file whose deps could not be checked.
func (a *app) move(args []string, parent string, isReparented bool) (Result, error) {
	issues, err := tree.Load(a.store.IssuesDir(), readAliases)
	problems, err := a.treeProblems(err)
	if err != nil {
		return Result{}, err
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
	resolvedParentID := ""
	if isReparented {
		resolvedParentID = destination.ParentID
	}
	warned, err := a.triageMoveProblems(issues, problems, move.Move, resolvedParentID, readAliases)
	if err != nil {
		return Result{}, err
	}
	if err := move.Apply(); err != nil {
		return Result{}, asError(err)
	}
	path := relativeSlashPath(a.store.WorktreeRoot, move.NewPath)
	result := writeResult(move.NewID, path, edit.ETag(move.Content))
	result.Warnings = append(result.Warnings, a.movedWarnings(move.Move, warned)...)
	for _, unread := range move.Unread {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"%s: deps not checked for %s: the file does not parse",
			relativeSlashPath(a.store.WorktreeRoot, unread),
			move.Node.ID,
		))
	}
	return result, nil
}

// triageMoveProblems splits the tree problems of move as splitTreeProblems does, against the
// moved node's file, its new path and the file of newParentID, the parent the command resolved
// ("" when it resolved none, or the top of the tree), and returns the problems to warn about. A
// new id is first refused as orphanedIDFailure does, with aliases reading the Aliases of a file
// inside an orphan directory.
func (a *app) triageMoveProblems(
	issues *tree.Tree,
	problems tree.Errors,
	move *tree.Move,
	newParentID string,
	aliases tree.AliasFunc,
) (tree.Errors, error) {
	if move.NewID != move.Node.ID {
		if err := a.orphanedIDFailure(problems, move.NewID, aliases); err != nil {
			return nil, err
		}
	}
	parentPath := ""
	if parent, isFound := issues.Lookup(newParentID); isFound {
		parentPath = a.worktreeRelative(parent.Path)
	}
	return a.splitTreeProblems(
		problems,
		a.worktreeRelative(move.Node.Path),
		a.worktreeRelative(move.NewPath),
		parentPath,
	)
}

// movedWarnings renders warned, the problems triageMoveProblems returned, as warnings once move
// has been applied. A path inside the moved node's children directory, in warned and in the
// index refresh's warnings alike, is renamed to where that file now lives, so no warning names a
// file by a path that no longer exists.
func (a *app) movedWarnings(move *tree.Move, warned tree.Errors) []string {
	oldDir := a.worktreeRelative(strings.TrimSuffix(move.Node.Path, nodeFileExtension)) + "/"
	newDir := a.worktreeRelative(strings.TrimSuffix(move.NewPath, nodeFileExtension)) + "/"
	for i := range a.indexReport.Warnings {
		warning := &a.indexReport.Warnings[i]
		warning.Path = movedPath(warning.Path, oldDir, newDir)
	}
	for _, problem := range warned {
		problem.Path = movedPath(problem.Path, oldDir, newDir)
		problem.Other = movedPath(problem.Other, oldDir, newDir)
	}
	return treeProblemWarnings(warned)
}

// movedPath returns path with its leading oldDir replaced by newDir, or path unchanged when it
// does not lie inside oldDir. All three use forward slashes; both directories end in one.
func movedPath(path string, oldDir string, newDir string) string {
	if rest, isInside := strings.CutPrefix(path, oldDir); isInside {
		return newDir + rest
	}
	return path
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
