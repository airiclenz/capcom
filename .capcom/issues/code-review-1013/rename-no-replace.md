---
title: Node rename can replace a file created a moment earlier
status: done
type: bug
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:15Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

A Node-file rename can replace a file created a moment earlier. `renameStep` checks the
destination with `Lstat` and then calls `os.Rename`, which replaces whatever appeared in between.
Severity: Low.

Trigger: a `capcom create --id y` (through `WriteFileExclusive`) lands in the microseconds
between a move's `Lstat` and its `Rename`. The create reports success, and its file is then
replaced by the moved file.

Wanted: node-file renames use a no-replace primitive, as `placeExclusive` already does:
`os.Link(old, new)` (atomic `EEXIST`), then `os.Remove(old)`, with the undo reversed to match.
Directory renames are already safe, because the target is non-empty.

## Acceptance

- A tree test places a file at the destination after the `Lstat`, through a test seam, and
  asserts the rename fails with `EEXIST` and both files survive.
- `GOMEMLIMIT=2GiB go test ./internal/tree/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-11(c).
