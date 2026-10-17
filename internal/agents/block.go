// Package agents owns the Managed block of AGENTS.md: the part of the hand-written agent guide
// between `<!-- BEGIN CAPCOM -->` and `<!-- END CAPCOM -->` that capcom writes. The block is
// rendered deterministically from the top level of the Knowledge tree and the configured doing
// status, the one its claim line names, and a write splices it in by its markers, or appends it
// when absent, keeping every byte outside the markers and the file's line ending. Sync is the one way callers reconcile the block with the Knowledge tree: it reads the
// top level and inspects, refreshes or writes in one pass. The block is read and written only in a
// file inside the working tree: an AGENTS.md that links outside it is never read or written. Guides
// checks that the other agent guides beside AGENTS.md (CLAUDE.md, GEMINI.md) reach the block, by
// being AGENTS.md through a link or by an `@AGENTS.md` import line, and appends that line to the
// ones that do not.
package agents

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/tree"
)

// FileName is the agent guide the block lives in, at the working tree root.
const FileName = "AGENTS.md"

// The marker lines that delimit the Managed block.
const (
	BeginMarker = "<!-- BEGIN CAPCOM -->"
	EndMarker   = "<!-- END CAPCOM -->"
)

// Line endings the block is spliced in.
const (
	lineFeed       = "\n"
	carriageReturn = "\r"
	crlf           = carriageReturn + lineFeed
)

// newFilePermissions are the permissions of a newly created AGENTS.md.
const newFilePermissions = 0o644

// blockHeader is the fixed part of the block between the begin marker and the command list.
const blockHeader = `<!-- Managed by capcom: run "capcom agents" to refresh. Edits inside these markers are overwritten. -->
## Issues and knowledge: capcom

capcom is the issue register of record for this repository. Issues and Knowledge Notes are
markdown files under ` + "`.capcom/`" + `, versioned with the code; track work there and nowhere else.

Without a ` + "`capcom`" + ` binary, install it from https://github.com/airiclenz/capcom#install.

`

// loopOpening is the command list up to the claim line: finding work and reading it.
const loopOpening = "- `capcom ready`: the issues ready to work on, most urgent first\n" +
	"- `capcom show <id>`: one issue in full, with what blocks it\n"

// claimCommand opens the claim line; the configured doing status follows it.
const claimCommand = "- `capcom update <id> --status "

// claimPurpose closes the claim line after the doing status.
const claimPurpose = "`: claim it before you start\n"

// loopClosing is the command list after the claim line: the rest of the work loop, then the other
// commands, then where their flags are.
const loopClosing = "- `capcom log <id> <text>`: note progress as work lands\n" +
	"- `capcom close <id> --reason <text>`: close an issue; it names the issues this made ready\n" +
	"- `capcom reopen <id> --reason <text>`: undo a close\n" +
	"- `capcom create --id <slug> --title <title> [--dep <id>]`: file a new issue\n" +
	"- `capcom dep add <id> <dep-id>`: make an issue wait for another; `capcom dep list <id>` shows both sides\n" +
	"- `capcom list --tree --depth 2`: the outline of the issues\n" +
	"- `capcom know tree`: the outline of the Knowledge tree; `capcom know show <id>` reads a Note\n" +
	"\n" +
	"Each command's flags are in `capcom <cmd> --help`.\n" +
	"\n"

// blockGuidance is the fixed part of the block between the command list and the Knowledge list.
const blockGuidance = `An issue's body says what is wrong or wanted, why, and how to tell it is done (` + "`## Acceptance`" + `),
readable on its own; a plan says how and in what order.

Where things are: an issue is ` + "`.capcom/issues/<id>.md`" + `, and its children sit in the directory
` + "`.capcom/issues/<id>/`" + ` beside it; Notes follow the same shape under ` + "`.capcom/knowledge/`" + `.
Prefer capcom commands; after a hand edit run ` + "`capcom check`" + `.

A standing issue (` + "`standing: true`" + ` in its file) only groups its children: never claim or close it.

Tree lock: before a change that spans several files or commits, it is recommended to claim this
worktree with ` + "`capcom lock acquire --purpose <text>`" + ` and keep the token it prints.
` + "`capcom lock touch --token <token>`" + ` renews it during long work,
` + "`capcom lock release --token <token>`" + ` frees it when done, and acquiring again with ` + "`--token <token>`" + `
resumes it after a restart or compaction. When ` + "`capcom lock check --token <token>`" + ` exits 3, or
` + "`capcom lock check`" + ` does while you hold no token, another session holds this worktree: ask the user
before editing it.

`

