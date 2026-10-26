---
title: "capcom know search"
description: "Find Notes whose title or body contains every word, else some of them"
sidebar:
  order: 1
---

## capcom know search

Find Notes whose title or body contains every word, else some of them

### Synopsis

Find the Notes whose title or body contains every word given, each as a whole word, and list them best match first, one line each with the id and title.

When no Note holds every word, it lists the Notes holding some of them under the line "No item holds every word; these hold some of them:", and --json sets "match" to "any" instead of "all".

--excerpt prints under each Note the body lines holding a word searched for, indented four spaces, and --json gives them as "excerpt".

--label keeps only the Notes carrying every label given.

Retired Notes and everything below them are left out unless --retired is given.

A Note holding no word fails with not_found, exit code 3.

```
capcom know search <word>... [flags]
```

### Examples

```
  capcom know search release                   # Notes that mention release
  capcom know search release tag --label release   # both words, labelled release
  capcom know search index --retired           # retired Notes too
  capcom know search release --excerpt         # with the body lines that mention release
```

### Options

```
      --excerpt         print under each hit the body lines holding a word searched for: the first 3 such lines with 1 line of context either side, at most 8 lines per hit, each cut at 120 characters
  -h, --help            help for search
      --label strings   only Notes with every one of these labels; repeat or separate with commas
      --retired         include retired Notes and everything below them
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom know](capcom_know.md)	 - Read, search, add, update, move, remove and verify Knowledge Notes

