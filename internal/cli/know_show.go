package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodestore"
)

// Flags of capcom know show.
const (
	flagWithChildren = "with-children"
)

// Help text of capcom know show.
const (
	knowShowLong = "Print one Note in full: its id, path, etag, parent, title, Labels, Refs, retirement, " +
		"superseded-by entries, Aliases and body. The argument may be an id, an Alias or a unique id " +
		"prefix, and a retired Note still resolves. --with-children also prints the Notes directly below " +
		"it, their retired ones only with --retired. The etag is what --if-match takes on know update, " +
		"know mv and know rm."
	knowShowExample = "  capcom know show release-steps                   # one Note in full\n" +
		"  capcom know show release-steps --with-children   # and the Notes directly below it\n" +
		"  capcom know show release-steps --json            # as a JSON envelope"
)

// newKnowShowCommand prints one Note in full, and with --with-children the Notes directly below
// it after it. The named Note resolves even when retired; its retired children, and every child of
// a retired Note, are shown only with --retired.
func (a *app) newKnowShowCommand() *cobra.Command {
	isWithChildren := false
	isRetiredIncluded := false
	cmd := &cobra.Command{
		Use:         "show <id|alias|prefix>",
		Short:       "Show a Note in full",
		Long:        knowShowLong,
		Example:     knowShowExample,
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			entry, err := a.resolveNote(args[0])
			if err != nil {
				return err
			}
			shown := []index.NoteEntry{entry}
			if isWithChildren {
				notes, err := a.issueIndex.Notes(isRetiredIncluded)
				if err != nil {
					return err
				}
				shown = append(shown, newNoteTree(notes, nil).all(entry.ID)...)
			}
			result, err := a.noteShowResult(shown)
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	cmd.Flags().BoolVar(&isWithChildren, flagWithChildren, false, "also show the Notes directly below it")
	cmd.Flags().BoolVar(&isRetiredIncluded, flagRetired, false, retiredFlagUsage)
	return cmd
}

// noteShowResult renders each Note in full, separated by a blank line. Its fields, body and etag
// come from one read of the Note file, the bytes a later --if-match is checked against.
func (a *app) noteShowResult(entries []index.NoteEntry) (output.Result, error) {
	data := make([]noteData, 0, len(entries))
	var text strings.Builder
	reader := nodestore.NoteReader()
	for _, indexed := range entries {
		reading, err := parseNodeFile(a, reader, indexed.Path)
		if err != nil {
			return output.Result{}, err
		}
		entry, body := noteFromFile(indexed, reading.Node)
		etag := reading.ETag
		if text.Len() > 0 {
			text.WriteString("\n")
		}
		text.WriteString(noteText(entry, etag, body))
		shown := newNoteData(entry)
		shown.ETag = etag
		shown.Body = &body
		data = append(data, shown)
	}
	return output.Result{Kind: "note", Data: data, Text: text.String()}, nil
}

// noteFromFile returns entry with every frontmatter field taken from note, the Note its file
// parses to, and the body. The entry keeps its id, path and parent, which the file's place in the
// tree gives.
func noteFromFile(entry index.NoteEntry, note knowledge.Note) (index.NoteEntry, string) {
	return index.NoteEntry{
		ID:           entry.ID,
		Path:         entry.Path,
		ParentID:     entry.ParentID,
		Title:        note.Title,
		Labels:       note.Labels,
		Refs:         note.Refs,
		Aliases:      note.Aliases,
		IsRetired:    note.IsRetired,
		SupersededBy: note.SupersededBy,
	}, note.Body
}

// noteText renders one Note in full: its id, location, etag and frontmatter as `key: value`
// lines, a blank line, then the body byte for byte. Empty fields are left out.
func noteText(entry index.NoteEntry, etag string, body string) string {
	fields := []output.KVPair{
		{Key: "id", Value: entry.ID},
		{Key: "path", Value: entry.Path},
		{Key: "etag", Value: etag},
		{Key: "parent", Value: entry.ParentID},
		{Key: "title", Value: entry.Title},
		{Key: "labels", Value: flowList(entry.Labels)},
		{Key: "refs", Value: flowList(entry.Refs)},
		{Key: "retired", Value: retiredText(entry.IsRetired)},
		{Key: "superseded-by", Value: flowList(entry.SupersededBy)},
		{Key: "aliases", Value: flowList(entry.Aliases)},
	}
	var text strings.Builder
	for _, field := range fields {
		if field.Value != "" {
			text.WriteString(field.Key + ": " + flatten(field.Value) + "\n")
		}
	}
	if body == "" {
		return text.String()
	}
	if !strings.HasPrefix(body, "\n") {
		text.WriteString("\n")
	}
	text.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		text.WriteString("\n")
	}
	return text.String()
}

// retiredText renders a Note's retired flag as noteText prints it: "true", or "" when false so the
// line is left out.
func retiredText(isRetired bool) string {
	if !isRetired {
		return ""
	}
	return "true"
}
