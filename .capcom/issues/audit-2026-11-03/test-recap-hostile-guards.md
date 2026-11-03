---
title: Recap's hostile-repository guards are untested
status: open
type: task
priority: 3
plan: 2026-11-03 - 03 - recap-warning-text-plan
created: 2026-11-03T08:23:42Z
---

In `internal/recap/recap.go`, the oversized-blob guard (`gitx.ErrBlobTooLarge` becoming a
warning, `:361`) and the warning when a second file claims an id already seen (`:191`) have no
test; `recap_test.go` has no too-large or already-used case. They are the only defences when
`capcom recap` reads a crafted history.

## Acceptance

- A `TestCompute` case commits a file over 1 MiB and asserts a `recap:` warning, the file left out of the Report and the rest of the recap intact.
- A case commits two files with one id (`a/x.md`, `b/x.md`) and asserts the same.

## Log

- 2026-11-03 Airic Lenz: plan '2026-11-03 - 02 - eval-harness-and-test-gaps-plan' (archived in e1a1506) item 10 landed TestCompute_OversizedFile_IsAWarningAndLeftOut and TestCompute_DuplicateID_KeepsTheFirstFileAndWarnsOfTheSecond. Left open: the oversized test checks only the 'recap:' prefix and huge.md, not the 'larger than 1 MiB' text, so it would pass if the ErrBlobTooLarge branch fell through to the generic read-error warning.
