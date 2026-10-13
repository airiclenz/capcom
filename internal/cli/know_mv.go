package cli

import (
	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
)

// knowMvOptions holds the flags of one capcom know mv.
type knowMvOptions struct {
	parent       string
	isReparented bool
	ifMatch      string
}

// newKnowMvCommand renames a Note, moves it below another, or both. A rename keeps the old id as
// an Alias and rewrites every other Note's superseded-by entry naming it; the Note's subtree moves
// with it. capcom moves files with a plain file-system rename
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
				return output.UsageErrorf("know mv needs a new id, --parent, or both")
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

// moveNote moves the Note args[0] names to the new id args[1], when given, and below
// options.parent, when reparented. It refuses with conflict, before anything moves, when
// --if-match is set and is not the Note file's etag, and, taking the move back, when a Note file
// it rewrites changed while it ran. The result names the Note's new id, path and etag, with a
// warning for every tree problem that concerns neither the Note, its new path nor the new parent,
// for every Note file whose superseded-by entries could not be checked, and for an emptied old
// parent directory it could not remove; the Kind's sync hook (see notes) refreshes a stale capcom
// block of AGENTS.md afterwards.
func (a *app) moveNote(args []string, options *knowMvOptions) (output.Result, error) {
	destination, err := moveDestination(args, options.parent, options.isReparented)
	if err != nil {
		return output.Result{}, err
	}
	moved, problems, err := a.notes(nil).Move(args[0], destination, options.ifMatch)
	if err != nil {
		return output.Result{}, kindFailure(err)
	}
	result := noteWriteResult(moved.ID, moved.Path, moved.ETag)
	indexWarnings, warnings := a.movedWarnings(moved, problems)
	a.indexWarnings = indexWarnings
	result.Warnings = append(result.Warnings, warnings...)
	result.Warnings = append(result.Warnings, supersededByUnreadWarnings(moved.Unread, moved.OldID)...)
	result.Warnings = append(result.Warnings, cleanupWarnings(moved.LeftInPlace)...)
	addSyncEffects(&result, moved.Effects)
	return result, nil
}
