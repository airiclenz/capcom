---
title: Config plans directories skip the local-path check
status: done
type: bug
priority: 2
plan: 2026-11-03 - 00 - audit-urgent-fixes-plan
created: 2026-11-03T08:23:42Z
closed: 2026-11-03T09:20:04Z
reason: Resolved by plan 2026-11-03 - 00 - audit-urgent-fixes-plan item 5 (archived in 1dcb36a)
---

`plans` entries go through `nameList` only (`internal/config/parse.go:140`), so unlike
`agents.file` and `agents.guides` they are never checked with `localPath`:
`plans: [../../../etc, /srv/x, 'a\b']` is accepted although the documented contract is
"relative to the repository root". `PlanPath` (`internal/config/config.go:271`) then joins and
`os.Stat`s paths outside the worktree, and `check`, `context` and `ready` report a plan found or
missing and print a `../..` path.

Wanted: validate each `plans` entry with `p.localPath` in the `keyPlans` case, as `guides` does.

Source: docs/reviews/code-audit-2026-11-03.md.

## Why

A hostile repository's config can make capcom probe and report on files outside the worktree.

## Acceptance

- A config with `plans: [../x]`, `plans: [/abs]` or a backslash path fails to load, naming the offending line.
- The default `plans` list and ordinary relative paths still load.
