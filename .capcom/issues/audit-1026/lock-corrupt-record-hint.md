---
title: A corrupt lock record wedges every lock command and names no fix
status: open
type: bug
priority: 2
created: 2026-10-26T20:06:45Z
---

A corrupt or token-less lock record wedges every lock command, and no message names the command
that fixes it.

Reproduced with `.git/capcom/locks/_main.json` holding `not json{{{`:

    capcom lock check            -> exit 1, hint: capcom lock check --help
    capcom lock touch --token x  -> exit 1, hint: capcom lock touch --help
    capcom lock release --token x-> exit 1, hint: capcom lock release --help
    capcom lock acquire --purpose p -> exit 1, hint: capcom lock acquire --help
    capcom lock status           -> exit 0, "_main: invalid" + warning   (this one is right)

`lock status` handles the state well; every command that would fix it does not. `finish` already
gets this right (`run capcom lock status, then capcom lock break to free it`), so the wording
exists in the codebase to copy.

A lock nobody can acquire, touch or release is worse than a stale one, because `--take-stale`
cannot help and the exit code is 1 rather than 3, so a hook cannot tell it apart from an I/O
failure.

## Acceptance

- Every lock command that meets an unreadable record exits 3 and names `capcom lock break`.
- `--json` carries the same code and message.
- A test corrupts the record and asserts all five commands agree.
