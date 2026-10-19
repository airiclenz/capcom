---
title: Make capcom easier for agents, small local models included
status: open
type: feature
priority: 1
created: 2026-10-19T14:54:44Z
---

Groups the work that makes capcom easier for coding agents to drive, with a particular eye on
small local models (the 25-30B class) as well as frontier models.

Small models fail in recognisable ways: they lose their place in sequences of three to five
commands, retry near-miss spellings in a loop, skip multi-step retrieval rules written in
AGENTS.md, struggle with heredocs and stdin, and cannot keep a secret such as a Lock token
across turns or a context compaction. Each child Issue removes one of those failure points
inside the tool, so the tool's answer is the easy path.

`agent-eval-harness` measures these failures; its results should decide the order and whether
the larger items (`mcp-server-design`, `batch-apply`) are worth building.

## Acceptance

- Every child Issue is closed.
- The eval harness has been run before and after the child Issues landed, and the run reports
  are kept under `docs/reviews/`.
