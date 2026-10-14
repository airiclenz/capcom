---
title: "The session briefing"
description: "What capcom prime prints at the start of an agent session (Ready work, Issues in progress, Tree locks, broken Note refs, store health, Elsewhere) and when to run it."
sidebar: { order: 2 }
---

The **Briefing** is what capcom tells an agent at the start of a session, or after its context is
compacted: the work that is Ready, what is already in progress, who holds the Tree locks, the
changes waiting on other branches, and the Notes whose Refs are gone. `capcom prime` prints it.

## What `capcom prime` prints

```console
$ capcom prime
ready, planned: 0
ready, unplanned: 2
  session-refresh  open  P2  Refresh the session token
  rate-limit  open  P2  Rate-limit login attempts
doing: 1
  login-error-copy  active  P1  Reword the login errors
locks: 1
  _main: held by claude@devbox for 0m, heartbeat 0m ago: implementing the login rework
this worktree: held; if the token is yours resume with capcom lock acquire --purpose <text> --token <token>, otherwise do not edit it
broken note refs: 0
check: no problems
elsewhere: rate-limit: active on rate-limit-work (unmerged)
```

Section by section:

- **`ready, planned` and `ready, unplanned`**: the [Ready](../concepts/issues.md#ready) Issues,
  split into those linked to a plan document (their `plan:` field is set) and those without one,
  each in the order `capcom ready` uses and capped at 15. A planned Issue usually has its how and
  its order written down already, so it is often the better place to start.
- **`doing`**: the Issues in the `doing` Status category: work someone has claimed. Check it before
  starting something new, so as not to pick up an Issue another session is on.
- **`locks`**: the [Tree locks](../concepts/tree-locks.md) held in every worktree of the clone, with
  their holder, age, heartbeat and purpose. The `this worktree:` line after the list says what to
  do about the lock of the worktree `prime` runs in:
  - `free`: nobody holds it; take it with `capcom lock acquire --purpose <text>` before a change
    that spans several files or commits, and keep the token it prints.
  - `held`: a session holds it; if the token is yours, resume with
    `capcom lock acquire --purpose <text> --token <token>`, otherwise another session is editing
    the worktree, so do not edit it.
  - `stale`: its holder stopped sending heartbeats; resume with your `--token` if it is yours,
    otherwise ask the user before taking it over with `--take-stale`.

  The line is left out when the locks cannot be listed, and when this worktree's lock record is
  damaged; the warning then names the record.
- **`broken note refs`**: the [Refs](../concepts/knowledge.md#ref-and-suspect-notes) of Notes that
  no longer resolve, each as `path:line: ref "<ref>": <why>`. Those Notes are suspect: read them
  with care, and fix them when the work touches them.
- **`check`**: the number of problems `capcom check` finds. Anything other than `no problems` is
  worth a `capcom check` before going further.
- **`elsewhere`**: [Issue changes on other branches](../concepts/branches-and-merging.md#elsewhere)
  that this one has not merged, here an Issue claimed on `rate-limit-work`.

## When to run it

- **At the start of every session.** The agent learns the state of the work before it touches
  anything. The [Claude Code and Codex hooks](agent-setup.md) do this automatically.
- **Around a context compaction.** A long session forgets its early context when it is compacted;
  the hooks run `prime` again so the agent keeps an up-to-date picture.
- **After a merge or a branch switch.** The Ready list and the Elsewhere report are those of the
  checked-out branch, so they change when it does.

People can run it too: it is a quick answer to "where were we?" when coming back to a repository.

## It never fails

`capcom prime` is built to run unattended in a hook, so it always exits 0. A part it cannot read
becomes a `warning:` line and the rest of the Briefing is still printed. A store written by a newer
capcom than the one installed is reported the same way, as one `warning: store:` line, where every
other command fails.

## As JSON

`capcom prime --json` returns the same Briefing in the [JSON envelope](../reference/json-output.md)
with kind `prime`. Its `data` holds:

| Key | Value |
| --- | --- |
| `ready_planned`, `ready_unplanned` | `{"count": N, "issues": [...]}`, the issues as `capcom show --json` describes them, capped at 15 |
| `doing` | the Issues in the `doing` category |
| `locks` | one object per held lock: `lock`, `worktree`, `holder`, `purpose`, `host`, `user`, `branch`, `since`, `heartbeat`, `stale` |
| `broken_refs` | the Note Refs that do not resolve |
| `check_problems` | the number of problems `capcom check` finds |

The Elsewhere entries sit in the envelope's top-level `elsewhere` array, as for every command.

## Next

- [Agent setup](agent-setup.md): run `prime` from Claude Code and Codex hooks.
- [The Managed block](managed-block.md): the standing instructions an agent reads beside the
  Briefing.
