---
title: Split cli/dep.go into dep, dep_list, dep_graph
status: open
type: task
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:43Z
---

`internal/cli/dep.go` (452 lines) holds every `dep` subcommand and the dependency-graph loader
that `related.go` also uses. Split it:

- `dep.go`: the `dep` root plus `dep add` and `dep rm` (`depRemoval`).
- `dep_list.go`: `dep list`.
- `dep_graph.go`: `loadDepGraph` and `names`.

Update `internal/cli/doc.go`'s file map.

Rules for this pure move: only moves, no renames of exported symbols and no logic edits, so the
diff is file creation plus deletion of the same text. Update the package's `doc.go` file map
where one exists or is created; the structural test enforces it. Line ranges in the review are
from `d192883`; re-derive them by symbol.

## Acceptance

- `internal/cli` holds `dep_list.go` and `dep_graph.go` beside `dep.go`.
- `go vet ./...` is clean and `GOMEMLIMIT=2GiB go test ./internal/cli/` passes with the existing
  tests unchanged, the `doc.go` structural test included.
- The diff is move-only: `git diff --color-moved=zebra` shows no changed lines beyond package and
  import headers and the `doc.go` map.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, P2-10.
