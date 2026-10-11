package cli

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/agents"
	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/tree"
)

// Flags of the know subcommands; --id, --title, --parent, --label and --body-file are shared with
// the issue commands.
const (
	flagDepth        = "depth"
	flagWithChildren = "with-children"
	flagRef          = "ref"
)

// unlimitedDepth is the --depth of know tree that prints every level.
const unlimitedDepth = 0

// noteData is one Note in the --json data of the know commands. Lists are always present, empty
// when the file has none; ETag and Body are set by know show only.
type noteData struct {
	ID      string   `json:"id"`
	Path    string   `json:"path"`
	ETag    string   `json:"etag,omitempty"`
	Parent  string   `json:"parent,omitempty"`
	Title   string   `json:"title"`
	Labels  []string `json:"labels"`
	Refs    []string `json:"refs"`
	Aliases []string `json:"aliases"`
	Body    *string  `json:"body,omitempty"`
}

// outlineData is one line of know tree in --json: a Note and its indentation depth in the outline.
type outlineData struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Parent string `json:"parent,omitempty"`
	Depth  int    `json:"depth"`
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

// newKnowCommand groups the commands that read and add Knowledge Notes.
func (a *app) newKnowCommand() *cobra.Command {
	know := &cobra.Command{
		Use:   "know",
		Short: "Read, search, add and verify Knowledge Notes",
		Args:  cobra.NoArgs,
	}
	know.AddCommand(a.newKnowAddCommand())
	know.AddCommand(a.newKnowSearchCommand())
	know.AddCommand(a.newKnowShowCommand())
	know.AddCommand(a.newKnowTreeCommand())
	know.AddCommand(a.newKnowVerifyCommand())
	return know
}

// newKnowTreeCommand prints the outline of the Knowledge tree, or of the subtree of one Note: one
// line per Note, `id  title`, indented two spaces per level.
func (a *app) newKnowTreeCommand() *cobra.Command {
	depth := unlimitedDepth
	cmd := &cobra.Command{
		Use:         "tree [id|alias|prefix]",
		Short:       "Print the outline of Note ids and titles",
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			if depth < unlimitedDepth {
				return UsageErrorf("--%s must be %d (every level) or more", flagDepth, unlimitedDepth)
			}
			lines, err := a.outline(args, depth)
			if err != nil {
				return err
			}
			data := make([]outlineData, 0, len(lines))
			var text strings.Builder
			for _, line := range lines {
				data = append(data, outlineData{line.entry.ID, line.entry.Title, line.entry.ParentID, line.depth})
				text.WriteString(strings.Repeat(textIndent, line.depth) + line.entry.ID + "  " + flatten(line.entry.Title) + "\n")
			}
			return a.emit(cmd, Result{Kind: "know-tree", Data: data, Text: text.String()})
		}),
	}
	cmd.Flags().IntVar(&depth, flagDepth, unlimitedDepth,
		"levels below the top (or below the named note) to print; 0 prints every level")
	return cmd
}

// outlineLine is one Note of an outline at its indentation depth.
type outlineLine struct {
	entry index.NoteEntry
	depth int
}

