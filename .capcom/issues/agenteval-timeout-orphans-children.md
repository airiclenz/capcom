---
title: An agent timeout in the eval harness leaves its children running
status: done
type: bug
priority: 3
plan: 2026-11-03 - 02 - eval-harness-and-test-gaps-plan
created: 2026-11-03T08:23:42Z
closed: 2026-11-03T11:42:19Z
reason: Resolved by plan '2026-11-03 - 02 - eval-harness-and-test-gaps-plan' (archived in e1a1506) items 1-3, which closed the remaining gaps (cancel wording, ledger sweep of agent-loop groups, signal-wiring tests).
---

The harness kills a timed-out agent with plain `exec.CommandContext`
(`cmd/agenteval/workspace.go:134`): SIGKILL to the one process, no process group. The package
already has `isolateProcessGroup` (`cmd/agenteval/agent_unix.go:15`) for this, but applies it
only to the `agent` verb's shell commands (`cmd/agenteval/agent.go:296`). `WaitDelay` only stops
the harness waiting.

Wanted: in `runAgent`, start the agent in its own process group and kill the whole group on
cancel, reusing or generalising `isolateProcessGroup`. Windows stays as it is.

Source: docs/reviews/code-audit-2026-11-03.md.

## Why

A timed-out `claude -p` or `agenteval agent` run leaves shell children alive for up to the
two-minute shell timeout, still with the shim first on PATH, inside a workspace being deleted.

## Acceptance

- On Unix, a test agent that spawns a long-running child and hits the task timeout leaves no surviving child.
- Windows behaviour is unchanged.

## Log

- 2026-11-03 Airic Lenz: Plan '2026-11-03 - 01 - remaining-small-bugs-plan' item 3 (archived in 3ecf9f7) landed the process-group kill and sweep after every agent run on Unix, with a timeout test. Left open for deferred gaps: an agent outliving WaitDelay leaves its agent-loop shell groups (own pgid) outside the sweep; an agent exiting 0 after a parent-cancel SIGTERM reports 'start the agent X: context canceled'; no test covers main.go's NotifyContext wiring.
