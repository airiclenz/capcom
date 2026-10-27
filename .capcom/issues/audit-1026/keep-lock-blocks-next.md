---
title: finish --keep-lock then start on its own next issue is refused
status: done
type: bug
priority: 2
deps: [lock-purpose-alias-resume]
plan: 2026-10-27 - 00 - audit-p3-fixes-plan
created: 2026-10-26T20:06:49Z
closed: 2026-10-27T08:53:53Z
reason: Resolved by plan '2026-10-27 - 00 - audit-p3-fixes-plan' (items 13), archived in 0e1c89b
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

`finishLock` with `--keep-lock` only reads the lock (`internal/cli/finish.go:122-128`). `start b`
finds no token file for `b` (`internal/cli/start.go:88`) and is refused. `capcom start b --token <T>`
works, but nothing says so: the hint (`internal/cli/start.go:254`) names only `capcom lock status`.

`--keep-lock` exists so one session can do more work under one claim, but the output's own `next:`
line invites the command that then fails, and `finish` is the documented way to end a piece of work.

The ADR 0016 amendment of 2026-10-26 (`adr-0016-amend-1026`) decides the fix: the `next:` line names
`capcom start <next-id> --token <token>`, and that start moves the token file to the new purpose.
The move is `lock-purpose-alias-resume`, which this waits for.

## Acceptance

- `finish --keep-lock` prints the token, and its `next:` line names
  `capcom start <next-id> --token <token>`.
- That start exits 0 and leaves one token file, for `<next-id>`.
- `capcom start <other-id>` without `--token` after `--keep-lock` still fails `locked`, and its hint
  names `--token` alongside `capcom lock status`.
- A test runs `finish --keep-lock`, then the start its `next:` line names, and asserts exit 0 and a
  single token file.
