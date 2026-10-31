---
title: "capcom apply"
description: "Apply a file of issue writes, all or none"
sidebar:
  order: 1
---

## capcom apply

Apply a file of issue writes, all or none

### Synopsis

Apply an apply file: a YAML list of Issue writes, run in order and written all or none. Give its path, or - to read it from stdin.

Each entry is a map whose op key names the write (create, update, dep add, close or log) and whose other keys are that command's flag names without the leading --, plus id, dep and text. Values are inline: body-file and body-append-file are refused.

update, dep add, close and log take if-match, compared with the Issue's file as it was before the batch; it is refused on an Issue an earlier operation created.

The writes are staged in memory and land together once every one has passed. The first that fails stops the batch, naming its operation and line, and nothing is written. Closing an Issue already Closed fails the batch.

--dry-run runs the same batch and reports the same outcome, but writes nothing.

It prints one line per operation: its position, op, Issue id and outcome, then a "now ready:" line naming the Issues that existed before the batch and are Ready after it.

```
capcom apply <file|-> [flags]
```

### Examples

```
  capcom apply batch.yaml             # every operation of batch.yaml, or none
  capcom apply batch.yaml --dry-run   # the outcome, with nothing written
  capcom apply - < batch.yaml         # the apply file from stdin
  capcom apply batch.yaml --json      # the outcome in a JSON envelope
```

### Options

```
      --dry-run   run the batch and report its outcome, writing nothing
  -h, --help      help for apply
      --kv        print KEY=VALUE lines ending in STATUS=
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

