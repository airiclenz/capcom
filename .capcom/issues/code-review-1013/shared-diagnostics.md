---
title: One per-file diagnostics module for index and check
status: done
type: task
priority: 3
deps: [left-out-name-claims, split-check-go, split-index-refresh, duplicate-id-subtree]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:43Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

The same file problems have two names, `CodeX` in `check` and `WarningX` in `index`, and three
mapping functions translate between them (`treeCode`, `walkWarningKind`, `readRefusal`). Two
duplicate detectors (`tree.Load` for `check`, the SQL `excludeClaims` for the index) disagree, as
`duplicate-id-subtree` shows. Tests such as `TestRun_DirectoryProblems_WordedAsTheIndexWarnsThem`
are all that keeps them in sync. Strength: worth exploring.

Wanted: one module, in `tree` or a new package `diagnose`, with a single Kind enum,
`FileProblems(kind, relPath, data, cfg) []Finding` and `NameClashes(claims) []Finding`. The index
caches the per-file result in `files.problems`, and `check` adds the cross-file rules. The rule
"check never reads the index" is kept: only the function is shared, and `diagnose` must not import
`index`. This settles `duplicate-id-subtree` and `left-out-name-claims` structurally, with one
duplicate detector and claims of left-out files recorded uniformly; both land first as local
fixes.

Risks: `check`'s JSON codes are a public interface; renaming one is a user-visible change and
needs a CHANGELOG note.

## Acceptance

- One problem-kind enum and one duplicate detector serve both `index` and `check`; `treeCode`,
  `walkWarningKind` and `readRefusal` are gone.
- `check --json` codes are unchanged, and `internal/check` does not import `internal/index`.
- `GOMEMLIMIT=2GiB go test ./internal/tree/`, then `./internal/index/`, `./internal/check/` and
  `./internal/cli/`, pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, A-06.
