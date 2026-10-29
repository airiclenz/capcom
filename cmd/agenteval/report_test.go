package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// reportRun is the run header the report tests render.
var reportRun = runInfo{
	Date:          time.Date(2026, time.October, 29, 14, 5, 0, 0, time.UTC),
	CapcomVersion: "capcom 0.12.1\n",
	AgentArgv:     []string{"agenteval", "agent", "--model", "qwen 30b", "--base-url", "http://localhost:8080"},
}

func TestRenderReport_RendersTheHeaderTableAndTotals(t *testing.T) {
	results := []taskResult{
		{
			ID:       "file-issue",
			Passed:   true,
			Score:    taskScore{Calls: 4, Failed: 1},
			Duration: 12340 * time.Millisecond,
		},
		{
			ID:       "find-note",
			Passed:   false,
			Why:      "no Note matched | wanted `tree-lock`\nsecond line",
			Score:    taskScore{Calls: 7, Failed: 3, RepeatedFailures: 2},
			Duration: 61 * time.Second,
		},
	}
	var report bytes.Buffer

	err := renderReport(&report, reportRun, results)

	if err != nil {
		t.Fatalf("renderReport() error = %v", err)
	}
	want := "# capcom agent eval\n" +
		"\n" +
		"- Date: 2026-10-29\n" +
		"- capcom: capcom 0.12.1\n" +
		"- Agent: `agenteval agent --model \"qwen 30b\" --base-url http://localhost:8080`\n" +
		"\n" +
		"| Task | Result | Calls | Failed | Repeated failures | Time | Why |\n" +
		"| --- | --- | ---: | ---: | ---: | ---: | --- |\n" +
		"| file-issue | pass | 4 | 1 | 0 | 12.3s |  |\n" +
		"| find-note | fail | 7 | 3 | 2 | 1m1s | no Note matched \\| wanted `tree-lock` second line |\n" +
		"| **Total** | 1/2 passed | 11 | 4 | 2 | 1m13.3s |  |\n"
	if got := report.String(); got != want {
		t.Errorf("renderReport() =\n%s\nwant\n%s", got, want)
	}
}

func TestRenderReport_NoTasksRendersAnEmptyTotal(t *testing.T) {
	var report bytes.Buffer

	err := renderReport(&report, reportRun, nil)

	if err != nil {
		t.Fatalf("renderReport() error = %v", err)
	}
	if want := "| **Total** | 0/0 passed | 0 | 0 | 0 | 0s |  |\n"; !strings.HasSuffix(report.String(), want) {
		t.Errorf("renderReport() =\n%s\nwant it to end with\n%s", report.String(), want)
	}
}

// failingWriter is a writer whose every write fails with err.
type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestRenderReport_ReturnsTheWriteError(t *testing.T) {
	writeFailure := errors.New("disk full")

	err := renderReport(failingWriter{err: writeFailure}, reportRun, nil)

	if !errors.Is(err, writeFailure) {
		t.Errorf("renderReport() error = %v, want %v", err, writeFailure)
	}
}

func TestRenderReport_QuotesOnlyTheAgentArgumentsThatNeedIt(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want string
	}{
		{name: "plain", argv: []string{"claude", "-p"}, want: "claude -p"},
		{name: "space", argv: []string{"sh", "-c", "cat | agent"}, want: `sh -c "cat | agent"`},
		{name: "empty", argv: []string{"agent", ""}, want: `agent ""`},
		{name: "quote", argv: []string{`it's`}, want: `"it's"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info := reportRun
			info.AgentArgv = test.argv
			var report bytes.Buffer

			err := renderReport(&report, info, nil)

			if want := "\n- Agent: `" + test.want + "`\n"; err != nil || !strings.Contains(report.String(), want) {
				t.Errorf("renderReport() = %q, %v, want it to hold %q", report.String(), err, want)
			}
		})
	}
}
