---
title: "capcom lock check"
description: "Exit 0 when this worktree's lock is free or yours, 3 when another holds it"
sidebar:
  order: 1
---

## capcom lock check

Exit 0 when this worktree's lock is free or yours, 3 when another holds it

### Synopsis

Exit 0 when this worktree's Tree lock is free or held by your --token, and fail locked, exit code 3, when another session holds it, stale or not. Without --token any held lock counts as another's. It prints the lock's state, free, held or stale, with the holder, purpose, since time and heartbeat of a held lock. A script or hook runs it before it edits the working tree.

```
capcom lock check [flags]
```

### Examples

```
  capcom lock check                   # exit 3 when any session holds the lock
  capcom lock check --token <token>   # exit 0 when the lock is free or yours
```

### Options

```
  -h, --help           help for check
      --kv             print KEY=VALUE lines ending in STATUS=
      --token string   your token; the lock counts as free when you hold it
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom lock](capcom_lock.md)	 - Acquire, refresh, release and inspect this worktree's Tree lock

