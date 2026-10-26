---
title: "capcom show"
description: "Show issues in full"
sidebar:
  order: 1
---

## capcom show

Show issues in full

### Synopsis

Print one or more Issues in full: their fields, readiness, dependencies and dependents, child progress, Related links, body and Log.

Name an Issue by its id, an Alias or a unique id prefix.

The etag printed is the value --if-match takes on update, mv and rm.

Changes to these Issues on other branches are reported as Elsewhere unless --no-elsewhere is given.

```
capcom show <id|alias|prefix>... [flags]
```

### Examples

```
  capcom show login-timeout-race
  capcom show login-timeout-race refresh-retry   # several at once
  capcom show login-ti                           # a unique id prefix
  capcom show login-timeout-race --json          # the versioned JSON envelope
```

### Options

```
  -h, --help           help for show
      --kv             print KEY=VALUE lines ending in STATUS=
      --no-elsewhere   skip the report of changes to these issues on other branches
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

