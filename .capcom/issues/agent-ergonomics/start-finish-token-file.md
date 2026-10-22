---
title: start and finish resume and release through the token file (ADR 0016)
status: open
type: feature
priority: 1
deps: [lock-token-recall, lock-token-file, start-command, finish-command]
created: 2026-10-22T09:29:08Z
---

`capcom start <id>` claims an Issue and acquires this worktree's Tree lock with the Issue id as
purpose, and `capcom finish <id>` closes it and releases the lock, but both still need the Lock token
carried by the agent: `start` resumes only with `--token`, and `finish` without `--token` closes the
Issue and leaves the lock held. Agents lose the token across turns and compactions
(`lock-token-recall`). ADR 0016 (`adr-0016`, proposed) has capcom keep the token in a token file
keyed by worktree and purpose; this Issue makes `start` and `finish` use the file that
`lock-token-file` builds, so neither needs `--token` from the agent. Do not start before the ADR is
accepted, which closes `lock-token-recall`.

Wanted:

- `capcom start <id>` writes the token file for the Issue id as purpose whenever it takes or
  resumes the lock, and still prints the Lock token line.
- `capcom start <id>` without `--token` resumes the lock when the token file for this worktree and
  the Issue id holds the current record's token, so running it again after a compaction is a
  successful no-op as it is today with `--token`. A missing or non-matching file behaves as start
  does without `--token` today; a `--token` given on the command line wins over the file.
- `capcom finish <id>` without `--token` releases the lock through the token file for the Issue id
  and deletes the file; `--keep-lock` leaves both the lock and the file. A missing or non-matching
  file closes the Issue and reports the lock left alone, as a wrong token does today.
- The help text of `start` and `finish`, the `issue-workflow` Note and the Managed block say the
  token is kept for the session and `--token` is optional.

## Acceptance

- `capcom start <id>` then `capcom finish <id> --reason <text>`, neither with `--token`, leaves the
  Issue closed, the lock free and no token file behind.
- `capcom start <id>` twice without `--token` resumes the same lock the second time and changes
  nothing else.
- A token file left by another purpose, or holding a token that no longer matches the record, is
  never used: start refuses as it does today and finish leaves the lock and says so.
- CLI tests per `cli-test-harness` cover each case; `docs/site/reference/cli/` is regenerated with
  `go run ./cmd/gendocs`.
