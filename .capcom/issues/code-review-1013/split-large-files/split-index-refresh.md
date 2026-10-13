---
title: Split index/refresh.go into refresh, problems, duplicates
status: open
type: task
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:42Z
---

`internal/index/refresh.go` (476 lines) mixes refresh orchestration with the mapping of file
problems to Warnings and the duplicate and claim exclusion. Split it:

- `refresh.go`: the refresh orchestration, `indexFile`, `forget` and `rewarn`.
- `problems.go`: file problems mapped to Warnings (`readRefusal` and its helpers).
- `duplicates.go`: duplicate and claim exclusion.

This split comes before `shared-diagnostics`.

Rules for this pure move: only moves, no renames of exported symbols and no logic edits, so the
diff is file creation plus deletion of the same text. Update the package's `doc.go` file map
where one exists or is created; the structural test enforces it. Line ranges in the review are
from `d192883`; re-derive them by symbol.

## Acceptance

- `internal/index` holds `problems.go` and `duplicates.go` beside `refresh.go`.
- `go vet ./...` is clean and `GOMEMLIMIT=2GiB go test ./internal/index/` passes with the existing
  tests unchanged.
- The diff is move-only: `git diff --color-moved=zebra` shows no changed lines beyond package and
  import headers.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, P2-06.
