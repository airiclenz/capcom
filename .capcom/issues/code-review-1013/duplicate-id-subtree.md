---
title: A repeated id leaks the dropped file's subtree and Aliases
status: done
type: bug
priority: 2
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:15Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

A repeated id leaks the dropped file's subtree and Aliases. When two files answer to one id,
`tree.Load` keeps the first by path, while the index and the documented rule drop both. The id
itself is refused for writes, but three leaks remain. Severity: Medium.

- (a) The dropped duplicate's subtree joins the kept node. With `a/x.md`, `b/x.md` and
  `b/x/c2.md` after a merge, `ChildrenOf("x")` lists `c2` under `a/x.md`, and
  `capcom know mv c2 --id c2-new` moves `b/x/c2.md` to `a/x/c2-new.md`, silently re-parenting it.
- (b) Aliases of a dropped file are invisible to claim checks. If the dropped `b/x.md` lists
  `aliases: [old-bar]`, `capcom create --id old-bar` or `capcom mv y --id old-bar` succeeds and
  writes a second holder of `old-bar`, breaking ADR 0002 ("a new id may not reuse an id or
  alias").
- (c) The `AGENTS.md` block lists a duplicated top-level Note, and every agent is told to run a
  `capcom know show` that errors. Low.

Wanted (decided 2026-10-13): `tree.Load` drops both files of a repeated id, and their subtrees are
reported as orphans; check's duplicate wording is unchanged. The index leaves out a repeated id's
subtree like an orphan directory, so every verb agrees. Create and Move refuse an id or Alias
that a dropped duplicate file claims, naming that file.

## Acceptance

- Tree tests: with `a/x.md`, `b/x.md` and `b/x/c2.md`, `ChildrenOf("x")` does not list `c2`,
  and a move of `c2` is refused or keeps it under `b/x`.
- Nodestore tests: `create --id old-bar`, where `old-bar` is an Alias of a dropped duplicate, is
  refused with the claim error naming the dropped file.
- Agents test: a duplicated top-level id is not rendered.
- Index test: the repeated id's subtree is left out of the index.
- `TestRun_DirectoryProblems_WordedAsTheIndexWarnsThem` still passes.
- `GOMEMLIMIT=2GiB go test` passes for `./internal/tree/`, `./internal/nodestore/`,
  `./internal/agents/` and `./internal/index/`, one package at a time.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-10.
