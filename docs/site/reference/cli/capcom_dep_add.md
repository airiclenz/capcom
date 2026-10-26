---
title: "capcom dep add"
description: "Make an issue depend on another"
sidebar:
  order: 1
---

## capcom dep add

Make an issue depend on another

### Synopsis

Make the first Issue depend on the second: the first is not Ready until the second is closed.

Either argument may be an id, an Alias or a unique id prefix.

A dependency of an Issue on itself is refused, and so is one that would close a cycle, with the cycle named; an edge that is already there is left as it is.

It prints the id, path and etag of the dependent Issue.

```
capcom dep add <id|alias|prefix> <dep-id|alias|prefix> [flags]
```

### Examples

```
  capcom dep add login-timeout-race token-store-rework   # login-timeout-race waits for token-store-rework
  capcom dep add refresh-retry login-timeout-race --kv    # KEY=VALUE lines ending in STATUS=
```

### Options

```
  -h, --help   help for add
      --kv     print KEY=VALUE lines ending in STATUS=
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom dep](capcom_dep.md)	 - Manage dependencies between issues

