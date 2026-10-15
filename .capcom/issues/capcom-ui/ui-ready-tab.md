---
title: Ready tab
status: open
type: feature
priority: 2
deps: [ui-skeleton, ui-markdown]
created: 2026-10-15T20:03:24Z
---

The Ready tab, which `capcom ui` opens on: the queue `capcom ready` prints, most urgent first, via
`briefing.ReadyList`, with the selected issue's frontmatter header and markdown body in the detail
pane.

## Acceptance

- The sidebar lists the same issues in the same order as `capcom ready` for the same store.
- Selecting a row shows the issue: status, priority, labels, parent, deps, then the body.
- Golden tests cover a populated queue and an empty one.
