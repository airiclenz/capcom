package markdown

import (
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// vs16Warning is U+26A0 WARNING SIGN followed by U+FE0F VARIATION SELECTOR-16 — the grapheme the
// two measures disagree about. GraphemeWidth promotes any cluster carrying VS16 to two cells;
// WcWidth takes the first non-zero-width rune of the cluster, and U+26A0 is one cell wide.
const vs16Warning = "\u26a0\ufe0f"

// paintMethods is the pair every painted assertion runs against: the measure a terminal that never
// answers mode 2027 paints in, and the one Bubble Tea switches to when a terminal does.
var paintMethods = []struct {
	name   string
	method ansi.Method
}{
	{"WcWidth — the painter's default", ansi.WcWidth},
	{"GraphemeWidth — the terminal answered mode 2027", ansi.GraphemeWidth},
}

// wrapText holds the cap itself rather than trusting the wrap it delegates to. x/ansi's
// breakpoint branch (x/ansi@v0.11.7/wrap.go:406-419) has neither of the already-full-line checks
// its default branch has, so a run of the wrapper's own breakpoints kept growing a word onto a
// full line: ansi.Wrap("| --- | --- | --- |", 3, "") returned a five-cell first line, and the
// same input at limit 8 an eleven-cell one. That input is not exotic — a markdown delimiter row
// is rendered verbatim whenever a table is too narrow to lay out, and any hyphenated
// word carries the same run in miniature.
//
// The cap is measured in the Theme's Measure, since that is the measure the frame is painted in, so the whole table runs under both of them.
func TestWrapTextHoldsTheWidthCap(t *testing.T) {
	t.Parallel()

	cases := []struct{ name, in string }{
		{"delimiter row", "| --- | --- | --- |"},
		{"hyphen run", "----"},
		{"pipe row", "| alpha | beta | gamma |"},
		{"hyphenated prose", "a deeply nested sub-agent message that must wrap hard"},
		{"mixed pipe and hyphen runs", "|-|--|---|-- a-b-c |"},
		{"leading indent", "    indented -- text"},
		{"cjk", "日本語のテキスト"},
		{"vs16", "a " + vs16Warning + " b " + strings.Repeat(vs16Warning, 3) + " c"},
		{"cjk and vs16", "警告 " + vs16Warning + " 日本語-テキスト | " + vs16Warning},
		{"empty", ""},
	}

	for _, pm := range paintMethods {
		for _, c := range cases {
			for limit := 1; limit <= 8; limit++ {
				t.Run(pm.name+"/"+c.name+"/limit "+strconv.Itoa(limit), func(t *testing.T) {
					t.Parallel()
					th := testTheme()
					th.Measure = Measure{method: pm.method}

					lines := wrapText(th, c.in, limit)
					if len(lines) == 0 {
						t.Fatalf("wrapText returned no lines at all")
					}
					for i, ln := range lines {
						w := th.Measure.Width(ln)
						if w <= limit {
							continue
						}
						// The one thing a break cannot divide is a single grapheme wider than the
						// limit — a CJK glyph at limit 1 — and it gets a line to itself.
						if cluster, _ := ansi.FirstGraphemeCluster(ln, pm.method); cluster == ln {
							continue
						}
						t.Errorf("line %d %q is %d cells wide, over the %d cap: %q", i, ln, w, limit, lines)
					}
					// Capping moves where the lines break and nothing else: every non-space
					// character the wrap produced is still there, in order. (A break may swallow
					// the space it happens at, which is the wrapper's own behaviour.)
					capped := withoutSpaces(strings.Join(lines, ""))
					uncapped := withoutSpaces(strings.ReplaceAll(ansi.Wrap(c.in, limit, ""), "\n", ""))
					if capped != uncapped {
						t.Errorf("capping changed the content to %q, want the wrap's own %q", capped, uncapped)
					}
				})
			}
		}
	}
}

// withoutSpaces drops every space from s — the part of a wrapped line that must survive a
// re-break, since a break may consume the space it falls on.
func withoutSpaces(s string) string { return strings.ReplaceAll(s, " ", "") }

// Enforcing the cap in the Measure is only half of it: the BREAK has to be chosen in that measure
// too, or the wrapper is deciding where lines end by a ruler the terminal is not using. The
// package-level ansi.Wrap is hard-wired to ansi.GraphemeWidth whatever the painter does; on the
// painter's DEFAULT WcWidth that reads a VARIATION SELECTOR-16 cluster as two cells where the
// terminal paints one, so a wrap through it breaks a cell early on every line carrying one.
//
// Under GraphemeWidth the Measure's wrap IS ansi.Wrap, so the grapheme half of this table also
// pins that nothing changes where the two measures agree.
func TestWrapTextBreaksInThePaintersMeasure(t *testing.T) {
	t.Parallel()

	warns := vs16Warning + " " + vs16Warning + " " + vs16Warning

	cases := []struct {
		name         string
		in           string
		limit        int
		wc, grapheme []string
	}{
		{
			// Five painted cells at limit 5 — one line the painter fills exactly. GraphemeWidth
			// calls the same string eight cells and breaks it.
			name:     "VS16 clusters that fit the painted limit",
			in:       warns,
			limit:    5,
			wc:       []string{warns},
			grapheme: []string{vs16Warning + " " + vs16Warning, vs16Warning},
		},
		{
			name:     "VS16 clusters that fit a narrow painted limit",
			in:       vs16Warning + " " + vs16Warning,
			limit:    3,
			wc:       []string{vs16Warning + " " + vs16Warning},
			grapheme: []string{vs16Warning, vs16Warning},
		},
		{
			// The wrap only moves where the measures disagree: plain prose breaks identically.
			name:     "ascii prose the measures agree about",
			in:       "the quick brown fox jumps",
			limit:    10,
			wc:       []string{"the quick", "brown fox", "jumps"},
			grapheme: []string{"the quick", "brown fox", "jumps"},
		},
		{
			// Wide glyphs are two cells to BOTH measures, so they break the same way as well.
			name:     "cjk the measures agree about",
			in:       "日本語のテキスト",
			limit:    6,
			wc:       []string{"日本語", "のテキ", "スト"},
			grapheme: []string{"日本語", "のテキ", "スト"},
		},
	}

	for _, pm := range paintMethods {
		for _, c := range cases {
			t.Run(pm.name+"/"+c.name, func(t *testing.T) {
				t.Parallel()
				th := testTheme()
				th.Measure = Measure{method: pm.method}

				want := c.wc
				if pm.method == ansi.GraphemeWidth {
					want = c.grapheme
				}
				if got := wrapText(th, c.in, c.limit); !reflect.DeepEqual(got, want) {
					t.Errorf("wrapText(%q, %d) = %q, want %q", c.in, c.limit, got, want)
				}
			})
		}
	}
}

// The painter starts at ansi.WcWidth (the zero value of ansi.Method), so the Measure must too. The
// zero value is what a Measure that was never set holds — inside a zero Theme, or a value-copied
// Bubble Tea model — so it has to be the honest default rather than a wrong one.
func TestMeasureStartsAtThePainterMeasure(t *testing.T) {
	t.Parallel()

	var zero Measure
	if got := zero.Method(); got != ansi.WcWidth {
		t.Errorf("the zero Measure measures with %v, want the painter's ansi.WcWidth", got)
	}
	if got := (Theme{}).Measure.Method(); got != ansi.WcWidth {
		t.Errorf("a zero Theme measures with %v, want the painter's ansi.WcWidth", got)
	}
}

// Observe mirrors Bubble Tea's own reaction to a mode report, so the cases are Bubble Tea's:
// mode 2027 answered as anything other than "not recognized" or "permanently reset" moves the
// painter to GraphemeWidth — including the plain "reset", which says the terminal understands the
// mode and merely has it off, and which Bubble Tea then switches on. Every other report leaves the
// painter, and so the Measure, alone.
func TestMeasureObservesTheUnicodeCoreReport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		report tea.ModeReportMsg
		want   ansi.Method
	}{
		{"2027 set", tea.ModeReportMsg{Mode: ansi.ModeUnicodeCore, Value: ansi.ModeSet}, ansi.GraphemeWidth},
		{"2027 reset — understood but off, which bubbletea turns on", tea.ModeReportMsg{Mode: ansi.ModeUnicodeCore, Value: ansi.ModeReset}, ansi.GraphemeWidth},
		{"2027 permanently set", tea.ModeReportMsg{Mode: ansi.ModeUnicodeCore, Value: ansi.ModePermanentlySet}, ansi.GraphemeWidth},
		{"2027 permanently reset", tea.ModeReportMsg{Mode: ansi.ModeUnicodeCore, Value: ansi.ModePermanentlyReset}, ansi.WcWidth},
		{"2027 not recognized", tea.ModeReportMsg{Mode: ansi.ModeUnicodeCore, Value: ansi.ModeNotRecognized}, ansi.WcWidth},
		{"a different mode, set", tea.ModeReportMsg{Mode: ansi.ModeSynchronizedOutput, Value: ansi.ModeSet}, ansi.WcWidth},
		{"a different mode, reset", tea.ModeReportMsg{Mode: ansi.ModeSynchronizedOutput, Value: ansi.ModeReset}, ansi.WcWidth},
		{"no mode at all", tea.ModeReportMsg{}, ansi.WcWidth},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := (Measure{}).Observe(tc.report).Method(); got != tc.want {
				t.Errorf("observing %+v measures with %v, want %v", tc.report, got, tc.want)
			}
		})
	}

	t.Run("the switch is one-way, as the renderer's is", func(t *testing.T) {
		t.Parallel()
		w := (Measure{}).Observe(tea.ModeReportMsg{Mode: ansi.ModeUnicodeCore, Value: ansi.ModeSet})
		for _, v := range []ansi.ModeSetting{ansi.ModeNotRecognized, ansi.ModePermanentlyReset} {
			if got := w.Observe(tea.ModeReportMsg{Mode: ansi.ModeUnicodeCore, Value: v}).Method(); got != ansi.GraphemeWidth {
				t.Errorf("a later %v report dropped the measure back to %v; bubbletea never moves the "+
					"painter back, so neither may the Measure", v, got)
			}
		}
	})

	t.Run("Observe returns rather than mutates", func(t *testing.T) {
		t.Parallel()
		w := Measure{}
		_ = w.Observe(tea.ModeReportMsg{Mode: ansi.ModeUnicodeCore, Value: ansi.ModeSet})
		if got := w.Method(); got != ansi.WcWidth {
			t.Errorf("Observe mutated its receiver (now %v); it must return the new Measure", got)
		}
	})
}

