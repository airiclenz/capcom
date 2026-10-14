---
title: know update removes a superseded-by name two files claim
status: open
type: bug
priority: 1
plan: 2026-10-14 - 00 - architecture-review-fixes-plan
created: 2026-10-14T08:35:11Z
---

`capcom know update <id> --remove-superseded-by <name>` removes the entry when two files claim
`<name>` (as an id or an alias), where `capcom dep rm` refuses the same situation naming both
files. The superseded-by removal treats a duplicate-name resolution error as a miss
(`isResolveMiss` counts `tree.ErrDuplicate` with not-found and ambiguous) and then drops the entry
whose text equals the argument, so the edit goes through on a name the Tree cannot say which
Note it means. dep and related (`resolveFailure`) do not treat a duplicate as a miss.

Wanted: the removal refuses a doubly-claimed name exactly as `dep rm` does, and writes nothing.
A removal whose name matches no entry and no node stays a silent no-op.

## Acceptance

- `capcom know update <id> --remove-superseded-by <name>` exits 1 naming both files and leaves
  the Note unchanged when two files claim `<name>`.
- A runCLI test covers it; `go vet ./...` is clean and `GOMEMLIMIT=2GiB go test ./internal/cli/`
  passes.

Source: docs/reviews/architecture-review-2026-10-14-0848.html, Defect leads.
