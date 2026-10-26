---
title: "capcom list"
description: "List issues; without flags, every issue that is not closed"
sidebar:
  order: 1
---

## capcom list

List issues; without flags, every issue that is not closed

### Synopsis

List Issues one per line with id, status, priority and title, each child indented below its Parent.

Without flags it lists every Issue that is not closed; --all includes closed ones, and the filters (--status, --category, --label, --type, --priority, --parent, --plan and more) narrow the list.

--tree prints the Issue outline instead: each Parent with its closed/total children, --depth levels deep.

Changes to Issues on other branches are reported as Elsewhere unless --no-elsewhere is given.

```
capcom list [flags]
```

### Examples

```
  capcom list                     # every issue that is not closed
  capcom list --label owner-run   # open issues carrying a label
  capcom list --all --type bug    # every bug, closed ones included
  capcom list --tree --depth 2    # the outline, two levels deep
```

### Options

```
      --all                include closed issues
      --category strings   only issues in one of these categories (includes closed ones)
      --depth int          with --tree, levels below the top (or below --parent) to print; 0 prints every level
  -h, --help               help for list
      --label strings      only issues carrying every one of these labels
      --limit int          list at most this many issues (0: all)
      --no-elsewhere       skip the report of issue changes on other branches
      --no-label strings   only issues carrying none of these labels
      --parent string      only the direct children of this issue (id, alias or prefix); with --tree, this issue first, then its subtree
      --plan string        only issues whose plan contains this text
      --planned            only issues with a plan
      --priority ints      only issues with one of these priorities
      --status strings     only issues with one of these statuses (includes closed ones)
      --tree               print the issue outline: each parent with its closed/total children, closed issues left out unless --all
      --type strings       only issues of one of these types
      --unplanned          only issues without a plan
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

