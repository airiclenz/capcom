package markdown

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// The assertions are written against the VISIBLE text (ansi.Strip) and the visible width
// (lipgloss.Width), so they hold whatever colour profile the test environment resolves — the
// parser must consume its markers and keep the content and layout correct regardless of whether
// ANSI is emitted. The few "is it actually styled?" checks are guarded by colorActive so they are
// skipped (not failed) on a no-colour profile.

// strip returns the visible text of a rendered line with its ANSI styling removed.
func strip(s string) string { return ansi.Strip(s) }

// colorActive reports whether this environment's lipgloss profile emits ANSI escapes, so the
// style-presence assertions run only where colour is actually produced.
func colorActive(th Theme) bool { return strings.Contains(th.Code.Render("x"), "\x1b") }

// underlineSGR returns the opening escape sequence th.Underline paints with, probed from the
// theme rather than spelled literally: lipgloss composes the underline parameter with others of its
// own choosing (today "\x1b[4;4m", not the bare "\x1b[4m"), and the wire spelling is its business,
// not the test's. Empty string on a no-colour profile, where there is nothing to look for.
func underlineSGR(th Theme) string {
	probe := th.Underline.Render("x")
	if !strings.HasPrefix(probe, "\x1b[") {
		return ""
	}
	if idx := strings.IndexByte(probe, 'm'); idx >= 0 {
		return probe[:idx+1]
	}
	return ""
}

func TestRenderInlineBold(t *testing.T) {
	t.Parallel()

	th := testTheme()
	got := renderInline(th, nil, "a **bold** b")
	if v := strip(got); v != "a bold b" {
		t.Errorf("visible = %q; want %q (the ** markers consumed)", v, "a bold b")
	}
	if strings.Contains(got, "**") {
		t.Errorf("output still carries the literal ** markers: %q", got)
	}
	if colorActive(th) && !strings.Contains(got, "\x1b") {
		t.Errorf("bold span emitted no styling: %q", got)
	}
}

// <u>…</u> is the one HTML pair the renderer knows: the tags are consumed and the enclosed run is
// underlined (SGR 4).
func TestRenderInlineUnderline(t *testing.T) {
	t.Parallel()

	th := testTheme()
	got := renderInline(th, nil, "press <u>Enter</u> now")
	if v := strip(got); v != "press Enter now" {
		t.Errorf("visible = %q; want %q (the <u> tags consumed)", v, "press Enter now")
	}
	if strings.Contains(got, "<u>") || strings.Contains(got, "</u>") {
		t.Errorf("output still carries the literal tags: %q", got)
	}
	if sgr := underlineSGR(th); sgr != "" && !strings.Contains(got, sgr) {
		t.Errorf("underline span emitted no underline SGR %q: %q", sgr, got)
	}
}

// Only the exact lowercase pair is markup. Everything else that looks like it — CommonMark's __,
// an uppercase tag, a tag with anything after the "u" — survives byte for byte and unstyled.
func TestRenderInlineUnderlineLiteralPassthrough(t *testing.T) {
	t.Parallel()

	th := testTheme()
	for _, in := range []string{"__text__", "<U>x</U>", "<u >x</u>"} {
		got := renderInline(th, nil, in)
		if got != in {
			t.Errorf("renderInline(%q) = %q; want it byte-for-byte literal", in, got)
		}
	}
}

func TestRenderInlineCode(t *testing.T) {
	t.Parallel()

	th := testTheme()
	got := renderInline(th, nil, "run `go test` now")
	if v := strip(got); v != "run go test now" {
		t.Errorf("visible = %q; want %q (the backticks consumed)", v, "run go test now")
	}
	if strings.Contains(got, "`") {
		t.Errorf("output still carries a literal backtick: %q", got)
	}
}

// A code span wins over bold: ** inside `…` stays literal (CommonMark code-span precedence).
func TestRenderInlineCodeBeatsBold(t *testing.T) {
	t.Parallel()

	th := testTheme()
	got := renderInline(th, nil, "`**x**`")
	if v := strip(got); v != "**x**" {
		t.Errorf("visible = %q; want %q (bold not parsed inside a code span)", v, "**x**")
	}
}

