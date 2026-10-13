---
title: "capcom know rm"
description: "Remove a Note that has no children"
sidebar:
  order: 1
---

## capcom know rm

Remove a Note that has no children

### Synopsis

Remove a Note's file. A Note with children is refused, so no orphan directory is left behind. Every superseded-by entry naming it is dropped, and its Aliases go with the file. capcom deletes the file itself and never runs git. It prints the removed Note's id and path, and the Notes whose superseded-by entries were dropped. To keep a Note that no longer holds on record, retire it with know update --retire instead.

```
capcom know rm <id|alias|prefix> [--if-match <etag>] [flags]
```

### Examples

```
  capcom know rm release-tags                    # refused while it has children
  capcom know rm release-tags --if-match <etag>  # etag from know show
```

### Options

```
  -h, --help              help for rm
      --if-match string   only remove when the file's etag is this one (see know show)
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom know](capcom_know.md)	 - Read, search, add, update, move, remove and verify Knowledge Notes

