---
title: Test the selection fill in the dependency view
status: open
type: task
priority: 2
labels: [ui, test]
plan: 2026-10-16 - 03 - ui-fill-tests-and-file-map-plan
created: 2026-10-16T17:22:19Z
---

Since the selection became a background fill, `TestTheme_SelectedRow_FillsTheBackgroundKeepingItsColours`
(internal/ui/theme_test.go) covers it only on the Ready and Issues tabs. The dependency view's
selected row draws through `theme.markOf` too (internal/ui/depview.go, `depRow.text`), but no test
renders the dependency view in a colour theme, so a dep-view row that drops the fill, or whose
status loses its category colour under it, would ship silently.

Wanted: a test that opens the dependency view in a colour theme and checks the selected row.

## Acceptance

- A test opens the dependency view (as `deps-both` does) under `theme: dark`, moves the selection
  onto a dependency row, and asserts every span of that row carries the selection fill
  (`48;2;<selection>`) while its status keeps its `status-<category>` foreground.
- The test fails when `depRow.text`'s selected row is rendered without `markOf`'s fill (checked
  by temporarily breaking it).
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/` passes.
