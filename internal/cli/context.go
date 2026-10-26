package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/index"
)

// flagBrief makes capcom context print the Issue as one line and leave out the Note bodies.
const flagBrief = "brief"

// The caps of capcom context's text, which --help states.
const (
	// contextNoteLimit is how many applying Notes context lists.
	contextNoteLimit = 8
	// contextBodyLimit is how many of the listed Notes, the first ones, have their body printed.
	contextBodyLimit = 3
	// contextBodyLineLimit is how many lines of a Note body print before the body is cut.
	contextBodyLineLimit = 60
	// contextDepLimit is how many lines of the deps or the dependents section print before it is cut.
	contextDepLimit = 15
)

// The text of capcom context.
const (
	contextNone         = "none"
	contextNotesHeading = "notes:"
	contextPlanHeading  = "plan:"
	contextPlanNotFound = "(not found)"
	contextBodyMarker   = "---"
	contextSourceRef    = "ref"
	contextSourceSearch = "search"
)

// The help text of capcom context.
var (
	contextLong = "Print one Issue with what it takes to work on it: the Issue as capcom show prints it, its " +
		"deps and dependents one level deep as capcom dep list prints them, its plan's path, and the " +
		"Notes that apply.\n\n" +
		"A Note applies when one of its refs names a path the Issue's body mentions, whole or as a " +
		"directory followed by /. Then come the Notes a search on the Issue's title finds, every word " +
		"first, else any word. Retired Notes are left out.\n\n" +
		fmt.Sprintf(
			"The text is capped: at most %d Notes, the bodies of the first %d only, %d lines per body and "+
				"%d Issues per dep section. A cut section ends with a line naming the command that prints "+
				"the rest.\n\n",
			contextNoteLimit, contextBodyLimit, contextBodyLineLimit, contextDepLimit,
		) +
		"An empty section prints as none. --brief prints the Issue as one line and no Note bodies.\n\n" +
		"--json gives kind context with data {issue, deps, dependents, plan, notes}, none of it cut."
	contextExample = "  capcom context login-timeout-race            # the Issue, its deps, plan and Notes\n" +
		"  capcom context login-timeout-race --brief    # one line per item, no bodies\n" +
		"  capcom context login-timeout-race --json     # the versioned JSON envelope"
)

// contextData is the --json data of context: the Issue as show gives it, its direct deps and
// dependents as dep list gives them, its plan (null when it names none) and the Notes that apply.
type contextData struct {
	Issue      showData          `json:"issue"`
	Deps       []depListItem     `json:"deps"`
	Dependents []depListItem     `json:"dependents"`
	Plan       *contextPlanData  `json:"plan"`
	Notes      []contextNoteData `json:"notes"`
}

// contextPlanData is the plan an Issue names: its name as the Issue writes it, and the path of its
// file relative to the worktree root, empty when no plans directory holds it.
type contextPlanData struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// contextNoteData is one applying Note in --json: the Note, with its body for the first
// contextBodyLimit of them, how it was found (contextSourceRef or contextSourceSearch) and, for
// contextSourceRef, the ref that names a path the Issue mentions.
type contextNoteData struct {
	noteData
	Source string `json:"source"`
	Ref    string `json:"ref,omitempty"`
}

// applyingNote is a Note that applies to an Issue, with the ref that made it apply; ref is empty
// for a Note the title search found.
type applyingNote struct {
	entry index.NoteEntry
	ref   string
}

// newContextCommand prints an Issue with what it takes to work on it: the Issue as show prints
// it, its direct deps and dependents, its plan and the Notes that apply, each capped as --help
// states. --brief prints the Issue as one line and leaves out the Note bodies.
func (a *app) newContextCommand() *cobra.Command {
	isBrief := false
	cmd := &cobra.Command{
		Use:         "context <id|alias|prefix>",
		Short:       "Show an issue with its deps, plan and the Notes that apply",
		Long:        contextLong,
		Example:     contextExample,
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			entry, err := a.resolveIssue(args[0])
			if err != nil {
				return err
			}
			result, err := a.contextResult(cmd, entry, isBrief)
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	}
	cmd.Flags().BoolVar(&isBrief, flagBrief, false, "print the Issue as one line and leave out the Note bodies")
	return cmd
}

