---
title: Starting an issue takes three commands agents get wrong
status: done
type: feature
priority: 1
plan: 2026-10-22 - 00 - start-finish-commands-plan
created: 2026-10-19T14:54:44Z
closed: 2026-10-22T09:57:09Z
reason: "Delivered by plan 2026-10-22 - 00 - start-finish-commands-plan (archived in bfe8cfc): capcom start, its tests, Managed block and guides"
---

Starting work on an Issue takes three commands: `capcom update <id> --status active`,
`capcom lock acquire --purpose <id>` (keeping the printed token) and `capcom show <id>`
(see the `issue-workflow` Note). Small models drop one of the three, most often the lock.

Wanted: `capcom start <id>` claims the Issue (moves it to the first doing status), acquires
this worktree's Tree lock with the Issue id as purpose, and prints the Issue in full followed
by the Lock token on its own clearly labelled line. `--token` resumes an existing lock instead
of acquiring a new one. It refuses, before writing anything, when the Issue is not Ready, is a
standing Parent, or another session holds the lock, and each refusal names the command that
resolves it. Starting an Issue already in doing under your own token is a successful no-op.

Writes go through `internal/ops` (ADR 0012), not straight to files.

## Acceptance

- One `capcom start <id>` leaves the Issue in doing and the lock held, and prints the token.
- Every refusal case writes nothing: not Ready, standing, closed, lock held by another session.
- Running it twice with `--token` succeeds without changing anything the second time.
- `--kv` and `--json` outputs include the token; CLI tests cover every case above.
- The `issue-workflow` Note and the Managed block name `capcom start` as the way to claim.
