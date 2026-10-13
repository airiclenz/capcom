---
title: check, elsewhere and lock take the store, not cwd
status: open
type: task
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:44Z
---

Entry points take `cwd` and open the store themselves. `briefing.Compose` opens the store, then
`check.Run(cwd)` opens it again, repeating git discovery and the config parse. The Issue-tree
pathspec for Elsewhere is computed in both `briefing` and `cli`. Lock callers pair `lock.Open`
with `WorktreeKey` by hand. Six commands open the store themselves (`check`, `agents`, `fmt`,
`lock`, `prime`, `init`). Strength: speculative to worth exploring.

Wanted: `check.Run(*store.Store)`, `elsewhere.Scan(store, now)` or `store.IssuesPathspec()`, and a
`lock.ForWorktree(store)` handle with the key bound. A `store` value on the CLI's command
annotation removes four of the six manual opens; `prime` and `init` stay special, and `prime`
must keep turning a store failure into a warning (ADR 0009).

## Acceptance

- `check.Run` takes the store, and a session-start briefing opens the store once.
- The Elsewhere Issue-tree pathspec is computed in one place, and lock callers no longer pair
  `lock.Open` with `WorktreeKey` by hand.
- `prime`'s store-failure warning test passes unchanged.
- `GOMEMLIMIT=2GiB go test ./internal/check/`, then `./internal/briefing/`, `./internal/lock/` and
  `./internal/cli/`, pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, A-10.
