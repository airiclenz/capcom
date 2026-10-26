# Plan: the 2026-10-26 robustness audit, P1 and P2

**Goal:** Close the eleven ready P1/P2 issues of the 2026-10-26 audit: a corrupt index that
`reindex` repairs, a stale-index check that sees same-size same-mtime edits, four Tree lock fixes,
three `--json` gaps, a toolchain-proof hooks test, and CI that runs `-race` and gates coverage.

**Date:** 2026-10-26
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** ec7e795

**Sources:**
- `.capcom/issues/audit-1026/*.md` (each item names its issue)
- ADR 0016 and its amendment (`capcom know show adr-0016-amend-1026`), ADR 0005, ADR 0007
- Notes `cli-test-harness`, `error-messages`, `store-layout`
- `docs/site/reference/json-output.md`

**Ratified design calls** (user, 2026-10-26):
- **Coverage:** CI gates total coverage on Linux at a floor set at implement time: today's total, rounded down to a whole percent, minus 2.
- **Windows change source:** NTFS `FILE_BASIC_INFO.ChangeTime` through `golang.org/x/sys/windows`; Unix uses ctime, inode and device from the walk's stat.
- **Group `--json`:** refused like `capcom ui --json`: usage error `<path> does not support --json`, exit 2; `--help` still prints help.
- **prime JSON:** `"worktree": {"key", "state", "advice"}`, `advice` the exact text line; left out only when the lock section failed.
- **Corrupt lock hint** (writer): `capcom lock status, then capcom lock break`, the wording `finish` already uses.
- **Tree JSON fields** (writer): `children` reuses show's `{"total","closed"}` object; `more` and `repeat` are set exactly when the text prints the cut line or `(see above)`; all three omitted when empty.
- **ADR 0005 stale rule:** amended, not yielded to: a heartbeat more than one TTL in the future is also stale.

**Standing requirements:**
- skills: coding-standards
- Tests on the Pi host: one package at a time, `GOMEMLIMIT=2GiB`, `-race` only on a single package, never on `./...`.
- CLI tests go through `runCLI` (Note `cli-test-harness`), never `t.Parallel()`.
- Every new error names the file, and the line when known (Note `error-messages`).

**Out of scope:**
- The P3 issues, `agent-eval-harness`, `lock-warn-on-write`, `keep-lock-blocks-next`.
- `know tree --json` and any other tree output not named in item 8.
- CHANGELOG release headings, VERSION, tags.

**Regression check (2026-10-26, ec7e795):**
- 1: guard folded
- 2: guard folded
- 3: guard folded (writer decision on the exit code)
- 4: guard folded
- 5: guard folded; yields to ADR 0005 (adr-0005-decision.md:11-12)
- 6: guard folded
- 7: guard folded
- 8: guard folded
- 9: guard folded
- 10: guard folded
- 11: guard folded (writer decision on the release gate)
- 5: decision applied (user ratified call, second round): supersedes ADR 0005's decision (`adr-0005-decision.md`) by amendment Note `adr-0005-amend-1026`; replaces the "yields to ADR 0005" call above

## 1. reindex repairs a corrupt index

