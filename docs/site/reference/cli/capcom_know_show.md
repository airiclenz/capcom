---
title: "capcom know show"
description: "Show Notes in full"
sidebar:
  order: 1
---

## capcom know show

Show Notes in full

### Synopsis

Print each named Note in full: its id, path, etag, parent, title, Labels, Refs, retirement, superseded-by entries, Aliases and body.

Each argument may be an id, an Alias or a unique id prefix, and a retired Note still resolves. An argument that names none of those is tried as a title, case-insensitive: the whole title, then the part before its first colon, then the argument spelled as an id ("Release steps!" spells release-steps). When several Notes match, it fails naming them all.

The Notes found are printed even when another argument names none, and the command then exits with the first failure's code.

--with-children also prints the Notes directly below each, their retired ones only with --retired.

The etag is what --if-match takes on know update, know mv and know rm.

```
capcom know show <id|alias|prefix|title>... [flags]
```

### Examples

```
  capcom know show release-steps                   # one Note in full
  capcom know show release-steps store-layout      # several at once
  capcom know show "Release steps"                 # by its title
  capcom know show release-steps --with-children   # and the Notes directly below it
  capcom know show release-steps --json            # as a JSON envelope
```

### Options

```
  -h, --help            help for show
      --retired         include retired Notes and everything below them
      --with-children   also show the Notes directly below each
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom know](capcom_know.md)	 - Read, search, add, update, move, remove and verify Knowledge Notes

