---
title: "capcom reindex"
description: "Rebuild the index from the issue and Note files"
sidebar:
  order: 1
---

## capcom reindex

Rebuild the index from the issue and Note files

### Synopsis

Rebuild the index from the Issue and Note files. The index is a disposable SQLite file under the git directory, never committed, that the other commands bring up to date themselves; reindex recreates it from scratch. It prints the number of Issues indexed and the index path, and reports each file it left out as a warning.

```
capcom reindex [flags]
```

### Examples

```
  capcom reindex          # rebuild the index from scratch
  capcom reindex --json   # {path, issues} in a JSON envelope
```

### Options

```
  -h, --help   help for reindex
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

