---
title: "ADR 0005 rejected options: one lock per clone, pid liveness or a held flock, and an enforcing pre-commit guard"
labels: [background]
---

- **One lock per clone (git common dir).** Simpler, but it serialises agents that work in separate
  worktrees, which do not share files.
- **pid liveness or a held `flock`.** Fails with fresh-shell orchestrators, as described above.
- **An enforcing pre-commit guard, like MCP Agent Mail's.** Brings back the hook-trust burden capcom
  exists to remove; a `lock check` command lets a repo opt in.
