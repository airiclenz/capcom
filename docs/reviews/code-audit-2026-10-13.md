# Code Review — changes since a88137f (cmd/gendocs, cmd/release, internal/*, release scripts, go.mod) — 2026-10-13

**Scope:** Every file changed since commit a88137f (the 2026-10-12 audit): `cmd/gendocs`, `cmd/release`, all changed `internal/*` packages, `.github/scripts/version-bump.sh`, `.github/workflows/tag-on-version-bump.yml`, `VERSION` and `go.mod`. Changed code was reviewed first; older code in those files was also in scope. Reviewed for intent and architecture, correctness, security and missing critical-path tests. A separate concurrency review was not run, because capcom has no in-process concurrency (no goroutines or `sync` outside tests). `go vet` and `go test` passed, and `-race` was clean on index, issue, tree, lock, store, cli and cmd/release. golangci-lint, staticcheck and govulncheck are not installed, so dependency CVEs were not checked.
**Mission:** capcom is a git-native issue tracker and knowledge store for coding agents. It ships as one cross-platform Go binary and keeps issues and Notes as markdown files under `.capcom/`, versioned with the code, with a disposable SQLite index as a cache.
**Files reviewed:** 251

## Executive Summary

The most important finding is in `config.RaiseFormat`. It finds the `format:` line with a plain-text regexp, so a `config.yaml` that the parser accepts but that spells the key differently (`"format": 2`, or a file starting with a UTF-8 BOM) gets a second `format:` line. After that, every command fails on a duplicate key. The trigger is everyday use: adding a related link or retiring a Note raises the format. Nothing reaches Critical or High, and the codebase is in good shape overall. Coverage is 80–98% across packages, the release tool's gate and publish convergence are sound, and most findings are edge cases or missing tests. Three patterns run through the findings:
- Write-path guarantees weaken at the edges: format raises that land before late failures, a non-atomic first write of `config.yaml`, and the exclusive-create copy fallback.
- A hostile repository can make two parsers do quadratic work.
- The `hooks` installers bypass the guarded `nodefile` read and write helpers that every other working-tree file goes through.

## Intent & Architecture Findings

### Medium — Store format is raised before the write can still fail `[Intent & Structure + Correctness]`

- **Where:** `internal/nodestore/edit.go:75` (same shape at `internal/nodestore/create.go:79-97`)
- **What:** `raise` rewrites `config.yaml` to the newer store format before the last checks that can still fail the write. In Edit those checks are `recheck` (edit.go:79, `*ConflictError` on a concurrent edit) and `place` (`ErrNotExist`). In Create they are the parent `Lstat` (create.go:86) and the exclusive `place` (`ErrExist`).
- **Why it matters:** In a format-1 or format-2 store, two sessions race on `know retire`, `related add` or `capcom edit <issue> --related x`. The loser reports "changed since read, run again", but `config.yaml` has already been raised and no node holds the new key. The store now declares a newer format than any file needs. Older binaries refuse every command, and the user finds an unexpected uncommitted `config.yaml` change.
- **Fix:** Call `raise` immediately before `place`: after `recheck` in Edit and after the parent `Lstat` in Create. Alternatively, roll the raise back when the write fails. The invariant "no newer-format value under an older declared format" still holds, because the raise still runs before the node write. Add a test that uses the `beforeRecheck` seam to change the file mid-edit, then asserts that `config.yaml` still says `format: 1`.

### Medium — `hooks` installers read working-tree files without the `nodefile` guard `[Intent & Structure]`

- **Where:** `internal/hooks/settings.go:137`, `internal/hooks/gitattributes.go:38` (also `internal/cli/node_edit.go:84`, which reads a user-supplied path and is less exposed)
- **What:** `nodefile` documents itself as "the one guarded reader for repository-controlled files" (read.go:97, doc.go:1-2). That reader uses Lstat, O_NOFOLLOW and a 1 MiB cap. `hooks.Install` and `hooks.InstallMergeAttribute` instead read `.claude/settings.json`, `.codex/hooks.json` and `.gitattributes` with plain `os.ReadFile`.
- **Why it matters:** A hostile repository can commit `.gitattributes` or `.claude/settings.json` as a symlink to `/dev/zero` or as a FIFO. `capcom init` then follows it and either reads without a size cap until memory runs out or blocks forever. This is the exact threat the guard exists for.
- **Fix:** Read both files through `nodefile.ReadRegularFile`. Map `ErrNotRegular` and `ErrTooLarge` to the `*Error` that names the path, and keep missing-file detection through `fs.ErrNotExist`. Fix this together with the `Install` write-mode finding below.

