---
title: Split nodestore/create.go into create, load, claim, write
status: done
type: task
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:42Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

`internal/nodestore/create.go` (395 lines) holds the helpers that Create, Move, Remove, Format
and Edit share, so a reader of Move or Edit opens `create.go` for them. Split it:

- `create.go`: `Create` only.
- `load.go`: `load`, `triage`, `isBlockingProblem`, `resolveFailure` and `relativeTreeError`.
- `claim.go`: `claimFailure`, the orphan and unparsed checks and `findNodeFile`.
- `write.go`: `write`, `encode`, `place` and `validationError`.

Add a `doc.go` file map. This split comes before `reference-rewrite-engine` and
`node-layout-claims`.

Rules for this pure move: only moves, no renames of exported symbols and no logic edits, so the
diff is file creation plus deletion of the same text. Update the package's `doc.go` file map
where one exists or is created; the structural test enforces it. Line ranges in the review are
from `d192883`; re-derive them by symbol.

## Acceptance

- `internal/nodestore` holds `create.go`, `load.go`, `claim.go`, `write.go` and `doc.go`.
- `go vet ./...` is clean and `GOMEMLIMIT=2GiB go test ./internal/nodestore/` passes with the
  existing tests unchanged.
- The diff is move-only: `git diff --color-moved=zebra` shows no changed lines beyond package and
  import headers and the new `doc.go`.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, P2-02.
