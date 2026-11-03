---
title: ReplaceFileVerbatim can recreate a file deleted between check and write
status: done
type: bug
priority: 2
plan: 2026-11-03 - 00 - audit-urgent-fixes-plan
created: 2026-11-03T08:23:42Z
closed: 2026-11-03T09:20:04Z
reason: Resolved by plan 2026-11-03 - 00 - audit-urgent-fixes-plan item 8 (archived in 1dcb36a)
---

`ReplaceFileVerbatim` (`internal/nodefile/atomic.go:81`) guards "fails when path is missing" with
a bare `os.Stat` before `WriteFileAtomicVerbatim`, which calls `writeFileAtomic(...,
isReplacing=false)`, runs `MkdirAll` and renames over a missing path. If another session moves or
deletes the node file between the Stat and the rename, the old path is silently recreated: the
case `ReplaceFileAtomic` already prevents with `checkReplaceTarget`.

Wanted: route the verbatim mode through `writeFileAtomic` with `isReplacing=true`, so the
missing-target check and the no-mkdir rule apply.

Source: docs/reviews/code-audit-2026-11-03.md.

## Why

Parallel agent sessions share worktrees; a moved or deleted node must not reappear at its old path.

## Acceptance

- A test removes the target between the existence check and the rename (or calls the write path with the target missing) and asserts the replace fails and creates no file or directory.
- CRLF endings and the file mode still survive an ordinary verbatim replace.