### Medium — Exclusive create breaks its "no file or the whole new one" promise on the copy fallback `[Intent & Structure]`

- **Where:** `internal/nodefile/atomic.go:204-229`
- **What:** `WriteFileExclusive` and `placeExclusive` promise "Readers see no file or the whole new one". When the file system refuses hard links (FAT/exFAT, some network or FUSE mounts, non-NTFS Windows volumes), `copyExclusive` creates the file at its final `path` with `O_CREATE|O_EXCL` first and fills it afterwards.
- **Why it matters:** A concurrent reader can see an empty or half-written node file under its real name. Such a reader is another agent session, another worktree, or the index walk. A crash mid-copy leaves a truncated node behind, because the cleanup only runs on a returned error.
- **Fix:** On the fallback, copy to a second temp name and move it into place with a no-replace rename, which keeps the invariant. If that is not possible on every platform, correct the doc comments on `WriteFileExclusive` to state the weaker guarantee and make `Load` tolerate an empty node file.

### Medium — `know update --remove-superseded-by` ignores aliases and silently does nothing `[Intent & Structure]`

- **Where:** `internal/cli/know_update.go:243`
- **What:** `--add-superseded-by` arguments are resolved to canonical ids "so the Note stores canonical ids only" (know_update.go:145). `--remove-superseded-by` arguments go to `edit.List` unresolved.
- **Why it matters:** `capcom know update <note> --remove-superseded-by <alias>` matches nothing, the command reports success, and the stale `superseded-by` entry stays. This breaks the rule that every lookup resolves aliases.
- **Fix:** Resolve removal arguments through the same Note lookup. If an argument no longer resolves, fall back to the literal string so stale entries can still be removed. Otherwise, reject an argument that matches no stored entry.

### Medium — `ReadNodeFile` is a second name for `ReadRegularFile` `[Intent & Structure]`

- **Where:** `internal/nodefile/read.go:141`
- **What:** `ReadNodeFile` is a one-line pass-through that "returns exactly what ReadRegularFile returns". About five callers use it (agents/block.go:146, tree/plan.go:93, tree/remove.go:182, index/walk.go, diagnose). Other callers use `ReadRegularFile` directly (store.go:210, knowledge/verify.go:166, agents/block.go:371).
- **Why it matters:** Two names for one reader contradict the package's "one guarded reader" claim. They also invite the two to drift apart later. Deleting the wrapper removes no complexity anywhere.
- **Fix:** Delete `ReadNodeFile` and switch its callers to `ReadRegularFile`.

## Medium Findings

### Medium — `RaiseFormat` writes a duplicate `format:` key and leaves the store unreadable `[Correctness]`

- **Where:** `internal/config/format.go:14`, `:92` (`realFormatLine` / `findFormatLine`), `:35-36` (`insertLine`)
- **What:** `findFormatLine` looks for the key only with the regexp `^format:`. `Parse` (parse.go:94) accepts other spellings of the same key: a quoted `"format": 2`, `format : 2`, or a first line with a UTF-8 BOM in front, as some Windows editors save it. On those, `RaiseFormat` concludes the format is missing and inserts `format: 3`. With a quoted key the file then holds two `format` keys, and `Parse` rejects it at parse.go:145-146. With a BOM, the new line goes in front of the BOM, which then ends up mid-file.
- **Why it matters:** `store.RaiseFormat` (store/store.go:145-152) runs from `nodestore` `Kind.raise` (kind.go:189). That happens whenever an issue gains a related link or a Note gains `retired` or `superseded-by` (codec.go:76-80, 109-113). It never checks the parsed `Config.Format`, so even a quoted `"format": 3` store gets the duplicate line. After that one ordinary command, every capcom command fails at store open. *(independently verified)*
- **Fix:** Find the `format` key's line from the parsed YAML node instead of a regexp, or strip a leading BOM and allow optional quotes and spaces in `realFormatLine`. If the parser sees a `format` key that the line search cannot place, return an error instead of inserting. Also skip the raise when `Config.Format` is already at or above the target. Add tests for a quoted key, `format : 2`, and a BOM-prefixed file.

### Medium — `release smoke` runs a host binary whose hash did not match `[Correctness]`

