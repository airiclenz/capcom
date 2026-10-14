package cli

import (
	"cmp"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/index"
)

// Help text of capcom know.
const (
	knowLong = "Read and change the Knowledge tree: Notes are markdown files under .capcom/knowledge/, each " +
		"with a title, Labels, Refs and a body, nested by Parent like Issues. A Ref names a path or " +
		"path#symbol the Note depends on, and know verify lists the Refs that no longer resolve. A Note " +
		"that no longer holds is retired rather than removed, optionally naming the Notes that " +
		"supersede it, and leaves browsing with its subtree. A write that leaves the capcom block of " +
		"AGENTS.md stale refreshes it."
	knowExample = "  capcom know tree --depth 2       # the top two levels of the Knowledge tree\n" +
		"  capcom know show release-steps   # one Note in full\n" +
		"  capcom know search release       # Notes that mention release\n" +
		"  capcom know verify               # Note refs that no longer resolve"
)

// retiredFlagUsage is the help of the --retired flag shared by know tree, know show and know search.
const retiredFlagUsage = "include retired notes and everything below them"

// noteData is one Note in the --json data of the know commands. Lists are always present, empty
// when the file has none; retired is left out when false; ETag and Body are set by know show only.
type noteData struct {
	ID           string   `json:"id"`
	Path         string   `json:"path"`
	ETag         string   `json:"etag,omitempty"`
	Parent       string   `json:"parent,omitempty"`
	Title        string   `json:"title"`
	Labels       []string `json:"labels"`
	Refs         []string `json:"refs"`
	IsRetired    bool     `json:"retired,omitempty"`
	SupersededBy []string `json:"superseded_by"`
	Aliases      []string `json:"aliases"`
	Body         *string  `json:"body,omitempty"`
}

// newKnowCommand groups the commands that read, add, change, move and remove Knowledge Notes.
func (a *app) newKnowCommand() *cobra.Command {
	know := &cobra.Command{
		Use:     "know",
		Short:   "Read, search, add, update, move, remove and verify Knowledge Notes",
		Long:    knowLong,
		Example: knowExample,
		Args:    cobra.NoArgs,
	}
	know.AddCommand(a.newKnowAddCommand())
	know.AddCommand(newMvCommand(a, noteVerbs))
	know.AddCommand(newRmCommand(a, noteVerbs))
	know.AddCommand(a.newKnowSearchCommand())
	know.AddCommand(a.newKnowShowCommand())
	know.AddCommand(a.newKnowTreeCommand())
	know.AddCommand(a.newKnowUpdateCommand())
	know.AddCommand(a.newKnowVerifyCommand())
	return know
}

// newNoteTree groups notes under their parents, each group in id order, for an outline that
// prints the Notes isShown accepts (every Note when nil). A Note whose parent is not among notes,
// unparseable or left out, joins the top level, as an issue does in list --tree.
func newNoteTree(notes []index.NoteEntry, isShown func(index.NoteEntry) bool) *visibleTree[index.NoteEntry] {
	return newVisibleTree(
		notes,
		noteID,
		noteParentID,
		func(left, right index.NoteEntry) int { return cmp.Compare(left.ID, right.ID) },
		isShown,
	)
}

// noteParentID returns the id of the parent of entry, "" for a top-level Note.
func noteParentID(entry index.NoteEntry) string {
	return entry.ParentID
}

// resolveNote resolves one argument to its Note, typing the failure as resolveFailure does.
func (a *app) resolveNote(arg string) (index.NoteEntry, error) {
	entry, err := a.issueIndex.ResolveNote(arg)
	if err != nil {
		failure, _ := resolveFailure(err) // either way the command stops here
		return index.NoteEntry{}, failure
	}
	return entry, nil
}

// newNoteData converts an indexed Note to its --json form.
func newNoteData(entry index.NoteEntry) noteData {
	return noteData{
		ID:           entry.ID,
		Path:         entry.Path,
		Parent:       entry.ParentID,
		Title:        entry.Title,
		Labels:       nonNil(entry.Labels),
		Refs:         nonNil(entry.Refs),
		IsRetired:    entry.IsRetired,
		SupersededBy: nonNil(entry.SupersededBy),
		Aliases:      nonNil(entry.Aliases),
	}
}
