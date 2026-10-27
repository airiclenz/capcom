---
title: "capcom update"
description: "Change an issue's fields or body"
sidebar:
  order: 1
---

## capcom update

Change an issue's fields or body

### Synopsis

Change an Issue's fields, labels or body in place; at least one flag that changes the Issue is required.

--status moves it between statuses that are not closed: close and reopen cross into and out of the closed categories.

--body-append and --body-append-file (- reads stdin) add to the end of the body, and --body-replace-old with --body-replace-new replaces text that must occur exactly once.

--body-append and the --body-replace-old/--body-replace-new pair repeat: every pair applies in order, each against the body the pairs before it left, then every append in order, and when one pair fails nothing is written.

--standing marks the Issue a standing Parent that only groups its children: it is never Ready, and while it is marked a --status in the doing category is refused, as is close.

An Issue that is closed or in the doing category cannot be marked.

--no-standing clears the mark, and is applied before --status, so one call can clear it and claim the Issue.

The first standing Issue in a store below format 5 raises the store format to 5, with a warning that older capcom releases can no longer read the store.

--if-match refuses the write when the file no longer has the etag show printed.

capcom writes the frontmatter in canonical form, so comments in it are not kept; a write that drops one warns, naming the file.

It prints the Issue's id, path and new etag.

```
capcom update <id|alias|prefix> [flags]
```

### Examples

```
  capcom update login-timeout-race --priority 1 --add-label owner-run
  capcom update login-timeout-race --status active   # claim it: a status in the doing category
  capcom update login-timeout-race --body-append "Seen on Safari only."
  capcom update login-timeout-race --body-replace-old "Safari only" --body-replace-new "Safari and Firefox"
  capcom update login-timeout-race --body-replace-old "P2" --body-replace-new "P1" --body-replace-old "todo" --body-replace-new "done"   # pairs apply in order
  capcom update feature-requests --standing   # only groups its children: never Ready
  capcom update feature-requests --no-standing --status active   # clear the mark and claim it
```

### Options

```
      --add-label strings              labels to add
      --body-append stringArray        text to append to the body, taken literally; repeat to append several in order
      --body-append-file string        file to append to the body; - reads stdin
      --body-replace-new stringArray   text that replaces the --body-replace-old in the same position
      --body-replace-old stringArray   body text to replace; must occur exactly once; repeat with --body-replace-new for several pairs
  -h, --help                           help for update
      --if-match string                only write when the file's etag is this one (see show)
      --kv                             print KEY=VALUE lines ending in STATUS=
      --no-standing                    clear the issue's standing mark
      --plan string                    new plan basename; empty clears it
      --priority int                   new priority, 0 (most urgent) to 4
      --remove-label strings           labels to remove
      --standing                       mark the issue standing: it only groups its children
      --status string                  new status, not in a closed category (use close or reopen)
      --title string                   new title
      --type string                    new type
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

