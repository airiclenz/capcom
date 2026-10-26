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

ADR 0016 says "a session naming any other purpose still fails `locked`, so the lock keeps telling
different work apart". That holds only until one token is reused under a second purpose, which is
exactly what an agent does when it holds the token and runs `capcom start <id>` after a
`capcom lock acquire --purpose "<free text>"`.

It also leaks one token file per purpose ever resumed; only `release` or `break` with a matching
token cleans them up.

## Acceptance

- Resuming a token under a new purpose moves the token file rather than adding one.
- After the move, the old purpose fails `locked` with exit 3.
- A test resumes one token under two purposes and asserts one token file and a refusal.
