---
title: No repeatable measure of how well an agent drives capcom
status: open
type: feature
priority: 1
created: 2026-10-19T14:54:44Z
---

There is no way to measure how well an agent drives capcom. Ideas for agent ergonomics are
guesses until they are measured, and the owner runs a local 27B model on a schedule against
this repository, so a repeatable measurement is within reach.

Wanted: a dev tool under `cmd/` (like `cmd/release`, never shipped) or a script under
`scripts/` that runs a fixed set of capcom tasks against an agent and scores the transcripts.

- Tasks: 10 to 20 short scenarios in a fresh temp git repository seeded by the harness, for
  example "pick the next ready issue, claim it, log progress, close it with a reason",
  "file an issue with a body and an Acceptance section that depends on X", "find the Note
  that explains the Tree lock", "resume after losing the Lock token".
- Agent: any command that takes a prompt and a working directory, so a local model behind an
  OpenAI-compatible endpoint and a frontier model can both be run.
- Score per task: success (checked from the store files afterwards, not from the transcript),
  number of capcom invocations, number that exited non-zero, and repeated identical failures.
- Report: a markdown table per run, written where the caller says.

Hermetic tests of the harness itself use a fake agent command (see the `cli-test-harness` and
`adr-0007` Notes); no test may need a model.

## Acceptance

- One command runs every task against a given agent command and writes the report.
- Success is judged from the resulting `.capcom/` files, never from the agent's own claims.
- The harness's tests pass with a scripted fake agent and no network.
- A first report for the scheduled local model is committed under `docs/reviews/`.
