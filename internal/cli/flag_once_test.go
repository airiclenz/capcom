package cli

import (
	"strings"
	"testing"
)

func TestRepeatedFlag_SingleValueOnAWrite_IsRefusedAndWritesNothing(t *testing.T) {
	cases := []struct {
		name     string
		readFile func(t *testing.T, root string, name string) string
		file     string
		args     []string
		flagName string
	}{
		{"update title", readStoreFile, "login-flow.md",
			[]string{"update", "login-flow", "--title", "T1", "--title", "T2"}, "title"},
		{"update status", readStoreFile, "login-flow.md",
			[]string{"update", "login-flow", "--status", "active", "--status", "parked"}, "status"},
		{"update priority", readStoreFile, "login-flow.md",
			[]string{"update", "login-flow", "--priority", "1", "--priority", "2"}, "priority"},
		{"update plan", readStoreFile, "login-flow.md",
			[]string{"update", "login-flow", "--plan", "first", "--plan", "second"}, "plan"},
		{"update standing", readStoreFile, "login-flow.md",
			[]string{"update", "login-flow", "--standing", "--standing"}, "standing"},
		{"know update title", readNoteFile, "release-flow.md",
			[]string{"know", "update", "release-flow", "--title", "T1", "--title", "T2"}, "title"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newWriteStore(t)
			if testCase.args[0] == "know" {
				root = newKnowStore(t)
			}
			before := testCase.readFile(t, root, testCase.file)

			_, stderr, exitCode := runCLI(t, root, testCase.args...)

			if exitCode != ExitUsage {
				t.Fatalf("exit = %d, want %d; stderr = %q", exitCode, ExitUsage, stderr)
			}
			want := "--" + testCase.flagName + " given 2 times; give it once"
			if !strings.Contains(stderr, want) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, want)
			}
			if after := testCase.readFile(t, root, testCase.file); after != before {
				t.Errorf("file changed:\nbefore = %q\nafter  = %q", before, after)
			}
		})
	}
}

func TestRepeatedFlag_SingleValueOnARead_ExitsUsage(t *testing.T) {
	root := newWriteStore(t)

	_, stderr, exitCode := runCLI(t, root, "ready", "--limit", "1", "--limit", "2")

	if exitCode != ExitUsage || !strings.Contains(stderr, "--limit given 2 times; give it once") {
		t.Errorf("exit = %d stderr = %q, want %d naming --limit", exitCode, stderr, ExitUsage)
	}
}

func TestRepeatedFlag_JSONGivenTwice_RefusedInAJSONEnvelope(t *testing.T) {
	root := newWriteStore(t)

	stdout, _, exitCode := runCLI(t, root, "show", "login-flow", "--json", "--json")

	if exitCode != ExitUsage {
		t.Fatalf("exit = %d, want %d", exitCode, ExitUsage)
	}
	envelope := decodeEnvelope(t, stdout)
	errorBody, isObject := envelope["error"].(map[string]any)
	if !isObject || errorBody["code"] != string(CodeInvalid) {
		t.Fatalf("error = %v, want code %s", envelope["error"], CodeInvalid)
	}
	if message, _ := errorBody["message"].(string); !strings.Contains(message, "--json given 2 times") {
		t.Errorf("message = %q, want it to name --json", message)
	}
}

func TestRepeatedFlag_SliceAndArrayFlags_StillAccumulate(t *testing.T) {
	cases := []struct {
		name  string
		store func(t *testing.T) string
		args  []string
	}{
		{"list label", newWriteStore, []string{"list", "--label", "a", "--label", "b"}},
		{"know update add-ref", newKnowStore,
			[]string{"know", "update", "release-flow", "--add-ref", "go.mod", "--add-ref", "go.mod#module"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := testCase.store(t)

			mustRun(t, root, testCase.args...)
		})
	}
}
