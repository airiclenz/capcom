# Plan: the lock knows its holder, and the agent guide names search

**Goal:** A session's own writes under its Tree lock stay silent, while another session's writes
get a warning that says what to do. Touch, release and check work without a pasted token for the
holder, warnings print first, and the managed block teaches search and when `check` is needed.
**Date:** 2026-11-22
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** ad35f88
**Closes:** lock-warns-own-writes; lock-warning-ignored; lock-touch-kept-token; check-after-every-write

**Sources:**
- `.capcom/issues/session-friction-0160.md` and its children named on the Closes lines
- ADR 0005, 0016, 0017 (`capcom know show adr-0016 --with-children`, `adr-0017`)
- Notes `issue-workflow`, `cli-test-harness`, `coding-conventions`, `store-layout`

**Ratified design calls** (Airic, 2026-11-22):
- **Holder identity:** acquire and start record the holder process, the nearest non-shell ancestor of capcom (pid + start stamp), in the lock record; a caller whose process ancestry contains it is the holder. Kept token files are not identity.
- **Holder writes:** silent, any Issue or Note write, and each renews the heartbeat.
- **Touch, release, check without --token:** allowed for the holder process only, through the kept token file that matches the record; anyone else fails as a non-holder does today.
- **Warning order:** in text output every warning line prints before the result; `--json` and `--kv` unchanged.
- **Search ids:** `search` and `know search` also match the words of the id.
- **Docs shape:** the holder rule is an amendment Note under ADR 0017, the token-less touch/release one under ADR 0016.

**Standing requirements:**
- skills: coding-standards
- Run single packages with `GOMEMLIMIT=2GiB`; never `./...` with `-race`; never two test commands at once (AGENTS.md).
- CLI tests go through `runCLI` (Note `cli-test-harness`); never `t.Parallel()`.
- After changing a command's help text: `go run ./cmd/gendocs` and commit `docs/site/reference/cli/`; never hand-edit it.
- Knowledge Notes are written with `capcom know add` / `capcom know update`, then `capcom check`.
- Any deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- The other `session-friction-0160` children (list-by-ids, show-brief-section, know-show-*, apply-*, id-word-limit, know-add-body-flag).
- `capcom ui`'s lock display (ADR 0013) and the Lock overview.
- Telling apart sessions that share one harness process (two panes of one tmux server).

