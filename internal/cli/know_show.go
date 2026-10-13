package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodefile"
)

// Flags of capcom know show.
const (
	flagWithChildren = "with-children"
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
				shown = append(shown, childrenByParent(notes)[entry.ID]...)
			}
			result, err := a.noteShowResult(shown)
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	cmd.Flags().BoolVar(&isWithChildren, flagWithChildren, false, "also show the notes directly below it")
	cmd.Flags().BoolVar(&isRetiredIncluded, flagRetired, false, retiredFlagUsage)
	return cmd
}

// noteShowResult renders each Note in full, separated by a blank line. Its fields, body and etag
// come from one read of the Note file, the bytes a later --if-match is checked against.
func (a *app) noteShowResult(entries []index.NoteEntry) (Result, error) {
	data := make([]noteData, 0, len(entries))
	var text strings.Builder
	for _, indexed := range entries {
		file, err := a.readIssueFile(indexed.Path)
		if err != nil {
			return Result{}, err
		}
		entry, body, err := a.noteFromFile(indexed, file)
		if err != nil {
			return Result{}, err
		}
		etag := nodefile.ETag(file)
		if text.Len() > 0 {
			text.WriteString("\n")
		}
		text.WriteString(noteText(entry, etag, body))
		shown := newNoteData(entry)
		shown.ETag = etag
		shown.Body = &body
		data = append(data, shown)
	}
	return Result{Kind: "note", Data: data, Text: text.String()}, nil
}

// noteFromFile parses file, the bytes of the Note file entry indexes, and returns the entry with
// every frontmatter field taken from those bytes, and the body. The entry keeps its id, path and
// parent, which the file's place in the tree gives. A file that does not parse fails as invalid at
// its file and line.
func (a *app) noteFromFile(entry index.NoteEntry, file []byte) (index.NoteEntry, string, error) {
	note, err := knowledge.Parse(entry.Path, file)
	if err != nil {
		return index.NoteEntry{}, "", a.frontmatterFailure(err)
	}
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
	}, note.Body, nil
}

// noteText renders one Note in full: its id, location, etag and frontmatter as `key: value`
// lines, a blank line, then the body byte for byte. Empty fields are left out.
func noteText(entry index.NoteEntry, etag string, body string) string {
	fields := []KVPair{
		{"id", entry.ID},
		{"path", entry.Path},
		{"etag", etag},
		{"parent", entry.ParentID},
		{"title", entry.Title},
		{"labels", flowList(entry.Labels)},
		{"refs", flowList(entry.Refs)},
		{"retired", retiredText(entry.IsRetired)},
		{"superseded-by", flowList(entry.SupersededBy)},
		{"aliases", flowList(entry.Aliases)},
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
