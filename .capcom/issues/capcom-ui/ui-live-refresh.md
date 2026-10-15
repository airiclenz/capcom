---
title: Live refresh by polling the index
status: open
type: feature
priority: 2
deps: [ui-skeleton, ui-index-refresh]
created: 2026-10-15T20:03:24Z
---

`capcom ui` exists to be watched while agents work, so it must update on its own. Poll the index
refresh about once a second and redraw when anything changed (design `docs/design/tui.md`, "Live
refresh"). No file watcher.

## Acceptance

- A change to an issue or Note file by another process appears within about two seconds.
- Selection, expanded nodes and scroll position survive a refresh; when the selected node vanished,
  the selection moves to its parent or nearest sibling.
- Polling with no changes does no visible work (no redraw, no file parse).
- Tests drive refresh with a fake clock and cover a change, a removal of the selected node, and no
  change.
