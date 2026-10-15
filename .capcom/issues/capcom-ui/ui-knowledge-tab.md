---
title: Knowledge tab
status: open
type: feature
priority: 2
deps: [ui-issue-tree]
created: 2026-10-15T20:03:24Z
---

The Knowledge tab: the Note tree `capcom know tree` prints, using the tree widget from the Issues
tab, and the selected Note in the detail pane with its labels and refs. Retired Notes are hidden as
in `know tree`. The only write here is `$EDITOR` (separate issue).

## Acceptance

- The tree matches `capcom know tree` for the same store.
- The detail pane shows title, labels, refs and the rendered body.
- `capcom ui knowledge <id>` opens with that Note selected.
- Golden tests cover a nested tree and a Note with refs.
