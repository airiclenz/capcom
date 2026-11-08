---
title: "capcom ui"
description: "Open the terminal UI"
sidebar:
  order: 1
---

## capcom ui

Open the terminal UI

### Synopsis

Open the terminal UI over this store: the Ready, Issues, Knowledge and Closed tabs.

With no argument it reopens on the tab and the selection of each tab it last had in this worktree, kept in <git-dir>/capcom/ui-state.yaml, and on the Ready tab the first time.

A tab name opens that tab, and an id after it selects that Issue or Note there; the arguments win over what was kept.

It needs a terminal: when stdin or stdout is not one, it fails.

With CAPCOM_UI_TRACE set to a path, it appends to that file one line per detail-pane read event (requested, applied, stale, rendered, slow-read notice), per wait over 100ms for the store, and per change of the detail pane it draws, each a UTC time, an event and key=value fields; a path it cannot open fails it before the screen opens.

q or ctrl+c quits.

--json does not apply and is refused.

```
capcom ui [issues|ready|knowledge|closed] [id] [flags]
```

### Examples

```
  capcom ui                           # the last tab and selection in this worktree
  capcom ui issues                    # the Issues tab
  capcom ui issues login-timeout-race # the Issues tab with that Issue selected
  capcom ui knowledge store-layout    # the Knowledge tab with that Note selected
  capcom ui closed login-timeout-race # the Closed tab with that closed Issue selected
```

### Options

```
  -h, --help   help for ui
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

