---
title: Friction agents hit with capcom 0.16.0 in real sessions
status: open
type: task
priority: 2
labels: [agent-ergonomics]
standing: true
created: 2026-11-22T17:47:25Z
---

Groups the findings of the session review of 2026-11-22: every capcom call made since the
v0.16.0 cut (2026-11-08) in 9 apogee sessions (a small model, 30 calls) and about 80 Claude
Code session files (about 1,750 calls), in other repositories that use capcom. Counts in
the children are approximate: a resumed session repeats some calls.

Hard failures were rare, about 30 in 1,780 calls. The friction is noise (warnings on the
holder's own writes), extra calls (lock touch, check, show loops) and agents reading
`.capcom/` files directly instead of using the CLI.

One finding is not capcom's: a guide file that links Notes by file path
(`.capcom/knowledge/decisions/<id>.md`) teaches agents to `cat` them.

## Acceptance

- Every child is closed or dropped with a reason.
