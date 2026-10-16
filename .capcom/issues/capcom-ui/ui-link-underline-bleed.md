---
title: A link wrapped at a hyphen bleeds its underline across the screen
status: open
type: bug
priority: 2
plan: 2026-10-16 - 06 - ui-wrap-styles-and-lock-pad-plan
created: 2026-10-16T19:41:57Z
---

When a link in the detail pane's markdown is wrapped over two lines at a hyphen, its underline is
not ended at the end of the first line. Seen in the Ready tab on issue `phase-5-capcom`:
`(agents-md-no-block)` wraps as `agents-md-` / `no-block`, and

- the underline runs on past `agents-md-` through the padding to the pane's right edge, and
- the next screen row starts underlined, so the left list pane paints an underline across its
  whole width on that row, though it has no link there.

So an underline SGR stays open past the end of the wrapped line and leaks into whatever the
terminal paints next. The wrap is meant to be SGR-aware and close and reopen a link at each break
(`wrapText` and `balanceLinkMarkers` in `internal/ui/markdown/wrap.go` and
`internal/ui/markdown/links.go`); the hyphen breakpoint path looks like the one that misses it.
The same leak would hit `<u>` underline, bold or any styled span split at a hyphen.

Why it matters: the bleed paints a stray line across the issue list, which reads as a divider or
selection that is not there.

## Acceptance

- A link, `<u>` span or bold span wrapped at a hyphen (and at a space) leaves every wrapped line
  with its styles closed at the line's end and reopened on the next line.
- In the UI, no underline appears past a link's visible text, and none in the list pane.
- A test in `internal/ui/markdown` wraps a hyphenated link over two lines and asserts no line
  ends with an open SGR style.
