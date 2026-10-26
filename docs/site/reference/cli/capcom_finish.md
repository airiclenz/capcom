---
title: "capcom finish"
description: "Close an Issue and free this worktree's Tree lock"
sidebar:
  order: 1
---

## capcom finish

Close an Issue and free this worktree's Tree lock

### Synopsis

Close an Issue and free this worktree's Tree lock in one step, then name the Ready Issue to work on next.

--reason is required; the Issue moves to the first done status, and the output gives close's line and its "now ready:" line, a lock line, and a last line naming the top Ready Issue as capcom next chooses it, or "nothing is ready to work on".

The lock is released through the Lock token capcom kept for the Issue when start took the lock, and that kept token is deleted; --token is optional and, when given, is used in place of the kept token.

When neither holds the lock, the lock is left alone and the lock line names capcom lock status.

--keep-lock leaves the lock held and needs no token.

An Issue already Closed is reported already_closed and the lock is still released.

An Issue that does not exist or is standing fails the command and the lock is not touched.

--kv gives one record with ID=, LOCK= and NEXT=, and --json gives the close, the lock and the next Issue.

```
capcom finish <id|alias|prefix> --reason <text> [flags]
```

### Examples

```
  capcom finish login-timeout-race --reason "Fixed in 3f2a1c9"
  capcom finish login-timeout-race --reason "Fixed in 3f2a1c9" --keep-lock   # more work under this lock
  capcom finish login-timeout-race --reason "Fixed in 3f2a1c9" --token <token> --kv
```

### Options

```
  -h, --help            help for finish
      --keep-lock       leave the Tree lock held
      --kv              print KEY=VALUE lines ending in STATUS=
      --reason string   why the Issue is closed (required)
      --token string    your token from start or lock acquire, used in place of the kept one
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

