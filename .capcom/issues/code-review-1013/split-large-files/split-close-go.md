---
title: Split reopen out of cli/close.go
status: open
type: task
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:43Z
---

`internal/cli/close.go` (299 lines) holds both `close` and `reopen`. Move `reopen` and
`targetStatus` to a new `internal/cli/reopen.go`, leaving `close` in `close.go`, and update
`internal/cli/doc.go`'s file map. Small; it comes before `issue-lifecycle-module`.

Rules for this pure move: only moves, no renames of exported symbols and no logic edits, so the
diff is file creation plus deletion of the same text. Update the package's `doc.go` file map
where one exists or is created; the structural test enforces it. Line ranges in the review are
from `d192883`; re-derive them by symbol.

## Acceptance

- `internal/cli/reopen.go` holds the `reopen` command and `targetStatus`.
- `go vet ./...` is clean and `GOMEMLIMIT=2GiB go test ./internal/cli/` passes with the existing
  tests unchanged, the `doc.go` structural test included.
- The diff is move-only: `git diff --color-moved=zebra` shows no changed lines beyond package and
  import headers and the `doc.go` map.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, P2-09.