// Each operation is the ansi helper of the same name with the Measure's method substituted for
// the hard-wired one: under GraphemeWidth it must equal ansi.X exactly, under WcWidth it must
// equal ansi.XWc exactly. Pinning the whole set against the library this way means an operation
// cannot quietly acquire behaviour of its own.
func TestMeasureOperationsMatchTheAnsiHelpers(t *testing.T) {
	t.Parallel()

	grapheme := Measure{method: ansi.GraphemeWidth}
	wc := Measure{}

	samples := []struct{ name, s string }{
		{"empty", ""},
		{"ascii", "the quick brown fox"},
		{"cjk", "日本語のテキスト"},
		{"emoji presentation", "🙂 done"},
		{"vs16", "a" + vs16Warning + "b"},
		{"styled", "\x1b[31mred\x1b[0m text"},
		{"pipes and hyphens", "| --- | --- |"},
	}
	limits := []int{0, 1, 2, 3, 5, 8, 20}

	for _, sample := range samples {
		t.Run(sample.name, func(t *testing.T) {
			t.Parallel()
			s := sample.s

			if got, want := grapheme.Width(s), ansi.StringWidth(s); got != want {
				t.Errorf("GraphemeWidth Width(%q) = %d, want ansi.StringWidth's %d", s, got, want)
			}
			if got, want := wc.Width(s), ansi.StringWidthWc(s); got != want {
				t.Errorf("WcWidth Width(%q) = %d, want ansi.StringWidthWc's %d", s, got, want)
			}

			for _, n := range limits {
				if got, want := grapheme.Truncate(s, n, "…"), ansi.Truncate(s, n, "…"); got != want {
					t.Errorf("GraphemeWidth Truncate(%q, %d) = %q, want ansi.Truncate's %q", s, n, got, want)
				}
				if got, want := wc.Truncate(s, n, "…"), ansi.TruncateWc(s, n, "…"); got != want {
					t.Errorf("WcWidth Truncate(%q, %d) = %q, want ansi.TruncateWc's %q", s, n, got, want)
				}
				if got, want := grapheme.Cut(s, 0, n), ansi.Cut(s, 0, n); got != want {
					t.Errorf("GraphemeWidth Cut(%q, 0, %d) = %q, want ansi.Cut's %q", s, n, got, want)
				}
				if got, want := wc.Cut(s, 0, n), ansi.CutWc(s, 0, n); got != want {
					t.Errorf("WcWidth Cut(%q, 0, %d) = %q, want ansi.CutWc's %q", s, n, got, want)
				}
				if n == 0 {
					continue // the wrappers divide by the limit; ansi's own helpers are undefined at 0
				}
				if got, want := grapheme.Wrap(s, n, ""), ansi.Wrap(s, n, ""); got != want {
					t.Errorf("GraphemeWidth Wrap(%q, %d) = %q, want ansi.Wrap's %q", s, n, got, want)
				}
				if got, want := wc.Wrap(s, n, ""), ansi.WrapWc(s, n, ""); got != want {
					t.Errorf("WcWidth Wrap(%q, %d) = %q, want ansi.WrapWc's %q", s, n, got, want)
				}
				if got, want := grapheme.Wordwrap(s, n, ""), ansi.Wordwrap(s, n, ""); got != want {
					t.Errorf("GraphemeWidth Wordwrap(%q, %d) = %q, want ansi.Wordwrap's %q", s, n, got, want)
				}
				if got, want := wc.Wordwrap(s, n, ""), ansi.WordwrapWc(s, n, ""); got != want {
					t.Errorf("WcWidth Wordwrap(%q, %d) = %q, want ansi.WordwrapWc's %q", s, n, got, want)
				}
				if got, want := grapheme.Hardwrap(s, n, true), ansi.Hardwrap(s, n, true); got != want {
					t.Errorf("GraphemeWidth Hardwrap(%q, %d) = %q, want ansi.Hardwrap's %q", s, n, got, want)
				}
				if got, want := wc.Hardwrap(s, n, true), ansi.HardwrapWc(s, n, true); got != want {
					t.Errorf("WcWidth Hardwrap(%q, %d) = %q, want ansi.HardwrapWc's %q", s, n, got, want)
				}
			}
		})
	}
}

