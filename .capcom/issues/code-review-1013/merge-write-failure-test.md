---
title: Merge driver write failure and ReplaceFileVerbatim missing path untested
status: open
type: task
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:13Z
---

The merge driver's failure to write `<ours>` (the `tree.ReplaceFileVerbatim` error that becomes
`failedMerge`) and `ReplaceFileVerbatim`'s missing-path branch are untested. The write is a
temporary file plus a rename, so ours cannot be half-written today; these tests guard against a
future regression only. Severity: Low.

## Acceptance

- A merge-driver test with `<ours>` replaced by a directory, or in a read-only directory
  (skipped on Windows and as root), asserts a `CodeInvalid` failure naming ours, exit 1, the
  original bytes intact and no `.tmp-` litter.
- A tree test calls `ReplaceFileVerbatim` on a missing path and asserts the error names it.
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run Merge` and `GOMEMLIMIT=2GiB go test
  ./internal/tree/` pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, T-06.
