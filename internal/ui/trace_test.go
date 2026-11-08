package ui

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/store"
)

// newTestTracer returns a tracer over a fresh file in a temp dir, and the file's path. The tracer
// is closed when the test ends, before the temp dir is removed.
func newTestTracer(t *testing.T) (*tracer, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "trace.log")
	trace, err := openTracer(path)
	if err != nil {
		t.Fatalf("openTracer: %v", err)
	}
	t.Cleanup(func() {
		if err := trace.close(); err != nil {
			t.Errorf("close the tracer: %v", err)
		}
	})
	return trace, path
}

// withTracer returns m with trace set on it and its session, as Run sets it.
func withTracer(m model, trace *tracer) model {
	m.tracer = trace
	m.session.tracer = trace
	return m
}

// traceLines returns the lines of the trace file at path.
func traceLines(t *testing.T, path string) []string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the trace: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
}

// tracedEvents returns the lines of the trace at path whose event is event, each without its
// time.
func tracedEvents(t *testing.T, path, event string) []string {
	t.Helper()
	var found []string
	for _, line := range traceLines(t, path) {
		_, rest, _ := strings.Cut(line, " ")
		if name, _, _ := strings.Cut(rest, " "); name == event {
			found = append(found, rest)
		}
	}
	return found
}

func TestTracer_Write_TimeEventAndFields(t *testing.T) {
	trace, path := newTestTracer(t)
	trace.now = func() time.Time { return time.Date(2026, 11, 8, 13, 4, 5, 678e6, time.FixedZone("x", 3600)) }

	trace.write(traceWatch, traceField{"id", "alpha"}, traceField{"notice", "reading alpha: waiting 2s"})

	want := []string{`2026-11-08T12:04:05.678Z watch id=alpha notice="reading alpha: waiting 2s"`}
	if got := traceLines(t, path); !slices.Equal(got, want) {
		t.Errorf("trace = %q, want %q", got, want)
	}
}

func TestTrace_TabSwitch_TracesTheReadRequestedAppliedAndRendered(t *testing.T) {
	trace, path := newTestTracer(t)
	m := withTracer(newTestModel(t, newTestStore(t, pendingFiles), 120, 30), trace)

	press(t, m, runeKey('1'))

	for _, want := range []string{
		"requested id=alpha note=false seq=",
		"applied id=alpha note=false seq=",
		"rendered id=alpha lines=",
	} {
		event, _, _ := strings.Cut(want, " ")
		lines := tracedEvents(t, path, event)
		if !slices.ContainsFunc(lines, func(line string) bool { return strings.HasPrefix(line, want) }) {
			t.Errorf("%s lines = %q, want one starting %q", event, lines, want)
		}
	}
}

func TestTrace_StaleRead_TracesItSuperseded(t *testing.T) {
	ts := newTestStore(t, pendingFiles)
	trace, path := newTestTracer(t)
	m := withTracer(newIssuesModel(t, ts, "alpha", 120, 30), trace)
	m, olderRead := reloadIssues(t, m)
	older := olderRead()
	m, _ = reloadIssues(t, m)

	send(t, m, older)

	lines := tracedEvents(t, path, traceStale)
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "stale id=alpha") ||
		!strings.HasSuffix(lines[0], " reason="+staleSuperseded) {
		t.Errorf("stale lines = %q, want one for alpha with reason %s", lines, staleSuperseded)
	}
}

func TestTrace_ViewTwiceOverTheSamePane_TracesOneDrawnLine(t *testing.T) {
	ts := newTestStore(t, pendingFiles)
	trace, path := newTestTracer(t)
	m := withTracer(newIssuesModel(t, ts, "alpha", 120, 30), trace)

	m.View()
	m.View()

	lines := tracedEvents(t, path, traceDrawn)
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "drawn id=alpha text_rows=") {
		t.Errorf("drawn lines = %q, want one for alpha", lines)
	}
}

func TestTrace_DividerMoved_TracesAnotherDrawnLine(t *testing.T) {
	ts := newTestStore(t, pendingFiles)
	trace, path := newTestTracer(t)
	m := withTracer(newIssuesModel(t, ts, "alpha", 120, 30), trace)
	m.View()

	m = press(t, m, runeKey('>'))
	m.View()

	lines := tracedEvents(t, path, traceDrawn)
	if len(lines) != 2 || lines[0] == lines[1] {
		t.Errorf("drawn lines = %q, want two that differ", lines)
	}
}

func TestTrace_PollWaitsBehindAHeldAccess_TracesTheWait(t *testing.T) {
	ts := newTestStore(t, nil)
	trace, path := newTestTracer(t)
	s := newSession(ts.store, ts.index)
	s.tracer = trace
	free := holdSession(t, s, purposeWrite)
	polled := make(chan struct{})

	go func() {
		s.poll()
		close(polled)
	}()
	time.Sleep(2 * traceWaitThreshold)
	free()
	<-polled

	lines := tracedEvents(t, path, traceWait)
	if len(lines) != 1 || !strings.HasPrefix(lines[0], `wait purpose="the index poll" waited=`) ||
		!strings.HasSuffix(lines[0], ` holder="a write"`) {
		t.Errorf("wait lines = %q, want the poll's wait behind the write", lines)
	}
}

func TestTrace_ShortWait_TracesNothing(t *testing.T) {
	ts := newTestStore(t, nil)
	trace, path := newTestTracer(t)
	s := newSession(ts.store, ts.index)
	s.tracer = trace

	err := s.doFor(purposeWrite, func(*store.Store, *index.Index) error { return nil })

	if err != nil {
		t.Fatalf("doFor: %v", err)
	}
	if content, _ := os.ReadFile(path); len(content) != 0 {
		t.Errorf("trace = %q, want it empty", content)
	}
}

func TestTracer_WriteAfterClose_IsDropped(t *testing.T) {
	trace, path := newTestTracer(t)
	if err := trace.close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	trace.write(traceRequested, traceField{"id", "alpha"})
	trace.drawn(traceField{"id", "alpha"})

	if content, _ := os.ReadFile(path); len(content) != 0 {
		t.Errorf("trace = %q, want it empty", content)
	}
}

func TestTracer_NoPath_IsANilTracerThatWritesNothing(t *testing.T) {
	trace, err := openTracer("")

	trace.write(traceRequested, traceField{"id", "alpha"})
	trace.drawn(traceField{"id", "alpha"})

	if err != nil || trace != nil {
		t.Fatalf("openTracer(\"\") = %v, %v; want nil, nil", trace, err)
	}
	if err := trace.close(); err != nil {
		t.Errorf("close of a nil tracer = %v, want nil", err)
	}
}

func TestRun_TracePathCannotBeOpened_FailsBeforeTheModel(t *testing.T) {
	ts := newTestStore(t, nil)
	options := newTestOptions(t, ts)
	options.TracePath = filepath.Join(t.TempDir(), "missing", "trace.log")

	err := Run(options)

	if err == nil {
		t.Fatal("Run succeeded, want it to fail on the trace path")
	}
	text := err.Error()
	if !strings.HasPrefix(text, TraceVariable+": "+options.TracePath+": ") ||
		strings.Count(text, options.TracePath) != 1 || !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Run error = %q, want %s: <path>: <error> naming the path once", text, TraceVariable)
	}
	if _, statErr := os.Stat(options.UserConfigPath); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("user config stat = %v, want it never created: newModel ran", statErr)
	}
}
