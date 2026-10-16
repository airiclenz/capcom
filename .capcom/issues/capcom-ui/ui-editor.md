---
title: Open the selected file in $EDITOR
status: done
type: feature
priority: 2
deps: [ui-writes]
created: 2026-10-15T20:03:24Z
closed: 2026-10-16T10:28:25Z
reason: Resolved by the capcom-ui plan (docs/plans/archived/2026-10-16 - 00 - capcom-ui-plan.md, archived in 65127ff)
---

`e` opens the selected Issue or Note file in `$VISUAL`, else `$EDITOR`. The UI suspends while the
editor runs, then runs the `capcom check` checks on the store and shows any problem in the status
bar. While another session holds the Tree lock it asks first, as the other writes do (ADR 0013).

## Acceptance

- The editor gets the file's path; the UI restores the terminal and redraws after it exits.
- A problem `capcom check` would report appears after the edit.
- No editor configured shows a message instead of failing.
- The lock prompt appears when another session holds the lock.
- Tests use a fake editor command that edits the file.
