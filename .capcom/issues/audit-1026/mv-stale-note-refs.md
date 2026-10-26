---
title: A rename leaves path-shaped Note refs stale and check failing
status: open
type: bug
priority: 2
created: 2026-10-26T20:06:50Z
---

Renaming an issue rewrites deps and related links but not a Note's `refs:` path, so the rename
leaves the store broken.

Reproduced: a Note with `refs: [.capcom/issues/beta-new.md]`, then

    capcom mv beta-new beta-final   -> exit 0, id: beta-final
    grep refs .capcom/knowledge/nref.md -> refs: [.capcom/issues/beta-new.md]   (stale)
    capcom check -> .capcom/knowledge/nref.md:3: ref ".capcom/issues/beta-new.md": no such file or directory   exit 1

`deps:` and `related:` are rewritten correctly, and an id-shaped ref survives only because the
old id stays an alias; a path-shaped ref gets no such rescue.

`capcom check` catches it, so it is a footgun rather than silent loss — but `capcom mv --help`
promises references are kept working, and an agent that renames and moves on leaves a store that
fails `check`.

## Acceptance

- A rename rewrites path-shaped `refs:` entries in Notes, as it already does for `deps` and
  `related`.
- A test renames an issue that a Note refs by path and asserts `capcom check` still exits 0.
