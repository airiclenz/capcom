---
title: "capcom mv"
description: "Rename an issue or move it below another"
sidebar:
  order: 1
---

## capcom mv

Rename an issue or move it below another

### Synopsis

Rename an Issue, move it below another Issue, or both; --parent . moves it to the top of the tree.

A rename keeps the old id as an Alias, so old citations still resolve, and rewrites every deps entry that named the Issue.

Its children move with it.

capcom renames the files itself and never runs git.

It prints the Issue's new id, path and etag.

```
capcom mv <id|alias|prefix> [<new-id>] [--parent <id|alias|prefix|.>] [--if-match <etag>] [flags]
```

### Examples

```
  capcom mv refresh-retry token-refresh-retry          # the old id stays an alias
  capcom mv login-timeout-race --parent login-flow     # file it below login-flow
  capcom mv login-timeout-race --parent .              # back to the top of the tree
```

### Options

```
  -h, --help              help for mv
      --if-match string   only move when the file's etag is this one (see show)
      --kv                print KEY=VALUE lines ending in STATUS=
      --parent string     move the issue below this one; . moves it to the top of the tree
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

