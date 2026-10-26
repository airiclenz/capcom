---
title: prime --json drops whether this worktree is free to edit
status: open
type: bug
priority: 1
created: 2026-10-26T20:06:49Z
---

`capcom prime --json` drops the one sentence that decides whether an agent may edit the tree.

Text prints:

    locks: 0
    this worktree: free; before a multi-file change run capcom lock acquire --purpose <text>, ...
    (or: this worktree: held; if it is yours, ... otherwise do not edit it)

JSON has `"locks": []` and no field naming the caller's own worktree, its lock state, or the
advice:

    data keys: broken_refs, check_problems, doing, locks, ready_planned, ready_unplanned

`prime` is the session-start briefing, and the lock answer is the permission to write at all. An
agent that parses JSON, as agents are encouraged to do, loses exactly the safety-critical part and
still has to parse prose or guess.

## Acceptance

- `prime --json` carries the caller's worktree key, its lock state (free, held by you, held by
  another, stale, unreadable) and the advice sentence.
- The text output is unchanged.
- A test compares the text and JSON for each state and asserts the state is present in both.
