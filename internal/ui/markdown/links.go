package markdown

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Links. A followable link is one whose target is an issue or Note id, as Options.IsID decides:
// a markdown link [text](id), or an inline-code span whose whole text is an id. It is painted in
// the Link style and Render reports where it landed on screen. Every other link target (a URL, a
// file path, an anchor) and every <scheme://...> autolink is painted in the Inert style and
// reported nowhere.
//
// Where a link lands is only known once the block has been wrapped, padded and prefixed, so the
// inline pass brackets each followable link's styled text with a pair of zero-width markers: APC
// escape sequences, which every measure and every wrap in x/ansi passes through as zero cells and
// copies verbatim. wrapText closes a link at the end of each line it spans and reopens it at the
// start of the next, so every physical line carries complete pairs whatever is later composed
// around it (a table's columns, a list's hang, a quote's bars). Render then reads each pair's
// column and width off its finished line and strips the markers before returning.

// Marker spellings. An open marker carries the index of its target in linkSet.targets; the close
// marker carries none, since links never nest.
const (
	linkMarkerIntro  = "\x1b_capcom-link"
	linkMarkerEnd    = "\x1b\\"
	linkOpenPayload  = ":"
	linkClosePayload = "/"
	linkCloseMarker  = linkMarkerIntro + linkClosePayload + linkMarkerEnd
)

// noLink is the open-link state of a line position outside every link.
const noLink = -1

// Options tunes what Render recognises beyond the markdown itself.
type Options struct {
	// IsID reports whether s is the id of an existing issue or Note, and so a followable link
	// target. A nil IsID treats nothing as an id: no link is followable and Rendered.Links is
	// empty.
	IsID func(s string) bool
}

// Rendered is Render's result: the styled physical lines and every followable link on them.
type Rendered struct {
	// Lines are the styled lines, none wider than the width Render was given.
	Lines []string
	// Links are the followable links' screen regions, in reading order. A link wrapped over
	// several lines has one region per line it occupies.
	Links []Link
}

// Link is one screen region of a followable link: the cells Width wide starting at column Col of
// line Line in Rendered.Lines (both zero-based), measured in the Theme's Measure. The region
// covers the link's visible text only, never the spaces a wrap left at either end of it.
type Link struct {
	// Target is the issue or Note id the link names.
	Target string
	// Line is the index of the line the region is on.
	Line int
	// Col is the region's first column.
	Col int
	// Width is the region's width in cells; it is always at least 1.
	Width int
}

// linkSet is one Render call's link state: the id test and the targets marked so far, indexed by
// the open markers that name them. A nil linkSet is valid and makes nothing followable.
type linkSet struct {
	isID    func(string) bool
	targets []string
}

// isFollowable reports whether target names an id, and so is rendered as a followable link.
func (ls *linkSet) isFollowable(target string) bool {
	return ls != nil && ls.isID != nil && ls.isID(target)
}

// mark records target and returns styled bracketed by the markers that name it.
func (ls *linkSet) mark(target, styled string) string {
	ls.targets = append(ls.targets, target)
	return linkOpenMarker(len(ls.targets)-1) + styled + linkCloseMarker
}

// linkOpenMarker returns the open marker naming the target at index.
func linkOpenMarker(index int) string {
	return linkMarkerIntro + linkOpenPayload + strconv.Itoa(index) + linkMarkerEnd
}

// renderCodeSpan renders an inline-code span: as a followable link when its whole text is an id,
// otherwise in the Code style.
func renderCodeSpan(th Theme, links *linkSet, text string) string {
	if links.isFollowable(text) {
		return links.mark(text, th.Link.Render(text))
	}
	return th.Code.Render(text)
}

// matchLink reports whether a markdown link [text](target) opens at s[i], returning its styled
// text and the index just past its closing parenthesis. The text is shown literally, with no
// inline markup parsed inside it, and an empty text shows the target instead. A title after the
// target ([text](target "title")) and angle brackets around it are dropped. The text may hold no
// bracket and the target no parenthesis; anything else is not a link and stays literal.
func matchLink(th Theme, links *linkSet, s string, i int) (styled string, next int, ok bool) {
	textEnd := strings.Index(s[i+1:], "](")
	if textEnd < 0 {
		return "", 0, false
	}
	text := s[i+1 : i+1+textEnd]
	if strings.ContainsAny(text, "[]") {
		return "", 0, false
	}
	targetStart := i + 1 + textEnd + len("](")
	targetEnd := strings.IndexByte(s[targetStart:], ')')
	if targetEnd < 0 {
		return "", 0, false
	}
	target := linkTarget(s[targetStart : targetStart+targetEnd])
	if target == "" {
		return "", 0, false
	}

	if text == "" {
		text = target
	}
	next = targetStart + targetEnd + 1
	if links.isFollowable(target) {
		return links.mark(target, th.Link.Render(text)), next, true
	}
	return th.Inert.Render(text), next, true
}

