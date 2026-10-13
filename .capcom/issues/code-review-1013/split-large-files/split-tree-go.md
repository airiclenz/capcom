---
title: Split tree/tree.go into errors, ids, walk, claims
status: done
type: task
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:42Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

`internal/tree/tree.go` (586 lines) holds the error types, id and Alias validation, the
directory walk and the claims bookkeeping beside `Tree` itself. Split it:

- `errors.go`: the sentinels, `Error` and `Errors`.
- `ids.go`: `ValidateID`, `ValidateAlias`, `CheckAliases`, plus `RenameAliases` from `move.go`.
- `walk.go`: `Walk`, `treeWalker` and `leftOutFile`.
- `claims.go`: `indexIDs`, `indexAliases` and the duplicates bookkeeping.
- `tree.go` keeps `Tree`, `Load`, the accessors and `PathFor`.

Add `internal/tree/doc.go` with a file map and a structural test modelled on
`internal/cli/doc_test.go`. This split comes before `nodefile-package` and `node-layout-claims`.

Rules for this pure move: only moves, no renames of exported symbols and no logic edits, so the
diff is file creation plus deletion of the same text. Update the package's `doc.go` file map
where one exists or is created; the structural test enforces it. Line ranges in the review are
from `d192883`; re-derive them by symbol.

## Acceptance

- `internal/tree` holds `errors.go`, `ids.go`, `walk.go`, `claims.go` and `doc.go` beside `tree.go`,
  and the new structural test passes.
- `go vet ./...` is clean and `GOMEMLIMIT=2GiB go test ./internal/tree/` passes with the existing
  tests unchanged.
- The diff is move-only: `git diff --color-moved=zebra` shows no changed lines beyond package and
  import headers and the new `doc.go` and test.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, P2-01.
