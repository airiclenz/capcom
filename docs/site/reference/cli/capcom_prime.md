---
title: "capcom prime"
description: "Print the session Briefing: ready work, locks, other branches and store health"
sidebar:
  order: 1
---

## capcom prime

Print the session Briefing: ready work, locks, other branches and store health

### Synopsis

Print the Briefing an agent session starts with: the Ready Issues, split into those with a plan and those without, each list capped at 15; the Issues in progress; the Tree locks held in every worktree; changes to Issues on other branches that are not merged yet; Note refs that no longer resolve; and the number of problems capcom check finds. It never fails: a part that cannot be read becomes a warning line, and the command exits 0. init --claude-hooks and init --codex-hooks run it at session start and before compaction.

```
capcom prime [flags]
```

### Examples

```
  capcom prime          # the session-start Briefing
  capcom prime --json   # the same Briefing as a JSON envelope
```

### Options

```
  -h, --help   help for prime
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

