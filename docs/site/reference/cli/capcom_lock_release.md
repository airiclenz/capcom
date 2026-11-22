---
title: "capcom lock release"
description: "Release the lock you hold; releasing a free lock succeeds"
sidebar:
  order: 1
---

## capcom lock release

Release the lock you hold; releasing a free lock succeeds

### Synopsis

Free the Tree lock you hold.

Under the holder process --token may be left out: capcom uses the Lock token it keeps for the lock. Any other caller passes --token, the Lock token acquire printed, and without it fails locked, exit code 3, when the lock is held.

It also deletes the kept token files holding that token.

Releasing a lock that is already free succeeds and only deletes those token files, so it is safe on every exit path.

It prints the worktree and released, or free when there was nothing to release.

A lock another session holds fails locked, exit code 3, and deletes nothing; break frees a lock whose holder is gone.

Every capcom write of an Issue or a Note made under the holder process, the agent session's process that acquired or last resumed the lock, renews its heartbeat, so a touch is needed only when the work outlasts the lock TTL without a capcom write.

```
capcom lock release [flags]
```

### Examples

```
  capcom lock release                   # from the holder process
  capcom lock release --token <token>
  capcom lock release --token <token> --kv   # KEY=VALUE lines ending in STATUS=
```

### Options

```
  -h, --help           help for release
      --kv             print KEY=VALUE lines ending in STATUS=
      --token string   the token acquire returned; not needed under the holder process
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom lock](capcom_lock.md)	 - Acquire, refresh, release and inspect this worktree's Tree lock

