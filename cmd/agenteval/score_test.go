package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// loggedCall is a call log line for a call that exited with exitCode, with args as its arguments.
func loggedCall(exitCode int, args ...string) shimCall {
	return shimCall{Args: args, Exit: exitCode}
}

func TestScore_CountsCallsFailuresAndRepeatedFailures(t *testing.T) {
	tests := []struct {
		name  string
		calls []shimCall
		want  taskScore
	}{
		{name: "no calls", calls: nil, want: taskScore{}},
		{
			name:  "all ok",
			calls: []shimCall{loggedCall(0, "ready"), loggedCall(0, "show", "a"), loggedCall(0, "ready")},
			want:  taskScore{Calls: 3},
		},
		{
			name: "mixed",
			calls: []shimCall{
				loggedCall(0, "ready"), loggedCall(3, "show", "a"), loggedCall(2, "shwo"), loggedCall(0, "show", "b"),
			},
			want: taskScore{Calls: 4, Failed: 2},
		},
		{
			name:  "two identical failures",
			calls: []shimCall{loggedCall(3, "show", "a"), loggedCall(0, "ready"), loggedCall(3, "show", "a")},
			want:  taskScore{Calls: 3, Failed: 2, RepeatedFailures: 1},
		},
		{
			name:  "same args, different exit",
			calls: []shimCall{loggedCall(3, "show", "a"), loggedCall(1, "show", "a")},
			want:  taskScore{Calls: 2, Failed: 2},
		},
		{
			name:  "args that only join alike",
			calls: []shimCall{loggedCall(1, "show a"), loggedCall(1, "show", "a")},
			want:  taskScore{Calls: 2, Failed: 2},
		},
		{
			name:  "three identical failures",
			calls: []shimCall{loggedCall(1, "close"), loggedCall(1, "close"), loggedCall(1, "close")},
			want:  taskScore{Calls: 3, Failed: 3, RepeatedFailures: 2},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := score(test.calls)

			if got != test.want {
				t.Errorf("score() = %+v, want %+v", got, test.want)
			}
		})
	}
}

// writeCallLog writes content as a call log in a fresh directory and returns its path.
func writeCallLog(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "calls.jsonl")
	if err := os.WriteFile(path, []byte(content), callLogFileMode); err != nil {
		t.Fatalf("write the call log: %v", err)
	}
	return path
}

func TestReadCallLog_ReadsEachLineInOrder(t *testing.T) {
	path := writeCallLog(t, `{"args":["ready"],"exit":0,"ms":12}`+"\n\n"+`{"args":["show","a"],"exit":3,"ms":7}`+"\n")

	calls, err := readCallLog(path)

	if err != nil {
		t.Fatalf("readCallLog() error = %v", err)
	}
	if len(calls) != 2 || !slices.Equal(calls[1].Args, []string{"show", "a"}) || calls[1].Exit != 3 ||
		calls[0].Milliseconds != 12 {
		t.Errorf("readCallLog() = %+v, want ready (12 ms) then show a (exit 3)", calls)
	}
}

func TestReadCallLog_MissingLogHasNoCalls(t *testing.T) {
	calls, err := readCallLog(filepath.Join(t.TempDir(), "calls.jsonl"))

	if err != nil || len(calls) != 0 {
		t.Errorf("readCallLog() = %+v, %v, want no calls and no error", calls, err)
	}
}

func TestReadCallLog_MalformedLineNamesTheFileAndLine(t *testing.T) {
	path := writeCallLog(t, `{"args":["ready"],"exit":0,"ms":1}`+"\n"+`{"args":["show"`+"\n")

	_, err := readCallLog(path)

	if err == nil || !strings.HasPrefix(err.Error(), path+":2: ") {
		t.Errorf("readCallLog() error = %v, want it to start with %q", err, path+":2: ")
	}
}

func TestReadCallLog_UnreadableLogIsAnError(t *testing.T) {
	_, err := readCallLog(t.TempDir())

	if err == nil {
		t.Error("readCallLog(a directory) error = nil, want one")
	}
}
