---
title: Open the selected file in $EDITOR
status: open
type: feature
priority: 2
deps: [ui-writes]
created: 2026-10-15T20:03:24Z
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
