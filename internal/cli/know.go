package cli

import (
	"cmp"
	"slices"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/index"
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
		Use:   "know",
		Short: "Read, search, add, update, move, remove and verify Knowledge Notes",
		Args:  cobra.NoArgs,
	}
	know.AddCommand(a.newKnowAddCommand())
	know.AddCommand(newMvCommand(a, noteVerbs))
	know.AddCommand(a.newKnowRmCommand())
	know.AddCommand(a.newKnowSearchCommand())
	know.AddCommand(a.newKnowShowCommand())
	know.AddCommand(a.newKnowTreeCommand())
	know.AddCommand(a.newKnowUpdateCommand())
	know.AddCommand(a.newKnowVerifyCommand())
	return know
}

// childrenByParent groups notes by the id of their parent ("" for the top level), each group in
// id order.
func childrenByParent(notes []index.NoteEntry) map[string][]index.NoteEntry {
	children := map[string][]index.NoteEntry{}
	for _, note := range notes {
		children[note.ParentID] = append(children[note.ParentID], note)
	}
	for _, group := range children {
		slices.SortFunc(group, func(left, right index.NoteEntry) int {
			return cmp.Compare(left.ID, right.ID)
		})
	}
	return children
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
