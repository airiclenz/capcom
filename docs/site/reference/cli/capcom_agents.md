---
title: "capcom agents"
description: "Write or refresh the capcom block of AGENTS.md"
sidebar:
  order: 1
---

## capcom agents

Write or refresh the capcom block of AGENTS.md

### Synopsis

Write or refresh capcom's Managed block in the AGENTS.md at the working tree root: the section between <!-- BEGIN CAPCOM --> and <!-- END CAPCOM --> that tells agents how to use capcom and lists the top level of the Knowledge tree. A stale block is replaced, an absent one appended and a missing AGENTS.md created; the rest of the file is left alone. Each agent guide config.yaml lists under agents.guides (CLAUDE.md and GEMINI.md by default) that exists but neither links to AGENTS.md nor imports it gets an @AGENTS.md import line appended; a missing guide is not created, and one that links outside the working tree is skipped. --check writes nothing: it reports whether the block is current, and fails invalid when it is stale or a guide does not reach it, so CI catches a block that has drifted; a file without the block passes.

```
capcom agents [flags]
```

### Examples

```
  capcom agents           # write or refresh the capcom block of AGENTS.md, import it into the guides
  capcom agents --check   # fail when the block is out of date or a guide does not reach it
```

### Options

```
      --check   report whether the block is current; fail when it is stale or a guide does not reach it
  -h, --help    help for agents
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

