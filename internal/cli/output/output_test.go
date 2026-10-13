package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/airiclenz/capcom/internal/elsewhere"
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

// controlInput carries ESC (a screen clear), a lone byte 0x9b, the C1 control U+009B and BEL,
// then the TAB and LF that text output keeps.
const (
	controlInput   = "a\x1b[2Jb\x9bc\u009bd\x07e\tf\ng"
	controlEscaped = `a\x1b[2Jb\x9bc\x9bd\x07e` + "\tf\ng"
)

func TestTextRenderer_ControlCharacters_EscapedInEveryStream(t *testing.T) {
	cases := []struct {
		name       string
		result     Result
		wantStdout string
		wantStderr string
	}{
		{
			name:       "text",
			result:     Result{Text: controlInput + "\n"},
			wantStdout: controlEscaped + "\n",
		},
		{
			name:       "elsewhere entry",
			result:     Result{Elsewhere: []elsewhere.Entry{{ID: "login", Branch: controlInput, Change: elsewhere.Added}}},
			wantStdout: textElsewhereLabel + "login: added on " + controlEscaped + " (unmerged)\n",
		},
		{
			name:       "warning",
			result:     Result{Warnings: []string{controlInput}},
			wantStdout: textWarnLabel + controlEscaped + "\n",
		},
		{
			name:       "failure",
			result:     Result{Failure: &Error{Code: CodeNotFound, Message: controlInput}},
			wantStderr: "capcom: " + controlEscaped + "\n",
		},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			err := NewRenderer(FormatText, &stdout, &stderr).Render(current.result)

			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if stdout.String() != current.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout.String(), current.wantStdout)
			}
			if stderr.String() != current.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr.String(), current.wantStderr)
			}
		})
	}

	t.Run("RenderError", func(t *testing.T) {
		var stderr bytes.Buffer

		err := NewRenderer(FormatText, nil, &stderr).RenderError(&Error{Code: CodeInvalid, Message: controlInput})

		if err != nil {
			t.Fatalf("RenderError: %v", err)
		}
		if want := "capcom: " + controlEscaped + "\n"; stderr.String() != want {
			t.Errorf("stderr = %q, want %q", stderr.String(), want)
		}
	})
}

func TestJSONAndKVRenderers_ControlCharacters_PassedVerbatim(t *testing.T) {
	// rawValue is valid UTF-8, so the JSON round trip can carry it unchanged; it has no LF, which
	// --kv folds onto one line.
	const rawValue = "a\x1b[2Jb\u009bc\x07d\te"
	result := Result{
		Kind:      "issue",
		Data:      map[string]string{"title": rawValue},
		Text:      rawValue,
		KV:        []KVRecord{{Pairs: []KVPair{{Key: "TITLE", Value: rawValue}}}},
		Elsewhere: []elsewhere.Entry{{ID: "login", Branch: rawValue, Change: elsewhere.Added, Path: "login.md"}},
		Warnings:  []string{rawValue},
	}
	failure := &Error{Code: CodeInvalid, Message: rawValue}

	t.Run("json result", func(t *testing.T) {
		var stdout bytes.Buffer

		err := NewRenderer(FormatJSON, &stdout, nil).Render(result)

		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		envelope := decodeEnvelope(t, stdout.String())
		if title := envelope["data"].(map[string]any)["title"]; title != rawValue {
			t.Errorf("data.title = %q, want %q", title, rawValue)
		}
		if branch := envelope["elsewhere"].([]any)[0].(map[string]any)["branch"]; branch != rawValue {
			t.Errorf("elsewhere[0].branch = %q, want %q", branch, rawValue)
		}
		if warning := envelope["warnings"].([]any)[0]; warning != rawValue {
			t.Errorf("warnings[0] = %q, want %q", warning, rawValue)
		}
	})

	t.Run("json failure", func(t *testing.T) {
		var stdout bytes.Buffer

		err := NewRenderer(FormatJSON, &stdout, nil).RenderError(failure)

		if err != nil {
			t.Fatalf("RenderError: %v", err)
		}
		if message := decodeEnvelope(t, stdout.String())["error"].(map[string]any)["message"]; message != rawValue {
			t.Errorf("error.message = %q, want %q", message, rawValue)
		}
	})

	t.Run("kv result", func(t *testing.T) {
		var stdout bytes.Buffer

		err := NewRenderer(FormatKV, &stdout, nil).Render(result)

		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		want := "WARNING=" + rawValue + "\nTITLE=" + rawValue + "\nSTATUS=ok\n"
		if stdout.String() != want {
			t.Errorf("stdout = %q, want %q", stdout.String(), want)
		}
	})

	t.Run("kv failure", func(t *testing.T) {
		var stdout bytes.Buffer

		err := NewRenderer(FormatKV, &stdout, nil).RenderError(failure)

		if err != nil {
			t.Fatalf("RenderError: %v", err)
		}
		if want := "MESSAGE=" + rawValue + "\nSTATUS=invalid\n"; stdout.String() != want {
			t.Errorf("stdout = %q, want %q", stdout.String(), want)
		}
	})
}