// linkTarget returns the destination of a link's parenthesised part: its first field, unwrapped
// from <...>, with any title after it dropped.
func linkTarget(raw string) string {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(fields[0], "<"), ">")
}

// matchAutolink reports whether an autolink <scheme://...> opens at s[i], returning the URL in the
// Inert style without its angle brackets and the index just past the closing >. An autolink holds
// no space and no <, and an autolink is never followable: it is a URL, not an id.
func matchAutolink(th Theme, s string, i int) (styled string, next int, ok bool) {
	end := strings.IndexByte(s[i+1:], '>')
	if end < 0 {
		return "", 0, false
	}
	url := s[i+1 : i+1+end]
	if !strings.Contains(url, "://") || strings.ContainsAny(url, " \t<") {
		return "", 0, false
	}
	return th.Inert.Render(url), i + 1 + end + 1, true
}

// linkMarker is one marker found in a line: the byte span it occupies and the target index it
// opens, or noLink for a close marker.
type linkMarker struct {
	start  int
	end    int
	target int
}

// nextLinkMarker finds the first marker in s at or after byte from.
func nextLinkMarker(s string, from int) (linkMarker, bool) {
	at := strings.Index(s[from:], linkMarkerIntro)
	if at < 0 {
		return linkMarker{}, false
	}
	start := from + at
	payloadStart := start + len(linkMarkerIntro)
	payloadEnd := strings.Index(s[payloadStart:], linkMarkerEnd)
	if payloadEnd < 0 {
		return linkMarker{}, false
	}
	payload := s[payloadStart : payloadStart+payloadEnd]
	marker := linkMarker{start: start, end: payloadStart + payloadEnd + len(linkMarkerEnd), target: noLink}
	if payload == linkClosePayload {
		return marker, true
	}
	index, err := strconv.Atoi(strings.TrimPrefix(payload, linkOpenPayload))
	if err != nil || !strings.HasPrefix(payload, linkOpenPayload) {
		// Not a marker this package wrote; the scan resumes after it.
		return nextLinkMarker(s, marker.end)
	}
	marker.target = index
	return marker, true
}

// balanceLinkMarkers makes every wrapped line carry complete marker pairs: a link still open at
// the end of a line is closed there and reopened at the start of the next, so a link wrapped over
// lines becomes one pair per line. lines is rewritten in place and returned.
func balanceLinkMarkers(lines []string) []string {
	open := noLink
	for i, line := range lines {
		if open != noLink {
			line = linkOpenMarker(open) + line
		}
		open = openLinkAtEnd(line)
		if open != noLink {
			line += linkCloseMarker
		}
		lines[i] = line
	}
	return lines
}

// openLinkAtEnd returns the target index of the link still open at the end of line, or noLink.
func openLinkAtEnd(line string) int {
	open := noLink
	for from := 0; ; {
		marker, ok := nextLinkMarker(line, from)
		if !ok {
			return open
		}
		open = marker.target
		from = marker.end
	}
}

// collectLinks strips the markers from every line and returns the lines with the region of each
// marker pair, its target looked up in targets.
func collectLinks(th Theme, lines []string, targets []string) Rendered {
	rendered := Rendered{Lines: make([]string, len(lines))}
	for i, line := range lines {
		plain, links := stripLinkMarkers(th, line, i, targets)
		rendered.Lines[i] = plain
		rendered.Links = append(rendered.Links, links...)
	}
	return rendered
}

// stripLinkMarkers returns line without its markers, and the region of every marker pair in it
// as line number lineIndex. A pair whose text is blank yields no region.
func stripLinkMarkers(th Theme, line string, lineIndex int, targets []string) (string, []Link) {
	var b strings.Builder
	var links []Link
	open, openAt := noLink, 0
	for from := 0; ; {
		marker, ok := nextLinkMarker(line, from)
		if !ok {
			b.WriteString(line[from:])
			return b.String(), links
		}
		b.WriteString(line[from:marker.start])
		from = marker.end
		if marker.target != noLink {
			open, openAt = marker.target, b.Len()
			continue
		}
		if open == noLink || open >= len(targets) {
			continue
		}
		prefix := b.String()
		if link, ok := linkRegion(th, prefix[:openAt], prefix[openAt:]); ok {
			link.Target, link.Line = targets[open], lineIndex
			links = append(links, link)
		}
		open = noLink
	}
}

// linkRegion measures a link segment that follows before on its line: its column and width, less
// any spaces at either end of its visible text. ok is false for a segment with no visible text.
func linkRegion(th Theme, before, segment string) (Link, bool) {
	text := ansi.Strip(segment)
	trimmed := strings.TrimLeft(text, " ")
	lead := len(text) - len(trimmed)
	trimmed = strings.TrimRight(trimmed, " ")
	width := th.Measure.Width(trimmed)
	if width == 0 {
		return Link{}, false
	}
	return Link{Col: th.Measure.Width(before) + lead, Width: width}, true
}