**Regression check (2026-11-22, ad35f88):**
- 2: guard folded (the record reference's `process` field lands in item 10)
- 3: guard folded; supersedes ADR 0017 "Why the purpose and not a token" for the holder process
- 4: guard folded; supersedes ADR 0017 "Reading the lock creates nothing" for a holder write
- 6: guard folded; supersedes the warning-after-result example in docs/site/concepts/issues.md:39
- 7: guard folded
- 8: guard folded; yields to the warn-like-check rule (internal/config/config.go:284-285, internal/cli/create.go:194-196)
- 9: guard folded
- 10: guard folded; amends ADR 0005 "session, not a process" (adr-0005-decision.md:6-9) in Note adr-0005-amend-1122

## 1. Read a process's ancestry and its harness anchor — ✅ DONE (2026-11-22)

NOTES (2026-11-22): go.mod unchanged: golang.org/x/sys v0.46.0 is already a direct require, as the item's Read first line says.
NOTES (2026-11-22): the "parent started after its child" stop is applied on every OS through one shared walk, not on Windows only; on Linux and macOS the parent PID is always the live parent, so it never fires there.
NOTES (2026-11-22): added internal/proc/proc_linux_test.go (not in Files) to pin the /proc stat parse for a comm holding spaces and parentheses, which only builds on Linux.

**What:**
**Goal:** a new package `internal/proc` exposes `Process{PID int; Started string; Name string}`,
`Ancestry() []Process` (the caller's ancestors, parent first) and `Anchor(chain []Process)
(Process, bool)` (the first ancestor that is not a shell or wrapper); `Ancestry` works on Linux,
macOS and Windows and returns nil elsewhere.
**Approach (assumed at the header base):** one deep module; callers never see OS details.
`Started` is an opaque per-OS start stamp: Linux `/proc/<pid>/stat` field 22 (parse after the last
`)`, the comm may hold spaces), macOS `unix.SysctlKinfoProc("kern.proc.pid", pid)` (`P_starttime`,
`Eproc.Ppid`, `P_comm`), Windows toolhelp snapshot (`ParentProcessID`, `ExeFile`) plus
`GetProcessTimes` creation time. The walk stops at pid ≤ 1, after 64 steps, on any read error, and
on Windows when a parent was created after its child (a reused pid). `Anchor` lowercases the base
name, strips `.exe`, and skips exactly: sh, bash, zsh, dash, ksh, mksh, ash, fish, tcsh, csh,
busybox, pwsh, powershell, cmd, env, timeout, nice, nohup, xargs, stdbuf, time, sudo, doas.
Per-OS files by build tag: `proc_linux.go`, `proc_darwin.go`, `proc_windows.go`, `proc_other.go`.
`golang.org/x/sys` becomes a direct dependency if it is not one.
**Files:** internal/proc/proc.go, internal/proc/proc_linux.go, internal/proc/proc_darwin.go, internal/proc/proc_windows.go, internal/proc/proc_other.go, internal/proc/proc_test.go, go.mod
**Read first:** golang.org/x/sys@v0.46.0/unix/ztypes_darwin_arm64.go — KinfoProc.Proc (ExternProc.P_starttime, P_comm), KinfoProc.Eproc.Ppid; golang.org/x/sys@v0.46.0/unix/syscall_darwin.go — SysctlKinfoProc;
golang.org/x/sys@v0.46.0/windows/types_windows.go — ProcessEntry32.ParentProcessID, ExeFile; golang.org/x/sys@v0.46.0/windows/zsyscall_windows.go — CreateToolhelp32Snapshot, GetProcessTimes;
go.mod — golang.org/x/sys v0.46.0 (already a direct require, so go.mod needs no change)
**Tests:** `Anchor` over fake chains: shells and wrappers skipped, first other name returned,
all-shell chain gives false, `BASH.EXE` skipped. `Ancestry` on the test process: first entry's PID
is `os.Getppid()` with a non-empty `Started`; two calls return equal chains.
**Acceptance:** `cd /home/airic/repos/capcom && go vet ./internal/proc/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/proc/ && GOOS=windows go vet ./internal/proc/ && GOOS=darwin go vet ./internal/proc/ && GOOS=freebsd go vet ./internal/proc/`
**Commit:** `feat(proc): read the process ancestry and its harness anchor`

## 2. The lock record names the holder process — ✅ DONE (2026-11-22)

NOTES (2026-11-22): the record stores the process as a lock-local `HolderProcess{PID, Started}` (`json:"pid"`, `json:"started"`) rather than `proc.Process`, so the on-disk shape holds only PID and start stamp, not the executable name; `Claim.Process` is `*proc.Process` as the approach says.
NOTES (2026-11-22): a resume with a claim that has no process clears the stored one ("none when the claim has none"); `HeldBy` does not consult IsHeld, matching the Goal's definition exactly.

**What:** Depends on item 1.
**Goal:** `lock.Record` carries an optional holder process (`json:"process,omitempty"`, PID and
start stamp); an acquire that takes or resumes the lock stores the claim's process (none when the
claim has none); `Status.HeldBy(chain []proc.Process) bool` is true only when the record has a
process and an entry of chain equals it on both PID and start stamp. Records without the field
(older capcom) read as before.
**Approach (assumed at the header base):** `Claim` gains `Process *proc.Process`; `Locks.Acquire`
writes it into the record on a new take, a resume, and `--take-stale`; `Touch` keeps the stored
one. `CurrentClaim` does not compute it: the CLI sets it (item 3). Unknown JSON fields stay
tolerated by `json.Unmarshal`.
**Regression guard.** `docs/site/reference/file-format.md` lists every lock record field, and item 10
owns that section: item 10 adds the **process** bullet (holder process PID and start stamp, left out
when unknown or written by an older capcom) and notes that an older capcom's touch or resume drops
it (its `Record` has no such field); this item leaves the file alone.
**Files:** internal/lock/lock.go, internal/lock/claim.go, internal/lock/lock_test.go
**Read first:** internal/lock/lock.go — Record, Claim, Status, Locks.Acquire, Locks.Touch, Locks.write; internal/lock/claim.go — CurrentClaim;
docs/site/reference/file-format.md — lock record field list
**Tests:** acquire with a process stores it; resume with a different process replaces it; touch
keeps it; `HeldBy` true for a chain containing it, false for a PID match with a different stamp,
false for a record without one; a record JSON without `process` parses.
**Acceptance:** `cd /home/airic/repos/capcom && go vet ./internal/lock/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/lock/`
**Commit:** `feat(lock): record the holder process in the lock record`

## 3. A write from under the holder process stays silent — ✅ DONE (2026-11-22)

NOTES (2026-11-22): cmd/agenteval/main_test.go is not in Files: its TestMain dispatches the seed helper too, since a workspace a test prepares in process re-executes the test binary itself (os.Executable), which would otherwise run the whole test suite as the seed's capcom.
NOTES (2026-11-22): the seed helper is a hidden first argument, `agenteval __seed-capcom <capcom> <args…>`, dispatched in main() after the shim check (no cobra command, so it stays out of help); the helper runs capcom as its child and exits with its code.
NOTES (2026-11-22): consequential edit — cmd/agenteval/doc.go: made necessary by the seed helper mode added to main.go and workspace.go (package doc and file map lines for main.go and workspace.go)
NOTES (2026-11-22): consequential edit — docs/agent-eval.md: made necessary by seeding no longer running the build directly but under the seed helper

**What:** Depends on item 2. Regression from ADR 0017's purpose-only rule: issue `lock-warns-own-writes`.
**Goal:** `capcom lock acquire` and `capcom start` record the anchor of the caller's ancestry; a
CLI write (Issue or Note, `apply` included) made by a caller whose ancestry contains the record's
process emits no lock warning; a write by any other caller still warns, and the purpose-id rule of
ADR 0017 still silences a write of the purpose Issue.
**Approach (assumed at the header base):** a package-level seam in `internal/cli/root.go`,
`var processAncestry = proc.Ancestry`, read once per `Run` into the `app`. Acquire and start put
`proc.Anchor(chain)` on the claim. `app.lockWarning` returns "" when `held.Status.HeldBy(chain)`.
CLI tests default the seam to an empty chain (no anchor), so every existing test keeps today's
behaviour; a test helper sets a fake chain for one test and restores it.
**Regression guard.**
- Default the seam with a package-level `func init() { processAncestry = func() []proc.Process { return nil } }`
  in `testharness_test.go` (untagged, no `TestMain`: `stress_main_test.go` owns it under the `stress` tag),
  so `runCLIWith` and `bench_test.go`'s direct `Run` calls get it too; the per-test helper sets the
  var and restores it in `t.Cleanup`.
- Read the chain lazily and at most once per `Run`: in acquire and start, and in `app.lockWarning`
  only after `Check` returned a `*lock.HeldError` whose record carries a process.
- Update every comment that states the purpose-only rule (`grep -rn "purpose alone\|another session's\|whose id is the lock's" internal/cli`);
  the same line in `docs/site/reference/file-format.md` (:917-919) is item 10's.
- The chain-A/B tests acquire with `lock acquire --purpose batch` (a purpose that names no Issue
  the writes target), so every write under chain A warns before this item.
- Supersedes ADR 0017's "Why the purpose and not a token" (adr-0017-decision.md:17-19) for a caller
  under the holder process; the purpose rule still silences the purpose Issue; item 10 records it.
- cmd/agenteval's seed must never record a holder process that an agent call descends from (today env.capcom in cmd/agenteval/workspace.go runs capcom as a direct child of agenteval, so agenteval would become the anchor and the resume-lost-token scenario in cmd/agenteval/scenario_work.go would stop testing token loss). The seed's capcom runs under a short-lived non-shell intermediate that exits when the seed command ends (agenteval re-executing itself in a hidden helper mode), so the recorded anchor is dead before the agent starts; no environment knob is added to capcom. Add the agenteval files to Files, a test that the seed's recorded process is not in the agent's ancestry or that the helper path is used, and `GOMEMLIMIT=2GiB go test -count=1 ./cmd/agenteval/` to Acceptance.
**Files:** internal/cli/root.go, internal/cli/lock.go, internal/cli/start.go, internal/cli/lock_warn_test.go, internal/cli/testharness_test.go, cmd/agenteval/workspace.go, cmd/agenteval/main.go, cmd/agenteval/workspace_unix_test.go
**Read first:** internal/cli/root.go — Run, app.lockWarning, app.writesOnly; internal/cli/lock.go — newLockAcquireCommand; internal/cli/start.go — app.start;
internal/cli/testharness_test.go — runCLI; internal/cli/write_test.go — runCLIWith; internal/cli/stress_main_test.go — TestMain
**Tests:** `lock_warn_test.go`: acquire with fake chain A, then `create`, `log`, `update`,
`close`, `apply`, `know add`, `know update` under chain A print no warning; the same writes under
chain B warn; acquire under A, resume under B, write under B is silent and under A warns. The
chain-A/B tests acquire with `lock acquire --purpose batch`. agenteval: the seed's recorded process
is not in the agent's ancestry, or the seed's capcom runs through the helper path.
**Acceptance:** `cd /home/airic/repos/capcom && go vet ./internal/cli/ ./cmd/agenteval/ && GOMEMLIMIT=2GiB go test -count=1 -run 'LockWarn|Lock|Start' ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./cmd/agenteval/`
**Commit:** `fix(cli): keep the lock holder's own writes silent`

## 4. The holder renews by writing, and touches, releases and checks without a token — ✅ DONE (2026-11-22)

NOTES (2026-11-22): `KeptToken` takes the Status a caller read (`KeptToken(current Status)`) rather than a bare key, so the holder check and the token lookup judge the same record and a lock taken over between them is never acted on with the new holder's token; the CLI's `presentedToken` reads the record once, checks `HeldBy`, then asks for the kept token of that record.
NOTES (2026-11-22): `Touch` and `TouchHolder` share a private `renew` helper in internal/lock/lock.go; Touch's behaviour is unchanged.
NOTES (2026-11-22): a holder write whose renewal finds the lock taken over by another session since it was read is judged against that session's lock (purpose rule, then the foreign-lock warning); a renewal that finds the lock freed is silent; any other renewal failure warns `<record>: the lock's heartbeat was not renewed: <why>`.
NOTES (2026-11-22): the `--token` hint (`capcom lock status; pass --token <token> if the lock is yours`) applies only when touch or release acted with no token (none given and none kept for the holder); a wrong `--token` keeps the plain `capcom lock status` hint.
NOTES (2026-11-22): bare `lock check` and `lock touch` by the holder print the kept token on their token line, as they do for a given `--token`.
NOTES (2026-11-22): docs/site/concepts/tree-locks.md and README.md still show `lock touch/release --token <token>` only; those lines stay true (`--token` still works) and the concepts page is item 10's.

**What:** Depends on item 3.
**Goal:** a write by the holder process renews the lock's heartbeat (a failed renewal is a
warning, never a failed write); `capcom lock touch`, `lock release` and `lock check` accept no
`--token` when the caller is the holder process and a kept token file holds the record's token;
any other caller without `--token` fails exit 3 with the held-lock message and a hint naming
`--token`. `--token` keeps working as today and wins when given.
**Approach (assumed at the header base):** `lock.Locks` gains `KeptToken(key)` — the cleartext of
the token file whose token digests to the current record — and a guarded `TouchHolder(key, chain)`
that re-checks `HeldBy` under the guard. `MarkFlagRequired(flagToken)` goes from touch and release.
Help texts of touch, release and check say a touch is needed only when work outlasts the TTL
without a capcom write.
**Regression guard.**
- The `--token` hint for a non-holder applies to touch and release only (`emitLock`'s
  `takeOverPurpose == ""` branch); a bare `lock check` by a non-holder keeps
  `heldLockHint(..., checkTakeOverPurpose)` unchanged, so
  `TestLockRefusal_HeldByAnother_HintNamesResumeAndTakeOver` stays green.
- Holder renewal runs in `app.lockWarning` before both silencing returns (the `writesOnly` purpose
  return and the `HeldBy` return); "a holder write moves the heartbeat" writes the purpose Issue after `start`.
- Supersedes ADR 0017's "Reading the lock creates nothing ... reads the record without the guard"
  (adr-0017-decision.md:32) for a holder write, which takes the guard to renew; item 10 records it.
- Help texts of lock touch, release and check change, so the item also runs `go run ./cmd/gendocs` and commits docs/site/reference/cli/ (add that directory to Files) and its Acceptance adds `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`.
**Files:** internal/lock/lock.go, internal/lock/token.go, internal/lock/worktree.go, internal/lock/token_test.go, internal/lock/lock_test.go, internal/cli/lock.go, internal/cli/root.go, internal/cli/lock_test.go, docs/site/reference/cli/
**Read first:** internal/cli/root.go — lockWarning; internal/lock/lock.go — Touch, withGuard, tokenMatches; internal/lock/token.go — SessionToken;
internal/cli/lock.go — emitLock, heldLockHint; internal/cli/lock_test.go — TestLockRefusal_HeldByAnother_HintNamesResumeAndTakeOver
**Tests:** holder chain: bare touch renews, bare release frees and deletes the token file, bare
check exits 0; other chain: each bare verb exits 3; a non-holder's bare check keeps the resume /
`--take-stale` hint; a holder write of the purpose Issue after `start` moves the heartbeat
(`TouchHolder` in `internal/lock/lock_test.go`); a write
under a free lock creates nothing under the lock directory (`TestLockWarn_FreshRepo_CreatesNoLockFile` still passes).
**Acceptance:** `cd /home/airic/repos/capcom && go vet ./internal/lock/ ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/lock/ && GOMEMLIMIT=2GiB go test -count=1 -run 'Lock' ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`
**Commit:** `feat(lock): let the holder renew, release and check without a token`

## 5. The warning to another session says what to do — ✅ DONE (2026-11-22)

NOTES (2026-11-22): the holder's prime line applies to a live or a stale lock alike, since the Goal names the holder process without a state; the prime stale lines no longer name --take-stale, as the Goal fixes their ending (the stale write warning still says "or taking it over").
NOTES (2026-11-22): primeResult, renderBriefing and addLocks take the caller's ancestry as a func (a.callerAncestry), read only when the own lock's record names a holder process, as lockWarning does.
NOTES (2026-11-22): the old wording still stands in AGENTS.md, internal/agents/block.go, docs/site/agents/managed-block.md (item 9) and docs/site/concepts/tree-locks.md, docs/site/agents/briefing.md, .capcom/knowledge/issue-workflow.md, ADR 0017 decision, plugins/capcom/skills/capcom/SKILL.md (item 10); left to those items.

**What:** Depends on item 3. Issue `lock-warning-ignored`.
**Goal:** the lock warning ends, for a live lock, `; another session holds this worktree: the write
landed, ask the user before writing more`, and for a stale one `; another session's stale lock
holds this worktree: the write landed, ask the user before writing more or taking it over`; no
warning or prime line says "carry on". `capcom prime` prints `this worktree: held by this session
(<purpose>)` for the holder process, and its other held and stale lines end `otherwise ask the user
before editing it`.
**Approach (assumed at the header base):** replace `foreignLockAdvice` in `internal/cli/root.go`
with the two advices chosen by `held.Status.IsStale`; prime's `worktreeLockHeld*`/`Stale*`
constants in `internal/cli/prime.go` and the code choosing them get the holder case.
**Files:** internal/cli/root.go, internal/cli/prime.go, internal/cli/lock_warn_test.go, internal/cli/prime_test.go
**Read first:** internal/cli/root.go — foreignLockAdvice, lockWarning; internal/cli/prime.go — worktreeLockAdvice, addLocks, renderBriefing; internal/lock/overview.go — Overview.Own;
internal/cli/lock_warn_test.go — TestLockWarn_StaleLock_SaysItIsStale; internal/cli/prime_test.go — TestPrime_FreshLock_AdvisesItIsHeld
**Tests:** live and stale warning text pinned in `lock_warn_test.go`; prime under the holder chain
and under another chain pinned in `prime_test.go`; `grep -rn "carry on" internal/cli` is empty.
**Acceptance:** `cd /home/airic/repos/capcom && go vet ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 -run 'LockWarn|Prime' ./internal/cli/ && ! grep -rn "carry on" internal/cli --include=*.go`
**Commit:** `fix(cli): tell another session what to do when it writes under a held lock`

## 6. Warnings print before the result in text output — ✅ DONE (2026-11-22)

NOTES (2026-11-22): lock_test.go and related_test.go also pinned a text warning after the result (TestLockStatus_CorruptOwnRecord_ReportsInvalidWithAWarning, TestRelatedAdd_KeylessConfig_RaisesTheFormatOnceWithAWarning, TestRelatedAdd_MissingConfig_CreatesItAtFormatTwo); updated per the item's "every internal/cli test" rule (Files list is a floor)
NOTES (2026-11-22): renamed TestTextRenderer_Render_WritesWarningsAfterResult to ...WritesWarningsBeforeResult and TestReindex_BrokenFile_WarnsAfterResultAndIndexesTheRest to ...WarnsBeforeResult..., whose names the change made false; added TestTextRenderer_Render_KeepsElsewhereAndFailureAfterResult to pin the Goal's Elsewhere/failure placement
NOTES (2026-11-22): consequential edit — docs/site/agents/managed-block.md: made necessary by warnings-first rendering (the capcom agents example showed the warning after the result)
NOTES (2026-11-22): consequential edit — docs/site/reference/file-format.md: made necessary by warnings-first rendering ("prints each warning: line after the command's result" became false)

**What:** Issue `warnings-print-last`.
**Goal:** a text-mode result writes every `warning: ` line before its result text; Elsewhere
lines and a partial result's failure keep their place after it; `--json` and `--kv` output is
byte-identical to before.
**Approach (assumed at the header base):** the text branch of the renderer in
`internal/cli/output/output.go` that loops over `result.Warnings` with `textWarnLabel` moves ahead
of the result text. Golden files under `internal/cli/testdata/` that hold warnings are regenerated
and each diff is checked to be a reorder only.
**Regression guard.**
- The warning order also shows in tests outside ./internal/cli/output and the -run subset; Acceptance runs the whole `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/` package and every other package whose tests pin a text warning line (grep for "warning: " in *_test.go outside internal/cli) one package at a time.
- Text output has no golden files (`internal/cli/testdata/` holds help, json, kv and stores only):
  the regenerate step is dropped; the line-1 `create` test goes in `internal/cli/lock_warn_test.go`.
- Supersedes the warning-after-result example in `docs/site/concepts/issues.md:39`: it shows the
  warning first.
**Files:** internal/cli/output/output.go, internal/cli/output/output_test.go, internal/cli/lock_warn_test.go, internal/cli/agents_test.go, internal/cli/next_test.go, internal/cli/fmt_test.go, internal/cli/ready_test.go, internal/cli/reindex_test.go, internal/cli/read_test.go, internal/cli/know_test.go, internal/cli/know_update_test.go, docs/site/concepts/issues.md
**Read first:** internal/cli/output/output.go — textRenderer.Render, Result; internal/cli/root.go — emit; internal/cli/output/output_test.go — TestTextRenderer_Render_WritesWarningsAfterResult;
internal/cli/know_test.go — know tree "warning follows the outline" cases; internal/cli/read_test.go — list --tree warning case;
internal/cli/ready_test.go — TestReady_Text_ReadyLineEndsWithWhatItUnblocks; internal/cli/lock_warn_test.go — lockWarning
**Closes:** warnings-print-last
**Tests:** renderer test: warnings then text; a CLI test in `lock_warn_test.go` that `create` under
a foreign lock prints the warning on line 1; every internal/cli test that pins a text `warning: `
line after the result is updated to warnings-first.
**Acceptance:** `cd /home/airic/repos/capcom && go vet ./internal/cli/... && GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/output/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/index/`
**Commit:** `fix(output): print warnings before the result in text output`

## 7. search and know search match the words of the id — ✅ DONE (2026-11-22)

NOTES (2026-11-22): widened rather than limited: `capcom context`'s searched Notes and the `capcom ui` filter now match id words too (id-only hits rank last, so context's ref and title/body hits keep their places); every line describing search as title/body only now says id, title or body.
NOTES (2026-11-22): CLI tests went to internal/cli/read_test.go and internal/cli/know_test.go (where the search tests live, per the item's Read first), not internal/cli/search_test.go.
NOTES (2026-11-22): consequential edit — docs/site/reference/json-output.md: made necessary by the id column; an empty excerpt now also follows a hit on the id alone ("as when only its id or title matched").

**What:**
**Goal:** `capcom search token` lists an Issue whose id is `lock-touch-kept-token` even when its
title and body lack the word; `capcom know search` does the same for Note ids; both `--help` texts
say "id, title or body".
**Approach (assumed at the header base):** the `fts` table in `internal/index/schema.go` gains an
`id` column (hyphens split words under the default tokenizer); both `INSERT INTO fts` sites
(`internal/index/refresh.go`, `internal/index/notes.go`) fill it; `schemaVersion` is bumped so a
stale index rebuilds. Excerpts (`--excerpt`) stay body lines.
**Regression guard.**
- Keep today's ranking: give the `id` column bm25 weight 0 after the CREATE in `schemaStatements`
  (columns `path, kind, id, title, body`: `INSERT INTO fts (fts, rank) VALUES ('rank', 'bm25(0.0, 0.0, 0.0, 1.0, 1.0)')`),
  so id-only hits are found and ranked last and the `search` / `search-any` goldens of
  `TestGolden_ReadKinds` stay as they are; Acceptance runs `Golden` too.
- Name the widening (`capcom context`'s Notes that apply, the UI filter) and fix every line that
  describes search as title/body only (`grep -rn "title or body\|title and body" internal docs/site`, minus
  docs/site/reference/cli), incl. docs/site/concepts/knowledge.md:63, terminal-ui.md:221,
  internal/ui/filter.go:23,54 and internal/cli/doc.go:32,55; or limit context and the UI filter to an FTS5 `{title body}:` column filter.
- search and know search help texts change: same gendocs step, docs/site/reference/cli/ in Files, and `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/` in Acceptance.
**Files:** internal/index/schema.go, internal/index/refresh.go, internal/index/notes.go, internal/index/index.go, internal/index/query.go, internal/index/index_test.go, internal/cli/search.go, internal/cli/know_search.go, internal/cli/search_test.go, internal/cli/doc.go, internal/ui/filter.go, docs/site/concepts/knowledge.md, docs/site/getting-started/terminal-ui.md, docs/site/reference/cli/
**Read first:** internal/index/schema.go — schemaStatements, schemaVersion; internal/index/refresh.go — insertIssue; internal/index/notes.go — insertNote; internal/index/index.go — matchExpression;
internal/cli/search.go — searchLong; internal/cli/read_test.go — TestSearch_Words_RankTheBestMatchFirst (CLI search tests live here and in know_test.go, not search_test.go);
internal/cli/golden_read_test.go — TestGolden_ReadKinds
**Tests:** index: a word only in an id finds the Issue and the Note, ranked after title and body
hits; CLI: `search` and `know search` on such a word, text and `--json`; the search goldens unchanged.
**Acceptance:** `cd /home/airic/repos/capcom && go vet ./internal/index/ ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/index/ && GOMEMLIMIT=2GiB go test -count=1 -run 'Search|Golden' ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`
**Commit:** `feat(search): match the words of the id`

## 8. check says when it is needed, and writes that can fail it say so

**What:** Part of issue `check-after-every-write`.
**Goal:** `capcom check --help` says a write made through a capcom command is already validated
and check is for hand edits, merges and before committing hand-edited store files; every CLI write
that can leave the store with a `capcom check` finding warns in its own output and names
`capcom check`.
**Approach (assumed at the header base):** go through each `Kind` in
`internal/diagnose/diagnose.go`; for each one a CLI write can produce (known: an unresolved
`--ref`, warned today), confirm a warning exists or add one in the writing command. Record the
Kind-by-Kind result as a dated NOTES line under this item.
**Regression guard.**
- The Goal's "names `capcom check`" reads "warns with the message `capcom check` reports for it":
  the item yields to today's rule (internal/config/config.go:284-285, internal/cli/create.go:194-196),
  so the byte-pinned missing-plan and ref warnings (write_test.go, know_test.go, know_update_test.go) stay.
- A new warning can break its command's tests and JSON goldens (`"warnings":[]`): Acceptance runs
  the whole `./internal/cli/` package, and `-run Golden -update` only where a warning is intended.
- check help text changes: same gendocs step, docs/site/reference/cli/ in Files, and `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/` in Acceptance.
**Files:** internal/cli/check.go, internal/cli/check_test.go, docs/site/reference/cli/, plus the writing commands the review finds
**Read first:** internal/cli/check.go — checkLong; internal/diagnose/diagnose.go — Kind constants; internal/check/issues.go — MissingPlan, MissingDep, Standing, Cycle adds;
internal/cli/create.go — missingPlanWarnings; internal/cli/know_add.go — refWarnings; internal/cli/errors.go — unwarnedTreeProblems;
internal/cli/write_test.go — TestCreateAndUpdate_MissingPlan_WritesAndWarnsLikeCheck; internal/cli/know_test.go — TestKnowAdd_MissingAndIgnoredRefs_WarnAndWrite
**Tests:** `check --help` text; one test per write warning added.
**Acceptance:** `cd /home/airic/repos/capcom && go vet ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`
**Commit:** `docs(cli): say when capcom check is needed`

## 9. The managed block names search and the new lock rules

**What:** Depends on items 4, 5, 7, 8.
**Goal:** the block from `capcom agents` lists, in its one-line style,
"`capcom search <word>... --excerpt`: find issues by words in the id, title or body" and the
`capcom know search` twin; its check sentence says a capcom write is already validated and check
is for hand edits and merges; its lock paragraph teaches bare `capcom lock touch` (only when work
outlasts the TTL without a capcom write) and bare `capcom lock release`, says the holder's own
writes are silent, and that a warning means another session holds the worktree: ask the user. No
"carry on". This repo's AGENTS.md block and `docs/site/agents/managed-block.md` match.
**Approach (assumed at the header base):** edit the command list, `blockGuidance` and
`lockClosing` in `internal/agents/block.go`; update `goldenBlock` and any size pin in
`block_test.go`; refresh AGENTS.md with `go run ./cmd/capcom agents`.
**Regression guard.**
- `TestCheck_AgentsBlockWithoutStoreMap_IsStaleUntilAgentsRefreshesIt` (internal/cli/agents_test.go)
  pins the old check sentence: update its `storeMapParagraph`, and Acceptance runs `-run 'Agents' ./internal/cli/`.
- Every prose description of the block's content (`grep -rn "after a hand edit\|another purpose\|purpose other than" docs/site`)
  matches the new block, not only the copied block: managed-block.md's intro and closing prose included.
- 9 and 10: both edit docs/site/reference/file-format.md — item 9 owns its Managed block section only, item 10 owns its lock record section only (the new `process` field); list the file in both items' Files.
**Files:** internal/agents/block.go, internal/agents/block_test.go, internal/cli/agents_test.go, AGENTS.md, docs/site/agents/managed-block.md, docs/site/reference/file-format.md
**Read first:** internal/agents/block.go — blockGuidance, lockClosing, render; internal/agents/block_test.go — goldenBlock, keepTheTokenBlock;
internal/cli/agents_test.go — TestCheck_AgentsBlockWithoutStoreMap_IsStaleUntilAgentsRefreshesIt; docs/site/agents/managed-block.md — block copy and closing prose;
docs/site/reference/file-format.md — Managed block section
**Closes:** agents-block-omits-search
**Tests:** `block_test.go` golden; size pin kept; `agents_test.go` store-map paragraph.
**Acceptance:** `cd /home/airic/repos/capcom && go vet ./internal/agents/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/agents/ && GOMEMLIMIT=2GiB go test -count=1 -run 'Agents' ./internal/cli/ && grep -n "capcom search" AGENTS.md && grep -n "capcom know search" AGENTS.md && ! grep -n "carry on" AGENTS.md && go run ./cmd/capcom check`
**Commit:** `docs(agents): name search and the holder lock rules in the managed block`

## 10. Record the holder rule in the decisions, glossary and docs

**What:** Depends on items 4, 5, 6. Owns every knowledge and site-doc amendment of this plan
except the managed block (item 9).
**Goal:** Note `adr-0017-amend-1122` under `adr-0017` states the holder-process rule, heartbeat
renewal by holder writes and the new warning words; `adr-0016-amend-1122` under `adr-0016` states
token-less touch/release/check for the holder; a glossary Note `holder-process` under
`glossary-coordination` defines the term; Note `issue-workflow` and `docs/site/concepts/tree-locks.md`,
`docs/site/agents/briefing.md` say the same. Every Note or doc line that says a write warns unless
it writes the purpose Issue, or says "carry on", is updated (`grep -rn "carry on\|purpose other than" .capcom/knowledge docs/site`).
**Regression guard.**
- The guard grep widens to `grep -rni "carry on\|purpose other than\|another purpose\|unless it writes" .capcom/knowledge docs/site plugins README.md`,
  minus docs/site/reference/cli: file-format.md:917-919, apply-file.md:80 and the shipped
  plugins/capcom/skills/capcom/SKILL.md Tree lock section (`--token` touch/release, "carry on") are updated.
- Decision records keep their history (adr-0017-decision.md:20-22 quotes the old warning): the
  grep and Acceptance exclude them with `--exclude-dir=decisions`; amendments go in the new Notes.
- ADR 0005 says the lock is held by a session, not a process; item 10 also writes Note `adr-0005-amend-1122` under `adr-0005` saying the record now names the holder's harness process to recognise the holder's own writes, while the token still decides acquire/resume/touch/release; add it to Files and Acceptance (`go run ./cmd/capcom know show adr-0005-amend-1122`).
- 9 and 10: both edit docs/site/reference/file-format.md — item 9 owns its Managed block section only, item 10 owns its lock record section only (the new `process` field); list the file in both items' Files.
  In that section: the **process** bullet (PID and start stamp, left out when unknown or written by
  an older capcom, dropped by an older capcom's touch or resume), the heartbeat bullet and the warn rule.
**Files:** .capcom/knowledge/decisions/adr-0017/adr-0017-amend-1122.md, .capcom/knowledge/decisions/adr-0016/adr-0016-amend-1122.md, .capcom/knowledge/decisions/adr-0005/adr-0005-amend-1122.md, .capcom/knowledge/glossary/glossary-coordination/holder-process.md, .capcom/knowledge/issue-workflow.md, docs/site/concepts/tree-locks.md, docs/site/agents/briefing.md, docs/site/reference/file-format.md, docs/site/reference/apply-file.md, plugins/capcom/skills/capcom/SKILL.md
**Read first:** .capcom/knowledge/decisions/adr-0017/adr-0017-decision.md — "When it warns", "Why the purpose"; .capcom/knowledge/decisions/adr-0016/adr-0016-amend-1026.md — amendment shape;
.capcom/knowledge/decisions/adr-0005/adr-0005-decision.md; .capcom/knowledge/issue-workflow.md — lock paragraph; docs/site/concepts/tree-locks.md — warning example;
docs/site/reference/file-format.md — lock record; plugins/capcom/skills/capcom/SKILL.md — Tree lock section
**Tests:** none (docs).
**Acceptance:** `cd /home/airic/repos/capcom && go run ./cmd/capcom know show adr-0017-amend-1122 && go run ./cmd/capcom know show adr-0016-amend-1122 && go run ./cmd/capcom know show adr-0005-amend-1122 && go run ./cmd/capcom know show holder-process && ! grep -rni "carry on\|purpose other than\|another purpose\|unless it writes" .capcom/knowledge docs/site plugins README.md --exclude-dir=decisions --exclude-dir=cli && go run ./cmd/capcom check`
**Commit:** `docs(lock): record the holder-process rule`
