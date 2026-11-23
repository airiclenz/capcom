---
title: "capcom know seed"
description: "Add the base set of Notes capcom ships; only --update changes a Note that exists"
sidebar:
  order: 1
---

## capcom know seed

Add the base set of Notes capcom ships; only --update changes a Note that exists

### Synopsis

Add the base set of Notes that capcom ships to the Knowledge tree: one root Note and the Notes below it, each a rule a repository may adopt.

It adds every Note of the set the store lacks, the root first, and without --update never changes a Note that exists. An added Note carries two Labels: seed:<root id>, and seed-text:<version>:<hash>, which holds the capcom version and a hash of the text it was added with.

It prints one line per Note of the set, its id and its outcome: added; skipped (same) for a Note whose title and body are the set's; skipped (retired) for a retired Note; skipped (alias of <id>) when the id is an Alias of another Note. A retired root Note skips the whole set as retired.

A Note with another text is skipped (outdated) when its text is still the text it was added with, so the set has changed since, and skipped (differs) otherwise: it was edited, or it is the repository's own.

A root Note that exists without the seed:<root id> Label is the repository's own: the command fails and writes nothing. A write that fails stops the run; the Notes added before it stay, and running the command again adds the rest.

--update also replaces the title and body of every outdated Note with the set's, and prints updated for it. It puts the new seed-text Label in place of the old one and keeps every other Label, Ref and Alias. It never changes a Note that differs or a retired one.

--dry-run prints the same lines and writes nothing, with --update too.

--show <id> prints one Note of the set as this capcom ships it, its id, its title and its body, and writes nothing: the text to compare with the Note the store holds. An id the set does not hold fails with not_found and names the ids it holds. It excludes --dry-run and --update.

```
capcom know seed [[--update] [--dry-run] | --show <id>] [flags]
```

### Examples

```
  capcom know seed             # add the Notes of the base set the store lacks
  capcom know seed --dry-run   # print what it would add, and write nothing
  capcom know seed --update    # also replace the Notes capcom ships a newer text for
  capcom know seed --show <id> # print the text capcom ships for one Note of the set
```

### Options

```
      --dry-run       print each Note's outcome and write nothing
  -h, --help          help for seed
      --show string   print the embedded title and body of this Note of the base set, and write nothing
      --update        also replace the title and body of every Note whose text is still what it was added with
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom know](capcom_know.md)	 - Read, search, add, update, move, remove and verify Knowledge Notes