// contextResult builds the result of context for entry: the Issue, its deps and dependents, its
// plan and its applying Notes, each section in its text and --json form. With isBrief the Issue
// prints as its list line and no body is printed or given.
func (a *app) contextResult(cmd *cobra.Command, entry index.Entry, isBrief bool) (output.Result, error) {
	shownResult, _, err := a.showResult(cmd, []string{entry.ID})
	if err != nil {
		return output.Result{}, err
	}
	if shownResult.Failure != nil {
		return output.Result{}, shownResult.Failure
	}
	shownIssue := shownResult.Data.([]showData)[0]
	body := *shownIssue.Body

	graph, err := a.loadDepGraph()
	if err != nil {
		return output.Result{}, err
	}
	deps := graph.listResult(entry, defaultDepListDepth, false)
	depData := deps.Data.(depListData)

	plan, planText, err := a.contextPlan(shownIssue.Plan)
	if err != nil {
		return output.Result{}, err
	}
	applying, err := a.applyingNotes(shownIssue.Title, body)
	if err != nil {
		return output.Result{}, err
	}
	notes, notesText, err := a.contextNotes(applying, isBrief)
	if err != nil {
		return output.Result{}, err
	}

	issueText := shownResult.Text
	if isBrief {
		issueText = issueLines([]index.Entry{entry}, "")
		shownIssue.Body = nil
	}
	data := contextData{Issue: shownIssue, Deps: depData.Deps, Dependents: depData.Dependents, Plan: plan, Notes: notes}
	text := issueText + "\n" + contextDepsText(deps.Text, entry.ID) + planText + notesText
	return output.Result{Kind: "context", Data: data, Text: text}, nil
}

// contextDepsText turns depsText, the text of a one-level dep list of the Issue with id, into
// context's deps and dependents sections, each cut after contextDepLimit lines.
func contextDepsText(depsText string, id string) string {
	depRows, dependentRows, _ := strings.Cut(strings.TrimPrefix(depsText, depsHeading+"\n"), dependentsHeading+"\n")
	cutLine := "capcom dep list " + id
	return contextSection(depsHeading, splitRows(depRows), contextDepLimit, cutLine) +
		contextSection(dependentsHeading, splitRows(dependentRows), contextDepLimit, cutLine)
}

// splitRows splits rows, newline-ended lines, into its lines; no rows yields none.
func splitRows(rows string) []string {
	if rows == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(rows, "\n"), "\n")
}

// contextSection renders one section of context: `<heading> none` without rows, else heading on
// its own line and the rows, indented as they come, the first limit of them followed by
// `... <n> more: <cutLine>` when more remain.
func contextSection(heading string, rows []string, limit int, cutLine string) string {
	if len(rows) == 0 {
		return heading + " " + contextNone + "\n"
	}
	var text strings.Builder
	text.WriteString(heading + "\n")
	for _, row := range rows[:min(limit, len(rows))] {
		text.WriteString(row + "\n")
	}
	if len(rows) > limit {
		fmt.Fprintf(&text, "%s... %d more: %s\n", textIndent, len(rows)-limit, cutLine)
	}
	return text.String()
}

// contextPlan resolves plan, an Issue's plan basename, to its file in the plans directories: the
// --json form (nil when plan is empty) and the plan line, `plan: <path>` relative to the worktree
// root, `plan: <name> (not found)`, or `plan: none`.
func (a *app) contextPlan(plan string) (*contextPlanData, string, error) {
	if plan == "" {
		return nil, contextPlanHeading + " " + contextNone + "\n", nil
	}
	path, isFound := a.store.Config.PlanPath(a.store.WorktreeRoot, plan)
	if !isFound {
		return &contextPlanData{Name: plan}, contextPlanHeading + " " + plan + " " + contextPlanNotFound + "\n", nil
	}
	relative, err := filepath.Rel(a.store.WorktreeRoot, path)
	if err != nil {
		return nil, "", fmt.Errorf("plan %s: %w", plan, err)
	}
	relative = filepath.ToSlash(relative)
	return &contextPlanData{Name: plan, Path: relative}, contextPlanHeading + " " + relative + "\n", nil
}

// applyingNotes lists the Notes that apply to an Issue with title and body, at most
// contextNoteLimit: first, in Note path order, each Note with a ref whose path part body
// mentions, then the Notes a search on title finds, every word first, else any word, best match
// first. Retired Notes and Notes under one are left out, and each Note appears once.
func (a *app) applyingNotes(title string, body string) ([]applyingNote, error) {
	notes, err := a.issueIndex.Notes(false)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]index.NoteEntry, len(notes))
	for _, note := range notes {
		byID[note.ID] = note
	}
	refs, err := a.issueIndex.NoteRefs()
	if err != nil {
		return nil, err
	}
	hits, _, err := searchEveryWordFirst(
		func() ([]index.NoteEntry, error) { return a.issueIndex.SearchNotes(title, nil, false) },
		func() ([]index.NoteEntry, error) { return a.issueIndex.SearchNotesAnyWord(title, nil, false) },
	)
	if err != nil {
		return nil, err
	}

	applying := make([]applyingNote, 0, contextNoteLimit)
	isListed := map[string]bool{}
	add := func(note applyingNote) {
		if len(applying) < contextNoteLimit && !isListed[note.entry.ID] {
			isListed[note.entry.ID] = true
			applying = append(applying, note)
		}
	}
	for _, ref := range refs {
		refPath, _, _ := strings.Cut(ref.Ref, "#")
		if note, isUsable := byID[ref.NoteID]; isUsable && mentionsPath(body, refPath) {
			add(applyingNote{entry: note, ref: ref.Ref})
		}
	}
	for _, hit := range hits {
		add(applyingNote{entry: hit})
	}
	return applying, nil
}

