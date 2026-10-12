---
title: The AGENTS block omits dependencies, the issue-body rule and the store layout
status: done
type: feature
priority: 2
deps: [close-now-ready, show-readiness, create-dep-flag, dep-list-depth, list-tree-outline, know-tree-background]
plan: 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan
created: 2026-10-12T18:13:07Z
closed: 2026-10-12T19:09:44Z
reason: Resolved by plan 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan, item 14 (archived in e61d7bf)
---

The AGENTS.md block that `capcom agents` writes is often the only capcom guidance an agent sees.
It does not mention dependencies, says nothing about what an issue body should hold, and does not
explain the file layout, which is hard to work out from `ls` (knowledge-organization review,
findings 1 and 2). The block adds:
- `dep add`/`dep list`, `create --dep`, `list --tree --depth 2`, and notes that `show` and `close`
  report blocking and readiness;
- the rule that an issue's body states what, why and `## Acceptance` on its own, and a plan says how;
- a short map of where issues, their children and Notes live.

## Acceptance

- `capcom agents` writes the new text; the block test pins it.
- The repository's own AGENTS.md is refreshed, and the managed-block section of `docs/format.md`
  matches.
