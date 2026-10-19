---
title: Shell completion does not complete issue or Note ids
status: open
type: feature
priority: 2
created: 2026-10-19T14:54:45Z
---

Shell completion completes commands and flags but not Issue or Note ids:
`capcom __complete show stand` returns nothing. A human typing ids by hand mistypes them, and
the ids are chosen slugs that are hard to remember exactly.

Wanted: completion of Issue ids and aliases for every argument and flag that takes an Issue
(`show`, `update`, `close`, `dep add`, `--parent`, `--dep`, ...), Note ids for `know` verbs,
and configured statuses and types for `--status` and `--type`. Each candidate carries its
title as the description, for shells that show one. Ids come from the index, so completion
stays fast.

## Acceptance

- `capcom __complete show stand` lists `standing-parents` with its title.
- `capcom __complete update x --status ""` lists the configured statuses.
- Closed Issues are offered after open ones, or omitted for `update --status`.
- Tests cover Issue ids, Note ids, statuses and types.