- **Where:** `cmd/release/smoke.go:370-387` (download at `:300-314`)
- **What:** `checkAssets` downloads each archive into `workDir` and only then compares its hash. When the hash does not match `SHA256SUMS`, it records a FAIL but leaves the file in place. `checkHostBinary` checks only that the host archive exists in `workDir`, then extracts it and runs it through `checkReportsVersion`.
- **Why it matters:** A corrupt or tampered release asset is exactly what smoke exists to catch. When it happens, `release smoke vX.Y.Z`, or the smoke step at the end of `publish`, runs that unverified binary on the maintainer's machine.
- **Fix:** Record which archives verified in `checkAssets`, or delete any download whose hash does not match. Skip `checkHostBinary` with a clear reason unless the host archive verified. Add a test where the host archive's hash does not match, and assert that nothing is extracted or run.

### Medium — `release smoke` upgrades the Homebrew install when the release lookup fails `[Correctness]`

- **Where:** `cmd/release/homebrew.go:85`
- **What:** `checkHomebrew` skips only when `LatestRelease` succeeds, a release is published, and its tag differs from the one being smoked. When `LatestRelease` returns an error (gh failure, rate limit, network blip) or reports nothing published, the code goes on to `brew update && brew upgrade capcom`. `checkStaleBucket` (scoop.go:158-170) treats both of those cases as skips.
- **Why it matters:** Run `release smoke vOLD` on a Homebrew host while `gh release view` fails briefly. The host's capcom is upgraded to whatever the tap pins, and `checkReportsVersion` then FAILs. The result is a false failure plus an unwanted change to the maintainer's install.
- **Fix:** Treat `err != nil` and "not published" as skips, as `checkStaleBucket` does, and run the upgrade only when `latest.Tag == r.version.String()`. Add a test for each skip case.

### Medium — `Install` makes an existing settings file world-readable or replaces its symlink `[Correctness]`

- **Where:** `internal/hooks/settings.go:148`
- **What:** `Install` writes the merged settings through `nodefile.WriteFileAtomic`. That function forces mode 0644 and replaces a symlink at the path with a regular file. `InstallMergeAttribute` already avoids this by using `ReplaceFileVerbatim` when the file exists.
- **Why it matters:** Run `capcom init` in a repo whose `.claude/settings.json` or `.codex/hooks.json` has mode 0600 (these files often hold `env` tokens). The file becomes world-readable. If the file is a symlink into a dotfiles checkout, the link is cut and the real file never gets the hooks.
- **Fix:** When the file already exists, write the merged bytes with `ReplaceFileVerbatim`, and use `WriteFileAtomic` only for a new file. Add a test that a 0600 file keeps its mode after `Install`.

### Medium — Windows: a save fails while another process has the node file open `[Correctness]`

- **Where:** `internal/nodefile/atomic.go:133-135`
- **What:** `ReadRegularFile` opens files with `os.Open`. On Windows that shares the file for read and write but not for delete. `WriteFileAtomic`'s final `os.Rename` over the path uses `MoveFileEx(MOVEFILE_REPLACE_EXISTING)`, which Windows refuses while any such handle is open. The code returns that error without retrying, and nothing in `internal/` retries.
- **Why it matters:** Several agent sessions or worktrees reading nodes while another session saves one is the stated deployment scenario, and antivirus scanners and indexers hold files open the same way. On Windows, saves then fail at random with "Access is denied". *(independently verified)*
- **Fix:** On Windows, retry the rename a few times with a short backoff when it fails with access-denied or a sharing violation. Add a Windows-only test that holds a read handle while `WriteFileAtomic` runs.

### Medium — Quadratic fence rescan in the issue Log parser, reachable from the git merge driver `[Security]`

- **Where:** `internal/issue/mutate.go:101` (`logBounds` / `scanLogBounds`; reached from `parseLog` at merge.go:117 and `appendLogEntry` at mutate.go:49)
- **What:** `logBounds` rescans the whole file once for each unclosed fence opener and marks only one opener as plain text per pass. An attacker can write a body of N lines like "```x" that never close, because a closing fence may not carry an info string. That costs N passes of N lines each.
- **Why it matters:** `.capcom/` files are untrusted. Take a crafted branch with an issue file of about 200k such lines (about 1 MB). It pins a CPU for minutes in the merge driver (merge_driver.go:98, which parses three versions per merge), so it hangs the victim's `git merge` or `git rebase`. It does the same in `capcom log` on that issue.
- **Fix:** Make the scan single-pass, for example by finding the first fence that never closes and treating everything after it as plain text in one step. Otherwise, cap the number of passes and, at the cap, treat the file as having no Log section. Add a test with a large number of unclosed openers and a time bound.

### Medium — `NameClashes` produces k×(k−1) findings for k files sharing a name `[Security]`

