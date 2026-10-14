package cli

import (
	"slices"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/reflist"
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
// of --body-file, the text of --body-append-file, and the superseded-by list whose lookups decide
// the --add-superseded-by and --remove-superseded-by edits, its Entries left for the Note as read.
type noteInputs struct {
	body       string
	appendFile string
	successors reflist.List
}

// successorWords words know update's refusal of a Note superseded by itself.
var successorWords = refusalWords{self: "%s cannot be superseded by itself"}

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
				return output.UsageErrorf("know update needs at least one flag that changes the Note")
			}
			inputs, err := readNoteInputs(cmd, options)
			if err != nil {
				return err
			}
			if len(options.addSuccessors) > 0 || len(options.removeSuccessors) > 0 {
				if inputs.successors, err = a.successorList(args[0]); err != nil {
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
	flags.BoolVar(&options.isRetiring, flagRetire, false, "mark the Note retired: it and its subtree leave browsing")
	flags.BoolVar(&options.isUnretiring, flagUnretire, false, "clear the Note's retired mark")
	flags.StringArrayVar(&options.addSuccessors, flagAddSupersededBy, nil,
		"the id, alias or prefix of a Note that replaces this one; repeat for more")
	flags.StringArrayVar(&options.removeSuccessors, flagRemoveSupersededBy, nil,
		"the id, alias or prefix of a superseded-by entry to remove; repeat for more")
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

// successorList is the superseded-by list of the Note arg names, whose edits reflist decides as
// it decides deps and related: the index resolves an argument, and every usable Note, retired ones
// included, names an entry by exact id or Alias (noteNames). Its Entries are left unset, for
// applyNoteUpdate to fill from the Note as read.
func (a *app) successorList(arg string) (reflist.List, error) {
	holder, err := a.issueIndex.ResolveNote(arg)
	if err != nil {
		failure, _ := resolveFailure(err) // either way the command stops here
		return reflist.List{}, failure
	}
	notes, err := a.issueIndex.Notes(true)
	if err != nil {
		return reflist.List{}, err
	}
	return reflist.List{Holder: holder.ID, Resolve: a.resolveNoteID, Names: noteNames(notes)}, nil
}

// resolveNoteID resolves arg to the id of the Note it names, failing as index.Index.ResolveNote
// does: the reflist.ResolveFunc of a Note's superseded-by list.
func (a *app) resolveNoteID(arg string) (string, error) {
	entry, err := a.issueIndex.ResolveNote(arg)
	return entry.ID, err
}

// noteNames returns the reflist.NamesFunc over notes, a superseded-by entry as the file writes it
// naming the Note whose id it is, else the one Note of notes listing it as an Alias. An alias two
// Notes list names neither.
func noteNames(notes []index.NoteEntry) reflist.NamesFunc {
	byID := make(map[string]bool, len(notes))
	for _, note := range notes {
		byID[note.ID] = true
	}
	byAlias := map[string]string{}
	for _, note := range notes {
		for _, alias := range note.Aliases {
			if _, isClaimed := byAlias[alias]; isClaimed {
				byAlias[alias] = ""
				continue
			}
			byAlias[alias] = note.ID
		}
	}
	return func(entry string) (string, bool) {
		if byID[entry] {
			return entry, true
		}
		id := byAlias[entry]
		return id, id != ""
	}
}

// editSuccessors applies the addArgs, then the removeArgs, to entries, a superseded-by list as the
// file writes it, each decided by list against entries as the edits before it left them. An
// addition appends the id its argument resolves to unless an entry already names that Note, by id
// or by any Alias; a removal drops the entries it matches, and one matching none changes nothing.
// It fails on reflist's refusals, typed by refusalFailure with no path.
func editSuccessors(
	list reflist.List,
	addArgs []string,
	removeArgs []string,
	entries []string,
) ([]string, error) {
	entries = slices.Clone(entries)
	for _, arg := range addArgs {
		list.Entries = entries
		addition, err := list.Add(arg)
		if err != nil {
			return nil, refusalFailure(err, "", successorWords)
		}
		if !addition.IsPresent {
			entries = append(entries, addition.ID)
		}
	}
	for _, arg := range removeArgs {
		list.Entries = entries
		removal, err := list.Remove(arg)
		if err != nil {
			return nil, refusalFailure(err, "", successorWords)
		}
		// Drop's only failure is a removal that drops no entry: a silent no-op here.
		if dropped, err := removal.Drop(entries); err == nil {
			entries = withoutEntries(entries, dropped)
		}
	}
	return entries, nil
}

// updateNote changes the Note arg names through the node store of the Knowledge tree, which
// resolves arg through the index, refuses with conflict when --if-match is set and is not the
// file's etag, applies the flags to the parsed Note, then validates and writes the result
// atomically (Kind.Edit), whose sync hook (see notes) then refreshes a stale capcom block of
// AGENTS.md. Every failure names the Note file, or config.yaml when its raise fails. An edit whose
// written file carries a retired or superseded-by line raises a store below format 3 to 3 before
// the write (Kind.Edit), with a warning (editNode).
func (a *app) updateNote(
	cmd *cobra.Command,
	arg string,
	options *knowUpdateOptions,
	inputs noteInputs,
) (output.Result, error) {
	notes := a.notes(noteResolver{index: a.issueIndex})
	written, err := editNode(a, notes, arg, options.ifMatch, func(note knowledge.Note) (knowledge.Note, error) {
		return applyNoteUpdate(cmd, options, inputs, note)
	})
	if err != nil {
		return output.Result{}, err
	}
	result := noteWriteResult(written.ID, written.Path, written.ETag)
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
// superseded-by, whole body, body replacement, append. It fails on a new ref of the wrong form, on
// a superseded-by edit reflist refuses (editSuccessors) and on replacement text that does not
// occur exactly once.
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
	if len(options.addSuccessors) > 0 || len(options.removeSuccessors) > 0 {
		successors, err := editSuccessors(
			inputs.successors,
			options.addSuccessors,
			options.removeSuccessors,
			note.SupersededBy,
		)
		if err != nil {
			return knowledge.Note{}, err
		}
		note.SupersededBy = successors
	}
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