// The disagreement the Measure exists to resolve, spelled out on one line rather than left to
// the parity test's indirection: under WcWidth — what an unquestioned terminal actually paints —
// "a⚠️b" is three columns and a two-column cut keeps the warning sign; under GraphemeWidth it is
// four and the same cut stops before it. A dependency bump that moves either column changes what
// the TUI paints and should fail here.
func TestMeasureMeasuresVS16TheWayItIsPainted(t *testing.T) {
	t.Parallel()

	const line = "a" + vs16Warning + "b"
	wc := Measure{}
	grapheme := Measure{method: ansi.GraphemeWidth}

	if got := wc.Width(line); got != 3 {
		t.Errorf("WcWidth Width(%q) = %d, want 3", line, got)
	}
	if got := grapheme.Width(line); got != 4 {
		t.Errorf("GraphemeWidth Width(%q) = %d, want 4", line, got)
	}
	if got, want := wc.Cut(line, 0, 2), "a"+vs16Warning; got != want {
		t.Errorf("WcWidth Cut(%q, 0, 2) = %q, want %q", line, got, want)
	}
	if got, want := grapheme.Cut(line, 0, 2), "a"; got != want {
		t.Errorf("GraphemeWidth Cut(%q, 0, 2) = %q, want %q", line, got, want)
	}
	if got, want := wc.Truncate(line, 3, ""), line; got != want {
		t.Errorf("WcWidth Truncate(%q, 3) = %q, want the whole line %q", line, got, want)
	}
	if got, want := grapheme.Truncate(line, 3, ""), "a"+vs16Warning; got != want {
		t.Errorf("GraphemeWidth Truncate(%q, 3) = %q, want %q", line, got, want)
	}
}

