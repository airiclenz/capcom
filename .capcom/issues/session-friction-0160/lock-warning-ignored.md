---
title: An agent that writes under another session's lock reads the warning as permission
status: done
type: bug
priority: 2
labels: [agent-ergonomics]
deps: [lock-warns-own-writes]
plan: 2026-11-22 - 00 - lock-holder-and-agent-guide-plan
created: 2026-11-22T17:47:25Z
closed: 2026-11-22T19:29:03Z
reason: "resolved by plan \"2026-11-22 - 00 - lock-holder-and-agent-guide-plan\" (bf30b14): items 5, 9 and 10 give the warning and prime a next step and drop 'carry on'"
---

The warning of a write made under a held Tree lock ends "; if it is yours, carry on"
(`foreignLockAdvice`, `internal/cli/root.go`). It does not say what to do when the lock is
not yours.

Seen: apogee session `20261109T142749Z-a42d54b1` (a small model) ran `capcom create` and
`capcom update` in a worktree while a Claude session held its lock for a plan; session
`20261110T103116Z-01dc2e79` did the same under another plan's lock. Both got the warning and
carried on without a word to the user.

Why: this is the one case the warning exists for, and its wording invites the wrong
reading. It waits for `lock-warns-own-writes`: while the holder is warned too, a firmer
wording would be false most of the time.

Wanted: once the holder's own writes are silent, the warning says plainly that another
session holds this worktree and what to do, for example "another session holds this
worktree (<holder>, <purpose>, heartbeat <age> ago): the write landed; ask the user before
writing more". The managed block in `internal/agents/block.go` and `capcom prime` use the
same words.

## Acceptance

- The warning names the next step for a session that does not hold the lock, and no longer
  ends in "carry on".
- The capcom block of AGENTS.md, `prime` and the Note `issue-workflow` agree with it.
- Golden or text tests cover the live and the stale wording.