// Headings of the Knowledge list.
const (
	knowledgeHeading = "Knowledge tree, top level:"
	noNotesLine      = "Knowledge tree, top level: no Notes yet."
)

// Note is one top-level Note as the block lists it.
type Note struct {
	ID    string
	Title string
}

// State is what Sync found in AGENTS.md.
type State int

const (
	// StateAbsent is an AGENTS.md without the block, no AGENTS.md at all, or an AGENTS.md that links
	// outside the working tree when the mode does not write.
	StateAbsent State = iota
	// StateCurrent is a block equal to what render produces now.
	StateCurrent
	// StateStale is a block that differs from what render produces now.
	StateStale
)

// Mode is what Sync does with the block once it knows its State.
type Mode int

const (
	// ModeInspect reports the State and writes nothing.
	ModeInspect Mode = iota
	// ModeRefresh rewrites a stale block only, leaving an AGENTS.md without the block, or no
	// AGENTS.md, as it is.
	ModeRefresh
	// ModeWrite brings the block up to date: a stale block is replaced, an absent one appended,
	// and a missing AGENTS.md created.
	ModeWrite
)

// Status is the block's State and, when the block is present, the 1-based line of its begin
// marker.
type Status struct {
	State State
	Line  int
}

// Error is an AGENTS.md whose markers cannot be told apart: a marker repeated, an end marker
// before the begin marker, or a begin marker never closed. Line is the offending marker's line.
// It is also an AGENTS.md that cannot be read, being not a regular file or larger than 1 MiB, and
// one a write refuses because it links outside the working tree; Line is 0 then. Path is the file's
// name in the working tree: AGENTS.md, or the agent guide Guides refuses to read.
type Error struct {
	Path    string
	Line    int
	Message string
}

// Error renders "path:line: message", or "path: message" when Line is 0.
func (e *Error) Error() string {
	if e.Line == 0 {
		return fmt.Sprintf("%s: %s", e.Path, e.Message)
	}
	return fmt.Sprintf("%s:%d: %s", e.Path, e.Line, e.Message)
}

// guidePath returns the AGENTS.md of the working tree at worktreeRoot.
func guidePath(worktreeRoot string) string {
	return namedGuidePath(worktreeRoot, FileName)
}

// namedGuidePath returns the agent guide name, a path relative to the working tree in slash form,
// of the working tree at worktreeRoot.
func namedGuidePath(worktreeRoot, name string) string {
	return filepath.Join(worktreeRoot, filepath.FromSlash(name))
}

// topNotes reads the top level of the Knowledge tree in knowledgeDir, ordered by id, straight from
// the files. A retired Note is left out, and with it its whole subtree, and so is a Note labelled
// knowledge.LabelBackground. A Note file the Note reader judges unusable (one that
// nodefile.ReadRegularFile refuses unread, that does not parse, or whose fields or aliases do not
// validate) is left out, as the index leaves it out of `capcom know tree`, and so is one a load
// problem names (a duplicate id or alias, an invalid or repeated alias); `capcom check` reports
// each. It fails only when the tree or a Note file cannot be read.
func topNotes(knowledgeDir string) ([]Note, error) {
	reader := nodestore.NoteReader()
	readings := map[string]nodestore.Reading[knowledge.Note]{}
	readContent := func(path string) (tree.Content, error) {
		// The findings are not reported here, so the path relative to knowledgeDir names them.
		reading, err := reader.Read(path, filepath.ToSlash(relativeTo(knowledgeDir, path)))
		if err != nil {
			return tree.Content{}, fmt.Errorf("%s: %w", path, err)
		}
		readings[path] = reading
		return contentOf(reading), nil
	}
	loaded, err := tree.LoadContent(knowledgeDir, "note", readContent)
	var problems tree.Errors
	if err != nil && !errors.As(err, &problems) {
		return nil, err
	}
	isNamed := map[string]bool{}
	for _, problem := range problems {
		isNamed[problem.Path] = true
		isNamed[problem.Other] = true
	}

	var notes []Note
	for _, node := range loaded.Nodes() {
		if node.ParentID != "" || isNamed[node.Path] {
			continue
		}
		reading := readings[node.Path]
		note := reading.Node
		if !reading.IsUsable() || note.IsRetired || knowledge.IsBackground(note.Labels) {
			continue
		}
		notes = append(notes, Note{ID: node.ID, Title: note.Title})
	}
	return notes, nil
}

