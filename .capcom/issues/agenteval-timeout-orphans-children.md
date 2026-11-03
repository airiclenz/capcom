---
title: An agent timeout in the eval harness leaves its children running
status: open
type: bug
priority: 3
created: 2026-11-03T08:23:42Z
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
