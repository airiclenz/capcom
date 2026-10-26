---
title: "capcom next"
description: "Show the top Ready Issue in full"
sidebar:
  order: 1
---

## capcom next

Show the top Ready Issue in full

### Synopsis

Print the Issue on the first line of capcom ready in full, as capcom show prints it: the most urgent Ready Issue, the one that unblocks more Issues first among equals.

--label, --no-label, --planned, --unplanned and --show-hidden select as they do for ready.

--json gives show's envelope for that Issue.

With nothing Ready it prints "nothing is ready to work on" and exits 4, a code used for nothing else; --json then gives kind next with data null.

Changes on other branches are reported as Elsewhere unless --no-elsewhere is given.

```
capcom next [flags]
```

### Examples

```
  capcom next                    # the Issue to work on now, in full
  capcom next --label auth       # the top Ready Issue labelled auth
  capcom next --unplanned        # the top Ready Issue without a plan
  capcom next --json             # show's JSON envelope for it
```

### Options

```
  -h, --help               help for next
      --kv                 print KEY=VALUE lines ending in STATUS=
      --label strings      only issues carrying every one of these labels
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

