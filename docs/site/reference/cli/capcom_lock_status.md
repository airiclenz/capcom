---
title: "capcom lock status"
description: "Show this worktree's lock, or with --all every worktree's"
sidebar:
  order: 1
---

## capcom lock status

Show this worktree's lock, or with --all every worktree's

### Synopsis

Show this worktree's Tree lock: free, or held or stale with its holder, purpose, since time and heartbeat. --all shows every held lock in the repository, one per worktree, or "no locks held". It only reads the lock, and it never prints a Lock token.

```
capcom lock status [flags]
```

### Examples

```
  capcom lock status         # this worktree's lock
  capcom lock status --all   # every worktree's lock
```

### Options

```
      --all    show the locks of every worktree
  -h, --help   help for status
      --kv     print KEY=VALUE lines ending in STATUS=
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom lock](capcom_lock.md)	 - Acquire, refresh, release and inspect this worktree's Tree lock

