---
title: A cancelled eval run reports a clean agent exit as a failed start
status: done
type: bug
priority: 3
plan: 2026-11-03 - 02 - eval-harness-and-test-gaps-plan
created: 2026-11-03T10:19:32Z
closed: 2026-11-03T11:42:19Z
reason: "Resolved by plan '2026-11-03 - 02 - eval-harness-and-test-gaps-plan' (archived in e1a1506) item 1: a cancelled run whose agent exits cleanly ends as cancelled, not a start failure, with tests."
---

When the run is cancelled (Ctrl-C or SIGTERM through `signal.NotifyContext` in `cmd/agenteval/main.go:29`) and the agent exits 0 after the SIGTERM its group receives, `exec.Cmd.Run` returns the context error, not an `*exec.ExitError`. `runAgent` (`cmd/agenteval/workspace.go:158-159`) then treats it as a start failure and returns `start the agent <argv0>: context canceled`, though the agent started and ran.

Found in the closeout of plan 2026-11-03 - 01 - remaining-small-bugs-plan (item 3, issue agenteval-timeout-orphans-children).

## Why

The message names a failure that did not happen, so a user who stopped a run reads it as a broken agent command.

## Acceptance

- A run cancelled while its agent exits 0 after the SIGTERM no longer reports `start the agent ...: context canceled`; it ends as any other cancelled run does
- A test covers an agent that exits 0 after a parent cancel
