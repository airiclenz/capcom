---
title: No Note on filing, claiming and closing issues with dependencies
status: done
type: task
priority: 2
deps: [close-now-ready, ready-unblock-order, dep-list-depth, list-tree-outline, dep-graph-mermaid, agents-block-map]
plan: 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan
created: 2026-10-12T18:13:07Z
closed: 2026-10-12T19:09:44Z
reason: Resolved by plan 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan, item 16 (archived in e61d7bf)
---

The new dependency commands and the issue conventions need one place an agent reads before
working an issue. A top-level Note `issue-workflow` covers:
- the self-contained issue body: what, why and `## Acceptance`, while the plan says how, so
  archiving a plan strands no issue;
- claiming work: a doing status plus the Tree lock with `--purpose <id>`; `prime` lists doing issues;
- recording and reading dependencies: `create --dep`, `dep add`, `dep list --depth`, `dep graph`,
  `list --tree`, what `close` reports and how `ready` ranks.

## Acceptance

- `capcom know tree --depth 1` lists `issue-workflow`, and the AGENTS.md block lists it after
  `capcom agents`.
- `capcom check` and `capcom fmt --check` report no problems.
