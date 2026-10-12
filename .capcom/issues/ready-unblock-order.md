---
title: ready ignores how much work an issue unblocks when ordering a priority
status: open
type: feature
priority: 2
deps: [unblocks-query]
plan: 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan
created: 2026-10-12T18:13:07Z
---

`capcom ready` orders by priority, then by tree position, so an issue that holds up three others
ranks no higher than a leaf of the same priority. Within a priority, issues that would make more
issues Ready come first, and their line says so with `-- unblocks N`; JSON items carry `unblocks`.
Priority still always wins.

## Acceptance

- Of two Ready issues with the same priority, the one whose close makes another issue Ready is
  listed first and ends with `-- unblocks 1`.
- A lower-priority issue never outranks a higher-priority one, whatever it unblocks.
- `prime` lists ready issues in the same order; JSON `unblocks` lists the ids.