// contentOf is the tree.Content of a Note file the Note reader read as reading: unread when the
// read was refused, its recovered aliases when it does not parse, and its aliases and usable
// verdict when it parses.
func contentOf(reading nodestore.Reading[knowledge.Note]) tree.Content {
	switch {
	case !reading.IsRead():
		return tree.Content{Parse: tree.UnreadFile}
	case !reading.IsParsed():
		return tree.Content{Aliases: reading.Aliases, Parse: tree.UnparsedFile}
	default:
		return tree.Content{Aliases: reading.Aliases, Parse: tree.ParsedFile, IsUsable: reading.IsUsable()}
	}
}

// relativeTo returns path relative to dir, or path itself when it cannot be made relative.
func relativeTo(dir, path string) string {
	relative, err := filepath.Rel(dir, path)
	if err != nil {
		return path
	}
	return relative
}

// render returns the block for notes and doingStatus, the begin and end markers included, with LF
// line endings and no line ending after the end marker. The claim line names doingStatus, and is
// left out when doingStatus is empty, the config declaring no doing status. Titles are flattened
// onto one line.
func render(notes []Note, doingStatus string) string {
	var block strings.Builder
	block.WriteString(BeginMarker + lineFeed)
	block.WriteString(blockHeader)
	block.WriteString(loopOpening)
	if doingStatus != "" {
		block.WriteString(claimCommand + doingStatus + claimPurpose)
	}
	block.WriteString(loopClosing)
	block.WriteString(blockGuidance)
	if len(notes) == 0 {
		block.WriteString(noNotesLine + lineFeed)
	} else {
		block.WriteString(knowledgeHeading + lineFeed + lineFeed)
		for _, note := range notes {
			block.WriteString("- `" + note.ID + "`: " + flatten(note.Title) + lineFeed)
		}
	}
	block.WriteString(EndMarker)
	return block.String()
}

// inspect reads the AGENTS.md of worktreeRoot and reports whether its block is absent, current or
// stale against notes and doingStatus. A missing file is StateAbsent, and so is one that links outside the working
// tree, which is not read. Line endings are ignored in the comparison, so a CRLF file holding the
// current block is current. It fails with an *Error on markers that cannot be told apart and on a
// file that is not a regular file or is larger than 1 MiB, and when the file cannot be read.
func inspect(worktreeRoot string, notes []Note, doingStatus string) (Status, error) {
	return reconcile(worktreeRoot, notes, doingStatus, ModeInspect)
}

// write brings the block of the AGENTS.md of worktreeRoot up to date with notes and doingStatus and
// returns the
// Status found before writing. A stale block is replaced in place, an absent one is appended after
// a blank line, and a missing file is created holding the block alone; a current block leaves the
// file untouched. Every byte outside the markers is kept, and the block takes the file's line
// ending (CRLF when the file has any). It fails as inspect does, and when the file cannot be
// written. An AGENTS.md that links outside the working tree fails it with an *Error, the file it
// links to neither read nor written.
func write(worktreeRoot string, notes []Note, doingStatus string) (Status, error) {
	return reconcile(worktreeRoot, notes, doingStatus, ModeWrite)
}

// Sync reconciles the block of the AGENTS.md of worktreeRoot with the top level of the Knowledge
// tree in knowledgeDir and with doingStatus, the configured doing status the claim line names (the
// empty string when the config declares none, which leaves the claim line out), as mode says, and
// returns the Status found before any write. Renaming the doing status therefore stales the block. It reads the
// file once: the Status it reports is the one it acted on. It fails as topNotes does, as inspect
// does, and, when mode writes, when the file cannot be written. An AGENTS.md that links outside the
// working tree is never read: ModeWrite fails as write does, and the other modes report
// StateAbsent.
func Sync(worktreeRoot, knowledgeDir, doingStatus string, mode Mode) (Status, error) {
	notes, err := topNotes(knowledgeDir)
	if err != nil {
		return Status{}, err
	}
	return reconcile(worktreeRoot, notes, doingStatus, mode)
}

