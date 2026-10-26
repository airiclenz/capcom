---
title: "capcom related add"
description: "Link an issue to a related one"
sidebar:
  order: 1
---

## capcom related add

Link an issue to a related one

### Synopsis

Link the first Issue to the second by writing the second's id into the first's related list.

Either argument may be an id, an Alias or a unique id prefix.

Linking an Issue to itself is refused, and a link already there, on either Issue, is left as it is.

--if-match writes only when the first Issue's etag still matches.

It prints the id, path and etag of the first Issue.

The first link in a store below format 2 raises the store format to 2, with a warning that older capcom releases can no longer read the store.

```
capcom related add <id|alias|prefix> <related-id|alias|prefix> [flags]
```

### Examples

```
  capcom related add login-timeout-race session-refresh-bug   # shown on both; blocks neither
  capcom related add login-timeout-race session-refresh-bug --if-match <etag>   # etag from show
```

### Options

```
  -h, --help              help for add
      --if-match string   only write when the first issue's etag is this one (see show)
      --kv                print KEY=VALUE lines ending in STATUS=
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom related](capcom_related.md)	 - Manage related links between issues

