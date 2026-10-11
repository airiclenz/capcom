# Issues are versioned with the code, and a central store stays possible

The issue files live in the working tree and branch, merge and travel with the code: the state
capcom reports is the state of the checked-out branch. Closing an issue in the same commit as its
fix means the two merge together or not at all, and git is the only sync path, so the drift bd
suffered between its Dolt database, its JSONL export and its separate Dolt remote cannot happen.
The price is that a change made on one branch is not visible on another until it is merged. capcom
softens that with a cheap warning: `ready` and `prime` scan local branches and worktrees whose
issue files differ from HEAD and report those changes separately (`elsewhere`), without merging
any state.

A central store, with one issue state shared by every branch, may be added later as an optional
mode: the issue files on a dedicated data branch, checked out in a hidden worktree. Two rules keep
that door open from the first version. All reads and writes go through one store whose root
directory is resolved at startup, never hard-coded. And whether capcom commits its own writes is a
setting of that store; in the default mode it never commits.

## Considered Options

- **bd's model, a store outside git branches.** Instant global visibility, but a second sync path
  next to git, and the drift that follows from it.
- **Merging issue state across branches, as Backlog.md does.** About 2,000 lines of loader code
  and many git calls per read, and its own CLI still reads only the local branch.
