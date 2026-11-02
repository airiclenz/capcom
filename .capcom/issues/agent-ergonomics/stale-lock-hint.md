---
title: A refusal on a held lock does not say how to resume or take it over
status: open
type: bug
priority: 2
created: 2026-11-02T20:58:47Z
---

When `capcom lock acquire`, `capcom start` or `capcom lock check` meets a lock another purpose
holds, the failure ends in `hint: capcom lock status`, even when the lock is stale. Neither the
error nor `lock status` names the two ways on: `capcom lock acquire --purpose "<held purpose>"`
resumes the lock from its kept token file when the work is yours, and `--take-stale` takes a stale
lock over. Both are only in `capcom lock acquire --help`.

Seen on capcom 0.14.0 (reproduced in a scratch repo): with a stale lock held for purpose `foo`,
`capcom lock acquire --purpose bar` prints

    capcom: .../locks/_main.json: locked by airic@devbox-pi5 (foo) for 44h56m24s, heartbeat 44h56m24s ago, stale
    hint: capcom lock status

and `capcom lock status` prints only the holder line.

## Why

Seqcom, the autonomous agent on seqcom-pi4, found an abandoned run's lock at the start of four runs
(20261102T1404Z, 1700Z, 1720Z, 1803Z) and spent turns on each, one reading the raw token from
`.git/capcom/locks/_main.*.token` because nothing pointed it at resuming by purpose. Seqcom's
wrapper now frees the lock after every run, but any agent that inherits an abandoned lock hits the
same dead end.

## Acceptance

- A locked refusal on a stale lock hints both `capcom lock acquire --purpose "<held purpose>"` (resume) and the same command with `--take-stale` (take over), with the held purpose filled in.
- A locked refusal on a live lock hints the resume command with the held purpose and says to ask the user otherwise; it does not suggest `--take-stale`.
- `--json` carries the same hint; tests cover the stale and the live case.
