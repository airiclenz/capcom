# Tree lock visibility plan

**Goal:** Agents learn about the Tree lock from what they always read. The Managed block
recommends taking it before an extensive change, and the Briefing tells an agent what to do
about this worktree's lock.
**Date:** 2026-10-14
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** ffef2a6
**Closes:** lock-visibility

**Sources:**
- `.capcom/issues/lock-visibility.md`
- `.capcom/knowledge/decisions/adr-0005.md` (and its children `adr-0005-decision`, `adr-0005-rejected`)
- `.capcom/knowledge/glossary/glossary-coordination/` (Tree lock, Lock token, Stale)

**Ratified design calls** (user, 2026-10-14):
- **Block wording:** recommend the lock for a change spanning several files or commits; never a requirement.
- **Coupling:** the lock is not tied to claiming or closing an Issue; `block-work-loop` no longer names it.
- **Briefing:** keep the locks list; add one `this worktree:` line whose advice depends on its state; text only, `--json` data unchanged.
- **Enforcement:** none; no hook (ADR 0005).

**Standing requirements:**
- skills: coding-standards
- Raspberry Pi host: run single packages with `GOMEMLIMIT=2GiB`; never `-race` on `./...`; never two test commands at once.
- Refresh this repository's AGENTS.md with the built binary (`go run ./cmd/capcom agents`), never the installed one.

**Out of scope:**
- Taking or releasing the lock from `capcom update`, `close` or any Issue command.
- Enforcing hooks, a `lock check` hook mode, changes to `capcom init`.
- New `--json` fields on `capcom prime`.
- The work loop text of `block-work-loop`.

