package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom"
	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/seed"
	"github.com/airiclenz/capcom/internal/tree"
)

// Help text of capcom know seed.
const (
	knowSeedLong = "Add the base set of Notes that capcom ships to the Knowledge tree: one root Note " +
		"and the Notes below it, each a rule a repository may adopt.\n\n" +
		"It adds every Note of the set the store lacks, the root first, and without --update never " +
		"changes a Note that exists. An added Note carries two Labels: seed:<root id>, and seed-text:<version>:<hash>, " +
		"which holds the capcom version and a hash of the text it was added with.\n\n" +
		"It prints one line per Note of the set, its id and its outcome: added; skipped (same) for a " +
		"Note whose title and body are the set's; skipped (retired) for a retired Note; " +
		"skipped (alias of <id>) when the id is an Alias of another Note. A retired root Note skips " +
		"the whole set as retired.\n\n" +
		"A Note with another text is skipped (outdated) when its text is still the text it was added " +
		"with, so the set has changed since, and skipped (differs) otherwise: it was edited, or it is " +
		"the repository's own.\n\n" +
		"A root Note that exists without the seed:<root id> Label is the repository's own: the " +
		"command fails and writes nothing. A write that fails stops the run; the Notes added before " +
		"it stay, and running the command again adds the rest.\n\n" +
		"--update also replaces the title and body of every outdated Note with the set's, and prints " +
		"updated for it. It puts the new seed-text Label in place of the old one and keeps every other " +
		"Label, Ref and Alias. It never changes a Note that differs or a retired one.\n\n" +
		"--dry-run prints the same lines and writes nothing, with --update too.\n\n" +
		"--show <id> prints one Note of the set as this capcom ships it, its id, its title and its " +
		"body, and writes nothing: the text to compare with the Note the store holds. An id the set " +
		"does not hold fails with not_found and names the ids it holds. It excludes --dry-run and --update."
	knowSeedExample = "  capcom know seed             # add the Notes of the base set the store lacks\n" +
		"  capcom know seed --dry-run   # print what it would add, and write nothing\n" +
		"  capcom know seed --update    # also replace the Notes capcom ships a newer text for\n" +
		"  capcom know seed --show <id> # print the text capcom ships for one Note of the set"
)

// flagShow prints one Note of the base set as it is embedded, in place of seeding.
const flagShow = "show"

// flagSeedUpdate also replaces the text of every outdated Note of the base set.
const flagSeedUpdate = "update"

// The --json kinds of capcom know seed.
const (
	kindKnowSeed = "know-seed"
	// kindKnowSeedShow is the kind under --show.
	kindKnowSeedShow = "know-seed-show"
)

// The outcomes capcom know seed reports for a Note of the base set.
const (
	seedAdded           = "added"
	seedUpdated         = "updated"
	seedSkippedSame     = "skipped (same)"
	seedSkippedOutdated = "skipped (outdated)"
	seedSkippedDiffers  = "skipped (differs)"
	seedSkippedRetired  = "skipped (retired)"
	// seedSkippedAliasFormat takes the id of the Note that holds the set Note's id as an Alias.
	seedSkippedAliasFormat = "skipped (alias of %s)"
)

// unseededRootFormat is the failure for a root Note that exists without the seed Label: the root
// id, then the Label.
const unseededRootFormat = "Note %s was not seeded: it has no Label %s, so capcom know seed writes nothing"

// unknownSeedNoteFormat is the failure of --show for an id the base set does not hold: the id,
// then the ids of the set.
const unknownSeedNoteFormat = "no Note %s in the base set: it holds %s"

// seedShowHint is the hint of that failure, in place of the default of not_found, which searches
// the Issues.
const seedShowHint = "capcom know seed --" + flagShow + " <id>"

// seedShowFormat is the text of --show: the id, the title, a blank line, then the body.
const seedShowFormat = "id: %s\ntitle: %s\n\n%s\n"

// seedIDSeparator stands between the ids of the base set in a message.
const seedIDSeparator = ", "

// noteFileExtension ends the name of a Note file.
const noteFileExtension = ".md"

// seedLineFormat is one text line of capcom know seed: the id, then the outcome.
const seedLineFormat = "%s  %s\n"

// knowSeedData is the --json data of capcom know seed.
type knowSeedData struct {
	IsDryRun bool `json:"dry_run"`
	// Notes holds the Notes of the base set in the order they were weighed, a parent before its
	// children; a run a failed write stopped holds the Notes up to that write.
	Notes []seedNoteData `json:"notes"`
}

// seedNoteData is one Note of the base set in the --json data of capcom know seed.
type seedNoteData struct {
	ID string `json:"id"`
	// Path is the Note's file, relative to the working tree root: for a Note that was not added,
	// the file that holds the id, which an update rewrites, or, when no file does, the file an add
	// would write.
	Path    string `json:"path"`
	Outcome string `json:"outcome"`
}

