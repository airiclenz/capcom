---
title: Lock Touch with a foreign token untested; heartbeat assertion too weak
status: open
type: task
priority: 2
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:13Z
---

Two gaps on the lock-touch critical path. (a) `lock.Touch` with a token that does not match
the stored record returns `*HeldError`, which is the path a displaced session hits after a
stale takeover, and no test covers it. (b) The heartbeat-advance assertion in the lock tests is
`!touched.Heartbeat.Before(acquired.Heartbeat)`, which passes when the heartbeat is unchanged,
so a Touch that never advances it would go unnoticed. Severity: Medium.

## Acceptance

- Acquire, take over with `Acquire(..., isTakeStale=true)`, then `Touch(key, oldToken)`
  returns `*HeldError`, and the new holder's record and heartbeat are unchanged on disk.
- Backdate the stored heartbeat (as the stale-resume test does), Touch, and assert that the
  heartbeat is strictly `After` the backdated value, in memory and on disk.
- `GOMEMLIMIT=2GiB go test ./internal/lock/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, T-05.
