---
title: Issue rename leaves the new id twice in deps or related
status: done
type: bug
priority: 3
deps: [reference-rewrite-engine]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:44Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

Issue rename leaves the new id twice in `deps` or `related`. `issue`'s `renameDeps`, applied to
both lists by `PlanMove`, skips a renamed entry only when the new id is already in its output, so
an entry already equal to the new id that comes after the renamed one is kept as well; Note
`renameSuccessors` dedupes correctly. `issue.Validate` does not reject repeated entries, so the
file is written. The dependency graph tolerates the repeat, but the file and `show` carry it, and
no test covers this order. Severity: Low.

Trigger: Issue `a` has `deps: [x, y]` with `y` dangling, and `capcom mv x y` rewrites `a` to
`deps: [y, y]`; `related: [x, y]` becomes `[y, y]` the same way (`[y, x]` comes out right).

Wanted: dedupe on the output as `renameSuccessors` does: replace a renamed entry with the new id,
then skip any entry equal to the new id once one is kept. `reference-rewrite-engine` replaces both
functions with one rename op, so fix it there unless that refactor is declined.

## Acceptance

- An `issue` test renames `x` to the dangling `y` with `deps: [x, y]` and `related: [x, y]` on
  another Issue and asserts both lists are `[y]`.
- `GOMEMLIMIT=2GiB go test ./internal/issue/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, U-02.
