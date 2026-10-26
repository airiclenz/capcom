---
title: Resuming a token under a new purpose lets the old purpose take the lock
status: open
type: bug
priority: 2
created: 2026-10-26T20:06:45Z
---

Resuming a token under a second purpose leaves the first purpose's token file behind, so a
session naming the old purpose can take back a live lock. ADR 0016 promises the opposite.

Reproduced:

    capcom lock acquire --purpose alpha              -> token e9d9...
    capcom lock acquire --purpose beta --token e9d9... -> "resumed ... : beta"
    ls .git/capcom/locks/*.token                     -> TWO token files, both e9d9...
    capcom lock acquire --purpose alpha              -> exit 0, "resumed ... : alpha"   (should be exit 3)

`writeTokenFile` (`internal/lock/lock.go:213`) adds a file for the new purpose and never removes the
others holding the same token. ADR 0016 says "a session naming any other purpose still fails
`locked`", which holds only until one token is reused under a second purpose: exactly what an agent
does when it runs `capcom start <id> --token <T>` after a `capcom lock acquire --purpose "<text>"`,
and what `finish --keep-lock` will recommend (`keep-lock-blocks-next`). It also leaks one token
file per purpose resumed; only `release` or `break` cleans them up.

The ADR 0016 amendment of 2026-10-26 (`adr-0016-amend-1026`) decides the fix: the token file moves.

## Acceptance

- Resuming a token under a new purpose writes the new purpose's token file and deletes every other
  token file of this worktree holding the same token, in the same guarded step.
- After the move, the old purpose fails `locked` with exit 3.
- A test resumes one token under two purposes and asserts one token file and a refusal.
