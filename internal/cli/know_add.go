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

// Help text of capcom know add.
const (
	knowAddLong = "Add a Note with the id given by --id, a kebab slug of 1 to 4 words that capcom never " +
		"generates, and the title given by --title; both are required.\n\n" +
		"--parent files it below another Note, --label and --ref set its Labels and Refs, and " +
		"--body-file reads its markdown body from a file, - for stdin.\n\n" +
		"A --ref inside the git directory is refused, since a clone never has it; one that does not " +
		"resolve here, or that git ignores, is written with a warning, as capcom check fails on it.\n\n" +
		"It prints the new Note's id, path and etag, and a refreshed line when the capcom block of " +
		"AGENTS.md was rewritten."
	knowAddExample = "  capcom know add --id release-steps --title \"How a release is cut\" --ref docs/release.md   # a Note that depends on a file\n" +
		"  capcom know add --id release-tags --parent release-steps --title \"Tagging a release\" --label release   # below release-steps\n" +
		"  capcom know add --id adr-0031 --title \"ADR 0031: one index per worktree\" --body-file -   # body from stdin"
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
		Long:        knowAddLong,
		Example:     knowAddExample,
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
	flags.StringVar(&options.id, flagID, "", "the new Note's id: a kebab slug of 1 to 4 words (required)")
	flags.StringVar(&options.title, flagTitle, "", "the Note's title (required)")
	flags.StringVar(&options.parent, flagParent, "", "add the Note below this one")
	a.completeFlag(cmd, flagParent, completeIDValue(noteIDs))
	flags.StringSliceVar(&options.labels, flagLabel, nil, "labels; repeat or separate with commas")
	flags.StringArrayVar(&options.refs, flagRef, nil, "a path or path#symbol the Note depends on; repeat for more")
	flags.StringVar(&options.bodyFile, flagBodyFile, "", "file holding the markdown body; - reads stdin")
	_ = cmd.MarkFlagRequired(flagID)    // fails only for an undefined flag
	_ = cmd.MarkFlagRequired(flagTitle) // fails only for an undefined flag
	return cmd
}

// addNote places the new Note in the Knowledge tree and writes it, reporting the etag of the
// bytes written, with a warning for every tree problem that concerns neither the Note's path nor
// its parent, unless the index refresh already warned about it, and for every --ref checkNewRefs
// warns about. A malformed --ref, or one inside the git directory, is refused before the Note's
// id is weighed against the tree. A Note added at the top of the tree refreshes a stale capcom
// block of AGENTS.md through the Kind's sync hook (see notes).
func (a *app) addNote(cmd *cobra.Command, options *knowAddOptions) (output.Result, error) {
	for _, ref := range options.refs {
		if err := knowledge.ValidateRef(ref); err != nil {
			return output.Result{}, output.Errorf(output.CodeInvalid, "%s", err.Error())
		}
	}
	unresolved, err := a.checkNewRefs(options.refs)
	if err != nil {
		return output.Result{}, err
	}
	result, err := createNode(
		a,
		noteVerbs,
		options.id,
		options.parent,
		func() (knowledge.Note, error) { return newNote(cmd, options) },
	)
	if err != nil {
		return output.Result{}, err
	}
	written, _ := result.Data.(map[string]string) // noteWriteResult's data
	result.Warnings = append(result.Warnings, refWarnings(unresolved, written["path"])...)
	return result, nil
}

// checkNewRefs checks refs, each of a valid form, against the working tree as know verify checks
// the Refs of a Note. It refuses with invalid the first Ref inside the git directory, naming it,
// since a clone never has that path, and returns the Findings of the Refs that do not resolve here
// or that git ignores, which refWarnings renders once the Note is written.
func (a *app) checkNewRefs(refs []string) ([]knowledge.Finding, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	findings, err := knowledge.Verify(nodestore.RefRepository(a.store), []knowledge.NoteFile{{Note: knowledge.Note{Refs: refs}}})
	if err != nil {
		return nil, output.AsError(err)
	}
	for _, finding := range findings {
		if finding.IsInGitDir {
			return nil, output.Errorf(output.CodeInvalid, "ref %q %s", finding.Ref, finding.Message)
		}
	}
	return findings, nil
}

// refWarnings renders the Findings of checkNewRefs as warnings at notePath, the Note file written:
// "path: ref "<ref>": message", one per Ref.
func refWarnings(findings []knowledge.Finding, notePath string) []string {
	warnings := make([]string, 0, len(findings))
	for _, finding := range findings {
		finding.Path = notePath
		warnings = append(warnings, finding.String())
	}
	return warnings
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
	status, err := syncAgentsBlock(a.store, agents.ModeRefresh)
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
	body, err := readTextInput(cmd, flagBodyFile, options.bodyFile)
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
