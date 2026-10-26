package index

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExcerpt_BodyAndQuery_ReturnsTheExcerptShape(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		text string
		want []string
	}{
		{"one match keeps a line either side", "a\nb\nlogin here\nc\nd\n", "login", []string{"b", "login here", "c"}},
		{"match on the first line has no line before", "login first\nsecond\nthird\n", "login", []string{"login first", "second"}},
		{"match on the last line has no line after", "first\nsecond\nlast login", "login", []string{"second", "last login"}},
		{
			"overlapping windows merge", "a\nlogin one\nb\nlogin two\nc\nd\n", "login",
			[]string{"a", "login one", "b", "login two", "c"},
		},
		{
			"touching windows merge without a gap", "a\nlogin\nb\nc\nlogin\nd\n", "login",
			[]string{"a", "login", "b", "c", "login", "d"},
		},
		{"windows apart get a gap line", "login\nb\nc\nd\nlogin\n", "login", []string{"login", "b", "…", "d", "login"}},
		{
			"only the first three matching lines count", "login a\nlogin b\nlogin c\nlogin d\nlogin e\n", "login",
			[]string{"login a", "login b", "login c", "login d"},
		},
		{
			"eight lines at most", "login\n1\n2\n3\nlogin\n5\n6\n7\nlogin\n9\n", "login",
			[]string{"login", "1", "…", "3", "login", "5", "…", "7"},
		},
		{
			"never ends on a gap", "0\nlogin\n2\n3\n4\nlogin\n6\n7\n8\nlogin\n10\n11\n", "login",
			[]string{"0", "login", "2", "…", "4", "login", "6"},
		},
		{"any word of the query matches", "a\nparser here\nb\nc\nd\ncache there\n", "parser cache", []string{
			"a", "parser here", "b", "…", "d", "cache there",
		}},
		{"punctuated query word matches its token run", "the login timeout race shows\n", "login-timeout-race", []string{
			"the login timeout race shows",
		}},
		{"punctuated query word needs its tokens in order", "login race timeout\n", "login-timeout-race", nil},
		{"punctuation in the line does not hide a word", "see (parser), then\n", "parser", []string{"see (parser), then"}},
		{"case folds", "Login Fails\n", "LOGIN", []string{"Login Fails"}},
		{"plain query matches a composed diacritic", "a\nthe café opens\nb\n", "cafe", []string{
			"a", "the café opens", "b",
		}},
		{"composed query matches a plain word", "the cafe opens\n", "café", []string{"the cafe opens"}},
		{"plain query matches a decomposed diacritic", "the cafe\u0301 opens\n", "cafe", []string{"the cafe\u0301 opens"}},
		{"decomposed query matches a composed diacritic", "the café opens\n", "cafe\u0301", []string{"the café opens"}},
		{"diacritics fold with case", "Café Crème\n", "CAFE CREME", []string{"Café Crème"}},
		{"words match as whole tokens", "login fails\n", "log", nil},
		{"title-only match gives no lines", "nothing relevant here\n", "login", nil},
		{"blank query gives no lines", "login\n", "  ", nil},
		{"query without tokens gives no lines", "login\n", `"`, nil},
		{"line endings are dropped", "a\r\nlogin\r\nb\r\n", "login", []string{"a", "login", "b"}},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := Excerpt(test.body, test.text)

			if !slices.Equal(got, test.want) {
				t.Errorf("Excerpt(%q, %q) = %q; want %q", test.body, test.text, got, test.want)
			}
		})
	}
}

func TestExcerpt_LongLine_CutsAtTheRuneLimitOnARuneBoundary(t *testing.T) {
	t.Parallel()
	line := "login " + strings.Repeat("é", 200)

	got := Excerpt(line, "login")

	want := string([]rune(line)[:excerptRuneLimit-1]) + "…"
	if len(got) != 1 || got[0] != want || !utf8.ValidString(got[0]) || utf8.RuneCountInString(got[0]) != excerptRuneLimit {
		t.Errorf("Excerpt(long line) = %q; want one valid line of %d runes ending in …", got, excerptRuneLimit)
	}
}

func TestExcerpt_LineAtTheRuneLimit_KeptWhole(t *testing.T) {
	t.Parallel()
	line := "login " + strings.Repeat("é", excerptRuneLimit-len("login "))

	got := Excerpt(line, "login")

	if !slices.Equal(got, []string{line}) {
		t.Errorf("Excerpt(line of %d runes) = %q; want it whole", excerptRuneLimit, got)
	}
}

func TestBody_IndexedIssueAndNote_ReturnsTheirBodies(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "login-bug.md", issueText("Login fails", "open", "The callback drops the session.\n"))
	writeNote(t, s, "testing-rules.md", "---\ntitle: Testing rules\n---\nTests use real temp dirs.\n")
	x := openIndex(t, s)
	refresh(t, x)
	cases := map[string]string{
		mustIssue(t, x, "login-bug").Path:    "The callback drops the session.\n",
		".capcom/knowledge/testing-rules.md": "Tests use real temp dirs.\n",
		".capcom/knowledge/not-indexed.md":   "",
	}

	for path, want := range cases {
		got, err := x.Body(path)

		if err != nil || got != want {
			t.Errorf("Body(%q) = %q, %v; want %q, nil", path, got, err, want)
		}
	}
}
