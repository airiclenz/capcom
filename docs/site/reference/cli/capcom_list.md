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

--tree prints the Issue outline instead: each Parent with its closed/total children, --depth levels deep. With --tree the filters but --limit and --parent (which starts the outline) keep the matching Issues and their ancestors, an ancestor that does not match ending in [context]; --status and --category decide by themselves whether closed Issues match, the other filters follow --all.

Given ids, it prints one line per id, in the order given: the Issue whose exact id or Alias it is, closed ones included, or the id marked not found, so one call answers each id's status and whether an id is still free; the exit code is 0 either way.

A prefix of an id is not found here; an id only an invalid file claims is marked unusable with that file's path, and one that two files claim duplicate. Ids take no flag but --no-elsewhere.

Changes to Issues on other branches are reported as Elsewhere unless --no-elsewhere is given.

```
capcom list [<id>...] [flags]
```

### Examples

```
  capcom list                     # every issue that is not closed
  capcom list login-flow new-idea # each id's status, or not found
  capcom list --label owner-run   # open issues carrying a label
  capcom list --all --type bug    # every bug, closed ones included
  capcom list --tree --depth 2    # the outline, two levels deep
  capcom list --tree --status open  # open issues, ancestors marked [context]
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
      --planned            only issues whose plan field is set
      --priority ints      only issues with one of these priorities
      --status strings     only issues with one of these statuses (includes closed ones)
      --tree               print the issue outline: each parent with its closed/total children, closed issues left out unless --all; with row filters, the matches and their ancestors
      --type strings       only issues of one of these types
      --unplanned          only issues whose plan field is not set
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

