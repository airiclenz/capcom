---
title: agents.file set and no AGENTS.md leaves the block reached by no guide
status: open
type: bug
priority: 2
created: 2026-11-01T20:49:20Z
---

With `agents.file` naming a file other than `AGENTS.md` and no `AGENTS.md` in the working tree, `capcom agents` creates no `AGENTS.md`, so a `CLAUDE.md` holding `@AGENTS.md` points at nothing and the Managed block is reached by no guide, yet `capcom check` passes.

`internal/agents/guides.go:233` (`reachBlock`) returns `GuideMissing` for a missing `AGENTS.md` and never creates it, and `internal/check/agents.go:55` reports only `GuideUnreached`, so the missing guide raises no diagnostic.

## Why

agents.file exists so a workspace can keep the block out of AGENTS.md; when AGENTS.md is absent the block silently reaches no agent.

## Acceptance

- With agents.file set and no AGENTS.md, either capcom agents makes the block reachable or capcom check reports that no guide reaches the block file
- A test covers the case
