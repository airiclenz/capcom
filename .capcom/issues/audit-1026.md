---
title: "Robustness audit of 2026-10-26: defects and improvements it found"
status: open
type: task
priority: 3
labels: [audit]
standing: true
created: 2026-10-26T20:06:44Z
---

A robustness audit of capcom on 2026-10-26, run against the tree at v0.11.0. Every
finding below was reproduced with the built binary in a throwaway git repository before it was
filed, except where an Issue says it is inferred from code.

The audit looked for three things: work that can be lost or silently hidden, the Tree lock as a
signal to other agents, and how easily a small model can drive the tool. The children are the
defects and improvements it turned up, grouped by those concerns.

Close the children, not this Issue: it only groups them.
