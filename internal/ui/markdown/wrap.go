package markdown

import (
	"strings"
)

// tabCells is how many spaces one TAB becomes when this package expands it: lipgloss's own tab
// width, so a line handed to a style with its tabs already expanded paints exactly what that
// style would have made of it.
const tabCells = 4

// wrapText word-wraps text to limit columns, hard-breaking any word longer than the limit and
// preserving the text's own newlines. An empty string yields a single empty line.
//
// It breaks in th.Measure, so the break is chosen in the same measure the cap below is enforced
// in and the painter draws in. No line it returns is wider than limit in that measure. The
// upstream wrap does not hold the cap on its own: its breakpoint branch lacks the full-line checks
// its default branch has, on the wcwidth and grapheme paths alike (x/ansi v0.11.7), so a run of
// breakpoints keeps growing a word onto an already-full line; a wrap of "| --- | --- | --- |" at
// limit 3 comes back with a five-cell first line. Every line over the limit is therefore
// hard-broken down to it. The one thing no break can divide is a single grapheme wider than the
// limit, a CJK glyph at limit 1, and that keeps a line to itself.
//
// A style in effect at a break is closed at the end of the line and reopened at the start of the
// next (balanceStyles), so no line it returns ends with an SGR style open. A followable link the
// wrap splits over lines is closed and reopened at each break the same way (balanceLinkMarkers),
// so every line it returns holds whole link-marker pairs.
func wrapText(th Theme, text string, limit int) []string {
	if limit < 1 {
		limit = 1
	}
	// Tabs are settled before the break is chosen: a tab the wrap counts as nothing is four cells
	// once a style has been past the line (expandTabs).
	text = expandTabs(text)
	wrapped := strings.Split(th.Measure.Wrap(text, limit, ""), "\n")
	out := make([]string, 0, len(wrapped))
	for _, line := range wrapped {
		if th.Measure.Width(line) <= limit {
			out = append(out, line)
			continue
		}
		// preserveSpace keeps this pass purely additive: it inserts breaks and drops nothing, so a
		// segment's leading indentation survives the cap. The break the hard wrap opens ahead of
		// an over-wide leading grapheme would otherwise surface as a blank row.
		segments := strings.Split(th.Measure.Hardwrap(line, limit, true), "\n")
		if len(segments) > 1 && segments[0] == "" {
			segments = segments[1:]
		}
		out = append(out, segments...)
	}
	return balanceLinkMarkers(balanceStyles(out))
}

// sgrReset is the SGR sequence balanceStyles closes a line's open style with.
const sgrReset = "\x1b[m"

// balanceStyles makes every line end with no SGR style in effect: the SGR sequences still in
// effect at the end of a line are followed there by a reset and repeated at the start of the next
// line, so a styled span the wrap splits over lines paints on each of its lines and on nothing
// after them. lines is rewritten in place and returned; a line with no SGR sequence in effect at
// either end comes back byte for byte.
//
// The upstream wrap passes SGR sequences through as word bytes and never closes one at a break
// (x/ansi v0.11.7): a span broken at a space ends its line open, and one broken at a hyphen ends
// it open too, its reset carried onto the next line ahead of the next character's sequence. An
// open style runs past the line's visible text into whatever the frame paints after it.
func balanceStyles(lines []string) []string {
	carried := ""
	for i, line := range lines {
		line = carried + line
		carried = sgrInEffectAtEnd(line)
		if carried != "" {
			line += sgrReset
		}
		lines[i] = line
	}
	return lines
}

// sgrInEffectAtEnd returns the SGR sequences of line still in effect at its end, concatenated in
// order: every SGR sequence after the last reset (a sequence with no parameter or only zeros),
// and the empty string when the line ends with no style in effect. Escapes other than a CSI
// sequence, a link marker among them, are not SGR and are passed over.
func sgrInEffectAtEnd(line string) string {
	var effect strings.Builder
	for from := 0; ; {
		at := strings.Index(line[from:], sgrIntro)
		if at < 0 {
			return effect.String()
		}
		start := from + at
		end, final := csiEnd(line, start+len(sgrIntro))
		from = end
		if final != sgrFinal {
			continue
		}
		sequence := line[start:end]
		if isSGRReset(sequence) {
			effect.Reset()
			continue
		}
		effect.WriteString(sequence)
	}
}

// sgrIntro opens every CSI sequence, and sgrFinal is the final byte that makes one an SGR.
const (
	sgrIntro = "\x1b["
	sgrFinal = 'm'
)

// csiEnd returns the byte just past the CSI sequence whose parameters start at from, and its final
// byte; a sequence cut off by the end of s ends there with final 0.
func csiEnd(s string, from int) (int, byte) {
	for i := from; i < len(s); i++ {
		// Parameter (0x30-0x3F) and intermediate (0x20-0x2F) bytes run until the final byte.
		if c := s[i]; c >= 0x40 && c <= 0x7e {
			return i + 1, c
		}
	}
	return len(s), 0
}

// isSGRReset reports whether the SGR sequence turns every attribute off: it carries no parameter,
// or zeros only.
func isSGRReset(sequence string) bool {
	parameters := sequence[len(sgrIntro) : len(sequence)-1]
	return strings.Trim(parameters, "0;") == ""
}

// hangCollapses reports whether a block width columns wide is too narrow to hold an mw-column
// hanging marker and one column of text beside it. A marker is shed whole there, never squeezed:
// prepending it to text floored at one column would compose a line wider than the block.
func hangCollapses(width, mw int) bool { return width < mw+1 }

// hangingPrefixes word-wraps text to the width left of marker and prepends the marker to the
// first line and a blank indent of the same width to the rest, so a wrapped block stays aligned
// under its marker. A block too narrow to hold the marker plus one text column drops the marker
// and the indent and wraps the text flat at the block's full width (hangCollapses).
func hangingPrefixes(th Theme, marker, text string, width int) []string {
	mw := th.Measure.Width(marker)
	if hangCollapses(width, mw) {
		return wrapText(th, text, width)
	}
	indent := strings.Repeat(" ", mw)
	lines := wrapText(th, text, width-mw)
	out := make([]string, len(lines))
	for i, line := range lines {
		if i == 0 {
			out[i] = marker + line
		} else {
			out[i] = indent + line
		}
	}
	return out
}

// expandTabs replaces every TAB in s with the spaces a lipgloss style would otherwise put there.
//
// A TAB is zero cells to the measure and to the painter, but every lipgloss.Style.Render rewrites
// it into tabCells spaces after the measure has counted the line and before the painter sees it,
// so a styled line with tabs would paint wider than it was measured. Expanding before anything
// measures puts the three back in step.
func expandTabs(s string) string {
	if !strings.Contains(s, "\t") {
		return s
	}
	return strings.ReplaceAll(s, "\t", strings.Repeat(" ", tabCells))
}

// blankLine reports whether line carries nothing visible: it is empty or whitespace only.
func blankLine(line string) bool {
	return strings.TrimSpace(line) == ""
}
