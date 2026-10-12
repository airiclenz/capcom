---
title: No visual export of the dependency graph
status: done
type: feature
priority: 2
deps: [unblocks-query]
plan: 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan
created: 2026-10-12T18:13:07Z
closed: 2026-10-12T19:09:43Z
reason: Resolved by plan 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan, item 11 (archived in e61d7bf)
---

People reviewing a plan want to see the dependency graph, not read lists. `capcom dep graph [<id>]`
prints a Mermaid flowchart (rendered by GitHub and most editors): a node per issue labelled
`<id> (<status>)`, and an edge from each dependency to its dependent. Without an id it covers
every open issue; with one, that issue's transitive deps and dependents. `--all` adds closed
issues. JSON gives the nodes and edges.

## Acceptance

- On a small store the output is the exact expected flowchart, starting `flowchart LR`.
- With an id, only issues reachable from it appear; a closed dependency appears only with `--all`.
- Missing dependencies are left out without error.
