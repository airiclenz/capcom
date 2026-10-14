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

`_main` names the primary worktree; a linked worktree is `wt-<name>`. The token is the secret
that proves you are the holder: whoever presents it can renew, release or resume the lock. Keep
it for the rest of the session. capcom prints it only to the session that acquired it; `status`
never shows it. `--holder` defaults to `user@host`.

While the lock is held, a second session's `acquire` fails with exit code 3:

```console
$ capcom lock acquire --purpose "fixing the docs"
capcom: /home/you/your-repo/.git/capcom/locks/_main.json: locked by claude@devbox (implementing the login rework) for 0s, heartbeat 0s ago
```

Running `acquire` again with your own `--token` resumes your lock, even a stale one, and keeps its
original `since` time.

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

- its holder resumes it with `lock acquire --token <token>` or renews it with `lock touch`;
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

`capcom lock release --token <token>` frees the lock you hold. Releasing a lock that is already
free succeeds and changes nothing, so it is safe on every exit path:

```console
$ capcom lock release --token 6071ae76aab99defb4d17a08f832bf3d
_main: released
$ capcom lock release --token 6071ae76aab99defb4d17a08f832bf3d
_main: free
```

`capcom lock break` frees the lock whoever holds it, without a token. Use it when the holder is
gone for good, or when the lock record itself is damaged. The session that held the lock is not
told; its next `touch` fails.

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
this worktree: held; if the token is yours resume with capcom lock acquire --purpose <text> --token <token>, otherwise do not edit it
```

## A session's lock, end to end

```sh
token=$(capcom lock acquire --purpose "implementing login-flow" --kv | sed -n 's/^TOKEN=//p')
# ... work, running this every so often:
capcom lock touch --token "$token"
# ... and when done, or on any exit path:
capcom lock release --token "$token"
```

The `--kv` output prints one `KEY=VALUE` line per field, the token as `TOKEN=`, and ends with
`STATUS=`; see [JSON and KV output](../reference/json-output.md).

## Next

- [The session briefing](../agents/briefing.md): what `capcom prime` tells an agent, locks
  included.
- [The lock record](../reference/file-format.md#the-lock-record): the JSON file behind a lock.
