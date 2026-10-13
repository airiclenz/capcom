---
title: Move removes the emptied old directory as a hard, rolling-back step
status: open
type: bug
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:16Z
---

Move removes the emptied old parent directory as a hard step (`removeEmptyDirStep`), where
Remove treats the same removal as a cleanup that only warns. Severity: Low.

Trigger: a concurrent write into the old parent between `ReadDir` and `Remove` (`ENOTEMPTY`), or
a Windows handle held on the directory, rolls back an otherwise complete move, every rename and
rewrite included, over a harmless empty directory.

Wanted: Move removes the emptied old parent as a cleanup (`AddCleanup`), `removeEmptyDirStep` is
deleted, and `Move.Apply` surfaces cleanup warnings as Remove does. The "PlanMove adds no
cleanups" doc comment is updated.

## Acceptance

- A tree test makes the old directory's removal fail through the remove seam and asserts that
  the move succeeds, with a cleanup warning naming the directory.
- `GOMEMLIMIT=2GiB go test ./internal/tree/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-25.
