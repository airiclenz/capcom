---
title: Agents run capcom check after nearly every CLI write
status: open
type: task
priority: 3
labels: [agent-ergonomics]
plan: 2026-11-22 - 00 - lock-holder-and-agent-guide-plan
created: 2026-11-22T17:47:25Z
---

The managed block says "Prefer capcom commands; after a hand edit run `capcom check`"
(`internal/agents/block.go`), and `check --help` says to run it after a hand edit and
every merge. Agents run it far more often.

Seen: 306 `capcom check` calls in the reviewed sessions, while hand edits of store files
were rare (4 `sed -i`, 4 Edit or Write tool calls, a few python replaces). Most checks
follow a `create`, `know add`, `update` or `close`, which already validate what they write.

Why: about one wasted call per write.

Wanted: the block and `check --help` say that a write made through a capcom command is
already validated and needs no check; check is for hand edits, merges and before a commit
of hand-edited store files. First confirm the claim: list any problem `check` reports that
a CLI write can introduce (for example a `--ref` that does not resolve is written with a
warning and fails check), and name those cases in the wording.

## Acceptance

- The block and `check --help` state when check is needed and when it is not.
- Any CLI write that can leave the store failing check says so in its own output.
