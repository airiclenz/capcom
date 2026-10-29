---
title: cmd/agenteval/scenario.go is far past the file-size guide
status: done
type: task
priority: 4
plan: 2026-10-29 - 01 - agenteval-cleanup-plan
created: 2026-10-29T18:08:41Z
closed: 2026-10-29T20:21:19Z
reason: "Resolved by plan 2026-10-29 - 01 - agenteval-cleanup-plan (46a03d2): scenarios split into scenario_work/shape/find.go, each under 400 lines, listed in doc.go"
---

`cmd/agenteval/scenario.go` holds all 12 scenarios with their seeds and checks and is 1074 lines (863 after item 5 of the agent eval harness plan), more than twice the coding-standards guide of about 400 lines. `cmd/agenteval/doc.go:47` lists it in the `# Files` map as the one file for the scenario type and every scenario.

## Why

Each new scenario grows the one file; a file per scenario group keeps each readable and lets doc.go say where a scenario lives.

## Acceptance

- `cmd/agenteval/scenario.go` is split by scenario group, each file within about 400 lines.
- Each new file is listed in the `# Files` map in `cmd/agenteval/doc.go`.
- `go test ./cmd/agenteval` passes unchanged.
