---
title: No test for WriteFileExclusive's copy fallback without hard links
status: open
type: task
priority: 3
plan: 2026-10-12 - 00 - code-audit-fixes-plan
created: 2026-10-12T16:43:02Z
---

Deferred from the code-audit-fixes plan, item 14: the `O_CREATE|O_EXCL` copy fallback that
`WriteFileExclusive` uses on file systems without hard links has no test. `placeExclusive`
(internal/tree/atomic.go:130) falls back to `copyExclusive` (internal/tree/atomic.go:149) only when
`os.Link` fails, so reaching it needs an injected link failure.

## Acceptance

- internal/tree/atomic_test.go injects a failing link and covers the copy fallback: the new file
  is written whole, and an existing path fails with `fs.ErrExist` and is left as it was.