// A cut with a non-zero LEFT is what a mouse column is, and it is the one operation the
// Measure cannot delegate to ansi.Method: on the WcWidth branch — the painter's default —
// x/ansi@v0.11.7 binds cut's left-truncation to TruncateWc rather than TruncateLeftWc, so the
// library's own Cut spends left as a width rather than as an offset and hands back the first
// `left` columns — Cut("abcdef", 2, 5) is "ab", not "cde". The parity test above cannot see it:
// every case there cuts from column 0.
func TestMeasureCutsFromTheLeft(t *testing.T) {
	t.Parallel()

	const line = "abcdef"
	for _, w := range []Measure{Measure{}, {method: ansi.GraphemeWidth}} {
		if got, want := w.Cut(line, 2, 5), "cde"; got != want {
			t.Errorf("%v Cut(%q, 2, 5) = %q, want %q", w.Method(), line, got, want)
		}
		if got, want := w.Cut(line, 4, 99), "ef"; got != want {
			t.Errorf("%v Cut(%q, 4, 99) = %q, want the rest of the line %q", w.Method(), line, got, want)
		}
		if got := w.Cut(line, 3, 3); got != "" {
			t.Errorf("%v Cut(%q, 3, 3) = %q, want the empty span", w.Method(), line, got)
		}
	}

	// And on the grapheme the two measures disagree about: a click past the ⚠️ names the glyph
	// after it in the measure that is actually being painted, one column apart in the two.
	const vs16 = "a" + vs16Warning + "bc"
	if got, want := (Measure{}).Cut(vs16, 2, 3), "b"; got != want {
		t.Errorf("WcWidth Cut(%q, 2, 3) = %q, want %q — the ⚠️ paints one column there", vs16, got, want)
	}
	if got, want := (Measure{method: ansi.GraphemeWidth}).Cut(vs16, 3, 4), "b"; got != want {
		t.Errorf("GraphemeWidth Cut(%q, 3, 4) = %q, want %q — the ⚠️ paints two columns there", vs16, got, want)
	}
}

