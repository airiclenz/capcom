package config

import (
	"errors"
	"testing"
)

func TestRaiseFormat_EverySpellingOfTheKey_RewritesItsLineInPlace(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"double-quoted key", "types: [bug]\n\"format\": 2\nlock:\n  ttl: 1h\n",
			"types: [bug]\nformat: 3\nlock:\n  ttl: 1h\n"},
		{"single-quoted key", "'format': 2 # old\ntypes: [bug]\n", "format: 3\ntypes: [bug]\n"},
		{"blank before the colon", "types: [bug]\nformat : 2\n", "types: [bug]\nformat: 3\n"},
		{"tab before the colon", "\"format\"\t: 2\r\ntypes: [bug]\r\n", "format: 3\r\ntypes: [bug]\r\n"},
		{"byte order mark", byteOrderMark + "format: 2\ntypes: [bug]\n", byteOrderMark + "format: 3\ntypes: [bug]\n"},
		{"byte order mark, quoted key", byteOrderMark + "\"format\": 2\n", byteOrderMark + "format: 3\n"},
		{"byte order mark, no key", byteOrderMark + "types: [bug]\n", byteOrderMark + "format: 3\ntypes: [bug]\n"},
		{"byte order mark, document start", byteOrderMark + "---\ntypes: [bug]\n",
			byteOrderMark + "---\nformat: 3\ntypes: [bug]\n"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			raised, isChanged, err := RaiseFormat(testPath, []byte(testCase.content), 3)

			if err != nil || !isChanged || string(raised) != testCase.want {
				t.Fatalf("RaiseFormat(%q) = %q, %v, %v; want %q, true, nil",
					testCase.content, raised, isChanged, err, testCase.want)
			}
			if parsed, err := Parse(testPath, raised); err != nil || parsed.Format != 3 {
				t.Errorf("Parse(raised) = format %d, %v; want format 3", parsed.Format, err)
			}
		})
	}
}

func TestRaiseFormat_QuotedKeyAtTheTarget_IsUnchanged(t *testing.T) {
	t.Parallel()
	const content = "\"format\": 3\ntypes: [bug]\n"

	raised, isChanged, err := RaiseFormat(testPath, []byte(content), 3)

	if err != nil || isChanged || string(raised) != content {
		t.Errorf("RaiseFormat = %q, %v, %v; want it unchanged", raised, isChanged, err)
	}
}

func TestRaiseFormat_UnplaceableKey_FailsAtTheKeyLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		content  string
		wantLine int
	}{
		{"flow mapping", "{types: [bug], format: 2}\n", 1},
		{"flow mapping over several lines", "{types: [bug],\nformat: 2}\n", 2},
		{"indented mapping", "  types: [bug]\n  format: 2\n", 2},
		{"value on the next line", "types: [bug]\nformat:\n  2\n", 2},
		{"folded block scalar", "types: [bug]\nformat: >-\n  2\n", 2},
		{"explicit key", "types: [bug]\n? format\n: 2\n", 2},
		{"escaped key", "types: [bug]\n\"form\\x61t\": 2\n", 2},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Parse(testPath, []byte(testCase.content)); err != nil {
				t.Fatalf("Parse(%q): %v, want the fixture to parse", testCase.content, err)
			}

			raised, isChanged, err := RaiseFormat(testPath, []byte(testCase.content), 3)

			var failure *Error
			if !errors.As(err, &failure) || isChanged || raised != nil {
				t.Fatalf("RaiseFormat(%q) = %q, %v, %v; want nil, false and a *config.Error",
					testCase.content, raised, isChanged, err)
			}
			want := &Error{Path: testPath, Line: testCase.wantLine, Message: unplaceableFormatMessage}
			if *failure != *want {
				t.Errorf("err = %q, want %q", failure, want)
			}
		})
	}
}

func TestRaiseFormat_ContentThatFailsToParse_ReturnsTheParseError(t *testing.T) {
	t.Parallel()

	raised, isChanged, err := RaiseFormat(testPath, []byte("types: [bug]\nbogus: 1\n"), 3)

	want := testPath + `:2: unknown key "bogus"`
	if err == nil || err.Error() != want || isChanged || raised != nil {
		t.Errorf("RaiseFormat = %q, %v, %v; want nil, false and %q", raised, isChanged, err, want)
	}
}
