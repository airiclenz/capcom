---
title: "Lock overview: every Tree lock of a clone and one worktree's lock state, read together"
---

Every **Tree lock** of a clone read together at one moment: each held lock, one worktree's lock
state named as free, held, stale, invalid or unreadable, and one problem per lock record that cannot
be read as a lock. A damaged record of another worktree is one problem beside the other locks; only
a lock directory that cannot be listed leaves no **Lock overview** at all.
_Avoid_: lock list, lock report, lock summary
