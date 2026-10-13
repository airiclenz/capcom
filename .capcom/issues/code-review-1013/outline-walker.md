---
title: One outline and progress walker for list and know tree
status: done
type: task
priority: 3
deps: [split-know-go]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:43Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

`list --tree` (`issueTree.visit`, `add` and `progress`, `issuesByParent`) and `know tree`
(`outline`, `childrenByParent`) each own an outline walker. `show`'s `childProgress` runs two SQL
queries per Issue, while `list` computes the same progress in memory. "List every Issue including
closed, then convert to `[]issue.Node`" is repeated in `close`, `dep` and `list`. Strength:
worth exploring.

Wanted: one generic `walkOutline[E](children, start, maxDepth, isShown)` beside `renderOutline` in
`outline.go`, and one `index.AllIssueNodes()` (or `issue.Progress(children)`) used by `show`,
`list` and `close`, so `--depth` behaves the same in `list --tree` and `know tree`.

Risks: the outline text output is pinned by tests and must stay byte-identical.

## Acceptance

- `list --tree` and `know tree` walk through the one generic walker, and the Issue-node list and
  child progress come from one function.
- The existing `list`, `know tree`, `show` and `close` tests pass unchanged.
- `GOMEMLIMIT=2GiB go test ./internal/index/`, then `./internal/cli/`, pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, A-08.
