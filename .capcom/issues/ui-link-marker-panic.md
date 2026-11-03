---
title: A forged link marker in a body crashes capcom ui
status: done
type: bug
priority: 1
plan: 2026-11-03 - 00 - audit-urgent-fixes-plan
created: 2026-11-03T08:23:42Z
closed: 2026-11-03T09:20:04Z
reason: Resolved by plan 2026-11-03 - 00 - audit-urgent-fixes-plan item 2 (archived in 1dcb36a)
---

The terminal UI's markdown renderer trusts any `ESC _capcom-link:<n> ESC \` marker it finds in
a rendered line, but issue and Note bodies reach `markdown.Render` raw
(`internal/ui/detail.go:345`). `nextLinkMarker` (`internal/ui/markdown/links.go:184`) accepts a
negative index through `strconv.Atoi`, and `stripLinkMarkers` (`links.go:254`) guards only
`open == noLink` and `open >= len(targets)`, so `targets[-5]` panics at `links.go:259`. Nothing
in `internal/ui` recovers. A forged non-negative index also turns visible text into a followable
link to an unrelated id.

Wanted: reject `index < 0 || index >= len(targets)` in `nextLinkMarker`, `stripLinkMarkers` and
`openLinkAtEnd`, treating it as a foreign marker, and strip ESC and the other C0/APC control
bytes (keeping tab and newline) from source lines at the top of `Render`, so only the renderer
can create markers.

Source: docs/reviews/code-audit-2026-11-03.md.

## Why

A hostile repository or pull request commits a body such as
`\x1b_capcom-link:-5\x1b\\x\x1b_capcom-link/\x1b\\`; opening that issue or Note in `capcom ui`,
or restoring it from `ui-state.yaml`, kills the UI every time. Crash only, no data loss.

## Acceptance

- A `Render` test with a forged open marker around visible text, once with a negative and once with an out-of-range index, does not panic and reports no Link.
- A body holding a forged marker with an in-range index renders no followable link.
- Control bytes other than tab and newline in a body never reach the rendered output.
