---
title: "capcom lock acquire"
description: "Acquire this worktree's lock, or resume it with your own token"
sidebar:
  order: 1
---

## capcom lock acquire

Acquire this worktree's lock, or resume it with your own token

### Synopsis

Claim this worktree's Tree lock for --purpose, which is required.

A free lock gets a new Lock token, printed on a token line; touch and release need it.

capcom keeps the Lock token for the session in a token file under the git directory: acquiring again with the same --purpose and no --token resumes your lock, even a stale one, refreshes its heartbeat and prints the token again.

A --token from an earlier acquire is used in place of the kept one.

A lock another session holds fails locked, exit code 3, unless it is stale and --take-stale is given, which takes it over with a new token.

--holder names the holder, user@host by default.

```
capcom lock acquire [flags]
```

### Examples

```
  capcom lock acquire --purpose "implementing the release plan"   # prints and keeps the token
  capcom lock acquire --purpose "implementing the release plan"   # again: resume after a restart
  capcom lock acquire --purpose "finishing the release" --take-stale   # take over a stale lock
```

### Options

```
  -h, --help             help for acquire
      --holder string    who holds the lock (default user@host)
      --kv               print KEY=VALUE lines ending in STATUS=
      --purpose string   what the lock is held for
      --take-stale       take the lock over when it is stale
      --token string     your token from an earlier acquire, used in place of the kept one
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom lock](capcom_lock.md)	 - Acquire, refresh, release and inspect this worktree's Tree lock

