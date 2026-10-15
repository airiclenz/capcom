---
title: "ADR 0013 decision: the UI shows the lock, confirms each write while another session holds it, and never takes it"
---

The Tree lock is advisory (ADR 0005): it binds only the sessions that check it, and no write
command checks it today. That suits agents, which check it as part of their workflow before a
multi-file change. A person in `capcom ui` is different: they are watching agents work, and the
lock is the one signal that an agent is in the middle of something.

**Always visible.** The status bar shows the lock of this worktree: free, held (by whom, with its
purpose and heartbeat age), or stale.

**Confirm before writing while someone else holds it.** When another session holds the lock, live
or stale, every write (status, claim, close, reopen, log, `$EDITOR`) first asks
"locked by <purpose>, write anyway?". Yes writes; no cancels. A free lock asks nothing. The check
reads the lock record at the moment of the write, not the status bar's last poll.

**The UI never takes the lock.** Its writes are single-file edits guarded by an ETag, not the
multi-file changes the lock exists to serialise, and a person browsing for an hour must not shut
agents out of the worktree.
