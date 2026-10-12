---
title: No outline of the issue hierarchy
status: done
type: feature
priority: 2
deps: [show-child-progress, outline-depth-hint]
plan: 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan
created: 2026-10-12T18:13:07Z
closed: 2026-10-12T19:09:43Z
reason: Resolved by plan 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan, item 10 (archived in e61d7bf)
---

There is no way to see the issue hierarchy at a glance; `list --parent` shows one level. `capcom
list --tree [--depth N] [--parent <id>] [--all]` prints the issues as an indented outline in the
shape of `know tree`, each parent ending with `(<closed>/<total> closed)`. This keeps a large
store navigable without a generated index (knowledge-organization review, finding 7).

## Acceptance

- A parent with an open child prints the child indented under it and `(0/1 closed)`.
- Closed issues are left out unless `--all`, but counted in the progress; `--depth` cuts print the
  outline hint; `--parent` starts at that issue.
- `--tree` combined with another row filter, or `--depth` without `--tree`, is a usage error that
  names the flag.
