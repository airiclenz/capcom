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
	// kvKeySuggestions carries an Error's Suggestions in --kv, comma-separated, after SUGGESTION.
	kvKeySuggestions = "SUGGESTIONS"
	// kvKeyValid carries an Error's Valid values in --kv, comma-separated, just before STATUS.
	kvKeyValid = "VALID"
	// kvKeyHint carries an Error's hint in --kv, after VALID and just before STATUS.
	kvKeyHint = "HINT"
	// kvListSeparator joins the items of a list value on one --kv line.
	kvListSeparator = ","
	KVKeyMessage    = "MESSAGE"
	kvStatusOK      = "ok"
	textWarnLabel   = "warning: "
	// textElsewhereLabel starts each Elsewhere line in text mode.
	textElsewhereLabel = "elsewhere: "
	// textSuggestionFormat ends a text failure line that carries a Suggestion.
	textSuggestionFormat = "; did you mean %s?"
	// textValidFormat ends a text failure line that carries Valid values.
	textValidFormat = "; use one of: %s"
	// textHintFormat is the line after a text failure line that names its hint.
	textHintFormat = "hint: %s\n"
	// textListSeparator joins the items of a list in a text failure line.
	textListSeparator = ", "
	// textLastAlternative joins the last two Suggestions in a text failure line.
	textLastAlternative = " or "
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
	// Suggestions are the closest known ids or aliases to one the caller named, closest first.
	Suggestions []string `json:"suggestions,omitempty"`
	// Valid are the declared values the flag accepts.
	Valid []string `json:"valid,omitempty"`
	// Hint is the command that gets the caller past the failure.
	Hint string `json:"hint,omitempty"`
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
// when the failure carries a Suggestion, "; did you mean a, b or c?" when it carries Suggestions
// and "; use one of: x, y, z" when it carries Valid values, in that order, then the line
// "hint: <command>" when the failure has a hint.
func (r textRenderer) RenderError(failure *Error) error {
	line := failure.Error()
	if failure.Suggestion != "" {
		line += fmt.Sprintf(textSuggestionFormat, failure.Suggestion)
	}
	if len(failure.Suggestions) > 0 {
		line += fmt.Sprintf(textSuggestionFormat, JoinAlternatives(failure.Suggestions))
	}
	if len(failure.Valid) > 0 {
		line += fmt.Sprintf(textValidFormat, strings.Join(failure.Valid, textListSeparator))
	}
	text := fmt.Sprintf("capcom: %s\n", line)
	if hint := failure.hintText(); hint != "" {
		text += fmt.Sprintf(textHintFormat, hint)
	}
	_, err := io.WriteString(r.stderr, sanitizeTerminal(text))
	return err
}

// JoinAlternatives lists items as "a", "a or b" or "a, b or c": comma-separated, the last two
// joined by "or".
func JoinAlternatives(items []string) string {
	if len(items) == 1 {
		return items[0]
	}
	last := len(items) - 1
	return strings.Join(items[:last], textListSeparator) + textLastAlternative + items[last]
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
		Code:        failure.Code,
		Message:     failure.Message,
		Path:        failure.Path,
		Line:        failure.Line,
		Suggestion:  failure.Suggestion,
		Suggestions: failure.Suggestions,
		Valid:       failure.Valid,
		Hint:        failure.hintText(),
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
	for _, pair := range ErrorHintPairs(failure) {
		writeKVLine(&out, pair.Key, pair.Value)
	}
	writeKVLine(&out, kvKeyStatus, string(failure.Code))
	_, err := io.WriteString(r.stdout, out.String())
	return err
}

// ErrorHintPairs are the --kv lines that carry failure's hints, in order: SUGGESTION= when it
// has a Suggestion, SUGGESTIONS= and VALID= (each comma-separated) when it has Suggestions or
// Valid values, and HINT= when it has a hint. A failure without hints has none. They belong just
// before the failure's STATUS= line.
func ErrorHintPairs(failure *Error) []KVPair {
	var pairs []KVPair
	if failure.Suggestion != "" {
		pairs = append(pairs, KVPair{Key: kvKeySuggestion, Value: failure.Suggestion})
	}
	if len(failure.Suggestions) > 0 {
		pairs = append(pairs, KVPair{
			Key:   kvKeySuggestions,
			Value: strings.Join(failure.Suggestions, kvListSeparator),
		})
	}
	if len(failure.Valid) > 0 {
		pairs = append(pairs, KVPair{Key: kvKeyValid, Value: strings.Join(failure.Valid, kvListSeparator)})
	}
	if hint := failure.hintText(); hint != "" {
		pairs = append(pairs, KVPair{Key: kvKeyHint, Value: hint})
	}
	return pairs
}

// KVLineBreaks flattens a value onto one line; a KEY=VALUE consumer reads line by line.
var KVLineBreaks = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")

func writeKVLine(out *strings.Builder, key, value string) {
	out.WriteString(key + "=" + KVLineBreaks.Replace(value) + "\n")
}
