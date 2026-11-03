---
title: An unclosed code fence hides conflict markers from check
status: done
type: bug
priority: 2
plan: 2026-11-03 - 00 - audit-urgent-fixes-plan
created: 2026-11-03T08:23:42Z
closed: 2026-11-03T09:20:04Z
reason: Resolved by plan 2026-11-03 - 00 - audit-urgent-fixes-plan item 7 (archived in 1dcb36a)
---

`ConflictMarkers` (`internal/rawfm/conflict.go:45`) tracks fences with the streaming
`CodeFence.IsCode`, so a fence opener no later line closes (a stray three-backtick line or a
`~~~` typo) makes every following line count as code and skips all conflict markers after it.
`ClosedCodeLines` (`internal/rawfm/fence.go:39`, used by `issue/mutate.go`) follows the opposite
rule: a block left open hides nothing.

Wanted: compute the code mask with `ClosedCodeLines(lines, bodyStart)` and skip only the lines it
marks, so both consumers share one definition of a code line.

Source: docs/reviews/code-audit-2026-11-03.md.

## Why

A body with one unbalanced fence line and a later merge that leaves `<<<<<<<`, `=======` and
`>>>>>>>` below it passes `capcom check` and the frontmatter decoder, so a file with conflict
markers can be committed.

## Acceptance

- A test runs a body of an unclosed fence line, text, then a full conflict block through `check` and reports the conflict.
- Conflict markers inside a properly closed fence are still ignored.
