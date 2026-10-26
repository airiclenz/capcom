---
title: "capcom know tree"
description: "Print the outline of Note ids and titles"
sidebar:
  order: 1
---

## capcom know tree

Print the outline of Note ids and titles

### Synopsis

Print the outline of the Knowledge tree, or of the subtree of the Note named: one line per Note with its id and title, indented two spaces per level.

--depth limits the levels printed below the top, or below the named Note, and a "... <n> more" line marks each Note whose children it cut.

Retired subtrees are left out unless --retired is given, and Background subtrees, labelled background, unless --background is given.

A Note named as the start is printed even when retired or background.

```
capcom know tree [id|alias|prefix] [flags]
```

### Examples

```
  capcom know tree                          # the whole Knowledge tree
  capcom know tree --depth 2                # the top level and its direct children
  capcom know tree release-steps            # one branch
  capcom know tree --retired --background   # retired and background Notes included
```

### Options

```
      --background   include Notes labelled background and everything below them
      --depth int    levels below the top (or below the named Note) to print; 0 prints every level
  -h, --help         help for tree
      --retired      include retired Notes and everything below them
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom know](capcom_know.md)	 - Read, search, add, update, move, remove and verify Knowledge Notes

