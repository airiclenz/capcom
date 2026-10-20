---
title: capcom init cannot wire capcom prime into apogee
status: open
type: task
priority: 2
labels: [agents]
created: 2026-10-20T09:00:44Z
---

`capcom init --claude-hooks` and `--codex-hooks` wire `capcom prime` into Claude Code and Codex
sessions, each on its own flag, so a user installs only the agent they run. apogee users have no
such flag: their sessions start without the briefing unless they write the reaction by hand.

Each agent's hooks must stay opt-in and independent: `--apogee-hooks` installs apogee's alone and
never touches `.claude/` or `.codex/`, and the existing flags never touch `.apogee/`. Running
`capcom init --apogee-hooks` on an existing store adds the hooks later, as the other flags do.

**Blocked on apogee.** apogee runs session hooks as Reactions in `.apogee/config.yaml`, but a
`run:` Reaction's output never reaches the model, and `advise:` on `session-started` is refused
until apogee admits it (apogee issue `session-start-advise`). apogee also has no compaction
notice to match the `PreCompact` hook. Until apogee lets session-start output reach the model,
there is nothing useful for capcom to write.

Plain `capcom init` prints only the Claude Code snippet as a hint. Once a third agent is
supported, decide whether it should name all three flags instead.

## Acceptance

- `capcom init --apogee-hooks` merges a `session-started` Reaction running `capcom prime` into the
  repository's `.apogee/config.yaml`, creating it when needed and keeping its other entries; a
  second run adds nothing. Its output tells the user to adopt the proposed entry in apogee.
- `--claude-hooks`, `--codex-hooks` and `--apogee-hooks` each write only their own agent's file,
  and combine freely.
- `docs/site/agents/agent-setup.md` has an apogee section, and the CLI reference is regenerated.
- Tests cover a fresh file, a merge into an existing file and a second run.
