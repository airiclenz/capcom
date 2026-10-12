---
title: Resolve misses a duplicate alias of a file dropped as a duplicate id
status: open
type: bug
priority: 3
plan: 2026-10-12 - 00 - code-audit-fixes-plan
created: 2026-10-12T16:43:02Z
---

Deferred from the code-audit-fixes plan, item 5: a name aliased only by a file that `tree.Load`
drops as a duplicate id (and also by a kept file) is not refused by `Resolve`
(internal/tree/resolve.go:56), though the index treats it as a duplicate. `Load` records duplicate
names from the ids of every walked file (internal/tree/tree.go:370) but reads aliases only for the
files it keeps (internal/tree/tree.go:436), so the dropped file's aliases never reach
`t.duplicates`.

## Acceptance

- `Resolve` refuses a name that a kept file and a file dropped as a duplicate id both claim, as
  the index does.
- internal/tree/resolve_test.go covers that case.