// reconcile resolves the AGENTS.md of worktreeRoot, reads it, finds its block's Status against
// notes and doingStatus and, when mode calls for it, writes the block render produces for them to
// the same file.
// A guide that links outside the working tree is neither read nor written.
func reconcile(worktreeRoot string, notes []Note, doingStatus string, mode Mode) (Status, error) {
	guide, err := resolveGuide(worktreeRoot, FileName)
	if err != nil {
		return Status{}, fmt.Errorf("%s: %w", FileName, err)
	}
	if guide.isOutside && mode == ModeWrite {
		return Status{}, outsideError(guide.path)
	}
	if guide.isOutside {
		return Status{State: StateAbsent}, nil
	}

	content, err := readGuide(guide)
	if err != nil {
		return Status{}, err
	}
	span, err := findBlock(content)
	if err != nil {
		return Status{}, err
	}
	block := render(notes, doingStatus)
	status := statusOf(content, span, block)
	if !mode.writes(status.State) {
		return status, nil
	}
	spliced := splice(content, span, block)
	err = nodefile.WriteFileAtomicVerbatim(guide.path, bytes.NewReader(spliced), guide.perm)
	if err != nil {
		return status, fmt.Errorf("%s: %w", FileName, err)
	}
	return status, nil
}

// guideFile is the file an agent guide of a working tree, AGENTS.md or another, is read from and
// written to.
type guideFile struct {
	// name is the guide's path relative to the working tree, in slash form, as errors name it.
	name string
	// path is the guide's resolved target, so a symlinked guide stays a link, or the guide's own
	// path when it is missing.
	path string
	// perm is the mode a write keeps: the target's, or newFilePermissions for a missing guide.
	perm fs.FileMode
	// isMissing is a guide that does not exist, or whose link dangles: it reads as no content.
	isMissing bool
	// isOutside is a guide whose target lies outside the working tree; perm is unset then.
	isOutside bool
}

// resolveGuide resolves the agent guide name, a path relative to worktreeRoot in slash form, to the
// file it is read from and written to. A missing guide, or one whose link dangles, is the guide's
// own path with newFilePermissions. A target outside the working tree is marked isOutside and is
// not touched.
func resolveGuide(worktreeRoot, name string) (guideFile, error) {
	path := namedGuidePath(worktreeRoot, name)
	target, err := filepath.EvalSymlinks(path)
	if errors.Is(err, fs.ErrNotExist) {
		return guideFile{name: name, path: path, perm: newFilePermissions, isMissing: true}, nil
	}
	if err != nil {
		return guideFile{}, err
	}

	isInside, err := isInsideTree(worktreeRoot, target)
	if err != nil {
		return guideFile{}, err
	}
	if !isInside {
		return guideFile{name: name, path: target, isOutside: true}, nil
	}
	info, err := os.Stat(target)
	if err != nil {
		return guideFile{}, err
	}
	return guideFile{name: name, path: target, perm: info.Mode().Perm()}, nil
}

// isInsideTree reports whether target, a path with its links resolved, lies inside the working tree
// at worktreeRoot, its links resolved too. It fails when worktreeRoot cannot be resolved.
func isInsideTree(worktreeRoot, target string) (bool, error) {
	root, err := filepath.EvalSymlinks(worktreeRoot)
	if err != nil {
		return false, err
	}
	relative, err := filepath.Rel(root, target)
	if err != nil {
		// Rel fails only on paths it cannot relate, such as ones on different volumes: outside.
		return false, nil
	}
	return filepath.IsLocal(relative), nil
}

// outsideError is the refusal of a write to an AGENTS.md that links to target, outside the working
// tree.
func outsideError(target string) *Error {
	return &Error{
		Path:    FileName,
		Message: "links to " + target + ", outside the working tree; the capcom block is not read or written",
	}
}

// writes reports whether mode rewrites a block found in state.
func (m Mode) writes(state State) bool {
	switch {
	case state == StateCurrent:
		return false
	case m == ModeWrite:
		return true
	case m == ModeRefresh:
		return state == StateStale
	default:
		return false
	}
}