// seedShowData is the --json data of capcom know seed --show: one Note of the base set as it is
// embedded.
type seedShowData struct {
	ID string `json:"id"`
	// ParentID is empty for the root Note.
	ParentID string `json:"parent"`
	Title    string `json:"title"`
	// Body is the embedded body byte for byte; it keeps the blank line that follows the
	// frontmatter.
	Body string `json:"body"`
}

// seedStep is what capcom know seed does with one Note of the base set.
type seedStep struct {
	note seed.Note
	data seedNoteData
}

// isWrite reports whether the step writes its Note, by adding it or by updating it, unless the run
// is a dry run.
func (s seedStep) isWrite() bool {
	return s.data.Outcome == seedAdded || s.data.Outcome == seedUpdated
}

// newKnowSeedCommand adds the Notes of the base set the store lacks, with --update also replaces
// the text of the outdated ones, or with --show prints one of them as it is embedded. Every id is the set's own, a slug read from the Note's file name: none
// is generated, derived or renamed.
func (a *app) newKnowSeedCommand() *cobra.Command {
	var isDryRun bool
	var isUpdating bool
	var shownID string
	cmd := &cobra.Command{
		Use:         "seed [[--update] [--dry-run] | --show <id>]",
		Short:       "Add the base set of Notes capcom ships; only --update changes a Note that exists",
		Long:        knowSeedLong,
		Example:     knowSeedExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			var result output.Result
			var err error
			if cmd.Flags().Changed(flagShow) {
				result, err = a.showSeedNote(shownID)
			} else {
				result, err = a.seedNotes(isUpdating, isDryRun)
			}
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	cmd.Flags().BoolVar(&isDryRun, flagDryRun, false, "print each Note's outcome and write nothing")
	cmd.Flags().StringVar(&shownID, flagShow, "",
		"print the embedded title and body of this Note of the base set, and write nothing")
	cmd.Flags().BoolVar(&isUpdating, flagSeedUpdate, false,
		"also replace the title and body of every Note whose text is still what it was added with")
	cmd.MarkFlagsMutuallyExclusive(flagShow, flagDryRun)
	cmd.MarkFlagsMutuallyExclusive(flagShow, flagSeedUpdate)
	a.completeFlag(cmd, flagShow, a.completeSeedNoteIDs)
	return cmd
}

// seedSet loads the base set through the app's loader, seed.Load unless a test set another.
func (a *app) seedSet() (seed.Set, error) {
	if a.loadSeed == nil {
		return seed.Load()
	}
	return a.loadSeed()
}

// showSeedNote returns the Note id of the base set as it is embedded: its id, parent, title and
// body. It reads nothing of the store and writes nothing. An id the set does not hold fails with
// not_found, naming the ids the set holds.
func (a *app) showSeedNote(id string) (output.Result, error) {
	set, err := a.seedSet()
	if err != nil {
		return output.Result{}, output.AsError(err)
	}
	note, isHeld := set.Note(id)
	if !isHeld {
		ids := strings.Join(seedNoteIDs(set), seedIDSeparator)
		return output.Result{}, &output.Error{
			Code:    output.CodeNotFound,
			Message: fmt.Sprintf(unknownSeedNoteFormat, id, ids),
			Hint:    seedShowHint,
		}
	}
	return output.Result{
		Kind: kindKnowSeedShow,
		Data: seedShowData{ID: note.ID, ParentID: note.ParentID, Title: note.Title, Body: note.Body},
		Text: fmt.Sprintf(seedShowFormat, note.ID, note.Title, strings.Trim(note.Body, "\n")),
	}, nil
}

// seedNoteIDs returns the ids of the Notes of set, the root first.
func seedNoteIDs(set seed.Set) []string {
	notes := set.Notes()
	ids := make([]string, 0, len(notes))
	for _, note := range notes {
		ids = append(ids, note.ID)
	}
	return ids
}

// completeSeedNoteIDs completes the value of --show: the ids of the base set that start with
// toComplete, the root first, each described by its title. They come from the binary, so they
// complete outside a store too. A set that does not load leaves no candidates; the failure is
// written to cobra's completion debug log, as the shell shows no errors.
func (a *app) completeSeedNoteIDs(
	_ *cobra.Command, _ []string, toComplete string,
) ([]cobra.Completion, cobra.ShellCompDirective) {
	set, err := a.seedSet()
	if err != nil {
		cobra.CompDebugln(err.Error(), false)
		return nil, completionDirective
	}
	var completions []cobra.Completion
	for _, note := range set.Notes() {
		if strings.HasPrefix(note.ID, toComplete) {
			completions = append(completions, cobra.CompletionWithDesc(note.ID, note.Title))
		}
	}
	return completions, completionDirective
}

// seedNotes weighs every Note of the base set against the Knowledge tree (planSeed), then, unless
// isDryRun, writes one Note at a time, a parent before its children: it adds the Notes the tree
// lacks and, when isUpdating, replaces the text of the outdated ones (writeSeedStep). The result
// names every Note with its outcome. A failed write stops the run: the result then holds the
// Notes up to it and the failure, and the Notes written before it stay. The add of the root Note
// and every update refresh a stale capcom block (see notes), reported as know add reports it.
func (a *app) seedNotes(isUpdating bool, isDryRun bool) (output.Result, error) {
	set, err := a.seedSet()
	if err != nil {
		return output.Result{}, output.AsError(err)
	}
	steps, err := a.planSeed(set, isUpdating)
	if err != nil {
		return output.Result{}, err
	}
	data := knowSeedData{IsDryRun: isDryRun, Notes: make([]seedNoteData, 0, len(steps))}
	var failure *output.Error
	var warnings []string
	var effects []nodestore.Effects
	for _, step := range steps {
		if step.isWrite() && !isDryRun {
			written, problems, err := a.writeSeedStep(set, step)
			if err != nil {
				failure = output.AsError(err)
				break
			}
			step.data.Path = written.Path
			effects = append(effects, written.Effects)
			for _, warning := range a.unwarnedTreeProblems(problems) {
				if !slices.Contains(warnings, warning) {
					warnings = append(warnings, warning)
				}
			}
		}
		data.Notes = append(data.Notes, step.data)
	}
	result := output.Result{
		Kind:     kindKnowSeed,
		Data:     data,
		Text:     seedText(data),
		Warnings: warnings,
		Failure:  failure,
	}
	for _, written := range effects {
		a.addSyncEffects(&result, written)
	}
	return result, nil
}

// writeSeedStep does the write of step, a step that adds or updates its Note of set (isWrite), and
// notes it for the Tree lock warning (app.noteKindWrite). It returns the Note written and, for an
// add, the problems of the tree it was added to. A failure is typed for the command.
func (a *app) writeSeedStep(set seed.Set, step seedStep) (nodestore.Written, tree.Errors, error) {
	if step.data.Outcome == seedUpdated {
		written, err := a.updateSeedNote(set, step.note)
		return written, nil, err
	}
	written, problems, err := a.addSeedNote(set, step.note)
	if err != nil {
		return nodestore.Written{}, nil, a.kindFailure(err, noteIDs)
	}
	a.noteKindWrite(noteIDs, written.ID)
	return written, problems, nil
}

// addSeedNote writes note, a Note of set the tree lacks, below its parent in the set, with the
// set's title and body as they are embedded and the two seed Labels of this capcom version. It
// goes through the node store's Create, as know add does, so it never replaces a file.
func (a *app) addSeedNote(set seed.Set, note seed.Note) (nodestore.Written, tree.Errors, error) {
	return a.notes(nil).Create(note.ID, note.ParentID, func() (knowledge.Note, error) {
		return knowledge.Note{
			Title:  note.Title,
			Labels: set.Labels(note, capcom.Version()),
			Body:   note.Body,
		}, nil
	})
}

// updateSeedNote rewrites the store's Note of the id of note, a Note of set, through the write
// path of know update (editNode): it takes the set's title and body as they are embedded, so the
// Note then stands as seed.Same, and the seed-text Label of this capcom version in place of the
// old one (seed.Set.Relabel). Every other Label, the Refs, the Aliases and the rest of the
// frontmatter stay. The write is noted and its failure typed as editNode does.
func (a *app) updateSeedNote(set seed.Set, note seed.Note) (nodestore.Written, error) {
	notes := a.notes(noteResolver{index: a.issueIndex})
	return editNode(a, notes, noteIDs, note.ID, "", func(existing knowledge.Note) (knowledge.Note, error) {
		existing.Title = note.Title
		existing.Body = note.Body
		existing.Labels = set.Relabel(existing.Labels, note, capcom.Version())
		return existing, nil
	})
}

// seedText renders data as text: one line per Note, its id and outcome, then dryRunLine on a dry
// run.
func seedText(data knowSeedData) string {
	var text strings.Builder
	for _, note := range data.Notes {
		fmt.Fprintf(&text, seedLineFormat, note.ID, note.Outcome)
	}
	if data.IsDryRun {
		text.WriteString(dryRunLine + "\n")
	}
	return text.String()
}

// planSeed weighs every Note of set against the Knowledge tree as it is on disk and returns one
// step per Note, a parent before its children, writing nothing. A Note the tree lacks, as an id
// and as an Alias, is to be added; one it holds is skipped or, when isUpdating and it is outdated,
// to be updated, see seedOutcome. It fails with invalid,
// at the file, when a file that holds an id of the set cannot be read or parsed, and when the root
// Note exists without the seed Label, so a tree that was never seeded is left alone.
func (a *app) planSeed(set seed.Set, isUpdating bool) ([]seedStep, error) {
	nodes, err := a.loadKnowledgeTree()
	if err != nil {
		return nil, err
	}
	notes := parentFirst(set)
	steps := make([]seedStep, 0, len(notes))
	// paths holds, for each Note weighed, the file that holds its id or that an add would write.
	paths := make(map[string]string, len(notes))
	root := set.Root()
	isRootRetired := false
	for _, note := range notes {
		node, isFound := nodes.Lookup(note.ID)
		var existing knowledge.Note
		path := seedNotePath(a.store.KnowledgeDir(), paths[note.ParentID], note)
		if isFound {
			path = node.Path
			reading, err := parseNodeFile(a, nodestore.NoteReader(), a.store.Relative(path))
			if err != nil {
				return nil, err
			}
			existing = reading.Node
		}
		if isFound && note.ID == root.ID {
			if !set.IsSeeded(existing.Labels) {
				return nil, a.unseededRootFailure(set, path)
			}
			isRootRetired = existing.IsRetired
		}
		paths[note.ID] = path
		outcome := seedSkippedRetired
		if !isRootRetired {
			outcome = seedOutcome(set, note, node, isFound, existing, isUpdating)
		}
		steps = append(steps, seedStep{note: note, data: seedNoteData{
			ID:      note.ID,
			Path:    a.store.Relative(path),
			Outcome: outcome,
		}})
	}
	return steps, nil
}

// loadKnowledgeTree loads the Knowledge tree from disk as the node store loads it for a write. The
// problems of single files do not stop it: each write reports those that concern it.
func (a *app) loadKnowledgeTree() (*tree.Tree, error) {
	nodes, err := tree.LoadContent(a.store.KnowledgeDir(), nodestore.NoteDescriptor().Noun, a.notes(nil).ReadContent)
	var problems tree.Errors
	if err != nil && !errors.As(err, &problems) {
		return nil, a.kindFailure(err, noteIDs)
	}
	return nodes, nil
}

// unseededRootFailure is the failure for the root Note of set that exists at path without the seed
// Label: invalid, at its file.
func (a *app) unseededRootFailure(set seed.Set, path string) error {
	root := set.Root()
	// The seed Label is the first of the two Labels; it does not depend on the version.
	label := set.Labels(root, capcom.Version())[0]
	return &output.Error{
		Code:    output.CodeInvalid,
		Path:    a.store.Relative(path),
		Message: fmt.Sprintf(unseededRootFormat, root.ID, label),
	}
}

// seedOutcome is the outcome of note, a Note of set, below a root Note that is not retired. node
// and existing are the tree's node that answers to its id and that node's file, when isFound. A
// Note the tree lacks is added; an id that is an Alias of another Note, a retired Note and a Note
// with the set's text or an edited one are skipped as such. A Note in the state seed.Outdated is
// updated when isUpdating and skipped as outdated otherwise.
func seedOutcome(
	set seed.Set,
	note seed.Note,
	node tree.Node,
	isFound bool,
	existing knowledge.Note,
	isUpdating bool,
) string {
	switch {
	case !isFound:
		return seedAdded
	case node.ID != note.ID:
		return fmt.Sprintf(seedSkippedAliasFormat, node.ID)
	case existing.IsRetired:
		return seedSkippedRetired
	}
	switch set.State(note.ID, existing) {
	case seed.Same:
		return seedSkippedSame
	case seed.Outdated:
		if isUpdating {
			return seedUpdated
		}
		return seedSkippedOutdated
	}
	return seedSkippedDiffers
}

// seedNotePath is the file an add of note writes: at the top of knowledgeDir for the root Note,
// else in the children directory of parentPath, the file of the Note's parent.
func seedNotePath(knowledgeDir string, parentPath string, note seed.Note) string {
	dir := knowledgeDir
	if note.ParentID != "" {
		dir = tree.ChildrenDir(parentPath)
	}
	return filepath.Join(dir, note.ID+noteFileExtension)
}

// parentFirst returns the Notes of set with every Note after its parent: by depth below the root,
// and within one depth in the set's own order.
func parentFirst(set seed.Set) []seed.Note {
	depths := make(map[string]int)
	var depthOf func(note seed.Note) int
	depthOf = func(note seed.Note) int {
		if depth, isKnown := depths[note.ID]; isKnown {
			return depth
		}
		depth := 0
		if parent, hasParent := set.Note(note.ParentID); hasParent {
			depth = depthOf(parent) + 1
		}
		depths[note.ID] = depth
		return depth
	}
	notes := slices.Clone(set.Notes())
	slices.SortStableFunc(notes, func(first, second seed.Note) int {
		return depthOf(first) - depthOf(second)
	})
	return notes
}
