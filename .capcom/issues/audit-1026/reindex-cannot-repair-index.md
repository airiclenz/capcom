---
title: reindex cannot repair a corrupt index, so the named fix fails
status: open
type: bug
priority: 1
created: 2026-10-26T20:06:45Z
---

`capcom reindex` cannot repair a corrupt index, so the one recovery path capcom names does not
work. `prime` prints `warning: index: ...: file is not a database (26); run capcom reindex`, and
`reindex` then fails with the same error, because it reads `sqlite_master` to find the tables to
drop before it drops anything.

Reproduced:

    head -c 3000 /dev/urandom > .git/capcom/index.db
    capcom list      -> capcom: ...index.db: file is not a database (26)   exit 1
    capcom reindex   -> capcom: ...index.db: file is not a database (26)   exit 1

The only thing that works is `rm .git/capcom/index.db*`, which no message suggests and an agent
has to guess.

This matters because the index is explicitly disposable: the markdown files are the truth, and
losing the index should never be more than a slow rebuild. A corrupt index is reachable from a
killed process, a full disk, or a merge of `.git` state, and today it takes the whole tool down
for a user who follows the advice capcom printed.

## Acceptance

- `capcom reindex` succeeds on an index whose header is unreadable or whose schema is foreign,
  by discarding the file rather than inspecting it.
- A command that meets a corrupt index names `capcom reindex` as the fix, and `reindex` then
  actually fixes it.
- A test writes garbage bytes over `index.db` and asserts that `reindex` exits 0 and the next
  `list` shows every issue.
