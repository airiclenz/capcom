---
title: "capcom show"
description: "Show issues in full"
sidebar:
  order: 1
---

## capcom show

Show issues in full

### Synopsis

Print one or more Issues in full: their fields, readiness, dependencies and dependents, child progress, Related links, body and Log.

Name an Issue by its id, an Alias or a unique id prefix.

The etag printed is the value --if-match takes on update, mv and rm.

--brief prints the fields only, without the body and its Log. --section prints the fields and the body sections under the headings named, in body order, heading lines included; a heading matches case-insensitively, its # marks ignored, so --section log selects ## Log. A section runs to the next heading of its level or a higher one. An Issue whose body lacks a named heading fails, naming the headings it has; the others still print.

Changes to these Issues on other branches are reported as Elsewhere unless --no-elsewhere is given.

```
capcom show <id|alias|prefix>... [flags]
```

### Examples

```
  capcom show login-timeout-race
  capcom show login-timeout-race refresh-retry   # several at once
  capcom show login-ti                           # a unique id prefix
  capcom show login-timeout-race --brief         # the fields only, no body
  capcom show login-timeout-race --section acceptance --section log
  capcom show login-timeout-race --json          # the versioned JSON envelope
```

### Options

```
      --brief                 print the fields only, without the body and its Log
  -h, --help                  help for show
      --kv                    print KEY=VALUE lines ending in STATUS=
      --no-elsewhere          skip the report of changes to these issues on other branches
      --section stringArray   print the fields and only the body section under this heading; repeat for more
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

