---
title: "Mouse: click, scroll and links"
status: done
type: feature
priority: 2
deps: [ui-issue-tree, ui-markdown]
created: 2026-10-15T20:03:24Z
closed: 2026-10-16T10:28:25Z
reason: Resolved by the capcom-ui plan (docs/plans/archived/2026-10-16 - 00 - capcom-ui-plan.md, archived in 65127ff)
---

Mouse support across the UI: a click selects a row, a tab or a link; clicking a tree arrow expands
or collapses; the wheel scrolls the pane under the pointer. Hit-testing is new code; apogee's
`mouse.go` is tied to its own model, so borrow its patterns, and port `listCursor` from apogee's
`internal/tui/listsurface.go` for clickable lists.

## Acceptance

- Clicks on tabs, sidebar rows, tree arrows and detail-pane links do what the matching keys do.
- The wheel scrolls the sidebar or detail pane, whichever is under the pointer.
- Hit-testing stays correct in the narrow drill-down layout.
- Tests drive mouse messages at fixed coordinates.
