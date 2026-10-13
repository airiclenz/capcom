---
title: "capcom update"
description: "Change an issue's fields or body"
sidebar:
  order: 1
---

## capcom update

Change an issue's fields or body

### Synopsis

Change an Issue's fields, labels or body in place; at least one flag that changes the Issue is required. --status moves it between statuses that are not closed: close and reopen cross into and out of the closed categories. --body-append and --body-append-file (- reads stdin) add to the end of the body, and --body-replace-old with --body-replace-new replaces text that must occur exactly once. --if-match refuses the write when the file no longer has the etag show printed. It prints the Issue's id, path and new etag.

```
capcom update <id|alias|prefix> [flags]
```

### Examples

```
  capcom update login-timeout-race --priority 1 --add-label owner-run
  capcom update login-timeout-race --status active   # claim it: a status in the doing category
  capcom update login-timeout-race --body-append "Seen on Safari only."
  capcom update login-timeout-race --body-replace-old "Safari only" --body-replace-new "Safari and Firefox"
```

### Options

```
      --add-label strings         labels to add
      --body-append string        text to append to the body, taken literally
      --body-append-file string   file to append to the body; - reads stdin
      --body-replace-new string   text that replaces --body-replace-old
      --body-replace-old string   body text to replace; must occur exactly once
  -h, --help                      help for update
      --if-match string           only write when the file's etag is this one (see show)
      --kv                        print KEY=VALUE lines ending in STATUS=
      --plan string               new plan basename; empty clears it
      --priority int              new priority, 0 (most urgent) to 4
      --remove-label strings      labels to remove
      --status string             new status, not in a closed category (use close or reopen)
      --title string              new title
      --type string               new type
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

