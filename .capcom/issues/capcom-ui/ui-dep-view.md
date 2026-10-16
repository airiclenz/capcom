---
title: Dependency view for the selected issue
status: done
type: feature
priority: 2
deps: [ui-issue-tree]
created: 2026-10-15T20:03:24Z
closed: 2026-10-16T10:28:25Z
reason: Resolved by the capcom-ui plan (docs/plans/archived/2026-10-16 - 00 - capcom-ui-plan.md, archived in 65127ff)
---

For the selected issue, show what it waits on and what it blocks, both sides as `capcom dep list`
reports them, each row a link that selects that issue. This is how a supervisor sees why an issue
is not ready.

## Acceptance

- The detail pane (or a toggle within it) lists dependencies and dependents of the selected issue,
  matching `capcom dep list <id>`.
- Enter on a row, or a click, selects that issue in the Issues tab.
- Tests cover an issue with both sides, and one with neither.
