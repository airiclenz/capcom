---
title: Test the eval harness's Ctrl-C and SIGTERM wiring
status: done
type: task
priority: 3
plan: 2026-11-03 - 02 - eval-harness-and-test-gaps-plan
created: 2026-11-03T10:19:33Z
closed: 2026-11-03T11:42:19Z
reason: "Resolved by plan '2026-11-03 - 02 - eval-harness-and-test-gaps-plan' (archived in e1a1506) item 3: tests send SIGINT/SIGTERM to the built agenteval during a run, and a second signal ends it at once."
---

`main` in `cmd/agenteval/main.go:29-31` cancels the command tree's context on SIGINT or SIGTERM through `signal.NotifyContext` and restores the default action once cancelled (`context.AfterFunc(ctx, stop)`), so a second Ctrl-C kills at once. No test covers this wiring: the tests drive `runRun` and `runTask` with a context they cancel themselves.

Found in the closeout of plan 2026-11-03 - 01 - remaining-small-bugs-plan (item 3, issue agenteval-timeout-orphans-children).

## Why

A change that drops the signal wiring or the default-action restore would pass the suite.

## Acceptance

- A test sends the built agenteval a SIGINT or SIGTERM during a run and sees it stop the agent and exit without a report
- A test shows a second signal ends the process at once