- **Where:** `internal/diagnose/clashes.go:37-48`
- **What:** A loop over every claimant of a shared id or alias sits inside a loop over the same set, with no cap. It emits one formatted Finding for every ordered pair. The only caller, `index/duplicates.go:53`, runs on every index refresh (refresh.go:74). It appends every finding to the warnings, which refresh.go:84-87 then sorts.
- **Why it matters:** A hostile repository with about 3,000 files listing the same alias produces about 9 million warning strings on every command, including `reindex` and `check`. That is enough to exhaust a memory-limited host. The behaviour predates this change set. *(independently verified)*
- **Fix:** Cap the output per name. For example, report each claimant once together with the first other claimant, or the first N, plus "and K more". That keeps the findings and the set of left-out files at O(k). Add a test with many claimants of one name that asserts a linear finding count.

### Medium — A body line like "```inline``` …" hides every conflict marker below it `[Correctness]`

- **Where:** `internal/rawfm/fence.go:40-46`
- **What:** `codeFenceOpener` treats any line that starts with three backticks as a fence opener. Under CommonMark, a backtick fence does not open when its info string contains a backtick. A body line such as "```inline``` is how you quote it" therefore opens a fence that never closes.
- **Why it matters:** `IsCode` then reports every later line as code. `ConflictMarkers` silently skips every git conflict marker below that line, and `IsOpen` reports an unclosed fence. A file with conflict markers passes `capcom check` and the parsers without a warning.
- **Fix:** For a backtick fence, return "" from `codeFenceOpener` when the text after the backtick run contains a backtick. Add a `ConflictMarkers` test with such a line above a conflict block.

### Medium — `init` can leave an empty or truncated `config.yaml` that a re-run will not repair `[Correctness]`

- **Where:** `internal/store/store.go:243-261`
- **What:** `createConfig` creates `config.yaml` with `O_EXCL` and writes the template into it directly, not atomically. When the write or the close fails (disk full, interrupt), the partial file stays and only the error is returned.
- **Why it matters:** A re-run of `init` gets `ErrExist` and reports "nothing to do". An empty file then silently parses as format-1 defaults, and a truncated one may fail strict parsing on every command.
- **Fix:** Remove the file (`os.Remove(path)`) when the write or close fails, or create it through `nodefile.WriteFileAtomicVerbatim` as `RaiseFormat` already does.

### Medium — No test that a `komac update` failure keeps the GitHub token out of the output `[Critical-Path Tests]`

- **Where:** `cmd/release/winget.go:148`
- **What:** When `komac update` fails, the code removes the token from the error with `strings.ReplaceAll(err.Error(), token, redactedToken)`. No test makes `komac update` fail, and nothing in `cmd/release/*_test.go` matches "manifest update:". Only the first-submission path (`komac submit`) is tested.
- **Why it matters:** Every release after the first goes through the update path. A regression there would print a personal token into publish output and CI logs.
- **Fix:** In `channels_test.go`, add a case where the package is listed and the fake runner fails `komac update` while echoing the token. Assert that the winget step fails, that the output names the manifest update, and that the token appears nowhere in the output or the error.

### Medium — No test for the winget listing-read error that chooses `submit` or `update` `[Critical-Path Tests]`

- **Where:** `cmd/release/winget.go:100`
- **What:** `resolveWingetPackage` fails the run with "could not tell whether ... has ..." when the winget-pkgs listing cannot be read while the step is pending. That error is what stops a flaky read from choosing the wrong komac command. No test reaches it.
- **Why it matters:** If this guard regressed, a temporary read failure would make `publish` run `komac submit` for a package that already exists, or `update` for one that does not, which leaves a partial-release state.
- **Fix:** Add a publish test where the probe's first `ListDirs` succeeds and reports the package absent, and the second call errors. Assert that `publish` returns that error and runs no komac command.

### Medium — `check` does not test its AGENTS.md block stage `[Critical-Path Tests]`

- **Where:** `internal/check/agents.go:10`
- **What:** `agentsBlock`, the third stage of `check.Run`, reports a stale Managed block, tangled markers, and an unreadable or oversized AGENTS.md. No test in `internal/check` covers it; `check_test.go` never mentions AGENTS, `CodeAgentsBlock` or a stale block. It is the only stage that turns `*agents.Error` into problems instead of failing.
- **Why it matters:** `capcom check` is the gate for the fmt/check round trip. A regression in this wiring would silently stop reporting stale or broken blocks, or turn them into hard errors.
- **Fix:** Add tests that:
  - a stale block yields one `agents-block` problem at AGENTS.md, at the block's line;
  - tangled markers yield `agents-block` with the error's path and line, and no error return;
  - no AGENTS.md, or one without a block, yields nothing;
  - an AGENTS.md that is not a regular file, or is over 1 MiB, yields `agents-block`.

