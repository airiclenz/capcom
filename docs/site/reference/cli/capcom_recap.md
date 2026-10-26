---
title: "capcom recap"
description: "List what changed in .capcom since a date or a commit"
sidebar:
  order: 1
---

## capcom recap

List what changed in .capcom since a date or a commit

### Synopsis

List what changed in .capcom since a date or a commit: Issues closed (with the reason), filed, whose status changed, and Log entries added; then Notes added, changed and retired.

--since takes a date YYYY-MM-DD (local midnight), a number of days <n>d, or a git ref; it defaults to 7d. A date or day count starts at the last commit before it.

Only committed history counts: the snapshot of .capcom at the start is compared with HEAD, by id. An Issue filed and closed in the window is both filed and closed.

An empty group is left out. --json gives kind recap with data {since, boundary, closed, filed, status_changed, log, notes_added, notes_changed, notes_retired}.

```
capcom recap [--since <date|days|ref>] [flags]
```

### Examples

```
  capcom recap                       # what changed in the last 7 days
  capcom recap --since 2026-10-01    # since local midnight of that date
  capcom recap --since v1.2.0        # since a tag, branch or commit
  capcom recap --since 30d --json    # the versioned JSON envelope
```

### Options

```
  -h, --help           help for recap
      --since string   start of the window: YYYY-MM-DD, <n>d or a git ref (default "7d")
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

