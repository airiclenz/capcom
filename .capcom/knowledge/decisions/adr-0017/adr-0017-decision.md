---
title: "ADR 0017 decision: a CLI write warns while the lock is held unless it writes the Issue named by the lock's purpose, in the normal warnings channel"
---

The Tree lock is advisory (ADR 0005), and until now only `capcom ui` read it before a write
(ADR 0013). An agent checks it with `capcom lock check` before a multi-file change, but a second
agent that runs `capcom create` or `capcom mv` in the same worktree writes without a word, so the
lock separates two sessions on paper and not in practice.

**When it warns.** A mutating command reads this worktree's lock record at the moment of the write.
It adds a warning when the record is held, live or stale, and the write does not target the Issue
whose id is the lock's purpose. `capcom start <id>` sets the purpose to the Issue id, so the
holder's own `capcom log <id>`, `update <id>` and `close <id>` on the Issue it claimed stay silent,
which is the work loop the managed block teaches. A free lock warns nothing. `start`, `finish` and
every `lock` verb never warn: they act on the lock itself. A read-only command never warns.

**Why the purpose and not a token.** A CLI write takes no token, and ADR 0016 rejected telling
sessions apart by a harness session id. The purpose is the one thing every write can compare
against without a new flag: it is the Issue id `capcom prime` lists as doing. The rule is
therefore a heuristic: a holder that writes a different Issue (a child it files, a dependency it
adds) is warned too. The warning reads as `prime` does, "held by <who> for <purpose>; if it is
yours, carry on", so a holder recognises its own lock and moves on.

**What it says.** A live lock names the holder, the purpose and the heartbeat age. A stale lock
says it is stale, so the reader knows `--take-stale` would take it. An invalid or unreadable record
warns that the lock cannot be read and names `capcom lock status`; it never fails the write.

**Where it goes.** In the warnings every command already reports: the text output's warning lines
and the envelope's `warnings` with `--json` and `--kv`. A second channel on stderr would be one
more place for a parsing agent to learn.

**Reading the lock creates nothing.** The check reads the record without the guard file, so a write
in a fresh clone or a read-only git dir neither creates `.lck` files nor fails on the lock.
