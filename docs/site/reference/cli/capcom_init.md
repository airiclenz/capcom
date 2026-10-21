---
title: "capcom init"
description: "Create the capcom store (.capcom/) in this git repository"
sidebar:
  order: 1
---

## capcom init

Create the capcom store (.capcom/) in this git repository

### Synopsis

Create the capcom store in the current git working tree: .capcom/ with its issues and knowledge directories and config.yaml. An existing store is left as it is. Without --claude-hooks it prints the Claude Code hooks that run capcom prime at session start and before compaction, for you to add; --claude-hooks merges those SessionStart and PreCompact hooks into .claude/settings.json, and --codex-hooks into .codex/hooks.json. --apogee-hooks adds a reaction running capcom prime on apogee's session-started and compaction-finished to .apogee/config.yaml, which apogee asks each user to adopt once; it needs apogee v0.28.0 or later. --merge-driver hands issue files to the capcom-log merge driver in .gitattributes and prints the two git config commands that define it. The flags are independent and can be combined.

```
capcom init [flags]
```

### Examples

```
  capcom init                  # create .capcom/ in this git repository
  capcom init --claude-hooks   # ...and run capcom prime from Claude Code's session hooks
  capcom init --codex-hooks    # ...and from Codex's, which needs [features] hooks = true
  capcom init --apogee-hooks   # ...and from apogee's session-started and compaction-finished reaction
  capcom init --merge-driver   # ...and merge issue Logs by entry (prints two git config lines)
```

### Options

```
      --apogee-hooks   also add a reaction running capcom prime at session start and after compaction to .apogee/config.yaml
      --claude-hooks   also add SessionStart and PreCompact hooks running capcom prime to .claude/settings.json
      --codex-hooks    also add SessionStart and PreCompact hooks running capcom prime to .codex/hooks.json
  -h, --help           help for init
      --merge-driver   also hand issue files to the capcom-log merge driver in .gitattributes
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

