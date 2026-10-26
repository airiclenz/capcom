---
title: "capcom know show"
description: "Show a Note in full"
sidebar:
  order: 1
---

## capcom know show

Show a Note in full

### Synopsis

Print one Note in full: its id, path, etag, parent, title, Labels, Refs, retirement, superseded-by entries, Aliases and body.

The argument may be an id, an Alias or a unique id prefix, and a retired Note still resolves.

--with-children also prints the Notes directly below it, their retired ones only with --retired.

The etag is what --if-match takes on know update, know mv and know rm.

```
capcom know show <id|alias|prefix> [flags]
```

### Examples

```
  capcom know show release-steps                   # one Note in full
  capcom know show release-steps --with-children   # and the Notes directly below it
  capcom know show release-steps --json            # as a JSON envelope
```

### Options

```
  -h, --help            help for show
      --retired         include retired Notes and everything below them
      --with-children   also show the Notes directly below it
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom know](capcom_know.md)	 - Read, search, add, update, move, remove and verify Knowledge Notes

