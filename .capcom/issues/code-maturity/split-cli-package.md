---
title: internal/cli holds 26k lines mixing commands with their logic
status: open
type: task
priority: 3
labels: [maturity, refactor]
created: 2026-11-04T18:27:23Z
---

`internal/cli` is about 26k lines of non-test Go and `internal/ui` about 16k (measure with
`cat internal/<pkg>/*.go | wc -l`, excluding `_test.go`). Some command logic already lives in
domain packages (`internal/ops`, `internal/batch`, `internal/briefing`, `internal/recap`), but
much still sits in the cobra command files, which makes it hard to test without `runCLI`,
hard to reuse (for example from a future MCP server, see the issue `mcp-server-design`) and
slow to compile and review.

This issue is the analysis and plan, not the refactor:

1. List every file in `internal/cli` with its line count and classify each function as
   command wiring (flags, output rendering) or domain logic.
2. Propose target packages for the domain logic, following the existing pattern
   (`internal/ops`, `internal/briefing`), and an order of moves that keeps every commit green.
3. Name the tests that would move from `runCLI` to direct unit tests.

Save the result as a plan under `docs/plans/` (see the Note `docs-layout` for naming), file
one child issue per move step under this issue's parent with `capcom apply`, and link them
to the plan with `capcom update <id> --plan "<plan basename>"` as `AGENTS.md` requires.

## Why

A package boundary is cheap to draw now and expensive after a 1.0 promise and outside
contributors arrive.

## Acceptance

- A plan under `docs/plans/` classifies `internal/cli` and gives an ordered list of moves.
- One issue per move step exists, each linked to the plan with `--plan`.
