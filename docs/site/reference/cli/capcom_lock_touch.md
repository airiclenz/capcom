---
title: "capcom lock touch"
description: "Refresh the heartbeat of the lock you hold"
sidebar:
  order: 1
---

## capcom lock touch

Refresh the heartbeat of the lock you hold

### Synopsis

Refresh the heartbeat of the Tree lock you hold, so that it does not go stale; --token, the Lock token acquire printed, is required. Touch it more often than the lock TTL while the work goes on. It prints the lock as it now stands. It fails not_found when nobody holds the lock and locked when another session holds it, both with exit code 3.

```
capcom lock touch [flags]
```

### Examples

```
  capcom lock touch --token <token>
  capcom lock touch --token <token> --kv   # KEY=VALUE lines ending in STATUS=
```

### Options

```
  -h, --help           help for touch
      --kv             print KEY=VALUE lines ending in STATUS=
      --token string   the token acquire returned
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom lock](capcom_lock.md)	 - Acquire, refresh, release and inspect this worktree's Tree lock

