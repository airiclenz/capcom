---
title: "capcom context"
description: "Show an issue with its deps, plan and the Notes that apply"
sidebar:
  order: 1
---

## capcom context

Show an issue with its deps, plan and the Notes that apply

### Synopsis

Print one Issue with what it takes to work on it: the Issue as capcom show prints it, its deps and dependents one level deep as capcom dep list prints them, its plan's path, and the Notes that apply.

A Note applies when one of its refs names a path the Issue's body mentions, whole or as a directory followed by /. Then come the Notes a search on the Issue's title finds, every word first, else any word. Retired Notes are left out.

The text is capped: at most 8 Notes, the bodies of the first 3 only, 60 lines per body and 15 Issues per dep section. A cut section ends with a line naming the command that prints the rest.

An empty section prints as none. --brief prints the Issue as one line and no Note bodies.

--json gives kind context with data {issue, deps, dependents, plan, notes}, none of it cut.

```
capcom context <id|alias|prefix> [flags]
```

### Examples

```
  capcom context login-timeout-race            # the Issue, its deps, plan and Notes
  capcom context login-timeout-race --brief    # one line per item, no bodies
  capcom context login-timeout-race --json     # the versioned JSON envelope
```

### Options

```
      --brief   print the Issue as one line and leave out the Note bodies
  -h, --help    help for context
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