**What:**
**Goal:** `capcom reindex` succeeds on an index SQLite reports as not a database or corrupt, by
discarding `index.db`, `index.db-wal` and `index.db-shm` and rebuilding; an index failing for any
other reason (busy, locked, permission) is never deleted. Every CLI command that meets a corrupt
index fails `invalid` with hint `capcom reindex`, in text and `--json`.
Fix: `reindex` dies in `db.Ping()` on the bad header before it can drop anything.
**Approach (assumed at the header base):** `internal/index/index.go` gains an exported sentinel
`ErrCorrupt` and a classifier `isCorruptIndex(err)` true only for a `*sqlite.Error` whose primary
code (`Code() & 0xff`) is `SQLITE_NOTADB` or `SQLITE_CORRUPT` (constants from
`modernc.org/sqlite/lib`). `Open` wraps a corrupt failure from `Ping` or the first `refresh` in
`ErrCorrupt`; in `ModeRebuild` it closes the db, removes the three files (missing ones ignored; a
failed remove is returned naming its path) and opens once more. `prepare` in
`internal/cli/root.go` maps `errors.Is(err, index.ErrCorrupt)` to `output.Error{Code: CodeInvalid,
Hint: "capcom reindex"}`. `briefing.go`'s existing `; run capcom reindex` stays.
**Regression guard.** The Goal's second sentence is narrowed to: a command whose index
`prepareIndex` opens (`internal/cli/root.go`) and whose `Open` meets a corrupt index fails `invalid`
with hint `capcom reindex`; `prime` keeps its `; run capcom reindex` warning. The `ErrCorrupt`
mapping goes in `prepareIndex`, not `prepare`.
**Files:** internal/index/index.go, internal/index/index_test.go, internal/cli/root.go, internal/cli/reindex_test.go
**Read first:** internal/index/index.go — Open, dsnOptions; internal/cli/root.go — prepareIndex;
internal/cli/output/errors.go — Error, defaultHints; internal/briefing/briefing.go — readWork;
internal/cli/prime_test.go — TestPrime_GarbageIndex_ExitsZeroWithOneWarningLine; internal/cli/reindex_test.go — newIssueStore
**Tests:** index: garbage bytes over `index.db` → `Open(ModeRefresh)` fails `errors.Is(ErrCorrupt)`,
`Open(ModeRebuild)` succeeds with every issue; `isCorruptIndex` is false for a real `SQLITE_BUSY`
error from a second connection holding `BEGIN EXCLUSIVE` with `_pragma=busy_timeout(0)`, and that
index file survives. CLI: garbage → `capcom list` exit 1 with `hint: capcom reindex` (and `--json`
`error.hint`), `capcom reindex` exit 0, then `capcom list` shows every issue.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/`
- `GOMEMLIMIT=2GiB go test -count=1 -run 'Reindex|Corrupt' ./internal/cli/`
- `go vet ./internal/index/ ./internal/cli/`
**Closes:** reindex-cannot-repair-index
**Commit:** `fix(index): let reindex discard an index SQLite reports corrupt`

## 2. The index sees a content change that keeps size and mtime

**What:**
**Goal:** a node file rewritten with the same byte length and its old mtime restored is re-read on
the next refresh, on Linux, macOS and Windows, without reading file bodies on a clean refresh; `list`,
`show` and `search` then agree on its title.
Fix: `recordedFile.isDirty` compares only size and mtime.
**Approach (assumed at the header base):** the walk in `internal/index/walk.go` (the one producer of
`foundFile.size`/`mtime`) also records `ctime int64` and `identity string` from a per-OS
`changeStamp(path, info)`: `filestat_linux.go` (`Stat_t.Ctim`, `Dev:Ino`), `filestat_darwin.go`
(`Ctimespec`, `Dev:Ino`), `filestat_windows.go` (`windows.CreateFile` with
`FILE_READ_ATTRIBUTES` and `FILE_FLAG_BACKUP_SEMANTICS`, then `GetFileInformationByHandleEx`
`FileBasicInfo` → `ChangedTime`; identity `""`), and `filestat_other.go` (0, `""`). The `files`
table in `schema.go` gains `ctime INTEGER NOT NULL` and `identity TEXT NOT NULL`; bump
`schemaVersion`. Consumers: `recordedFiles` scans them, the `INSERT INTO files` stores them,
`isDirty` compares both, and its racy-clean rule also re-reads when `ctime >= refreshed`. Update
the package comment in `index.go` naming "size or mtime".
**Regression guard.** (a) Windows: `x/sys/windows` v0.46.0 has no `FILE_BASIC_INFO` type, so
`filestat_windows.go` defines a local `fileBasicInfo{CreationTime, LastAccessTime, LastWriteTime,
ChangeTime int64; FileAttributes uint32; _ uint32}` and passes its `unsafe.Sizeof` to
`GetFileInformationByHandleEx`. (b) `changeStamp` returns no error: any failure gives `(0, "")`, so
the file falls back to today's size+mtime test; on Unix it type-asserts `info.Sys().(*syscall.Stat_t)`
with an ok check (`lstat` is a test seam). (c) `internal/ui` `TestRefresh_NoChangeTick_ReloadsNothing`
and `TestRefresh_ConfigDeclaresStatus_ShowsWithoutRebuildLoop` go racily clean under `ctime >=
refreshed` (`backdateTree`'s `Chtimes` sets ctime to now): in those two tests only, wait past the next
whole second and poll once more before the asserted poll; never in `newTestStore` (250 callers);
reword its "never racily clean" comment. (d) Update every comment and doc naming size and mtime as
the change test, found with `grep -rniE 'size (or|and|\+) ?(mtime|modification time)' internal docs/site`.
**Files:** internal/index/walk.go, internal/index/refresh.go, internal/index/schema.go, internal/index/index.go, internal/index/filestat_linux.go, internal/index/filestat_darwin.go, internal/index/filestat_windows.go, internal/index/filestat_other.go, internal/index/refresh_test.go, internal/ui/harness_test.go, internal/ui/refresh_test.go, docs/site/reference/file-format.md
**Read first:** internal/index/refresh.go — recordedFile.isDirty, recordedFiles, racyGranularity;
internal/index/walk.go — walkIssues, lstat; internal/index/schema.go — schemaVersion;
internal/ui/harness_test.go — newTestStore, backdateTree
**Tests:** `refresh_test.go`: index an issue with mtime set in the past, rewrite the title with one
of equal length, `os.Chtimes` back to the old mtime, refresh → the entry has the new title and
`search` finds it. Runs on all three OSes (no skip). `internal/ui`: the two refresh tests above stay
green with their extra second.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/`
- `GOMEMLIMIT=2GiB go test -count=1 -run 'TestRefresh_' ./internal/ui/`
- `GOOS=windows go vet ./internal/index/ && GOOS=darwin go vet ./internal/index/ && GOOS=freebsd go build ./internal/index/`
**Closes:** index-stale-size-mtime
**Commit:** `fix(index): detect a same-size edit that keeps the mtime`

