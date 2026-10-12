package cli

import (
	"bytes"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/tree"
)

// Flags of know update beyond those it shares with capcom update.
const (
	flagAddRef    = "add-ref"
	flagRemoveRef = "remove-ref"
)

// knowUpdateOptions holds the flags of one capcom know update.
type knowUpdateOptions struct {
	title          string
	addLabels      []string
	removeLabels   []string
	addRefs        []string
	removeRefs     []string
	bodyFile       string
	bodyAppend     string
	bodyAppendFile string
	replaceOld     string
	replaceNew     string
	ifMatch        string
}

// noteInputs is the text know update reads from files or stdin before it touches the Note: the
// whole new body of --body-file and the text of --body-append-file.
type noteInputs struct {
	body       string
	appendFile string
}

// newKnowUpdateCommand changes the title, labels, refs and body of one Note. Only the flags given
// change anything. --body-file replaces the whole body and so excludes the flags that edit it.
func (a *app) newKnowUpdateCommand() *cobra.Command {
	options := &knowUpdateOptions{}
	cmd := &cobra.Command{
		Use:         "update <id|alias|prefix>",
		Short:       "Change a Note's title, labels, refs or body",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			if !hasAnyFlag(cmd, flagTitle, flagAddLabel, flagRemoveLabel, flagAddRef, flagRemoveRef,
				flagBodyFile, flagBodyAppend, flagBodyAppendFile, flagBodyReplaceOld) {
				return UsageErrorf("know update needs at least one flag that changes the note")
			}
			inputs, err := readNoteInputs(cmd, options)
			if err != nil {
				return err
			}
			result, err := a.updateNote(cmd, args[0], options, inputs)
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&options.title, flagTitle, "", "new title")
	flags.StringSliceVar(&options.addLabels, flagAddLabel, nil, "labels to add")
	flags.StringSliceVar(&options.removeLabels, flagRemoveLabel, nil, "labels to remove")
	flags.StringArrayVar(&options.addRefs, flagAddRef, nil, "a path or path#symbol to add to the refs; repeat for more")
	flags.StringArrayVar(&options.removeRefs, flagRemoveRef, nil, "a ref to remove; repeat for more")
	flags.StringVar(&options.bodyFile, flagBodyFile, "", "file holding the whole new markdown body; - reads stdin")
	flags.StringVar(&options.bodyAppend, flagBodyAppend, "", "text to append to the body, taken literally")
	flags.StringVar(&options.bodyAppendFile, flagBodyAppendFile, "", "file to append to the body; - reads stdin")
	flags.StringVar(&options.replaceOld, flagBodyReplaceOld, "", "body text to replace; must occur exactly once")
	flags.StringVar(&options.replaceNew, flagBodyReplaceNew, "", "text that replaces --body-replace-old")
	flags.StringVar(&options.ifMatch, flagIfMatch, "", "only write when the file's etag is this one (see know show)")
	cmd.MarkFlagsMutuallyExclusive(flagBodyAppend, flagBodyAppendFile)
	cmd.MarkFlagsMutuallyExclusive(flagBodyFile, flagBodyAppend)
	cmd.MarkFlagsMutuallyExclusive(flagBodyFile, flagBodyAppendFile)
	cmd.MarkFlagsMutuallyExclusive(flagBodyFile, flagBodyReplaceOld)
	cmd.MarkFlagsRequiredTogether(flagBodyReplaceOld, flagBodyReplaceNew)
	return cmd
}

// readNoteInputs reads the files --body-file and --body-append-file name, each only when given.
func readNoteInputs(cmd *cobra.Command, options *knowUpdateOptions) (noteInputs, error) {
	var inputs noteInputs
	var err error
	if cmd.Flags().Changed(flagBodyFile) {
		if inputs.body, err = readTextInput(cmd, options.bodyFile); err != nil {
			return noteInputs{}, err
		}
	}
	if cmd.Flags().Changed(flagBodyAppendFile) {
		if inputs.appendFile, err = readTextInput(cmd, options.bodyAppendFile); err != nil {
			return noteInputs{}, err
		}
	}
	return inputs, nil
}

