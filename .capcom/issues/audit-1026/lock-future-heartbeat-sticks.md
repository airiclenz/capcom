---
title: A lock heartbeat in the future can never go stale or be taken over
status: open
type: bug
priority: 1
created: 2026-10-26T20:06:45Z
---

A Tree lock whose heartbeat is dated in the future never counts as stale, so it can never be
taken over: `lock acquire --take-stale` refuses forever and prints a nonsense age.

Staleness is `now - heartbeat > ttl`. A heartbeat later than now makes that negative, so the
lock looks fresh indefinitely.

Reproduced (ttl 1s, heartbeat patched to 2099):

    capcom lock status  -> _main: held ... (heartbeat 2099-01-01T00:00:00Z): dead
    capcom lock check   -> exit 3, "locked by ... (dead) for 5s, heartbeat -632739h58m52s ago"
    capcom lock acquire --purpose me --take-stale -> exit 3, same message

Real triggers are ordinary: a laptop or VM whose clock runs ahead acquires on a shared worktree;
an NTP correction steps the clock backwards after acquisition; a container has a skewed clock.
The only way out is a human running `capcom lock break`, which is exactly the dead end ADR 0005
and ADR 0016 were written to remove.

The negative duration also leaks into `lock check`, `prime` and the UI status bar.

## Acceptance

- A heartbeat far enough in the future (say more than one TTL ahead) is treated as stale, so
  `lock acquire --take-stale` takes the lock.
- No user-facing message prints a negative age; `AgeText` and `HeldError` agree.
- A test patches a record's heartbeat into the future and asserts the lock is stale and
  `--take-stale` succeeds.
