---
title: Move the write glue out of internal/cli into a shared ops package
status: done
type: task
priority: 2
created: 2026-10-15T20:03:23Z
closed: 2026-10-16T10:28:24Z
reason: Resolved by the capcom-ui plan (docs/plans/archived/2026-10-16 - 00 - capcom-ui-plan.md, archived in 65127ff)
---

The write glue for issues is private to `internal/cli` (`editIssue`, `editNode`, `closeResult`,
the log and dep helpers), so a second front end cannot reuse it. ADR 0012 moves it into a shared
package below `cli` (working name `internal/ops`) that both the CLI and `capcom ui` call, so a rule
added to a write lands in one place.

Each operation takes an open store and index and an optional if-match ETag, and returns a typed
result (new ETag; for close, the issues it made ready), never rendered text. Operations: set
status, claim, close, reopen, add a log line, plus the dep and node edits the CLI already does.

## Acceptance

- `internal/ops` (or the name chosen) exists; `internal/cli` calls it for update, close, reopen,
  log and dep, and keeps only flag parsing and output.
- Each operation accepts an if-match ETag and returns a conflict error when the file changed.
- CLI output and exit codes are unchanged: the existing `internal/cli` tests pass untouched.
- Direct tests exist only for cases the CLI cannot reach.
