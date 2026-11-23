---
title: Move Note successor-list edits out of internal/cli into internal/ops
status: done
type: task
priority: 3
labels: [maturity, refactor]
plan: 2026-11-23 - 01 - internal-cli-split-plan
created: 2026-11-23T08:01:16Z
closed: 2026-11-23T08:56:55Z
reason: Resolved by plan 2026-11-23 - 01 - internal-cli-split-plan, item 7 (archived in b56a611)
---

`capcom know update` resolves Note names and applies the add and remove edits to a Note's
superseded-by list in `internal/cli/know_update.go` (`noteNames`, `editSuccessors`), while
the Issue-side equivalents already live in `internal/ops`. Move them beside those, with direct
unit tests; cli keeps the refusal wording. Item 7 of the plan.

## Why

ADR 0012 puts the write glue both front ends share in internal/ops.

## Acceptance

- `internal/ops` holds both with direct tests; `internal/cli` defines neither noteNames nor editSuccessors
- `know update` output and refusals stay byte-identical
