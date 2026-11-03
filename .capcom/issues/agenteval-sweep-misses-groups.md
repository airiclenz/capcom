---
title: The eval sweep misses agent-loop shell groups after WaitDelay
status: open
type: bug
priority: 3
created: 2026-11-03T10:19:37Z
---

The harness sweeps what the agent leaves running by sending SIGKILL to the agent's process group (`cmd/agenteval/agent_unix.go:39`). The `agenteval agent` loop starts each shell command in a process group of its own (`isolateProcessGroup`, `cmd/agenteval/agent_unix.go:19`, called at `cmd/agenteval/agent.go:333`). When the agent outlives `agentWaitDelay` (`cmd/agenteval/workspace.go:39`, set at `cmd/agenteval/workspace.go:142`) after the group SIGTERM, Go kills only the agent process, and the group sweep cannot reach the shell commands' own groups, so they keep running after the workspace is removed.

Found in the closeout of plan 2026-11-03 - 01 - remaining-small-bugs-plan (item 3, issue agenteval-timeout-orphans-children).

## Why

A shell command left running outlives its workspace, which is what the group kill set out to prevent.

## Acceptance

- An `agenteval agent` loop that ignores SIGTERM past agentWaitDelay leaves none of its shell commands running once runAgent returns, on Unix
- A test covers it
