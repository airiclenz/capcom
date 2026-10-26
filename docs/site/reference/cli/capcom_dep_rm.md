---
title: "capcom dep rm"
description: "Remove a dependency of an issue"
sidebar:
  order: 1
---

## capcom dep rm

Remove a dependency of an issue

### Synopsis

Remove the dependency of the first Issue on the second, whether the file names the second by its id or by an Alias.

A dep that names no Issue, such as one a hand edit or a merge left behind, is removed by its text as the file writes it.

It prints the id, path and etag of the dependent Issue, and fails not_found, exit code 3, when the first Issue does not depend on the second.

```
capcom dep rm <id|alias|prefix> <dep-id|alias|prefix> [flags]
```

### Examples

```
  capcom dep rm login-timeout-race token-store-rework   # login-timeout-race no longer waits for it
  capcom dep rm refresh-retry old-cache-issue           # drop a dep that names no Issue any more
```

### Options

```
  -h, --help   help for rm
      --kv     print KEY=VALUE lines ending in STATUS=
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom dep](capcom_dep.md)	 - Manage dependencies between issues

