---
title: dep add and close decide on the index snapshot outside the guarded edit
status: open
type: bug
priority: 3
deps: [issue-lifecycle-module]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:44Z
---

`dep add` and `close` decide on the index snapshot, outside the guarded edit. `dep add` runs
`FindCycle` on `loadDepGraph`, which reads index rows, and `close` takes the Issue's category from
the index entry; its change callback then overwrites `Status`, `Closed` and `Reason` on whatever
the file holds. Severity: Low.

Trigger: `dep add a b` and `dep add b a` run at once; both pass the cycle check and both commit,
so the store holds a cycle (`check` reports it). Two concurrent `close X` runs: the second
overwrites the first's closed time and reason, and both announce the same "now ready".

Wanted (decided 2026-10-13: local compare-and-swap only, no store-wide write lock): inside the
change callback, re-check the fresh file's category and return `already_closed` without writing;
once `issue-lifecycle-module` lands this is `issue.Close`'s own rule. The dep-cycle race stays,
reported by `check`: a re-check inside the callback would only narrow the window, because the
reverse edge lives in another file.

## Acceptance

- A CLI test closes an already-closed file inside the change callback, through a seam, and asserts
  `already_closed` and an unchanged `closed:` time.
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Close|Dep'` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-11(d).
