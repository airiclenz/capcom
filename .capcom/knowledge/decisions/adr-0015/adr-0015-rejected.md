---
title: "ADR 0015 rejected options: one global file keyed by repository path, folding it into state.yaml, writing on quit"
labels: [background]
---

- **One global file keyed by repository path.** One file for every repository, beside
  `state.yaml`, but it grows by an entry for every worktree ever opened and nothing removes one,
  and an entry is lost when a repository or worktree moves or is cloned again, since its path is
  its key. A file in the git dir lives and dies with its worktree.
- **Fold it into `state.yaml`** (ADR 0014). No new file, but the tab and the selections are per
  worktree and `state.yaml` is per user: folding them in needs that same key by repository path,
  with the same growth and the same loss on a move, and makes every worktree of the user rewrite
  one shared file on each change of selection.
- **Write only on quit.** One write per session, but a crash, a killed terminal or a dropped SSH
  session loses the tab and the selections, and quitting would gain a failure with no status bar
  left to report it on. A tab change is rare, and the one-second wait folds a run of moves into one
  write, so writing as they change costs nothing.
