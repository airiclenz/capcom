---
title: Test the selection fill on the Knowledge tab
status: done
type: task
priority: 2
labels: [ui, test]
plan: 2026-10-16 - 03 - ui-fill-tests-and-file-map-plan
created: 2026-10-16T17:22:19Z
closed: 2026-10-16T19:07:35Z
reason: closed by plan 2026-10-16 - 03 - ui-fill-tests-and-file-map-plan (bf3fb87), item 2
---

Since the selection became a background fill, `TestTheme_SelectedRow_FillsTheBackgroundKeepingItsColours`
(internal/ui/theme_test.go) covers it only on the Ready and Issues tabs. The Knowledge tab's
selected row draws through the same `theme.markOf` path (internal/ui/tree.go, `noteTreeRow`), but
the theme fixture holds no Notes ("No Notes."), so no test catches a Knowledge row that drops
the fill or loses its colours under it. A regression there would ship silently.

Wanted: a test that renders the Knowledge tab with at least one Note, in a colour theme, and
checks the selected row.

## Acceptance

- A test opens the Knowledge tab with Notes present under `theme: dark` and asserts every span of
  the selected row carries the selection fill (`48;2;<selection>`), the tree connector does not
  (the fill starts after it, per docs/design/tui.md), and a muted (revealed) selected row is
  faint over the fill.
- The test fails when `noteTreeRow`'s selected row is rendered without `markOf`'s fill (checked
  by temporarily breaking it).
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/` passes.
