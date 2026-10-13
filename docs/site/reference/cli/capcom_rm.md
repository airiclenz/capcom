---
title: "capcom rm"
description: "Remove an issue that has no children"
sidebar:
  order: 1
---

## capcom rm

Remove an issue that has no children

### Synopsis

Remove an Issue's file. An Issue with children is refused, so no orphan directory is left behind, and so is an Issue other Issues depend on, unless --drop-deps removes their deps entries naming it. Every other reference to the Issue is dropped, and its Aliases go with the file. capcom deletes the file itself and never runs git. It prints the removed Issue's id and path, and the Issues whose deps entries were dropped.

```
capcom rm <id|alias|prefix> [--if-match <etag>] [--drop-deps] [flags]
```

### Examples

```
  capcom rm login-copy                       # refused while it has children or dependents
  capcom rm login-timeout-race --drop-deps   # also drops the deps entries that name it
```

### Options

```
      --drop-deps         remove the deps entries of the issues that depend on it instead of refusing
  -h, --help              help for rm
      --if-match string   only remove when the file's etag is this one (see show)
      --kv                print KEY=VALUE lines ending in STATUS=
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

