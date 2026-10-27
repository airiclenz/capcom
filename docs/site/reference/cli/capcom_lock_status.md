---
title: "capcom lock status"
description: "Show this worktree's lock, or with --all every worktree's"
sidebar:
  order: 1
---

## capcom lock status

Show this worktree's lock, or with --all every worktree's

### Synopsis

Show this worktree's Tree lock: free, or held or stale with its holder, purpose, since time and heartbeat.

A lock record that does not parse shows as invalid, and one that cannot be read as unreadable, each with a warning naming the file; the command still exits 0.

--all shows every held lock in the repository, one per worktree, or "no locks held", with one warning per invalid or unreadable record.

A record left by a linked worktree that was removed, or removed and created again, holds no lock: it shows as free (orphaned record of a removed worktree), with "orphaned": true in --json, and --all lists it after the held locks. The next acquire in that worktree takes it over.

It only reads the lock, and it never prints a Lock token.

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

