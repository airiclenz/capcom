# Code audit 2026-10-13: fixes and missing tests

**Goal:** Fix every finding of the 2026-10-13 code audit and add the critical-path tests it names. The last item archives the review.
**Date:** 2026-10-13
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** ed4c209

**Sources:**
- `docs/reviews/code-audit-2026-10-13.md` (ground truth; item 20 moves it to `docs/reviews/archived/`)
- `AGENTS.md`; Notes `coding-conventions`, `cli-test-harness`, `issue-workflow`, `store-layout`, `docs-layout`

**Ratified design calls (user, 2026-10-13):**
- **Copy fallback:** copy, then no-replace rename (Linux `renameat2` `RENAME_NOREPLACE`, macOS `renamex_np` `RENAME_EXCL`, Windows `MoveFileEx` without `MOVEFILE_REPLACE_EXISTING`); the in-place copy stays only where no such call exists, and the doc comment says so.
- **Clash cap:** one finding per claimant, naming the first other claimant plus "and K more".
- **--remove-superseded-by:** resolve through the Note lookup, fall back to the literal text; no match stays a quiet no-op.
- **Tracking:** one parent issue `code-audit-1013`, filed by item 0, closed at closeout.
- **hooks symlinks:** a symlinked, non-regular or oversized settings or `.gitattributes` file is refused with an `*Error` naming the path; nothing is followed.

**Closes:** code-audit-1013

**Standing requirements:**
- skills: coding-standards
- Raspberry Pi 5: one package per test command with `GOMEMLIMIT=2GiB`; never `go test -race ./...`; never two test commands at once.
- Commit to `main` (the run branch fast-forwards to it); no AI attribution anywhere.
- Never tag, bump `VERSION` or edit a CHANGELOG release heading.
- After a command, flag or help-text change run `go run ./cmd/gendocs` and commit the result.

**Out of scope:**
- `internal/cli/node_edit.go`'s read of a user-supplied path (the user chose it).
- Rename retries outside `nodefile`'s atomic write.
- A wider `nodefile` read-API cleanup beyond deleting `ReadNodeFile`.

