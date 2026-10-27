---
title: A rebuild outlasts the busy timeout and concurrent commands fail with SQLITE_BUSY
status: open
type: bug
priority: 3
plan: 2026-10-27 - 00 - audit-p3-fixes-plan
created: 2026-10-26T20:06:45Z
---

At a plausible store size, a rebuild outlasts the busy timeout and concurrent commands fail.

Reproduced with 12,000 issues: `capcom reindex` takes about 19 s, while the SQLite busy timeout is
5 s with no retry (`internal/index/index.go:40`). Every command, reads included, opens the index
with a refresh in an IMMEDIATE transaction (`internal/index/refresh.go:64`), and a rebuild holds
that lock for its whole run. A `capcom list` started during the rebuild fails:

    capcom reindex &
    capcom list -> begin refresh: database is locked (5) (SQLITE_BUSY)

Three concurrent lists all failed.

A long rebuild is the moment a user is most likely to run something else, so this is the common
case at that size, not an edge one.

The per-row query cost that makes the store slow at this size is `list-per-row-queries`.

## Acceptance

- A command that meets a busy index waits, retries, or fails with a message naming
  `capcom reindex` and saying the index is being rebuilt, rather than a raw SQLITE_BUSY.
- A test holds the index's write lock past the busy timeout and asserts the command's behaviour.
