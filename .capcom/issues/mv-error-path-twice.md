---
title: capcom mv onto a parent with a symlinked children directory prints the path twice
status: done
type: bug
priority: 3
plan: 2026-11-03 - 01 - remaining-small-bugs-plan
created: 2026-11-03T09:49:54Z
closed: 2026-11-03T10:21:23Z
reason: "Resolved by plan '2026-11-03 - 01 - remaining-small-bugs-plan' item 4 (archived in 3ecf9f7): planFailure returns the *tree.Error, so mv names the children directory once"
---

`capcom mv bar --parent foo`, where `.capcom/issues/foo` is a symlink, fails with the path twice:

```
capcom: .capcom/issues/bar.md: .capcom/issues/foo: children directory of foo is not a real directory
```

`Tree.PathFor` (since 134cf37) already returns a `*tree.Error` naming the children directory.
`nodestore.Kind.planFailure` passes it to `fileError(node.Path, err)`, which prefixes the moved
node's file. `capcom create --parent foo` names the directory once, because `Create` returns the
`*tree.Error` through `resolveFailure`.

## Why

The message names two files, and the first one (`bar.md`) is not the problem. The error-messages
convention says an error names the one file it is about.

## Acceptance

- `capcom mv bar --parent foo` with a symlinked or file `foo` children path prints
  `.capcom/issues/foo: children directory of foo is not a real directory` once, with no
  `.capcom/issues/bar.md:` prefix, and moves nothing.
- `errors.Is(err, tree.ErrNotDirectory)` still holds for the error `nodestore.Kind.Move` returns.
- Other `mv` failures (unknown parent, taken id, unreadable node file) print as before.
