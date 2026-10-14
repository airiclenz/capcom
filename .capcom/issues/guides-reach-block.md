---
title: Every agent guide reaches the Managed block
status: open
type: feature
priority: 2
created: 2026-10-14T20:20:07Z
---

The Managed block lives only in AGENTS.md. Most agent hosts read AGENTS.md, but Claude Code reads
CLAUDE.md and Gemini CLI reads GEMINI.md. In a repository whose CLAUDE.md or GEMINI.md neither
links to AGENTS.md nor imports it, those agents never see the block, and nothing reports it.

Why: the block is the one text every agent is meant to read at session start. A repository that
starts from an existing CLAUDE.md gets an AGENTS.md from `capcom agents` that Claude never loads.
Writing the block into every guide was rejected: a CLAUDE.md that imports or links AGENTS.md
would carry it twice, and the copies drift.

Wanted (ratified 2026-10-14):
- AGENTS.md stays the only home of the block.
- An agent guide reaches the block when it resolves (through links) to the same file as AGENTS.md,
  or holds a line `@AGENTS.md` (or `@./AGENTS.md`).
- The guides are `agents.guides` in config.yaml, default `[CLAUDE.md, GEMINI.md]`, paths relative
  to the working tree root; `guides: []` turns the rule off. The key raises the store format to 4
  (ADR 0009); `agents:` in a store declaring an older format is an error naming the fix.
- `capcom agents` appends `@AGENTS.md` to each existing guide that does not reach the block. It
  never creates a guide, and never writes one that links outside the working tree.
- `capcom agents --check` and `capcom check` report a guide that exists and does not reach the
  block as a problem, but only when AGENTS.md carries the block.
- `capcom init` leaves the guides alone; its output hints at `capcom agents`.
- The command keeps its name `capcom agents`; no per-host command.

## Acceptance

- With AGENTS.md carrying the block and a CLAUDE.md without the import, `capcom check` and
  `capcom agents --check` report CLAUDE.md and exit non-zero; `capcom agents` appends
  `@AGENTS.md` to it and both then pass.
- A CLAUDE.md symlinked to AGENTS.md, or already holding the import, is left byte-identical.
- A missing CLAUDE.md or GEMINI.md is never created.
- An AGENTS.md without the block reports no guide problem.
- `agents: {guides: []}` in a format-4 store turns the rule off; in a format-3 store it fails.
- The Managed block glossary Note, docs/site/agents/managed-block.md and
  docs/site/reference/file-format.md describe the rule.
