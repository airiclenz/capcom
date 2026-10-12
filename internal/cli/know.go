package cli

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/agents"
	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodestore"
)

// Flags of the know subcommands; --id, --title, --parent, --label and --body-file are shared with
// the issue commands.
const (
	flagDepth        = "depth"
	flagWithChildren = "with-children"
	flagRef          = "ref"
	flagRetired      = "retired"
	kindNoteWrite    = "note-write"
)

// retiredFlagUsage is the help of the --retired flag shared by know tree, know show and know search.
const retiredFlagUsage = "include retired notes and everything below them"

// unlimitedDepth is the --depth of know tree that prints every level.
const unlimitedDepth = 0

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

// outlineData is one line of know tree in --json: a Note, its indentation depth in the outline and
// how many of its direct children --depth cut (more, left out when none).
type outlineData struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Parent string `json:"parent,omitempty"`
	Depth  int    `json:"depth"`
	More   int    `json:"more,omitempty"`
}

// findingData is one broken Ref in the --json data of know verify.
type findingData struct {
	Note    string `json:"note"`
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	Ref     string `json:"ref"`
	Message string `json:"message"`
}

// knowAddOptions holds the flags of one capcom know add.
type knowAddOptions struct {
	id       string
	title    string
	parent   string
	labels   []string
	refs     []string
	bodyFile string
}

// newKnowCommand groups the commands that read, add, change, move and remove Knowledge Notes.
func (a *app) newKnowCommand() *cobra.Command {
	know := &cobra.Command{
		Use:   "know",
		Short: "Read, search, add, update, move, remove and verify Knowledge Notes",
		Args:  cobra.NoArgs,
	}
	know.AddCommand(a.newKnowAddCommand())
	know.AddCommand(a.newKnowMvCommand())
	know.AddCommand(a.newKnowRmCommand())
	know.AddCommand(a.newKnowSearchCommand())
	know.AddCommand(a.newKnowShowCommand())
	know.AddCommand(a.newKnowTreeCommand())
	know.AddCommand(a.newKnowUpdateCommand())
	know.AddCommand(a.newKnowVerifyCommand())
	return know
}

// newKnowTreeCommand prints the outline of the Knowledge tree, or of the subtree of one Note: one
// line per Note, `id  title`, indented two spaces per level, and a `... <n> more (--depth <d>)`
// line under each Note whose children --depth cut. Retired subtrees are left out unless --retired
// is given, and never counted as cut; a Note named as the start is printed even when retired.
func (a *app) newKnowTreeCommand() *cobra.Command {
	depth := unlimitedDepth
	isRetiredIncluded := false
	cmd := &cobra.Command{
		Use:         "tree [id|alias|prefix]",
		Short:       "Print the outline of Note ids and titles",
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			if depth < unlimitedDepth {
				return UsageErrorf("--%s must be %d (every level) or more", flagDepth, unlimitedDepth)
			}
			lines, err := a.outline(args, depth, isRetiredIncluded)
			if err != nil {
				return err
			}
			data := make([]outlineData, 0, len(lines))
			rows := make([]outlineRow, 0, len(lines))
			for _, line := range lines {
				data = append(data, outlineData{
					line.entry.ID, line.entry.Title, line.entry.ParentID, line.depth, line.more,
				})
				rows = append(rows, line.outlineRow)
			}
			return a.emit(cmd, Result{Kind: "know-tree", Data: data, Text: renderOutline(rows)})
		}),
	}
	cmd.Flags().IntVar(&depth, flagDepth, unlimitedDepth,
		"levels below the top (or below the named note) to print; 0 prints every level")
	cmd.Flags().BoolVar(&isRetiredIncluded, flagRetired, false, retiredFlagUsage)
	return cmd
}

// outlineLine is one Note of an outline and its row: `id  title` at its indentation depth, with
// the count of direct children maxDepth cut.
type outlineLine struct {
	entry index.NoteEntry
	outlineRow
}

// noteOutlineLine returns the outline line of a Note at an indentation depth, cutting nothing.
func noteOutlineLine(entry index.NoteEntry, depth int) outlineLine {
	return outlineLine{entry, outlineRow{depth: depth, text: entry.ID + "  " + flatten(entry.Title)}}
}