## 3. Lock reads create nothing

**What:**
**Goal:** `lock check`, `lock status`, `prime` and the UI's lock reads create no file or directory
under `.git/capcom/`, on a fresh repo and in every lock state; `lock check` on a read-only locks
directory exits 0 or 3, never 1, for a missing or readable record; a record that does not parse or
carries no token exits 1 as item 6 defines.
Fix: every read takes the write guard, which creates `locks/` and `<key>.lck`.
**Approach (assumed at the header base):** in `internal/lock/lock.go`, `Read` and `Check` call
`read(key, time.Now().UTC())` directly instead of through `withGuard`; `Overview` (via `Read`)
follows. A record is replaced by `nodefile.WriteFileAtomic`'s rename, so a guardless reader sees a
whole old or new record. Writers (`Acquire`, `Touch`, `Release`, `Break`) keep `withGuard`. Update
the doc comments of `Read`, `Check` and `read` ("The caller holds the guard").
**Regression guard.** The Goal's "exits 0 or 3, never 1" holds for a missing or readable record; a
record that does not parse or carries no token exits 1 as item 6 defines, and item 3's Goal and
Tests say so. "Creates no file or directory" means under `.git/capcom/locks/`: `prime` opens the
index, which creates `.git/capcom/` and `index.db`. A guard directory no longer affects reads:
recast or drop `TestOverview_DirectoryAtAForeignGuardBesideAHeldLock_IsOneUnreadableProblem` and
reword `StateUnreadable`/`RecordProblem.State` docs ("record or guard cannot be read"). Build every
read-only case on a record with no `.lck` (`writeLockRecord`, or delete `_main.lck` before the chmod).
**Files:** internal/lock/lock.go, internal/lock/lock_test.go, internal/lock/overview.go, internal/lock/overview_test.go, internal/cli/lock_test.go
**Read first:** internal/lock/lock.go — Read, Check, withGuard, read; internal/lock/overview.go — Overview, StateUnreadable;
internal/cli/lock_test.go — writeLockRecord, TestLockCheck_ExitCodes
**Tests:** lock: `Read`, `Check`, `Overview` on a fresh dir leave no `locks/`; `Read`/`Check` on a
record with no guard file in a read-only `locks/` succeed. CLI: fresh repo,
`lock check`, `lock status`, `prime` → `.git/capcom/locks` does not exist; read-only `locks/` with a
held record written by `writeLockRecord` (no `.lck`) → `lock check` exits 3 (skip on Windows and as
root); a record that does not parse → exit 1 (item 6).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/lock/`
- `GOMEMLIMIT=2GiB go test -count=1 -run Lock ./internal/cli/`
**Closes:** lock-check-writes-file
**Commit:** `fix(lock): read the lock record without taking the guard`

## 4. A resume under a new purpose moves the token file

**What:**
**Goal:** resuming a token under a new purpose leaves exactly one token file of this worktree
holding that token, the new purpose's; a session then naming the old purpose fails `locked`, exit 3
(ADR 0016 amendment of 2026-10-26).
Fix: `writeTokenFile` adds a file per purpose and never removes the others.
**Approach (assumed at the header base):** in `Acquire` (`internal/lock/lock.go`), after
`writeTokenFile(key, token, purpose)` and inside the same `withGuard`, remove every token file of
`key` holding the same token except the new purpose's path, reusing `tokenFilePaths` and
`parseTokenFile` from `internal/lock/token.go` (extend `removeTokenFiles` with an exception path
rather than adding a parallel walk). `capcom start` reaches this through `Acquire`.
**Regression guard.** Run the move only on a resume (`current.Record.Token == token`); a fresh or
taken-over token has no other file to remove, and a fresh acquire must not read (and fail on) the
key's other token files. `docs/site/reference/file-format.md`'s token-file paragraph is restated as
one file per token (ADR 0016 amendment), and `Acquire`'s doc comment updated.
**Files:** internal/lock/lock.go, internal/lock/token.go, internal/lock/token_test.go, internal/cli/lock_test.go, docs/site/reference/file-format.md
**Read first:** internal/lock/lock.go — Acquire; internal/lock/token.go — removeTokenFiles, tokenFilePaths, parseTokenFile, SessionToken;
internal/lock/token_test.go — TestAcquire_Resume_RewritesTheTokenFile; internal/cli/lock_test.go — keptTokenFiles, acquireToken
**Tests:** lock: acquire `alpha`, resume as `beta` with the token → one token file (beta's);
then `SessionToken(key, "", "alpha")` is `""` and `tokenPath(key, "alpha")` is gone, while
`SessionToken(key, "", "beta")` gives the token. CLI: the issue's repro asserts exit 3 on the third call.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/lock/`
- `GOMEMLIMIT=2GiB go test -count=1 -run 'Lock|Start' ./internal/cli/`
**Closes:** lock-purpose-alias-resume
**Commit:** `fix(lock): move the token file when a resume names a new purpose`

