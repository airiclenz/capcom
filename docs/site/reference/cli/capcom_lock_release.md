---
title: "capcom lock release"
description: "Release the lock you hold; releasing a free lock does nothing"
sidebar:
  order: 1
---

## capcom lock release

Release the lock you hold; releasing a free lock does nothing

### Synopsis

Free the Tree lock you hold; --token, the Lock token acquire printed, is required. Releasing a lock that is already free succeeds and changes nothing, so it is safe on every exit path. It prints the worktree and released, or free when there was nothing to release. A lock another session holds fails locked, exit code 3; break frees a lock whose holder is gone.

```
capcom lock release [flags]
```

### Examples

```
  capcom lock release --token <token>
  capcom lock release --token <token> --kv   # KEY=VALUE lines ending in STATUS=
```

### Options

```
  -h, --help           help for release
      --kv             print KEY=VALUE lines ending in STATUS=
      --token string   the token acquire returned
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom lock](capcom_lock.md)	 - Acquire, refresh, release and inspect this worktree's Tree lock

