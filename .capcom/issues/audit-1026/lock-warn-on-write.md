---
title: A CLI write while another session holds the lock is silent
status: open
type: feature
priority: 2
deps: [lock-check-writes-file]
plan: 2026-10-27 - 00 - audit-p3-fixes-plan
created: 2026-10-26T20:13:21Z
---

A mutating command writes to the worktree even when another session holds the Tree lock, and says
nothing. The lock is how agents share one worktree, but only `capcom ui` reacts to it, so the CLI
record is a sign on the door that most commands do not read.

Reproduced: one session takes the lock, then, in the same worktree,

    capcom lock acquire --purpose other-session   -> lock taken
    capcom create --id sneaky --title Sneaky --body b   -> exit 0, file written
    capcom mv sneaky sneaky2                            -> exit 0, file moved
    capcom lock check                                   -> exit 3, "locked by ... (other-session)"

So `lock check` correctly reports the worktree as taken while `create` and `mv` write anyway. One
agent with a half-finished multi-file change, and a second agent running a command, now hold
overlapping work that neither of them caused on purpose. No mutating command reads the lock today;
only `start`, `finish`, `prime`, `lock` and the UI import `internal/lock`.

ADR 0017 decides the behaviour: warn, never refuse, and tell the holder apart by the lock's purpose,
since a CLI write takes no token. Read it before starting; it also says why a `--token` flag on
writes, a block and a stderr-only warning were rejected.

## Acceptance

- A mutating command that writes while this worktree's lock is held adds one warning naming the
  holder, the purpose and the heartbeat age, and still exits 0 with its normal output.
- No warning when the write targets the Issue whose id is the lock's purpose (so `log`, `update`
  and `close` of the started Issue stay silent), when the lock is free, or when the command is
  `start`, `finish` or any `lock` verb. A read-only command (`list`, `show`, `ready`, `search`,
  `prime`) never warns.
- A stale lock's warning says it is stale; an invalid or unreadable record warns and names
  `capcom lock status` without failing the write.
- The warning goes where every warning goes: the text output's warning lines, and the envelope's
  `warnings` with `--json` and `--kv`.
- Checking the lock creates no file under `.git/capcom/locks/` (see `lock-check-writes-file`).
- A test holds a foreign lock and runs `create` and `mv`, asserting exit 0, the file written and
  the warning; runs `log` on the purpose's Issue and asserts no warning; and runs with a free lock
  and asserts no warning.
- The behaviour is named in `AGENTS.md`'s managed block and in the `issue-workflow` Note, so an
  agent reading either learns the lock is signalled and not enforced.
