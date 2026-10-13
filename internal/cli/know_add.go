package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/agents"
	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodestore"
)

// Flags of capcom know add.
const (
	flagRef = "ref"
)

// kindNoteWrite is the --json kind of every command that writes a Note.
const kindNoteWrite = "note-write"

// knowAddOptions holds the flags of one capcom know add.
type knowAddOptions struct {
	id       string
	title    string
	parent   string
	labels   []string
	refs     []string
	bodyFile string
}

// newKnowAddCommand writes a new Note file. The id is always the caller's choice, never generated.
func (a *app) newKnowAddCommand() *cobra.Command {
	options := &knowAddOptions{}
	cmd := &cobra.Command{
		Use:         "add --id <slug> --title <title>",
		Short:       "Add a Note with a chosen id",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			result, err := a.addNote(cmd, options)
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&options.id, flagID, "", "the new note's id: a kebab slug of 1 to 4 words (required)")
	flags.StringVar(&options.title, flagTitle, "", "the note's title (required)")
	flags.StringVar(&options.parent, flagParent, "", "add the note below this one")
	flags.StringSliceVar(&options.labels, flagLabel, nil, "labels; repeat or separate with commas")
	flags.StringArrayVar(&options.refs, flagRef, nil, "a path or path#symbol the note depends on; repeat for more")
	flags.StringVar(&options.bodyFile, flagBodyFile, "", "file holding the markdown body; - reads stdin")
	_ = cmd.MarkFlagRequired(flagID)    // fails only for an undefined flag
	_ = cmd.MarkFlagRequired(flagTitle) // fails only for an undefined flag
	return cmd
}

// addNote places the new Note in the Knowledge tree and writes it, reporting the etag of the
// bytes written, with a warning for every tree problem that concerns neither the Note's path nor
// its parent, unless the index refresh already warned about it. A malformed --ref is refused
// before the Note's id is weighed against the tree. A Note added at the top of the tree refreshes
// a stale capcom block of AGENTS.md through the Kind's sync hook (see notes).
func (a *app) addNote(cmd *cobra.Command, options *knowAddOptions) (output.Result, error) {
	for _, ref := range options.refs {
		if err := knowledge.ValidateRef(ref); err != nil {
			return output.Result{}, output.Errorf(output.CodeInvalid, "%s", err.Error())
		}
	}
	return createNode(
		a,
		noteVerbs,
		options.id,
		options.parent,
		func() (knowledge.Note, error) { return newNote(cmd, options) },
	)
}

// noteWriteResult is the result of a command that wrote a Note: its id, path and new etag.
func noteWriteResult(id string, path string, etag string) output.Result {
	return output.Result{
		Kind: kindNoteWrite,
		Data: map[string]string{"id": id, "path": path, "etag": etag},
		Text: fmt.Sprintf("id: %s\npath: %s\netag: %s\n", id, path, etag),
	}
}

// notes is the node store of the Knowledge tree, resolving Note arguments through resolver, with
// syncAgents as its sync hook: each Note write that may have changed the Knowledge top level (a
// Create at the top, and every Edit, Move and Remove) refreshes the capcom block of AGENTS.md, so
// check stays green.
func (a *app) notes(resolver nodestore.Resolver) *nodestore.Kind[knowledge.Note] {
	return nodestore.Notes(a.store, resolver).WithSync(a.syncAgents)
}

// syncAgents refreshes the capcom block of AGENTS.md, reporting whether it was stale and so
// rewritten. Only a stale block is rewritten; an AGENTS.md without the block is left alone.
func (a *app) syncAgents() (bool, error) {
	status, err := agents.Sync(a.store.WorktreeRoot, a.store.KnowledgeDir(), agents.ModeRefresh)
	if err != nil {
		return false, err
	}
	return status.State == agents.StateStale, nil
}

// addSyncEffects renders the outcome of the AGENTS.md sync hook a Note write ran (see notes) onto
// result, a noteWriteResult or a noteRemoveResult. The Note is already written, so a failed refresh
// is a warning, not a failed command; a refresh is reported in the text and as an agents key in
// the --json data.
func addSyncEffects(result *output.Result, effects nodestore.Effects) {
	if effects.SyncFailure != nil {
		warning := flatten(agentsFailure(effects.SyncFailure).Error()) + "; the capcom block was not refreshed"
		result.Warnings = append(result.Warnings, warning)
		return
	}
	if !effects.IsSynced {
		return
	}
	switch data := result.Data.(type) {
	case map[string]string:
		data["agents"] = blockRefreshed
	case *noteRemoveData:
		data.Agents = blockRefreshed
	}
	result.Text += "refreshed: " + agents.FileName + "\n"
}

// newNote builds the Note the flags describe: labels without repeats or empty entries, and the
// body file's text as noteBody shapes it.
func newNote(cmd *cobra.Command, options *knowAddOptions) (knowledge.Note, error) {
	note := knowledge.Note{
		Title:  options.title,
		Labels: edit.List(nil, options.labels, nil),
		Refs:   options.refs,
	}
	if options.bodyFile == "" {
		return note, nil
	}
	body, err := readTextInput(cmd, options.bodyFile)
	if err != nil {
		return knowledge.Note{}, err
	}
	note.Body = noteBody(body)
	return note, nil
}

// noteBody shapes text as a whole Note body: CRLF line endings read as LF, blank text is no body,
// and any other text follows the blank line after the frontmatter and ends in a line break.
func noteBody(text string) string {
	text = edit.NormalizeNewlines(text)
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return "\n" + edit.WithFinalNewline(text)
}
