---
title: "capcom fmt"
description: "Rewrite every issue and Note file in canonical form"
sidebar:
  order: 1
---

## capcom fmt

Rewrite every issue and Note file in canonical form

### Synopsis

Rewrite every Issue and Note file whose bytes differ from its canonical form, listing each one, then the count.

A file that does not parse or validate is left alone with a warning; capcom check is the validator.

--check writes nothing, lists the files that would change and fails, exit code 1, when any would.

```
capcom fmt [--check] [flags]
```

### Examples

```
  capcom fmt           # rewrite Issue and Note files in canonical form
  capcom fmt --check   # list what would change; exit 1 when anything would
```

### Options

```
      --check   write nothing; list the files that would change and fail when any would
  -h, --help    help for fmt
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

