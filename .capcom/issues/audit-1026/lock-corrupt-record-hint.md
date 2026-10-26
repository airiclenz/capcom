---
title: A corrupt lock record wedges every lock command and names no fix
status: open
type: bug
priority: 2
plan: 2026-10-26 - 03 - audit-p1-p2-fixes-plan
created: 2026-10-26T20:06:45Z
---

A corrupt or token-less lock record wedges every lock command, and no message names the command
that fixes it.

Reproduced with `.git/capcom/locks/_main.json` holding `not json{{{`:

    capcom lock check            -> exit 1, hint: capcom lock check --help
    capcom lock touch --token x  -> exit 1, hint: capcom lock touch --help
    capcom lock release --token x-> exit 1, hint: capcom lock release --help
    capcom lock acquire --purpose p -> exit 1, hint: capcom lock acquire --help
    capcom start <id>            -> exit 1, the same generic hint
    capcom lock status           -> exit 0, "_main: invalid" + warning   (this one is right)

`read` returns `errCorruptRecord` (`internal/lock/lock.go:402-406`), which `emitLock`
(`internal/cli/lock.go:330`) and `start` (`internal/cli/start.go:248`) report as a generic
`invalid` with the command's own `--help` as hint. `finish` already gets this right (`run capcom
lock status, then capcom lock break to free it`), so the wording exists to copy. `lock break`
already clears such a record.

The code stays `invalid` with exit 1: no session holds the record, so `locked` would misreport it,
and a hook gating on a non-zero exit already blocks edits. What is missing is the way out.

## Acceptance

- `lock check`, `lock acquire`, `lock touch`, `lock release` and `start`, meeting an unreadable or
  token-less record, fail `invalid` with exit 1, a message saying the lock record cannot be read,
  and a hint naming `capcom lock status`, then `capcom lock break`.
- `lock status` keeps exit 0 and its `invalid` line; `lock break` keeps clearing the record.
- `--json` carries the same code, message and hint.
- A test corrupts the record and asserts the five commands agree.
