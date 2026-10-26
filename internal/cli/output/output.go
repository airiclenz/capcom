package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/airiclenz/capcom/internal/elsewhere"
)

// SchemaVersion is the version of the --json envelope. It changes only when an existing field
// changes meaning or disappears.
const SchemaVersion = 1

// The values of a search's Match: every query word held, or at least one of them.
const (
	MatchAll = "all"
	MatchAny = "any"
)

const (
	KindError    = "error"
	kvKeyStatus  = "STATUS"
	kvKeyWarning = "WARNING"
	// kvKeySuggestion carries an Error's Suggestion in --kv, just before its STATUS line.
	kvKeySuggestion = "SUGGESTION"
	KVKeyMessage    = "MESSAGE"
	kvStatusOK      = "ok"
	textWarnLabel   = "warning: "
	// textElsewhereLabel starts each Elsewhere line in text mode.
	textElsewhereLabel = "elsewhere: "
	// textSuggestionFormat ends a text failure line that carries a Suggestion.
	textSuggestionFormat = "; did you mean %s?"
)

// Result is what a successful command hands to the output layer. Text is written in text mode
// with control characters escaped by sanitizeTerminal (the command owns its trailing newline); Kind and Data form the --json envelope;
// KV holds the --kv records, and a command that leaves KV nil does not support --kv. Failure,
// when set, is a failure that goes with a partial result (a multi-id command missing one id): text
// writes it to stderr after the result, --json sets it as the envelope's error next to its data,
// and --kv leaves it to the records' own STATUS lines. Elsewhere holds the issue changes on other
// branches, reported apart from the result: text writes one line each after it, --json puts them
// in the envelope's elsewhere member, and --kv leaves them out. Match, set only by the search
// commands, tells --json whether the hits hold every query word (MatchAll) or some (MatchAny);
// text and --kv leave it out.
type Result struct {
	Kind      string
	Match     string
	Data      any
	Text      string
	KV        []KVRecord
	Elsewhere []elsewhere.Entry
	Warnings  []string
	Failure   *Error
}

// KVRecord is one block of --kv output: its pairs in order, then a closing STATUS line
// (Status, or "ok" when empty).
type KVRecord struct {
	Pairs  []KVPair
	Status string
}

// KVPair is one KEY=VALUE line. Keys are upper-case; values are flattened to a single line.
type KVPair struct {
	Key   string
	Value string
}

// Envelope is the --json document every command writes to stdout. Data is set on success, null
// when a command found nothing to report (a typed nil, such as next's with no Ready Issue), and
// Error on failure; Elsewhere and Warnings are always present, empty when there is nothing to
// report. Match is present only on a search's result: MatchAll or MatchAny.
type Envelope struct {
	Schema    int               `json:"schema"`
	Kind      string            `json:"kind"`
	Match     string            `json:"match,omitempty"`
	Data      any               `json:"data,omitempty"`
	Error     *ErrorBody        `json:"error,omitempty"`
	Elsewhere []elsewhere.Entry `json:"elsewhere"`
	Warnings  []string          `json:"warnings"`
}

