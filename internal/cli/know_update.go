package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodestore"
)

// Flags of know update beyond those it shares with capcom update.
const (
	flagAddRef             = "add-ref"
	flagRemoveRef          = "remove-ref"
	flagRetire             = "retire"
	flagUnretire           = "unretire"
	flagAddSupersededBy    = "add-superseded-by"
	flagRemoveSupersededBy = "remove-superseded-by"
)

// knowUpdateOptions holds the flags of one capcom know update.
type knowUpdateOptions struct {
	title            string
	addLabels        []string
	removeLabels     []string
	addRefs          []string
	removeRefs       []string
	isRetiring       bool
	isUnretiring     bool
	addSuccessors    []string
	removeSuccessors []string
	bodyFile         string
	bodyAppend       string
	bodyAppendFile   string
	replaceOld       string
	replaceNew       string
	ifMatch          string
}

// Help text of capcom know update.
const (
	knowUpdateLong = "Change one Note; only the flags given change anything, and at least one is required. " +
		"--title, --add-label, --remove-label, --add-ref and --remove-ref edit its frontmatter. --retire " +
		"marks it retired, so it and its subtree leave browsing, --unretire clears that, and " +
		"--add-superseded-by names the Notes that replace it. --body-file replaces the whole body, " +
		"--body-append and --body-append-file add to it, and --body-replace-old with --body-replace-new " +
		"replaces text that occurs exactly once. It prints the Note's id, path and new etag."
	knowUpdateExample = "  capcom know update release-steps --add-label release --if-match <etag>   # etag from know show\n" +
		"  capcom know update adr-0012 --retire --add-superseded-by adr-0031   # replaced in whole\n" +
		"  capcom know update release-steps --body-append \"Run the gate first.\"   # add to the body\n" +
		"  capcom know update release-steps --body-file notes.md   # replace the whole body"
)

// noteInputs is what know update reads and resolves before it touches the Note: the whole new body
// of --body-file, the text of --body-append-file, and the ids the --add-superseded-by arguments
// resolve to.
type noteInputs struct {
	body       string
	appendFile string
	successors []string
}

// newKnowUpdateCommand changes the title, labels, refs, retired flag, superseded-by list and body
// of one Note. Only the flags given change anything. --body-file replaces the whole body and so
// excludes the flags that edit it; --retire excludes --unretire.
func (a *app) newKnowUpdateCommand() *cobra.Command {
	options := &knowUpdateOptions{}
	cmd := &cobra.Command{
		Use:         "update <id|alias|prefix>",
		Short:       "Change a Note's title, labels, refs, retirement or body",
		Long:        knowUpdateLong,
		Example:     knowUpdateExample,
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			if !hasAnyFlag(cmd, flagTitle, flagAddLabel, flagRemoveLabel, flagAddRef, flagRemoveRef,
				flagRetire, flagUnretire, flagAddSupersededBy, flagRemoveSupersededBy,
				flagBodyFile, flagBodyAppend, flagBodyAppendFile, flagBodyReplaceOld) {
				return output.UsageErrorf("know update needs at least one flag that changes the note")
			}
			inputs, err := readNoteInputs(cmd, options)
			if err != nil {
				return err
			}
			if len(options.addSuccessors) > 0 {
				if inputs.successors, err = a.resolveSuccessors(args[0], options.addSuccessors); err != nil {
					return err
				}
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
	flags.BoolVar(&options.isRetiring, flagRetire, false, "mark the note retired: it and its subtree leave browsing")
	flags.BoolVar(&options.isUnretiring, flagUnretire, false, "clear the note's retired mark")
	flags.StringArrayVar(&options.addSuccessors, flagAddSupersededBy, nil,
		"the id, alias or prefix of a note that replaces this one; repeat for more")
	flags.StringArrayVar(&options.removeSuccessors, flagRemoveSupersededBy, nil,
		"a superseded-by id to remove; repeat for more")
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
	cmd.MarkFlagsMutuallyExclusive(flagRetire, flagUnretire)
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

// resolveSuccessors resolves each of successorArgs, the --add-superseded-by arguments, to the id of
// the Note it names, so the Note stores canonical ids only. arg names the Note they are added to;
// an argument naming no Note, several or the Note itself is refused, at the Note's file.
func (a *app) resolveSuccessors(arg string, successorArgs []string) ([]string, error) {
	holder, err := a.issueIndex.ResolveNote(arg)
	if err != nil {
		failure, _ := resolveFailure(err) // either way the command stops here
		return nil, failure
	}
	successors := make([]string, 0, len(successorArgs))
	for _, successorArg := range successorArgs {
		successor, err := a.issueIndex.ResolveNote(successorArg)
		if err != nil {
			failure, _ := resolveFailure(err) // either way the command stops here
			failure.Path = holder.Path
			return nil, failure
		}
		if successor.ID == holder.ID {
			return nil, &output.Error{
				Code:    output.CodeInvalid,
				Path:    holder.Path,
				Message: fmt.Sprintf("%s cannot be superseded by itself", holder.ID),
			}
		}
		successors = append(successors, successor.ID)
	}
	return successors, nil
}

// updateNote changes the Note arg names through the node store of the Knowledge tree, which
// resolves arg through the index, refuses with conflict when --if-match is set and is not the
// file's etag, applies the flags to the parsed Note, then validates and writes the result
// atomically (Kind.Edit), whose sync hook (see notes) then refreshes a stale capcom block of
// AGENTS.md. Every failure names the Note file, or config.yaml when its raise fails. An edit that
// retires the Note or gives it a new superseded-by entry raises a store below format 3 to 3 before
// the write (Kind.Edit), with a warning.
func (a *app) updateNote(
	cmd *cobra.Command,
	arg string,
	options *knowUpdateOptions,
	inputs noteInputs,
) (output.Result, error) {
	notes := a.notes(noteResolver{index: a.issueIndex})
	written, err := editNode(notes, arg, options.ifMatch, func(note knowledge.Note) (knowledge.Note, error) {
		return applyNoteUpdate(cmd, options, inputs, note)
	})
	if err != nil {
		return output.Result{}, err
	}
	result := noteWriteResult(written.ID, written.Path, written.ETag)
	a.warnRaisedFormat(&result, written.Effects)
	addSyncEffects(&result, written.Effects)
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

// applyNoteUpdate applies the changed flags to note in the order title, labels, refs, retired,
// superseded-by, whole body, body replacement, append. It fails on a new ref of the wrong form and
// on replacement text that does not occur exactly once.
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
	note.Labels = edit.List(note.Labels, options.addLabels, options.removeLabels)
	for _, ref := range options.addRefs {
		if err := knowledge.ValidateRef(ref); err != nil {
			return knowledge.Note{}, err
		}
	}
	// Refs merge like labels: an ordered set, added before removed.
	note.Refs = edit.List(note.Refs, options.addRefs, options.removeRefs)
	if options.isRetiring {
		note.IsRetired = true
	}
	if options.isUnretiring {
		note.IsRetired = false
	}
	// Successors merge like refs, the added ones already resolved to ids.
	note.SupersededBy = edit.List(note.SupersededBy, inputs.successors, options.removeSuccessors)
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
