---
title: AGENTS.md block links to the capcom repo and install instructions
status: done
type: feature
priority: 2
created: 2026-10-11T20:16:25Z
closed: 2026-10-11T22:37:09Z
reason: "Resolved by ready-issues-plan, 544d2ee item 3: Managed block points at github.com/airiclenz/capcom#install"
---

The Managed block that `capcom agents` writes into AGENTS.md (`blockIntro` in
internal/agents/block.go) says capcom is the issue register of record and lists its commands, but
never says what capcom is or where it comes from. A person or agent who clones a repo that uses
capcom, and has no `capcom` binary, has no pointer to the source or install instructions.

## How existing setups pick it up

The block is never versioned: `agents.Inspect` compares the bytes between the markers with
`Render(notes)`, which includes `blockIntro`. Changing `blockIntro` therefore makes every existing
block stale under the new binary, and the existing paths carry the new line in:

- `capcom check` reports `agents-block` "the capcom block is out of date; run capcom agents to
  refresh it", and `capcom prime` counts it in its check line at session start.
- `capcom agents --check` exits 1 (CI gate).
- `capcom agents` rewrites the block in place.
- `know add/update/mv/rm` refresh a block that is present and stale, as a side effect.

Nothing refreshes it silently on upgrade; that is deliberate (a read verb never writes AGENTS.md).

## Acceptance

- `blockIntro` gains one short line naming capcom's home, https://github.com/airiclenz/capcom, and
  pointing to its install instructions (README "Install" section).
- An AGENTS.md holding the old block reads as stale: a test in internal/agents/block_test.go
  feeds a pre-change block to `Inspect` and expects `StateStale`, then `Write` and expects the new
  line inside the markers with every byte outside them kept.
- A CLI test shows `capcom check` reporting `agents-block` on the old block and `capcom agents`
  clearing it.
- This repo's own AGENTS.md is refreshed with `capcom agents` in the same commit.
- CHANGELOG entry tells upgraders to run `capcom agents` once (or that `capcom check` will say so).
