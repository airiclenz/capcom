---
title: README and format reference do not cover the dependency output and issue outline
status: done
type: task
priority: 2
deps: [close-now-ready, ready-unblock-order, show-readiness, show-child-progress, dep-list-depth, list-tree-outline, dep-graph-mermaid]
plan: 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan
created: 2026-10-12T18:13:07Z
closed: 2026-10-12T19:09:44Z
reason: Resolved by plan 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan, item 17 (archived in e61d7bf)
---

The new output must be documented where users look. The README's Usage block shows `create
--dep`, `dep list --depth 2`, `dep graph` and `list --tree`. `docs/format.md` documents:
- `close`'s `now ready:` line;
- `show`'s `ready:`, `dependents:` and `children:` lines;
- `ready`'s order and `-- unblocks N`;
- `dep graph`'s output;
- each with its JSON field, and the order of `show`'s lines where the doc pins it.

## Acceptance

- Every documented string matches what the code emits.
- README examples run as shown against a store that has the issues they name.
