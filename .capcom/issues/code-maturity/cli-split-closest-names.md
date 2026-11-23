---
title: Move name suggestion ranking out of internal/cli into internal/nearest
status: done
type: task
priority: 3
labels: [maturity, refactor]
plan: 2026-11-23 - 01 - internal-cli-split-plan
created: 2026-11-23T08:01:16Z
closed: 2026-11-23T08:56:54Z
reason: Resolved by plan 2026-11-23 - 01 - internal-cli-split-plan, item 1 (archived in b56a611)
---

The edit distance and the ranking of the names closest to a mistyped command, flag or id
(`editDistance`, `closestName` in `internal/cli/root.go`; `closestNames`, `sharedWordCount`,
`letterCount`, `isNearName` in `internal/cli/suggest.go`) are domain logic living in the
command package. Move them to a new leaf package `internal/nearest`, with direct unit tests;
the cli callers word the suggestions as today. Item 1 of the plan.

## Why

Pure string ranking is tested and reused more easily outside the cobra command package.

## Acceptance

- `internal/nearest` holds the ranking with direct tests; `internal/cli` defines none of the six functions
- Every suggestion capcom prints stays byte-identical; `go test ./internal/nearest/ ./internal/cli/` passes
