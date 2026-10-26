---
title: "capcom lock"
description: "Acquire, refresh, release and inspect this worktree's Tree lock"
sidebar:
  order: 1
---

## capcom lock

Acquire, refresh, release and inspect this worktree's Tree lock

### Synopsis

Manage this worktree's Tree lock: the advisory claim one agent session holds on one working tree while it edits.

acquire claims it, prints a Lock token and keeps that token for the session in a token file under the git directory; touch renews its heartbeat with that token, and release frees it.

A lock whose heartbeat is older than the TTL (lock.ttl in config.yaml, two hours by default), or dated more than one TTL ahead by a clock that ran fast, is stale: it stays held until its holder resumes it, another session takes it over with acquire --take-stale, or someone breaks it.

Each git worktree has a lock of its own, and capcom installs no git hooks: the lock binds only the sessions that check it.

```
capcom lock [flags]
```

### Examples

```
  capcom lock acquire --purpose "implementing the release plan"   # prints and keeps the token
  capcom lock touch --token <token>     # renew the heartbeat
  capcom lock status --all              # every worktree's lock
  capcom lock release --token <token>   # free it when the work is done
```

### Options

```
  -h, --help   help for lock
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents
* [capcom lock acquire](capcom_lock_acquire.md)	 - Acquire this worktree's lock, or resume it with your own token
* [capcom lock break](capcom_lock_break.md)	 - Free this worktree's lock whoever holds it
* [capcom lock check](capcom_lock_check.md)	 - Exit 0 when this worktree's lock is free or yours, 3 when another holds it
* [capcom lock release](capcom_lock_release.md)	 - Release the lock you hold; releasing a free lock succeeds
* [capcom lock status](capcom_lock_status.md)	 - Show this worktree's lock, or with --all every worktree's
* [capcom lock touch](capcom_lock_touch.md)	 - Refresh the heartbeat of the lock you hold

