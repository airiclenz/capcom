// Package agents owns the Managed block of AGENTS.md: the part of the hand-written agent guide
// between `<!-- BEGIN CAPCOM -->` and `<!-- END CAPCOM -->` that capcom writes. Render produces the
// block deterministically from the top level of the Knowledge tree; Inspect tells whether the file
// holds it and whether it is current; Write splices it in by its markers, or appends it when
// absent, keeping every byte outside the markers and the file's line ending.
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

// Permissions of a newly created AGENTS.md and of the temp file it is replaced through.
const (
	newFilePermissions = 0o644
	tempPattern        = "." + FileName + ".tmp-*"
)

// blockIntro is the fixed part of the block, between the begin marker and the Knowledge list.
const blockIntro = `<!-- Managed by capcom: run "capcom agents" to refresh. Edits inside these markers are overwritten. -->
## Issues and knowledge: capcom

capcom is the issue register of record for this repository. Issues and Knowledge Notes are
markdown files under ` + "`.capcom/`" + `, versioned with the code; track work there and nowhere else.

Without a ` + "`capcom`" + ` binary, install it from https://github.com/airiclenz/capcom#install.

- ` + "`capcom ready`" + `: the issues ready to work on, most urgent first
- ` + "`capcom show <id>`" + `: one issue in full
- ` + "`capcom create --id <slug> --title <title>`" + `: file a new issue
- ` + "`capcom close <id> --reason <text>`" + `: close an issue
- ` + "`capcom know tree`" + `: the outline of the Knowledge tree; ` + "`capcom know show <id>`" + ` reads a Note

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

// State is what Inspect found in AGENTS.md.
type State int

const (
	// StateAbsent is an AGENTS.md without the block, or no AGENTS.md at all.
	StateAbsent State = iota
	// StateCurrent is a block equal to what Render produces now.
	StateCurrent
	// StateStale is a block that differs from what Render produces now.
	StateStale
)

// Status is the block's State and, when the block is present, the 1-based line of its begin
// marker.
type Status struct {
	State State
	Line  int
}

// Error is an AGENTS.md whose markers cannot be told apart: a marker repeated, an end marker
// before the begin marker, or a begin marker never closed. Line is the offending marker's line.
type Error struct {
	Path    string
	Line    int
	Message string
}

// Error renders "path:line: message".
func (e *Error) Error() string {
	return fmt.Sprintf("%s:%d: %s", e.Path, e.Line, e.Message)
}

// Path returns the AGENTS.md of the working tree at worktreeRoot.
func Path(worktreeRoot string) string {
	return filepath.Join(worktreeRoot, FileName)
}

// TopNotes reads the top level of the Knowledge tree in knowledgeDir, ordered by id, straight from
// the files. A Note file that does not parse is left out; `capcom check` reports it. It fails only
// when the tree cannot be read.
func TopNotes(knowledgeDir string) ([]Note, error) {
	noAliases := func(string) ([]string, error) { return nil, nil }
	loaded, err := tree.Load(knowledgeDir, noAliases)
	var problems tree.Errors
	if err != nil && !errors.As(err, &problems) {
		return nil, err
	}

	var notes []Note
	for _, node := range loaded.Nodes() {
		if node.ParentID != "" {
			continue
		}
		data, err := os.ReadFile(node.Path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", node.Path, err)
		}
		parsed, err := knowledge.Parse(node.Path, data)
		if err != nil {
			continue
		}
		notes = append(notes, Note{ID: node.ID, Title: parsed.Title})
	}
	return notes, nil
}

// Render returns the block for notes, the begin and end markers included, with LF line endings
// and no line ending after the end marker. Titles are flattened onto one line.
func Render(notes []Note) string {
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

// Inspect reads the AGENTS.md of worktreeRoot and reports whether its block is absent, current or
// stale against notes. A missing file is StateAbsent. Line endings are ignored in the comparison,
// so a CRLF file holding the current block is current. It fails with an *Error on markers that
// cannot be told apart, and when the file cannot be read.
func Inspect(worktreeRoot string, notes []Note) (Status, error) {
	content, err := readGuide(worktreeRoot)
	if err != nil {
		return Status{}, err
	}
	span, err := findBlock(content)
	if err != nil {
		return Status{}, err
	}
	return statusOf(content, span, notes), nil
}

// Write brings the block of the AGENTS.md of worktreeRoot up to date with notes and returns the
// Status found before writing. A stale block is replaced in place, an absent one is appended after
// a blank line, and a missing file is created holding the block alone; a current block leaves the
// file untouched. Every byte outside the markers is kept, and the block takes the file's line
// ending (CRLF when the file has any). It fails as Inspect does, and when the file cannot be
// written.
func Write(worktreeRoot string, notes []Note) (Status, error) {
	content, err := readGuide(worktreeRoot)
	if err != nil {
		return Status{}, err
	}
	span, err := findBlock(content)
	if err != nil {
		return Status{}, err
	}
	status := statusOf(content, span, notes)
	if status.State == StateCurrent {
		return status, nil
	}
	return status, replaceFile(Path(worktreeRoot), splice(content, span, Render(notes)))
}

// statusOf compares the block span locates in content with the block Render produces for notes,
// carriage returns stripped.
func statusOf(content []byte, span blockSpan, notes []Note) Status {
	if !span.isFound {
		return Status{State: StateAbsent}
	}
	existing := strings.ReplaceAll(string(content[span.start:span.end]), carriageReturn, "")
	if existing == Render(notes) {
		return Status{State: StateCurrent, Line: span.line}
	}
	return Status{State: StateStale, Line: span.line}
}

// readGuide reads AGENTS.md, returning nil content when it does not exist.
func readGuide(worktreeRoot string) ([]byte, error) {
	content, err := os.ReadFile(Path(worktreeRoot))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	return content, nil
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

// replaceFile replaces path with content through a temp file renamed over it, keeping the old
// file's permissions; readers see the old file or the whole new one. Unlike tree.WriteFileAtomic it
// keeps CRLF line endings.
func replaceFile(path string, content []byte) error {
	mode := os.FileMode(newFilePermissions)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	temp, err := os.CreateTemp(filepath.Dir(path), tempPattern)
	if err != nil {
		return fmt.Errorf("%s: %w", FileName, err)
	}
	_, writeErr := temp.Write(content)
	closeErr := temp.Close()
	err = errors.Join(writeErr, closeErr, os.Chmod(temp.Name(), mode))
	if err == nil {
		err = os.Rename(temp.Name(), path)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", FileName, errors.Join(err, os.Remove(temp.Name())))
	}
	return nil
}

// lineBreaks flattens a title onto one line.
var lineBreaks = strings.NewReplacer(crlf, " ", lineFeed, " ", carriageReturn, " ")

// flatten puts text onto one line.
func flatten(text string) string {
	return lineBreaks.Replace(text)
}
