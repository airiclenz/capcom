---
title: Make capcom easier for agents, small local models included
status: open
type: feature
priority: 1
standing: true
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

Run the eval harness before and after the child Issues land, and keep the run reports under
`docs/reviews/`.

This is a standing Parent: it only groups its children and is never claimed or closed.

## Log

- 2026-10-19 Airic Lenz: Field evaluation handed to a separate scheduled agent (27B, capcom 0.9.0): nine black-box experiments under its capcom-eval Parent, ending in a capcom-eval-report Note. Its findings arrive as capcom-feature-requests Issues; re-rank the children here against them before implementing.
- 2026-10-26 Airic Lenz: Field evaluation done (capcom-eval-report, 2026-10-22, capcom 0.9.1): all nine experiments closed done. Ranked by calls they would have saved: search-excerpt (~8), context-command (~5), kv-all-fields (~3), then tree-status-filter (~3) and suggest-ids-values (~1). Filed search-excerpt, kv-all-fields and tree-status-filter from it, and ref-outside-tree outside this Parent; evidence logged on context-command and suggest-ids-values. Every finding still reproduces on 0.10.0. No experiment touched batch, so batch-apply and mcp-server-design got no field evidence either way.
