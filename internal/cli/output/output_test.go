package output

import (
	"bytes"
	"encoding/json"
	"testing"
)

func decodeEnvelope(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout)
	}
	return envelope
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

	err := NewRenderer(FormatKV, &stdout, nil).Render(result)

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

	err := NewRenderer(FormatText, &stdout, &stderr).Render(Result{Text: "done\n", Warnings: []string{"a.md:3: bad"}})

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

	err := NewRenderer(FormatJSON, &stdout, nil).RenderError(failure)

	if err != nil {
		t.Fatalf("RenderError: %v", err)
	}
	errorBody := decodeEnvelope(t, stdout.String())["error"].(map[string]any)
	if errorBody["path"] != "issue.md" || errorBody["line"] != float64(4) {
		t.Errorf("error = %v, want path issue.md line 4", errorBody)
	}
}

func TestSanitizeTerminal_EscapesControlsOnly(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"escape", "a\x1b[2Jb", `a\x1b[2Jb`},
		{"bell", "a\ab", `a\x07b`},
		{"carriage return", "a\rb", `a\x0db`},
		{"delete", "a\x7fb", `a\x7fb`},
		{"C1 control U+009B", "a\u009bb", `a\x9bb`},
		{"lone byte 0x9b", "a\x9bb", `a\x9bb`},
		{"tab and line feed", "a\tb\nc", "a\tb\nc"},
		{"accented letter", "café", "café"},
		{"plain text", "Login flow", "Login flow"},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			got := sanitizeTerminal(current.input)

			if got != current.want {
				t.Errorf("sanitizeTerminal(%q) = %q, want %q", current.input, got, current.want)
			}
		})
	}
}
