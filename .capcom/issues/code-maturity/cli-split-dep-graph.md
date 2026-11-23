---
title: Move the indexed Dependency graph out of internal/cli into internal/index
status: open
type: task
priority: 3
labels: [maturity, refactor]
plan: 2026-11-23 - 01 - internal-cli-split-plan
created: 2026-11-23T08:01:16Z
---

The Dependency graph of the indexed Issues that `dep graph`, `dep list`, `related` and
`context` load (`depGraph`, `loadDepGraph`, `names`, `shownBy` in
`internal/cli/dep_graph.go`; `depBranches` in `internal/cli/dep_list.go`) lives in the
command package. Move it into `internal/index`, with direct unit tests; the
rendering stays in cli. Item 6 of the plan.

## Why

Four commands share the graph; moving it beside the index it reads makes it testable without runCLI.

## Acceptance

- `internal/index` holds the graph with direct tests; `internal/cli` defines no depGraph type, loadDepGraph or depBranches
- `dep graph`, `dep list`, `related` and `context` print byte-identical output
