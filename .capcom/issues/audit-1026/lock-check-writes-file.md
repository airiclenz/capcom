---
title: lock check is read-only by promise but creates the guard file
status: open
type: bug
priority: 2
created: 2026-10-26T20:06:45Z
---

`capcom lock check` is documented as read-only and as the thing hooks run before editing, but it
creates a file.

The guard file `.git/capcom/locks/_main.lck` is opened read-write-create on every lock operation,
including `check`, because all of them take the same file lock.

Reproduced on a fresh repo:

    capcom lock check -> _main: free, exit 0
    find .git/capcom  -> .git/capcom/locks/_main.lck    (new empty file)

And with `locks/` made read-only:

    capcom lock check -> exit 1, "..._main.lck: open ...: permission denied"
                         hint: capcom lock check --help

So on a read-only checkout, an NFS mount or a sandbox, the guard command fails, and fails with
exit 1, a code the contract does not define for `check` (0 or 3).

The "asks before writing" path in `capcom ui` uses the same read, so it also creates a `.lck` on
every attempted write.

## Acceptance

- `capcom lock check` and `capcom lock status` create nothing, on a fresh repo and on any state.
- `lock check` on a read-only git dir exits 0 or 3 and never fails on the guard file.
- A test runs `lock check` in a fresh repo and asserts `.git/capcom/locks/` is unchanged.
