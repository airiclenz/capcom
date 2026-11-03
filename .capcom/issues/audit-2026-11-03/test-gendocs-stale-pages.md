---
title: cmd/gendocs deletes files with no test
status: open
type: task
priority: 3
created: 2026-11-03T08:23:42Z
---

`run` and `removeStalePages` (`cmd/gendocs/main.go:44`) delete every `capcom*.md` in the output
directory before regenerating, and `cmd/gendocs` has no test file. A regression in the glob or
the order would silently delete or leave stale CLI reference pages, and a bad `-out` removes
unrelated files.

## Acceptance

- A test calls `run(t.TempDir())` with a pre-seeded `capcomold.md` and an unrelated `notes.md` and asserts the stale page is gone, the unrelated file survives and the reference pages are written.
