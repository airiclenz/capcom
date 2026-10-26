---
title: A rename leaves path-shaped Note refs stale and check failing
status: open
type: bug
priority: 3
created: 2026-10-26T20:06:50Z
---

Renaming an issue rewrites deps and related links but not a Note's `refs:` entry that points at the
issue's file, so the rename leaves `capcom check` failing.

Reproduced: a Note with `refs: [.capcom/issues/beta-new.md]`, then

    capcom mv beta-new beta-final   -> exit 0, id: beta-final
    grep refs .capcom/knowledge/nref.md -> refs: [.capcom/issues/beta-new.md]   (stale)
    capcom check -> .capcom/knowledge/nref.md:3: ref ".capcom/issues/beta-new.md": no such file or directory   exit 1

A Ref is a path by definition, so an alias never rescues it. Reporting a broken Ref, not rewriting
it, is the designed behaviour for files capcom does not own, and `capcom mv --help` promises only
that aliases resolve and deps are rewritten. But the moved file is capcom's own, it knows the old
and new path at the moment of the move, and an agent that renames and moves on leaves a store that
fails `check` for a reason it caused.

## Acceptance

- A move rewrites Note Refs that point at a file or directory it moved, including a `path#symbol`
  Ref, for `mv` (rename and `--parent`) and `know mv` alike.
- A test renames an issue that a Note refs by path and asserts `capcom check` still exits 0.