// The Measure rides on the Theme, which rides on a Bubble Tea model copied by value on every
// Update. So it must be a value all the way down: no pointer, no slice, no map, no channel, nothing
// whose copy shares state with its original. The walk is structural rather than behavioural so a
// field added later fails the guard even if no test happens to exercise it.
func TestMeasureSurvivesAValueCopy(t *testing.T) {
	t.Parallel()

	var walk func(typ reflect.Type, path string)
	walk = func(typ reflect.Type, path string) {
		switch typ.Kind() {
		case reflect.Bool,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64, reflect.String:
			// A plain scalar: a copy is wholly its own.
		case reflect.Struct:
			for i := range typ.NumField() {
				f := typ.Field(i)
				walk(f.Type, path+"."+f.Name)
			}
		case reflect.Array:
			walk(typ.Elem(), path+"[]")
		default:
			t.Errorf("Measure reaches a %s at %s — it rides on a value-copied model and must be "+
				"a plain value, so a copy shares nothing with its original", typ.Kind(), path)
		}
	}
	walk(reflect.TypeOf(Measure{}), "Measure")

	// And behaviourally: a copy that observes a report leaves the original measuring as it was.
	original := Measure{}
	adopted := original.Observe(tea.ModeReportMsg{Mode: ansi.ModeUnicodeCore, Value: ansi.ModeSet})
	if original.Method() != ansi.WcWidth || adopted.Method() != ansi.GraphemeWidth {
		t.Fatalf("copy is entangled: original %v, adopted %v", original.Method(), adopted.Method())
	}
	if got, want := original.Width(vs16Warning), 1; got != want {
		t.Errorf("the original now measures %q as %d, want %d", vs16Warning, got, want)
	}
	if got, want := adopted.Width(vs16Warning), 2; got != want {
		t.Errorf("the adopted copy measures %q as %d, want %d", vs16Warning, got, want)
	}
}

