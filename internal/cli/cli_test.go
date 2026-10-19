package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom"
	"github.com/airiclenz/capcom/internal/frontmatter"
	"github.com/airiclenz/capcom/internal/store"
)

func decodeEnvelope(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout)
	}
	return envelope
}

func lastLine(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	return lines[len(lines)-1]
}

func TestRun_UnknownCommandWithJSON_WritesInvalidEnvelopeAndExitsUsage(t *testing.T) {
	stdout, stderr, exitCode := runCLI(t, t.TempDir(), "--json", "no-such-command")

	if exitCode != ExitUsage {
		t.Fatalf("exit = %d, want %d", exitCode, ExitUsage)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty in --json mode", stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope["schema"] != float64(SchemaVersion) || envelope["kind"] != kindError {
		t.Errorf("envelope header = %v/%v, want %d/%s", envelope["schema"], envelope["kind"], SchemaVersion, kindError)
	}
	errorBody, isObject := envelope["error"].(map[string]any)
	if !isObject || errorBody["code"] != string(CodeInvalid) {
		t.Fatalf("error = %v, want code %s", envelope["error"], CodeInvalid)
	}
	if !strings.Contains(errorBody["message"].(string), "no-such-command") {
		t.Errorf("message = %q, want it to name the command", errorBody["message"])
	}
}

func TestRun_UnknownCommandInText_WritesStderrOnly(t *testing.T) {
	stdout, stderr, exitCode := runCLI(t, "", "no-such-command")

	if exitCode != ExitUsage || stdout != "" {
		t.Fatalf("exit = %d stdout = %q, want %d and empty", exitCode, stdout, ExitUsage)
	}
	if !strings.HasPrefix(stderr, "capcom: ") || strings.Contains(stderr, "Did you mean") {
		t.Errorf("stderr = %q, want one capcom: line without tips", stderr)
	}
}

// unknownCommandCases are mistyped commands and subcommands, each with the suggestion it gets.
var unknownCommandCases = []struct {
	args           []string
	wantSuggestion string
}{
	{[]string{"shwo", "x"}, "show"},
	{[]string{"know", "shwo", "x"}, "know show"},
	{[]string{"know", "tre"}, "know tree"},
	{[]string{"dep", "lst", "x"}, "dep list"},
	{[]string{"related", "ad", "x", "y"}, "related add"},
	{[]string{"updat", "x", "--status", "active"}, "update"},
}

func TestRun_UnknownCommandInText_SuggestsClosest(t *testing.T) {
	for _, testCase := range unknownCommandCases {
		t.Run(strings.Join(testCase.args, " "), func(t *testing.T) {
			stdout, stderr, exitCode := runCLI(t, "", testCase.args...)

			want := "; did you mean " + testCase.wantSuggestion + "?\n"
			if exitCode != ExitUsage || stdout != "" || !strings.HasSuffix(stderr, want) {
				t.Errorf("exit = %d stdout = %q stderr = %q, want %d, no stdout and a line ending %q",
					exitCode, stdout, stderr, ExitUsage, want)
			}
		})
	}
}

func TestRun_UnknownCommandWithJSON_SuggestsClosest(t *testing.T) {
	for _, testCase := range unknownCommandCases {
		t.Run(strings.Join(testCase.args, " "), func(t *testing.T) {
			args := append([]string{"--json"}, testCase.args...)

			stdout, _, exitCode := runCLI(t, "", args...)

			errorBody, isObject := decodeEnvelope(t, stdout)["error"].(map[string]any)
			if exitCode != ExitUsage || !isObject || errorBody["suggestion"] != testCase.wantSuggestion {
				t.Errorf("exit = %d error = %v, want %d and suggestion %q", exitCode, errorBody, ExitUsage, testCase.wantSuggestion)
			}
		})
	}
}

func TestRun_UnknownCommandNothingClose_OmitsSuggestion(t *testing.T) {
	stdout, stderr, exitCode := runCLI(t, "", "zzzzqq")
	jsonOut, _, jsonExitCode := runCLI(t, "", "--json", "zzzzqq")

	if exitCode != ExitUsage || stdout != "" || stderr != "capcom: unknown command \"zzzzqq\" for \"capcom\"\n" {
		t.Errorf("exit = %d stdout = %q stderr = %q, want %d and no suggestion", exitCode, stdout, stderr, ExitUsage)
	}
	errorBody, isObject := decodeEnvelope(t, jsonOut)["error"].(map[string]any)
	if _, hasSuggestion := errorBody["suggestion"]; jsonExitCode != ExitUsage || !isObject || hasSuggestion {
		t.Errorf("exit = %d error = %v, want %d and no suggestion key", jsonExitCode, errorBody, ExitUsage)
	}
}

// unknownFlagCases are mistyped flags, each with the suggestion it gets: one of the command's own
// flags, or one it inherits from capcom.
var unknownFlagCases = []struct {
	args           []string
	wantSuggestion string
}{
	{[]string{"update", "x", "--statsu", "active"}, "--status"},
	{[]string{"ready", "--jsno"}, "--json"},
}

