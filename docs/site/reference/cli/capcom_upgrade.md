---
title: "capcom upgrade"
description: "Raise the store format on request"
sidebar:
  order: 1
---

## capcom upgrade

Raise the store format on request

### Synopsis

Raise the store format that .capcom/config.yaml declares to --format, by default the newest this capcom reads, creating config.yaml when there is none. It lists what each added format brings.

capcom releases that read only an older format can no longer read the store afterwards: raise it once everyone sharing the repository runs a capcom that reads the new format.

A store already at that format or above is left as it is; capcom never lowers a store format.

```
capcom upgrade [--format <n>] [flags]
```

### Examples

```
  capcom upgrade              # raise the store to the newest format this capcom reads
  capcom upgrade --format 5   # raise it to format 5 and no further
  capcom upgrade --json       # {from, to, changed, adds} in a JSON envelope
```

### Options

```
      --format int   the store format to raise the store to (default 6)
  -h, --help         help for upgrade
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

