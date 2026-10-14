---
title: Lock overview answers this worktree's Tree lock state
status: done
type: task
priority: 2
plan: 2026-10-14 - 02 - architecture-review-2-plan
created: 2026-10-14T16:53:34Z
closed: 2026-10-14T19:03:49Z
reason: resolved by plan 2026-10-14 - 02 - architecture-review-2-plan (7123d17f)
---

No module answers "is this worktree's Tree lock free?". The Briefing (`internal/briefing/briefing.go`
`readLocks`) finds an invalid record of its own worktree by matching error text
(`strings.HasPrefix(warning.Error(), worktree.RecordPath()+":")`), encodes "invalid" as an empty
`WorktreeKey`, and opens two lock handles. cli maps lock states twice more: `foundState`
(`internal/cli/lock.go`) and `worktreeLockAdvice` (`internal/cli/prime.go`). And one unreadable
foreign record makes `lock.List` fail as a whole (`internal/lock/lock.go`), which hides every lock
and this worktree's advice in the Briefing.

Wanted (ratified 2026-10-14): `lock` returns a Lock overview: every lock, this worktree's state
as a named value (`free · held · stale · invalid · unreadable`), and typed per-record problems
(key, path, invalid or unreadable). A bad foreign record becomes one problem, never a failed
overview; only an unlistable locks directory fails it. The Briefing, `prime` advice and
`capcom lock status` read it. `lock status` uses the same state names in text, `--kv` and JSON,
and on an invalid or unreadable own record reports that state, warns naming the file, and exits
0. `lock check` exit codes do not change. Lock semantics (ADR 0005) do not change. A glossary
Note "Lock overview" under `glossary-coordination` names it.

## Acceptance

- `lock` exports the overview; tests in `lock` with a temp dir cover each of the five states
  and a foreign invalid and unreadable record next to a held lock.
- No caller parses lock error text; `WorktreeKey == ""` no longer encodes a state; cli has one
  state-to-name mapping.
- With one unreadable foreign record, `capcom prime` still lists the other locks and this
  worktree's advice, and warns about the bad record.
- `capcom lock status` on a corrupt own record prints `invalid`, warns naming the file and exits 0.
- The glossary Note "Lock overview" exists, the CHANGELOG `Unreleased` section has a line for the
  `lock status` change, and `capcom check` is clean.
- `go vet ./...` is clean and the touched packages' tests pass.

Source: docs/reviews/architecture-review-2026-10-14.html, 5 · The Tree lock state of a worktree, answered by lock.
