---
title: capcom init cannot wire capcom prime into apogee
status: open
type: task
priority: 2
labels: [agents]
plan: 2026-10-20 - 00 - apogee-hooks-plan
created: 2026-10-20T09:00:44Z
---
`capcom init --claude-hooks` and `--codex-hooks` wire `capcom prime` into Claude Code and Codex
sessions, each on its own flag, so a user installs only the agent they run. apogee users have no
such flag: their sessions start without the briefing unless they write the reaction by hand.

Each agent's hooks must stay opt-in and independent: `--apogee-hooks` installs apogee's alone and
never touches `.claude/` or `.codex/`, and the existing flags never touch `.apogee/`. Running
`capcom init --apogee-hooks` on an existing store adds the hooks later, as the other flags do.

apogee runs session hooks as Reactions in `.apogee/config.yaml`. apogee main (4b290a8b) admits an
`advise:` entry on `session-started`, whose output reaches the model ahead of the first prompt, and
adds a `compaction-finished` notice whose `advise:` re-briefs the model after every fold: the pair
matching Claude's `SessionStart` and `PreCompact`. Both are unreleased (apogee CHANGELOG
[Unreleased]); apogee 0.27.0 ignores such an entry with one start-up line and loads the rest of the
project config. Work waits for the apogee release that ships them.

Plain `capcom init` prints only the Claude Code snippet as a hint. Once a third agent is
supported, decide whether it should name all three flags instead.

## Acceptance

- `capcom init --apogee-hooks` merges a Reaction running `capcom prime` on `session-started` and
  `compaction-finished` into the repository's `.apogee/config.yaml`, creating it when needed and
  keeping its other entries; a second run adds nothing. Its output tells the user to adopt the
  proposed entry in apogee and names the minimum apogee release.
- `--claude-hooks`, `--codex-hooks` and `--apogee-hooks` each write only their own agent's file,
  and combine freely.
- `docs/site/agents/agent-setup.md` has an apogee section, and the CLI reference is regenerated.
- Tests cover a fresh file, a merge into an existing file and a second run.

## Log

- 2026-10-21 Airic Lenz: plan 2026-10-20 - 00 - apogee-hooks-plan landed (9b738b6): --apogee-hooks merges the capcom-prime reaction, flags independent, docs and CLI reference, tests; open only for the real minimum apogee version, tracked in apogee-minimum-version
