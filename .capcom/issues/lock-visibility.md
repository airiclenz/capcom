---
title: Agents never learn the Tree lock exists
status: done
type: feature
priority: 2
plan: 2026-10-14 - 01 - lock-visibility-plan
created: 2026-10-14T15:04:20Z
closed: 2026-10-14T15:20:18Z
reason: Resolved by 2026-10-14 - 01 - lock-visibility-plan (fe974f6)
---

Nothing an agent always reads mentions the Tree lock. The Managed block of AGENTS.md
(`blockIntro` in `internal/agents/block.go`) never names `capcom lock`, and the Briefing
(`capcom prime`) prints only `locks: N` and the held locks, with no word on what to do. An agent
finds the lock only if it reads `capcom lock --help`, so two sessions can change one worktree at
the same time without either of them knowing.

Why: the Tree lock (ADR 0005) is advisory and binds only sessions that check it. It helps only
if agents know to take it before an extensive change and to leave a worktree alone when another
session holds it. The block is read every session, and the Briefing runs at session start and
after compaction, which is when an agent decides what to do and may need to resume its lock.

Constraints:
- Taking the lock is recommended, not required: the text suggests it for a change that spans
  several files or commits and never makes it a rule.
- The lock is not tied to claiming or closing an Issue.
- No enforcing hook (ADR 0005 rejected one).

## Acceptance

- The Managed block recommends `capcom lock acquire --purpose <text>` before a multi-file or
  multi-commit change, says to keep the token for `touch`, `release` and resuming with `--token`,
  and says what to do when another session holds the lock.
- The Briefing's text adds a `this worktree:` line after the locks list, with advice that fits
  the state (free, held or stale). The `--json` data is unchanged.
- `internal/agents` and `internal/cli` tests cover the new text, this repository's AGENTS.md is
  refreshed with `capcom agents`, and the docs pages that quote the block or the Briefing match.
