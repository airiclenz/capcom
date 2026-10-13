---
title: One reference-rewrite engine in nodestore
status: done
type: task
priority: 3
deps: [split-nodestore-create, stale-rewrite-cas]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:43Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

Four near-identical loops (`issue.PlanMove`, `issue.PlanRemove`, and the move and remove planners
in `knowledge`) each read every other Node file, parse it or add it to Unread, rewrite a reference
list and emit a `tree.Rewrite`. Only the field (deps and related, or superseded-by) and the
operation (rename or drop) differ. Further duplication:

- `isRefusedRead` is copied three times (`issue`, `knowledge`, `nodestore`).
- The `rawfm` Alias fallback is copied twice (`issue` and `knowledge` remove).
- Kind-specific drop fields leak into the generic `nodestore.Removed` (`depsDropped`,
  `relatedDropped`, `supersededByDropped`).
- The etag check runs in `issue.PlanRemove` for Issues but in `nodestore` for Notes, giving two
  error vocabularies (`issue.ETagMismatchError`/`issue.DependentsError` translated to
  `nodestore.ConflictError`/`DependentsError`).
- The rename functions have drifted: `renameSuccessors` dedupes its output and `renameDeps` does
  not (`rename-dup-deps`).

Strength: Strong; the largest refactor in the review.

Wanted: `nodestore.Codec` gets a list of reference fields, `{Key, Get(T) []string, Set(*T,
[]string), BlocksRemoval bool}`. `nodestore` owns a single `planReferenceRewrites(nodes, names,
op)` and a single etag check; `Removed` and `Moved` report drops keyed by field. `issue` and
`knowledge` keep only Parse, Serialize, Validate and their domain rules. The compare-and-swap that
`stale-rewrite-cas` adds to `tree.Plan` lands first, and its tests stay green. This absorbs
`know-rm-report-drops` (drops reported per field), `note-self-supersede` (the rename op also
applies to the moved Node's own fields) and `rename-dup-deps`. ADR 0002 (Aliases) is unchanged.

Risks: the wording of Unread warnings and the `rm` result keys are pinned by CLI tests. Land
characterisation tests for the move and remove results first, and keep the `--json` keys
identical, adding only `superseded_by_dropped`.

## Acceptance

- Characterisation tests for `mv`, `rm`, `know mv` and `know rm` results (text, `--json`,
  `--kv`) land before the refactor and pass unchanged after it, apart from the added
  `superseded_by_dropped` key.
- `nodestore` holds the one reference rename and drop planner and the one etag check;
  `renameDeps`, `renameSuccessors` and the per-package copies of `isRefusedRead` are gone.
- The `stale-rewrite-cas` tests still pass.
- `GOMEMLIMIT=2GiB go test ./internal/issue/`, then `./internal/knowledge/`, `./internal/nodestore/`
  and `./internal/cli/`, pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, A-04.
