---
title: IsAncestor error path untested
status: done
type: task
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:13Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

`gitx.IsAncestor` treats exit 1 as "not an ancestor" and any other non-zero exit as an error,
but only its true and false cases are tested, so a change that swallowed git failures would
pass. Severity: Low. The matching `HasCommit` error case belongs to `hascommit-fatal-exit`.

## Acceptance

- A `branches_test.go` case calls `IsAncestor` with a ref that does not exist and asserts a
  non-nil error and `false`.
- `GOMEMLIMIT=2GiB go test ./internal/gitx/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, T-08.
