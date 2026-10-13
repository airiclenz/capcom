package cli

import (
	"strings"
	"testing"
)

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

// hostileTitleYAML is a YAML double-quoted title carrying ESC, the C1 control U+009B and BEL.
const hostileTitleYAML = `"a\e[2Jb\x9bc\ad"`

// hostileText is hostileTitleYAML decoded: the raw characters a file hands the output layer.
const hostileText = "a\x1b[2Jb\u009bc\x07d"

// escapedHostileText is hostileText as text mode writes it.
const escapedHostileText = `a\x1b[2Jb\x9bc\x07d`

// rawControlBytes are the raw forms of hostileText's controls that text output must never carry;
// U+009B is the two-byte UTF-8 sequence C2 9B.
var rawControlBytes = []string{"\x1b", "\x07", "\u009b"}

func TestText_ControlCharactersFromFiles_AreEscapedAndJSONKeepsThem(t *testing.T) {
	body := "\n" + hostileText + "\n"
	root := newIssueStore(t, map[string]string{
		"hostile.md": readIssue(hostileTitleYAML, "open", "", body),
	})
	writeNoteFiles(t, root, map[string]string{
		"hostile-note.md": "---\ntitle: " + hostileTitleYAML + "\n---\n" + body,
	})

	textOutputs := map[string]string{
		"show":      mustRun(t, root, "show", "hostile"),
		"list":      mustRun(t, root, "list"),
		"know show": mustRun(t, root, "know", "show", "hostile-note"),
		"know tree": mustRun(t, root, "know", "tree"),
	}
	shownList, _ := decodeEnvelope(t, mustRun(t, root, "show", "--json", "hostile"))["data"].([]any)
	if len(shownList) != 1 {
		t.Fatalf("show --json data = %v, want one issue", shownList)
	}
	shown, _ := shownList[0].(map[string]any)
	kvOutput := mustRun(t, root, "show", "--kv", "hostile")

	for command, stdout := range textOutputs {
		if !strings.Contains(stdout, escapedHostileText) {
			t.Errorf("%s stdout = %q, want it to contain %q", command, stdout, escapedHostileText)
		}
		for _, raw := range rawControlBytes {
			if strings.Contains(stdout, raw) {
				t.Errorf("%s stdout = %q, carries the raw control %q", command, stdout, raw)
			}
		}
	}
	if shown["title"] != hostileText {
		t.Errorf("show --json title = %q, want %q", shown["title"], hostileText)
	}
	if jsonBody, _ := shown["body"].(string); !strings.Contains(jsonBody, hostileText) {
		t.Errorf("show --json body = %q, want it to contain %q", jsonBody, hostileText)
	}
	if !strings.Contains(kvOutput, "\nTITLE="+hostileText+"\n") {
		t.Errorf("show --kv = %q, want a TITLE= line carrying %q", kvOutput, hostileText)
	}
}
