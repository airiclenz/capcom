---
title: prime --json drops whether this worktree is free to edit
status: done
type: bug
priority: 2
plan: 2026-10-26 - 03 - audit-p1-p2-fixes-plan
created: 2026-10-26T20:06:49Z
closed: 2026-10-26T21:45:33Z
reason: Resolved by plan 2026-10-26 - 03 - audit-p1-p2-fixes-plan (archived in 575c3b9)
---

`capcom prime --json` drops the sentence that tells an agent whether it may edit this worktree.

Text prints:

    locks: 0
    this worktree: free; before a multi-file change run capcom lock acquire --purpose <text>, ...
    (or: this worktree: held; if it is yours, ... otherwise do not edit it)

JSON has `locks` and no field naming the caller's own worktree, its state, or the advice
(`primeData`, `internal/cli/prime.go:47-56`):

    data keys: broken_refs, check_problems, doing, locks, ready_planned, ready_unplanned

Each `locks` entry carries its `worktree` key and state, but without the caller's own key an agent
cannot tell which entry is its own. `prime` is the session-start briefing, and an agent that parses
JSON, as agents are encouraged to do, loses the part about the lock and has to parse prose or run
`capcom lock check` as well. `lock check` remains the documented gate before editing and works, so
this is a gap in the briefing, not a hole in the gate.

## Acceptance

- `prime --json` carries the caller's worktree key, its lock state as the text classifies it
  (free, held, stale, invalid, unreadable; `prime` holds no token, so it cannot say whose) and the
  advice sentence.
- The text output is unchanged.
- The `prime` payload is described in `docs/site/reference/json-output.md`. The new fields are
  additive, so `schema` stays 1.
- A test compares the text and JSON for each state and asserts the state is present in both.
