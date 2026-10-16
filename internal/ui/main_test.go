package ui

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// The environment of the test binary run as the fake editor.
const (
	// fakeEditorVariable, set to any value, makes the test binary run as the fake editor in place
	// of the tests.
	fakeEditorVariable = "CAPCOM_UI_FAKE_EDITOR"
	// fakeEditOldVariable and fakeEditNewVariable are the text the fake editor replaces in the
	// file it is handed, and the text it puts in its place.
	fakeEditOldVariable = "CAPCOM_UI_FAKE_EDIT_OLD"
	fakeEditNewVariable = "CAPCOM_UI_FAKE_EDIT_NEW"
	// fakeEditorArgsVariable names the file the fake editor records its arguments in, one a line.
	fakeEditorArgsVariable = "CAPCOM_UI_FAKE_EDITOR_ARGS"
)

// fakeEditorMissingCode is the fake editor's exit code when the file it is handed holds no old
// text, and fakeEditorFailureCode its exit code when it cannot read or write a file.
const (
	fakeEditorMissingCode = 2
	fakeEditorFailureCode = 3
)

// TestMain runs the tests, or, re-executed by fakeEditor with fakeEditorVariable set, the fake
// editor, so the fake works the same on every OS.
func TestMain(m *testing.M) {
	if os.Getenv(fakeEditorVariable) != "" {
		os.Exit(runFakeEditor(os.Args[1:]))
	}
	os.Exit(m.Run())
}

// runFakeEditor edits the file named by the last of args, as an editor handed it would: it
// replaces the first fakeEditOldVariable text with fakeEditNewVariable's, after recording args
// in the file fakeEditorArgsVariable names. It returns the process exit code.
func runFakeEditor(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "fake editor: no file")
		return fakeEditorFailureCode
	}
	if argsPath := os.Getenv(fakeEditorArgsVariable); argsPath != "" {
		if err := os.WriteFile(argsPath, []byte(strings.Join(args, "\n")), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "fake editor:", err)
			return fakeEditorFailureCode
		}
	}
	path := args[len(args)-1]
	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fake editor:", err)
		return fakeEditorFailureCode
	}
	old := os.Getenv(fakeEditOldVariable)
	if !strings.Contains(string(content), old) {
		fmt.Fprintf(os.Stderr, "fake editor: %s holds no %q\n", path, old)
		return fakeEditorMissingCode
	}
	edited := strings.Replace(string(content), old, os.Getenv(fakeEditNewVariable), 1)
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "fake editor:", err)
		return fakeEditorFailureCode
	}
	return 0
}
