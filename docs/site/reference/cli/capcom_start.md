---
title: "capcom start"
description: "Claim a Ready Issue and this worktree's Tree lock"
sidebar:
  order: 1
---

## capcom start

Claim a Ready Issue and this worktree's Tree lock

### Synopsis

Claim a Ready Issue and this worktree's Tree lock in one step, then print the Issue in full and the Lock token. The Issue moves to the first doing status and the lock is held with the Issue id as its purpose. capcom keeps the Lock token for the session in a token file under the git directory, so starting the same Issue again resumes your lock, even after a restart, and finish releases it; --token is optional and, when given, is used in place of the kept token. Starting an Issue already in doing under your own lock changes nothing in its file and resumes the lock. It refuses, writing nothing, an Issue that is Closed, standing, not Ready or already in doing without your lock, and a lock another session holds (locked, exit code 3); each refusal names the command that resolves it. --kv adds LOCK= and TOKEN= to show's record, and --json adds a lock object to show's data.

```
capcom start <id|alias|prefix> [flags]
```

### Examples

```
  capcom start login-timeout-race                   # prints the Issue, then the Lock token
  capcom start login-timeout-race                   # again after a restart: resumes the lock
  capcom start login-timeout-race --kv              # TOKEN= carries the Lock token
```

### Options

```
  -h, --help           help for start
      --kv             print KEY=VALUE lines ending in STATUS=
      --token string   your token from an earlier start or lock acquire, used in place of the kept one
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

