---
title: HasCommit reports a fatal git failure as no commit
status: open
type: bug
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:15Z
---

`gitx.HasCommit` reports a fatal git failure as "no such commit". Every non-zero exit of
`git rev-parse --verify --quiet` maps to `(false, nil)`, including exit 128 on a corrupt object
store, so the Elsewhere scan warns "HEAD has no commit yet" instead of giving the git error.
`IsAncestor` beside it already checks the exit code. Severity: Low.

Wanted: `HasCommit` returns `false, nil` only for exit code 1 and the error otherwise.

## Acceptance

- A `branches_test.go` case runs `HasCommit` on a non-repository directory, or on a repository
  with a corrupted HEAD object, and gets a non-nil error.
- `GOMEMLIMIT=2GiB go test ./internal/gitx/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-19.
