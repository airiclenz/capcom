---
title: A replace-mode write recreates a Node another session moved or removed
status: open
type: bug
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:15Z
---

A replace-mode write recreates a Node that another session moved or removed. The atomic write
renames a temporary file over the target and `writeTemp` runs `os.MkdirAll`, so a write landing
after the target vanished brings it back. Severity: Low.

Trigger: A edits `x` while B moves or removes `x` in the gap between A's re-read and its rename.
A's write brings `x` back at its old path, so two files claim `x` (B kept `x` as an Alias), or a
removed Issue reappears. A create under a parent being removed recreates `<parent>/` with no node
file. The result is visible and recoverable.

Wanted: replace mode has a variant that refuses a missing target (`Lstat` just before the rename,
no `MkdirAll`), and Create and Move check that the parent node file (`<dir>.md`) still exists
right before placing.

## Acceptance

- A tree test removes the target between plan and write and asserts the replace write fails and
  creates no directory.
- A test removes the parent node file before a create or move places its file and asserts the
  write is refused.
- `GOMEMLIMIT=2GiB go test ./internal/tree/`, then `./internal/nodestore/`, pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-11(b).
