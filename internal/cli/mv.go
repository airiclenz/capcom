package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/tree"
)

// rootParentArg is the --parent value of capcom mv that moves an issue to the top of the tree.
const rootParentArg = "."

// mvOptions holds the flags of one capcom mv.
type mvOptions struct {
	parent       string
	isReparented bool
	ifMatch      string
}

// newMvCommand renames an issue, moves it below another, or both. A rename keeps the old id as an
// Alias and rewrites every deps entry that named the issue; the issue's subtree moves with it.
// capcom moves files with a plain file-system rename and never runs git.
func (a *app) newMvCommand() *cobra.Command {
	options := &mvOptions{}
	cmd := &cobra.Command{
		Use:         "mv <id|alias|prefix> [<new-id>] [--parent <id|alias|prefix|.>] [--if-match <etag>]",
		Short:       "Rename an issue or move it below another",
		Args:        cobra.RangeArgs(1, 2),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			options.isReparented = cmd.Flags().Changed(flagParent)
			if len(args) == 1 && !options.isReparented {
				return UsageErrorf("mv needs a new id, --parent, or both")
			}
			result, err := a.move(args, options)
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&options.parent, flagParent, "", "move the issue below this one; . moves it to the top of the tree")
	flags.StringVar(&options.ifMatch, flagIfMatch, "", "only move when the file's etag is this one (see show)")
	return a.withKV(cmd)
}

// move moves the issue args[0] names to the new id args[1], when given, and below options.parent,
// when reparented. It refuses with conflict, before anything moves, when --if-match is set and is
// not the issue file's etag. The result names the issue's new id, path and etag, with a warning
// for every tree problem that concerns neither the issue, its new path nor the new parent, and for
// every issue file whose deps could not be checked.
func (a *app) move(args []string, options *mvOptions) (Result, error) {
	moved, problems, err := nodestore.Issues(a.store, nil).Move(
		args[0],
		moveDestination(args, options.parent, options.isReparented),
		options.ifMatch,
	)
	if err != nil {
		return Result{}, kindFailure(err)
	}
	result := writeResult(moved.ID, moved.Path, moved.ETag)
	result.Warnings = append(result.Warnings, a.movedWarnings(moved, problems)...)
	for _, unread := range moved.Unread {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"%s: deps not checked for %s: the file does not parse",
			unread,
			moved.OldID,
		))
	}
	return result, nil
}

// moveDestination is the nodestore.Destination of mv and know mv: the new id args[1], when given,
// and parent, when isReparented, rootParentArg naming the top of the tree.
func moveDestination(args []string, parent string, isReparented bool) nodestore.Destination {
	destination := nodestore.Destination{Parent: parent, IsReparented: isReparented}
	if len(args) == 2 {
		destination.ID = args[1]
	}
	if parent == rootParentArg {
		destination.Parent = ""
	}
	return destination
}

// movedWarnings renders problems, the tree problems a move returned, as warnings once the move has
// been applied, leaving out every problem the index refresh already warned about. A path inside
// the moved node's children directory, in problems and in the index refresh's warnings alike, is
// renamed to where that file now lives, so no warning names a file by a path that no longer
// exists.
func (a *app) movedWarnings(moved nodestore.Moved, problems tree.Errors) []string {
	var warned tree.Errors
	for _, problem := range problems {
		if !a.isIndexWarnedAbout(problem) {
			warned = append(warned, problem)
		}
	}
	oldDir := strings.TrimSuffix(moved.OldPath, nodeFileExtension) + "/"
	newDir := strings.TrimSuffix(moved.Path, nodeFileExtension) + "/"
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
