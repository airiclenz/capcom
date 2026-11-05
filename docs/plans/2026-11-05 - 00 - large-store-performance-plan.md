# Large-store performance plan

**Goal:** Make the cold index build grow linearly with the store, read recap's file copies in one
git process, let a refresh that finds nothing to write run without the write lock, and stop the
stress tests leaking their built binary.
**Date:** 2026-11-05
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 07ff7a8
**Sources:**
- `docs/reviews/benchmarks-2026-11-05.md` (the baseline the numbers come from)
- `.capcom/issues/code-maturity/index-build-quadratic.md`
- `.capcom/issues/code-maturity/recap-large-store.md`
- `.capcom/issues/code-maturity/index-refresh-contention.md`
- `.capcom/issues/code-maturity/stress-binary-dir-leak.md`
- Note `decisions` / `adr-0007` (tests use real temp files, SQLite and git)

**Ratified design calls:**
- **Scope:** the four issues above; `warm-reads-large-store` waits for its profile (Airic, 2026-11-05).
- **Busy reads:** a clean refresh takes no write lock; read-only commands may still fail busy after the wait when files changed (Airic, 2026-11-05).

**Standing requirements:**
- skills: coding-standards
- Tests on the Pi: one package at a time with `GOMEMLIMIT=2GiB`; never `./...` with `-race`; never two test commands at once.
- Benchmark numbers are measured on the Raspberry Pi 5 host and added to `docs/reviews/benchmarks-2026-11-05.md` as a new dated section, the baseline rows left as they are.

**Out of scope:**
- `warm-reads-large-store`, `bench-workflow-runtime` (needs a first GitHub run), `.github/workflows/bench.yml`.
- Any change to the index schema version or the store format.

