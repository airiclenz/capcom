---
title: The update operation of batch.Run is untested
status: done
type: task
priority: 3
plan: 2026-11-03 - 02 - eval-harness-and-test-gaps-plan
created: 2026-11-03T08:23:42Z
closed: 2026-11-03T11:42:19Z
reason: "Resolved by plan '2026-11-03 - 02 - eval-harness-and-test-gaps-plan' (archived in e1a1506) item 4: tests cover an update setting a plan (MissingPlan) and an empty update."
---

In `internal/batch/run.go:171`, the empty-change branch (`op.Update.IsEmpty()` rewrites the issue
through `EditIssue`) and the `MissingPlan` result set when an update sets a plan appear in
neither `run_test.go` nor `cli/apply_test.go`. `apply` is the batch write path.

## Acceptance

- `TestRun_UpdateSettingPlan_ReportsMissingPlanOnTheResult`: a plan with no file yields `Result.MissingPlan`, an existing plan yields an empty string.
- `TestRun_EmptyUpdate_RewritesTheIssueUnchanged` passes.
