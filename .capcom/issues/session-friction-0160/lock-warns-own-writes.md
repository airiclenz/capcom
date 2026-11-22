---
title: The lock warning fires on the holder's own writes when the lock is held for a plan
status: open
type: bug
priority: 1
labels: [agent-ergonomics]
plan: 2026-11-22 - 01 - session-friction-followups-plan
created: 2026-11-22T17:47:25Z
---

A capcom write warns "locked by <holder> (<purpose>) ...; if it is yours, carry on" unless it
writes the Issue whose id is the lock's purpose (ADR 0017, `app.lockWarning` and
`app.writesOnly` in `internal/cli/root.go`). In real sessions the purpose is almost never an
issue id: agents run `capcom lock acquire --purpose "<plan basename>"` once per plan (67
acquires against 2 `capcom start`), then create, log, update and close many issues and add
Notes under that lock. Every such write warns its own holder.

Seen: 46 warnings in the reviewed sessions, on `know add` (25), `create` (12), `log`,
`update`, `apply` and `close`. One apogee session was warned by `capcom apply` 29 seconds
after its own `capcom lock acquire --purpose "Filing <id> issue"`. Purposes seen: a plan
basename (`<date> - <nn> - <name>-plan`), the same with `.md`, and `write plan <basename>`.

Why: a warning that is false most of the time trains agents to ignore it, which defeats the
one case it exists for (see `lock-warning-ignored`).

Wanted: the holder's own writes stay silent. Options, to be decided in a plan and recorded
as an amendment to ADR 0017:
- Stay silent when the written Issue's `plan:` field names the lock's purpose (compare
  basenames, with or without `.md`).
- Stay silent when a kept token file (ADR 0016, `internal/lock/token.go`) for this
  worktree holds the token of the current lock record: a session here acquired it.
  ADR 0016 rejected a harness session id; this uses only the token file.
- A first-class plan lock, `capcom lock acquire --plan <basename>`, so the purpose is
  structured and Note writes made under a plan can be matched too.

Note writes target no Issue (`noteWrite` with no ids), so the `plan:` match alone leaves
`know add` and `know update` warning; they were the largest share.

## Acceptance

- A session that acquires the lock for a plan and then runs `create`, `log`, `update`,
  `close`, `apply`, `know add` and `know update` for that plan's work gets no lock warning.
- A write from a checkout that did not acquire the lock, or that targets other work, still
  warns.
- ADR 0017 and the Note `issue-workflow` say what the rule now is; tests cover both sides.

## Log

- 2026-11-22 Airic Lenz: Plan 2026-11-22 - 00 - lock-holder-and-agent-guide-plan (bf30b14) landed holder-process writes staying silent and renewing the heartbeat; open: proc.Anchor does not skip a capcom-named wrapper, so under a per-call shim (agenteval's capcom shim, Scoop's capcom.exe) the holder's later writes still warn.
