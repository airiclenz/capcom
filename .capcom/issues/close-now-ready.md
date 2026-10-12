---
title: close does not say which issues it made Ready
status: open
type: feature
priority: 2
deps: [unblocks-query]
plan: 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan
created: 2026-10-12T18:13:07Z
---

After `capcom close`, an agent has to run `capcom ready` again to learn what it can do next. The
close itself should say so: the text output ends with `now ready: <id>, <id>` naming the todo
issues the close made Ready, and each JSON entry carries `now_ready`.

## Acceptance

- Closing the only dependency of an issue prints `now ready: <that issue>`, also when closed as
  dropped and when the dependency is written as an alias.
- No line is printed when nothing became Ready; issues hidden by `ready.hide` are not named.
- Closing several ids at once names each newly Ready issue once; JSON `now_ready` is present on
  every entry.
