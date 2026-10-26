---
title: "capcom lock break"
description: "Free this worktree's lock whoever holds it"
sidebar:
  order: 1
---

## capcom lock break

Free this worktree's lock whoever holds it

### Synopsis

Free this worktree's Tree lock whoever holds it, without its Lock token, and delete this worktree's kept token files, so no session resumes the lock from one.

Use it when the holder is gone, for a stale lock nobody will resume, or when the lock record itself is invalid: such a record is discarded, with a warning.

The session that held the lock is not told; its next touch fails not_found.

It prints the worktree and released, or free when nobody held the lock.

```
capcom lock break [flags]
```

### Examples

```
  capcom lock break        # free the lock whoever holds it
  capcom lock break --kv   # KEY=VALUE lines ending in STATUS=
```

### Options

```
  -h, --help   help for break
      --kv     print KEY=VALUE lines ending in STATUS=
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom lock](capcom_lock.md)	 - Acquire, refresh, release and inspect this worktree's Tree lock

