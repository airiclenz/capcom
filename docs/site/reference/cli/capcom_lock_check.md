---
title: "capcom lock check"
description: "Exit 0 when this worktree's lock is free or yours, 3 when another holds it"
sidebar:
  order: 1
---

## capcom lock check

Exit 0 when this worktree's lock is free or yours, 3 when another holds it

### Synopsis

Exit 0 when this worktree's Tree lock is free or yours, and fail locked, exit code 3, when another session holds it, stale or not.

The lock is yours when --token is its Lock token. Without --token it is yours when capcom runs under the holder process and keeps the lock's token, and any other caller counts a held lock as another's.

It prints the lock's state, free, held or stale, with the holder, purpose, since time and heartbeat of a held lock.

A script or hook runs it before it edits the working tree.

Every capcom write of an Issue or a Note made under the holder process, the agent session's process that acquired or last resumed the lock, renews its heartbeat, so a touch is needed only when the work outlasts the lock TTL without a capcom write.

```
capcom lock check [flags]
```

### Examples

```
  capcom lock check                   # exit 3 unless the lock is free or held from this process
  capcom lock check --token <token>   # exit 0 when the lock is free or yours
```

### Options

```
  -h, --help           help for check
      --kv             print KEY=VALUE lines ending in STATUS=
      --token string   your token; the lock counts as free when you hold it; not needed under the holder process
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom lock](capcom_lock.md)	 - Acquire, refresh, release and inspect this worktree's Tree lock

