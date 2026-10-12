package cli

import (
	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/nodestore"
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

// moveNote moves the Note args[0] names to the new id args[1], when given, and below
// options.parent, when reparented. It refuses with conflict, before anything moves, when
// --if-match is set and is not the Note file's etag. The result names the Note's new id, path and
// etag, with a warning for every tree problem that concerns neither the Note, its new path nor the
// new parent; a stale capcom block of AGENTS.md is refreshed afterwards.
func (a *app) moveNote(args []string, options *knowMvOptions) (Result, error) {
	moved, problems, err := nodestore.Notes(a.store, nil).Move(
		args[0],
		moveDestination(args, options.parent, options.isReparented),
		options.ifMatch,
	)
	if err != nil {
		return Result{}, kindFailure(err)
	}
	result := noteWriteResult(moved.ID, moved.Path, moved.ETag)
	indexWarnings, warnings := a.movedWarnings(moved, problems)
	a.indexWarnings = indexWarnings
	result.Warnings = append(result.Warnings, warnings...)
	a.refreshAgentsAfterWrite(&result)
	return result, nil
}
