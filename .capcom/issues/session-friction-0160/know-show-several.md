---
title: know show takes one Note while show takes several issues
status: open
type: feature
priority: 3
labels: [agent-ergonomics]
plan: 2026-11-22 - 01 - session-friction-followups-plan
created: 2026-11-22T17:47:25Z
---

`capcom show <id>...` prints several issues; `capcom know show` accepts one Note.

Seen: 12 shell loops such as
`for a in 0012 0031 0044; do capcom know show "ADR $a" | sed -n 1,30p; done` and
`for t in <term> <term> <term>; do capcom know show "$t" | head -40; done`, and chains of
`capcom know show a; echo ====; capcom know show b`. `know show` was the second most-run
read (392 calls).

Wanted: `capcom know show <id|alias|prefix>...` prints each Note in turn, separated as
`show` separates issues, and reports an argument that names no Note as `show` does.
`--with-children` applies to each.

## Acceptance

- `capcom know show a b` prints both Notes; text and `--json` are tested.
- A missing id among several is reported the same way `capcom show` reports it.
