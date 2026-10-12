---
title: dep list shows only direct dependencies and dependents
status: done
type: feature
priority: 2
deps: [unblocks-query, outline-depth-hint]
plan: 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan
created: 2026-10-12T18:13:07Z
closed: 2026-10-12T19:09:43Z
reason: Resolved by plan 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan, item 9 (archived in e61d7bf)
---

`capcom dep list` shows only direct dependencies and dependents, so "everything this waits on"
takes repeated calls. `dep list <id> --depth N` prints both sections as indented trees: default 1
keeps the current output, 0 is unlimited, an issue seen earlier in a section prints as
`(see above)`, and a cut branch prints the outline hint.

## Acceptance

- A three-level chain at `--depth 2` prints two levels and the hint; `--depth 0` prints all.
- A diamond prints the shared issue once in full and once as `(see above)`.
- Without `--depth` the output matches the current output; a negative depth is a usage error.
