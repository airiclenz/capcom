---
title: The lock record is world-readable and holds the token in cleartext
status: open
type: bug
priority: 3
plan: 2026-10-27 - 00 - audit-p3-fixes-plan
created: 2026-10-26T20:06:45Z
---

The lock record is world-readable and contains the token in cleartext, while the token file
beside it is carefully owner-only.

    -rw-r--r--  _main.json          -> "token": "f950957d0acefa08ad1ec0a312bb28da"
    -rw-------  _main.<hash>.token

The record is written through `nodefile.WriteFileAtomic`, which uses 0644
(`internal/lock/lock.go:423`, `internal/nodefile/atomic.go:14`); `writeTokenFile` uses 0600 and its
doc says "owner-only". The 0600 file protects nothing while the same secret sits next to it at 0644.

ADR 0016 accepts that the token guards against accident rather than a same-user process, and
another user cannot use the token anyway, since the lock directory is not writable to them. So
this is defence in depth, but the two files should not disagree.

Making the record 0600 has a cost: in a group-shared clone (`core.sharedRepository`) another user's
`lock status` and `prime` would report the record unreadable instead of naming the holder. Keeping
only a hash of the token in the record avoids that, since a presented token is checked by hashing it.

Out of scope: commands that echo the caller's own token (`acquire`, a resume, `touch`,
`check --token`) do so by design (ADR 0016).

## Acceptance

- No file another user can read holds the token in cleartext.
- `lock status` by another user in a group-shared clone still names the holder.
- A test asserts the record does not expose the token (its mode, or that it holds no cleartext
  token, depending on the fix chosen).