**Regression check (2026-10-14, ffef2a6):**
- 1: guard folded (TangledMarkers line numbers; `lock check --token` wording; every doc enumerating the block's contents)
- 2: guard folded (FixtureStore want string; no free line over a corrupt record of this worktree)

## 1. Managed block recommends the Tree lock

**What:**
**Goal:** The block that `capcom agents` renders has a Tree lock paragraph that recommends
`capcom lock acquire --purpose <text>` before a multi-file or multi-commit change. It explains
the token, says what to do when another session holds the lock, and this repository's
AGENTS.md and the docs that quote the block match it.
**Approach (assumed at the header base):** In `internal/agents/block.go`, `blockIntro`
gets a new paragraph after the "Where things are" paragraph and before the Knowledge list. The
binding text, wrapped at 100 columns like the rest of the block:

> Tree lock: before a change that spans several files or commits, it is recommended to claim this
> worktree with `capcom lock acquire --purpose <text>` and keep the token it prints. `capcom lock
> touch --token <token>` renews it during long work, `capcom lock release --token <token>` frees
> it when done, and acquiring again with `--token <token>` resumes it after a restart or
> compaction. When `capcom lock check` exits 3, another session holds this worktree: ask the user
> before editing it.

Update `goldenBlock` in `internal/agents/block_test.go`. Refresh AGENTS.md with
`go run ./cmd/capcom agents`. The docs that quote the block or list what it holds must match:
the example in `docs/site/agents/managed-block.md`, and the block description under "The Managed
block in AGENTS.md" in `docs/site/reference/file-format.md`.

**Regression guard.** The binding text's last sentence becomes "When `capcom lock check --token <token>` exits 3, or `capcom lock check` does while you hold no token, another session holds this worktree: ask the user before editing it." Every sentence that enumerates the block's contents must match, not just the two named: `grep -rn 'core commands\|map of the store\|what it holds' docs/site .capcom/knowledge README.md`.

**Files:** internal/agents/block.go; internal/agents/block_test.go; AGENTS.md; docs/site/agents/managed-block.md; docs/site/reference/file-format.md; .capcom/knowledge/glossary/glossary-agent-interface/managed-block.md
**Read first:** internal/agents/block.go — blockIntro, render; internal/agents/block_test.go — goldenBlock, TestRender_Notes_MatchesGolden, TestInspect_TangledMarkers_FailAtTheMarkerLine;
internal/cli/lock.go — lockCheckLong; docs/site/agents/managed-block.md — intro, example block

**Tests:**
- `TestRender_Notes_MatchesGolden` with the updated `goldenBlock`.
- `TestInspect_TangledMarkers_FailAtTheMarkerLine`: "second begin"/"second end" move with the block (38/37 for a 6-line paragraph plus its blank line), or derive them from `strings.Count(goldenBlock, "\n")`.
- A new test: `render(nil)` contains `capcom lock acquire --purpose <text>`, `--token <token>` and `capcom lock check`.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/agents/`
- `go run ./cmd/capcom check` exits 0 (the block is current)
- `grep -n "capcom lock acquire --purpose <text>" AGENTS.md docs/site/agents/managed-block.md`

**Commit:** `feat(agents): Managed block recommends the Tree lock before extensive changes`

## 2. Briefing advises on this worktree's Tree lock

**What:**
**Goal:** The text output of `capcom prime` has a `this worktree:` line right after the locks
list when the locks were listed. Its advice matches the state of the current worktree's lock
(free, held or stale). The line is absent when the locks section failed, and the `--json` data
is unchanged.
**Approach (assumed at the header base):** `briefing.Briefing` in
`internal/briefing/briefing.go` gets a field for this worktree's lock key, which `readLocks` sets
from `lock.ForWorktree(s).Key()`. In `internal/cli/prime.go`, `renderBriefing` passes it to
`addLocks`. `addLocks` finds that key in the held list and writes exactly one line of the
following, each on a single line. The literals are binding:
- not in the list: `this worktree: free; before a multi-file change run capcom lock acquire --purpose <text> and keep the token`
- held, not stale: `this worktree: held; if the token is yours resume with capcom lock acquire --purpose <text> --token <token>, otherwise do not edit it`
- stale: `this worktree: stale; resume with your --token, otherwise ask the user before taking it over with --take-stale`

`primeData` and `lockData` are unchanged. Update the documented output and the `locks` bullet in
`docs/site/agents/briefing.md`, the `capcom prime` output in
`docs/site/getting-started/quickstart.md`, and the Briefing excerpt in
`docs/site/concepts/tree-locks.md`.

**Regression guard.** `Locks.List` skips a corrupt record with a warning, yet `acquire` fails on it, so "not in the list" is not "free": when a `SectionLocks` Failure names this worktree's record (`lock.ForWorktree(s).RecordPath()`), write no `this worktree:` line. `TestPrime_FixtureStore_PrintsTheBriefing` compares the whole stdout, so it gains the free line between `locks: 0` and `broken note refs: 1`.

**Files:** internal/briefing/briefing.go; internal/briefing/briefing_test.go; internal/cli/prime.go; internal/cli/prime_test.go; docs/site/agents/briefing.md; docs/site/getting-started/quickstart.md; docs/site/concepts/tree-locks.md
**Read first:** internal/cli/prime.go — renderBriefing, addLocks; internal/briefing/briefing.go — Briefing, readLocks; internal/lock/worktree.go — Worktree.Key, Worktree.RecordPath;
internal/lock/lock.go — Locks.List; internal/cli/prime_test.go — TestPrime_FixtureStore_PrintsTheBriefing

**Tests:** in `internal/cli/prime_test.go`, through `runCLI`:
- No lock: `TestPrime_FixtureStore_PrintsTheBriefing`, its want string with the free line after `locks: 0\n`.
- A garbage `_main.json` record: no `this worktree:` line.
- Fresh lock on `_main`: the held line.
- The existing stale `_main` test: the stale line.
- A lock held in `_main` while prime runs in a linked worktree (`git worktree add`): the free line.
- The existing JSON key test still passes, with no new key.

In `internal/briefing/briefing_test.go`: `Compose` sets the key to `_main` in the main worktree.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/briefing/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Prime'`
- `go run ./cmd/capcom prime | grep -n '^this worktree: '`

**Commit:** `feat(prime): Briefing advises on this worktree's Tree lock`
