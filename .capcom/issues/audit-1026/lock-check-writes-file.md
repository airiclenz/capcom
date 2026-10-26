---
title: lock check is read-only by promise but creates the guard file
status: open
type: bug
priority: 2
created: 2026-10-26T20:06:45Z
---

`capcom lock check` is documented as read-only and as the thing hooks run before editing, but it
creates a file.

Every lock operation runs through `withGuard`, which creates the lock directory and opens
`.git/capcom/locks/<key>.lck` read-write-create (`internal/lock/lock.go:363-371`), including
`check`, `Read` and `Overview`.

Reproduced on a fresh repo:

    capcom lock check -> _main: free, exit 0
    find .git/capcom  -> .git/capcom/locks/_main.lck    (new empty file)

And with `locks/` made read-only:

    capcom lock check -> exit 1, "..._main.lck: open ...: permission denied"
                         hint: capcom lock check --help

So on a read-only checkout, an NFS mount or a sandbox, the guard command fails, and fails with
exit 1, a code the contract does not define for `check` (0 or 3).

`Read` and `Overview` back more than `lock check`: `prime` (`internal/briefing/briefing.go:166`)
and the UI's status bar and ask-before-writing path read through them too, so each of those creates
a `.lck` as well. A record is replaced by a rename, so a reader sees a whole old or new record and
needs no guard.

## Acceptance

- `lock check`, `lock status`, `prime` and the UI's lock reads create nothing under
  `.git/capcom/`, on a fresh repo and on any lock state.
- `lock check` on a read-only git dir exits 0 or 3 and never fails on the guard file.
- A test runs `lock check` and `prime` in a fresh repo and asserts `.git/capcom/locks/` is unchanged.
