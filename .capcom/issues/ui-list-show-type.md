---
title: Show the issue type in the capcom ui issue lists
status: open
type: feature
priority: 2
plan: 2026-11-03 - 04 - ui-list-type-column-plan
created: 2026-11-03T18:15:49Z
---

The issue lists in capcom ui do not show an issue's type (bug, feature, task, ...). A reviewer has to open an issue to see what kind of work it is, which makes scanning a long list slower than it needs to be.

Add a type column to the issue lists in capcom ui, after the priority: id, status, priority (PX), type, description. Every column must be vertically aligned, so rows read like a table.

## Acceptance
- The issue lists in capcom ui show the issue's type as a column, after the priority.
- Columns line up vertically across rows: the type column starts at the same character position in every row.
- The layout still renders sensibly when a type label is longer than the column width (no broken rows).
- The existing columns (id, status, priority, description) keep their meaning and order.