// A code span wins over <u> too: the tag text stays visible inside `…` and is never underlined.
func TestRenderInlineCodeBeatsUnderline(t *testing.T) {
	t.Parallel()

	th := testTheme()
	got := renderInline(th, nil, "`<u>x</u>`")
	if v := strip(got); v != "<u>x</u>" {
		t.Errorf("visible = %q; want %q (<u> not parsed inside a code span)", v, "<u>x</u>")
	}
	if sgr := underlineSGR(th); sgr != "" && strings.Contains(got, sgr) {
		t.Errorf("code span underlined its contents: %q", got)
	}
}

// An unterminated marker is left literal — never a leaked escape or eaten text.
func TestRenderInlineUnterminated(t *testing.T) {
	t.Parallel()

	th := testTheme()
	for _, in := range []string{"a **bold start", "a `code start", "trailing *", "a <u>open"} {
		got := renderInline(th, nil, in)
		if v := strip(got); v != in {
			t.Errorf("renderInline(%q) visible = %q; want it left literal", in, v)
		}
		if strings.Contains(got, "\x1b") {
			t.Errorf("renderInline(%q) styled an unterminated span: %q", in, got)
		}
	}
}

func TestHeadingStripsMarkers(t *testing.T) {
	t.Parallel()

	th := testTheme()
	for _, lvl := range []string{"#", "##", "###", "####", "#####", "######"} {
		out := Render(th, lvl+" Title", 40, Options{}).Lines
		if len(out) != 1 {
			t.Fatalf("%q heading: got %d lines, want 1", lvl, len(out))
		}
		if v := strip(out[0]); v != "Title" {
			t.Errorf("%q heading visible = %q; want %q", lvl, v, "Title")
		}
		if strings.Contains(out[0], "#") {
			t.Errorf("%q heading still carries a literal #: %q", lvl, out[0])
		}
		if colorActive(th) && !strings.Contains(out[0], "\x1b") {
			t.Errorf("%q heading emitted no styling: %q", lvl, out[0])
		}
	}
}

// Seven #s (or a # with no following space) is not a heading; it renders as a plain paragraph.
func TestNotAHeading(t *testing.T) {
	t.Parallel()

	th := testTheme()
	for _, in := range []string{"####### TooDeep", "#NoSpace"} {
		out := Render(th, in, 40, Options{}).Lines
		if v := strip(out[0]); v != in {
			t.Errorf("Render(%q) visible = %q; want it unchanged (not a heading)", in, v)
		}
	}
}

func TestBulletList(t *testing.T) {
	t.Parallel()

	th := testTheme()
	for _, b := range []string{"-", "*", "+"} {
		out := Render(th, b+" one\n"+b+" two", 40, Options{}).Lines
		if len(out) != 2 {
			t.Fatalf("bullet %q: got %d lines, want 2", b, len(out))
		}
		if v := strip(out[0]); v != "• one" {
			t.Errorf("bullet %q line 0 visible = %q; want %q", b, v, "• one")
		}
		if v := strip(out[1]); v != "• two" {
			t.Errorf("bullet %q line 1 visible = %q; want %q", b, v, "• two")
		}
	}
}

func TestNumberedList(t *testing.T) {
	t.Parallel()

	th := testTheme()
	out := Render(th, "1. first\n2) second", 40, Options{}).Lines
	if v := strip(out[0]); v != "1. first" {
		t.Errorf("numbered line 0 visible = %q; want %q", v, "1. first")
	}
	if v := strip(out[1]); v != "2) second" {
		t.Errorf("numbered line 1 visible = %q; want %q", v, "2) second")
	}
}

// A wrapped list item hangs under its text: the continuation line is indented to the marker width.
func TestListHangingIndent(t *testing.T) {
	t.Parallel()

	th := testTheme()
	out := Render(th, "- "+strings.Repeat("word ", 8), 20, Options{}).Lines
	if len(out) < 2 {
		t.Fatalf("expected the long item to wrap, got %d line(s)", len(out))
	}
	if v := strip(out[0]); !strings.HasPrefix(v, "• ") {
		t.Errorf("first line = %q; want it to lead with the bullet", v)
	}
	if v := strip(out[1]); !strings.HasPrefix(v, "  ") {
		t.Errorf("continuation = %q; want a two-column hanging indent under the text", v)
	}
}

