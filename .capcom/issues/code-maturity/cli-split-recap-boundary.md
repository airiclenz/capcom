---
title: Move the recap --since boundary out of internal/cli into internal/recap
status: done
type: task
priority: 3
labels: [maturity, refactor]
plan: 2026-11-23 - 01 - internal-cli-split-plan
created: 2026-11-23T08:01:16Z
closed: 2026-11-23T08:56:55Z
reason: Resolved by plan 2026-11-23 - 01 - internal-cli-split-plan, item 3 (archived in b56a611)
---

Resolving a `capcom recap --since` value (a date, `<n>d` or a git ref) to its boundary commit
lives in `internal/cli/recap.go` (`recapBoundary`, `sinceTime`). Move it to `internal/recap`,
returning a sentinel error for an unknown value that cli words as today, with direct unit
tests. Item 3 of the plan.

## Why

The boundary is recap's own input; testing its forms should not need a full CLI run.

## Acceptance

- `internal/recap` resolves the boundary with a direct table test; `internal/cli` defines neither function
- An unknown --since still fails with the same message and hint
