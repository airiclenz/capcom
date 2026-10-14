---
title: Architecture review 2026-10-14
status: open
type: task
priority: 2
plan: 2026-10-14 - 00 - architecture-review-fixes-plan
created: 2026-10-14T08:35:10Z
---

Parent of the issues filed from the architecture review of 2026-10-14
(`docs/reviews/architecture-review-2026-10-14-0848.html`). The review found three Strong
deepening candidates, a handful of small locality fixes and defect leads; the verified defects
are filed as bugs. The work runs in phases:

- Phase 1: one owner for Tree membership and resolution (tree decides, the index persists and
  queries what tree decided), with the name-claim bugs it fixes.
- Phase 2: one finding type and one code set for check and the index.
- Phase 3: one module for reference-list edits (deps, related, superseded-by), with the
  superseded-by bug it fixes.
- Phase 4: the small locality fixes: one frontmatter fence predicate, store-owned
  worktree-relative paths, and one visible-subtree outline with the know tree bug it fixes.

Out of scope: review candidates 4 to 10 and "Multi-argument results". Each child issue is
readable on its own and carries its own acceptance.

## Acceptance

- every child issue is closed

Source: docs/reviews/architecture-review-2026-10-14-0848.html, Top recommendation.
