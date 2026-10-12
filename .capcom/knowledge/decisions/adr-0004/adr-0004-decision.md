---
title: "ADR 0004 decision: issue files branch and merge with the code, and a central store stays possible later"
---

The issue files live in the working tree and branch, merge and travel with the code: the state
capcom reports is the state of the checked-out branch. Closing an issue in the same commit as its
fix means the two merge together or not at all, and git is the only sync path, so the drift bd
suffered between its Dolt database, its JSONL export and its separate Dolt remote cannot happen.
The price is that a change made on one branch is not visible on another until it is merged. capcom
softens that with a cheap warning: `ready` and `prime` scan local branches, worktrees and
remote-tracking branches whose issue files differ from HEAD and report those changes separately
(`elsewhere`), without merging any state.

A central store, with one issue state shared by every branch, may be added later as an optional
mode: the issue files on a dedicated data branch, checked out in a hidden worktree. Two rules keep
that door open from the first version. All reads and writes go through one store whose root
directory is resolved at startup, never hard-coded. And whether capcom commits its own writes is a
setting of that store; in the default mode it never commits.
