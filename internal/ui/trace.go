package ui

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TraceVariable names the environment variable that, set to a path, makes capcom ui append a
// trace of its detail reads to that file.
const TraceVariable = "CAPCOM_UI_TRACE"

// The trace file's shape: one line per event, its time in UTC to the millisecond, the event, then
// its fields as key=value.
const (
	// traceTimeLayout is RFC 3339 with milliseconds.
	traceTimeLayout = "2006-01-02T15:04:05.000Z07:00"
	// traceFileMode keeps the trace, which names issue ids, to its owner.
	traceFileMode fs.FileMode = 0o600
	// traceWaitThreshold is how long an access waits on the session's mutex before the wait is
	// traced.
	traceWaitThreshold = 100 * time.Millisecond
)

// The events a trace line names.
const (
	// traceRequested is a detail read asked for.
	traceRequested = "requested"
	// traceApplied is a detail read's answer shown in the pane.
	traceApplied = "applied"
	// traceStale is a detail read's answer dropped, with the reason.
	traceStale = "stale"
	// traceRendered is the shown issue or Note rendered, with its line count.
	traceRendered = "rendered"
	// traceWatch is the status bar's report of a detail read that waits.
	traceWatch = "watch"
	// traceWait is an access that waited on the session's mutex past traceWaitThreshold.
	traceWait = "wait"
	// traceDrawn is the detail pane View draws, traced only when it differs from the last one.
	traceDrawn = "drawn"
)

// traceField is one key=value of a trace line.
type traceField struct {
	key   string
	value any
}

// tracer appends the trace lines to the file CAPCOM_UI_TRACE names. Lines come from Update and
// from tea.Cmd goroutines, so each is formatted and written whole under the tracer's mutex. A nil
// tracer, the one a model and a session start with, writes nothing, and a closed one drops every
// line. A failed write is kept for close to return and drops every later line.
type tracer struct {
	mu  sync.Mutex
	out io.WriteCloser
	now func() time.Time
	// lastDrawn is the formatted fields of the last drawn line, kept here because View has a
	// value receiver and cannot keep it on the model.
	lastDrawn string
	isClosed  bool
	writeErr  error
}

// openTracer opens the trace file at path for appending, creating it with traceFileMode when it
// is missing, and returns its tracer. An empty path is no trace: a nil tracer and no error. A
// path that cannot be opened fails with `CAPCOM_UI_TRACE: <path>: <error>`, the path named once.
func openTracer(path string) (*tracer, error) {
	if path == "" {
		return nil, nil
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, traceFileMode)
	if err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			err = pathErr.Err
		}
		return nil, fmt.Errorf("%s: %s: %w", TraceVariable, path, err)
	}
	return &tracer{out: file, now: time.Now}, nil
}

// write appends the line of event with fields, in their order.
func (t *tracer) write(event string, fields ...traceField) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.writeLocked(event, traceFields(fields))
}

// drawn appends a drawn line with fields, unless they are the last drawn line's fields.
func (t *tracer) drawn(fields ...traceField) {
	if t == nil {
		return
	}
	formatted := traceFields(fields)
	t.mu.Lock()
	defer t.mu.Unlock()
	if formatted == t.lastDrawn {
		return
	}
	t.lastDrawn = formatted
	t.writeLocked(traceDrawn, formatted)
}

// writeLocked writes one line: the time, event, then the formatted fields. The caller holds the
// mutex.
func (t *tracer) writeLocked(event, formatted string) {
	if t.isClosed || t.writeErr != nil {
		return
	}
	line := t.now().UTC().Format(traceTimeLayout) + " " + event + formatted + "\n"
	_, t.writeErr = io.WriteString(t.out, line)
}

// traceFields returns fields as a trace line ends: each as ` key=value`, in their order.
func traceFields(fields []traceField) string {
	var text strings.Builder
	for _, field := range fields {
		fmt.Fprintf(&text, " %s=%s", field.key, traceValue(field.value))
	}
	return text.String()
}

// traceValue returns value as a trace line shows it: quoted when it is empty or holds a space, a
// quote or an equals sign, so every line splits back into its fields.
func traceValue(value any) string {
	text := fmt.Sprint(value)
	if text == "" || strings.ContainsAny(text, " \t\n\"=") {
		return strconv.Quote(text)
	}
	return text
}

// close closes the trace file; every line written after it is dropped. It returns the first
// write that failed, then the close's error. Closing a nil or a closed tracer does nothing.
func (t *tracer) close() error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.isClosed {
		return nil
	}
	t.isClosed = true
	return errors.Join(t.writeErr, t.out.Close())
}
