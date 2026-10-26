---
title: A stale index serves wrong results when size and mtime are unchanged
status: open
type: bug
priority: 1
created: 2026-10-26T20:06:45Z
---

The index decides whether a file changed by comparing only its size and modification time, so a
file that changes without changing either is never re-read. `capcom list` and `capcom show` then
disagree about the same issue.

Reproduced: set an issue's mtime far in the past, index it, then replace the title with another
of the same byte length and restore the old mtime (`cp -p`, `rsync -t`, `tar -x` and some
editors' atomic saves all do this naturally):

    capcom list      -> s1  open  P2  Alpha original title
    capcom show s1   -> title: Alpha HACKED title!!
    capcom search HACKED -> no issue matches
    capcom reindex; capcom list -> Alpha HACKED title!!

So the index silently serves an old title, and `search` misses the issue entirely.

This is the worst failure mode for a store whose promise is that the files are the truth: the
tool quietly contradicts itself, and only `reindex` fixes it.

## Acceptance

- A change to a file's contents is picked up even when its size and mtime are unchanged.
- `list`, `show` and `search` never disagree about one file's fields.
- A test rewrites a node file preserving length and mtime, then asserts `list` shows the new
  title, at a cost that does not make `list` read every body on every call.