func TestRun_UnknownFlagInText_SuggestsClosest(t *testing.T) {
	for _, testCase := range unknownFlagCases {
		t.Run(strings.Join(testCase.args, " "), func(t *testing.T) {
			stdout, stderr, exitCode := runCLI(t, "", testCase.args...)

			want := "; did you mean " + testCase.wantSuggestion + "?\n"
			if exitCode != ExitUsage || stdout != "" || !strings.Contains(stderr, "unknown flag: --") ||
				!strings.HasSuffix(stderr, want) {
				t.Errorf("exit = %d stdout = %q stderr = %q, want %d, no stdout and an unknown flag line ending %q",
					exitCode, stdout, stderr, ExitUsage, want)
			}
		})
	}
}

func TestRun_UnknownFlagWithJSON_SuggestsClosest(t *testing.T) {
	for _, testCase := range unknownFlagCases {
		t.Run(strings.Join(testCase.args, " "), func(t *testing.T) {
			args := append([]string{"--json"}, testCase.args...)

			stdout, _, exitCode := runCLI(t, "", args...)

			errorBody, isObject := decodeEnvelope(t, stdout)["error"].(map[string]any)
			if exitCode != ExitUsage || !isObject || errorBody["suggestion"] != testCase.wantSuggestion {
				t.Errorf("exit = %d error = %v, want %d and suggestion %q", exitCode, errorBody, ExitUsage, testCase.wantSuggestion)
			}
		})
	}
}

func TestRun_UnknownFlagNothingClose_OmitsSuggestion(t *testing.T) {
	cases := map[string]string{
		"--zzzzqq": "capcom: unknown flag: --zzzzqq\n",
		"-z":       "capcom: unknown shorthand flag: 'z' in -z\n",
	}
	for flag, wantStderr := range cases {
		t.Run(flag, func(t *testing.T) {
			stdout, stderr, exitCode := runCLI(t, "", "ready", flag)
			jsonOut, _, jsonExitCode := runCLI(t, "", "--json", "ready", flag)

			if exitCode != ExitUsage || stdout != "" || stderr != wantStderr {
				t.Errorf("exit = %d stdout = %q stderr = %q, want %d and stderr %q", exitCode, stdout, stderr, ExitUsage, wantStderr)
			}
			errorBody, isObject := decodeEnvelope(t, jsonOut)["error"].(map[string]any)
			if _, hasSuggestion := errorBody["suggestion"]; jsonExitCode != ExitUsage || !isObject || hasSuggestion {
				t.Errorf("exit = %d error = %v, want %d and no suggestion key", jsonExitCode, errorBody, ExitUsage)
			}
		})
	}
}

func TestRun_GroupWithoutSubcommand_PrintsHelp(t *testing.T) {
	for _, group := range []string{"know", "dep", "lock", "related"} {
		t.Run(group, func(t *testing.T) {
			stdout, stderr, exitCode := runCLI(t, "", group)

			if exitCode != ExitOK || stderr != "" || !strings.Contains(stdout, "Usage:") {
				t.Errorf("exit = %d stderr = %q stdout = %q, want 0, no stderr and the help", exitCode, stderr, stdout)
			}
		})
	}
}

func TestRun_UsageFailures_ExitUsage(t *testing.T) {
	cases := map[string][]string{
		"unknown flag":       {"version", "--no-such-flag"},
		"unexpected arg":     {"version", "extra"},
		"json and kv":        {"version", "--json", "--kv"},
		"unknown flag in kv": {"--kv", "--no-such-flag"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, exitCode := runCLI(t, "", args...)

			if exitCode != ExitUsage {
				t.Errorf("exit = %d, want %d", exitCode, ExitUsage)
			}
		})
	}
}

func TestApp_Emit_ResultWithoutKVRecordsRejectsKV(t *testing.T) {
	var stdout bytes.Buffer
	application := &app{stdout: &stdout, isKV: true, renderer: newRenderer(formatKV, &stdout, nil)}

	err := application.emit(&cobra.Command{Use: "list"}, Result{Kind: "issue-list", Text: "x\n"})

	var failure *Error
	if !errors.As(err, &failure) || failure.ExitCode() != ExitUsage || stdout.Len() != 0 {
		t.Errorf("err = %v stdout = %q, want a usage error and no output", err, stdout.String())
	}
}

func TestRun_UsageFailureWithKV_EndsWithErrorStatus(t *testing.T) {
	stdout, _, _ := runCLI(t, "", "--kv", "no-such-command")

	if got := lastLine(stdout); got != "STATUS=invalid" {
		t.Errorf("last line = %q, want STATUS=invalid\n%s", got, stdout)
	}
}

func TestRun_NoArgs_PrintsHelp(t *testing.T) {
	stdout, stderr, exitCode := runCLI(t, "")

	if exitCode != ExitOK || stderr != "" {
		t.Fatalf("exit = %d stderr = %q, want 0 and empty", exitCode, stderr)
	}
	if !strings.Contains(stdout, "Usage:") || !strings.Contains(stdout, "version") {
		t.Errorf("stdout = %q, want root help listing version", stdout)
	}
}

