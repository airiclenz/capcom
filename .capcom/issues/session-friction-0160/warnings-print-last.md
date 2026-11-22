---
title: Warnings print after the result, where head and tail pipelines hide or split them
status: done
type: bug
priority: 3
labels: [agent-ergonomics]
plan: 2026-11-22 - 00 - lock-holder-and-agent-guide-plan
created: 2026-11-22T17:47:25Z
closed: 2026-11-22T19:29:03Z
reason: "resolved by plan \"2026-11-22 - 00 - lock-holder-and-agent-guide-plan\" (bf30b14): item 6 prints warnings before the result in text output"
---

A command's warnings print below its result (`app.emit`, `internal/cli/root.go`). Agents
often cut the output.

Seen: `capcom create ... 2>&1 | tail -5` and `| tail -6`, where what survives depends on
the line count; `capcom show ... | head -N` (92 calls) drops any warning; a lock warning appeared
between the `id/path/etag` lines of two commands of one call, reading as part of the
second.

Why: a warning the agent never sees does nothing, and one that lands in the middle of the
next command's output is misread.

Wanted: decide where warnings go so that a cut output still shows them: first, before the
result, is the simple answer for text output. `--json` and `--kv` keep their fields.
ADR 0017 chose the normal warnings channel over stderr; this changes the order, not the
channel.

## Acceptance

- In text output a command's warnings are visible in its first lines.
- Golden tests are updated; `--json` and `--kv` are unchanged.
