---
title: Align status and priority in sidebar columns
status: done
type: feature
priority: 2
labels: [ui]
plan: 2026-10-16 - 02 - ui-apogee-colors-columns-version-plan
created: 2026-10-16T16:12:45Z
closed: 2026-10-16T16:54:45Z
reason: Delivered by plan 2026-10-16 - 02 - ui-apogee-colors-columns-version-plan (item 4), archived in 8907e8f
---

In capcom ui's sidebar each row prints `id  status  P<n>  title` with two spaces between fields,
so the status and priority start at a different column on every row: after a longer id, and
further right on a deeper tree row. They are hard to scan down.

Wanted (decided by the user, 2026-10-16): the status and the priority each start at one column
for every row of the Issues tree, however deep the row sits, and of the Ready list. The id
column is as wide as the widest connector, arrow and id of the rows shown; the status column as
wide as the widest status. The title follows. Knowledge rows are unchanged.

## Acceptance

- On the Issues tab with nested issues expanded, every row's status starts at one column and
  every `P<n>` at one column.
- The same holds for the Ready list.
