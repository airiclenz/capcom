---
title: ready.hide and duplicate-key config handling are untested
status: open
type: task
priority: 3
created: 2026-11-03T08:23:42Z
---

No test in `internal/config` sets a non-empty `hide` list, an unknown `ready.*` key, a non-list
or empty-string `hide` entry, or a duplicate top-level key (`internal/config/parse.go:226`). A
regression could drop the user's hide labels or accept a malformed entry and silently change
what `capcom ready` shows agents.

## Acceptance

- A load case `ready:\n  hide: [wip, blocked]` yields `Ready.Hide == [wip blocked]`.
- Invalid-config rows for `ready:\n  hide: wip`, `ready:\n  other: x`, `ready:\n  hide: [""]` and `types: [bug]\ntypes: [task]` each fail at the offending line.
