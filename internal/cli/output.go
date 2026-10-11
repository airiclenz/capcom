package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// SchemaVersion is the version of the --json envelope. It changes only when an existing field
// changes meaning or disappears.
const SchemaVersion = 1

const (
	kindError     = "error"
	kvKeyStatus   = "STATUS"
	kvKeyWarning  = "WARNING"
	kvKeyMessage  = "MESSAGE"
	kvStatusOK    = "ok"
	textWarnLabel = "warning: "
)

// Result is what a successful command hands to the output layer. Text is written verbatim in
// text mode (the command owns its trailing newline); Kind and Data form the --json envelope;
// KV holds the --kv records, and a command that leaves KV nil does not support --kv. Failure,
// when set, is a failure that goes with a partial result (a multi-id command missing one id): text
// writes it to stderr after the result, --json sets it as the envelope's error next to its data,
// and --kv leaves it to the records' own STATUS lines.
type Result struct {
	Kind     string
	Data     any
	Text     string
	KV       []KVRecord
	Warnings []string
	Failure  *Error
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

// Envelope is the --json document every command writes to stdout. Data is set on success and
// Error on failure; Elsewhere and Warnings are always present, empty when there is nothing to
// report.
type Envelope struct {
	Schema    int        `json:"schema"`
	Kind      string     `json:"kind"`
	Data      any        `json:"data,omitempty"`
	Error     *ErrorBody `json:"error,omitempty"`
	Elsewhere []any      `json:"elsewhere"`
	Warnings  []string   `json:"warnings"`
}

// ErrorBody is the error member of a failure Envelope.
type ErrorBody struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
	Line    int    `json:"line,omitempty"`
}

// Renderer writes a command's outcome in one output format.
type Renderer interface {
	// Render writes a successful result.
	Render(result Result) error
	// RenderError writes a failure.
	RenderError(failure *Error) error
}

// outputFormat names the format the global flags select.
type outputFormat int

const (
	formatText outputFormat = iota
	formatJSON
	formatKV
)

// newRenderer returns the Renderer for format, writing results to stdout and, in text mode,
// failures to stderr.
func newRenderer(format outputFormat, stdout, stderr io.Writer) Renderer {
	switch format {
	case formatJSON:
		return jsonRenderer{stdout: stdout}
	case formatKV:
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
	for _, warning := range result.Warnings {
		out.WriteString(textWarnLabel + warning + "\n")
	}
	if _, err := io.WriteString(r.stdout, out.String()); err != nil {
		return err
	}
	if result.Failure == nil {
		return nil
	}
	return r.RenderError(result.Failure)
}

func (r textRenderer) RenderError(failure *Error) error {
	_, err := fmt.Fprintf(r.stderr, "capcom: %s\n", failure.Error())
	return err
}

// jsonRenderer writes one Envelope per command to stdout, failures included, so a caller parses
// stdout alone.
type jsonRenderer struct {
	stdout io.Writer
}

func (r jsonRenderer) Render(result Result) error {
	envelope := Envelope{Kind: result.Kind, Data: result.Data, Warnings: result.Warnings}
	if result.Failure != nil {
		envelope.Error = errorBody(result.Failure)
	}
	return r.write(envelope)
}

func (r jsonRenderer) RenderError(failure *Error) error {
	return r.write(Envelope{Kind: kindError, Error: errorBody(failure)})
}

// errorBody is the envelope's error member for failure.
func errorBody(failure *Error) *ErrorBody {
	return &ErrorBody{
		Code:    failure.Code,
		Message: failure.Message,
		Path:    failure.Path,
		Line:    failure.Line,
	}
}

func (r jsonRenderer) write(envelope Envelope) error {
	envelope.Schema = SchemaVersion
	if envelope.Elsewhere == nil {
		envelope.Elsewhere = []any{}
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
	writeKVLine(&out, kvKeyMessage, failure.Error())
	writeKVLine(&out, kvKeyStatus, string(failure.Code))
	_, err := io.WriteString(r.stdout, out.String())
	return err
}

// kvLineBreaks flattens a value onto one line; a KEY=VALUE consumer reads line by line.
var kvLineBreaks = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")

func writeKVLine(out *strings.Builder, key, value string) {
	out.WriteString(key + "=" + kvLineBreaks.Replace(value) + "\n")
}
