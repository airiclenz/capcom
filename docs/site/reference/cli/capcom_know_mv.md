---
title: "capcom know mv"
description: "Rename a Note or move it below another"
sidebar:
  order: 1
---

## capcom know mv

Rename a Note or move it below another

### Synopsis

Rename a Note, move it below another Note, or both; --parent . moves it to the top of the Knowledge tree.

A rename keeps the old id as an Alias, so old citations still resolve, and rewrites every superseded-by entry that named the Note.

Its children move with it.

capcom renames the files itself and never runs git.

It prints the Note's new id, path and etag.

```
capcom know mv <id|alias|prefix> [<new-id>] [--parent <id|alias|prefix|.>] [--if-match <etag>] [flags]
```

### Examples

```
  capcom know mv release-steps cutting-a-release        # the old id stays an alias
  capcom know mv release-tags --parent .                # to the top of the tree
  capcom know mv release-tags --parent cutting-a-release --if-match <etag>   # etag from know show
```

### Options

```
  -h, --help              help for mv
      --if-match string   only move when the file's etag is this one (see know show)
      --parent string     move the Note below this one; . moves it to the top of the tree
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom know](capcom_know.md)	 - Read, search, add, update, move, remove and verify Knowledge Notes

