---
title: The agent eval harness runs only by hand, against one local model
status: open
type: task
priority: 3
labels: [maturity, agent-eval]
created: 2026-11-04T18:27:23Z
---

`cmd/agenteval` (documented in `docs/agent-eval.md`, Note `agent-eval`) runs 12 scenarios
against any agent command, but it has been run once, by hand, against a local Qwen model
(`docs/reviews/agent-eval-2026-10-29-qwen3.8-27b-ud-q4-k-m.md`). A change to help texts, the
Managed block or error hints can make agents fail at capcom without any test noticing, and
there is no evidence for how hosted models fare.

Add `.github/workflows/agent-eval.yml`, triggered by `workflow_dispatch` and a weekly
`schedule`, with a matrix of agent configurations, each skipped when its secret is absent so
forks and unconfigured repositories pass:

- `agenteval agent` against an OpenAI-compatible endpoint, with `--base-url`, `--model` and
  `--api-key-env` from repository variables and secrets (one entry per model the maintainer
  configures);
- optionally `claude -p` if the maintainer adds an API key secret.

Each job runs `go run ./cmd/agenteval run --out eval/<model>.md -- <agent argv>` with the
default `--timeout`, uploads the report and the `.transcripts/` directory as a workflow
artifact, and writes the report's totals row to the job summary (`$GITHUB_STEP_SUMMARY`).
The job fails when the pass count drops below a per-model threshold kept in a small file
`cmd/agenteval/thresholds.yaml`, so a regression in agent usability is a red run.

Document the workflow, the secrets and variables it reads and the threshold file in
`docs/agent-eval.md`. Do not commit reports from CI runs automatically; the maintainer copies
noteworthy ones into `docs/reviews/`.

## Why

capcom's users are agents; a recurring, multi-model eval is the evidence that it works for
them, and catches regressions that unit tests cannot see.

## Acceptance

- `.github/workflows/agent-eval.yml` runs on dispatch and weekly, skips cleanly without secrets, and uploads the report and transcripts.
- A pass count below the model's threshold in `cmd/agenteval/thresholds.yaml` fails the job.
- `docs/agent-eval.md` documents the workflow, its secrets and the thresholds.
