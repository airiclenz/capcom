---
title: show prints neither readiness nor dependents
status: open
type: feature
priority: 2
plan: 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan
created: 2026-10-12T18:13:07Z
---

`capcom show` prints an issue's raw `deps` list but not whether the issue is Ready, why not, or
which issues wait on it, so an agent needs `ready --all` and `dep list` as well. `show` prints
`ready: yes` or `ready: no (<reasons>)` for todo issues, worded as `ready --all` words them, and
`dependents: [...]` for issues others depend on; JSON gains `ready`, `blocking` and `dependents`.

## Acceptance

- A blocked todo issue shows `ready: no (blocked by …)`; a Ready one shows `ready: yes`; a done
  issue shows no `ready:` line.
- An issue others depend on lists them on a `dependents:` line after `deps:`.
- The output of every other command is unchanged.