### Medium — No test for the related-links self-reference rule in `check` `[Critical-Path Tests]`

- **Where:** `internal/check/issues.go:106`
- **What:** The rule that reports a `related` entry naming the issue itself, as `CodeField`, has no test. The `related` tests cover only alias resolution, format-too-old and a missing target.
- **Why it matters:** If this rule broke, `check` would quietly accept self-links. Nothing would fail.
- **Fix:** Write an issue whose `related:` lists its own id, and another that lists its own alias. Assert one `field` problem at the `related` line, with the message "names the issue itself".

### Medium — No test for the merge driver's fallback when a file already contains the Log placeholder `[Critical-Path Tests]`

- **Where:** `internal/issue/merge.go:70`
- **What:** When any version contains the sentinel `capcomLogMergePlaceholder9f4c2e7a1b`, the `bytes.Contains` guard sends the whole file to a plain text merge. Without the guard, `bytes.ReplaceAll` (merge.go:90) would splice the merged Log into whatever line holds the sentinel. No test feeds a version that contains the sentinel.
- **Why it matters:** The sentinel turns up only in a hostile or hand-edited branch, or when an issue body quotes it. If the guard regressed, the merge driver would silently corrupt the issue body.
- **Fix:** Add a `MergeFile` table case where ours, base or theirs contains the sentinel in the body. Assert that the output equals what `textMerge` alone returns, with no Log merging applied.

### Medium — No tests for the verbatim write pair that puts user files back unchanged `[Critical-Path Tests]`

- **Where:** `internal/nodefile/atomic.go:77-92`
- **What:** `WriteFileAtomicVerbatim` and `ReplaceFileVerbatim` exist to write a file back exactly as it was read, keeping CRLF and the old file's permission bits. Neither has a test for a successful write. The only test (atomic_test.go:107) covers `ReplaceFileVerbatim` on a missing path.
- **Why it matters:** If either function were routed through the line-ending conversion or hard-coded 0644, user files such as `.gitattributes` would silently change on rewrite, and no test would fail.
- **Fix:** Add tests that:
  - `ReplaceFileVerbatim` on a CRLF file with mode 0600 leaves the bytes identical and the mode 0600, with no temp file left behind;
  - `WriteFileAtomicVerbatim` writes `a\r\nb` unchanged with the requested permissions;
  - a failing reader leaves the old file intact.

## Recommended Action Order

1. **Fix `RaiseFormat`'s key search, and move the format raise after the last checks in `nodestore`.** These are the same code path. One normal command can currently leave a store unreadable or declaring a format no file needs, so these are the highest-impact fixes and both are small.
2. **Release tool quick wins.** Stop `checkHostBinary` running a binary that failed its hash check, and make `checkHomebrew` skip on lookup errors. Then add the two winget tests, starting with the token-removal test for `komac update`.
3. **Hostile-repository denial of service.** Make `logBounds` single-pass, cap `NameClashes` per name, and tighten `codeFenceOpener`. Each is a self-contained change in one function.
4. **`hooks` installers.** Read through `nodefile.ReadRegularFile`, and use `ReplaceFileVerbatim` for existing files, in one change.
5. **`nodefile` write path.** Add the Windows rename retry and make `createConfig` atomic. The exclusive-create copy fallback needs a design decision first: either fix it with a no-replace rename or document the weaker guarantee.
6. **Test batch.** Cover the `check` AGENTS.md block stage, the related self-link rule, the merge placeholder fallback and the verbatim write pair.
7. **`know update --remove-superseded-by`**: resolve aliases on removal.
8. **Remove `ReadNodeFile`.** This is a candidate for `/improve-codebase-architecture`, which can fold it into a wider cleanup of the `nodefile` read API.

## What Looked Good

- **Release tool:** `cmd/release` is careful on its main paths. Gate peeling, publish convergence by probing the remote, binding the push to the SHA the gate judged, version validation and most failure modes are correct and tested.
- **Index and file reads:** The SQLite index behaves as a true disposable cache with parameterized SQL. The `nodefile` read side (Lstat, O_NOFOLLOW, a re-check after open, 1 MiB cap) does what it claims.
- **Tree, nodestore and config:** `internal/tree` and `internal/nodestore` re-read before writing, check etags and roll back failed moves and removals, at 92–94% coverage. Strict parsing in `internal/config` is consistent.
- **Small packages:** `internal/edit`, `internal/lock`, `internal/agents` and `internal/briefing`, and the CI version-bump workflow with its SHA-pinned actions, showed nothing worth changing.

Leave these areas as they are; the fixes above are local to the edges named.
