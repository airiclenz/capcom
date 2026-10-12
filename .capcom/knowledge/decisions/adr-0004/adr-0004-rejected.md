---
title: "ADR 0004 rejected options: a store outside git branches and merging issue state across branches"
---

- **bd's model, a store outside git branches.** Instant global visibility, but a second sync path
  next to git, and the drift that follows from it.
- **Merging issue state across branches, as Backlog.md does.** About 2,000 lines of loader code
  and many git calls per read, and its own CLI still reads only the local branch.
