---
title: lock touch and release demand the token capcom already keeps, and touch is the most-run command
status: open
type: feature
priority: 2
labels: [agent-ergonomics]
plan: 2026-11-22 - 01 - session-friction-followups-plan
created: 2026-11-22T17:47:25Z
---

`capcom lock acquire` keeps the Lock token in a token file (ADR 0016), and `start` and
`finish` use it, but `capcom lock touch` and `capcom lock release` still require `--token`
(`internal/cli/lock.go`). Agents carry the 32-hex token through their context and paste it
into every call.

Seen: `capcom lock touch --token <32 hex>` was the most-run capcom command, 436 calls for
about 67 acquires (6 to 7 per lock), though the TTL is two hours and most plan runs end
sooner. `lock release --token <32 hex>` ran 72 times. The token was always pasted
literally.

Why: each touch is a tool call and about 20 tokens of hex that buys nothing, and a token
lost across compaction blocks the release.

Wanted:
- `--token` becomes optional on `touch` and `release`: without it capcom uses the kept
  token file whose token matches the current lock record, and fails as today when none
  matches.
- Consider renewing the heartbeat on any capcom write made by the holder (as recognised
  under `lock-warns-own-writes`), so a session that works through capcom never needs to
  touch at all.
- `lock touch --help` and the managed block say when a touch is needed at all (only when
  the work may outlast the TTL).

## Acceptance

- After `capcom lock acquire --purpose X`, `capcom lock touch` and `capcom lock release`
  succeed without `--token` from the same checkout.
- A checkout with no matching kept token still fails with the present message and exit
  code.
- The managed block no longer teaches pasting the token for touch and release.

## Log

- 2026-11-22 Airic Lenz: Plan 2026-11-22 - 00 - lock-holder-and-agent-guide-plan (bf30b14) landed bare lock touch, release and check for the holder process; open: proc.Anchor does not skip a capcom-named wrapper, so under a per-call shim (agenteval's capcom shim, Scoop's capcom.exe) the holder is not recognised and bare touch/release fail.