func TestFencedCodeBlock(t *testing.T) {
	t.Parallel()

	th := testTheme()
	out := Render(th, "```go\nfmt.Println()\n```", 40, Options{}).Lines
	if len(out) != 1 {
		t.Fatalf("got %d lines, want 1 (fence lines dropped)", len(out))
	}
	if v := strip(out[0]); v != "  fmt.Println()" {
		t.Errorf("code line visible = %q; want %q (indented, no fence)", v, "  fmt.Println()")
	}
	for _, ln := range out {
		if strings.Contains(ln, "```") {
			t.Errorf("a fence marker leaked into the output: %q", ln)
		}
	}
	if colorActive(th) && !strings.Contains(out[0], "\x1b") {
		t.Errorf("code block emitted no styling: %q", out[0])
	}
}

// An unterminated fence renders the body it has, with no leaked fence marker.
func TestFencedCodeBlockUnterminated(t *testing.T) {
	t.Parallel()

	th := testTheme()
	out := Render(th, "```\ncode line", 40, Options{}).Lines
	if v := strip(out[0]); v != "  code line" {
		t.Errorf("visible = %q; want %q", v, "  code line")
	}
	for _, ln := range out {
		if strings.Contains(ln, "```") {
			t.Errorf("a fence marker leaked: %q", ln)
		}
	}
}

// A run of two or more blank lines collapses to a single blank row (the paragraph break stays,
// the padding goes); one blank line is left exactly as it was.
func TestCollapsesBlankRuns(t *testing.T) {
	t.Parallel()

	th := testTheme()
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"single blank kept", "first\n\nsecond", []string{"first", "", "second"}},
		{"triple blank collapses", "first\n\n\n\nsecond", []string{"first", "", "second"}},
		{"whitespace-only lines count as blank", "first\n \n\t\nsecond", []string{"first", " ", "second"}},
		{"leading run collapses", "\n\n\nfirst", []string{"", "first"}},
		{"all blank is one blank line", "\n\n\n", []string{""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out := Render(th, tc.in, 40, Options{}).Lines
			if len(out) != len(tc.want) {
				t.Fatalf("Render(%q) = %#v; want %#v", tc.in, visible(out), tc.want)
			}
			for i := range out {
				if v := strip(out[i]); v != tc.want[i] {
					t.Errorf("line %d visible = %q; want %q", i, v, tc.want[i])
				}
			}
		})
	}
}

// Blank lines inside a fenced code block are code, not padding: they survive verbatim, and a
// blank line immediately after the closing fence is not swallowed by one inside it.
func TestFencedCodeBlockKeepsBlankLines(t *testing.T) {
	t.Parallel()

	th := testTheme()
	out := Render(th, "```go\na()\n\n\nb()\n```\n\ntail", 40, Options{}).Lines
	want := []string{"  a()", "  ", "  ", "  b()", "", "tail"}
	if len(out) != len(want) {
		t.Fatalf("got %#v; want %#v (fence interior verbatim)", visible(out), want)
	}
	for i := range out {
		if v := strip(out[i]); v != want[i] {
			t.Errorf("line %d visible = %q; want %q", i, v, want[i])
		}
	}
}

// visible is the ANSI-stripped text of every rendered line — the failure-message form of a block.
func visible(lines []string) []string {
	out := make([]string, len(lines))
	for i, ln := range lines {
		out[i] = strip(ln)
	}
	return out
}

// visibleTrimmed is visible with each line's trailing blanks dropped — the form to compare a
// block against text quoted from a doc, where a run of spaces at a line's end cannot be written
// down. It is only ever the right comparison where a separate assertion pins the widths.
func visibleTrimmed(lines []string) []string {
	out := visible(lines)
	for i, ln := range out {
		out[i] = strings.TrimRight(ln, " ")
	}
	return out
}

