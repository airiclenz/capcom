---
title: know tree --depth cuts children without saying so
status: done
type: feature
priority: 2
plan: 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan
created: 2026-10-12T18:13:07Z
closed: 2026-10-12T19:09:43Z
reason: Resolved by plan 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan, item 8 (archived in e61d7bf)
---

`capcom know tree --depth N` stops silently at the cap, so a reader cannot tell a leaf from a
Note whose children were cut. Under each cut Note it prints `... <n> more (--depth <d>)`, and
JSON carries `more`. The indented-outline rendering moves into one shared renderer that
`dep list --depth` and `list --tree` reuse.

## Acceptance

- `know tree --depth 1` on a tree with grandchildren prints the hint with the right count and depth.
- With no cut, the output is byte-identical to the current output; retired children are not counted.
