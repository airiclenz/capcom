---
title: "capcom version"
description: "Print the capcom version"
sidebar:
  order: 1
---

## capcom version

Print the capcom version

### Synopsis

Print the version of this capcom binary as capcom <version>.

--kv prints it as a VERSION line followed by STATUS, and --json inside a versioned JSON envelope.

```
capcom version [flags]
```

### Examples

```
  capcom version          # capcom <version>
  capcom version --kv     # VERSION=<version>, then STATUS=ok
  capcom version --json   # {version} in a JSON envelope
```

### Options

```
  -h, --help   help for version
      --kv     print KEY=VALUE lines ending in STATUS=
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

