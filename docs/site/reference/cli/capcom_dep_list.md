---
title: "capcom dep list"
description: "List an issue's dependencies and dependents"
sidebar:
  order: 1
---

## capcom dep list

List an issue's dependencies and dependents

### Synopsis

List the Issues the given Issue depends on and the Issues that depend on it, under a deps: and a dependents: heading. Without --depth only the direct ones print; --depth n walks n levels, each indented one step further, and --depth 0 walks every level. A dep that names no Issue is marked missing, and an Issue already printed in its section prints again as (see above) without its branch. With --depth given, a branch it cut ends in a "... <n> more" line.

```
capcom dep list <id|alias|prefix> [flags]
```

### Examples

```
  capcom dep list login-timeout-race              # its direct deps and dependents
  capcom dep list --depth 2 login-timeout-race    # two levels deep
  capcom dep list --depth 0 login-timeout-race    # every level
```

### Options

```
      --depth int   levels of deps and dependents to print; 0 prints every level (default 1)
  -h, --help        help for list
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom dep](capcom_dep.md)	 - Manage dependencies between issues

