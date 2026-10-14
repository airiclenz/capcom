---
title: One module for reference-list edits
status: open
type: task
priority: 2
deps: [supersede-dup-name]
plan: 2026-10-14 - 00 - architecture-review-fixes-plan
created: 2026-10-14T08:35:11Z
---

Edits to reference lists (an issue's `deps` and `related`, a Note's `superseded-by`) are written
by hand per command in `internal/cli`: `dep.go`, `related.go`, `know_update.go`, and
`create.go`'s `resolveDeps`. Each repeats the self-link refusal, the already-present check and
the removal rule, and the copies disagree: dep and related remove by exact id or alias and refuse
a doubly-claimed name, while superseded-by removal matches the argument's text first
(`removedSuccessors`) and treats a duplicate as a miss (`isResolveMiss`); superseded-by has no
already-present check. About five mirrored runCLI test pairs in `dep_test.go` and
`related_test.go` pin the same rules twice.

Wanted: a leaf package `internal/reflist`, importing only `tree`, decides add and remove on a
reference list. Given the holder, its entries, the argument, a resolver and an entry-names
lookup, it returns the canonical id and whether it is already present, the entries to drop, or a
typed refusal (self, cycle, not found, ambiguous, duplicate). cli maps flags and refusals only.
Ratified call (2026-10-14): superseded-by edits follow dep/related: an exact id or alias beats
the entry text, a removal matches every alias spelling of the node, ambiguous or doubly-claimed
names are refused, already-present is judged by names; a removal matching nothing stays a silent
no-op. dep, related and create --dep output is unchanged.

## Acceptance

- `internal/reflist` exists and imports only `tree` from this module; dep add/rm, related
  add/rm, create --dep and know update --add/--remove-superseded-by decide through it.
- A table test in `reflist` covers each refusal and removal rule; dep, related and create --dep
  output is unchanged.
- `go vet ./...` is clean and the touched packages' tests pass.

Source: docs/reviews/architecture-review-2026-10-14-0848.html, 3 · One module for reference-list edits.
