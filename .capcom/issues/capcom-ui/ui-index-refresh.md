---
title: Export an incremental refresh on an open index
status: open
type: task
priority: 2
created: 2026-10-15T20:03:23Z
---

`capcom ui` refreshes live by polling (design `docs/design/tui.md`, "Live refresh"), but the
index's incremental `refresh` is unexported: the only way to re-sync today is Close and re-Open,
which costs a full open per poll. Export a refresh on an open index that reports whether anything
changed, so a caller can poll cheaply and redraw only on change.

## Acceptance

- An open index exposes a refresh method returning whether any issue or Note was added, changed
  or removed.
- A refresh with no file changes reports no change and parses no file.
- Tests cover an added, changed and removed file each reporting a change, and the racy-clean case
  (mtime in the same second) still being re-parsed.
