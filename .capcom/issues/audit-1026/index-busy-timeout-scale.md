---
title: A rebuild outlasts the busy timeout and concurrent commands fail
status: open
type: bug
priority: 3
created: 2026-10-26T20:06:45Z
---

At a plausible store size, a rebuild outlasts the busy timeout and concurrent commands fail.

Reproduced with 12,000 issues: `capcom reindex` takes about 19 s, while the SQLite busy timeout is
5 s. A `capcom list` started during the rebuild fails:

    capcom reindex &
    capcom list -> begin refresh: database is locked (5) (SQLITE_BUSY)

Three concurrent lists all failed. There is no retry past the 5 s timeout.

Related, measured on the same store: `list` runs about five statements per issue (0.52 s for
12,000 rows), and `search --excerpt` runs one body query per hit (1.10 s versus 0.14 s without).
Both grow linearly with the store, and `ready` walks every issue to rank them.

A long rebuild is the moment a user is most likely to run something else, so this is the common
case, not an edge one.

## Acceptance

- A command that meets a busy index waits, retries, or reports a clear message naming
  `capcom reindex` and how long it may take, rather than a raw SQLITE_BUSY.
- `list` and `search --excerpt` stop issuing per-row queries.
- A benchmark records the timings so a regression is visible.
