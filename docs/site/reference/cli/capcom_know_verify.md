---
title: "capcom know verify"
description: "List the Note refs that no longer resolve"
sidebar:
  order: 1
---

## capcom know verify

List the Note refs that no longer resolve

### Synopsis

List every Ref of every Note that no longer resolves: a missing path, a path#symbol whose file lacks the symbol text, or a path outside the working tree.

It also lists a Ref a clone would lack although it resolves here: one inside the git directory, or one git ignores.

Each line cites the Ref's line in its Note as path:line.

Retired subtrees are exempt.

It takes no arguments, and any finding fails the command, exit code 1, after the list.

```
capcom know verify [flags]
```

### Examples

```
  capcom know verify          # Note refs that no longer resolve
  capcom know verify --json   # the findings as a JSON envelope
```

### Options

```
  -h, --help   help for verify
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom know](capcom_know.md)	 - Read, search, add, update, move, remove and verify Knowledge Notes

