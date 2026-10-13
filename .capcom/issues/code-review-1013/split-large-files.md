---
title: Split the large source files (pure moves)
status: open
type: task
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:42Z
---

Several source files have grown to 300-600 lines and mix concerns that change for different
reasons, so a reader of one verb opens a file full of another's helpers. The review's section 5
lists the splits as pure moves: zero behaviour change, tests unchanged, one commit each, done
before the deepening refactors that build on them.

This parent holds the pure-move issues. Two rows of the review's table are not filed:
`split-init-go` (P2-12) and `split-release-files` (P2-13). `hooks-package` (A-09) and
`release-step-table` (A-11) replace them.

## Acceptance

- every child issue is closed

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, sections 5.2 and 7 (Pure moves).
