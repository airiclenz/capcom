---
title: Agents lose the Lock token across turns and compaction
status: open
type: task
priority: 1
created: 2026-10-19T14:54:44Z
---

The Tree lock (ADR 0005) needs the session to keep a secret Lock token and present it on
`touch`, `release` and resume. Agents lose it: a context compaction drops it, a fresh shell per
tool call loses any environment variable, and small models fail to carry it from one turn to the
next even without compaction. A lost token means a lock that can only be broken, or a stale one
that blocks the next session.

This Issue is a design decision, not an implementation. Constraints to weigh:

- Agent harnesses run each tool call in a fresh shell, so `export` does not persist.
- A token readable by every process in the worktree no longer tells sessions apart.
- Sub-agents of one session commit under the session's lock (see the skills research in
  `docs/design/research-notes.md`).
- Some harnesses expose a session id in the environment, most do not.

Options include: a token file named by the holder and purpose under `<git-dir>/capcom/`, a
`--token-file` flag, deriving the token from a harness session id when one is present, or
`lock acquire` printing a one-line resume command that the agent's own memory can keep.

## Acceptance

- An ADR under `docs/adr/` with status "proposed" compares the options against the constraints
  above and recommends one, plus its Note tree under `decisions` (see `adr-0010`).
- Follow-up implementation Issues are filed under `agent-ergonomics`, depending on this one.
- No code changes in this Issue; the owner accepts or rejects the ADR.

## Log

- 2026-10-22 Airic Lenz: Proposed ADR 0016 (adr-0016, with adr-0016-decision and adr-0016-rejected): a token file keyed by worktree and purpose under capcom/locks, resumed by naming the purpose again. Follow-ups lock-token-file and start-finish-token-file wait on this Issue; accepting the ADR closes it.
