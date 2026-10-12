---
title: create cannot record dependencies, so filing a waiting issue takes two commands
status: done
type: feature
priority: 2
plan: 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan
created: 2026-10-12T18:13:07Z
closed: 2026-10-12T19:09:43Z
reason: Resolved by plan 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan, item 6 (archived in e61d7bf)
---

Filing an issue that waits on another takes two commands, `create` then `dep add`, and smaller
models drop the second. `capcom create --dep <id|alias|prefix>`, repeatable, writes the
dependencies into the new file directly, each as its canonical id, once.

## Acceptance

- A dependency given by alias is stored as the issue's id; one given twice is stored once.
- A dependency that names no issue fails the create with the same error `dep add` gives, and no
  file is written.
- `capcom dep list <dependency>` lists the new issue as a dependent.
