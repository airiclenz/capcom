# The Tree lock is a token with a heartbeat, not a held process

A Tree lock belongs to one worktree, and the records of all of a clone's worktrees live together
under the git common dir (`<git-common-dir>/capcom/locks/<worktree>.json`), never in git. The holder
is an agent *session*, not a process. An orchestrating skill such as `/implement-plan` runs every
command in a fresh shell, and several sub-agents work under one claim, so a lock bound to a pid or
an open OS file lock would look abandoned the moment `acquire` returned. Acquiring therefore
returns a token that the holder keeps (in its run ledger) and presents to `touch`, `release` and a
re-acquire. A heartbeat refreshed by `touch` keeps the lock live, and the lock is stale once the
heartbeat is older than the configured TTL (2 h by default). Acquiring with your own token always
resumes, even a stale lock, which is how a session that lost its context to compaction carries
on. A stale lock is only taken over on request (`--take-stale`), and `break` is an explicit human
act. The lock is advisory: capcom installs no git hooks by default, and an OS file lock guards only
the brief read-modify-write of the record.

## Considered Options

- **One lock per clone (git common dir).** Simpler, but it serialises agents that work in separate
  worktrees, which do not share files.
- **pid liveness or a held `flock`.** Fails with fresh-shell orchestrators, as described above.
- **An enforcing pre-commit guard, like MCP Agent Mail's.** Brings back the hook-trust burden capcom
  exists to remove; a `lock check` command lets a repo opt in.
