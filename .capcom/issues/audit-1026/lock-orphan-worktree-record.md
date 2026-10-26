---
title: A removed worktree's lock record is never pruned and haunts its successor
status: open
type: bug
priority: 3
created: 2026-10-26T20:06:45Z
---

A lock record left by a removed worktree is never pruned, and a new worktree that git names the
same way inherits a "held" lock nobody holds. Inferred from the code; not reproduced.

`WorktreeKey` is `wt-` plus the basename of git's worktree admin directory, which is the branch
name with a numeric suffix when two worktrees collide. `git worktree remove` deletes the admin
directory but nothing deletes `<key>.json` or `<key>.*.token`, and no code checks whether the
worktree that keyed a record still exists.

So: tidy up a worktree on branch `feature`, later add a new one on `feature` — a very likely
reuse — and it starts with a live-looking lock. `lock acquire` there exits 3, `capcom start`
fails, `prime` and `lock status --all` report a ghost lock, and `lock break` must be run inside
that worktree because `break` only touches the current key.

This is worth confirming with a reproduction before it is fixed.

## Acceptance

- A record whose worktree no longer exists is reported as orphaned, or pruned, by `lock status`
  and `lock acquire`.
- A test acquires in a linked worktree, removes it, re-adds a worktree on the same branch, and
  asserts the lock is free there.
