---
title: AGENTS.md block links to the capcom repo and install instructions
status: open
type: feature
priority: 2
created: 2026-10-11T20:16:25Z
---

The agent-guide block that `capcom init` writes into AGENTS.md (internal/agents/block.go) says
capcom is the issue register of record and lists its commands, but never says what capcom is or
where it comes from. A person or agent who clones a repo that uses capcom, and has no `capcom`
binary, has no pointer to the source or install instructions.

## Acceptance

- The managed AGENTS.md block names capcom's home, https://github.com/airiclenz/capcom, and points
  to its install instructions, in one short line.
- The line survives block regeneration (`capcom init` re-run) and stays inside the managed block.
- internal/agents/block_test.go covers the new line.
