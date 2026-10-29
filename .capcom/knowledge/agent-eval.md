---
title: "Before changing or running the agent eval harness: cmd/agenteval and docs/agent-eval.md"
labels: [agenteval]
refs: [cmd/agenteval/doc.go, docs/agent-eval.md]
---

`cmd/agenteval` is the agent eval harness: it runs 12 seeded capcom tasks against an agent command
(`claude -p`, or its own `agenteval agent` loop for a model behind an OpenAI-compatible endpoint),
judges each from the store files the agent left behind, counts the agent's capcom calls through a
PATH shim, and writes a markdown report with the transcripts beside it.

`docs/agent-eval.md` covers running it, the scenarios by id, how a task is judged and counted, and
where the report and transcripts land. `cmd/agenteval/doc.go` holds the package overview and the
`# Files` map, which `TestDocFileMap_MatchesDirectory` keeps in step with the directory: list every
new non-test file there.

It is a dev tool, never shipped. Unlike `cmd/release` it imports `internal/`, so it reads store
files with capcom's own parsers. A scenario's check judges only the repo's `.capcom/` files, the
lock state and the call log, never the transcript; a prompt names ids and goals, never a capcom
command. Its tests never reach a network or a model.
