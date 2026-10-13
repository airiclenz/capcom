// Package agents owns the Managed block of AGENTS.md: the part of the hand-written agent guide
// between `<!-- BEGIN CAPCOM -->` and `<!-- END CAPCOM -->` that capcom writes. The block is
// rendered deterministically from the top level of the Knowledge tree, and a write splices it in by
// its markers, or appends it when absent, keeping every byte outside the markers and the file's
// line ending. Sync is the one way callers reconcile the block with the Knowledge tree: it reads the
// top level and inspects, refreshes or writes in one pass. The block is read and written only in a
// file inside the working tree: an AGENTS.md that links outside it is never read or written.
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

// blockIntro is the fixed part of the block, between the begin marker and the Knowledge list.
const blockIntro = `<!-- Managed by capcom: run "capcom agents" to refresh. Edits inside these markers are overwritten. -->
## Issues and knowledge: capcom

capcom is the issue register of record for this repository. Issues and Knowledge Notes are
markdown files under ` + "`.capcom/`" + `, versioned with the code; track work there and nowhere else.

Without a ` + "`capcom`" + ` binary, install it from https://github.com/airiclenz/capcom#install.

- ` + "`capcom ready`" + `: the issues ready to work on, most urgent first
- ` + "`capcom show <id>`" + `: one issue in full, with what blocks it
- ` + "`capcom create --id <slug> --title <title> [--dep <id>]`" + `: file a new issue
- ` + "`capcom dep add <id> <dep-id>`" + `: make an issue wait for another; ` + "`capcom dep list <id>`" + ` shows both sides
- ` + "`capcom close <id> --reason <text>`" + `: close an issue; it names the issues this made ready
- ` + "`capcom list --tree --depth 2`" + `: the outline of the issues
- ` + "`capcom know tree`" + `: the outline of the Knowledge tree; ` + "`capcom know show <id>`" + ` reads a Note

An issue's body says what is wrong or wanted, why, and how to tell it is done (` + "`## Acceptance`" + `),
readable on its own; a plan says how and in what order.

Where things are: an issue is ` + "`.capcom/issues/<id>.md`" + `, and its children sit in the directory
` + "`.capcom/issues/<id>/`" + ` beside it; Notes follow the same shape under ` + "`.capcom/knowledge/`" + `.
Prefer capcom commands; after a hand edit run ` + "`capcom check`" + `.

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
// one a write refuses because it links outside the working tree; Line is 0 then.
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
	return filepath.Join(worktreeRoot, FileName)
}

// topNotes reads the top level of the Knowledge tree in knowledgeDir, ordered by id, straight from
// the files. A retired Note is left out, and with it its whole subtree, and so is a Note labelled
// knowledge.LabelBackground. A Note file that does not parse, that nodefile.ReadNodeFile refuses
// unread, or that a load problem names (a duplicate id or alias, an invalid or repeated alias) is
// left out; `capcom check` reports it. It fails only when the tree or a Note file cannot be read.
func topNotes(knowledgeDir string) ([]Note, error) {
	parsed := map[string]knowledge.Note{}
	readAliases := func(path string) ([]string, error) {
		data, err := nodefile.ReadNodeFile(path)
		if errors.Is(err, tree.ErrNotRegular) || errors.Is(err, tree.ErrTooLarge) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		note, err := knowledge.Parse(path, data)
		if err != nil {
			return nil, nil
		}
		parsed[path] = note
		return note.Aliases, nil
	}
	loaded, err := tree.Load(knowledgeDir, readAliases)
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
		note, isParsed := parsed[node.Path]
		if !isParsed || note.IsRetired || knowledge.IsBackground(note.Labels) {
			continue
		}
		notes = append(notes, Note{ID: node.ID, Title: note.Title})
	}
	return notes, nil
}

// render returns the block for notes, the begin and end markers included, with LF line endings
// and no line ending after the end marker. Titles are flattened onto one line.
func render(notes []Note) string {
	var block strings.Builder
	block.WriteString(BeginMarker + lineFeed)
	block.WriteString(blockIntro)
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
// stale against notes. A missing file is StateAbsent, and so is one that links outside the working
// tree, which is not read. Line endings are ignored in the comparison, so a CRLF file holding the
// current block is current. It fails with an *Error on markers that cannot be told apart and on a
// file that is not a regular file or is larger than 1 MiB, and when the file cannot be read.
func inspect(worktreeRoot string, notes []Note) (Status, error) {
	return reconcile(worktreeRoot, notes, ModeInspect)
}

// write brings the block of the AGENTS.md of worktreeRoot up to date with notes and returns the
// Status found before writing. A stale block is replaced in place, an absent one is appended after
// a blank line, and a missing file is created holding the block alone; a current block leaves the
// file untouched. Every byte outside the markers is kept, and the block takes the file's line
// ending (CRLF when the file has any). It fails as inspect does, and when the file cannot be
// written. An AGENTS.md that links outside the working tree fails it with an *Error, the file it
// links to neither read nor written.
func write(worktreeRoot string, notes []Note) (Status, error) {
	return reconcile(worktreeRoot, notes, ModeWrite)
}

// Sync reconciles the block of the AGENTS.md of worktreeRoot with the top level of the Knowledge
// tree in knowledgeDir, as mode says, and returns the Status found before any write. It reads the
// file once: the Status it reports is the one it acted on. It fails as topNotes does, as inspect
// does, and, when mode writes, when the file cannot be written. An AGENTS.md that links outside the
// working tree is never read: ModeWrite fails as write does, and the other modes report
// StateAbsent.
func Sync(worktreeRoot, knowledgeDir string, mode Mode) (Status, error) {
	notes, err := topNotes(knowledgeDir)
	if err != nil {
		return Status{}, err
	}
	return reconcile(worktreeRoot, notes, mode)
}

// reconcile resolves the AGENTS.md of worktreeRoot, reads it, finds its block's Status against
// notes and, when mode calls for it, writes the block render produces for notes to the same file.
// A guide that links outside the working tree is neither read nor written.
func reconcile(worktreeRoot string, notes []Note, mode Mode) (Status, error) {
	guide, err := resolveGuide(worktreeRoot)
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
	status := statusOf(content, span, notes)
	if !mode.writes(status.State) {
		return status, nil
	}
	spliced := splice(content, span, render(notes))
	err = nodefile.WriteFileAtomicVerbatim(guide.path, bytes.NewReader(spliced), guide.perm)
	if err != nil {
		return status, fmt.Errorf("%s: %w", FileName, err)
	}
	return status, nil
}

// guideFile is the file the AGENTS.md of a working tree is read from and written to.
type guideFile struct {
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

// resolveGuide resolves the AGENTS.md of worktreeRoot to the file it is read from and written to.
// A missing guide, or one whose link dangles, is the guide's own path with newFilePermissions. A
// target outside the working tree is marked isOutside and is not touched.
func resolveGuide(worktreeRoot string) (guideFile, error) {
	path := guidePath(worktreeRoot)
	target, err := filepath.EvalSymlinks(path)
	if errors.Is(err, fs.ErrNotExist) {
		return guideFile{path: path, perm: newFilePermissions, isMissing: true}, nil
	}
	if err != nil {
		return guideFile{}, err
	}

	isInside, err := isInsideTree(worktreeRoot, target)
	if err != nil {
		return guideFile{}, err
	}
	if !isInside {
		return guideFile{path: target, isOutside: true}, nil
	}
	info, err := os.Stat(target)
	if err != nil {
		return guideFile{}, err
	}
	return guideFile{path: target, perm: info.Mode().Perm()}, nil
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

// statusOf compares the block span locates in content with the block render produces for notes,
// carriage returns stripped.
func statusOf(content []byte, span blockSpan, notes []Note) Status {
	if !span.isFound {
		return Status{State: StateAbsent}
	}
	existing := strings.ReplaceAll(string(content[span.start:span.end]), carriageReturn, "")
	if existing == render(notes) {
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
		return nil, unreadableError(tree.ErrNotRegular)
	case errors.Is(err, tree.ErrTooLarge):
		return nil, unreadableError(tree.ErrTooLarge)
	case err != nil:
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	return content, nil
}

// unreadableError is the refusal of a guide that the guarded reader refused with refusal, its
// message the refusal's own text.
func unreadableError(refusal error) *Error {
	return &Error{Path: FileName, Message: refusal.Error()}
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