// statusOf compares the block span locates in content with block, the one render produces now,
// carriage returns stripped.
func statusOf(content []byte, span blockSpan, block string) Status {
	if !span.isFound {
		return Status{State: StateAbsent}
	}
	existing := strings.ReplaceAll(string(content[span.start:span.end]), carriageReturn, "")
	if existing == block {
		return Status{State: StateCurrent, Line: span.line}
	}
	return Status{State: StateStale, Line: span.line}
}

// readGuide reads the file guide resolved to through nodefile.ReadRegularFile, returning nil content
// when the guide is missing or its file no longer exists. A file that is not a regular file, or is
// larger than 1 MiB, is refused with an *Error naming the guide, without blocking or reading it
// whole.
func readGuide(guide guideFile) ([]byte, error) {
	if guide.isMissing {
		return nil, nil
	}
	content, err := nodefile.ReadRegularFile(guide.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case errors.Is(err, tree.ErrNotRegular):
		return nil, unreadableError(guide.name, tree.ErrNotRegular)
	case errors.Is(err, tree.ErrTooLarge):
		return nil, unreadableError(guide.name, tree.ErrTooLarge)
	case err != nil:
		return nil, fmt.Errorf("%s: %w", guide.name, err)
	}
	return content, nil
}

// unreadableError is the refusal of the guide name that the guarded reader refused with refusal,
// its message the refusal's own text.
func unreadableError(name string, refusal error) *Error {
	return &Error{Path: name, Message: refusal.Error()}
}

// blockSpan locates the block in a file: from the start of the begin marker to the end of the end
// marker, before its line ending. line is the begin marker's 1-based line.
type blockSpan struct {
	isFound bool
	start   int
	end     int
	line    int
}

// findBlock scans content for the marker lines, a marker being a line that is exactly the marker
// apart from surrounding white space.
func findBlock(content []byte) (blockSpan, error) {
	var span blockSpan
	isEnded := false
	offset := 0
	for number := 1; offset < len(content); number++ {
		lineEnd := bytes.IndexByte(content[offset:], '\n')
		next := offset + lineEnd + 1
		if lineEnd < 0 {
			next = len(content)
		}
		line := string(content[offset:next])
		switch text := strings.TrimSpace(line); {
		case text == BeginMarker && span.isFound:
			return blockSpan{}, markerError(number, "a second "+BeginMarker+"; keep one capcom block")
		case text == BeginMarker:
			span = blockSpan{isFound: true, start: offset + strings.Index(line, BeginMarker), line: number}
		case text == EndMarker && isEnded:
			return blockSpan{}, markerError(number, "a second "+EndMarker+"; keep one capcom block")
		case text == EndMarker && !span.isFound:
			return blockSpan{}, markerError(number, EndMarker+" without a "+BeginMarker+" before it")
		case text == EndMarker:
			span.end = offset + strings.Index(line, EndMarker) + len(EndMarker)
			isEnded = true
		}
		offset = next
	}
	if span.isFound && !isEnded {
		return blockSpan{}, markerError(span.line, BeginMarker+" is never closed by "+EndMarker)
	}
	return span, nil
}

func markerError(line int, message string) *Error {
	return &Error{Path: FileName, Line: line, Message: message}
}

// splice puts block, given with LF line endings, into content in place of span, or appends it
// after a blank line when span is not found. The block takes content's line ending.
func splice(content []byte, span blockSpan, block string) []byte {
	lineEnding := lineFeed
	if bytes.Contains(content, []byte(crlf)) {
		lineEnding = crlf
		block = strings.ReplaceAll(block, lineFeed, crlf)
	}
	if span.isFound {
		spliced := make([]byte, 0, len(content)+len(block))
		spliced = append(spliced, content[:span.start]...)
		spliced = append(spliced, block...)
		return append(spliced, content[span.end:]...)
	}

	var appended bytes.Buffer
	appended.Write(content)
	if len(content) > 0 {
		if !bytes.HasSuffix(content, []byte(lineFeed)) {
			appended.WriteString(lineEnding)
		}
		appended.WriteString(lineEnding)
	}
	appended.WriteString(block + lineEnding)
	return appended.Bytes()
}

// lineBreaks flattens a title onto one line.
var lineBreaks = strings.NewReplacer(crlf, " ", lineFeed, " ", carriageReturn, " ")

// flatten puts text onto one line.
func flatten(text string) string {
	return lineBreaks.Replace(text)
}
