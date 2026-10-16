package markdown

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Measure is the one display-width measure the renderer uses, and it is the painter's.
//
// Two measures of how many terminal columns a string occupies are in play in a Bubble Tea
// program, and they disagree. The layout side measures grapheme clusters: lipgloss.Width and the
// package-level ansi helpers (StringWidth, Cut, Truncate, Wrap, Wordwrap, Hardwrap) are hard-wired
// to ansi.GraphemeWidth, which promotes any cluster carrying VARIATION SELECTOR-16 (U+FE0F) to two
// cells, so "⚠️" is two. The paint side is the screen buffer Bubble Tea draws the view into, and it
// starts at ansi.WcWidth (the zero value of ansi.Method), which takes the first non-zero-width rune
// of the cluster, so the same "⚠️" is one cell. Bubble Tea moves the painter to ansi.GraphemeWidth
// only when the terminal answers its start-up mode-2027 (Unicode core) query. A frame laid out in
// one measure and painted in the other is off by a column per such grapheme: a row composed to
// exactly the pane width overruns it, and a mouse column names a different rune than the one under
// the pointer.
//
// Measure follows the painter: it starts where the painter starts (ansi.WcWidth) and adopts
// ansi.GraphemeWidth on the same message the painter does (Observe). It is a plain value, safe
// inside a value-copied Bubble Tea model, and its zero value is the painter's starting measure.
type Measure struct {
	// method is the measure every operation dispatches on; its zero value is ansi.WcWidth.
	method ansi.Method
}

// Method reports the measure m dispatches on: the same value the painter's screen buffer holds.
// Production code calls the operations below rather than branch on it; it exists so a test can
// check which measure is in force.
func (m Measure) Method() ansi.Method { return m.method }

// Observe folds a terminal mode report into m and returns the measure to use from then on; m
// itself is unchanged.
//
// It mirrors Bubble Tea's own reaction to the same message: on a mode-2027 (Unicode core) report
// whose setting is set, reset or permanently set, Bubble Tea switches its renderer to
// ansi.GraphemeWidth and then hands the message on to the model's Update, which is how a model
// learns what the painter decided. Any other report leaves the measure alone. The switch is
// one-way, as the renderer's is.
func (m Measure) Observe(report tea.ModeReportMsg) Measure {
	if report.Mode != ansi.ModeUnicodeCore {
		return m
	}
	switch report.Value {
	case ansi.ModeSet, ansi.ModeReset, ansi.ModePermanentlySet:
		m.method = ansi.GraphemeWidth
	}
	return m
}

// The operations below each stand in for the package-level ansi function of the same name, with
// the same signature and semantics; the only difference is that the method is m's rather than a
// hard-wired ansi.GraphemeWidth.

// Width reports how many terminal columns s occupies when painted. ANSI escape sequences count
// for nothing and wide glyphs count for two. It replaces lipgloss.Width and ansi.StringWidth.
func (m Measure) Width(s string) int {
	return m.method.StringWidth(s)
}

// Truncate cuts s down to length columns, appending tail when it had to cut. It replaces
// ansi.Truncate.
func (m Measure) Truncate(s string, length int, tail string) string {
	return m.method.Truncate(s, length, tail)
}

// Cut returns the columns [left, right) of s, keeping the styling of the parts it slices through
// intact. It replaces ansi.Cut.
//
// It composes the two truncations itself rather than calling ansi.Method.Cut: x/ansi v0.11.7's
// cut binds its left truncation to TruncateWc, a right truncation, on the WcWidth branch, so
// ansi.Method.Cut(s, left, right) under WcWidth spends left as a width rather than an offset:
// Cut("abcdef", 2, 5) returns "ab" where the span is "cde". WcWidth is the painter's default, and
// a mouse column is a cut with a non-zero left. TestMeasureCutsFromTheLeft pins the correct
// behaviour; if a dependency bump fixes it upstream, this can go back to delegating.
func (m Measure) Cut(s string, left, right int) string {
	if right <= left {
		return ""
	}
	cut := m.method.Truncate(s, right, "")
	if left <= 0 {
		return cut
	}
	return m.method.TruncateLeft(cut, left, "")
}

// Wrap wraps s to limit columns, breaking at spaces and at breakpoints and hard-breaking a word
// too long to fit. It replaces ansi.Wrap.
func (m Measure) Wrap(s string, limit int, breakpoints string) string {
	return m.method.Wrap(s, limit, breakpoints)
}

// Wordwrap wraps s to limit columns at word boundaries only, leaving a word longer than the limit
// over-long. It replaces ansi.Wordwrap.
func (m Measure) Wordwrap(s string, limit int, breakpoints string) string {
	return m.method.Wordwrap(s, limit, breakpoints)
}

// Hardwrap breaks s at exactly limit columns regardless of word boundaries. It replaces
// ansi.Hardwrap.
func (m Measure) Hardwrap(s string, limit int, preserveSpace bool) string {
	return m.method.Hardwrap(s, limit, preserveSpace)
}
