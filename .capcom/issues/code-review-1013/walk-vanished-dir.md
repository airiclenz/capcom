---
title: A directory vanishing during the walk fails the whole refresh
status: open
type: bug
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:15Z
---

A directory that vanishes during the walk fails the whole refresh. `treeWalker.walk` returns the
raw `os.ReadDir` error, the index walk wraps it and `index.Open` fails, so any reading command
exits 1. A file that vanishes the same way is skipped, but a directory is not. Severity: Low.

Trigger: another session's `capcom mv` or `rm` of an Issue with children, or a `git checkout`,
renames `<id>/` between the parent's `ReadDir` and the child's.

Wanted: the walk treats a not-exist error on a non-root directory as an empty directory.

## Acceptance

- A tree test removes a child directory through a walk seam (or a `ReadDir` hook) between the
  listing and the descent, and asserts that `Walk` succeeds without that subtree.
- `GOMEMLIMIT=2GiB go test ./internal/tree/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-21.
