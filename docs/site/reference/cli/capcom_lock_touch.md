---
title: "capcom lock touch"
description: "Refresh the heartbeat of the lock you hold"
sidebar:
  order: 1
---

## capcom lock touch

Refresh the heartbeat of the lock you hold

### Synopsis

Refresh the heartbeat of the Tree lock you hold, so that it does not go stale.

Every capcom write of an Issue or a Note made under the holder process, the agent session's process that acquired or last resumed the lock, renews its heartbeat, so a touch is needed only when the work outlasts the lock TTL without a capcom write.

Under the holder process --token may be left out: capcom uses the Lock token it keeps for the lock. Any other caller passes --token, the Lock token acquire printed, and without it fails locked, exit code 3, when the lock is held.

It prints the lock as it now stands.

It fails not_found when nobody holds the lock and locked when another session holds it, both with exit code 3.

```
capcom lock touch [flags]
```

### Examples

```
  capcom lock touch                   # from the holder process
  capcom lock touch --token <token>
  capcom lock touch --token <token> --kv   # KEY=VALUE lines ending in STATUS=
```

### Options

```
  -h, --help           help for touch
      --kv             print KEY=VALUE lines ending in STATUS=
      --token string   the token acquire returned; not needed under the holder process
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom lock](capcom_lock.md)	 - Acquire, refresh, release and inspect this worktree's Tree lock

