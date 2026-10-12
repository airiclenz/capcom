---
title: show gives no sign of how many children are closed
status: done
type: feature
priority: 2
deps: [show-readiness]
plan: 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan
created: 2026-10-12T18:13:07Z
closed: 2026-10-12T19:09:43Z
reason: Resolved by plan 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan, item 5 (archived in e61d7bf)
---

A parent issue gives no sign of how far along its children are. `capcom show` on an issue with
direct children prints `children: <closed>/<total> closed`; JSON gains `children` with `total`
and `closed`.

## Acceptance

- A parent with one open child shows `children: 0/1 closed`, and `children: 1/1 closed` once that
  child is closed.
- An issue without children prints no `children:` line and no JSON field.
