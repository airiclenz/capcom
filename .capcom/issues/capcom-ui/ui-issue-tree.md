---
title: Issues tab with a collapsible tree
status: done
type: feature
priority: 2
deps: [ui-skeleton, ui-markdown]
created: 2026-10-15T20:03:24Z
closed: 2026-10-16T10:28:25Z
reason: Resolved by the capcom-ui plan (docs/plans/archived/2026-10-16 - 00 - capcom-ui-plan.md, archived in 65127ff)
---

The Issues tab: the outline `capcom list --tree` prints, as a collapsible tree in the sidebar with
each row's status and priority, and the selected issue in the detail pane. Closed issues follow the
CLI's default visibility. The tree widget is new code (apogee has none) and the Knowledge tab
reuses it.

## Acceptance

- The tree shows the same nodes in the same order as `capcom list --tree`.
- `→`/`l` expands, `←`/`h` collapses or moves to the parent; expanded state survives tab switches.
- `capcom ui issues <id>` opens with that issue selected and its ancestors expanded; an alias
  resolves as `show` resolves it.
- Golden tests cover a nested tree, collapsed and expanded.
