---
title: "capcom know seed"
description: "Add the base set of Notes capcom ships, never changing a Note that exists"
sidebar:
  order: 1
---

## capcom know seed

Add the base set of Notes capcom ships, never changing a Note that exists

### Synopsis

Add the base set of Notes that capcom ships to the Knowledge tree: one root Note and the Notes below it, each a rule a repository may adopt.

It adds every Note of the set the store lacks, the root first, and never changes a Note that exists. An added Note carries two Labels: seed:<root id>, and seed-text:<version>:<hash>, which holds the capcom version and a hash of the text it was added with.

It prints one line per Note of the set, its id and its outcome: added; skipped (same) for a Note whose title and body are the set's; skipped (differs) for one with another text; skipped (retired) for a retired Note; skipped (alias of <id>) when the id is an Alias of another Note. A retired root Note skips the whole set as retired.

A root Note that exists without the seed:<root id> Label is the repository's own: the command fails and writes nothing. A write that fails stops the run; the Notes added before it stay, and running the command again adds the rest.

--dry-run prints the same lines and writes nothing.

```
capcom know seed [--dry-run] [flags]
```

### Examples

```
  capcom know seed             # add the Notes of the base set the store lacks
  capcom know seed --dry-run   # print what it would add, and write nothing
```

### Options

```
      --dry-run   print each Note's outcome and write nothing
  -h, --help      help for seed
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom know](capcom_know.md)	 - Read, search, add, update, move, remove and verify Knowledge Notes