// outline lists the Notes know tree prints: the whole tree, or the Note args names followed by its
// subtree, each Note followed by its children in id order, at most maxDepth levels below the
// start (unlimitedDepth for all). A Note on the last level counts the direct children it cut.
// Unless isRetiredIncluded, retired subtrees are left out below the start, and so never counted;
// the named start itself always resolves.
func (a *app) outline(args []string, maxDepth int, isRetiredIncluded bool) ([]outlineLine, error) {
	notes, err := a.issueIndex.Notes(isRetiredIncluded)
	if err != nil {
		return nil, err
	}
	children := childrenByParent(notes)
	var lines []outlineLine
	var visit func(parentID string, level int, indent int)
	visit = func(parentID string, level int, indent int) {
		isLastLevel := maxDepth != unlimitedDepth && level >= maxDepth
		for _, child := range children[parentID] {
			line := noteOutlineLine(child, indent)
			if isLastLevel {
				line.more, line.moreDepth = len(children[child.ID]), level+1
			}
			lines = append(lines, line)
			if !isLastLevel {
				visit(child.ID, level+1, indent+1)
			}
		}
	}
	if len(args) == 0 {
		visit("", 1, 0)
		return lines, nil
	}
	start, err := a.resolveNote(args[0])
	if err != nil {
		return nil, err
	}
	lines = append(lines, noteOutlineLine(start, 0))
	visit(start.ID, 1, 1)
	return lines, nil
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

// noteShowResult renders each Note in full, separated by a blank line. The etag comes from the
// Note file as it is on disk, the bytes a later --if-match is checked against.
func (a *app) noteShowResult(entries []index.NoteEntry) (Result, error) {
	data := make([]noteData, 0, len(entries))
	var text strings.Builder
	for _, entry := range entries {
		body, err := a.issueIndex.Body(entry.Path)
		if err != nil {
			return Result{}, err
		}
		file, err := a.readIssueFile(entry.Path)
		if err != nil {
			return Result{}, err
		}
		etag := edit.ETag(file)
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

// newKnowSearchCommand finds Notes by full text, every word as a whole word in the title or body,
// optionally only those carrying every --label. Hits are listed best match first; no hit is
// not_found. Retired subtrees are left out unless --retired is given.
func (a *app) newKnowSearchCommand() *cobra.Command {
	var labels []string
	isRetiredIncluded := false
	cmd := &cobra.Command{
		Use:         "search <word>...",
		Short:       "Find Notes whose title or body contains every word",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			text := strings.Join(args, " ")
			entries, err := a.issueIndex.SearchNotes(text, labels, isRetiredIncluded)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				return Errorf(CodeNotFound, "no note matches %q", text)
			}
			data := make([]noteData, 0, len(entries))
			var lines strings.Builder
			for _, entry := range entries {
				data = append(data, newNoteData(entry))
				lines.WriteString(entry.ID + "  " + flatten(entry.Title) + "\n")
			}
			return a.emit(cmd, Result{Kind: "know-search", Data: data, Text: lines.String()})
		}),
	}
	cmd.Flags().StringSliceVar(&labels, flagLabel, nil, "only notes with every one of these labels; repeat or separate with commas")
	cmd.Flags().BoolVar(&isRetiredIncluded, flagRetired, false, retiredFlagUsage)
	return cmd
}

// newKnowAddCommand writes a new Note file. The id is always the caller's choice, never generated.
func (a *app) newKnowAddCommand() *cobra.Command {
	options := &knowAddOptions{}
	cmd := &cobra.Command{
		Use:         "add --id <slug> --title <title>",
		Short:       "Add a Note with a chosen id",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			result, err := a.addNote(cmd, options)
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&options.id, flagID, "", "the new note's id: a kebab slug of 1 to 4 words (required)")
	flags.StringVar(&options.title, flagTitle, "", "the note's title (required)")
	flags.StringVar(&options.parent, flagParent, "", "add the note below this one")
	flags.StringSliceVar(&options.labels, flagLabel, nil, "labels; repeat or separate with commas")
	flags.StringArrayVar(&options.refs, flagRef, nil, "a path or path#symbol the note depends on; repeat for more")
	flags.StringVar(&options.bodyFile, flagBodyFile, "", "file holding the markdown body; - reads stdin")
	_ = cmd.MarkFlagRequired(flagID)    // fails only for an undefined flag
	_ = cmd.MarkFlagRequired(flagTitle) // fails only for an undefined flag
	return cmd
}