## 5. A heartbeat in the future is stale

**What:**
**Goal:** a lock whose heartbeat lies more than one TTL after now is stale, so `lock acquire
--take-stale` takes it; the `locked` error never prints a negative age or heartbeat age.
Fix: staleness is `now - heartbeat > ttl`, negative for a future heartbeat.
**Approach (assumed at the header base):** `read` in `internal/lock/lock.go` sets `IsStale` when
`now.Sub(hb) > ttl || hb.Sub(now) > ttl`; `heldError` clamps `Age` and `HeartbeatAge` to ≥ 0, as
`AgeText` already does. Update `Open`'s and `IsStale`'s doc comments.
**Regression guard.** Every stated definition of stale ("heartbeat older than the TTL") names the
future-heartbeat rule too: `lockLong` in `internal/cli/lock.go`, `docs/site/reference/file-format.md`,
and `docs/site/reference/cli/capcom_lock.md` regenerated with `go run ./cmd/gendocs` (the
`internal/clidoc` reference test fails otherwise). The item supersedes ADR 0005's
decision (`.capcom/knowledge/decisions/adr-0005/adr-0005-decision.md`, stale = "heartbeat is older
than the configured TTL"): add a dated amendment Note
`.capcom/knowledge/decisions/adr-0005/adr-0005-amend-1026.md` (shape as
`.capcom/knowledge/decisions/adr-0016/adr-0016-amend-1026.md`: title "ADR 0005 amendment of
2026-10-26: …", a short body stating the rule, a heartbeat more than one TTL in the future is also
stale, and the clock-skew reason), and a one-clause pointer to it in
`.capcom/knowledge/decisions/adr-0005.md`.
**Files:** internal/lock/lock.go, internal/lock/lock_test.go, internal/cli/lock.go, internal/cli/lock_test.go, docs/site/reference/file-format.md, docs/site/reference/cli/capcom_lock.md, .capcom/knowledge/decisions/adr-0005/adr-0005-amend-1026.md, .capcom/knowledge/decisions/adr-0005.md
**Read first:** internal/lock/lock.go — read, heldError, HeldError.Message, Open; internal/lock/age.go — AgeText;
internal/cli/prime_test.go — ageLockRecord; internal/cli/lock.go — lockLong; internal/clidoc/reference_test.go
**Tests:** lock: record with heartbeat now+2·ttl → `IsStale`; now+ttl/2 → not stale; `HeldError`
ages ≥ 0. CLI: heartbeat patched to 2099 → `lock acquire --purpose me --take-stale` exit 0;
without `--take-stale`, the message contains no `-`-prefixed duration. `internal/clidoc`'s
reference test passes on the regenerated pages.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/lock/`
- `GOMEMLIMIT=2GiB go test -count=1 -run Lock ./internal/cli/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`
- `capcom check`
**Closes:** lock-future-heartbeat-sticks
**Commit:** `fix(lock): treat a heartbeat dated past one TTL ahead as stale`

## 6. A corrupt lock record names its fix

**What:**
**Goal:** `lock check`, `lock acquire`, `lock touch`, `lock release` and `start`, meeting a record
that does not parse or carries no token, fail `invalid`, exit 1, with a message saying the lock
record cannot be read and hint `capcom lock status, then capcom lock break`, the same in `--json`;
`lock status` keeps exit 0 and its `invalid` line; `lock break` keeps clearing the record.
**Approach (assumed at the header base):** export `errCorruptRecord` as `lock.ErrCorruptRecord`
(update `Break` and `Overview`). One helper in `internal/cli/lock.go`,
`corruptLockFailure(err) *output.Error` (message `<path>: the lock record cannot be read: <cause>`,
`Path` set), called from `emitLock`'s error switch and from `start.go`'s lock failure path; no
second copy of the wording. `finish.go`'s note keeps its text.
**Regression guard.** The helper is `corruptLockFailure(path string, err error)`: Message `the lock
record cannot be read: <err text after "<path>: ">`, `Path` = `target.lock.RecordPath()` (since
`output.Error.Error` prefixes `Path` itself), overriding the Approach's signature and message. In
`start`, the call site is `refuseLock`'s `checkErr != nil && !isHeldByOther` return (Check runs before
Acquire), plus `startLockFailure` if Acquire can still meet it. Rename `errCorruptRecord` in
`internal/lock/lock_test.go` and `internal/lock/overview_test.go` too.
**Files:** internal/lock/lock.go, internal/lock/lock_test.go, internal/lock/overview.go, internal/lock/overview_test.go, internal/cli/lock.go, internal/cli/start.go, internal/cli/lock_test.go
**Read first:** internal/lock/lock.go — errCorruptRecord, read; internal/cli/lock.go — emitLock, failedLockResult;
internal/cli/start.go — refuseLock, startLockFailure; internal/cli/output/errors.go — Error.Error;
internal/cli/lock_test.go — TestLockCheck_CorruptRecord_FailsWithExitOne
**Tests:** CLI: record `not json{{{`, then a table over the five commands asserts exit 1, code
`invalid`, the message phrase and the hint in text and `--json`; a token-less `{}` record the same;
`lock status` exit 0. `TestLockCheck_CorruptRecord_FailsWithExitOne`'s pinned
`_main.json: invalid lock record` changes to the new wording.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/lock/`
- `GOMEMLIMIT=2GiB go test -count=1 -run 'Lock|Start' ./internal/cli/`
**Closes:** lock-corrupt-record-hint
**Commit:** `fix(lock): hint lock status and lock break on a corrupt record`

## 7. prime --json says whether this worktree is free

**What:**
**Goal:** `prime --json` data carries `worktree: {key, state, advice}`: this worktree's key, its
state as the text classifies it (`free`, `held`, `stale`, `invalid`, `unreadable`) and `advice`
equal to the `this worktree: …` line the text prints; text output unchanged; `schema` stays 1;
`docs/site/reference/json-output.md` describes the `prime` payload.
**Approach (assumed at the header base):** `primeData` in `internal/cli/prime.go` gains
`Worktree *primeWorktreeData json:"worktree,omitempty"`, filled where the lock section writes
`worktreeLockAdvice(overview.State, hasDoingStatus)`, from the same value. Add a `### prime`
subsection under the envelope section of `json-output.md` listing the data keys.
**Regression guard.** `docs/site/agents/briefing.md`'s "As JSON" table already lists the `prime
--json` data keys: add a `worktree` row there (`{key, state, advice}`, left out when the lock
section failed), and have `json-output.md` link that table rather than restate it.
**Files:** internal/cli/prime.go, internal/cli/prime_test.go, docs/site/reference/json-output.md, docs/site/agents/briefing.md
**Read first:** internal/cli/prime.go — addLocks, worktreeLockAdvice, primeData; internal/lock/overview.go — Overview;
internal/cli/prime_test.go — TestPrime_JSON_CarriesEverySection, TestPrime_GarbageLockRecord_AdvisesItIsInvalid;
docs/site/agents/briefing.md — "As JSON" table
**Tests:** a table over free, held, stale, invalid, unreadable: text and `--json` both carry the
state, and `data.worktree.advice` equals the text line.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run Prime ./internal/cli/`
**Closes:** prime-json-lock-state
**Commit:** `feat(prime): carry this worktree's lock state in --json`

## 8. Tree JSON carries counts, cuts and repeats

**What:**
**Goal:** `list --tree --json` items carry `children: {total, closed}` for a parent and `more: <n>`
where the text prints `... <n> more`; `dep list --json` items carry `repeat: true` where the text
prints `(see above)` and `more: <n>` where it prints a cut line; all omitted when empty; documented
in `docs/site/reference/json-output.md`.
**Approach (assumed at the header base):** in `internal/cli/list.go`, `issueTreeItem` gains
`Children *childrenData` and `More int`, set in `issueTree.add` from the same `progressOf` result
and `row.More` the text uses. In `internal/cli/dep_list.go`, `depListItem` gains `Repeat bool` and
`More int`, set in `depSection.add` under the same conditions as `row.text` and `row.more`.
**Regression guard.** `internal/cli/list_test.go` does not exist: the `list --tree` tests and their
fixture live in `internal/cli/read_test.go`; put the tree JSON test beside
`TestListTree_JSON_ItemsCarryDepth`.
**Files:** internal/cli/list.go, internal/cli/dep_list.go, internal/cli/read_test.go, internal/cli/dep_test.go, docs/site/reference/json-output.md
**Read first:** internal/cli/list.go — issueTree.add, issueTreeItem; internal/cli/dep_list.go — depSection.add, depListItem;
internal/cli/issue_render.go — childrenData, progressOf; internal/outline/outline.go — walkOutline;
internal/cli/read_test.go — TestListTree_JSON_ItemsCarryDepth, newReadStore
**Tests:** `list --tree --depth 1 --json` on a parent with two children (in `read_test.go`, beside
`TestListTree_JSON_ItemsCarryDepth`) → `children` `{2,0}` and `more` 2 matching the text; `dep list --depth 0 --json` on a diamond → the second shared node has
`repeat: true`; a `--depth 1` cut → `more` equals the text's count.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'List|Dep' ./internal/cli/`
**Closes:** tree-json-loses-progress
**Commit:** `fix(cli): keep child counts, cuts and repeats in tree --json`

## 9. Group commands refuse --json

**What:**
**Goal:** `capcom --json`, `capcom know --json`, `capcom dep --json`, `capcom lock --json` (and any
group with no subcommand given) print a JSON usage error `<path> does not support --json` and exit
2; no invocation of a group with `--json` writes non-JSON to stdout with exit 0; `--help` still
prints help.
**Approach (assumed at the header base):** the group body `withSubcommandDispatch` installs in
`internal/cli/root.go` returns `output.UsageErrorf("%s does not support --%s", cmd.CommandPath(),
flagJSON)` when `a.isJSON` and no args are given, before printing help, as `newUICommand`'s `Args`
does. An unknown subcommand keeps its suggestion error.
**Regression guard.** `withSubcommandDispatch` has no `app`: read the flag in the body with
`cmd.Flags().GetBool(flagJSON)` (the inherited persistent flag) so its signature and five callers
stay. The Goal excepts cobra's `completion` and `help` (json-output.md:10-11: they print their
text whatever `--json` says); the test skips them by name. `docs/site/reference/json-output.md`
:3 and :8 name the group commands beside `capcom ui`; if `rootLong` is reworded, regenerate
`docs/site/reference/cli` with `go run ./cmd/gendocs` (pinned by `internal/clidoc`).
`internal/cli/root_test.go` does not exist: the test goes in `internal/cli/cli_test.go`.
**Files:** internal/cli/root.go, internal/cli/cli_test.go, docs/site/reference/json-output.md
**Read first:** internal/cli/root.go — withSubcommandDispatch, newRootCommand; internal/cli/ui.go — newUICommand;
internal/cli/cli_test.go — TestRun_GroupWithoutSubcommand_PrintsHelp, TestRun_UnknownCommandWithJSON_WritesInvalidEnvelopeAndExitsUsage;
internal/cli/help_text_test.go — availableCommands; docs/site/reference/json-output.md — intro paragraph
**Tests:** in `cli_test.go`, walk the tree with `availableCommands`; for the root and every command
with subcommands except `completion` and `help`, run `<path> --json` and assert exit 2 and stdout
parsing as a `kind: error` envelope.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'Root|JSON|Group' ./internal/cli/`
**Closes:** group-json-prints-help
**Commit:** `fix(cli): refuse --json on group commands`

## 10. hooks tests assert capcom's wording, not the decoder's

**What:**
**Goal:** `go test ./internal/hooks` passes with `GOEXPERIMENT=jsonv2` and `GOEXPERIMENT=nojsonv2`;
the syntax-error case asserts path, line and the `invalid settings` prefix only.
Fix: `TestMerge_UnusableContent_FailsWithPathAndLine` pins the v1 decoder's message.
**Approach (assumed at the header base):** in `internal/hooks/settings_test.go` the syntax-error
case compares `Path`, `Line` and `strings.HasPrefix(Message, "invalid settings: ")`; other cases
keep exact matches. Sweep the package's tests for any other decoder wording (`grep -n "invalid character\|looking for" internal/hooks/*_test.go`).
**Regression guard.** The Goal's second clause gets its own Acceptance check: no decoder wording
left in `settings_test.go` and the `HasPrefix` compare present; the `Line` compare stays explicit.
**Files:** internal/hooks/settings_test.go
**Read first:** internal/hooks/settings_test.go — TestMerge_UnusableContent_FailsWithPathAndLine;
internal/hooks/settings.go — settingsFailure, decodeObject, Error; internal/hooks/apogee_test.go — TestMergeApogee_UnusableContent_FailsWithPathAndLine
**Tests:** the changed test itself; the syntax-error case compares `Path`, `Line` (explicitly) and
the `invalid settings: ` prefix.
**Acceptance:**
- `GOEXPERIMENT=jsonv2 GOMEMLIMIT=2GiB go test -count=1 ./internal/hooks/`
- `GOEXPERIMENT=nojsonv2 GOMEMLIMIT=2GiB go test -count=1 ./internal/hooks/`
- `! grep -n "looking for\|invalid character" internal/hooks/settings_test.go && grep -n 'HasPrefix' internal/hooks/settings_test.go`
**Closes:** hooks-test-red-jsonv2
**Commit:** `fix(hooks): assert the refusal's path and line, not decoder prose`

## 11. CI runs -race and gates coverage

**What:**
**Goal:** CI runs `go test -race -count=1 ./...` on Linux and macOS (Windows keeps
`go test -count=1 ./...`); the Linux job fails when total coverage drops below a floor written in
`.github/workflows/ci.yml`; the Makefile `test` target passes `-count=1`.
**Approach (assumed at the header base):** split the Test step by `runner.os`: Linux runs
`go test -race -count=1 -coverprofile=coverage.out ./...` then a step that reads the `total:` line
of `go tool cover -func=coverage.out` and fails below `COVERAGE_FLOOR` (an `env:` value); macOS
`-race` without coverage; Windows unchanged. Set the floor from today's total, measured on the Pi
with `GOMEMLIMIT=2GiB go test -p 1 -count=1 -coverprofile=… ./...` (never `-race`), rounded down to
a whole percent, minus 2. Makefile: `go test -count=1 $(PKG)`.
**Regression guard.** A Linux coverage step below the floor also fails cmd/release's green-CI gate
(checkGreenRun reads the whole ci.yml run) — accepted as intended, no change to cmd/release.
Every doc line naming the CI test command or `make test`'s command (`grep -rn "CI runs\|make test "
CONTRIBUTING.md AGENTS.md .capcom/knowledge`) names `-race`/`-count=1` and the coverage floor. The
host has no PyYAML: Acceptance drops the `python3 yaml` parse and checks each `runner.os` branch.
**Files:** .github/workflows/ci.yml, Makefile, CONTRIBUTING.md, AGENTS.md
**Read first:** .github/workflows/ci.yml — jobs.test (Vet, Test steps); Makefile — test target, PKG;
CONTRIBUTING.md — make targets block, CI paragraph; AGENTS.md — Tests on the Raspberry Pi 5 host;
cmd/release/gate.go — checkGreenRun
**Tests:** none beyond the workflow; record the measured total in a NOTES line under this item.
**Acceptance:**
- `grep -n -- '-race' .github/workflows/ci.yml && grep -n 'COVERAGE_FLOOR' .github/workflows/ci.yml && grep -n -- '-count=1' Makefile`
- `grep -A3 "runner.os == 'macOS'" .github/workflows/ci.yml | grep -- '-race'`
- `grep -A3 "runner.os == 'Windows'" .github/workflows/ci.yml | grep -- 'go test -count=1 ./...' && ! grep -A3 "runner.os == 'Windows'" .github/workflows/ci.yml | grep -q -- '-race'`
- `GOMEMLIMIT=2GiB go test -race -count=1 ./internal/lock/`
**Closes:** ci-race-and-coverage
**Commit:** `ci: run the race detector and gate coverage`
