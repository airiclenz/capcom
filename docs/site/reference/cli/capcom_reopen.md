---
title: "capcom reopen"
description: "Reopen a closed issue, logging why"
sidebar:
  order: 1
---

## capcom reopen

Reopen a closed issue, logging why

### Synopsis

Reopen a closed Issue and append the reason to its Log; --reason is required. The Issue moves to the first todo status, or to the todo, doing or waiting status --status names. It prints the Issue's id, path and new etag.

```
capcom reopen <id|alias|prefix> --reason <text> [flags]
```

### Examples

```
  capcom reopen login-timeout-race --reason "Came back on Safari"
  capcom reopen login-timeout-race --status active --reason "Picking it up again now"
```

### Options

```
  -h, --help            help for reopen
      --kv              print KEY=VALUE lines ending in STATUS=
      --reason string   why the issue is reopened (required)
      --status string   status that is not closed (default: the first todo status)
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

