---
title: "capcom ready"
description: "List the issues ready to work on, most urgent first"
sidebar:
  order: 1
---

## capcom ready

List the issues ready to work on, most urgent first

### Synopsis

List the Ready Issues, most urgent first: Issues in the todo category whose dependencies are all closed, with no open children and no waiting or dropped ancestor. Each line gives the id, status, priority and title, and how many Issues closing it would make Ready. --all also lists the todo Issues that are not Ready, with what holds each one back. Issues carrying a label listed under ready.hide in the config stay out unless --show-hidden is given. Changes to Issues on other branches are reported as Elsewhere unless --no-elsewhere is given.

```
capcom ready [flags]
```

### Examples

```
  capcom ready                               # what to work on next
  capcom ready --limit 1                     # only the most urgent
  capcom ready --all                         # also the todo issues that are not ready, and why
  capcom ready --label owner-run --unplanned # filtered by label and plan
```

### Options

```
      --all                also list todo issues that are not ready, with what blocks them
  -h, --help               help for ready
      --label strings      only issues carrying every one of these labels
      --limit int          list at most this many issues (0: all)
      --no-elsewhere       skip the report of issue changes on other branches
      --no-label strings   only issues carrying none of these labels
      --planned            only issues with a plan
      --show-hidden        include issues carrying a ready.hide label
      --unplanned          only issues without a plan
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

