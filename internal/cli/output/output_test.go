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

func TestJSONRenderer_Render_CarriesMatchOnlyWhenSet(t *testing.T) {
	cases := []struct {
		match     string
		wantMatch any
	}{
		{MatchAny, "any"},
		{MatchAll, "all"},
		{"", nil},
	}
	for _, testCase := range cases {
		var stdout bytes.Buffer

		err := NewRenderer(FormatJSON, &stdout, nil).Render(Result{Kind: "search", Match: testCase.match})

		envelope := decodeEnvelope(t, stdout.String())
		if gotMatch, hasMatch := envelope["match"]; err != nil || gotMatch != testCase.wantMatch ||
			hasMatch != (testCase.wantMatch != nil) {
			t.Errorf("Match %q: err = %v envelope = %v, want match %v", testCase.match, err, envelope, testCase.wantMatch)
		}
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

func TestRenderError_Suggestion_ReportedInEveryFormat(t *testing.T) {
	failure := &Error{Code: CodeInvalid, Message: `unknown command "shwo" for "capcom know"`, Suggestion: "know show"}
	cases := []struct {
		name   string
		format OutputFormat
		want   string
	}{
		{"text", FormatText, "capcom: unknown command \"shwo\" for \"capcom know\"; did you mean know show?\n"},
		{"kv", FormatKV, "MESSAGE=unknown command \"shwo\" for \"capcom know\"\nSUGGESTION=know show\nSTATUS=invalid\n"},
		{"json", FormatJSON, `"suggestion":"know show"`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var written bytes.Buffer

			err := NewRenderer(testCase.format, &written, &written).RenderError(failure)

			if err != nil {
				t.Fatalf("RenderError: %v", err)
			}
			got := written.String()
			isMatch := got == testCase.want
			if testCase.format == FormatJSON {
				isMatch = bytes.Contains(written.Bytes(), []byte(testCase.want))
			}
			if !isMatch {
				t.Errorf("output = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestRenderError_NoSuggestion_OmitsIt(t *testing.T) {
	failure := &Error{Code: CodeInvalid, Message: "bad"}
	cases := []struct {
		name   string
		format OutputFormat
		absent string
	}{
		{"text", FormatText, "did you mean"},
		{"kv", FormatKV, "SUGGESTION="},
		{"json", FormatJSON, `"suggestion"`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var written bytes.Buffer

			err := NewRenderer(testCase.format, &written, &written).RenderError(failure)

			if err != nil {
				t.Fatalf("RenderError: %v", err)
			}
			if bytes.Contains(written.Bytes(), []byte(testCase.absent)) {
				t.Errorf("output = %q, want no %q", written.String(), testCase.absent)
			}
		})
	}
}

func TestRenderError_SuggestionsAndValid_ReportedInEveryFormat(t *testing.T) {
	cases := []struct {
		name    string
		failure *Error
		format  OutputFormat
		want    string
	}{
		{
			"text one suggestion",
			&Error{Code: CodeNotFound, Message: `id "logn" not found`, Suggestions: []string{"login"}},
			FormatText,
			"capcom: id \"logn\" not found; did you mean login?\n",
		},
		{
			"text two suggestions",
			&Error{Code: CodeNotFound, Message: `id "logn" not found`, Suggestions: []string{"login", "logout"}},
			FormatText,
			"capcom: id \"logn\" not found; did you mean login or logout?\n",
		},
		{
			"text three suggestions",
			&Error{
				Code:        CodeNotFound,
				Message:     `id "logn" not found`,
				Suggestions: []string{"login", "logout", "log"},
			},
			FormatText,
			"capcom: id \"logn\" not found; did you mean login, logout or log?\n",
		},
		{
			"text valid",
			&Error{Code: CodeInvalid, Message: `unknown type "bgu"`, Valid: []string{"bug", "feature", "task"}},
			FormatText,
			"capcom: unknown type \"bgu\"; use one of: bug, feature, task\n",
		},
		{
			"kv one suggestion",
			&Error{Code: CodeNotFound, Message: `id "logn" not found`, Suggestions: []string{"login"}},
			FormatKV,
			"MESSAGE=id \"logn\" not found\nSUGGESTIONS=login\nSTATUS=not_found\n",
		},
		{
			"kv two suggestions",
			&Error{Code: CodeNotFound, Message: `id "logn" not found`, Suggestions: []string{"login", "logout"}},
			FormatKV,
			"MESSAGE=id \"logn\" not found\nSUGGESTIONS=login,logout\nSTATUS=not_found\n",
		},
		{
			"kv three suggestions",
			&Error{
				Code:        CodeNotFound,
				Message:     `id "logn" not found`,
				Suggestions: []string{"login", "logout", "log"},
			},
			FormatKV,
			"MESSAGE=id \"logn\" not found\nSUGGESTIONS=login,logout,log\nSTATUS=not_found\n",
		},
		{
			"kv valid",
			&Error{Code: CodeInvalid, Message: `unknown status "opne"`, Valid: []string{"open (open)", "done (closed)"}},
			FormatKV,
			"MESSAGE=unknown status \"opne\"\nVALID=open (open),done (closed)\nSTATUS=invalid\n",
		},
		{
			"kv every hint",
			&Error{
				Code:        CodeInvalid,
				Message:     "x",
				Suggestion:  "--status",
				Suggestions: []string{"login"},
				Valid:       []string{"bug"},
			},
			FormatKV,
			"MESSAGE=x\nSUGGESTION=--status\nSUGGESTIONS=login\nVALID=bug\nSTATUS=invalid\n",
		},
		{
			"json one suggestion",
			&Error{Code: CodeNotFound, Message: "x", Suggestions: []string{"login"}},
			FormatJSON,
			`"error":{"code":"not_found","message":"x","suggestions":["login"]}`,
		},
		{
			"json two suggestions",
			&Error{Code: CodeNotFound, Message: "x", Suggestions: []string{"login", "logout"}},
			FormatJSON,
			`"error":{"code":"not_found","message":"x","suggestions":["login","logout"]}`,
		},
		{
			"json three suggestions",
			&Error{Code: CodeNotFound, Message: "x", Suggestions: []string{"login", "logout", "log"}},
			FormatJSON,
			`"error":{"code":"not_found","message":"x","suggestions":["login","logout","log"]}`,
		},
		{
			"json valid",
			&Error{Code: CodeInvalid, Message: "x", Valid: []string{"bug", "task"}},
			FormatJSON,
			`"error":{"code":"invalid","message":"x","valid":["bug","task"]}`,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var written bytes.Buffer

			err := NewRenderer(testCase.format, &written, &written).RenderError(testCase.failure)

			if err != nil {
				t.Fatalf("RenderError: %v", err)
			}
			got := written.String()
			isMatch := got == testCase.want
			if testCase.format == FormatJSON {
				isMatch = bytes.Contains(written.Bytes(), []byte(testCase.want))
			}
			if !isMatch {
				t.Errorf("output = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestRenderError_EmptyHintLists_RenderAsWithout(t *testing.T) {
	cases := []struct {
		name   string
		format OutputFormat
		want   string
	}{
		{"text", FormatText, "capcom: bad\n"},
		{"kv", FormatKV, "MESSAGE=bad\nSTATUS=invalid\n"},
		{
			"json",
			FormatJSON,
			`{"schema":1,"kind":"error","error":{"code":"invalid","message":"bad"},"elsewhere":[],"warnings":[]}` + "\n",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			failure := &Error{Code: CodeInvalid, Message: "bad", Suggestions: []string{}, Valid: []string{}}
			var written bytes.Buffer

			err := NewRenderer(testCase.format, &written, &written).RenderError(failure)

			if err != nil {
				t.Fatalf("RenderError: %v", err)
			}
			if written.String() != testCase.want {
				t.Errorf("output = %q, want %q", written.String(), testCase.want)
			}
		})
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
