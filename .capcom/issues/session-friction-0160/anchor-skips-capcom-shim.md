---
title: A per-call capcom shim becomes the lock's holder process, so the holder's later writes still warn
status: open
type: bug
priority: 3
labels: [agent-ergonomics]
created: 2026-11-22T19:26:45Z
---

proc.Anchor (`internal/proc/proc.go:84`) returns the first ancestor whose name is not in `wrapperNames` (`internal/proc/proc.go:41`), and `app.holderProcess` (`internal/cli/root.go:563`) records that anchor as the lock's holder process. A capcom-named wrapper is not skipped: when capcom runs behind a per-call shim parent (agenteval's capcom shim, `cmd/agenteval/shim.go`; Scoop's `capcom.exe` shim), the shim becomes the anchor. It exits after the call, so the holder's later writes run under a different shim process and still draw the lock warning.

## Why

The holder-process rule exists so the holder's own writes stay silent; under a shim it silently does not apply.

## Acceptance

- proc.Anchor skips a capcom-named wrapper ancestor, so a lock acquired through a per-call shim records the next ancestor as its holder process
- A test covers a chain whose first ancestor is a capcom shim
