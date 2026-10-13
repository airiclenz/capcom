---
title: "capcom log"
description: "Append an entry to an issue's Log"
sidebar:
  order: 1
---

## capcom log

Append an entry to an issue's Log

### Synopsis

Append an entry to an Issue's Log, dated today and attributed to the author git would record for a commit. The Log is append-only: it carries progress, findings and remarks, while the body stays the statement of the work. The text is one argument, so quote it. It prints the Issue's id, path and new etag.

```
capcom log <id|alias|prefix> <text> [flags]
```

### Examples

```
  capcom log login-timeout-race "reproduced with two tabs"
  capcom log login-timeout-race "root cause: the refresh timer survives logout"
```

### Options

```
  -h, --help   help for log
      --kv     print KEY=VALUE lines ending in STATUS=
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

