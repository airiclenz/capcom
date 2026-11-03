---
title: The staged catalog's filters are not tested against the index
status: open
type: task
priority: 3
plan: 2026-11-03 - 02 - eval-harness-and-test-gaps-plan
created: 2026-11-03T08:23:42Z
---

`isSelected` (`internal/stage/catalog.go:241`) re-implements the index query filter in memory,
but `catalog_test.go` exercises only a few fields. A divergence from `index.List` (Statuses,
Categories, Types, Priorities, ParentID, Plan, Labels/NoLabels, IsClosedIncluded) would silently
change what a staged batch selects.

## Acceptance

- A table test applies each filter field to the same issues through `Catalog.List` and `index.List` and asserts equal ids.
