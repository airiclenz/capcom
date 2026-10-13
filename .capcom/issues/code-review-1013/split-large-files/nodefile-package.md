---
title: Move atomic writes and guarded reads from tree to internal/nodefile
status: done
type: task
priority: 3
deps: [split-tree-go, guarded-file-reader]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:42Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

`tree` hosts generic file IO that `lock`, `store`, `agents`, `check`, `index`, `cli` and
`nodestore` use; `lock` and `store` import `tree` only for it. Move it to a new leaf package
`internal/nodefile`: the atomic writes (`WriteFileAtomic`, `WriteFileAtomicVerbatim`,
`ReplaceFileVerbatim`, `WriteFileExclusive`, `lfWriter`), `ReadNodeFile`, the guarded reader
from `guarded-file-reader` and `syncDir` (`atomic.go`, `read.go`, `syncdir_*.go`), plus
`edit.ETag`. This is the first, mechanical half of `node-layout-claims`.

Constraints:

- Keep `tree.ErrNotRegular` and `tree.ErrTooLarge` matchable: define them in `nodefile` and
  re-export them from `tree` (`var ErrNotRegular = nodefile.ErrNotRegular`), so every
  `errors.Is` site still works.
- `nodefile` imports nothing from `internal/`; `tree` imports it, and `lock` and `store` import
  it instead of `tree`.
- Platform files keep their build tags, and the export_test seams (`SetLink`) move with them.
- Update the imports in `lock`, `store`, `agents`, `check`, `index`, `cli` and `nodestore`.

Rules for this pure move: only moves, no renames of exported symbols and no logic edits, so the
diff is file creation plus deletion of the same text. Update the package's `doc.go` file map
where one exists or is created; the structural test enforces it. Line ranges in the review are
from `d192883`; re-derive them by symbol.

## Acceptance

- `go list -f '{{join .Imports "\n"}}' ./internal/nodefile/` lists no `internal/` package, and
  neither `internal/lock` nor `internal/store` imports `internal/tree`.
- `errors.Is(err, tree.ErrNotRegular)` and `tree.ErrTooLarge` still match errors from the moved
  reader.
- `go vet ./...` is clean, and `internal/nodefile` and every importing package's tests pass, one
  package at a time with `GOMEMLIMIT=2GiB`.
- The diff is move-only apart from package qualifiers and import headers
  (`git diff --color-moved=zebra`).

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, P2-03 (and A-05).