// addNote places the new Note in the Knowledge tree and writes it, reporting the etag of the
// bytes written, with a warning for every tree problem that concerns neither the Note's path nor
// its parent, unless the index refresh already warned about it. A malformed --ref is refused
// before the Note's id is weighed against the tree.
func (a *app) addNote(cmd *cobra.Command, options *knowAddOptions) (Result, error) {
	for _, ref := range options.refs {
		if err := knowledge.ValidateRef(ref); err != nil {
			return Result{}, Errorf(CodeInvalid, "%s", err.Error())
		}
	}
	written, problems, err := nodestore.Notes(a.store, nil).Create(
		options.id,
		options.parent,
		func() (knowledge.Note, error) { return newNote(cmd, options) },
	)
	if err != nil {
		return Result{}, kindFailure(err)
	}
	result := noteWriteResult(written.ID, written.Path, written.ETag)
	result.Warnings = append(result.Warnings, a.unwarnedTreeProblems(problems)...)
	if options.parent == "" {
		a.refreshAgentsAfterWrite(&result)
	}
	return result, nil
}

// noteWriteResult is the result of a command that wrote a Note: its id, path and new etag.
func noteWriteResult(id string, path string, etag string) Result {
	return Result{
		Kind: kindNoteWrite,
		Data: map[string]string{"id": id, "path": path, "etag": etag},
		Text: fmt.Sprintf("id: %s\npath: %s\netag: %s\n", id, path, etag),
	}
}

// refreshAgentsAfterWrite refreshes the capcom block of AGENTS.md after a Note write that may have
// changed the Knowledge top level, so check stays green. Only a stale block is rewritten; an
// AGENTS.md without the block is left alone. The Note is already written, so a failure here is a
// warning, not a failed command. A refresh is reported in the text of result, a noteWriteResult,
// and as an agents key in its --json data.
func (a *app) refreshAgentsAfterWrite(result *Result) {
	status, err := agents.Sync(a.store.WorktreeRoot, a.store.KnowledgeDir(), agents.ModeRefresh)
	if err != nil {
		warning := flatten(agentsFailure(err).Error()) + "; the capcom block was not refreshed"
		result.Warnings = append(result.Warnings, warning)
		return
	}
	if status.State != agents.StateStale {
		return
	}
	if data, isMap := result.Data.(map[string]string); isMap {
		data["agents"] = blockRefreshed
	}
	result.Text += "refreshed: " + agents.FileName + "\n"
}

// newNote builds the Note the flags describe: labels without repeats or empty entries, and the
// body file's text as noteBody shapes it.
func newNote(cmd *cobra.Command, options *knowAddOptions) (knowledge.Note, error) {
	note := knowledge.Note{
		Title:  options.title,
		Labels: edit.Labels(nil, options.labels, nil),
		Refs:   options.refs,
	}
	if options.bodyFile == "" {
		return note, nil
	}
	body, err := readTextInput(cmd, options.bodyFile)
	if err != nil {
		return knowledge.Note{}, err
	}
	note.Body = noteBody(body)
	return note, nil
}

// noteBody shapes text as a whole Note body: CRLF line endings read as LF, blank text is no body,
// and any other text follows the blank line after the frontmatter and ends in a line break.
func noteBody(text string) string {
	text = edit.NormalizeNewlines(text)
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return "\n" + edit.WithFinalNewline(text)
}

// newKnowVerifyCommand lists every Ref of every Note that no longer resolves: a missing path, a
// `path#symbol` whose file lacks the symbol text, or a path outside the working tree. Each line
// cites the Ref's line in its Note. A retired subtree is exempt. Any finding fails the command,
// after the list.
func (a *app) newKnowVerifyCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "verify",
		Short:       "List the Note refs that no longer resolve",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			findings, err := a.verifyNotes()
			if err != nil {
				return err
			}
			data := make([]findingData, 0, len(findings))
			var text strings.Builder
			for _, finding := range findings {
				data = append(data, findingData{finding.NoteID, finding.Path, finding.Line, finding.Ref, finding.Message})
				text.WriteString(finding.String() + "\n")
			}
			result := Result{Kind: "know-verify", Data: data, Text: text.String()}
			if len(findings) > 0 {
				result.Failure = Errorf(CodeInvalid, "%d note refs do not resolve", len(findings))
			}
			return a.emit(cmd, result)
		}),
	}
}

// verifyNotes reads every usable Note file outside a retired subtree and checks its Refs, in tree
// order.
func (a *app) verifyNotes() ([]knowledge.Finding, error) {
	notes, err := a.issueIndex.Notes(false)
	if err != nil {
		return nil, err
	}
	var findings []knowledge.Finding
	for _, entry := range notes {
		data, err := a.readIssueFile(entry.Path)
		if err != nil {
			return nil, err
		}
		note, err := knowledge.Parse(entry.Path, data)
		if err != nil {
			return nil, a.frontmatterFailure(err)
		}
		found, err := knowledge.Verify(a.store.WorktreeRoot, entry.Path, entry.ID, note)
		if err != nil {
			return nil, asError(err)
		}
		findings = append(findings, found...)
	}
	return findings, nil
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