**Regression check (2026-10-13, ed4c209):**
- 0: guard folded (count the review's 20 findings, one bullet per `### ` heading)
- 1: guard folded (config_test.go call sites; parse with the store's relative config path)
- 2: guard folded (recast TestEdit_GainsRelatedBelowFormatTwo_RaisesBeforeTheWrite)
- 3: guard folded (hash-mismatch fixture keeps the archive extractable; "did not download" wording kept)
- 4: guard folded; supersedes the P1.40 NOTES in `docs/plans/archived/2026-10-13 - 00 - code-and-architecture-review-fixes-plan.md:935`
- 5: guard folded (isKomacUpdateFailing knob; exact error assertion)
- 6: guard folded (ListDirs-failing wrapper; fake with winget pending)
- 7: guard folded (writer decision: export rawfm's fence rules; bounds asserted through AppendLog)
- 10: guard folded (hooks fifo/symlink test helpers; Windows mode gate)
- 11: guard folded (writer decision: disjoint build tags; recheck the target before each retry); yields to `internal/nodefile/atomic.go` ReplaceFileAtomic doc and `internal/nodestore/write.go:59-62`
- 12: guard folded (writer decision: disjoint build tags, go.mod unchanged; fallback rule, temp cleanup, biting tests, every comment)
- 13: guard folded (writer decision: Lstat first, real-writer seam); yields to `internal/cli/init.go:42` and Init's doc
- 14: guard folded (literal match first; every resolve failure falls back; docs and flag help); supersedes `docs/site/reference/file-format.md:473`, and the `CHANGELOG.md:121` [0.3.0] entry stays as release history
- 18: guard folded (the new test pins the message)
- 19: guard folded (literal sentinel placed to bite; Windows mode gate)

## 0. File the audit's parent issue

**What:**
**Goal:** `.capcom/issues/code-audit-1013.md` exists, open, titled for the 2026-10-13 code audit; its body lists every finding of the review (20) as bullets (severity, one line each, file) and has a `## Acceptance` section: every finding fixed or tested, `go vet ./...` and `go test ./...` pass.
**Approach (assumed at the header base):** `capcom create --id code-audit-1013 --title "Code audit 2026-10-13: fix the findings" --priority 1 --body-file <tmp>`; body self-contained per Note `issue-workflow`. Do not commit the review file here.
**Regression guard.** The review holds 20 findings, not 18: one bullet per `### ` heading of `docs/reviews/code-audit-2026-10-13.md`, so the count is checked against the file.
**Files:** .capcom/issues/code-audit-1013.md
**Read first:** docs/reviews/code-audit-2026-10-13.md — "## Intent & Architecture Findings", "## Medium Findings"; .capcom/knowledge/issue-workflow.md — The body; capcom create --help — --body-file, --priority, --plan
**Tests:** none.
**Acceptance:**
- `capcom show code-audit-1013`
- `capcom check`
**Commit:** `chore(issues): file the 2026-10-13 code audit`

## 1. RaiseFormat finds every spelling of the format key

**What:** Fix: `config.RaiseFormat` matches the key only by `^format:`, so a quoted key, `format : 2` or a BOM-prefixed first line gets a second `format:` line and the store stops opening.
**Goal:** Raising a `config.yaml` whose `format` key is quoted (`"format": 2`, `'format': 2`), spaced (`format : 2`) or behind a UTF-8 BOM rewrites that one line in place; the BOM stays first; a file already at or above the target is returned unchanged; a file whose parsed config has a `format` key the line search cannot place returns an error and is not rewritten.
**Approach (assumed at the header base):** strip a leading BOM before matching the first line and keep it at the file start on output; widen `realFormatLine` to optional single or double quotes and spaces before the colon; `RaiseFormat` parses the data with the package's parser and returns unchanged when the parsed format ≥ version. Signature becomes `RaiseFormat(data []byte, version int) ([]byte, bool, error)`; its one consumer, `store.(*Store).RaiseFormat`, wraps the error with the config path.
**Regression guard.** The three-result signature breaks the existing callers in `internal/config/config_test.go` (:154, :190, :221) and `internal/store/store_test.go:360`: update all four. `format_test.go` is a new file. `config.Parse` needs a path: `Store.RaiseFormat` passes its relative config path (`s.relative(configFile)`, as Open does) into the parse, so a refusal reads `.capcom/config.yaml:<line>: ...`, never `:<line>: ...`.
**Files:** internal/config/format.go; internal/config/format_test.go (new); internal/config/config_test.go; internal/store/store.go; internal/store/store_test.go
**Read first:** internal/config/format.go — RaiseFormat, realFormatLine; internal/config/parse.go — Parse; internal/config/config_test.go — TestRaiseFormat_RewritesOrInsertsOneFormatKey; internal/store/store.go — Store.RaiseFormat, readConfig; internal/store/store_test.go — TestRaiseFormat_FormatOneTemplate_WritesEveryByteConfigRaiseFormatReturns; internal/nodestore/kind.go — Kind.raise
**Tests:** `format_test.go`: table over quoted, single-quoted, `format : 2`, BOM + `format: 2`, already-at-target `"format": 3`, and an unplaceable key; each asserts the exact output bytes and that `Parse` accepts the result. `store_test.go`: a quoted-key store raised through `Store.RaiseFormat` still opens; an unplaceable key fails naming `.capcom/config.yaml:<line>`. The four existing `RaiseFormat` call sites take the new signature.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/config/`
- `GOMEMLIMIT=2GiB go test ./internal/store/`
**Commit:** `fix(config): raise the store format on every spelling of the format key`

## 2. nodestore raises the format only after the last pre-write check

**What:** Fix: `Kind.Edit` raises `config.yaml` before `recheck`, and `Kind.Create` before the parent `Lstat`, so a conflict or missing parent leaves the store declaring a format no file needs.
**Goal:** An Edit that fails its recheck, or a Create whose parent vanished, leaves `config.yaml` byte-identical; a successful write that needs a newer format still raises it before the node file is placed.
**Approach (assumed at the header base):** in `Kind.Edit` move `k.raise` after `k.recheck`; in `Kind.Create` after the parent `os.Lstat`; update the numbered step lists in both doc comments.
**Regression guard.** `TestEdit_GainsRelatedBelowFormatTwo_RaisesBeforeTheWrite` (edit_test.go) reads `config.yaml` in the `beforeRecheck` hook and expects `format: 2`; once the raise follows the recheck it goes red. Recast it in-package: wrap the Kind's `raiseFormat` field to snapshot the node file at raise time (still unchanged) and assert `config.yaml` is raised once Edit returns.
**Files:** internal/nodestore/edit.go; internal/nodestore/create.go; internal/nodestore/edit_test.go; internal/nodestore/create_test.go
**Read first:** internal/nodestore/edit.go — Kind.Edit, Kind.recheck, beforeRecheck; internal/nodestore/create.go — Kind.Create; internal/nodestore/kind.go — Kind.raise; internal/nodestore/edit_test.go — setBeforeRecheck, TestEdit_GainsRelatedBelowFormatTwo_RaisesBeforeTheWrite; internal/nodestore/create_test.go — TestCreate_ParentRemovedBeforeTheWrite_FailsAtTheParentAndCreatesNoDirectory
**Tests:** `edit_test.go`: recast `TestEdit_GainsRelatedBelowFormatTwo_RaisesBeforeTheWrite` as the guard says; in a format-1 store, an edit adding a related link with `setBeforeRecheck` changing the file returns `*ConflictError` and `config.yaml` still says `format: 1`. `create_test.go`: the same assertion when the parent's node file is removed before the Lstat.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
**Commit:** `fix(nodestore): raise the store format only once the write can no longer fail its checks`

## 3. release smoke never runs a host binary that failed its hash

**What:** Fix: `checkAssets` leaves a mismatched download in `workDir` and `checkHostBinary` extracts and runs it.
**Goal:** `release smoke` runs the host binary only when its archive matched `SHA256SUMS`; otherwise the host-binary check reports SKIP naming the archive and why, and nothing is extracted or executed.
**Approach (assumed at the header base):** `checkAssets` records verified archive names on `smokeRun` and deletes any download whose hash does not match; `checkHostBinary` checks that set first.
**Regression guard.** Make the host archive fail its hash by rewriting its linux_amd64 line in `fake.repo.assetContent[checksumsFile]`, leaving the archive extractable so the pre-item tree runs the binary (set `distDir` "" or expect `checkDistSums`' FAIL too). Observe the work dir by building a `smokeRun` directly with `workDir: t.TempDir()`, calling `checkAssets` then `checkHostBinary`, and stating `workDir/host` and the archive. Keep the "the host's archive ... did not download" SKIP for an archive never downloaded (TestSmokeChecks "the release has no SHA256SUMS" pins it); the hash-mismatch case gets its own reason.
**Files:** cmd/release/smoke.go; cmd/release/smoke_test.go
**Read first:** cmd/release/smoke.go — smokeRun.checkAssets, smokeRun.checkHostBinary, extractBinary, runSmoke; cmd/release/smoke_test.go — TestSmokeChecks, runTestSmoke, newSmokeFake; cmd/release/memrepo_test.go — memRepo.DownloadAsset
**Tests:** `smoke_test.go`: host archive whose SHA256SUMS line is wrong (archive intact) → a FAIL for the asset, a SKIP for the host binary with its own reason, and the fake runner never invoked with the binary; a direct `smokeRun` over `t.TempDir()` → no `host` extract and no mismatched archive left. TestSmokeChecks "the release has no SHA256SUMS" stays green unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run Smoke`
**Commit:** `fix(release): never run a smoke host binary whose hash did not match`

## 4. release smoke skips Homebrew when the release lookup fails

**What:** Fix: `checkHomebrew` runs `brew upgrade capcom` when `LatestRelease` errors or reports nothing published.
**Goal:** `checkHomebrew` upgrades only when the latest published release's tag equals the version being smoked; a lookup error and "not published" each report SKIP with a reason, as `checkStaleBucket` does.
**Regression guard.** TestSmokeChecks "the latest release cannot be read" and "no release is published" pin `6 passed, 0 failed, 1 skipped` with Homebrew passing; both become `5 passed, 0 failed, 2 skipped` with a Homebrew SKIP result. `brew list --formula capcom` runs before the lookup, so assert no `brew update` or `brew upgrade capcom` is recorded. This supersedes the P1.40 NOTES in `docs/plans/archived/2026-10-13 - 00 - code-and-architecture-review-fixes-plan.md:935` (brew upgrade kept on a lookup error or nothing published).
**Files:** cmd/release/homebrew.go; cmd/release/smoke_test.go
**Read first:** cmd/release/homebrew.go — smokeRun.checkHomebrew; cmd/release/scoop.go — smokeRun.checkStaleBucket; cmd/release/smoke_test.go — TestSmokeChecks, smokeFake.respond, runTestSmoke; cmd/release/memrepo_test.go — memRepo.LatestRelease
**Tests:** one test per skip case (lookup error, nothing published, other tag) asserting SKIP and no `brew update` or `brew upgrade capcom` recorded; TestSmokeChecks "the latest release cannot be read" and "no release is published" move to `5 passed, 0 failed, 2 skipped`, each with a Homebrew SKIP wantResult.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run 'Smoke|Homebrew'`
**Commit:** `fix(release): skip the Homebrew smoke check when the latest release is unknown`

## 5. Pin the token redaction of a failed komac update

**What:** Test only (`cmd/release/winget.go`, the `komac update` error path).
**Regression guard.** `respondTools` answers every non-submit komac call with success, and `convergeWinget` drops stdout, so add a `fakeChannels` knob (e.g. `isKomacUpdateFailing`) whose error text carries the token from `call.env`, as `submitManifests` does. `describeWinget` already prints the manifest-update line, so assert `result.err` contains "the winget step failed: submit the AiricLenz.Capcom 0.24.11 manifest update" and the sentinel token is in neither `result.err` nor `result.output`.
**Files:** cmd/release/channels_test.go
**Read first:** cmd/release/winget.go — convergeWinget, komacArgs; cmd/release/channels_test.go — fakeChannels, publishFake.respondTools, publishFake.submitManifests, TestPublishWingetSubmitsWithTheTokenInKomacsEnvironment; cmd/release/main_test.go — fakeRunner
**Tests:** package listed; the new `fakeChannels` knob fails `komac update` with an error echoing the token from `call.env`. Assert `result.err` contains "the winget step failed: submit the AiricLenz.Capcom 0.24.11 manifest update" and the token appears in neither `result.err` nor `result.output`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run Winget`
**Commit:** `test(release): pin the token redaction when komac update fails`

## 6. Pin the winget listing-read error

**What:** Test only (`resolveWingetPackage`'s "could not tell whether" error).
**Regression guard.** `memChannelRepos` failures are fixed per (method, repo), and `withFailure` would fail the probe instead. Add, in publish_test.go, a `channelRepos` wrapper embedding `memChannelRepos` that fails `ListDirs` from its second call (pointer counter), passed through `publishOptions.channels`. Base it on `channelReposOf(tapHolding(testFormula), wingetHolding(nil))` with no open pull, so winget is pending and unlisted and `resolveWingetPackage`'s `ListDirs` is the second call.
**Files:** cmd/release/publish_test.go
**Read first:** cmd/release/winget.go — resolveWingetPackage, probeWinget; cmd/release/publish.go — runPublish; cmd/release/publish_test.go — newPublishFake, runTestPublishWith; cmd/release/memchannels_test.go — memChannelRepos.read, memChannelRepos.withFailure; cmd/release/probes_test.go — wingetHolding, channelReposOf
**Tests:** through the guard's wrapper, the probe's first `ListDirs` succeeds and reports the package absent, the second errors; assert `publish` returns that error and runs no `komac` command.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run 'Publish.*Winget|Winget'`
**Commit:** `test(release): pin the winget listing-read failure`

## 7. Issue Log bounds in one linear pass

**What:** Fix: `logBounds` rescans the file once per unclosed fence opener (quadratic), reachable from the git merge driver and `capcom log`.
**Goal:** `logBounds` returns the same bounds as before for every input and runs in linear time: a body of 50,000 unclosed "```x" lines followed by a Log section finishes well under a second.
**Approach (assumed at the header base):** an opener reached outside a fence is a fence only when a closing line for it follows; precompute, from the end, the longest backtick and tilde closer after each line (as `rawfm` closes fences), then scan once, treating an opener with no later closer as plain text. Delete `scanLogBounds`' retry loop.
**Regression guard.** logBounds reuses rawfm's fence opener and closer rules — export what it needs from internal/rawfm/fence.go (add that file to Files) and keep no copy of the fence rules in package issue, so item 9's opener change also moves the Log bounds. The issue tests are `package issue_test` with no export_test.go: state the hand-checked bounds through `AppendLog`'s output (where the entry lands, as in `TestIssue_AppendLog_AddsEntryToLogSection`).
**Files:** internal/issue/mutate.go; internal/issue/mutate_test.go; internal/rawfm/fence.go
**Read first:** internal/issue/mutate.go — logBounds, scanLogBounds, appendLogEntry; internal/rawfm/fence.go — CodeFence.IsCode, codeFenceOpener, closesCodeFence; internal/issue/mutate_test.go — TestIssue_AppendLog_AddsEntryToLogSection; internal/issue/merge.go — parseLog
**Tests:** `mutate_test.go`: the 50,000-line case with a time bound and the found Log section; a table over nested, unclosed and tilde fences asserting, through `AppendLog`'s output, where the entry lands against hand-checked bounds.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/issue/`
**Commit:** `fix(issue): find the Log section in one linear pass`

## 8. NameClashes reports each claimant once

**What:** Fix: k files sharing a name yield k×(k−1) findings on every index refresh.
**Goal:** `diagnose.NameClashes` yields exactly one finding per claimant of a shared name. With two claimants the message is unchanged (`...: also the <kind> of <path>; both are left out`); with k > 2 it names the first other claimant and ends `... and <k-2> more; all are left out`.
**Files:** internal/diagnose/clashes.go; internal/diagnose/clashes_test.go
**Read first:** internal/diagnose/clashes.go — NameClashes, Claim; internal/diagnose/clashes_test.go — TestNameClashes_Claims_FindsEveryClaimantOfASharedName (the "three claimants" case pins 6 findings and must be rewritten to 3); internal/index/duplicates.go — excludeDuplicatesOf, excludeClaims; internal/index/index_test.go — TestRefresh_FileProblems_PinKindsAndWording; internal/index/problems.go — aliasProblems
**Tests:** 3 claimants → 3 findings with the exact message; 1,000 claimants → 1,000 findings; the 2-claimant wording test stays green.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/diagnose/`
- `GOMEMLIMIT=2GiB go test ./internal/index/`
**Commit:** `fix(diagnose): report each claimant of a shared name once`

## 9. A backtick info string with a backtick opens no fence

**What:** Fix: a body line like "```inline``` quoted" opens a fence that never closes and hides every conflict marker below it.
**Goal:** `rawfm` treats a backtick run whose remaining text contains a backtick as plain text, per CommonMark; tilde fences are unchanged.
**Approach (assumed at the header base):** `codeFenceOpener` returns "" for that case.
**Files:** internal/rawfm/fence.go; internal/rawfm/conflict_test.go
**Read first:** internal/rawfm/fence.go — codeFenceOpener, CodeFence.IsCode, closesCodeFence; internal/rawfm/conflict.go — ConflictMarkers; internal/rawfm/conflict_test.go — TestConflictMarkers_FileContent_ReturnsTheMarkerLines; internal/issue/mutate.go — scanLogBounds; docs/site/reference/file-format.md — the "opened by three backticks or three tildes" sentence
**Tests:** `conflict_test.go`: such a line above a conflict block → `ConflictMarkers` reports the markers; a "~~~ a`b" opener still opens.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/rawfm/`
- `GOMEMLIMIT=2GiB go test ./internal/issue/`
**Commit:** `fix(rawfm): a backtick fence never opens on an info string holding a backtick`

## 10. hooks installers read through the guard and keep existing files

**What:** Fix: `hooks.Install` and `hooks.InstallMergeAttribute` read with plain `os.ReadFile`, and `Install` rewrites an existing settings file with mode 0644.
**Goal:** Both installers read through `nodefile.ReadRegularFile`; a symlink, non-regular or over-1-MiB file fails as `*Error` naming the target path; a missing file is created; an existing settings file keeps its bytes outside the merge and its permission bits (a 0600 file stays 0600).
**Approach (assumed at the header base):** map `nodefile.ErrNotRegular`/`ErrTooLarge` to `*Error`; `Install` writes an existing file with `nodefile.ReplaceFileVerbatim`, a new one with `WriteFileAtomic`, as `InstallMergeAttribute` does. Update both doc comments.
**Regression guard.** The hooks tests (`package hooks_test`) must still compile on Windows: put `makeFIFO` in `internal/hooks/fifo_unix_test.go` / `fifo_windows_test.go` as nodefile does, and a `makeSymlink` that skips on Windows and as root (as `internal/nodefile/helpers_test.go`). Gate the 0600 assertion off Windows, as `internal/cli/init_test.go:689` does. At base the FIFO case hangs in `os.ReadFile` instead of failing.
**Files:** internal/hooks/settings.go; internal/hooks/gitattributes.go; internal/hooks/settings_test.go; internal/hooks/gitattributes_test.go; internal/hooks/fifo_unix_test.go; internal/hooks/fifo_windows_test.go; internal/hooks/helpers_test.go
**Read first:** internal/hooks/settings.go — Install, Error; internal/hooks/gitattributes.go — InstallMergeAttribute; internal/nodefile/read.go — ReadRegularFile, ErrNotRegular, ErrTooLarge; internal/nodefile/atomic.go — ReplaceFileVerbatim; internal/cli/init_test.go — TestInitClaudeHooks_UnusableSettings_FailWithPath; internal/nodefile/fifo_unix_test.go — makeFIFO; internal/nodefile/helpers_test.go — makeSymlink
**Tests:** per installer: a symlink (skipped on Windows and as root) and a FIFO (Unix, through `makeFIFO`) → `*Error` with the path; `settings_test.go`: a 0600 file keeps 0600 after `Install` (asserted off Windows only).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/hooks/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run Init`
**Commit:** `fix(hooks): read agent settings through the guard and keep an existing file's mode`

## 11. Windows: retry the atomic rename while a reader holds the file

**What:** Fix: on Windows `WriteFileAtomic`'s final `os.Rename` fails with access denied while another process has the file open.
**Goal:** On Windows the replace-rename in `nodefile` retries on `ERROR_ACCESS_DENIED` and `ERROR_SHARING_VIOLATION` up to 5 attempts with backoff 10, 20, 40, 80 ms, then returns the last error; other platforms rename once.
**Approach (assumed at the header base):** a `replaceRename` helper in `rename_windows.go` / `rename_other.go` used by `writeFileAtomic`.
**Regression guard.** build tags stay disjoint — rename_windows.go `windows`, rename_other.go `!windows`. When `isReplacing`, rerun `checkReplaceTarget` before each retry and return its `fs.ErrNotExist` error, so a node another session moved between attempts is not recreated. The item yields to `ReplaceFileAtomic`'s documented promise (`internal/nodefile/atomic.go` doc, relied on by `internal/nodestore/write.go:59-62`): a path missing just before the rename fails with `fs.ErrNotExist`.
**Files:** internal/nodefile/atomic.go; internal/nodefile/rename_windows.go; internal/nodefile/rename_other.go; internal/nodefile/atomic_windows_test.go
**Read first:** internal/nodefile/atomic.go — writeFileAtomic, checkReplaceTarget, ReplaceFileAtomic; internal/nodefile/syncdir_windows.go — syncDir; internal/nodestore/write.go — Kind.place; internal/tree/plan.go — Plan.moveFile; internal/nodefile/atomic_test.go — TestReplaceFileAtomic_MissingPath_FailsNotExistCreatingNothing, dirEntries
**Tests:** `atomic_windows_test.go`: hold a read handle, release it after 30 ms in a goroutine, assert `WriteFileAtomic` succeeds with the new content; hold a read handle, remove the path before the retry, assert `ReplaceFileAtomic` fails wrapping `fs.ErrNotExist` and no file is at the path.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodefile/`
- `GOOS=windows go vet ./internal/nodefile/`
**Commit:** `fix(nodefile): retry the replacing rename on Windows while a reader holds the file`

## 12. Exclusive create keeps its promise without hard links

**What:** Fix: `copyExclusive` creates the final path first and fills it after, so readers can see a partial node file and a crash leaves one.
**Goal:** When hard links are refused, `WriteFileExclusive` places the file with a no-replace rename, so readers see no file or the whole one and an existing path fails with `fs.ErrExist`; the in-place copy runs only where the platform has no no-replace rename or the file system refuses it, and the doc comments on `WriteFileExclusive` and `placeExclusive` say exactly that.
**Approach (assumed at the header base):** `noReplaceRename` in `rename_noreplace_linux.go` (`unix.Renameat2`), `_darwin.go` (`unix.RenameatxNp`), `_windows.go` (`windows.MoveFileEx`, no replace flag), `_other.go` (unsupported); map exists errors to `fs.ErrExist`. `placeExclusive` tries link → rename the written temp (the caller's temp cleanup tolerates its absence) → `copyExclusive`. Add a seam beside `SwapLink`.
**Regression guard.** build tags stay disjoint — rename_noreplace_linux.go, _darwin.go, _windows.go by file suffix, rename_noreplace_other.go `!linux && !darwin && !windows`; golang.org/x/sys v0.37.0 already has Renameat2, RenameatxNp/RENAME_EXCL and MoveFileEx, so go.mod does not change. Any no-replace-rename failure other than "exists" falls back to `copyExclusive`; only "exists" becomes `fs.ErrExist` (on Windows pass the path through os's long-path handling or rely on that fallback).
After a rename placement `WriteFileExclusive` ignores `fs.ErrNotExist` from removing temp (or `placeExclusive` reports it consumed temp); today any remove error fails as "written, but removing the temp file". A seam counts no-replace-rename calls; the two existing `...WithoutHardLinks...` tests assert 1 call and are renamed. Update every comment in internal/nodefile naming the copy fallback or link-then-copy (`grep -n 'copy fallback\|O_EXCL\|copies the temp\|hard link' internal/nodefile/*.go`), not only `WriteFileExclusive` and `placeExclusive`.
**Files:** internal/nodefile/atomic.go; internal/nodefile/rename_noreplace_linux.go; internal/nodefile/rename_noreplace_darwin.go; internal/nodefile/rename_noreplace_windows.go; internal/nodefile/rename_noreplace_other.go; internal/nodefile/export_test.go; internal/nodefile/atomic_test.go
**Read first:** internal/nodefile/atomic.go — WriteFileExclusive, placeExclusive, copyExclusive, link, SwapLink; internal/nodefile/export_test.go — SetLink; internal/nodefile/atomic_test.go — refuseHardLinks; internal/nodestore/write.go — Kind.place
**Tests:** the existing `TestWriteFileExclusive_FreePathWithoutHardLinks_...` and `TestWriteFileExclusive_PathExistsWithoutHardLinks_...` are renamed and assert one no-replace-rename call (whole file placed, no temp left; `fs.ErrExist`, old bytes kept); link and rename both unsupported → copy fallback still writes the file; link and rename both unsupported + existing path → `fs.ErrExist`, old bytes kept.
Depends on 11.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodefile/`
- `GOOS=windows go vet ./internal/nodefile/` and `GOOS=darwin go vet ./internal/nodefile/`
**Commit:** `fix(nodefile): place an exclusive file by no-replace rename when hard links are refused`

## 13. init writes config.yaml atomically

**What:** Fix: `createConfig` fills an `O_EXCL` file in place, so a failed write leaves an empty or truncated `config.yaml` that a re-run of `init` will not repair.
**Goal:** `config.yaml` is either absent or the whole template after `createConfig`, including when the write fails; an existing file is still left untouched and reported unwritten.
**Approach (assumed at the header base):** write through `nodefile.WriteFileExclusive`, mapping `fs.ErrExist` to `false, nil`; a package-level seam for that call lets a test fail it.
**Regression guard.** createConfig first Lstats config.yaml and returns false, nil when anything is at the path, so a re-run of init on a complete store whose .capcom is not writable stays a no-op (keeping internal/cli/init.go's "An existing store is left as it is." and Init's "a re-run is a no-op"); fs.ErrExist from nodefile.WriteFileExclusive still maps to false, nil for a file that appears between the Lstat and the write. The test seam calls the real WriteFileExclusive with a reader that fails partway through the template (store tests never run in parallel, so a package variable is safe); assert an error naming config.yaml, no config.yaml, no temp file, and that a second Init writes the full template. Add a Unix test (skipped as root) that Init on a complete store with a read-only .capcom succeeds.
Depends on 12.
**Files:** internal/store/store.go; internal/store/store_test.go
**Read first:** internal/store/store.go — Store.createConfig, Init; internal/nodefile/atomic.go — WriteFileExclusive, writeTemp, placeExclusive; internal/store/store_test.go — TestInit_ReRun_IsNoOpAndKeepsConfig, TestInit_SymlinkedConfig_FailsNamingIt; internal/cli/init.go — newInitCommand
**Tests:** the seam's reader fails partway through the template → error naming `config.yaml`, no `config.yaml` and no temp file; a second `Init` then writes the full template. Unix only, skipped as root: `Init` on a complete store whose `.capcom` is read-only succeeds.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/store/`
**Commit:** `fix(store): write a new config.yaml atomically`

## 14. know update --remove-superseded-by resolves aliases

**What:** Fix: removal arguments go to `edit.List` unresolved, so an alias removes nothing and the command reports success.
**Goal:** `capcom know update <note> --remove-superseded-by <arg>` removes entries equal to `<arg>` resolved through the same Note lookup `--add-superseded-by` uses, or equal to `<arg>` as given; an argument that resolves to nothing is matched literally, and one that matches no entry is a quiet no-op.
**Regression guard.** Match the literal text first: an argument equal to a stored entry removes only that entry; resolve through `ResolveNote` only when the literal matches nothing. Every resolve failure other than an index read error (not found, ambiguous, duplicate, left-out claim) falls back to the literal text, and no match is still a quiet no-op — never `resolveSuccessors`' refusal. Update the flag help ("a superseded-by id to remove") and `docs/site/reference/file-format.md:473`, then run `go run ./cmd/gendocs`. This supersedes file-format.md:473's "removes an entry by its text"; the `CHANGELOG.md:121` [0.3.0] entry is release history and stays.
**Files:** internal/cli/know_update.go; internal/cli/know_update_test.go; docs/site/reference/file-format.md; docs/site/reference/cli/capcom_know_update.md
**Read first:** internal/cli/know_update.go — applyNoteUpdate, resolveSuccessors, newKnowUpdateCommand; internal/index/notes.go — Index.ResolveNote; internal/index/query.go — resolveID, unusableClaim; internal/cli/show.go — resolveFailure;
  internal/cli/know_update_test.go — TestKnowUpdate_RetireAndSupersededBy_WriteThenClearTheKeys; internal/clidoc/reference_test.go — TestReferenceUpToDate
**Tests:** remove by alias; remove by unique id prefix; remove a stale literal entry whose Note no longer exists; `[release, release-steps]` with Note `release` deleted minus `release` keeps `[release-steps]`; an ambiguous argument (`release-a`/`release-b` present) is a quiet no-op, exit 0; each asserts the file's `superseded-by` afterwards.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run KnowUpdate`
- `GOMEMLIMIT=2GiB go test ./internal/clidoc/`
**Commit:** `fix(cli): resolve aliases in know update --remove-superseded-by`

## 15. ReadNodeFile removal, part 1: callers outside nodestore

**What:** Switch every `nodefile.ReadNodeFile` call outside `internal/nodestore` and `internal/nodefile` to `nodefile.ReadRegularFile` with the same limit; behaviour unchanged.
**Goal:** `grep -rn ReadNodeFile internal --include=*.go | grep -v -e internal/nodestore/ -e internal/nodefile/` prints nothing.
**Files:** internal/agents/block.go; internal/tree/plan.go; internal/tree/remove.go; internal/index/walk.go; internal/index/refresh.go; internal/cli/update.go; internal/diagnose/diagnose.go; internal/check/problem.go
**Read first:** internal/nodefile/read.go — ReadRegularFile, ReadNodeFile; internal/index/refresh.go — readNodeFile seam var (index_test.go TestRefresh_FileGoneBeforeItsRead_IsDroppedAndNotParsed swaps it); internal/index/walk.go — orphanDirectoryClaims; internal/agents/block.go — topNotes;
  internal/tree/plan.go — checkUnchanged; internal/tree/remove.go — removeFileStep; internal/cli/update.go — app.readIssueFile
**Tests:** none new.
**Acceptance:**
- `go build ./...` and `go vet ./...`
- `GOMEMLIMIT=2GiB go test ./internal/tree/` then `./internal/index/` then `./internal/agents/`
**Commit:** `refactor: read node files through ReadRegularFile outside nodestore`

## 16. ReadNodeFile removal, part 2: nodestore and the deletion

**What:** Switch `internal/nodestore` to `ReadRegularFile`, delete `ReadNodeFile`, and fix every comment naming it (`grep -rn ReadNodeFile internal`).
**Goal:** `grep -rn ReadNodeFile internal` prints nothing; `nodefile`'s doc names `ReadRegularFile` as the one guarded reader.
Depends on 15.
**Files:** internal/nodestore/refs.go; internal/nodestore/format.go; internal/nodestore/edit.go; internal/nodestore/kind.go; internal/nodestore/refs_test.go; internal/nodefile/read.go; internal/nodefile/doc.go; internal/nodefile/read_test.go; internal/nodefile/read_unix_test.go
**Read first:** internal/nodefile/read.go — ReadNodeFile, ReadRegularFile; internal/nodefile/read_test.go — TestReadNodeFile_Missing_IsErrNotExist, testRefusedSentinelInPathError; internal/nodefile/read_unix_test.go — TestReadNodeFile_SwappedBeforeOpen_RefusedNotRegular;
  internal/nodestore/kind.go — ReadAliases, isRefusedRead; internal/nodestore/refs.go — planNodeMove; internal/nodestore/refs_test.go — unreadSwaps
**Tests:** `ReadNodeFile`'s tests are folded into `ReadRegularFile`'s where they assert something those do not.
**Acceptance:**
- `go build ./...` and `go vet ./...`
- `GOMEMLIMIT=2GiB go test ./internal/nodefile/` then `./internal/nodestore/`
**Commit:** `refactor(nodefile): delete ReadNodeFile`

## 17. Pin check's AGENTS.md block stage

**What:** Test only (`internal/check/agents.go`, `agentsBlock`).
**Files:** internal/check/agents_test.go
**Read first:** internal/check/agents.go — checker.agentsBlock; internal/agents/block.go — Sync, findBlock, Error, BeginMarker, EndMarker; internal/check/check_test.go — newRepo, runCheck; internal/check/fifo_unix_test.go — makeFIFO (skips on Windows)
**Tests:** a stale block → one `agents-block` problem at AGENTS.md at the block's line; tangled markers → `agents-block` with the error's path and line and no error return; no AGENTS.md, or none with a block → nothing; a non-regular or over-1-MiB AGENTS.md → `agents-block`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/check/`
**Commit:** `test(check): pin the AGENTS.md block stage`

## 18. Pin check's related self-reference rule

**What:** Test only (`internal/check/issues.go`, the self-reference rule).
**Regression guard.** `TestRun_OneFixturePerProblemClass_ReportsPathLineAndCode` (check_test.go) already pins the path, line and code for both the own id and the own alias; the new test pins the message `related %q names the issue itself` (`internal/check/issues.go:131`) for those cases, and its fixtures keep the `format: 2` config.
**Files:** internal/check/related_test.go
**Read first:** internal/check/issues.go — checker.checkRelated, checkIssue; internal/issue/deps.go — DepGraph.Resolve; internal/check/check_test.go — TestRun_OneFixturePerProblemClass_ReportsPathLineAndCode, issueFile, runCheck, newRepo; internal/check/problem.go — CodeField
**Tests:** an issue whose `related:` lists its own id, and one listing its own alias (with `.capcom/config.yaml` `format: 2`) → one `field` problem whose message is `related "<name>" names the issue itself`; the location stays pinned by the existing check_test.go cases.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/check/`
**Commit:** `test(check): pin the related self-reference rule`

## 19. Pin the merge driver's placeholder fallback and the verbatim writes

**What:** Test only (`internal/issue/merge.go` sentinel guard; `nodefile.WriteFileAtomicVerbatim`, `ReplaceFileVerbatim`).
**Regression guard.** A sentinel on a bare non-entry Log line already trips `!isEntryOnly`, so the test would not bite. Spell the literal `capcomLogMergePlaceholder9f4c2e7a1b` in the test (`logPlaceholder` is unexported; tests are `package issue_test`) and put it in the body or inside an entry's text, with both sides' Logs changed, so the entry merge and `fakeTextMerge` give different output. Gate the 0600 mode assertion with `runtime.GOOS != "windows"`, as `atomic_test.go:71` does.
**Files:** internal/issue/merge_test.go; internal/nodefile/atomic_test.go
**Read first:** internal/issue/merge.go — MergeFile, logPlaceholder, parseLog; internal/issue/merge_test.go — fakeTextMerge, TestMergeFile_TextMergeFails_ReturnsItsError; internal/nodefile/atomic.go — ReplaceFileVerbatim, WriteFileAtomicVerbatim; internal/nodefile/atomic_test.go — failingReader, dirEntries
**Tests:**
- `merge_test.go`: `MergeFile` cases with the literal sentinel in the body or an entry's text of ours, base or theirs, both sides' Logs changed → output equals `fakeTextMerge` alone.
- `atomic_test.go`: `ReplaceFileVerbatim` on a CRLF 0600 file keeps bytes and mode (mode asserted off Windows only), no temp left; `WriteFileAtomicVerbatim` writes `a\r\nb` unchanged with the given mode; a failing reader leaves the old file intact.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/issue/`
- `GOMEMLIMIT=2GiB go test ./internal/nodefile/`
**Commit:** `test: pin the merge placeholder fallback and the verbatim writes`

## 20. Closeout: archive the review

**What:**
**Goal:** the review lives at `docs/reviews/archived/code-audit-2026-10-13.md`, tracked by git, and `docs/reviews/code-audit-2026-10-13.md` no longer exists.
**Approach (assumed at the header base):** the review is untracked: `mv` it into `docs/reviews/archived/` and `git add` it. Update this plan's Sources path.
Depends on 0–19.
**Files:** docs/reviews/code-audit-2026-10-13.md; docs/reviews/archived/code-audit-2026-10-13.md
**Read first:** docs/reviews/archived — code-audit-2026-10-12.md (tracked precedent); docs/plans/2026-10-13 - 01 - code-audit-fixes-plan.md — Sources
**Tests:** none.
**Acceptance:**
- `test -f docs/reviews/archived/code-audit-2026-10-13.md && ! test -e docs/reviews/code-audit-2026-10-13.md`
- `git ls-files --error-unmatch docs/reviews/archived/code-audit-2026-10-13.md`
**Commit:** `docs(reviews): archive the 2026-10-13 code audit`