// ErrorBody is the error member of a failure Envelope.
type ErrorBody struct {
	Code       Code   `json:"code"`
	Message    string `json:"message"`
	Path       string `json:"path,omitempty"`
	Line       int    `json:"line,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// Renderer writes a command's outcome in one output format.
type Renderer interface {
	// Render writes a successful result.
	Render(result Result) error
	// RenderError writes a failure.
	RenderError(failure *Error) error
}

// OutputFormat names the format the global flags select.
type OutputFormat int

const (
	FormatText OutputFormat = iota
	FormatJSON
	FormatKV
)

// NewRenderer returns the Renderer for format, writing results to stdout and, in text mode,
// failures to stderr.
func NewRenderer(format OutputFormat, stdout, stderr io.Writer) Renderer {
	switch format {
	case FormatJSON:
		return jsonRenderer{stdout: stdout}
	case FormatKV:
		return kvRenderer{stdout: stdout}
	default:
		return textRenderer{stdout: stdout, stderr: stderr}
	}
}

// textRenderer writes plain text: the result and any warnings to stdout, failures to stderr.
type textRenderer struct {
	stdout io.Writer
	stderr io.Writer
}

func (r textRenderer) Render(result Result) error {
	var out strings.Builder
	out.WriteString(result.Text)
	for _, entry := range result.Elsewhere {
		out.WriteString(textElsewhereLabel + entry.String() + "\n")
	}
	for _, warning := range result.Warnings {
		out.WriteString(textWarnLabel + warning + "\n")
	}
	if _, err := io.WriteString(r.stdout, sanitizeTerminal(out.String())); err != nil {
		return err
	}
	if result.Failure == nil {
		return nil
	}
	return r.RenderError(result.Failure)
}

// RenderError writes the failure as one capcom: line, ending in "; did you mean <suggestion>?"
// when the failure carries a Suggestion.
func (r textRenderer) RenderError(failure *Error) error {
	line := failure.Error()
	if failure.Suggestion != "" {
		line += fmt.Sprintf(textSuggestionFormat, failure.Suggestion)
	}
	_, err := fmt.Fprintf(r.stderr, "capcom: %s\n", sanitizeTerminal(line))
	return err
}

// C0 and C1 control ranges escaped by sanitizeTerminal.
const (
	firstPrintableRune = 0x20
	deleteRune         = 0x7f
	firstC1Rune        = 0x80
	lastC1Rune         = 0x9f
)

// sanitizeTerminal returns text with every byte a terminal could act on written as a visible
// \xHH escape: C0 controls other than TAB and LF, DEL, C1 controls (U+0080 to U+009F) and each
// byte of invalid UTF-8. Repository files are untrusted, and a raw escape sequence in a title or
// body could clear the screen, set the window title or write to the clipboard. Everything else
// is returned unchanged.
func sanitizeTerminal(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	for position := 0; position < len(text); {
		character, width := utf8.DecodeRuneInString(text[position:])
		switch {
		case character == utf8.RuneError && width == 1:
			fmt.Fprintf(&out, "\\x%02x", text[position])
		case isTerminalControl(character):
			fmt.Fprintf(&out, "\\x%02x", character)
		default:
			out.WriteString(text[position : position+width])
		}
		position += width
	}
	return out.String()
}

// isTerminalControl reports whether character is a control sanitizeTerminal escapes.
func isTerminalControl(character rune) bool {
	if character == '\t' || character == '\n' {
		return false
	}
	return character < firstPrintableRune || character == deleteRune ||
		(character >= firstC1Rune && character <= lastC1Rune)
}

// jsonRenderer writes one Envelope per command to stdout, failures included, so a caller parses
// stdout alone.
type jsonRenderer struct {
	stdout io.Writer
}

func (r jsonRenderer) Render(result Result) error {
	envelope := Envelope{
		Kind:      result.Kind,
		Match:     result.Match,
		Data:      result.Data,
		Elsewhere: result.Elsewhere,
		Warnings:  result.Warnings,
	}
	if result.Failure != nil {
		envelope.Error = errorBody(result.Failure)
	}
	return r.write(envelope)
}

func (r jsonRenderer) RenderError(failure *Error) error {
	return r.write(Envelope{Kind: KindError, Error: errorBody(failure)})
}

// errorBody is the envelope's error member for failure.
func errorBody(failure *Error) *ErrorBody {
	return &ErrorBody{
		Code:       failure.Code,
		Message:    failure.Message,
		Path:       failure.Path,
		Line:       failure.Line,
		Suggestion: failure.Suggestion,
	}
}

func (r jsonRenderer) write(envelope Envelope) error {
	envelope.Schema = SchemaVersion
	if envelope.Elsewhere == nil {
		envelope.Elsewhere = []elsewhere.Entry{}
	}
	if envelope.Warnings == nil {
		envelope.Warnings = []string{}
	}
	encoder := json.NewEncoder(r.stdout)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(envelope)
}

// kvRenderer writes KEY=VALUE lines to stdout: warnings first, then each record closed by its
// STATUS line, so STATUS= is always the last line.
type kvRenderer struct {
	stdout io.Writer
}

func (r kvRenderer) Render(result Result) error {
	var out strings.Builder
	for _, warning := range result.Warnings {
		writeKVLine(&out, kvKeyWarning, warning)
	}
	for _, record := range result.KV {
		for _, pair := range record.Pairs {
			writeKVLine(&out, pair.Key, pair.Value)
		}
		status := record.Status
		if status == "" {
			status = kvStatusOK
		}
		writeKVLine(&out, kvKeyStatus, status)
	}
	_, err := io.WriteString(r.stdout, out.String())
	return err
}

func (r kvRenderer) RenderError(failure *Error) error {
	var out strings.Builder
	writeKVLine(&out, KVKeyMessage, failure.Error())
	if failure.Suggestion != "" {
		writeKVLine(&out, kvKeySuggestion, failure.Suggestion)
	}
	writeKVLine(&out, kvKeyStatus, string(failure.Code))
	_, err := io.WriteString(r.stdout, out.String())
	return err
}

// KVLineBreaks flattens a value onto one line; a KEY=VALUE consumer reads line by line.
var KVLineBreaks = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")

func writeKVLine(out *strings.Builder, key, value string) {
	out.WriteString(key + "=" + KVLineBreaks.Replace(value) + "\n")
}