// Plain text with no markup is returned byte-identical. A pipe that is not part of a table — no delimiter
// row under it — is plain text like any other character.
func TestPlainTextUnchanged(t *testing.T) {
	t.Parallel()

	th := testTheme()
	for _, in := range []string{
		"just plain body text",
		"run cat file | grep needle",
		"a | b",
		"| a | b |",
		"--- | ---",
		"the range is 1 -- 3",
	} {
		out := Render(th, in, 80, Options{}).Lines
		if len(out) != 1 || out[0] != in {
			t.Errorf("Render(%q) = %#v; want a single unchanged line", in, out)
		}
	}
}

// Every rendered line stays within the content width, even when a bold span crosses a soft-wrap
// boundary — the guarantee that baked-in ANSI never perturbs the wrap arithmetic. A table obeys
// the same cap: its columns shrink and its cells truncate until the whole block fits.
func TestWidthNeverExceeds(t *testing.T) {
	t.Parallel()

	th := testTheme()
	const width = 20
	bodies := []string{
		"**" + strings.Repeat("alpha ", 10) + "**bold tail",
		"| header one | header two |\n| :-- | --: |\n| " + strings.Repeat("long ", 6) + "| 7 |",
		"| a | b | c |\n| --- | :-: | ---: |\n| **bold cell** | `code cell` | x |",
	}
	for _, body := range bodies {
		for _, ln := range Render(th, body, width, Options{}).Lines {
			if w := lipgloss.Width(ln); w > width {
				t.Errorf("line %q has visible width %d > %d", strip(ln), w, width)
			}
		}
	}
}

// The same absolute cap, swept across every width a table can be handed and asserted in BOTH
// measures the painter can be in — the sweep is where the narrow-width endpoints live, and one
// width in isolation cannot find them. Before the readable floor, a column squeezed to a single
// cell could be handed a two-cell grapheme that no break divides: the cell stood a column proud of
// the cap at widths 5, 9 and 10 under GraphemeWidth. The floor and the paragraph fallback below it
// are what close that, so the only line left that may cross the cap is the one Render exempts —
// a single indivisible grapheme wider than the whole width, which keeps a line to itself.
func TestTableWidthNeverExceedsAcrossWidths(t *testing.T) {
	t.Parallel()

	source := strings.Join([]string{
		"| id | description | " + vs16Warning + " |",
		"| --- | :-: | --: |",
		"| 1 | " + strings.Repeat("wrap ", 8) + "| " + vs16Warning + " |",
		"| 22 | " + vs16Warning + " short | **bold** |",
	}, "\n")

	for _, pm := range paintMethods {
		t.Run(pm.name, func(t *testing.T) {
			t.Parallel()
			th := testTheme()
			th.Measure = Measure{method: pm.method}

			for width := 1; width <= 120; width++ {
				for i, ln := range Render(th, source, width, Options{}).Lines {
					v := strip(ln)
					w := th.Measure.Width(ln)
					if w <= width {
						continue
					}
					if cluster, _ := ansi.FirstGraphemeCluster(v, pm.method); cluster == v {
						continue // the one exemption: a grapheme no break can divide
					}
					t.Errorf("width %d: line %d %q is %d cells wide, over the cap", width, i, v, w)
				}
			}
		})
	}
}

func TestEmptyMessageRendersOneLine(t *testing.T) {
	t.Parallel()

	th := testTheme()
	if out := Render(th, "", 40, Options{}).Lines; len(out) != 1 || strip(out[0]) != "" {
		t.Errorf("empty text = %#v; want a single empty line", out)
	}
}

// testTheme returns a Theme of plain lipgloss styles, one distinct style per role, measuring in
// the painter's starting method.
func testTheme() Theme {
	return Theme{
		Bold:      lipgloss.NewStyle().Bold(true),
		Underline: lipgloss.NewStyle().Underline(true),
		Heading:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
		Code:      lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
		CodeBlock: lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
		Rule:      lipgloss.NewStyle().Faint(true),
		Link:      lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color("4")),
		Inert:     lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		Quote:     lipgloss.NewStyle().Faint(true),
	}
}
