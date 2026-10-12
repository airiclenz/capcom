---
title: "Tree lock: the advisory claim one agent session holds on one working tree"
---

The advisory claim one agent session holds on one working tree (one checkout or worktree) while it
changes it, naming the holder, the purpose and the time it was taken. Each worktree of a clone has
its own **Tree lock**, and the locks of all worktrees can be seen together.
_Avoid_: repo lock, lease, claim, mutex
