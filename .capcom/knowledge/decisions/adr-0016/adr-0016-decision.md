---
title: "ADR 0016 decision: acquire and start write the Lock token to a purpose-keyed token file in capcom/locks; naming that purpose again without --token resumes from it"
---

The Tree lock tells sessions apart by a secret Lock token that the holder presents to `touch`,
`release` and a resuming acquire (ADR 0005). Agents lose it. A context compaction drops it, a
harness that runs each tool call in a fresh shell drops any `export`, and small models fail to
carry it from one turn to the next even without compaction. A lost token leaves a lock that can
only be broken, or a stale one that blocks the next session until its TTL runs out. capcom
therefore keeps the token for the session, and the session finds it again by the purpose it
already names.

**The token file.** One file per worktree and purpose, in the lock directory beside the lock
records: `<git-common-dir>/capcom/locks/<worktree-key>.<purpose-hash>.token`. `<worktree-key>` is
the record's key (`_main` or `wt-<linked worktree name>`); `<purpose-hash>` is the lowercase hex
SHA-256 of the purpose, so any purpose text becomes a safe, fixed-length file name on every
platform. The file holds the token on its first line and the purpose on its second, so a reader
confirms the purpose it was asked for; it is written through a temp file renamed into place,
readable and writable by its owner only. It is never committed and never part of the store, as
the lock records are not.

**It is never read as a lock record.** The Lock overview, `capcom lock` and `Break` take every
non-hidden `*.json` file in `capcom/locks/` as one worktree's record (`recordKeys` in
`internal/lock/lock.go`). A token file's name must therefore never end in `.json`: the `.token`
extension keeps it out of that listing, as the `.lck` guard files are. Any other placement the
implementation prefers must meet the same rule: a hidden or non-`.json` name in `capcom/locks/`, or
a sibling directory such as `capcom/tokens/`, always with the purpose encoded into a safe file
name, never written into a path verbatim.

**Writing it.** `capcom lock acquire --purpose <text>` and `capcom start <id>` (whose purpose is
the Issue id) write the token file for this worktree and purpose whenever they take or resume the
lock, and still print the token on its token line, so a session that keeps its own token loses
nothing.

**Resuming from it.** An acquire or start without `--token` first reads the token file for this
worktree and the purpose it names. When the file's token matches the current lock record, the call
resumes that lock exactly as `--token` would, a stale one included, and prints the token again;
this is how a session that lost its context carries on. When the file is missing, its token does
not match the record, or the lock is free, the call behaves as today: a free lock gets a new token
and the file is rewritten; a lock another session holds fails `locked` with exit code 3, and
`--take-stale` keeps its meaning. A `--token` given on the command line always wins over the file.
A token file whose lock has gone, been broken or been taken over is harmless: its token no longer
matches any record, so it resumes nothing, and the next acquire for that purpose replaces it.

**Removing it.** `capcom lock release` deletes the worktree's token file whose token it presented;
`capcom finish <id>` releases through the token file for the Issue id and deletes it, so finishing
needs no `--token` either; `capcom lock break` deletes the worktree's token files with its record.
`touch` keeps taking `--token`, and a session that lost it re-runs its acquire or start, which
resumes, refreshes the heartbeat and prints the token.

**The cost, and why it is acceptable.** The token no longer proves which session took the lock:
any session in the clone that names the same purpose resumes it. That is acceptable because the
purpose an agent names is the Issue id that `capcom prime` lists as doing, so a second session
naming it is working the same Issue in the same worktree, which is a continuation of the same work,
not a rival. A session naming any other purpose still fails `locked`, so the lock keeps telling
different work apart, which is what it is for. The lock was always advisory (ADR 0005): it is a
public signal by which agents tell each other that a worktree is being worked on, and by which
they recognise their own work again, not a secret that secures the worktree. The token guards
against accident, not against a process of the same user that reads the git dir.

**Against the constraints of `lock-token-recall`:**

- *A fresh shell per tool call.* The file outlives every shell; nothing rests on the environment.
- *A token readable by every process no longer tells sessions apart.* Sessions are told apart by
  purpose instead of by secret, at the cost stated above; a different purpose is still refused.
- *Sub-agents commit under the session's lock.* A sub-agent that names the session's purpose
  resumes the same lock without being handed the token.
- *Some harnesses expose a session id, most do not.* Nothing depends on one, so every harness
  works the same way.

Implementing the file is the follow-up Issues under `agent-ergonomics` that depend on
`lock-token-recall`: one for the token file in `internal/lock` and `capcom lock`, one for
`capcom start` and `capcom finish` using it.