func TestVersion_Text_PrintsEmbeddedVersion(t *testing.T) {
	stdout, stderr, exitCode := runCLI(t, "", "version")

	if exitCode != ExitOK || stderr != "" {
		t.Fatalf("exit = %d stderr = %q", exitCode, stderr)
	}
	if want := "capcom " + capcom.Version() + "\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestVersion_JSON_WritesVersionEnvelope(t *testing.T) {
	stdout, _, exitCode := runCLI(t, "", "version", "--json")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d", exitCode)
	}
	envelope := decodeEnvelope(t, stdout)
	data, isObject := envelope["data"].(map[string]any)
	if envelope["schema"] != float64(SchemaVersion) || envelope["kind"] != "version" || !isObject {
		t.Fatalf("envelope = %v, want schema 1 kind version with data", envelope)
	}
	if data["version"] != capcom.Version() {
		t.Errorf("data.version = %v, want %s", data["version"], capcom.Version())
	}
	if _, hasError := envelope["error"]; hasError {
		t.Errorf("success envelope carries error: %v", envelope["error"])
	}
	for _, key := range []string{"elsewhere", "warnings"} {
		if list, isList := envelope[key].([]any); !isList || len(list) != 0 {
			t.Errorf("%s = %v, want an empty list", key, envelope[key])
		}
	}
}

func TestVersion_KV_EndsWithStatusOK(t *testing.T) {
	stdout, _, exitCode := runCLI(t, "", "version", "--kv")

	if exitCode != ExitOK {
		t.Fatalf("exit = %d", exitCode)
	}
	if want := "VERSION=" + capcom.Version() + "\nSTATUS=ok\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestUpdate_FrontmatterFailure_RelativizesOnlyAnAbsolutePath(t *testing.T) {
	root := t.TempDir()
	application := &app{store: &store.Store{WorktreeRoot: root}}
	tests := map[string]struct {
		path string
		want string
	}{
		"absolute path below the worktree": {
			path: filepath.Join(root, ".capcom", "issues", "a.md"),
			want: ".capcom/issues/a.md",
		},
		"relative index path": {
			path: ".capcom/issues/a.md",
			want: ".capcom/issues/a.md",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			parseFailure := &frontmatter.Error{Path: test.path, Line: 3, Message: "unknown key"}

			err := application.frontmatterFailure(fmt.Errorf("parse: %w", parseFailure))

			var failure *Error
			if !errors.As(err, &failure) {
				t.Fatalf("err = %v, want an *Error", err)
			}
			if failure.Code != CodeInvalid || failure.Path != test.want || failure.Line != 3 {
				t.Errorf("failure = %+v, want invalid at %s line 3", failure, test.want)
			}
		})
	}
}

func TestStoreCommands_OutsideAStore_FailAsBefore(t *testing.T) {
	commands := [][]string{{"agents"}, {"fmt"}, {"lock", "status"}}
	badConfig := "statuses:\n  open: todo\n  review: reviewing\n  done: done\n"
	tests := []struct {
		name     string
		setup    func(t *testing.T) string
		wantFail func(dir string) string
	}{
		{
			name:  "outside git",
			setup: outsideGitDir,
			wantFail: func(dir string) string {
				return "capcom: " + dir + ": " + store.ErrNotGitRepository.Error() + "\n"
			},
		},
		{
			name:  "no store",
			setup: newGitRepo,
			wantFail: func(root string) string {
				return "capcom: " + filepath.Join(root, ".capcom") + ": " + store.ErrNotInitialized.Error() + "\n"
			},
		},
		{
			name: "bad config",
			setup: func(t *testing.T) string {
				root := newGitRepo(t)
				runCLI(t, root, "init")
				if err := os.WriteFile(filepath.Join(root, ".capcom", "config.yaml"), []byte(badConfig), 0o644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
				return root
			},
			wantFail: func(string) string {
				return "capcom: .capcom/config.yaml:3: status \"review\": category must be one of [todo doing waiting done dropped]\n"
			},
		},
	}
	for _, test := range tests {
		for _, args := range commands {
			t.Run(test.name+"/"+strings.Join(args, " "), func(t *testing.T) {
				dir := test.setup(t)

				stdout, stderr, exitCode := runCLI(t, dir, args...)

				if want := test.wantFail(dir); exitCode != ExitError || stdout != "" || stderr != want {
					t.Errorf("exit = %d stdout = %q stderr = %q, want %d, no stdout and %q", exitCode, stdout, stderr, ExitError, want)
				}
			})
		}
	}

	t.Run("lock acquire without --purpose outside a store", func(t *testing.T) {
		dir := outsideGitDir(t)

		stdout, stderr, exitCode := runCLI(t, dir, "lock", "acquire")

		want := "capcom: required flag(s) \"" + flagPurpose + "\" not set\n"
		if exitCode != ExitUsage || stdout != "" || stderr != want {
			t.Errorf("exit = %d stdout = %q stderr = %q, want %d, no stdout and %q", exitCode, stdout, stderr, ExitUsage, want)
		}
	})
}
