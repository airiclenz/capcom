---
title: No call lists a Parent's descendants by status
status: done
type: feature
priority: 3
plan: 2026-10-27 - 01 - tree-status-filter-plan
created: 2026-10-26T07:51:30Z
closed: 2026-10-27T10:11:16Z
reason: "Resolved by plan 2026-10-27 - 01 - tree-status-filter-plan (archived eb1df67): list --tree combines with the row filters, marking context rows in text and --json; --help documents it."
---

There is no single call that lists the descendants of a Parent filtered by status.
`capcom list --tree --status <s>` fails with `--status cannot be combined with --tree`, and the flat
`capcom list --parent <id>` returns direct children only.

Why: in the field evaluation (capcom 0.9.1, experiment `eval-json-count`) the task "count the
issues that are not closed below this area, children of children included" took two rejected
flag combinations and a client-side filter over `list --tree --all --parent <id> --json` by path
prefix and status (about three extra calls). An agent that stops at the plain `--tree` output gets
an answer that looks right but leaves out every closed descendant.

Wanted: `--status`, `--category`, `--label` and `--type` combine with `--tree`. Rows that match print
as today; a non-matching ancestor of a matching row still prints, marked as context, so the
outline stays readable. The `(closed/total)` counts keep counting every child.

## Acceptance

- `capcom list --tree --parent <id> --status open` prints every open descendant of `<id>` at every
  depth `--depth` allows, and nothing else but their ancestors marked as context.
- `--json` marks the context rows, so a count needs no client-side filter.
- `capcom list --help` states how the filters apply under `--tree`.
