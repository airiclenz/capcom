---
title: update --no-standing=false sets the standing mark
status: done
type: bug
priority: 2
created: 2026-10-17T20:40:56Z
closed: 2026-10-17T20:54:40Z
reason: update --no-standing=false now leaves the standing mark alone; covered by TestUpdate_NoStandingFalse_LeavesTheMarkAlone
---

`capcom update <id> --no-standing=false` sets the standing mark instead of leaving it alone.
`standingChange` in internal/cli/update.go:173 reads the flag's presence, not its value: once
`--no-standing` is given at all it returns `!options.isNotStanding` (internal/cli/update.go:179),
so the explicit `=false` value marks the Issue standing.

Deferred out of the standing-parents plan run.

## Acceptance

- `capcom update <id> --no-standing=false` does not set the standing mark.
- A test in internal/cli covers it.
