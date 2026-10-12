package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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

func TestKVRenderer_Render_ClosesEveryRecordWithStatus(t *testing.T) {
	var stdout bytes.Buffer
	result := Result{
		Warnings: []string{"index rebuilt"},
		KV: []KVRecord{
			{Pairs: []KVPair{{Key: "ID", Value: "first-issue"}, {Key: "TITLE", Value: "two\nlines"}}},
			{Pairs: []KVPair{{Key: "ID", Value: "missing-issue"}}, Status: string(CodeNotFound)},
		},
	}

	err := newRenderer(formatKV, &stdout, nil).Render(result)

	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "WARNING=index rebuilt\n" +
		"ID=first-issue\nTITLE=two lines\nSTATUS=ok\n" +
		"ID=missing-issue\nSTATUS=not_found\n"
	if stdout.String() != want {
		t.Errorf("output = %q, want %q", stdout.String(), want)
	}
}

func TestTextRenderer_Render_WritesWarningsAfterResult(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := newRenderer(formatText, &stdout, &stderr).Render(Result{Text: "done\n", Warnings: []string{"a.md:3: bad"}})

	if err != nil || stderr.Len() != 0 {
		t.Fatalf("err = %v stderr = %q", err, stderr.String())
	}
	if want := "done\nwarning: a.md:3: bad\n"; stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestJSONRenderer_RenderError_CarriesLocation(t *testing.T) {
	var stdout bytes.Buffer
	failure := &Error{Code: CodeInvalid, Message: "unknown key", Path: "issue.md", Line: 4}

	err := newRenderer(formatJSON, &stdout, nil).RenderError(failure)

	if err != nil {
		t.Fatalf("RenderError: %v", err)
	}
	errorBody := decodeEnvelope(t, stdout.String())["error"].(map[string]any)
	if errorBody["path"] != "issue.md" || errorBody["line"] != float64(4) {
		t.Errorf("error = %v, want path issue.md line 4", errorBody)
	}
}

func TestError_Error_PrefixesKnownLocation(t *testing.T) {
	cases := []struct {
		name    string
		failure Error
		want    string
	}{
		{"no location", Error{Message: "boom"}, "boom"},
		{"path only", Error{Message: "boom", Path: "a.md"}, "a.md: boom"},
		{"path and line", Error{Message: "boom", Path: "a.md", Line: 7}, "a.md:7: boom"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := testCase.failure.Error(); got != testCase.want {
				t.Errorf("Error() = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestError_ExitCode_MapsCodes(t *testing.T) {
	cases := []struct {
		failure *Error
		want    int
	}{
		{Errorf(CodeNotFound, "x"), ExitNotFound},
		{Errorf(CodeLocked, "x"), ExitNotFound},
		{Errorf(CodeExists, "x"), ExitError},
		{Errorf(CodeInvalid, "x"), ExitError},
		{Errorf(CodeConflict, "x"), ExitError},
		{Errorf(CodeNotInitialized, "x"), ExitError},
		{UsageErrorf("x"), ExitUsage},
	}
	for _, testCase := range cases {
		t.Run(string(testCase.failure.Code), func(t *testing.T) {
			t.Parallel()

			if got := testCase.failure.ExitCode(); got != testCase.want {
				t.Errorf("ExitCode() = %d, want %d", got, testCase.want)
			}
		})
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
