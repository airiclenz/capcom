---
title: "capcom start"
description: "Claim a Ready Issue and this worktree's Tree lock"
sidebar:
  order: 1
---

## capcom start

Claim a Ready Issue and this worktree's Tree lock

### Synopsis

Claim a Ready Issue and this worktree's Tree lock in one step, then print the Issue in full and the Lock token to keep. The Issue moves to the first doing status and the lock is held with the Issue id as its purpose; --token, the Lock token an earlier start or lock acquire printed, resumes your lock instead of acquiring a new one. Starting an Issue already in doing under your own --token changes nothing in its file and resumes the lock. It refuses, writing nothing, an Issue that is Closed, standing, not Ready or already in doing without your lock, and a lock another session holds (locked, exit code 3); each refusal names the command that resolves it. --kv adds LOCK= and TOKEN= to show's record, and --json adds a lock object to show's data.

```
capcom start <id|alias|prefix> [flags]
```

### Examples

```
  capcom start login-timeout-race                   # prints the Issue, then the token to keep
  capcom start login-timeout-race --token <token>   # resume after a restart
  capcom start login-timeout-race --kv              # TOKEN= carries the Lock token
```

### Options

```
  -h, --help           help for start
      --kv             print KEY=VALUE lines ending in STATUS=
      --token string   your token from an earlier start or lock acquire, to resume the lock
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

