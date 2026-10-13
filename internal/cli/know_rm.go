package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/nodestore"
)

// kindNoteRemove is the --json kind of know rm: the id and path of the removed Note file and the
// ids of the Notes whose superseded-by entries naming it were dropped.
const kindNoteRemove = "note-remove"

// noteNoun names a Note in the messages know rm shares with rm.
const noteNoun = "note"

// knowRmOptions holds the flags of one capcom know rm.
type knowRmOptions struct {
	ifMatch string
}

// newKnowRmCommand deletes one Note file. A Note with children, or whose children directory holds
// anything, is refused, so removal never leaves an orphan directory. Its Aliases go with the file,
// and every other Note's superseded-by entries naming it are dropped.
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

// removeNote removes the Note arg names through the Note node store, which deletes its file
// together with the Note's own children directory when that is empty, then the parent's children
// directory when the removal emptied it, and drops every other Note's superseded-by entries naming
// it. It refuses, before anything is deleted, a Note with children, a children directory holding
// other files and an --if-match that is not the file's etag, and, taking the removal back, when a
// Note file it rewrites changed while it ran; it never parses the Note file. The
// result names the removed file, with a warning for every tree problem that concerns neither the
// Note nor its parent, for every Note file whose superseded-by entries could not be checked and for
// an emptied parent directory left in place; the Kind's sync hook (see notes) refreshes a stale
// capcom block of AGENTS.md afterwards.
func (a *app) removeNote(arg string, options *knowRmOptions) (output.Result, error) {
	removed, problems, err := a.notes(nil).Remove(arg, options.ifMatch, false)
	if err != nil {
		return output.Result{}, kindFailure(err)
	}

	result := noteRemoveResult(removed.ID, removed.Path, removed.Dropped[nodestore.RefSupersededBy])
	result.Warnings = append(result.Warnings, a.unwarnedTreeProblems(problems)...)
	result.Warnings = append(result.Warnings, supersededByUnreadWarnings(removed.Unread, removed.ID)...)
	result.Warnings = append(result.Warnings, cleanupWarnings(removed.LeftInPlace)...)
	addSyncEffects(&result, removed.Effects)
	return result, nil
}

// cleanupWarnings renders each of leftInPlace, an emptied directory the cleanup of a removal or a
// move left in place, as a warning naming the directory, why it was left, and that it was left.
func cleanupWarnings(leftInPlace []*nodestore.FileError) []string {
	warnings := make([]string, 0, len(leftInPlace))
	for _, failure := range leftInPlace {
		warnings = append(warnings, failure.Path+": "+flatten(failure.Message)+"; the empty directory was not removed")
	}
	return warnings
}

// supersededByUnreadWarnings renders each of unread, a Note file that does not parse, as a warning
// that its superseded-by entries were not checked for the Note id.
func supersededByUnreadWarnings(unread []string, id string) []string {
	warnings := make([]string, 0, len(unread))
	for _, path := range unread {
		warnings = append(warnings, fmt.Sprintf("%s: superseded-by not checked for %s: the file does not parse", path, id))
	}
	return warnings
}

// noteRemoveData is the --json data of know rm: the removed Note's id and path, the ids of the
// Notes whose superseded-by entries naming it were dropped, and whether the capcom block of
// AGENTS.md was refreshed.
type noteRemoveData struct {
	ID                  string   `json:"id"`
	Path                string   `json:"path"`
	SupersededByDropped []string `json:"superseded_by_dropped"`
	Agents              string   `json:"agents,omitempty"`
}

// noteRemoveResult is the result of know rm: the id and path of the removed Note file and the ids
// of the Notes whose superseded-by entries naming it were dropped.
func noteRemoveResult(id string, path string, supersededByDropped []string) output.Result {
	text := fmt.Sprintf("id: %s\npath: %s\n", id, path)
	if len(supersededByDropped) > 0 {
		text += "superseded_by_dropped: " + flowList(supersededByDropped) + "\n"
	}
	return output.Result{
		Kind: kindNoteRemove,
		Data: &noteRemoveData{
			ID:                  id,
			Path:                path,
			SupersededByDropped: append([]string{}, supersededByDropped...),
		},
		Text: text,
	}
}
