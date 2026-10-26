---
title: finish --keep-lock then start on its own next issue is refused
status: open
type: bug
priority: 2
created: 2026-10-26T20:06:49Z
---

`capcom finish <id> --keep-lock` leaves the lock's purpose as the issue just closed, and the very
next command it recommends is then refused.

Reproduced:

    capcom finish a --reason done --keep-lock
      -> a  closed
         _main: held by ...: a; kept (--keep-lock)
         next: b  B
    capcom start b
      -> exit 3, "locked by ... (a) for 0s, heartbeat 0s ago"
         hint: capcom lock status

`--keep-lock` exists so one session can do more work under one claim, but the state it leaves is
indistinguishable from a dead holder, and the output's own `next:` line invites the command that
then fails.

This is the single most common dead end for an agent following capcom's own advice, because
`finish` is the documented way to end a piece of work.

## Acceptance

- `--keep-lock` either retitles the purpose to something resumable, or keeps a token file that
  `capcom start <next-id>` resumes without `--token` and without `--take-stale`.
- The refusal, when one remains, says how to proceed, not just `capcom lock status`.
- A test runs `finish --keep-lock` then `start` on the named next issue and asserts exit 0.
