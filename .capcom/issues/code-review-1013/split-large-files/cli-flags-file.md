---
title: Collect cli flag constants in flags.go
status: open
type: task
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:42Z
---

The shared flag-name constants of `internal/cli` (`flagParent`, `flagDepth`, `unlimitedDepth`,
`flagIfMatch`, `flagTitle`, ...) are declared in whichever command file needed them first
(`list.go`, `update.go`, `know.go`, `create.go`, `know_update.go`). Commands import their
flags from a sibling command's file, which is hidden coupling. Collect every shared flag constant
in a new `internal/cli/flags.go`, and add it to `internal/cli/doc.go`'s file map.

Rules for this pure move: only moves, no renames of exported symbols and no logic edits, so the
diff is file creation plus deletion of the same text. Update the package's `doc.go` file map
where one exists or is created; the structural test enforces it. Line ranges in the review are
from `d192883`; re-derive them by symbol.

## Acceptance

- Every flag constant used by more than one command file is declared in `internal/cli/flags.go`.
- `go vet ./...` is clean and `GOMEMLIMIT=2GiB go test ./internal/cli/` passes with the existing
  tests unchanged, the `doc.go` structural test included.
- The diff is move-only: `git diff --color-moved=zebra` shows no changed lines beyond the new
  file's header and the `doc.go` map.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, P2-07.
