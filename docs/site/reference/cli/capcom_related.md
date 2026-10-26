---
title: "capcom related"
description: "Manage related links between issues"
sidebar:
  order: 1
---

## capcom related

Manage related links between issues

### Synopsis

Manage Related links: an undirected tie between two Issues that blocks neither of them.

A link is stored in one Issue's file, under the canonical id of the other, and show lists it on both.

Use a Dependency instead when one Issue has to wait for the other.

```
capcom related [flags]
```

### Examples

```
  capcom related add login-timeout-race session-refresh-bug   # shown on both; blocks neither
  capcom related rm login-timeout-race session-refresh-bug
```

### Options

```
  -h, --help   help for related
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents
* [capcom related add](capcom_related_add.md)	 - Link an issue to a related one
* [capcom related rm](capcom_related_rm.md)	 - Remove the related link between two issues

