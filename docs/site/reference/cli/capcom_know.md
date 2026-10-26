---
title: "capcom know"
description: "Read, search, add, update, move, remove and verify Knowledge Notes"
sidebar:
  order: 1
---

## capcom know

Read, search, add, update, move, remove and verify Knowledge Notes

### Synopsis

Read and change the Knowledge tree: Notes are markdown files under .capcom/knowledge/, each with a title, Labels, Refs and a body, nested by Parent like Issues. A Ref names a path or path#symbol the Note depends on, and know verify lists the Refs that no longer resolve. A Note that no longer holds is retired rather than removed, optionally naming the Notes that supersede it, and leaves browsing with its subtree. A write that leaves the capcom block of AGENTS.md stale refreshes it.

```
capcom know [flags]
```

### Examples

```
  capcom know tree --depth 2       # the top two levels of the Knowledge tree
  capcom know show release-steps   # one Note in full
  capcom know search release       # Notes that mention release
  capcom know verify               # Note refs that no longer resolve
```

### Options

```
  -h, --help   help for know
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents
* [capcom know add](capcom_know_add.md)	 - Add a Note with a chosen id
* [capcom know mv](capcom_know_mv.md)	 - Rename a Note or move it below another
* [capcom know rm](capcom_know_rm.md)	 - Remove a Note that has no children
* [capcom know search](capcom_know_search.md)	 - Find Notes whose title or body contains every word, else some of them
* [capcom know show](capcom_know_show.md)	 - Show a Note in full
* [capcom know tree](capcom_know_tree.md)	 - Print the outline of Note ids and titles
* [capcom know update](capcom_know_update.md)	 - Change a Note's title, labels, refs, retirement or body
* [capcom know verify](capcom_know_verify.md)	 - List the Note refs that no longer resolve

