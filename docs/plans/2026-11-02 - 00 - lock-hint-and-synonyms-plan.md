# Lock refusal hints and command synonyms

**Goal:** A refusal on a lock another session holds names the command that resumes it or takes it
over, and a guessed command name from another CLI is answered with the capcom command it means.

**Date:** 2026-11-02
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 35e87ee

**Regression check (2026-11-02, 35e87ee):**
- 1 — guard folded; yields to ADR 0005
- 2 — guard folded (writer decision applied)

**Sources:**
- `.capcom/issues/agent-ergonomics/stale-lock-hint.md`
- `.capcom/issues/agent-ergonomics/command-synonyms.md`
- `.capcom/knowledge/decisions/adr-0016/` (purpose-keyed token file; resume by purpose)
- `.capcom/knowledge/decisions/adr-0005/` (stale lock, take-over)

**Ratified design calls** (Airic, 2026-11-02):
- **Hint scope:** `lock acquire`, `lock check` and `start` get the new hint; `lock touch` and `lock release` keep `capcom lock status`.
- **Start on a stale lock:** hint only, no new `start --take-stale` flag.
- **Hint text:** stale names resume and take-over; live names resume "if the lock is yours; otherwise ask the user".
- **Synonym scope:** top level and the `know` group; a synonym match beats the edit-distance pick.

**Standing requirements:**
- skills: coding-standards
- CLI tests run through `runCLI`, never parallel (Note `cli-test-harness`).
- Tests on this host: `GOMEMLIMIT=2GiB`, one package at a time, no `-race` on `./...`.

**Out of scope:**
- A `--take-stale` flag on `start`; any change to lock semantics, the token file or exit codes.
- Synonyms that run a command (they only feed the suggestion); synonyms for groups other than root and `know`.
- `lock touch`, `lock release` and `finish` lock notes.

## 1. Lock refusals name the resume and take-over commands

**What:**
**Goal:** `capcom lock acquire`, `capcom lock check` and `capcom start <id>` refused by a lock held
under another purpose (exit 3, code `locked`) carry a hint built from the held purpose, in text,
`--json` and `--kv` alike. Purposes are quoted with Go `%q`. With held purpose `foo`:
- `lock acquire --purpose bar`, stale: `capcom lock acquire --purpose "foo" to resume it, or capcom lock acquire --purpose "bar" --take-stale to take it over`
- `lock acquire --purpose bar`, live: `capcom lock acquire --purpose "foo" if the lock is yours; otherwise ask the user`
- `lock check`: the same two texts, with `<purpose>` in place of `"bar"` in the take-over half.
- `start x`, stale: `capcom start x --token <token> if the lock is yours, or capcom lock acquire --purpose "x" --take-stale, then capcom start x, to take it over`
- `start x`, live: `capcom start x --token <token> if the lock is yours; otherwise ask the user`
The resume half `capcom lock acquire --purpose "foo"` appears only when a kept token file for
purpose `foo` exists in this worktree; without one, the stale acquire/check hint is only the
take-over half and the live one is `capcom lock status; ask the user before taking it over`.
`lock touch` and `lock release` keep `capcom lock status`. Closes the defect seqcom hit in four runs.

**Regression guard.** The helper treats any kept-token read error (`Locks.SessionToken` fails on an unreadable 0600 token file, `internal/lock/token.go:41-43`) as "no kept token": the refusal stays code `locked`, exit 3, never a plain exit-1 failure.
Fix every refusal example in `docs/site` (`grep -rn 'hint: capcom lock status\|<token>, or capcom lock status' docs/site`; at base `docs/site/concepts/tree-locks.md:62`) to the new hint and add one stale example there.
The item changes hint text only and yields to ADR 0005 (`.capcom/knowledge/decisions/adr-0005/adr-0005-decision.md:14`): a stale lock is taken over only via `--take-stale`.

**Approach (assumed at the header base):** one helper in `internal/cli/lock.go` builds the hint
from the `*lock.HeldError` (its `Status.IsStale`, `Status.Record.Purpose`), the caller's own purpose
(or none) and whether the worktree keeps a token for the held purpose (`lock.Worktree.SessionToken`
or the lookup it uses). `emitLock` takes the hint from its caller instead of `lockStatusHint` for
acquire and check only; `startLockFailure` in `internal/cli/start.go` uses the same helper for its
start texts. No new package; `output.defaultHints` stays as is.

