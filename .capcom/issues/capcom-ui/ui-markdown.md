---
title: Markdown renderer for the UI, ported from apogee, with links
status: done
type: feature
priority: 2
deps: [ui-skeleton]
created: 2026-10-15T20:03:24Z
closed: 2026-10-16T10:28:24Z
reason: Resolved by the capcom-ui plan (docs/plans/archived/2026-10-16 - 00 - capcom-ui-plan.md, archived in 65127ff)
---

The detail pane renders Issue and Note bodies as markdown. apogee already has a renderer that fits
a TUI: copy `internal/tui/markdown.go` and `internal/tui/mdtable.go` from apogee with the wrap and
width helpers they use, replacing apogee's `theme` with a small capcom one (design
`docs/design/tui.md`, "Taken from apogee").

The copy handles headings, fenced code, bullet and numbered lists, GFM pipe tables, inline code and
bold, but not links or blockquotes; add both. Links cover markdown links plus issue ids, Note ids
and refs, and expose their screen regions so a click can follow them.

## Acceptance

- The renderer lives in `internal/ui` (or a subpackage) with apogee's table tests ported.
- Links and blockquotes render, with tests.
- Rendering reports each link's target and region for hit-testing.
- Widths are measured in terminal cells (wide characters and ANSI sequences handled).
