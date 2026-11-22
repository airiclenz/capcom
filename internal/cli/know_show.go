package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/outline"
)

// Flags of capcom know show.
const (
	flagWithChildren = "with-children"
)

// Help text of capcom know show.
const (
	knowShowLong = "Print each named Note in full: its id, path, etag, parent, title, Labels, Refs, " +
		"retirement, superseded-by entries, Aliases and body.\n\n" +
		"Each argument may be an id, an Alias or a unique id prefix, and a retired Note still " +
		"resolves. An argument that names none of those is tried as a title, case-insensitive: " +
		"the whole title, then the part before its first colon, then the argument spelled as an " +
		"id (\"Release steps!\" spells release-steps). When several Notes match, it fails naming " +
		"them all.\n\n" +
		"The Notes found are printed even when another argument names none, and the " +
		"command then exits with the first failure's code.\n\n" +
		"--with-children also prints the Notes directly below each, their retired ones only with " +
		"--retired.\n\n" +
		"--brief prints the fields only, without the body. --section prints the fields and the body " +
		"sections under the headings named, in body order, heading lines included; a heading matches " +
		"case-insensitively, its # marks ignored.\n\n" +
		"A section runs to the next heading of its level or a higher one. Each Note shown is cut " +
		"alike, its children too; one whose body lacks a named heading fails, naming the headings it " +
		"has, and the others still print.\n\n" +
		"The etag is what --if-match takes on know update, know mv and know rm."
	knowShowExample = "  capcom know show release-steps                   # one Note in full\n" +
		"  capcom know show release-steps store-layout      # several at once\n" +
		"  capcom know show \"Release steps\"                 # by its title\n" +
		"  capcom know show release-steps --with-children   # and the Notes directly below it\n" +
		"  capcom know show release-steps --brief           # the fields only, no body\n" +
		"  capcom know show release-steps --section steps   # the fields and one body section\n" +
		"  capcom know show release-steps --json            # as a JSON envelope"
)

// newKnowShowCommand prints each named Note in full, and with --with-children the Notes directly
// below each after it. Every argument is resolved on its own, by its title too when it names no
// id, Alias or prefix (index.ResolveNoteByTitle); the Notes found are printed even when
// another argument fails, and the command then exits with the first failure's code. A named Note
// resolves even when retired; its retired children, and every child of a retired Note, are shown
// only with --retired. --brief leaves each body out and --section keeps only the body sections it
// names; a Note whose body lacks one fails as its argument does, invalid.
func (a *app) newKnowShowCommand() *cobra.Command {
	isWithChildren := false
	isRetiredIncluded := false
	var view bodyView
	cmd := &cobra.Command{
		Use:               "show <id|alias|prefix|title>...",
		Short:             "Show Notes in full",
		Long:              knowShowLong,
		Example:           knowShowExample,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeNoteIDs(everyArgument),
		Annotations:       map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			failures := a.newArgumentFailures(cmd, noteIDs)
			shown, err := a.shownNotes(args, isWithChildren, isRetiredIncluded, failures)
			if err != nil {
				return err
			}
			result, invalid, err := a.noteShowResult(shown, view)
			if err != nil {
				return err
			}
			for _, failure := range invalid {
				failures.addFailure(failure)
			}
			result.Failure = failures.failure()
			return a.emit(cmd, result)
		}),
	}
	flags := cmd.Flags()
	flags.BoolVar(&isWithChildren, flagWithChildren, false, "also show the Notes directly below each")
	flags.BoolVar(&isRetiredIncluded, flagRetired, false, retiredFlagUsage)
	flags.BoolVar(&view.isBrief, flagBrief, false, "print the fields only, without the body")
	flags.StringArrayVar(&view.sections, flagSection, nil,
		"print the fields and only the body section under this heading; repeat for more")
	cmd.MarkFlagsMutuallyExclusive(flagBrief, flagSection)
	return cmd
}

// shownNotes resolves every argument to its Note, each followed with isWithChildren by the Notes
// directly below it, their retired ones only with isRetiredIncluded. It returns the Notes found and
// collects in failures the failure of each argument that names none, as show reports them; the
// error is the index failing, which the command cannot go on from.
func (a *app) shownNotes(
	args []string,
	isWithChildren bool,
	isRetiredIncluded bool,
	failures *argumentFailures,
) ([]index.NoteEntry, error) {
	var shown []index.NoteEntry
	var notes *outline.Tree[index.NoteEntry]
	for _, arg := range args {
		entry, err := a.issueIndex.ResolveNoteByTitle(arg)
		if err != nil {
			if _, fatal := failures.add(err); fatal != nil {
				return nil, fatal
			}
			continue
		}
		shown = append(shown, entry)
		if !isWithChildren {
			continue
		}
		if notes == nil {
			entries, err := a.issueIndex.Notes(isRetiredIncluded)
			if err != nil {
				return nil, err
			}
			notes = outline.Notes(entries, true)
		}
		shown = append(shown, notes.Children(entry.ID)...)
	}
	return shown, nil
}

// noteShowResult renders each Note, separated by a blank line, its body printed and given as view
// has it: in full with the zero view. Its fields, body and etag come from one read of the Note file,
// the bytes a later --if-match is checked against. A Note whose body lacks a heading view names is
// left out, and its failure, typed invalid and naming its file, is returned in invalid; the zero
// view returns none.
func (a *app) noteShowResult(
	entries []index.NoteEntry,
	view bodyView,
) (result output.Result, invalid []*output.Error, err error) {
	data := make([]noteData, 0, len(entries))
	var text strings.Builder
	reader := nodestore.NoteReader()
	for _, indexed := range entries {
		reading, err := parseNodeFile(a, reader, indexed.Path)
		if err != nil {
			return output.Result{}, nil, err
		}
		entry, fullBody := noteFromFile(indexed, reading.Node)
		body, hasBody, err := view.body(fullBody)
		if err != nil {
			invalid = append(invalid, output.Errorf(output.CodeInvalid, "%s: %s", entry.Path, err))
			continue
		}
		etag := reading.ETag
		if text.Len() > 0 {
			text.WriteString("\n")
		}
		text.WriteString(noteText(entry, etag, body))
		shown := newNoteData(entry)
		shown.ETag = etag
		if hasBody {
			shown.Body = &body
		}
		data = append(data, shown)
	}
	return output.Result{Kind: "note", Data: data, Text: text.String()}, invalid, nil
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
