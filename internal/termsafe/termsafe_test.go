package termsafe

import "testing"

func TestEscape_EscapesControlsOnly(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"escape", "a\x1b[2Jb", `a\x1b[2Jb`},
		{"bell", "a\ab", `a\x07b`},
		{"carriage return", "a\rb", `a\x0db`},
		{"delete", "a\x7fb", `a\x7fb`},
		{"C1 control U+0085", "a\u0085b", `a\x85b`},
		{"C1 control U+009B", "a\u009bb", `a\x9bb`},
		{"lone byte 0x9b", "a\x9bb", `a\x9bb`},
		{"truncated multi-byte sequence", "a\xe2\x82b", `a\xe2\x82b`},
		{"tab and line feed", "a\tb\nc", "a\tb\nc"},
		{"accented letter", "café", "café"},
		{"multi-byte text", "→ 日本 🚀", "→ 日本 🚀"},
		{"plain text", "Login flow", "Login flow"},
		{"empty", "", ""},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			t.Parallel()

			got := Escape(current.input)

			if got != current.want {
				t.Errorf("Escape(%q) = %q, want %q", current.input, got, current.want)
			}
		})
	}
}
