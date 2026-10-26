---
title: "capcom related rm"
description: "Remove the related link between two issues"
sidebar:
  order: 1
---

## capcom related rm

Remove the related link between two issues

### Synopsis

Remove the Related link between two Issues from whichever of them holds it, so the arguments may come in either order.

An argument that names no Issue removes the first Issue's entry with that text, so a link that names no Issue any more can still be removed.

--if-match writes only when the etag of the Issue holding the link still matches.

It prints the id, path and etag of that Issue, and fails not_found, exit code 3, when the two are not linked.

```
capcom related rm <id|alias|prefix> <related-id|alias|prefix> [flags]
```

### Examples

```
  capcom related rm login-timeout-race session-refresh-bug
  capcom related rm session-refresh-bug login-timeout-race   # either order removes the same link
```

### Options

```
  -h, --help              help for rm
      --if-match string   only write when the etag of the issue holding the link is this one (see show)
      --kv                print KEY=VALUE lines ending in STATUS=
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom related](capcom_related.md)	 - Manage related links between issues

