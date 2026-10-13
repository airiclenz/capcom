---
title: "Managed block teaches the work loop: claim, log, close, reopen"
status: open
type: feature
priority: 2
created: 2026-10-13T11:59:13Z
---

The Managed block of AGENTS.md (`blockIntro` in `internal/agents/block.go`) teaches an agent how to
file, link and close an issue, but not how to work one. It never names claiming an issue
(`capcom update <id> --status <doing status>`, and `capcom lock acquire --purpose <id>`), keeping
a trail (`capcom log <id>`), or undoing a close (`capcom reopen`). It does not say that
`capcom ready` is where to start, and it does not say that each command's flags (`--type`,
`--priority`, ...) are in `capcom <cmd> --help`.

Why: a mid-size model reviewing a repository's AGENTS.md (2026-10-13) found the operating motions
missing and had to probe `--help` to work an issue. The block is the one text every agent always
reads, so the work loop belongs there. A separate generated skill was considered and rejected for
now: such a skill is loaded only on demand (the weakest models load it least), it would add
per-host install directories to what `init` and `agents` write, and the gap is a few lines.

Constraints:
- Status names are per-repository config, so the block must not hard-code `active`: render the
  configured doing status, or phrase it so it holds for any configuration.
- Point at `--help` for flags; do not list them in the block, where they would drift from the CLI.
- Keep the block short; it is read on every session.

## Acceptance

- The block gives a short work loop: start from `capcom ready`, `capcom show <id>`, claim it
  (doing status, Tree lock), `capcom log <id>` as work lands, `capcom close <id> --reason`,
  `capcom reopen <id>` to undo.
- The block says flags are in `capcom <cmd> --help`.
- The rendered text holds for a repository whose doing status is not named `active`.
- `internal/agents` tests cover the new text; this repository's AGENTS.md is refreshed with
  `capcom agents`.