**Files:** internal/cli/lock.go; internal/cli/start.go; internal/cli/lock_test.go; internal/cli/start_test.go; internal/cli/finish_test.go; docs/site/concepts/tree-locks.md
**Read first:** internal/cli/lock.go — emitLock, lockStatusHint, failedLockResult; internal/cli/start.go — startLockFailure;
internal/lock/token.go — Locks.SessionToken; internal/lock/lock.go — HeldError;
internal/cli/lock_test.go — TestLockAcquire_HeldByAnother_HintsLockStatus, makeLocksStale
**Closes:** stale-lock-hint

**Tests:** in `lock_test.go`, replace `TestLockAcquire_HeldByAnother_HintsLockStatus` with tests
asserting the exact hint strings above for acquire stale/live, check stale/live, and the no-kept-token
variant; keep `TestLockRelease_HeldByAnother_HintsLockStatus` and the touch assertion. In
`start_test.go`, the start stale and live texts. Update `finish_test.go` wherever it pins the old
start hint. One `--json` test asserts `error.hint` carries the stale text. One `--kv` assertion in
`lock_test.go`: `HINT=<stale acquire text>` directly before `STATUS=locked`. One test makes the kept
token file unreadable (skipped on Windows and as root) and asserts exit 3, code `locked` and the
no-kept-token text.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Lock|Start|Finish'`
- `go vet ./internal/cli/`

**Commit:** `fix(lock): name the resume and take-over commands when a held lock refuses`

## 2. A guessed command name suggests the command it means

**What:**
**Goal:** an unknown command word that is a known synonym is answered with that command as the
suggestion, before any edit-distance match; other unknown words get the suggestion they get at the
base. Synonyms only feed the suggestion: the command still fails, exit 2.
- top level: `get`, `view`, `read`, `cat` → `show`; `ls` → `list`; `new`, `add` → `create`; `done` → `close`
- `know`: `get`, `view`, `read`, `cat` → `know show`; `ls`, `list` → `know tree`; `new`, `create` → `know add`
So `capcom get foo` prints `capcom: unknown command "get" for "capcom"; did you mean show?`.

**Regression guard.** `capcom read` maps to `show` by design, replacing today's edit-distance suggestion `ready` (ratified synonym list, Airic, 2026-11-02); the item's Tests assert `capcom read x` suggests `show`.
The `suggestion` paragraph of `docs/site/reference/json-output.md` (line 143 at base) names the synonyms beside the closest match, and `unknownCommandError`'s doc comment (`internal/cli/root.go:234-236`) is rewritten to match.

**Approach (assumed at the header base):** extends `internal/cli/suggest.go`, which owns
suggestion knowledge: one table keyed by group command path mapping a synonym to a subcommand name.
`unknownCommandError` in `internal/cli/root.go` consults it first and falls back to
`group.SuggestionsFor` plus `closestName`. No cobra `SuggestFor` fields on the commands.

**Files:** internal/cli/suggest.go; internal/cli/root.go; internal/cli/cli_test.go; docs/site/reference/json-output.md
**Read first:** internal/cli/root.go — unknownCommandError, closestName, withSubcommandDispatch; internal/cli/suggest.go — isNearName;
internal/cli/know.go — newKnowCommand; internal/cli/cli_test.go — unknownCommandCases, TestRun_UnknownCommandWithJSON_SuggestsClosest;
docs/site/reference/json-output.md — the suggestion paragraph
**Closes:** command-synonyms

**Tests:** in `cli_test.go`, table-driven: every synonym above asserts the exact stderr line and exit
2; `capcom read x` asserts `did you mean show?` (not the edit-distance `ready`); `capcom know get x` asserts `did you mean know show?`; one non-synonym typo (`shwo`) still suggests
`show`; `--json` of `capcom get` carries `"suggestion": "show"`.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Unknown|Suggest'`
- `go vet ./internal/cli/`

**Commit:** `feat(cli): suggest the meant command for a guessed synonym`
