---
title: "capcom merge-driver"
description: "Merge three versions of an issue file, its Log by entry (a git merge driver)"
sidebar:
  order: 1
---

## capcom merge-driver

Merge three versions of an issue file, its Log by entry (a git merge driver)

### Synopsis

Merge three versions of an issue file as a git merge driver: `capcom merge-driver %O %A %B %P %S %X %Y`. The Log merges by entry; the rest of the file merges with git merge-file. The result replaces <ours>; the exit code is 1 when conflict markers remain.

```
capcom merge-driver <base> <ours> <theirs> <path> [<base-label> <ours-label> <theirs-label>] [flags]
```

### Examples

```
  capcom merge-driver %O %A %B %P %S %X %Y   # as git runs it, once init --merge-driver is configured
  capcom merge-driver base.md ours.md theirs.md .capcom/issues/login-flow.md   # by hand: the result replaces ours.md
```

### Options

```
  -h, --help   help for merge-driver
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

