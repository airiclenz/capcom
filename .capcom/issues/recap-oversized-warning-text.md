---
title: Recap oversized-file test does not check the size warning text
status: done
type: task
priority: 3
labels: [test]
plan: 2026-11-03 - 03 - recap-warning-text-plan
created: 2026-11-03T11:40:37Z
closed: 2026-11-03T12:28:58Z
reason: "plan '2026-11-03 - 03 - recap-warning-text-plan' (archived in 7cc04f4) item 1: the oversized-file test now asserts the tree.ErrTooLarge warning text"
---

TestCompute_OversizedFile_IsAWarningAndLeftOut (internal/recap/recap_test.go:228) asserts only that the warning starts with `recap: ` and names `huge.md` (internal/recap/recap_test.go:243). It never checks the `larger than 1 MiB` text that the gitx.ErrBlobTooLarge branch of load writes (internal/recap/recap.go:382), so the test would still pass if that branch fell through to the generic read-error warning at internal/recap/recap.go:386.

## Why

The test exists to guard the oversized-blob branch; without the text check it does not tell that branch from the generic read failure.

## Acceptance

- The test asserts the warning carries the oversized-file text from tree.ErrTooLarge (`larger than 1 MiB`)
- Removing the ErrBlobTooLarge branch in load makes the test fail
