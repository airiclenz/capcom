---
title: Decide whether capcom serves its operations over MCP
status: open
type: task
priority: 3
deps: [agent-eval-harness]
created: 2026-10-19T14:54:45Z
---

Agents drive capcom through shell commands and flags. A Model Context Protocol server
(`capcom mcp`, over stdio) would expose the same operations as typed tools with JSON schemas:
statuses and types as enums, required fields enforced, ids validated. For small models that
call tools with constrained decoding, this removes flag-syntax errors entirely; for frontier
models it removes shell quoting problems and output parsing.

CLI and UI already share one write path in `internal/ops` (ADR 0012), so a third front end is
plausible, but it adds a dependency, a surface to keep in step, and a second way to do
everything.

This Issue is a design decision. It waits for `agent-eval-harness`, whose results, together with
the field agent's `capcom-eval-report` findings, should show
whether flag and quoting errors are a large enough share of agent failures to justify it.

## Acceptance

- An ADR under `docs/adr/` with status "proposed" covers: the tool list, how schemas are
  derived from the cobra commands or `ops` so they cannot drift, the dependency cost, how the
  Tree lock and `--if-match` work over MCP, and the eval numbers that support or reject it.
- Its Note tree is added under `decisions`.
- Implementation Issues are filed only if the owner accepts the ADR.
