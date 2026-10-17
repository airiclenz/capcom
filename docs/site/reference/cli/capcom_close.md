---
title: "capcom close"
description: "Close issues, recording why"
sidebar:
  order: 1
---

## capcom close

Close issues, recording why

### Synopsis

Close one or more Issues, recording the reason on each; --reason is required. Each Issue moves to the first done status, or to the done or dropped status --status names. It prints one line per Issue with what happened to it, then a "now ready:" line naming the Issues the close made Ready. An argument that names no Issue, or names a standing Issue, is reported and fails the command; the other Issues are still closed. A standing Issue only groups its children: clear its mark with update --no-standing before closing it.

```
capcom close <id|alias|prefix>... --reason <text> [flags]
```

### Examples

```
  capcom close login-timeout-race --reason "Fixed in 3f2a1c9"   # names the issues it made ready
  capcom close refresh-retry login-copy --reason "Shipped with the login rework"
  capcom close cache-warmup --status dropped --reason "The CDN warms it now"
```

### Options

```
  -h, --help            help for close
      --kv              print KEY=VALUE lines ending in STATUS=
      --reason string   why the issues are closed (required)
      --status string   status in the done or dropped category (default: the first done status)
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

