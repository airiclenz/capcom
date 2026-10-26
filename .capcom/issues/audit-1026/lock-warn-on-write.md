---
title: A write while another session holds the lock is silent
status: open
type: feature
priority: 2
labels: [audit]
created: 2026-10-26T20:13:21Z
---

A mutating command writes to the worktree even when another session holds the Tree lock, and says
nothing. The lock is how agents share one worktree, but only `capcom ui` reacts to it, so the CLI
record is a sign on the door that most commands do not read.

Reproduced: one session takes the lock, then, in the same worktree,

    capcom lock acquire --purpose other-session   -> lock taken
    capcom create --id sneaky --title Sneaky --body b   -> exit 0, file written
    capcom mv sneaky sneaky2                            -> exit 0, file moved
    capcom lock check                                   -> exit 3, "locked by ... (other-session)"

So `lock check` correctly reports the worktree as taken while `create` and `mv` write anyway. One
agent with a half-finished multi-file change, and a second agent running a command, now hold
overlapping work that neither of them caused on purpose.

This is not an oversight in the code: ADR 0013 decided that only the UI asks before writing, and
its rejected list turns down blocking CLI writes because a supervisor often needs to act exactly
then, and `$EDITOR` would bypass a block anyway. That reasoning holds. But it leaves a gap for the
case capcom is built for — several agents on one worktree — where the lock separates them on
paper and not in practice. A warning keeps the supervisor's escape hatch and closes the silence.

## Proposal

Warn on stderr when a mutating command lands a write while this worktree's lock is held by another
session, naming the holder, and pending on nothing: the write still happens and the exit code does
not change.

The warning belongs on stderr rather than in the `warnings` array, which every existing warning
uses. `warnings` are part of the result written to stdout inside the envelope, and an agent that
pipes a result into a parser would have to learn a second place to look. A note about the tree's
state is not part of the result, and stderr is where capcom already writes what must not be parsed
as output. The same warning should also reach the envelope's `warnings` when `--json` is set, or
`--json` callers learn nothing.

Not this Issue: refusing the write (ADR 0013 rejected it), and warning while the lock is your own
— `capcom start`, `finish` and `lock acquire` hold it by design.

## Acceptance

- A mutating command that writes while another session holds this worktree's lock prints one
  warning on stderr naming the holder and the purpose, and still exits 0 with its normal output.
- No warning when the lock is free, when the token belongs to this session, or when the command is
  `start`, `finish` or any `lock` verb.
- A stale lock warns and says it is stale, so the reader knows `--take-stale` is available; a
  corrupt or unreadable record warns without failing the command.
- `--json` and `--kv` carry the same warning for a parsing caller, in the envelope's `warnings`;
  stdout keeps its documented shape.
- A read-only command (`list`, `show`, `ready`, `search`, `prime`) never warns.
- A test holds a foreign lock, runs `create` and `mv`, and asserts exit 0, the file written, and
  the stderr warning; and runs them with a free lock and asserts stderr is empty.
- The behaviour is named in `AGENTS.md`'s managed block and in the `issue-workflow` Note, so an
  agent reading either learns the lock is signalled and not enforced.
