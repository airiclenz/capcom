---
title: A symlinked children directory sends mv and child creation outside the store
status: open
type: bug
priority: 2
plan: 2026-11-03 - 00 - audit-urgent-fixes-plan
created: 2026-11-03T08:23:42Z
---

`planRenames` and `PlanMove` (`internal/tree/move.go:151`, `:163`) use `os.Stat`, which follows
symlinks, on the new and children directories, and `PathFor` (`internal/tree/tree.go:290`) joins
a children directory without an `Lstat`. `Walk` reports a symlink `issues/foo` as a problem but
keeps `issues/foo.md`.

With a committed symlink `issues/foo -> /elsewhere` beside `foo.md`, `capcom mv bar --parent foo`
(or creating a child of `foo`) skips the mkdir step and writes `bar.md` through the link into an
arbitrary directory; the node vanishes from the repository.

Wanted: `Lstat` the destination and children directories in `planRenames`, `PlanMove` and the
create path, and refuse anything that is not a real directory.

Source: docs/reviews/code-audit-2026-11-03.md.

## Why

A hostile repository can make an ordinary capcom write land outside the worktree and lose the node.

## Acceptance

- With `issues/foo` a symlink to a directory outside the store, `capcom mv bar --parent foo` fails naming the symlink and writes nothing.
- Creating a child of `foo` in the same setup fails the same way.
- Tests cover both, on platforms that support symlinks.
