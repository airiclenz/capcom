---
title: No test for a node file refused unread after tree.Load in rm and mv
status: done
type: task
priority: 3
plan: 2026-10-12 - 00 - code-audit-fixes-plan
created: 2026-10-12T16:43:02Z
closed: 2026-10-12T20:07:19Z
reason: Resolved by plan 2026-10-12 - 05 - audit-leftovers-and-doc-tidy-plan (items 2-3), archived in 72afde6
---

Deferred from the code-audit-fixes plan, item 4: no test pins the path where another node's file
is refused unread by `tree.ReadNodeFile` and joins `Unread`. Nothing covers a file being swapped to
a symlink, a FIFO or a file over 1 MiB after `tree.Load` in `planDrops`
(internal/issue/remove.go:127, internal/knowledge/remove.go:74), `planRewrite`
(internal/issue/move.go:87) or `planSuccessorRewrites` (internal/knowledge/move.go:82); that branch
is covered only by reading the code.

## Acceptance

- Tests for issue and Note `rm` and `mv` swap another node's file after `tree.Load` and assert it
  joins `Unread`, is left as it is, and the command still completes.
