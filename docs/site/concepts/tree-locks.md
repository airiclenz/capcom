---
title: "Tree locks"
description: "The Tree lock: an advisory claim one agent session holds on a working tree, its Lock token, when it goes Stale, and acquire, touch, release, break, check and status."
sidebar: { order: 5 }
---

Two agent sessions editing the same checkout at the same time will trip over each other: one
runs the tests while the other is halfway through a change, or both edit the same file. A
**Tree lock** lets a session say "I am working in this tree" so that every other session, and
every person, can see who, why and since when.

## Tree lock

A **Tree lock** is the advisory claim one agent session holds on one working tree (one checkout
or one git worktree) while it changes it. It names the holder, the purpose and the time it was
taken. Each worktree of a clone has its own Tree lock, so sessions in separate worktrees never
block each other, and the locks of all worktrees can be seen together.

The lock is **advisory**: capcom installs no git hooks and stops no edits. It binds only the
sessions that check it, which is why the [Managed block](../agents/managed-block.md) and the
session [briefing](../agents/briefing.md) show it to every agent.

The lock record is a small JSON file under the git directory, shared by every worktree of the
clone and never committed. Its fields are listed under
[The lock record](../reference/file-format.md#the-lock-record).

## Acquire, and the Lock token

`capcom lock acquire` claims the lock for a purpose and prints a **Lock token**:

```console
$ capcom lock acquire --purpose "implementing the login rework" --holder claude@devbox
_main: acquired by claude@devbox since 2026-10-13T18:57:00Z (heartbeat 2026-10-13T18:57:00Z): implementing the login rework
token 6071ae76aab99defb4d17a08f832bf3d
```

`_main` names the primary worktree; a linked worktree is `wt-<name>`. The token marks the lock as
yours: `touch` and `release` take it, and capcom keeps it for you (see
[The kept token](#the-kept-token)). `status` never shows it. `--holder` defaults to `user@host`.

While the lock is held, a second session's `acquire` fails with exit code 3:

```console
$ capcom lock acquire --purpose "fixing the docs"
capcom: /home/you/your-repo/.git/capcom/locks/_main.json: locked by claude@devbox (implementing the login rework) for 0s, heartbeat 0s ago
hint: capcom lock status
```

Running the same `acquire --purpose` again resumes your lock, even a stale one, prints the token
again and keeps its original `since` time:

```console
$ capcom lock acquire --purpose "implementing the login rework" --holder claude@devbox
_main: resumed by claude@devbox since 2026-10-13T18:57:00Z (heartbeat 2026-10-13T19:20:41Z): implementing the login rework
token 6071ae76aab99defb4d17a08f832bf3d
```

## The kept token

capcom keeps each session's Lock token in a token file under the git directory, one per token,
named for the purpose it is kept for, beside the lock record (ADR 0016 and its amendment;
[The lock record](../reference/file-format.md#the-lock-record) has the layout). `lock acquire`
and `capcom start` write it whenever they take or resume the lock. Acquiring again with the same
`--purpose` reads it, and resumes the lock when its token still holds it; this is how a session
that lost its context to a restart or a compaction carries on, without having kept anything
itself. A `--token` given on the command line is used in place of the kept one; it is the way back
into a lock taken by an older capcom that kept no token file. Resuming with `--token` under a new
purpose moves the token file to that purpose, so a session naming the old one then fails `locked`.

The token tells different work apart, not who may touch the worktree: any session that names the
same purpose resumes the lock, and a session naming another purpose fails `locked`. It is a public
signal between agents, not a secret; the lock stays advisory. `lock release` and `capcom finish`
delete the kept token they release, and `lock break` deletes all of the worktree's kept tokens.

## Touch: keep it fresh

A held lock carries a heartbeat. `capcom lock touch --token <token>` renews it; touch more often
than the lock's TTL while the work goes on:

```console
$ capcom lock touch --token 6071ae76aab99defb4d17a08f832bf3d
_main: held by claude@devbox since 2026-10-13T18:57:00Z (heartbeat 2026-10-13T18:57:00Z): implementing the login rework
token 6071ae76aab99defb4d17a08f832bf3d
```

## Stale

A lock is **Stale** when its holder has not renewed it within the TTL: `lock.ttl` in
`.capcom/config.yaml`, two hours by default (see [Config](../reference/file-format.md#config)).
A stale lock is still held. It stays held until one of three things happens:

- its holder resumes it by running the same `lock acquire --purpose` again or renews it with
  `lock touch`;
- another session takes it over on purpose with `lock acquire --take-stale`, which gives it a
  new token;
- someone breaks it.

```console
$ capcom lock status
_main: stale by claude@devbox since 2026-10-13T18:56:56Z (heartbeat 2026-10-13T09:00:00Z): implementing the login rework
$ capcom lock acquire --purpose "fixing the docs" --take-stale --holder codex@devbox
_main: acquired by codex@devbox since 2026-10-13T18:56:56Z (heartbeat 2026-10-13T18:56:56Z): fixing the docs
token 9baba9b72be5cfc3acc014df4156af49
```

A stale lock is a question, not an answer: the holder may simply have been thinking for a long
time. Taking one over is a deliberate act.

## Release and break

`capcom lock release --token <token>` frees the lock you hold and deletes the kept token files
holding that token. Releasing a lock that is already free succeeds, deleting only those kept token
files, so it is safe on every exit path:

```console
$ capcom lock release --token 6071ae76aab99defb4d17a08f832bf3d
_main: released
$ capcom lock release --token 6071ae76aab99defb4d17a08f832bf3d
_main: free
```

`capcom lock break` frees the lock whoever holds it, without a token, and deletes the worktree's
kept token files. Use it when the holder is gone for good, or when the lock record itself is
damaged. The session that held the lock is not told; its next `touch` fails.

## Check and status

`capcom lock status` shows this worktree's lock (`free`, `held` or `stale`, with the holder,
purpose and times). A record that does not parse shows as `invalid`, and one that cannot be read
as `unreadable`, each with a warning naming the file; the command still exits 0, and `lock break`
frees an invalid record. `capcom lock status --all` shows every held lock in the repository, one
per worktree, with one warning per invalid or unreadable record:

```console
$ capcom lock status --all
_main: held by claude@devbox since 2026-10-13T18:56:56Z (heartbeat 2026-10-13T18:56:56Z): implementing the login rework
```

`capcom lock check` is for scripts and hooks that want to refuse to edit a tree someone else
holds. It exits 0 when the lock is free or held by your `--token`, and 3 when another session holds
it, stale or not:

```sh
capcom lock check --token "$CAPCOM_TOKEN" || exit 1
```

`capcom prime` lists the held locks at the start of every session, then says what to do about the
lock of the worktree it runs in (`free`, `held`, `stale`, `invalid` when its record does not parse,
or `unreadable` when it cannot be read):

```text
locks: 1
  _main: held by claude@devbox for 0m, heartbeat 0m ago: implementing the login rework
this worktree: held; if it is yours, run the same capcom lock acquire --purpose <text> or capcom start <id> again to resume it (or pass --token <token>), otherwise do not edit it
```

## Start and finish: the lock with an Issue

Work on an Issue takes the lock with the Issue. `capcom start <id>` claims a Ready Issue and
acquires the lock with the Issue id as its purpose, printing the Issue and the token:

```console
$ capcom start db-schema
...
_main: acquired by claude@devbox since 2026-10-22T09:45:59Z (heartbeat 2026-10-22T09:45:59Z): db-schema
token 670fa2ee31064cc626fff5a087e22d95
```

capcom keeps the token for the Issue id, so running `capcom start <id>` again after a restart
resumes your lock and prints the token again, as a repeated `acquire` does. A lock another session
holds refuses the start with exit code 3, and the Issue is not claimed.

`capcom finish <id> --reason <text>` closes the Issue and releases the lock through the token kept
for it, deleting that kept token, then names the Ready Issue to work on next:

```console
$ capcom finish db-schema --reason "Added in the sessions migration."
db-schema  closed
now ready: session-refresh
_main: released
next: session-refresh  Refresh the session token
```

`--token` on `start` and `finish` is optional and, when given, is used in place of the kept token.
When neither holds the lock, `finish` still closes the Issue and leaves the lock alone, naming
`capcom lock status`; `--keep-lock` keeps it for more work under it.
`acquire` and `release` remain the way to hold the lock for a change outside an Issue.

## A session's lock, end to end

```sh
capcom start login-flow
# ... work, renewing the heartbeat every so often (start again prints the token):
token=$(capcom start login-flow --kv | sed -n 's/^TOKEN=//p')
capcom lock touch --token "$token"
# ... and when done:
capcom finish login-flow --reason "Reworked in 3f2a1c9"
```

Outside an Issue, `capcom lock acquire --purpose <text> --kv` resumes the same way and prints the
same `TOKEN=` line, and `capcom lock release --token "$token"` frees the lock on any exit path.

The `--kv` output prints one `KEY=VALUE` line per field, the token as `TOKEN=`, and ends with
`STATUS=`; see [JSON and KV output](../reference/json-output.md).

## Next

- [The session briefing](../agents/briefing.md): what `capcom prime` tells an agent, locks
  included.
- [The lock record](../reference/file-format.md#the-lock-record): the JSON file behind a lock.