**Regression check (2026-11-05, 07ff7a8):**
- 1: guard folded
- 2: guard folded
- 3: recast (writer's decision); supersedes the `internal/index/schema.go` comment on `file_verdicts`/`name_verdicts`
- 4: guard folded (writer's decision); the Approach's `ShowFile` callers corrected
- 3 (re-check): guard folded; narrows the `CHANGELOG.md` [0.12.0] busy-index entry (line 93), which the item's own CHANGELOG entry says

## 1. Stress tests remove their built binary — ✅ DONE (2026-11-05)

**What:**
**Regression guard.** Set the build-dir package var right after `os.MkdirTemp` succeeds, before `go build`, so a
failed build still leaves it set, and have `TestMain` `os.RemoveAll` it whenever it is non-empty.
**Goal:** after `go test -tags stress` of `internal/cli` finishes, no `capcom-stress-*` directory it created remains in the system temp dir.
**Approach (assumed at the header base):** `stressBinary` in `internal/cli/stress_test.go` builds into `os.MkdirTemp("", "capcom-stress-")` under `stressBuild` and never removes it. Add `internal/cli/stress_main_test.go` with `//go:build stress` holding a `TestMain` that runs `m.Run()`, removes the directory `stressBinary` built into (keep the dir in a package var beside `stressBinaryPath`; a build that never ran removes nothing) and exits with `m.Run`'s code. Untagged builds keep no `TestMain`, so `bench_test.go` and the other tests are unaffected. Rewrite the `stressBuild` comment that says the directory outlives the tests.
**Files:** internal/cli/stress_test.go; internal/cli/stress_main_test.go
**Read first:** internal/cli/stress_test.go — stressBinary, stressBuild, stressBinaryPath, stressBuildErr; internal/cli/bench_test.go — benchStoreSizes; .github/workflows/stress.yml — Stress step
**Closes:** stress-binary-dir-leak

**Tests:** no new test; the acceptance commands below check the temp dir before and after.

**Acceptance:**
- `go vet -tags stress ./internal/cli`
- `ls -d ${TMPDIR:-/tmp}/capcom-stress-* 2>/dev/null | sort > /tmp/before; GOMEMLIMIT=2GiB CAPCOM_STRESS_WORKERS=4 go test -tags stress -count=1 -run Stress ./internal/cli; ls -d ${TMPDIR:-/tmp}/capcom-stress-* 2>/dev/null | sort > /tmp/after; diff /tmp/before /tmp/after` prints nothing.

**Commit:** `fix(cli): remove the stress tests' built binary when they finish`

## 2. Index build grows linearly with the store — ✅ DONE (2026-11-05)

NOTES (2026-11-05): for a file without a files row, forget skips only the fts delete and keeps the other deletes (all by a path index), so an earlier walk's claims at that path are still dropped (the plan's regression guard); TestIndexFile_FileGrownPastTheLimitSinceTheWalk_WarnsAndKeepsItOut now calls indexFile(file, false).
NOTES (2026-11-05): measured with -count=3 (as the baseline) besides the Acceptance's -benchtime=1x run: reindex 0.41 s -> 4.33 s (10.6x mean, 9.3x without one slow count; 0.39 s -> 3.96 s in the 1x run), cold first command 1.94 s -> 5.34 s (0.83 s -> 4.81 s in the 1x run); per-issue reindex cost is flat at ~0.4 ms, so no profile was taken.

**What:** Fixes the quadratic cold build measured in the baseline (`BenchmarkReindex` 1.08 s → 53.95 s, `BenchmarkColdFirstCommand` 1.42 s → 54.51 s from N=1000 to N=10000).
**Regression guard.** A file unknown to `files` may still hold claims rows from an earlier walk that left it out
(`replaceWalkClaims` spares paths that have a files row), so for an unknown file skip only the `fts` delete (fts rows
exist only beside a files row: `insertIssue`, `insertNote`) and keep the other deletes, or delete `claims WHERE path = ?`.
`TestIndexFile_FileGrownPastTheLimitSinceTheWalk_WarnsAndKeepsItOut` calls `r.indexFile(file)`: update it, or keep the signature and call `forget` from `sync`.
**Goal:** `BenchmarkReindex` and `BenchmarkColdFirstCommand` at N=10000 take at most about ten times their N=1000 time and under 10 s on the Pi 5 host, and a file changed after a refresh still leaves exactly one `fts` row.
**Approach (assumed at the header base):** `refresher.indexFile` in `internal/index/refresh.go` calls `refresher.forget` for every file it indexes, and `forget` deletes by `path` from every table in `rowTables`, `fts` included, whose `path` is `UNINDEXED` (`internal/index/schema.go`), so each delete scans the whole full-text table. `sync` already knows from `recorded` whether a file has rows (`isKnown`): pass that to `indexFile` and call `forget` only for a known file. A rebuild (`ensureSchema` dropped every table) and a cold build then forget nothing. If the benchmarks still grow faster than about ten times, profile `BenchmarkReindex` (`-cpuprofile`) and fix the next hotspot inside `internal/index` in this item; record what the profile showed as a dated NOTES line.
**Files:** internal/index/refresh.go; internal/index/refresh_test.go; internal/index/index_test.go; docs/reviews/benchmarks-2026-11-05.md
**Read first:** internal/index/refresh.go — refresher.sync, refresher.indexFile, refresher.forget, replaceWalkClaims; internal/index/duplicates.go — membershipFacts; internal/index/schema.go — rowTables;
internal/index/index_test.go — TestRefresh_WalkRefusedFileFixed_DropsItsClaim, TestIndexFile_FileGrownPastTheLimitSinceTheWalk_WarnsAndKeepsItOut
**Closes:** index-build-quadratic

**Tests:** in `internal/index/refresh_test.go`: index a store, change one issue's body, refresh, and assert `SELECT count(*) FROM fts WHERE path = ?` is 1 for that file and its new body is the one searched; and a new file added after a refresh gets exactly one `fts` row. In `internal/index/index_test.go`, `TestRefresh_WalkRefusedFileFixed_DropsItsClaim` also asserts that no claims row of the old walk claim is left at the fixed file.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/index`
- `GOMEMLIMIT=2GiB go test -run='^$' -bench='Reindex|Cold' -benchtime=1x ./internal/cli` shows N=10000 at most ~10× N=1000 and under 10 s; the numbers are added to the baseline doc.

**Commit:** `perf(index): forget a file's rows only when the index holds them`

## 3. A refresh with nothing to write takes no write lock

**What:** Recast at the regression check (2026-11-05). Depends on item 1 and item 2 (both touch files this item edits).
**Regression guard.** (a) Ratified design call (Airic, 2026-11-05): the mixed 16-worker stress workload keeps accepting a busy failure after busyWaitFloor for every command, ready/prime included — do NOT drop it; instead add a separate stress test in internal/cli/stress_test.go: a quiescent store whose files are back-dated past the racy-clean window, one process holding the index write lock (an IMMEDIATE transaction) for longer than the busy timeout, and parallel `ready`/`prime` processes that must all exit 0; the Goal and Acceptance say this, not "no busy in the mixed run". Add to the header's Ratified design calls: `**Busy reads:** a clean refresh takes no write lock; read-only commands may still fail busy after the wait when files changed (Airic, 2026-11-05).` (b) Walk once: the read pass hands its found files, walk claims and walk warnings to the IMMEDIATE pass, which re-reads only recordedFiles and storedWalkClaims under the lock; the read pass reuses refresher.sync's isKnown/isDirty logic from item 2, never a copy. (c) The read pass also compares the decided membership verdicts with the stored file_verdicts/name_verdicts and takes the write path on any difference. (d) Run the read pass only when !isForced; word the Goal "an unforced refresh that finds …". (e) TestReindex_BusyIndex_ListFailsInvalidNamingReindexWithoutReindexHint (internal/cli/reindex_test.go) and TestRefresh_IndexLockedByAnotherCommand_FailsBusy (internal/index/index_test.go) dirty the store (write a further issue file) after taking the lock; add both files to **Files:**. (f) Comment guard as a rule: every comment saying a refresh or BeginTx takes the write lock / runs IMMEDIATE, or that verdicts are rebuilt whole by every refresh, is rewritten — found by `grep -n -i 'immediate\|write lock\|rebuilt whole' internal/index/*.go`; this supersedes the schema.go comment on file_verdicts/name_verdicts (add internal/index/schema.go to **Files:**). (g) The issue's own Acceptance in .capcom/issues/code-maturity/index-refresh-contention.md is already amended to match (a) by the plan author; the item does not edit it.
**Regression guard.** (re-check, 2026-11-05) (h) Split `persistMembershipOf` into a decide-and-warn half (`tree.DecideMembership`, `warnBelowRepeatedIDs`, `diagnose.NameClashes`, the alias-conflict loop, in that order) that both passes call, and a write half (`insertFileVerdicts`, `insertNameVerdicts`, the `excluded` UPDATE); the clean path reproduces all three warning kinds, so `TestRefresh_InvalidFileListsAnAliasFirst_WarnsNamingTheFirstCleanClaimant` stays green. (i) The IMMEDIATE path still runs `ensureSchema(ctx, tx, hash, false)` first under the lock; (b)'s hand-over replaces the walk only. (j) The clean-reads stress test reaches the racy-clean window by waiting past `racyGranularity` (>1 s) after the last write and running one refreshing command (e.g. `list`), no `os.Chtimes` (it sets ctime to now); only then does it hold the lock (`holdIndexLock`, `internal/cli/reindex_test.go`). It asserts every `ready` exits 0 and every `prime` exits 0 with no `the index is busy` in its stdout (prime turns an index failure into a text warning on stdout). (k) `TestRefresh_CommitFails_ReportsOnlyTheCommitError` writes a second issue file after `openIndex`, so its refresh takes the write path and meets `commitTx`; the file_verdicts test opens with `laterClock()`. (l) This narrows the `CHANGELOG.md` [0.12.0] entry (line 93: a command meeting a locked index past the busy timeout fails `invalid`) to commands with changes to write, per the Busy reads design call; the item's CHANGELOG entry (carried in its sidecar) says so.
**Goal:** an unforced refresh that finds no new, changed, racily clean or vanished file, a current schema, unchanged walk claims and unchanged membership verdicts writes nothing and holds no write lock, returning the same Report (warnings included) a write refresh would; two refreshes that both have changes to write never deadlock; on a quiescent store indexed clean past the racy-clean window, while another process holds the index write lock for longer than the busy timeout, parallel `ready`/`prime` processes all exit 0 with no index-busy warning. The mixed 16-worker workload of `TestStress_OneWorktree` keeps accepting a busy failure after `busyWaitFloor` for every command.
**Approach (assumed at the header base):** `Index.refresh` in `internal/index/refresh.go` opens one transaction that `dsnOptions` (`_txlock=immediate`, `internal/index/index.go`) makes IMMEDIATE, and always runs `persistMembership` (`internal/index/duplicates.go`), which deletes and rewrites `file_verdicts` and `name_verdicts`. When `!isForced`, add a read pass first: `x.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})` (modernc issues a plain deferred `begin` for ReadOnly), check `isSchemaCurrent`, walk, compare against `recordedFiles` with `refresher.sync`'s `isKnown`/`isDirty` logic from item 2 (reused, never copied), compare `storedWalkClaims` with this walk's claims, and compare the decided membership verdicts with the stored `file_verdicts`/`name_verdicts`. When all are clean, reproduce the Report without writing: `rewarn` stored problems, the membership warnings `persistMembershipOf` gives (repeated ids, name clashes, duplicate aliases: its decide-and-warn half, with no `DELETE`/`INSERT`), and the `Indexed` count; then end the read transaction. Otherwise end it and run the IMMEDIATE path, handing it the read pass's found files, walk claims and walk warnings: it walks no second time, runs `ensureSchema` first under the lock, and re-reads only `recordedFiles` and `storedWalkClaims`. The read transaction never upgrades, so no deadlock. Rewrite every comment `grep -n -i 'immediate\|write lock\|rebuilt whole' internal/index/*.go` finds that says a refresh or `BeginTx` takes the write lock or that verdicts are rebuilt whole by each refresh. In `internal/cli/stress_test.go`, add the separate clean-reads stress test; `mixedReady` and `mixedPrime` keep accepting a busy failure after `busyWaitFloor`.
**Files:** internal/index/refresh.go; internal/index/duplicates.go; internal/index/index.go; internal/index/schema.go; internal/index/refresh_test.go; internal/index/index_test.go; internal/cli/stress_test.go; internal/cli/reindex_test.go
**Read first:** internal/index/refresh.go — Index.refresh, refresher.sync, recordedFile.isDirty, storedWalkClaims; internal/index/duplicates.go — persistMembershipOf, warnBelowRepeatedIDs;
internal/index/schema.go — ensureSchema; internal/cli/reindex_test.go — holdIndexLock
**Closes:** index-refresh-contention

**Tests:** in `internal/index/refresh_test.go`: a clean refresh (opened with `laterClock()`) while another connection holds an IMMEDIATE transaction succeeds within the busy timeout and reports the same warnings (a stored problem, a repeated id and a duplicate alias) as the refresh before it; two goroutines each with their own `Index` refreshing a store with pending changes both succeed (no deadlock, no busy); a refresh opened with `laterClock()` whose stored `file_verdicts` differ from the decided ones takes the write path and rewrites them. In `internal/cli/stress_test.go`: `TestStress_CleanReadsWhileTheWriteLockIsHeld` — a quiescent store indexed clean by one refreshing command run more than `racyGranularity` after the last write, then one process holding the index lock (`holdIndexLock`) longer than the busy timeout, parallel `ready`/`prime` processes that all exit 0, `prime` with no `the index is busy` in its stdout. `TestReindex_BusyIndex_ListFailsInvalidNamingReindexWithoutReindexHint` (`internal/cli/reindex_test.go`) and `TestRefresh_IndexLockedByAnotherCommand_FailsBusy` (`internal/index/index_test.go`) write a further issue file after taking the lock. `TestRefresh_CommitFails_ReportsOnlyTheCommitError` (`internal/index/index_test.go`) writes a second issue file after `openIndex`.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/index`
- `GOMEMLIMIT=2GiB go test -count=1 -run 'BusyIndex' ./internal/cli`
- `GOMEMLIMIT=2GiB go test -tags stress -count=3 -run 'Stress_OneWorktree|Stress_CleanReadsWhileTheWriteLockIsHeld' ./internal/cli` (the clean-reads test: every `ready`/`prime` exits 0 with no index-busy warning)

**Commit:** `perf(index): check for changes before taking the index write lock`

## 4. recap reads every file copy through one git process

**What:**
**Regression guard.** When the cat-file --batch process dies after it has started answering, the object in flight gets an Err carrying git's stderr and the unanswered rest is read in a new process; ReadBlobs returns an error only when no object was answered; add a gitx test with a body-corrupt blob before a good one. Correct the approach: only internal/elsewhere calls gitx.ShowFile (cmd/release has its own execRepo.ShowFile), so ShowFile stays for internal/elsewhere.
**Goal:** `BenchmarkRecap` at N=10000 runs under 10 s on the Pi 5 host; a copy larger than `maxFileSize` still gives the `tree.ErrTooLarge` warning and is never buffered whole, and a copy git cannot read still gives a warning, each pinned by a test.
**Approach (assumed at the header base):** `load` in `internal/recap/recap.go` calls `gitx.ShowFile` (`internal/gitx/branches.go`) per copy, two git processes each. Add to `internal/gitx` (new file `blobs.go`) `ReadBlobs(dir string, objects []string, limit int64) (map[string]Blob, error)`, `Blob{Data []byte; Err error}`: one `git cat-file --batch` process (same `GIT_OPTIONAL_LOCKS=0` environment as `runWithInput`), object names written to stdin from a goroutine, stdout read as a stream; a header over `limit` is discarded with `io.CopyN(io.Discard, …)` and gets `Err` wrapping `ErrBlobTooLarge`, a `missing` line gets `Err` wrapping a new `ErrBlobMissing`. A process that dies after it has started answering is handled as the regression guard says; only a failure before any object was answered is the returned error. In `Compute`, after the snapshots, collect the blob of every HEAD file new or changed against the boundary and of every boundary file not kept identical at HEAD, read them in one `ReadBlobs` call into the computation, and make `load` take its bytes from there (the same warnings as now). `ShowFile` stays for `internal/elsewhere` (`cmd/release` has its own `execRepo.ShowFile`).
**Files:** internal/gitx/blobs.go; internal/gitx/blobs_test.go; internal/recap/recap.go; internal/recap/recap_test.go; docs/reviews/benchmarks-2026-11-05.md
**Read first:** internal/recap/recap.go — Compute, load; internal/gitx/gitx.go — runWithInput, optionalLocksOff; internal/gitx/branches.go — ShowFile, ErrBlobTooLarge;
internal/recap/recap_test.go — TestCompute_OversizedFile_IsAWarningAndLeftOut; internal/elsewhere/elsewhere_test.go — TestScan_BranchWithAMissingTree_WarnsForItAndReportsTheOthers
**Closes:** recap-large-store

**Tests:** `internal/gitx/blobs_test.go`: reads several blobs in one call; a blob over the limit gets `ErrBlobTooLarge` and the blobs after it still read; an unknown object name gets `ErrBlobMissing`; an empty list starts no process; a body-corrupt blob before a good one gets an `Err` carrying git's stderr and the good one still reads. `internal/recap/recap_test.go`: an oversize copy warns with `tree.ErrTooLarge`, and an unreadable copy warns and is left out while the rest of the recap holds.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/gitx ./internal/recap`
- `GOMEMLIMIT=2GiB go test -run='^$' -bench=Recap -benchtime=1x ./internal/cli` shows N=10000 under 10 s; the number is added to the baseline doc.

**Commit:** `perf(recap): read every file copy through one git cat-file process`
