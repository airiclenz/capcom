---
title: "Light writes: claim, status, close, reopen, log"
status: open
type: feature
priority: 2
deps: [ui-skeleton, ui-ops-package]
created: 2026-10-15T20:03:24Z
---

The light writes on the selected issue, through the shared write path:

| Key | Action |
|---|---|
| `a` | claim (status `active`) |
| `s` | pick a status from the configured statuses |
| `c` | close; prompts for a reason, then names the issues the close made ready |
| `R` | reopen; prompts for a reason |
| `L` | add a log line |

Each write sends the ETag of the version on screen as if-match; on a conflict the UI reloads the
issue, says what changed and offers to run the action again. While another session holds this
worktree's Tree lock (live or stale), each write first asks "locked by <purpose>, write anyway?",
reading the lock at that moment (ADR 0013). The UI never takes the lock.

## Acceptance

- Each key performs the same change as the matching CLI command, verified by reading the file.
- Close shows the newly ready issues.
- A conflicting edit made between display and write is detected, reloaded, and retryable.
- With another session holding the lock, every write asks; no cancels with nothing written.
- Prompts submit on Enter, cancel on Esc; tests cover each write, a conflict and the lock prompt.
