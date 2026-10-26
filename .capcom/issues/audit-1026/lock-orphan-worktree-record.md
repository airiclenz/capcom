---
title: A removed worktree's lock record is never pruned and haunts its successor
status: open
type: bug
priority: 3
created: 2026-10-26T20:06:45Z
---

A lock record left by a removed worktree is never pruned, and a new worktree that git gives the
same admin name inherits a "held" lock nobody holds.

`WorktreeKey` is `wt-` plus the basename of git's worktree admin directory. git names that directory
after the basename of the worktree's path, with a numeric suffix on a clash. `git worktree remove`
deletes the admin directory, but nothing deletes `<key>.json` or `<key>.*.token`, and no code checks
whether the worktree that keyed a record still exists.

Reproduced: acquire in a linked worktree at `../wt/feature`, remove it, then add a worktree again at
the same path, or at another path whose last element is `feature`. It starts with a live-looking
lock: `lock acquire` there exits 3, `capcom start` fails, and `prime` and `lock status --all` report
the ghost (both orphaned records stay listed). Re-adding the same branch at a path with another
basename comes up free. `lock break` must be run inside the new worktree, because `break` only
touches the current key.

The ghost goes stale after the TTL, so the cost is a delay and a confusing message, not a lock held
forever.

Checking whether the admin directory exists cannot tell the successor apart, since the directory
exists again. The record needs something that identifies the worktree it was taken in and changes
when git re-creates it, such as a marker in the admin directory or that directory's creation time
compared with the record's `since`.

## Acceptance

- A record whose worktree is gone, or was re-created since the record was taken, is reported as
  orphaned by `lock status`, and `lock acquire` in the new worktree takes the lock.
- A test acquires in a linked worktree, removes it, adds a worktree again at the same path, and
  asserts the lock is free there.
