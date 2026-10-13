---
title: "capcom search"
description: "Find issues whose title or body contains every word"
sidebar:
  order: 1
---

## capcom search

Find issues whose title or body contains every word

### Synopsis

Find the Issues whose title or body contains every word given, matched as whole words and case-insensitively, best match first; closed Issues are searched too. It prints one line per Issue with its id, status, priority and title, and fails with not_found when nothing matches.

```
capcom search <word>... [flags]
```

### Examples

```
  capcom search refresh timeout   # issues mentioning both words
  capcom search safari
```

### Options

```
  -h, --help   help for search
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

