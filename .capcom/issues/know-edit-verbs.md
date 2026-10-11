---
title: Notes cannot be changed, moved or removed through capcom
status: done
type: feature
priority: 2
labels: [knowledge, cli]
plan: 2026-10-11 - 03 - know-edit-verbs-plan
created: 2026-10-11T17:48:07Z
closed: 2026-10-11T18:36:37Z
reason: "Resolved by plan 2026-10-11 - 03 - know-edit-verbs-plan (00332bf): know update, know mv, know rm and Note etags"
---

Notes can be added but not changed through capcom. `capcom know` has `add`, `search`, `show`,
`tree` and `verify`; issues have `update`, `mv` and `close`. Changing a Note's title, labels,
refs or body, renaming it, moving it below another Note or removing it means editing the files
under `.capcom/knowledge/` by hand and running `capcom check` afterwards.

Found while moving AGENTS.md reference material into Notes: a wrong sentence in the
`error-messages` body could only be fixed by hand.

Wanted: verbs that mirror the issue ones, at least `capcom know update` (title, labels, refs,
body) and `capcom know mv` (rename, reparent), each refreshing the AGENTS.md managed block when
the top level changes. Whether Notes need a removal verb and Aliases on rename is for the plan.