// contextNotes renders the applying Notes: the --json form and the notes section, one
// `<id>  <title>  (ref <ref>)` or `<id>  <title>  (search)` line each, followed, unless isBrief,
// by the bodies of the first contextBodyLimit, each under `--- <id>: <title>` and cut after
// contextBodyLineLimit lines. The bodies come from the Note files, as know show reads them.
func (a *app) contextNotes(applying []applyingNote, isBrief bool) ([]contextNoteData, string, error) {
	notes := make([]contextNoteData, 0, len(applying))
	rows := make([]string, 0, len(applying))
	shown := make([]index.NoteEntry, 0, contextBodyLimit)
	for _, note := range applying {
		data := contextNoteData{noteData: newNoteData(note.entry), Source: contextSourceSearch, Ref: note.ref}
		found := contextSourceSearch
		if note.ref != "" {
			data.Source = contextSourceRef
			found = contextSourceRef + " " + note.ref
		}
		notes = append(notes, data)
		rows = append(rows, fmt.Sprintf("%s%s  %s  (%s)", textIndent, note.entry.ID, flatten(note.entry.Title), found))
		if !isBrief && len(shown) < contextBodyLimit {
			shown = append(shown, note.entry)
		}
	}
	text := contextSection(contextNotesHeading, rows, contextNoteLimit, "")
	if len(shown) == 0 {
		return notes, text, nil
	}

	bodies, err := a.noteShowResult(shown)
	if err != nil {
		return nil, "", err
	}
	var bodyText strings.Builder
	for position, read := range bodies.Data.([]noteData) {
		notes[position].noteData = read
		bodyText.WriteString("\n" + noteBodyText(read))
	}
	return notes, text + bodyText.String(), nil
}

// noteBodyText renders one Note's body for context: `--- <id>: <title>`, then the body without
// its leading and trailing blank lines, cut after contextBodyLineLimit lines with a
// `... cut: capcom know show <id>` line.
func noteBodyText(note noteData) string {
	var text strings.Builder
	fmt.Fprintf(&text, "%s %s: %s\n", contextBodyMarker, note.ID, flatten(note.Title))
	lines := splitRows(strings.Trim(*note.Body, "\n"))
	for _, line := range lines[:min(contextBodyLineLimit, len(lines))] {
		text.WriteString(line + "\n")
	}
	if len(lines) > contextBodyLineLimit {
		text.WriteString("... cut: capcom know show " + note.ID + "\n")
	}
	return text.String()
}

// mentionsPath reports whether text mentions path, a slash path without a trailing slash
// counting: whole, between characters that cannot belong to a path, or as a directory, followed
// by a slash. A path ending a sentence before its full stop still counts. An empty path is never
// mentioned.
func mentionsPath(text string, path string) bool {
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return false
	}
	for offset := 0; ; {
		at := strings.Index(text[offset:], path)
		if at < 0 {
			return false
		}
		start := offset + at
		end := start + len(path)
		if (start == 0 || !isPathByte(text[start-1])) && endsPath(text[end:]) {
			return true
		}
		offset = start + 1
	}
}

// endsPath reports whether rest, the text after a path mention, ends the path there: it is empty,
// starts with a slash (the path is a directory) or with a byte that cannot belong to a path, or
// with a full stop that ends the text or comes before such a byte.
func endsPath(rest string) bool {
	switch {
	case rest == "", rest[0] == '/', !isPathByte(rest[0]):
		return true
	case rest[0] == '.':
		return len(rest) == 1 || !isPathByte(rest[1])
	default:
		return false
	}
}

// isPathByte reports whether b can belong to a path as Note refs write them: an ASCII letter or
// digit, one of `_-./`, or a byte of a non-ASCII character.
func isPathByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	case b == '_', b == '-', b == '.', b == '/':
		return true
	default:
		return b >= 0x80
	}
}
