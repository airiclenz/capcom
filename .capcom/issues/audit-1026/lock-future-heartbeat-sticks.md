---
title: A lock heartbeat in the future is not stale until the clock catches up
status: done
type: bug
priority: 2
plan: 2026-10-26 - 03 - audit-p1-p2-fixes-plan
created: 2026-10-26T20:06:45Z
closed: 2026-10-26T21:45:33Z
reason: Resolved by plan 2026-10-26 - 03 - audit-p1-p2-fixes-plan (archived in 575c3b9)
---

A Tree lock whose heartbeat is dated in the future does not count as stale until the clock passes
the heartbeat plus the TTL, so `lock acquire --take-stale` refuses it until then, and the refusal
prints a negative age.

Staleness is `now - heartbeat > ttl` (`internal/lock/lock.go:412`). A heartbeat later than now makes
that negative, so the lock looks fresh.

Reproduced (ttl 1s, heartbeat patched to 2099):

    capcom lock status  -> _main: held ... (heartbeat 2099-01-01T00:00:00Z): dead
    capcom lock check   -> exit 3, "locked by ... (dead) for 5s, heartbeat -632739h58m52s ago"
    capcom lock acquire --purpose me --take-stale -> exit 3, same message

Real triggers: a machine whose clock runs ahead acquires on a worktree another machine reaches (a
network or synced share); an NTP correction steps the clock back after an acquire; a container runs
with a skewed clock. Ordinary skew heals once the clock passes the heartbeat and the TTL, so the
cost is a takeover delayed by the skew, not a lock held forever; a holder resuming with its own
token is not affected, and `lock break` still works.

The negative age comes from the `locked` error (`heldError`, `internal/lock/lock.go:436-437`),
printed by `lock check`, `lock acquire` and `start`. `prime` and the UI print the age through
`AgeText`, which already clamps it to 0.

## Acceptance

- A heartbeat more than one TTL in the future is treated as stale, so `lock acquire --take-stale`
  takes the lock.
- The `locked` error never prints a negative age.
- A test patches a record's heartbeat into the future and asserts the lock is stale and
  `--take-stale` succeeds.
