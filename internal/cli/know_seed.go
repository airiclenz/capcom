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
		"It adds every Note of the set the store lacks, the root first, and never changes a Note that " +
		"exists. An added Note carries two Labels: seed:<root id>, and seed-text:<version>:<hash>, " +
		"which holds the capcom version and a hash of the text it was added with.\n\n" +
		"It prints one line per Note of the set, its id and its outcome: added; skipped (same) for a " +
		"Note whose title and body are the set's; skipped (differs) for one with another text; " +
		"skipped (retired) for a retired Note; skipped (alias of <id>) when the id is an Alias of " +
		"another Note. A retired root Note skips the whole set as retired.\n\n" +
		"A root Note that exists without the seed:<root id> Label is the repository's own: the " +
		"command fails and writes nothing. A write that fails stops the run; the Notes added before " +
		"it stay, and running the command again adds the rest.\n\n" +
		"--dry-run prints the same lines and writes nothing."
	knowSeedExample = "  capcom know seed             # add the Notes of the base set the store lacks\n" +
		"  capcom know seed --dry-run   # print what it would add, and write nothing"
)

// kindKnowSeed is the --json kind of capcom know seed.
const kindKnowSeed = "know-seed"

// The outcomes capcom know seed reports for a Note of the base set.
const (
	seedAdded          = "added"
	seedSkippedSame    = "skipped (same)"
	seedSkippedDiffers = "skipped (differs)"
	seedSkippedRetired = "skipped (retired)"
	// seedSkippedAliasFormat takes the id of the Note that holds the set Note's id as an Alias.
	seedSkippedAliasFormat = "skipped (alias of %s)"
)

// unseededRootFormat is the failure for a root Note that exists without the seed Label: the root
// id, then the Label.
const unseededRootFormat = "Note %s was not seeded: it has no Label %s, so capcom know seed writes nothing"

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
	// the file that holds the id or, when no file does, the file an add would write.
	Path    string `json:"path"`
	Outcome string `json:"outcome"`
}

// seedStep is what capcom know seed does with one Note of the base set.
type seedStep struct {
	note seed.Note
	data seedNoteData
}

// newKnowSeedCommand adds the Notes of the base set the store lacks. Every id is the set's own, a
// slug read from the Note's file name: none is generated, derived or renamed.
func (a *app) newKnowSeedCommand() *cobra.Command {
	var isDryRun bool
	cmd := &cobra.Command{
		Use:         "seed [--dry-run]",
		Short:       "Add the base set of Notes capcom ships, never changing a Note that exists",
		Long:        knowSeedLong,
		Example:     knowSeedExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			result, err := a.seedNotes(isDryRun)
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	cmd.Flags().BoolVar(&isDryRun, flagDryRun, false, "print each Note's outcome and write nothing")
	return cmd
}

// seedSet loads the base set through the app's loader, seed.Load unless a test set another.
func (a *app) seedSet() (seed.Set, error) {
	if a.loadSeed == nil {
		return seed.Load()
	}
	return a.loadSeed()
}

// seedNotes weighs every Note of the base set against the Knowledge tree (planSeed), then adds the
// Notes the tree lacks one at a time, a parent before its children, unless isDryRun. The result
// names every Note with its outcome. A failed add stops the run: the result then holds the Notes
// up to it and the failure, and the Notes added before it stay. Each add is noted for the Tree
// lock warning (app.noteKindWrite), and the add of the root Note refreshes a stale capcom block
// (see notes), reported as know add reports it.
func (a *app) seedNotes(isDryRun bool) (output.Result, error) {
	set, err := a.seedSet()
	if err != nil {
		return output.Result{}, output.AsError(err)
	}
	steps, err := a.planSeed(set)
	if err != nil {
		return output.Result{}, err
	}
	data := knowSeedData{IsDryRun: isDryRun, Notes: make([]seedNoteData, 0, len(steps))}
	var failure *output.Error
	var warnings []string
	var effects []nodestore.Effects
	for _, step := range steps {
		if step.data.Outcome == seedAdded && !isDryRun {
			written, problems, err := a.addSeedNote(set, step.note)
			if err != nil {
				failure = a.kindFailure(err, noteIDs)
				break
			}
			a.noteKindWrite(noteIDs, written.ID)
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
// and as an Alias, is to be added; one it holds is skipped, see seedOutcome. It fails with invalid,
// at the file, when a file that holds an id of the set cannot be read or parsed, and when the root
// Note exists without the seed Label, so a tree that was never seeded is left alone.
func (a *app) planSeed(set seed.Set) ([]seedStep, error) {
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
		steps = append(steps, seedStep{note: note, data: seedNoteData{
			ID:      note.ID,
			Path:    a.store.Relative(path),
			Outcome: seedOutcome(set, note, node, isFound, existing, isRootRetired),
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

// seedOutcome is the outcome of note, a Note of set. node and existing are the tree's node that
// answers to its id and that node's file, when isFound. A retired root Note skips every Note as
// retired; a Note the tree lacks is added; an id that is an Alias of another Note, a retired Note
// and a Note with the set's text or another are skipped as such. A Note in the state
// seed.Outdated is skipped as one that differs.
func seedOutcome(
	set seed.Set,
	note seed.Note,
	node tree.Node,
	isFound bool,
	existing knowledge.Note,
	isRootRetired bool,
) string {
	switch {
	case isRootRetired:
		return seedSkippedRetired
	case !isFound:
		return seedAdded
	case node.ID != note.ID:
		return fmt.Sprintf(seedSkippedAliasFormat, node.ID)
	case existing.IsRetired:
		return seedSkippedRetired
	case set.State(note.ID, existing) == seed.Same:
		return seedSkippedSame
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