// sgrPattern matches one SGR sequence and captures its parameters.
var sgrPattern = regexp.MustCompile("\x1b\\[([0-9;:]*)m")

// isResetParameters reports whether SGR parameters turn every attribute off: none, or zeros only.
func isResetParameters(parameters string) bool {
	return strings.Trim(parameters, "0;") == ""
}

// styledVisibleRunes returns, for every visible non-space rune of s read on its own from a
// terminal with no style in effect, whether an SGR style is in effect where it paints. Spaces are
// left out: a break may swallow the space it falls at.
func styledVisibleRunes(s string) []bool {
	var styled []bool
	isOpen := false
	for len(s) > 0 {
		if match := sgrPattern.FindStringSubmatchIndex(s); match != nil && match[0] == 0 {
			isOpen = !isResetParameters(s[match[2]:match[3]])
			s = s[match[1]:]
			continue
		}
		r, size := utf8.DecodeRuneInString(s)
		if r != ' ' {
			styled = append(styled, isOpen)
		}
		s = s[size:]
	}
	return styled
}

// isStyleOpenAtEnd reports whether s, read from a terminal with no style in effect, leaves an SGR
// style in effect after its last byte.
func isStyleOpenAtEnd(s string) bool {
	matches := sgrPattern.FindAllStringSubmatch(s, -1)
	return len(matches) > 0 && !isResetParameters(matches[len(matches)-1][1])
}

// The upstream wrap passes SGR escapes through as word bytes and never closes them at a break, so
// a styled span wrapped at a space left its line with the style open and its continuation
// unstyled, and one wrapped at a hyphen left the line open with its reset carried onto the next
// line: an underline painted past the span's visible text. Every line wrapText returns must end
// with no style in effect, and each line read on its own must paint every character with the style
// it had before the wrap.
func TestWrapTextClosesAStyleAtEachBreakAndReopensIt(t *testing.T) {
	t.Parallel()

	th := testTheme()
	if !colorActive(th) {
		t.Skip("no-colour profile: there is no style to balance")
	}
	styles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"link", th.Link},
		{"underline", th.Underline},
		{"bold", th.Bold},
	}
	spans := []struct{ name, text string }{
		{"at a hyphen", "agents-md-no-block"},
		{"at a space", "alpha beta gamma"},
	}
	const limit = 10

	for _, pm := range paintMethods {
		for _, style := range styles {
			for _, span := range spans {
				t.Run(pm.name+"/"+style.name+"/"+span.name, func(t *testing.T) {
					t.Parallel()
					th := testTheme()
					th.Measure = Measure{method: pm.method}
					in := "see " + style.style.Render(span.text) + " now"

					lines := wrapText(th, in, limit)

					if len(lines) < 3 {
						t.Fatalf("wrapText(%q, %d) = %q; want the span broken over lines", in, limit, lines)
					}
					var painted []bool
					for i, line := range lines {
						if isStyleOpenAtEnd(line) {
							t.Errorf("line %d %q ends with a style in effect: %q", i, line, lines)
						}
						painted = append(painted, styledVisibleRunes(line)...)
					}
					if want := styledVisibleRunes(in); !reflect.DeepEqual(painted, want) {
						t.Errorf("lines %q paint styled %v, want %v as in %q", lines, painted, want, in)
					}
				})
			}
		}
	}
}

// Text with no style in it wraps exactly as the upstream wrap breaks it: balancing styles adds
// nothing to a line that has none.
func TestWrapTextLeavesUnstyledTextAsWrapped(t *testing.T) {
	t.Parallel()

	for _, pm := range paintMethods {
		for _, in := range []string{"see agents-md-no-block now", "see alpha beta gamma now"} {
			t.Run(pm.name+"/"+in, func(t *testing.T) {
				t.Parallel()
				th := testTheme()
				th.Measure = Measure{method: pm.method}

				got := wrapText(th, in, 10)

				if want := strings.Split(th.Measure.Wrap(in, 10, ""), "\n"); !reflect.DeepEqual(got, want) {
					t.Errorf("wrapText(%q, 10) = %q, want %q", in, got, want)
				}
			})
		}
	}
}
