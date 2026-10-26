---
title: "capcom search"
description: "Find issues whose title or body contains every word, else some of them"
sidebar:
  order: 1
---

## capcom search

Find issues whose title or body contains every word, else some of them

### Synopsis

Find the Issues whose title or body contains every word given, matched as whole words and case-insensitively, best match first; closed Issues are searched too. It prints one line per Issue with its id, status, priority and title. When no Issue holds every word, it lists the Issues holding some of them under the line "No item holds every word; these hold some of them:", and --json sets "match" to "any" instead of "all". It fails with not_found when no Issue holds any word.

```
capcom search <word>... [flags]
```

### Examples

```
  capcom search refresh timeout   # issues mentioning both words, else either
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

