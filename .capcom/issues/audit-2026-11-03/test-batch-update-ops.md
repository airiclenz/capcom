---
title: The update operation of batch.Run is untested
status: open
type: task
priority: 3
created: 2026-11-03T08:23:42Z
---

In `internal/batch/run.go:171`, the empty-change branch (`op.Update.IsEmpty()` rewrites the issue
through `EditIssue`) and the `MissingPlan` result set when an update sets a plan appear in
neither `run_test.go` nor `cli/apply_test.go`. `apply` is the batch write path.

## Acceptance

- `TestRun_UpdateSettingPlan_ReportsMissingPlanOnTheResult`: a plan with no file yields `Result.MissingPlan`, an existing plan yields an empty string.
- `TestRun_EmptyUpdate_RewritesTheIssueUnchanged` passes.
