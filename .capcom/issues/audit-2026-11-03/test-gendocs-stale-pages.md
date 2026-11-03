---
title: cmd/gendocs deletes files with no test
status: done
type: task
priority: 3
plan: 2026-11-03 - 02 - eval-harness-and-test-gaps-plan
created: 2026-11-03T08:23:42Z
closed: 2026-11-03T11:42:19Z
reason: "Resolved by plan '2026-11-03 - 02 - eval-harness-and-test-gaps-plan' (archived in e1a1506) item 8: cmd/gendocs/main_test.go covers stale-page removal and unrelated files surviving."
---

`run` and `removeStalePages` (`cmd/gendocs/main.go:44`) delete every `capcom*.md` in the output
directory before regenerating, and `cmd/gendocs` has no test file. A regression in the glob or
the order would silently delete or leave stale CLI reference pages, and a bad `-out` removes
unrelated files.

## Acceptance

- A test calls `run(t.TempDir())` with a pre-seeded `capcomold.md` and an unrelated `notes.md` and asserts the stale page is gone, the unrelated file survives and the reference pages are written.
