---
title: Keep the Lock token in a purpose-keyed token file (ADR 0016)
status: open
type: feature
priority: 1
deps: [lock-token-recall]
created: 2026-10-22T09:29:07Z
---

Agents lose the Lock token across turns, fresh shells and context compactions (see
`lock-token-recall`), which leaves a lock that can only be broken or a stale one that blocks the
next session. ADR 0016 (`adr-0016`, proposed) settles it: capcom keeps the token for the session in
a token file keyed by worktree and purpose, and naming the purpose again resumes the lock. This
Issue builds the file in `internal/lock` and wires it into `capcom lock`; `capcom start` and
`capcom finish` use it in `start-finish-token-file`. Do not start before the ADR is accepted, which
closes `lock-token-recall`.

Wanted:

- `internal/lock` writes, reads and deletes a token file per worktree key and purpose in the lock
  directory, `<git-common-dir>/capcom/locks/<worktree-key>.<purpose-hash>.token`, where
  `<purpose-hash>` is the lowercase hex SHA-256 of the purpose. The file holds the token on its first
  line and the purpose on its second; it is written through a temp file renamed into place with
  owner-only permissions.
- The token file is never read as a lock record. `recordKeys` (`internal/lock/lock.go`) takes every
  non-hidden `*.json` file in `capcom/locks/` as a worktree key, so a token file's name must never
  end in `.json`. Any other placement must keep the same rule: a hidden or non-`.json` name in
  `capcom/locks/`, or a sibling directory such as `capcom/tokens/`, with the purpose encoded into a
  safe file name and never written into a path verbatim.
- `capcom lock acquire --purpose <text>` writes the token file whenever it takes or resumes the
  lock, and still prints the token line.
- `capcom lock acquire --purpose <text>` without `--token` resumes the lock when the token file for
  this worktree and purpose holds the current record's token, a stale lock included, and prints
  the token again. A missing or non-matching file behaves as today: a free lock gets a new token, a
  lock another session holds fails `locked` with exit code 3, `--take-stale` keeps its meaning. A
  `--token` on the command line wins over the file.
- `capcom lock release` deletes the worktree's token file whose token it presented;
  `capcom lock break` deletes the worktree's token files with the record. `touch` is unchanged.
- The help text of `lock acquire` and `lock release` says the token is kept for the session, and
  the `issue-workflow` Note and the Managed block stop telling agents to keep the token themselves.

## Acceptance

- Acquiring with a purpose, then acquiring again with the same purpose and no `--token`, resumes
  the same lock with the same token, a stale one included; a different purpose fails `locked`.
- A token file whose token no longer matches the record (released, broken, taken over) resumes
  nothing, and the next acquire for that purpose replaces it.
- `capcom lock status --all`, `capcom lock check` and the Lock overview list no token file as a
  worktree and report no problem for one; a test puts a token file beside a record to prove it.
- Release and break remove the token files named above; `--token` still wins over the file.
- Unit tests in `internal/lock` and CLI tests per `cli-test-harness` cover each case;
  `docs/site/reference/cli/` is regenerated with `go run ./cmd/gendocs`.