// outline lists the Notes know tree prints: the whole tree, or the Note args names followed by its
// subtree, each Note followed by its children in id order, at most maxDepth levels below the
// start (unlimitedDepth for all).
func (a *app) outline(args []string, maxDepth int) ([]outlineLine, error) {
	notes, err := a.issueIndex.Notes()
	if err != nil {
		return nil, err
	}
	children := childrenByParent(notes)
	var lines []outlineLine
	var visit func(parentID string, level int, indent int)
	visit = func(parentID string, level int, indent int) {
		if maxDepth != unlimitedDepth && level > maxDepth {
			return
		}
		for _, child := range children[parentID] {
			lines = append(lines, outlineLine{entry: child, depth: indent})
			visit(child.ID, level+1, indent+1)
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
	lines = append(lines, outlineLine{entry: start, depth: 0})
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
// it after it.
func (a *app) newKnowShowCommand() *cobra.Command {
	isWithChildren := false
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
				notes, err := a.issueIndex.Notes()
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
// not_found.
func (a *app) newKnowSearchCommand() *cobra.Command {
	var labels []string
	cmd := &cobra.Command{
		Use:         "search <word>...",
		Short:       "Find Notes whose title or body contains every word",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			text := strings.Join(args, " ")
			entries, err := a.issueIndex.SearchNotes(text, labels)
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
	flags.StringVar(&options.id, flagID, "", "the new note's id: a kebab slug of 2 to 4 words (required)")
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
// bytes written.
func (a *app) addNote(cmd *cobra.Command, options *knowAddOptions) (Result, error) {
	if err := tree.ValidateID(options.id); err != nil {
		return Result{}, Errorf(CodeInvalid, "%s", err.Error())
	}
	for _, ref := range options.refs {
		if err := knowledge.ValidateRef(ref); err != nil {
			return Result{}, Errorf(CodeInvalid, "%s", err.Error())
		}
	}
	notes, err := tree.Load(a.store.KnowledgeDir(), readNoteAliases)
	if err != nil {
		return Result{}, asError(err)
	}
	if existing, found := notes.Lookup(options.id); found {
		return Result{}, noteExistsFailure(options.id, existing, a.store.WorktreeRoot)
	}
	path, err := a.newNotePath(notes, options)
	if err != nil {
		return Result{}, err
	}

	note, err := newNote(cmd, options)
	if err != nil {
		return Result{}, err
	}
	if problems := knowledge.Validate(path, note); len(problems) > 0 {
		return Result{}, &Error{Code: CodeInvalid, Path: path, Message: problems[0].Message}
	}
	content := note.Serialize()
	if err := tree.WriteFileAtomic(a.absoluteIssuePath(path), bytes.NewReader(content)); err != nil {
		return Result{}, asError(err)
	}
	etag := edit.ETag(content)
	data := map[string]string{"id": options.id, "path": path, "etag": etag}
	text := fmt.Sprintf("id: %s\npath: %s\netag: %s\n", options.id, path, etag)
	result := Result{Kind: "note-write", Data: data, Text: text}
	if options.parent == "" {
		a.refreshAgentsAfterAdd(&result, data)
	}
	return result, nil
}

// refreshAgentsAfterAdd refreshes the capcom block of AGENTS.md after a top-level Note joined the
// Knowledge tree, so check stays green; an AGENTS.md without the block is left alone. The Note is
// already written, so a failure here is a warning, not a failed command. A refresh is reported in
// result's text and, as an agents key, in its --json data.
func (a *app) refreshAgentsAfterAdd(result *Result, data map[string]string) {
	status, err := refreshAgentsBlock(a.store, true)
	if err != nil {
		result.Warnings = append(result.Warnings, flatten(err.Error())+"; the capcom block was not refreshed")
		return
	}
	if status.State == agents.StateStale {
		data["agents"] = blockRefreshed
		result.Text += "refreshed: " + agents.FileName + "\n"
	}
}

// newNotePath returns where the new Note goes, relative to the working tree root with forward
// slashes: below its resolved --parent, or at the top of the Knowledge tree.
func (a *app) newNotePath(notes *tree.Tree, options *knowAddOptions) (string, error) {
	parentID := ""
	if options.parent != "" {
		parent, err := notes.Resolve(options.parent)
		if err != nil {
			failure, _ := resolveFailure(err) // either way the command stops here
			return "", failure
		}
		parentID = parent.ID
	}
	absolute, err := notes.PathFor(options.id, parentID)
	if err != nil {
		return "", asError(err)
	}
	return relativeSlashPath(a.store.WorktreeRoot, absolute), nil
}

// newNote builds the Note the flags describe: labels without repeats or empty entries, and the
// body file's text after a blank line, ending in a line break.
func newNote(cmd *cobra.Command, options *knowAddOptions) (knowledge.Note, error) {
	note := knowledge.Note{Title: options.title, Refs: options.refs}
	for _, label := range options.labels {
		if label != "" && !slices.Contains(note.Labels, label) {
			note.Labels = append(note.Labels, label)
		}
	}
	if options.bodyFile == "" {
		return note, nil
	}
	body, err := readTextInput(cmd, options.bodyFile)
	if err != nil {
		return knowledge.Note{}, err
	}
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if strings.TrimSpace(body) == "" {
		return note, nil
	}
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	note.Body = "\n" + body
	return note, nil
}

// readNoteAliases is the tree.AliasFunc of the Knowledge tree: the Aliases a Note file records. A
// file that does not parse contributes none; the index reports it.
func readNoteAliases(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &tree.Error{Path: path, Err: err}
	}
	parsed, err := knowledge.Parse(path, data)
	if err != nil {
		// The index refresh before every command already warned about this file.
		return nil, nil
	}
	return parsed.Aliases, nil
}

// noteExistsFailure refuses id, which is the id or an Alias of the existing Note; it names that
// Note's file, relative to worktreeRoot.
func noteExistsFailure(id string, existing tree.Node, worktreeRoot string) *Error {
	message := fmt.Sprintf("id %q is already taken by note %s", id, existing.ID)
	if existing.ID != id {
		message = fmt.Sprintf("id %q is already an alias of note %s", id, existing.ID)
	}
	return &Error{
		Code:    CodeExists,
		Path:    relativeSlashPath(worktreeRoot, existing.Path),
		Message: message,
	}
}

// newKnowVerifyCommand lists every Ref of every Note that no longer resolves: a missing path, a
// `path#symbol` whose file lacks the symbol text, or a path outside the working tree. Each line
// cites the Ref's line in its Note. Any finding fails the command, after the list.
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

// verifyNotes reads every usable Note file and checks its Refs, in tree order.
func (a *app) verifyNotes() ([]knowledge.Finding, error) {
	notes, err := a.issueIndex.Notes()
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
			return nil, noteFailure(err)
		}
		found, err := knowledge.Verify(a.store.WorktreeRoot, entry.Path, entry.ID, note)
		if err != nil {
			return nil, asError(err)
		}
		findings = append(findings, found...)
	}
	return findings, nil
}

// noteFailure types a Note file that cannot be parsed: invalid, at its file and line.
func noteFailure(err error) error {
	var failure *knowledge.Error
	if errors.As(err, &failure) {
		return &Error{Code: CodeInvalid, Path: failure.Path, Line: failure.Line, Message: failure.Message}
	}
	return asError(err)
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
		ID:      entry.ID,
		Path:    entry.Path,
		Parent:  entry.ParentID,
		Title:   entry.Title,
		Labels:  nonNil(entry.Labels),
		Refs:    nonNil(entry.Refs),
		Aliases: nonNil(entry.Aliases),
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
