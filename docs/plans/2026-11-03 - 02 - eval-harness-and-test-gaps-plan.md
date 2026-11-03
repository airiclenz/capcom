# Eval harness follow-ups and audit test gaps

**Goal:** Close the three eval-harness gaps left by plan 01 (cancel wording, the sweep missing the agent loop's shell groups, untested signal wiring) and the ten audit test gaps under `audit-2026-11-03`. One of them also fixes a stale-Catalog duplicate dep in `ops.AddDep`.
**Date:** 2026-11-03
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 78e6603
**Closes:** agenteval-timeout-orphans-children

**Regression check (2026-11-03, 78e6603):**
- 1: guard folded
- 2: guard folded
- 3: guard folded (writer's decision plus the reviewer's four G: lines: basename dispatch, --capcom capcom-real, SIGINT resend, cleanup with TMPDIR)
- 5: guard folded
- 6: guard folded
- 10: guard folded
- 11: guard folded
- 13: guard folded
- header: **Closes:** agenteval-timeout-orphans-children kept in the header by the writer's decision — the plan-wide close of plan 01's leftover issue, whose remaining gaps are items 1-3; not moved to an item

**Sources:**
- `.capcom/issues/agenteval-*.md`, `.capcom/issues/audit-2026-11-03/test-*.md` (each item's Acceptance)
- `docs/reviews/code-audit-2026-11-03.md`
- `docs/plans/archived/2026-11-03 - 01 - remaining-small-bugs-plan.md` (item 3)
- Notes `agent-eval`, `cli-test-harness`, `coding-conventions`

**Ratified design calls:**
- **Sweep reach:** a group ledger file — the harness names a file in an env var, the `agent` loop appends each shell command's pgid, the sweep SIGKILLs the agent's group and every listed group. Covers the harness's own loop only, not `claude -p`. (Airic, 2026-11-03)

**Standing requirements:**
- skills: coding-standards
- Tests on the Pi: single packages with `GOMEMLIMIT=2GiB`, never `-race` on `./...`, never two test commands at once.
- `cmd/agenteval/doc.go` `# Files` map lists every new non-test file (`TestDocFileMap_MatchesDirectory`).
- Every authorised deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- `html-export`
- Sweeping the children of agents other than `agenteval agent` (`claude -p`)
- Windows process handling (stays as it is)

## 1. A cancelled run whose agent exits cleanly is not a start failure — ✅ DONE (2026-11-03)

**What:** fix: `runAgent` reports `start the agent <argv0>: context canceled` when the parent context is cancelled and the agent exits 0 after the group SIGTERM, though the agent ran (issue `agenteval-cancel-error-wording`).
**Regression guard.** The test script sets `trap 'exit 0' TERM` and only then touches a marker (`$0`); the test cancels with `cancelOnceExists` on that marker and asserts `err == nil` (or a message without `start the agent`), never `errors.Is(err, context.Canceled)`, which the `%w`-wrapped start error already satisfies.
`TestRun_AnAgentThatCannotStartFailsTheRun` fails in `resolveAgentArgv` (`the agent command:`) and never reaches `runAgent`: add a `runAgent`-level test whose argv0 exists but is not executable (Start fails, `Process == nil`), asserting the `start the agent` error is still returned.
**Goal:** when ctx is cancelled after the agent started, `runAgent` never returns a `start the agent` error; the cancelled run ends the way any other cancelled run ends in `runTask`/`runRun` (the cancellation returned unjudged), whatever the agent's exit status.
**Approach (assumed at the header base):** in `workspace.runAgent` (`cmd/agenteval/workspace.go`), the start-failure branch tests `!errors.As(err, &exited) && !run.IsTimedOut`. Treat an error while `ctx.Err() != nil` and the process started (`command.Process != nil`) as a cancellation, not a start failure; keep returning the start error only when the process never started. Keep the transcript note of how the agent ended.
**Files:** cmd/agenteval/workspace.go, cmd/agenteval/workspace_unix_test.go, cmd/agenteval/run_test.go
**Read first:** cmd/agenteval/workspace.go — runAgent; cmd/agenteval/run.go — resolveAgentArgv; cmd/agenteval/agent_unix.go — isolateAgentProcessGroup; cmd/agenteval/workspace_unix_test.go — TestRunAgent_KillsEveryProcessTheAgentStarted;
  cmd/agenteval/agent_test.go — cancelOnceExists, waitUntilGone; cmd/agenteval/run_test.go — TestRunTask_ACancelledRunReturnsTheCancellationUnjudged, TestRun_AnAgentThatCannotStartFailsTheRun
**Tests:** a Unix test runs an agent script that sets its TERM trap before touching the marker it is cancelled on, exits 0 on TERM, and asserts `err == nil` (no `start the agent` error) and the cancelled-run outcome; a `runAgent` test with a non-executable argv0 still returns the `start the agent` error; `TestRun_AnAgentThatCannotStartFailsTheRun` still passes.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./cmd/agenteval/`
- `go vet ./cmd/agenteval/`
**Closes:** agenteval-cancel-error-wording
**Commit:** `fix(agenteval): report a cancelled run as cancelled when its agent exits cleanly`

## 2. The sweep kills the shell groups the agent loop started — ✅ DONE (2026-11-03)

NOTES (2026-11-03): the CHANGELOG text above is an AMENDMENT, not a new entry: it replaces the existing `## [Unreleased]` line beginning "agenteval no longer leaves an agent's processes running" (CHANGELOG.md line 19), adding the ledger sweep; the CHANGELOG file itself is untouched.
NOTES (2026-11-03): the wait-delay seam is a `waitDelay` field on `workspace` (zero keeps `agentWaitDelay`); `agentWaitDelay` stays a const. runAgent creates the ledger as `<workspace root>/process-groups`, and a ledger it cannot create is an error before the agent starts (runAgent doc comment says so).
NOTES (2026-11-03): runShellCommand keeps its signature and reads the ledger path from the env; it now calls Start then Wait instead of Run so the group is recorded between them. Ledger read/append and the ended-group probe live in agent_unix.go (a `processGroupLedger` type); agent_other.go carries no-op methods. The sweep's skip rules are tested on `liveGroups` directly, so a broken guard can never SIGKILL pgid 0, 1 or the test's own group.
NOTES (2026-11-03): consequential edit — cmd/agenteval/doc.go: made necessary by agent_unix.go now holding the group ledger (its file-map line names it).

**What:** fix: when an `agenteval agent` loop outlives `agentWaitDelay` after the group SIGTERM, its shell commands (each in its own process group via `isolateProcessGroup`) survive the sweep of the agent's group and outlive the workspace (issue `agenteval-sweep-misses-groups`). Binding (ratified): a group ledger file.
**Regression guard.** The sweep skips any pgid `<= 1` and the harness's own `syscall.Getpgrp()`, and counts a ledger line only when it ends in `\n` (a torn tail is skipped); the env var reaches every agent (`os.Environ()`), `claude -p` included.
After each shell command's Wait the loop probes `kill(-pgid, 0)` and on ESRCH records the group as ended (a `-<pgid>` line); the sweep skips ended groups, and its doc comment names the remaining pid-reuse window.
`agentWaitDelay` is a `const`: the test shortens the wait through a named seam (a package var or a workspace field) or pays the real 5 s; its child gets its own group through a portable helper (a `copyTestBinary`-dispatched program setting `Setpgid`, or `perl -e 'setpgrp; exec @ARGV'`), never `setsid` or `set -m`. Keep `runShellCommand`'s signature (read the ledger path from the env inside it) or update its four callers in `agent_test.go`.
Every comment or doc line saying what the sweep kills is updated, found by `grep -rn 'sweep\|left running\|outlive\|in the group' cmd/agenteval docs/agent-eval.md CHANGELOG.md` (`isolateAgentProcessGroup` in `agent_unix.go` and `agent_other.go`, the Unreleased `CHANGELOG.md` entry); record the CHANGELOG amendment in the sidecar.
**Goal:** on Unix, after `runAgent` returns, no shell command started by an `agenteval agent` loop is still running, even when the loop ignores SIGTERM past `agentWaitDelay`.
**Approach (assumed at the header base):**
- A new env var constant `AGENTEVAL_PROCESS_GROUPS` beside `capcomVariable`/`callLogVariable` (`cmd/agenteval/shim.go`). `runAgent` creates the ledger file in the workspace (outside the repo) and passes its path in the agent's env.
- The `agent` verb (`cmd/agenteval/agent.go`, where it calls `isolateProcessGroup` on a shell command) appends the started process's pid (= its pgid) as one decimal line to the file named by the variable, when set; an append failure is reported in the loop's output, never fatal.
- `isolateAgentProcessGroup`'s sweep (`cmd/agenteval/agent_unix.go`) also reads the ledger and SIGKILLs `-pgid` for each line; ESRCH is no error, an unparsable line is skipped. `agent_other.go` stays a no-op.
- Update the `runAgent` doc comment and `docs/agent-eval.md` where they describe what the sweep reaches.
**Files:** cmd/agenteval/shim.go, cmd/agenteval/agent.go, cmd/agenteval/agent_unix.go, cmd/agenteval/agent_other.go, cmd/agenteval/workspace.go, cmd/agenteval/workspace_unix_test.go, cmd/agenteval/agent_test.go, docs/agent-eval.md
**Read first:** cmd/agenteval/agent_unix.go — isolateAgentProcessGroup, isolateProcessGroup; cmd/agenteval/agent.go — runShellCommand, runToolCall; cmd/agenteval/workspace.go — runAgent, agentWaitDelay; cmd/agenteval/shim.go — capcomVariable;
  cmd/agenteval/workspace_unix_test.go — TestRunAgent_KillsEveryProcessTheAgentStarted
**Tests:** a Unix test runs an agent that ignores TERM, starts a long-sleeping child in its own process group (through the portable helper), records it in the ledger, and outlives the shortened (or real 5 s) wait; after `runAgent` returns the child's pid is gone. A unit test of the ledger append in the agent verb (`agent_test.go`). A sweep test: lines `0`, `1`, the harness's own pgid, a torn tail and an ended `-<pgid>` group are skipped.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./cmd/agenteval/`
- `go vet ./cmd/agenteval/`
**Depends on item 1.**
**Closes:** agenteval-sweep-misses-groups
**Commit:** `fix(agenteval): sweep the shell groups the agent loop started`

## 3. Test the harness's signal wiring — ✅ DONE (2026-11-03)

NOTES (2026-11-03): the agent-group cleanup (SIGKILL -<agent pid>, then waitUntilGone) is registered by the shared waitForAgent helper for both tests, not only by the second-signal test, so the first test's failure path (a harness killed outright) cannot leak the agent's sleep either.
NOTES (2026-11-03): the first-signal test is a table over SIGINT and SIGTERM; it asserts exit status exitRunFailed, the agent gone and no report. The second asserts the harness died by SIGINT in under agentWaitDelay/2. Mutation-checked: dropping context.AfterFunc(ctx, stop) fails the second test, and dropping signal.NotifyContext fails both subtests of the first.

**What:** `main` (`cmd/agenteval/main.go`) cancels the command tree on SIGINT/SIGTERM via `signal.NotifyContext` and restores the default action once cancelled (`context.AfterFunc(ctx, stop)`). Nothing tests it (issue `agenteval-signal-wiring-test`).
**Regression guard.** binding — item 3 runs only after item 2 has landed (already declared "Depends on item 2"); its tests rely on item 2's ledger sweep, so no signal test may leave a child running.
Dispatch on a new basename (for example a const `agentevalName`) through `copyTestBinary` in `TestMain`'s name switch, calling `main()` then `os.Exit(0)`. Do not use an env switch: every child inherits it through `runAgent`'s `os.Environ()`, so the `capcom-real` copy would run as agenteval and seeding would fail.
Pass `--capcom copyTestBinary(t, dir, realCapcomName)` with `setHermeticGitEnv` and `--only next-claim-close`, as `runNextClaimClose` does. `prepareWorkspace` seeds with that build, so a fake capcom fails the run before the agent starts.
Second signal: after the first, resend SIGINT every ~50 ms until the process exits, because `context.AfterFunc` runs `stop` on its own goroutine and a single early resend is swallowed. Assert exit well under `agentWaitDelay`. Cleanup SIGKILLs `-<agent pid>` and then calls `waitUntilGone`, and the child's env sets `TMPDIR` to a `t.TempDir()`, so a workspace left by the killed harness (dead before `seeded.remove`) is removed with the test.
**Goal:** a Unix-only test shows a signal during `agenteval run` stops the agent and exits without writing the report, and a second signal ends the process at once.
**Approach (assumed at the header base):** re-execute the test binary as agenteval through a new basename (for example `agentevalName`) in `TestMain`'s name switch, copied with `copyTestBinary`, and run `run --out <tmp>/report.md --capcom copyTestBinary(t, dir, realCapcomName) --only next-claim-close` under `setHermeticGitEnv`, with an agent script that writes its pid and sleeps (first test) or ignores TERM (second test). Send SIGTERM/SIGINT once the agent pid appears. First: the process exits non-zero, the agent pid is gone, no report is written. Second: after the first signal, SIGINT resent every ~50 ms ends the process well before `agentWaitDelay`.
**Files:** cmd/agenteval/main_test.go, cmd/agenteval/main_unix_test.go
**Read first:** cmd/agenteval/main.go — main; cmd/agenteval/main_test.go — TestMain, copyTestBinary, setHermeticGitEnv; cmd/agenteval/run_test.go — runNextClaimClose; cmd/agenteval/run.go — runTask;
  cmd/agenteval/workspace.go — prepareWorkspace; cmd/agenteval/agent_test.go — waitUntilGone
**Tests:** the two tests above, run through the agenteval-basename copy against `capcom-real`. The second resends SIGINT until exit, and its Cleanup kills the agent's group.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'Signal' ./cmd/agenteval/`
- `go vet ./cmd/agenteval/`
**Depends on item 2.**
**Closes:** agenteval-signal-wiring-test
**Commit:** `test(agenteval): cover the Ctrl-C and SIGTERM wiring`

## 4. Test batch update operations — ✅ DONE (2026-11-03)

**What:** the empty-change update branch of `batch.Run` (`op.Update.IsEmpty()` rewriting through `EditIssue`) and `Result.MissingPlan` from an update setting a plan are untested.
**Goal:** `internal/batch/run_test.go` holds `TestRun_UpdateSettingPlan_ReportsMissingPlanOnTheResult` (no plan file → `MissingPlan` names it; existing plan → empty) and `TestRun_EmptyUpdate_RewritesTheIssueUnchanged`.
**Files:** internal/batch/run_test.go
**Read first:** internal/batch/run.go — runner.update, Result.MissingPlan; internal/batch/run_test.go — newFixture, fixture.run, issueText; internal/batch/parse.go — operationReader.update, changingKeys (`no-standing: false` is the only parse route to an empty update);
  internal/config/config.go — Config.PlanPath
**Tests:** the two named tests.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/batch/`
**Closes:** test-batch-update-ops
**Commit:** `test(batch): cover update ops setting a plan and empty updates`

## 5. Test the briefing's busy-index branch — ✅ DONE (2026-11-03)

**What:** the `errors.Is(err, index.ErrBusy)` branch of `readWork` (`internal/briefing/briefing.go`) is untested; it must not carry the "; run capcom reindex" hint.
**Regression guard.** `readWork` calls `index.Open` directly and the seam block holds only `readyList`, `listIssues`, `runCheck`: stubbing the open means adding an `openIndex = index.Open` seam beside `readyList` in `internal/briefing/briefing.go`, stubbed to return an error wrapping `index.ErrBusy`.
The held-lock route instead pays the full 5 s `defaultBusyTimeout` (`dsnOptions` is unexported): copy `holdIndexLock` from `internal/cli/reindex_test.go` locally (driver `sqlite` via the index import), build the index first (one `Compose`, or `index.Open` + Close), and say so with its cost.
**Goal:** a test holds the index busy (or stubs its open to return `index.ErrBusy`) and asserts exactly one `SectionIndex` failure with no reindex hint, with the locks and check sections still composed.
**Files:** internal/briefing/briefing_test.go, internal/briefing/briefing.go (the `openIndex` seam route only)
**Read first:** internal/briefing/briefing.go — Briefing.readWork, the seam var block (readyList, listIssues, runCheck), Compose; internal/briefing/briefing_test.go — newBriefingRepo, failedSections, TestCompose_BrokenIndexAndLocks_FailInOrderAndKeepTheRest;
  internal/index/index.go — ErrBusy; internal/cli/reindex_test.go — holdIndexLock
**Tests:** the test above.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/briefing/`
**Closes:** test-briefing-busy-index
**Commit:** `test(briefing): cover a busy index without the reindex hint`

## 6. Exercise the real clipboard runner and its timeout — ✅ DONE (2026-11-03)

NOTES (2026-11-03): the timeout is a new `Writer.Timeout` field (zero means `runTimeout`, 2 s), the item's preferred option; the two real-runner tests switch the helper on with `t.Setenv` and so do not call `t.Parallel`
NOTES (2026-11-03): consequential edit — internal/clipboard/doc.go: made necessary by the real-runner tests, which falsify "no real program runs" (now names the timeout field and the test binary as the one real program)

**What:** `runProgram` (`internal/clipboard/clipboard.go`), the only production runner, and the `runTimeout` kill are untested.
**Regression guard.** `runProgram` returns `cmd.Run()`'s `*exec.ExitError` (`signal: killed`), not `ctx.Err()`: the timeout test asserts a non-nil error returned well before the helper's sleep ends (an elapsed bound), not `errors.Is(err, context.DeadlineExceeded)` nor the `signal: killed` wording (Windows differs) — or `runProgram` returns `ctx.Err()` once ctx is done and the item names the new message.
Prefer a `Writer` field for the timeout (zero means `runTimeout`), matching "Every input is a field"; with a package var the test stays non-parallel (every clipboard test calls `t.Parallel`) and restores it in `t.Cleanup`.
`runProgram` leaves Stdout/Stderr nil and xclip's `-in -selection clipboard` reach the re-executed binary's flag parser: the helper writes what it read to a file named by an env var, switched on in a new `TestMain` before `m.Run`, or the test calls `runProgram` directly with `-test.run=` args.
**Goal:** a test runs the real default runner against a helper (the test binary re-executed through `os.Args[0]` with an env switch) and asserts stdin delivery and a returned non-zero exit; a helper sleeping past a shortened timeout is killed with a context error, without the test waiting 2 s.
**Approach (assumed at the header base):** `runTimeout` is a `const`; make the timeout overridable from tests (a package var or a `Writer` field), production value unchanged.
**Files:** internal/clipboard/clipboard.go, internal/clipboard/clipboard_test.go
**Read first:** internal/clipboard/clipboard.go — runProgram, Writer.runBounded, runTimeout, Writer.runner, Writer (field doc); internal/clipboard/clipboard_test.go — recorder, TestWriteNative_ProgramFailure_IsReturned;
  internal/ui/clipboard.go — clipboardWriter (keyed Writer literal, unaffected by a new field)
**Tests:** the stdin/exit test (the helper's read checked through its env-named file) and the timeout test (a non-nil error within an elapsed bound).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/clipboard/`
- `go vet ./internal/clipboard/`
**Closes:** test-clipboard-real-runner
**Commit:** `test(clipboard): exercise the real runner and its timeout`

## 7. Test ready.hide and duplicate-key config handling — ✅ DONE (2026-11-03)

**What:** no `internal/config` test sets a non-empty `hide`, an unknown `ready.*` key, a non-list or empty-string `hide` entry, or a duplicate top-level key.
**Goal:** a load case `ready:\n  hide: [wip, blocked]` yields `Ready.Hide == [wip blocked]`; invalid-config rows for `ready:\n  hide: wip`, `ready:\n  other: x`, `ready:\n  hide: [""]` and `types: [bug]\ntypes: [task]` each fail naming the offending line.
**Files:** internal/config/config_test.go
**Read first:** internal/config/parse.go — parser.ready, parser.nameList, parser.eachKey (duplicate key at the second key's line), parser.topLevel; internal/config/config_test.go — TestParse_InvalidConfig_FailsWithPathAndLine (rows: wantLine 2 for all four),
  TestParse_PartialConfig_KeepsDefaultsForMissingKeys (shape for the load case; there is no load table); internal/config/config.go — Config.Ready.Hide
**Tests:** the load case and the four invalid rows, in the existing table tests.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/config/`
**Closes:** test-config-ready-hide
**Commit:** `test(config): cover ready.hide and duplicate top-level keys`

## 8. Test gendocs' stale-page removal — ✅ DONE (2026-11-03)

**What:** `run` and `removeStalePages` (`cmd/gendocs/main.go`) delete every `capcom*.md` in the output directory; `cmd/gendocs` has no test file.
**Goal:** `cmd/gendocs/main_test.go` calls `run(t.TempDir())` with a pre-seeded `capcomold.md` and an unrelated `notes.md` and asserts the stale page is gone, `notes.md` survives and the reference pages are written.
**Files:** cmd/gendocs/main_test.go
**Read first:** cmd/gendocs/main.go — run, removeStalePages, stalePagePattern; internal/clidoc/reference.go — WriteReference, collectPages; internal/clidoc/reference_test.go — TestReferenceUpToDate, readPages (capcom.md is the root page to assert)
**Tests:** the test above.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./cmd/gendocs/`
**Closes:** test-gendocs-stale-pages
**Commit:** `test(gendocs): cover stale-page removal`

## 9. Dependency idempotence, alias matching and a stale-Catalog duplicate

**What:** fix: `Ops.AddDep`'s `EditIssue` closure appends when the Catalog-based `addition.IsPresent` is false, without checking the file it edits, so a stale Catalog duplicates a dep the file already names. Also untested: AddDep's no-op and RemoveDep matching a dep stored as an alias.
**Goal:** AddDep never writes a dep the target file already names (by id or by an Alias of that issue); AddDep twice, or with an alias of an existing dep, leaves `Deps` and the file bytes unchanged; RemoveDep with the id of an issue the file names by an alias removes that entry.
**Approach (assumed at the header base):** in `AddDep` (`internal/ops/deps.go`), the closure skips the append when `target.Deps` already holds `addition.ID` or an alias resolving to it, matching the way `RemoveDep` matches through the graph.
**Files:** internal/ops/deps.go, internal/ops/ops_test.go
**Read first:** internal/ops/deps.go — AddDep, RemoveDep, DepGraph; internal/reflist/reflist.go — List.Add, namesID; internal/ops/ops_test.go — fakeCatalog, TestAddDep_FakeCatalog_ResolvesBothReferencesThroughIt; internal/issue/deps.go — DepGraph.Resolve
**Tests:** AddDep twice; AddDep by alias of an existing dep; RemoveDep of an alias-stored dep; AddDep through a stale fake Catalog (the existing `fakeCatalog`) missing the dep — file unchanged. The stale-Catalog test fails against the pre-item tree.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ops/`
- `go vet ./internal/ops/`
**Closes:** test-ops-dep-idempotence
**Commit:** `fix(ops): never duplicate a dep the file already names`

## 10. Test recap's hostile-repository guards

**What:** in `internal/recap/recap.go`, the oversized-blob guard (`gitx.ErrBlobTooLarge` becoming a warning) and the warning when a second file claims an id already seen are untested.
**Regression guard.** `list` keeps the first file (`a/x.md`) and drops only the second: `b/x.md` is given a different title and is the one left out — `x` is listed once with `a/x.md`'s title and one `recap:` warning names `b/x.md`. Commit the pair at HEAD only (`Compute(s, "")`) so the boundary adds no second warning.
**Goal:** `TestCompute` cases: a committed file over 1 MiB gives a `recap:` warning, is left out of the Report, the rest intact; two committed files with one id (`a/x.md`, `b/x.md` with a different title) list `x` once with `a/x.md`'s title and give one `recap:` warning naming `b/x.md`.
**Files:** internal/recap/recap_test.go
**Read first:** internal/recap/recap.go — Compute, computation.list, load; internal/recap/recap_test.go — newStore, commit, issueFile, TestCompute_UnparsableFile_IsAWarningAndLeftOut; internal/gitx/branches.go — ErrBlobTooLarge
**Tests:** the two cases.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/recap/`
**Closes:** test-recap-hostile-guards
**Commit:** `test(recap): cover oversized blobs and duplicate ids`

## 11. Test the staged catalog's filters against the index

**What:** `isSelected` (`internal/stage/catalog.go`) re-implements the index query filter in memory; `catalog_test.go` covers few fields.
**Regression guard.** `Catalog.List` runs `isSelected` only on staged entries (index entries come back as `index.List` filtered them): every issue in the table is staged (`Overlay.ReplaceFile` of each indexed file with its own bytes, or staged creates in a twin store), so every row of `Catalog.List` goes through `isSelected`. `Filter.PlanContains`, which `isSelected` re-implements with `strings.Contains`, is in the field list beside Plan.
**Goal:** a table test applies each filter field (Statuses, Categories, Types, Priorities, ParentID, Plan, PlanContains, Labels, NoLabels, IsClosedIncluded) to the same issues through `Catalog.List` and `index.List` and asserts equal ids per row.
**Files:** internal/stage/catalog_test.go
**Read first:** internal/stage/catalog.go — isSelected, catalog.List; internal/index/query.go — Filter, Index.List; internal/stage/catalog_test.go — TestCatalog_List_MergesTheStagedIssuesInPathOrder, newCatalogStage;
  internal/stage/overlay.go — Overlay.ReplaceFile, Overlay.Catalog
**Tests:** the table test, every issue staged. A divergence it finds is fixed in `isSelected` toward `index.List` and noted as a dated NOTES line.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/stage/`
**Closes:** test-stage-catalog-parity
**Commit:** `test(stage): check catalog filters against the index`

## 12. Test the staged overlay's size cap and directory refusal

**What:** the 1 MiB cap on a staged file (`ReadRegularFile` → `nodefile.ErrTooLarge`) and the staged-directory `ErrNotRegular` refusal (`internal/stage/overlay.go`) are untested.
**Goal:** a test stages a ReplaceFile over 1 MiB and asserts `ReadRegularFile` fails with `nodefile.ErrTooLarge`; a test stages a CreateFile'd directory and asserts `nodefile.ErrNotRegular`.
**Files:** internal/stage/overlay_test.go
**Read first:** internal/stage/overlay.go — Overlay.ReadRegularFile, Overlay.ReplaceFile, Overlay.CreateFile; internal/nodefile/read.go — ErrTooLarge, ErrNotRegular, MaxNodeFileSize; internal/stage/overlay_test.go — newFixture, TestOverlay_WriteRefusals
**Tests:** the two tests.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/stage/`
**Closes:** test-stage-overlay-limits
**Commit:** `test(stage): cover the overlay's size cap and directory refusal`

## 13. Test the UI write path's failure branches

**What:** the post-write failure branches of `applyWritten` and `applyConflict` (`internal/ui/write.go`) are untested: `withRefreshFailure`, `reloadAfterFormat` failing on a vanished file, and the `writeFailurePrefix` notice.
**Regression guard.** A file deleted before the write fails in `Kind.Edit`'s read as a file error, not a `ConflictError`: build the vanished-file case as a `pendingWrite` whose `writeAction.run` deletes alpha's file and returns a `*nodestore.ConflictError`, run `write.land(ts.store, ts.index)`, then `m.applyWritten(msg)`.
`land` refreshes only after `action.run`, which resolves through the same index: test the refresh branch by feeding `m.applyWritten` a `writtenMsg{notice: ..., refreshErr: errors.New(...)}` directly.
**Goal:** tests assert: a failing `Refresh` gives `<notice> (refresh: ...)`; the issue file deleted between the conflict and the re-read gives `write: ... (reload: ...)`, no prompt opened, disk unchanged; a generic non-conflict write error gives the `writeFailurePrefix` notice.
**Files:** internal/ui/write_test.go
**Read first:** internal/ui/write.go — pendingWrite.land, model.applyWritten, model.applyConflict, withRefreshFailure, writtenMsg, writeAction; internal/ui/write_test.go — TestWrite_FileChangedOnDisk_AsksAndYRunsAgain, newTestStore
**Tests:** the three tests: the vanished-file case through a hand-built `pendingWrite` and `land`, the refresh case through a hand-built `writtenMsg`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/`
**Closes:** test-ui-write-failures
**Commit:** `test(ui): cover the write path's failure notices`
