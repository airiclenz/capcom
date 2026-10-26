---
title: A stale index serves wrong results when size and mtime are unchanged
status: open
type: bug
priority: 2
created: 2026-10-26T20:06:45Z
---

The index decides whether a file changed by comparing only its size and modification time
(`internal/index/refresh.go:210`, recorded by the walk at `internal/index/walk.go:67-68`), so a
file that changes without changing either is never re-read. `capcom list` and `capcom show` then
disagree about the same issue.

Reproduced: set an issue's mtime far in the past, index it, then replace the title with another
of the same byte length and restore the old mtime:

    capcom list      -> s1  open  P2  Alpha original title
    capcom show s1   -> title: Alpha HACKED title!!
    capcom search HACKED -> no issue matches
    capcom reindex; capcom list -> Alpha HACKED title!!

So the index silently serves an old title, and `search` misses the issue entirely.

The trigger is narrow: copying or extracting an older file of the same length over the current one
with its mtime kept (`cp -p`, `tar -x`). git checkout sets the mtime to now, and `rsync -t` skips a
file whose size and mtime match. But when it happens the tool quietly contradicts itself, which is
the worst failure for a store whose promise is that the files are the truth, and only `reindex`
fixes it.

git meets the same problem by also recording ctime, inode and device: they come from the stat the
walk already makes, and ctime changes on every content write and cannot be set from user space.
Windows' `FileInfo` carries no change time, so it needs its own source.

## Acceptance

- A change to a file's contents is picked up even when its size and mtime are unchanged, without
  reading file bodies on every refresh.
- `list`, `show` and `search` never disagree about one file's fields.
- A test rewrites a node file preserving length and mtime, then asserts `list` shows the new
  title, and passes on Linux, macOS and Windows.
