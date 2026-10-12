---
title: "Before filing, starting or closing an issue: its body, claiming it, dependencies"
---

**The body.** An issue's body is self-contained: what is wrong or wanted, why, and how to tell it
is done under `## Acceptance`, readable without its plan. The plan says how and in what order, so
archiving a plan strands no issue. Never file a stub: `create` without a body warns.

**Claiming work.** Move the issue to a doing status (`capcom update <id> --status active` in this
repository) and hold this worktree's Tree lock with `capcom lock acquire --purpose <id>`. `prime`
lists the doing issues, so the next session sees who works on what. There is no assignee key.

**Dependencies.**
- Record them with `capcom create --dep <id>` or `capcom dep add <id> <dep-id>`.
- Read them with `capcom dep list <id> --depth N` (deps and dependents, 0 prints every level),
  `capcom dep graph [id]` (a Mermaid flowchart, edges point dependency to dependent) and
  `capcom list --tree --depth N` (the parent and child outline).
- `capcom close <id>` ends its text with `now ready: a, b`, the issues this close made Ready.
- `capcom ready` ranks by priority, and a priority tie goes to the issue that unblocks more,
  shown as `-- unblocks N`.
