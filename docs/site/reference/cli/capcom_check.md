---
title: "capcom check"
description: "Validate every config, issue and Note file of the store"
sidebar:
  order: 1
---

## capcom check

Validate every config, issue and Note file of the store

### Synopsis

Validate the whole store from its files, never the index: the config, every Issue and Note file, and the trees they form.

Each problem is listed as path:line: message, and any problem fails the command, exit code 1, after the list.

A write made through a capcom command is already validated: one that leaves a problem check reports warns with check's own words for it. Run check after a hand edit of a store file, after a merge, and before committing hand-edited store files.

The --json data is the list of problems as {path, line, code, message} objects.

```
capcom check [flags]
```

### Examples

```
  capcom check          # validate the whole store
  capcom check --json   # the problems as a JSON envelope
```

### Options

```
  -h, --help   help for check
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

