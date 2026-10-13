---
title: "capcom know search"
description: "Find Notes whose title or body contains every word"
sidebar:
  order: 1
---

## capcom know search

Find Notes whose title or body contains every word

### Synopsis

Find the Notes whose title or body contains every word given, each as a whole word, and list them best match first, one line each with the id and title. --label keeps only the Notes carrying every label given. Retired Notes and everything below them are left out unless --retired is given. No match fails with not_found, exit code 3.

```
capcom know search <word>... [flags]
```

### Examples

```
  capcom know search release                   # Notes that mention release
  capcom know search release tag --label release   # both words, labelled release
  capcom know search index --retired           # retired Notes too
```

### Options

```
  -h, --help            help for search
      --label strings   only notes with every one of these labels; repeat or separate with commas
      --retired         include retired notes and everything below them
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom know](capcom_know.md)	 - Read, search, add, update, move, remove and verify Knowledge Notes