// updateNote changes the Note arg names through the node store of the Knowledge tree, which
// resolves arg through the index, refuses with conflict when --if-match is set and is not the
// file's etag, applies the flags to the parsed Note, then validates and writes the result
// atomically (Kind.Edit); it then refreshes a stale capcom block of AGENTS.md. Every failure names
// the Note file.
func (a *app) updateNote(
	cmd *cobra.Command,
	arg string,
	options *knowUpdateOptions,
	inputs noteInputs,
) (Result, error) {
	notes := nodestore.Notes(a.store, noteResolver{index: a.issueIndex})
	written, err := editNode(notes, arg, options.ifMatch, func(note knowledge.Note) (knowledge.Note, error) {
		return applyNoteUpdate(cmd, options, inputs, note)
	})
	if err != nil {
		return Result{}, err
	}
	result := noteWriteResult(written.ID, written.Path, written.ETag)
	a.refreshAgentsAfterWrite(&result)
	return result, nil
}

// noteResolver resolves a Note argument through the index: the Resolver of the Knowledge tree's
// node store.
type noteResolver struct {
	index *index.Index
}

// Resolve resolves arg to the Note it names, failing as index.Index.ResolveNote does.
func (r noteResolver) Resolve(arg string) (nodestore.Target, error) {
	entry, err := r.index.ResolveNote(arg)
	if err != nil {
		return nodestore.Target{}, err
	}
	return nodestore.Target{ID: entry.ID, Path: entry.Path}, nil
}

// applyNoteUpdate applies the changed flags to note in the order title, labels, refs, whole body,
// body replacement, append. It fails on a new ref of the wrong form and on replacement text that
// does not occur exactly once.
func applyNoteUpdate(
	cmd *cobra.Command,
	options *knowUpdateOptions,
	inputs noteInputs,
	note knowledge.Note,
) (knowledge.Note, error) {
	flags := cmd.Flags()
	if flags.Changed(flagTitle) {
		note.Title = options.title
	}
	note.Labels = edit.Labels(note.Labels, options.addLabels, options.removeLabels)
	for _, ref := range options.addRefs {
		if err := knowledge.ValidateRef(ref); err != nil {
			return knowledge.Note{}, err
		}
	}
	// Refs merge like labels: an ordered set, added before removed.
	note.Refs = edit.Labels(note.Refs, options.addRefs, options.removeRefs)
	if flags.Changed(flagBodyFile) {
		note.Body = noteBody(inputs.body)
	}
	if flags.Changed(flagBodyReplaceOld) {
		replaced, err := edit.ReplaceBody(note.Body, options.replaceOld, options.replaceNew)
		if err != nil {
			return knowledge.Note{}, err
		}
		note.Body = replaced
	}
	if flags.Changed(flagBodyAppend) {
		note.Body = edit.AppendBody(note.Body, options.bodyAppend)
	}
	if flags.Changed(flagBodyAppendFile) {
		note.Body = edit.AppendBody(note.Body, inputs.appendFile)
	}
	return note, nil
}

// writeNoteFile validates note and writes it atomically to path, relative to the working tree
// root with forward slashes, returning the new etag. A Note that does not validate fails invalid,
// citing its first problem's line, and is not written.
func (a *app) writeNoteFile(path string, note knowledge.Note) (string, error) {
	if problems := knowledge.Validate(path, note); len(problems) > 0 {
		messages := make([]string, 0, len(problems))
		for _, problem := range problems {
			messages = append(messages, problem.Message)
		}
		return "", &Error{
			Code:    CodeInvalid,
			Path:    path,
			Line:    problems[0].Line,
			Message: strings.Join(messages, problemSeparator),
		}
	}
	content := note.Serialize()
	if err := tree.WriteFileAtomic(a.absoluteIssuePath(path), bytes.NewReader(content)); err != nil {
		return "", asError(err)
	}
	return edit.ETag(content), nil
}
