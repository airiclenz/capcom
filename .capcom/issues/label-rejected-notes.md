---
title: Rejected ADR option Notes are not marked as background
status: done
type: task
priority: 2
deps: [know-tree-background]
plan: 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan
created: 2026-10-12T18:13:07Z
closed: 2026-10-12T19:09:44Z
reason: Resolved by plan 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan, item 13 (archived in e61d7bf)
---

With background Notes hidden from `know tree`, the rejected-option Notes of the decision records
must carry the label for it to help. Every `adr-NNNN-rejected` Note is labelled `background`;
`adr-0010-decision` says new records do the same; the glossary gains the term `background`; and
`docs/format.md` documents the label.

## Acceptance

- `capcom know tree decisions` lists no `-rejected` Note; `--background` lists all eight.
- `capcom know show background` prints the glossary term.
- `capcom check` and `capcom fmt --check` report no problems.
