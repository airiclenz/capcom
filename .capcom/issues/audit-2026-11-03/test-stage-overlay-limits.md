---
title: The staged overlay's size cap and directory refusal are untested
status: done
type: task
priority: 3
plan: 2026-11-03 - 02 - eval-harness-and-test-gaps-plan
created: 2026-11-03T08:23:42Z
closed: 2026-11-03T11:42:19Z
reason: "Resolved by plan '2026-11-03 - 02 - eval-harness-and-test-gaps-plan' (archived in e1a1506) item 12: tests cover the staged 1 MiB cap (ErrTooLarge) and directory refusal (ErrNotRegular)."
---

The 1 MiB cap on a staged file (`ReadRegularFile` returning `nodefile.ErrTooLarge`) and the
staged-directory `ErrNotRegular` refusal (`internal/stage/overlay.go:150`) have no test. The
stage must mirror disk behaviour, or a batch that grows a file past the cap is accepted on disk
but refused when read back.

## Acceptance

- A test stages a ReplaceFile over 1 MiB and asserts `ReadRegularFile` fails with `nodefile.ErrTooLarge`.
- A test stages a CreateFile'd directory and asserts `ReadRegularFile` fails with `nodefile.ErrNotRegular`.
