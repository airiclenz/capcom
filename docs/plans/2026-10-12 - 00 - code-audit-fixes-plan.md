# Code-audit fixes — implementation plan

**Goal:** Fix the 16 medium findings of the 2026-10-12 code audit: non-regular node files, duplicate and prefix resolution, the `AGENTS.md` rewrite, six narrow defects, the nodestore write race, and five test gaps.
**Date:** 2026-10-12
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** a91fa31
**Sources:** `docs/reviews/code-audit-2026-10-12.md`; `docs/skill-runs/code-audit/2026-10-12/summary.md`; `docs/format.md`; Notes `coding-conventions`, `error-messages`, `cli-test-harness`, `store-layout`.
**Ratified design calls** (user, 2026-10-12):
- **Duplicates:** `tree.Load` keeps its first-file-wins rule; every mutating command (rm, mv, update, close, know rm, know mv, create/know add with --parent, and any other nodestore write) refuses an id or alias that a duplicate warning names — as target, new id, or parent — with exit 1 and an error naming both files relative to the repo, never wrapping ErrNotFound. (user, 2026-10-12; supersedes the earlier 'drop all' call)
- **Prefix resolution:** a name whose exact id or alias exists only on an excluded node returns a not-found or duplicate error for every command; prefix matching otherwise stays as `docs/format.md` documents.
- **Non-regular files:** every non-regular entry under `.capcom/`, symlinks included, is skipped with a `not a regular file` warning; store reads are capped at 1 MiB and an over-cap file is warned and skipped.
- **Write race:** `Create` places its file with an exclusive create; `Edit` re-reads and re-checks the etag immediately before the rename. No OS lock.
- **elsewhere.days:** config rejects values above 36500 with a file:line error; `docs/format.md` states the bound.
- **version-bump.sh test:** an ADR 0008 amendment Note permits one hermetic local git+bash test of `.github/scripts/version-bump.sh` in cmd/release — no network, no gh.

**Regression check (2026-10-12, a91fa31):**
- 1: guard folded
- 2: guard folded
- 3: guard folded
- 4: guard folded
- 5: recast twice (user call: Load unchanged, writes refuse); writer review after the round limit put the refusal in `tree.Resolve` (its only callers are nodestore's writes), left `Edit` to item 6 and dropped the `show foo` pin
- 6: guard folded (duplicate case errors as item 5; only an invalid file wraps ErrNotFound)
- 7: guard folded
- 8: guard folded
- 10: guard folded; supersedes the Verify and checkRef doc comments
- 11: guard folded
- 12: guard folded
- 13: guard folded
- 14: guard folded
- 15: guard folded
- 16: recast, then guard folded; supersedes adr-0008-decision.md "tests never run git"
- 17: guard folded
- 18: guard folded

**Out of scope:** the `tree`/`index` duplicate-detection merge; low/nit findings; version, tag or CHANGELOG release-heading changes.
**Standing requirements:**
- skills: coding-standards
- Single packages with `GOMEMLIMIT=2GiB`; no `-race` on `./...`; one test command at a time; symlink, FIFO and permission tests skip on Windows and as root; errors and warnings name the file; deviations go on dated NOTES lines.

## 1. Store reads reject non-regular files and cap size; `tree.Walk` skips them — ✅ DONE (2026-10-12)

NOTES (2026-10-12): consequential edit — docs/format.md: made necessary by tree.Walk now leaving out non-regular entries and node files over 1 MiB (the tree rules listed only dotfiles and non-.md files as skipped)
NOTES (2026-10-12): a child directory beside a node file over 1 MiB is also left out as an orphan directory, worded `its node file x.md is larger than 1 MiB` — the plan spelled out only the non-regular case; the local `invalidIDs` set became `leftOutIDs` (id to reason) and `invalidIDDirectoryError` gained a `reason` field (name kept)
NOTES (2026-10-12): per the ratified "every non-regular entry" call, a non-regular entry is warned whatever its name (a symlink `notes-link` warns too), while a regular non-.md file is still skipped silently; new kind/code value is `not-regular` for both sentinels

**What:** Audit finding: non-regular node files are stat'd as links but read through (tree half).
**Regression guard.** `tree.walk` skips, warning at the path, every non-regular entry and every `.md` over 1 MiB by Lstat (`file is larger than 1 MiB`); `ReadNodeFile` keeps its own checks. Add `index.WarningNotRegular`, `check.CodeNotRegular`; map both sentinels in `check.treeCode`, `index.walkWarningKind`, `cli.treeProblemKindOf` (fix walk.go's "only two problems" comment). Track non-regular names like `invalidIDs`: `x/` beside a non-regular `x.md` warns "its node file x.md is not a regular file" (`ErrOrphanDirectory`). Errors are `*fs.PathError`s.
"Not regular" is a bit of `ModeSymlink|ModeDir|ModeNamedPipe|ModeSocket|ModeDevice|ModeCharDevice`, never `ModeIrregular`. Tests get FIFOs only from a `//go:build unix` helper whose windows twin calls `t.Skip`.
**Goal:** `tree.ReadNodeFile(path) ([]byte, error)` (Lstat, open, `File.Stat` re-check, `io.LimitReader` at 1 MiB+1) refuses a non-regular file and one over 1 MiB with `ErrNotRegular`/`ErrTooLarge` naming the path; `tree.Walk`/`tree.Load` skip non-regular entries with a `not a regular file` warning.
**Files:** internal/tree/read.go, internal/tree/tree.go, internal/tree/tree_test.go, internal/tree/read_test.go, internal/tree/fifo_unix_test.go, internal/tree/fifo_windows_test.go, internal/check/check.go, internal/index/index.go, internal/index/walk.go, internal/cli/errors.go
**Read first:** internal/tree/tree.go — treeWalker.walk, Walk, Load, invalidIDDirectoryError; internal/index/walk.go — walkWarningKind; internal/check/check.go — treeCode;
internal/cli/errors.go — treeProblemKindOf; internal/nodestore/remove.go — removalFailure
**Tests:** `read_test.go`: a regular file reads; a symlink, FIFO, directory and 1 MiB+1 file fail with the sentinel in a `*fs.PathError`. `tree_test.go`: `x.md -> /dev/zero`, FIFO `y.md` and 1 MiB+1 `z.md` load without hanging, absent, each warned once; `x/` beside the symlinked `x.md` warns as above.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `GOMEMLIMIT=2GiB go test ./internal/check/ ./internal/index/`
- `go vet ./internal/tree/ ./internal/check/ ./internal/index/ ./internal/cli/`
**Commit:** `fix(tree): skip non-regular node files and cap store reads`

## 2. Index walk and refresh skip non-regular files instead of reading through them — ✅ DONE (2026-10-12)

NOTES (2026-10-12): internal/index/walk.go needed no change — item 1 already routed both skips through tree.Walk/walkWarningKind, so walkIssues gets one warning per path with no second Lstat filter; only refresher.indexFile changed (tree.ReadNodeFile, its ErrNotRegular/ErrTooLarge become a stored WarningNotRegular problem, never a refresh failure)
NOTES (2026-10-12): the symlink-to-/dev/zero and FIFO cases went into a new TestRefresh_NonRegularNodeFiles_WarnOnceEachAndIndexTheRest (with the 1 MiB+1 file, a directory z.md and a regular sibling) rather than into TestRefresh_EachProducer_…, because their helpers skip on Windows and as root and would skip the whole producer test there; EachProducer gained the portable oversize file and directory z.md cases instead
NOTES (2026-10-12): added TestIndexFile_FileGrownPastTheLimitSinceTheWalk_WarnsAndKeepsItOut, an in-package test of indexFile's guarded-read refusal path, which the walk otherwise shields from every refresh-level test

**What:** Same finding, index half. Depends on item 1.
**Regression guard.** The index walk gets both skips from `tree.Walk`/`walkWarningKind` (item 1), one warning per path; no second Lstat filter in `walkIssues`. `indexFile` reads with `tree.ReadNodeFile`; its sentinels become warnings, never a refresh failure. A directory `z.md` keeps its `orphan-directory` warning.
**Goal:** a non-regular file named like a node never reaches `os.ReadFile` in `internal/index`, never fails or hangs a refresh, and yields one `not a regular file` warning; a file over 1 MiB is warned and skipped.
**Files:** internal/index/walk.go, internal/index/refresh.go, internal/index/index_test.go, internal/index/fifo_unix_test.go, internal/index/fifo_windows_test.go
**Read first:** internal/index/walk.go — walkIssues, walkWarningKind; internal/index/refresh.go — refresher.sync, refresher.indexFile, refresher.rewarn;
internal/index/index_test.go — TestRefresh_EachProducer_SetsTheWarningKindAndReplaysIt, TestRefresh_BadNames_WarnAndSkip
**Tests:** `index_test.go`: an issue file symlinked to `/dev/zero`, a FIFO one and a 1 MiB+1 one → refresh succeeds, lists none, one `WarningNotRegular` each (extend `TestRefresh_EachProducer_…`); a directory `z.md` gets orphan-directory; a regular sibling indexes.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `go vet ./internal/index/`
**Commit:** `fix(index): warn on non-regular store files instead of reading them`

## 3. `check`, `update` and `nodestore` read store files through the guarded reader — ✅ DONE (2026-10-12)

NOTES (2026-10-12): `findNodeFile` skips non-regular entries with its own mode test (`entry.Type()&^fs.ModeIrregular != 0`), matching `tree.Walk`'s rule, because tree's `isNotRegular` is unexported and the tree package is outside this item's files.
NOTES (2026-10-12): `Format` keeps its documented contract and fails with a `*FileError` on a file `tree.ReadNodeFile` refuses (a file swapped after the walk); `ReadAliases`/`RecoverAliases` return no aliases and no error for one, and check's alias reader records a `not-regular` problem and leaves the file unparsed. `FileError` has no Unwrap.
NOTES (2026-10-12): `readIssueFile` also serves `show`, `close` and `know`, so those reads are guarded too; `readTextInput` (`--body-file`) is unchanged, as the item says.

**What:** Same finding, reader half one. Depends on item 1.
**Regression guard.** `findNodeFile` skips non-regular entries. `ReadAliases`/`RecoverAliases` and check's `readAliases` treat another node's sentinel error as a left-out file (warning), never a failure. No Unwrap on `FileError`. The check FIFO test is a non-biting guard test.
**Goal:** store-file reads in `internal/check`, `cli/update.go` (`readIssueFile`) and `internal/nodestore` (`format.go`, `kind.go` ×2, `edit.go`) use `tree.ReadNodeFile` (not `--body-file` input); a non-regular or oversize file yields a Finding or an error naming the path, never a hang.
**Files:** internal/check/check.go, internal/check/check_test.go, internal/check/fifo_unix_test.go, internal/check/fifo_windows_test.go, internal/cli/update.go, internal/nodestore/format.go, internal/nodestore/kind.go, internal/nodestore/edit.go, internal/nodestore/edit_test.go, internal/nodestore/create.go
**Read first:** internal/nodestore/kind.go — ReadAliases, RecoverAliases; internal/check/check.go — checker.readFile, loadTree; internal/nodestore/create.go — findNodeFile;
internal/nodestore/edit.go — Kind.Edit; internal/nodestore/errors.go — FileError; internal/cli/update.go — readIssueFile
**Tests:** `check_test.go`: a FIFO issue file yields a Finding naming it and `check` returns. `edit_test.go`: editing a node whose file became a symlink fails with a `*FileError` at it whose Message contains `not a regular file`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/check/ ./internal/nodestore/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Update'`
**Commit:** `fix(store): read store files through the regular-file guard`

## 4. `issue`, `knowledge`, `agents` and `tree` removal read through the guarded reader — ✅ DONE (2026-10-12)

NOTES (2026-10-12): `isRefusedRead` is a private helper in `internal/issue/remove.go` and `internal/knowledge/remove.go` (the same one item 3 added to nodestore), because `internal/tree/read.go` is outside this item's files; `TopNotes` tests the two sentinels inline.
NOTES (2026-10-12): a refused file of another node joins `Unread`, so the existing CLI warnings for it still say "the file does not parse" (`internal/cli/rm.go`, `mv.go`, `know_rm.go`); the case only arises when a file is swapped after `tree.Load`, which already skips non-regular entries.
NOTES (2026-10-12): the new `TestTopNotes_SymlinkedTopLevelNote_IsLeftOutWithoutError` is a non-biting guard test, as the item says: `tree.Load` already skips the symlink before `TopNotes` reads it.

**What:** Same finding, reader half two. Depends on item 1.
**Regression guard.** Another node's sentinel error in issue/knowledge `planDrops`, `planRewrite`, `planSuccessorRewrites` joins Unread; `TopNotes` skips it like an unparsable Note. Leave the `AGENTS.md` (guide) read in `agents/block.go`. The block_test case is a non-biting guard test.
**Goal:** no store-file read in `internal/issue`, `internal/knowledge` (Ref targets in `verify.go` excepted), `internal/agents` (`TopNotes`) or `internal/tree/remove.go` uses `os.ReadFile`; each uses `tree.ReadNodeFile`.
**Files:** internal/issue/remove.go, internal/issue/move.go, internal/knowledge/remove.go, internal/knowledge/move.go, internal/agents/block.go, internal/tree/remove.go, internal/agents/block_test.go
**Read first:** internal/issue/remove.go — Removal.planDrops; internal/issue/move.go — Move.planRewrite; internal/knowledge/remove.go — Removal.planDrops;
internal/knowledge/move.go — Move.planSuccessorRewrites; internal/agents/block.go — TopNotes; internal/tree/remove.go — Tree.PlanRemove, Removal.removeFileStep;
internal/agents/block_test.go — TestTopNotes_RetiredTopLevelNote_IsLeftOut
**Tests:** `block_test.go`: a symlinked top-level Note is left out of the Managed block without error.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/issue/ ./internal/knowledge/ ./internal/agents/ ./internal/tree/`
- `grep -rn --include='*.go' --exclude='*_test.go' 'os.ReadFile' internal/issue internal/knowledge/move.go internal/knowledge/remove.go internal/tree/remove.go` prints nothing
**Commit:** `fix(store): guard the remaining node-file reads`

## 5. Tree resolution refuses a duplicated id or alias — ✅ DONE (2026-10-12)

NOTES (2026-10-12): re-derived from the assumed `duplicates map[string][2]string`: `Tree.duplicates` maps each name to the duplicate *Error Load already reports for it (both files in Path and Other), and Resolve returns a copy of it, so the refusal is worded exactly like Load's warning.
NOTES (2026-10-12): claims are collected from the ids of every walked file and the aliases of the files Load keeps; the aliases of a file dropped as a duplicate id are not read (reading them would add an AliasFunc failure mode to Load), so a name only such a file aliases is not refused here, unlike the index.
NOTES (2026-10-12): changed outcome — internal/nodestore/move_test.go "duplicate of the moved issue" now gets the duplicate *tree.Error from Resolve instead of the tree.Errors from triage; still a refusal naming the file with nothing moved.
NOTES (2026-10-12): the `know rm` runCLI test lives in internal/cli/know_rm_test.go, beside the other `know rm` tests, which the item's Files line does not list; the shared fixture and assertion helpers live in rm_test.go.

**What:** Fix: `tree.Load` keeps a duplicate's first file while the index drops both, so `rm`/`mv`/`know rm`/`know mv` act on a node `show` calls absent. Binding: the header's **Duplicates** call. `tree.Resolve`'s only callers are nodestore's `Create` (`--parent`), `Move` (ref, `--parent`) and `Remove` — exactly the tree-resolved writes — so the refusal lives in `tree.Resolve` alone (one deep module, no per-call-site checks). Index-resolved commands (`update`, `close`, `log`, `know update`, `dep`, `related`, `show`) are item 6's; `Edit` is not touched here.
**Goal:** `(*Tree).Resolve` fails with an `*Error` wrapping `tree.ErrDuplicate`, never `ErrNotFound`, naming two claimant files, when the normalised argument is an id or alias that two or more files claim — before exact, alias or prefix matching; `tree.Load`, its warnings and `Lookup` are unchanged.
**Approach (assumed at the header base):** in `Load`, record every claimed name over ids and aliases in one pass (as `index.excludeDuplicatesOf` collects claims) into a `duplicates map[string][2]string` on `Tree`; `indexIDs`/`indexAlias` keep their first-file-wins behaviour and warnings. `Resolve` checks that map first. nodestore relativizes the error through `relativeTreeError` in `Create`, `Move` and `Remove` so no absolute root is printed; it falls through `kindFailure` to `asError` (exit 1).
**Files:** internal/tree/tree.go, internal/tree/resolve.go, internal/tree/resolve_test.go, internal/nodestore/create.go, internal/nodestore/move.go, internal/nodestore/remove.go, internal/nodestore/remove_test.go, internal/cli/rm_test.go, internal/cli/mv_test.go
**Read first:** internal/tree/resolve.go — Resolve, Lookup; internal/tree/tree.go — Load, indexIDs, indexAlias; internal/nodestore/create.go — claimFailure, relativeTreeError;
internal/nodestore/remove.go — Remove; internal/nodestore/move.go — Move, resolveParent; internal/index/refresh.go — excludeDuplicatesOf
**Tests:** `resolve_test.go`: two `foo.md` + valid `foo-bar` → `Resolve("foo")` is `ErrDuplicate`, not `ErrNotFound`; a third file aliasing `foo` (id/alias clash) also refuses; `Resolve("foo-b")` and an unduplicated name resolve as before; `Lookup("foo")` unchanged. `remove_test.go`: `Remove("foo")` writes nothing. `runCLI`, same fixture: `rm foo` and `mv foo-bar --parent foo` exit 1 naming both `foo` files repo-relative, `foo-bar` and both `foo` files unchanged; `know rm` the same for a duplicated Note id.
Existing duplicate tests (mv, know mv/rm, nodestore, create, check, tree) keep their outcome; one that changes goes on a NOTES line.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/tree/ ./internal/nodestore/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Mv|Rm|Create|KnowAdd|KnowMv|KnowRm'`
**Commit:** `fix(tree): refuse a duplicated id or alias when resolving for a write`

## 6. Id resolution refuses a name that exactly matches an excluded node — ✅ DONE (2026-10-12)

NOTES (2026-10-12): the duplicate error for a name only one left-out node claims (it was left out for a clash on another of its names) names that clash, e.g. `foo.md: duplicate alias "qux": also the id of qux.md`, so it is still a *tree.Error wrapping tree.ErrDuplicate that names two files; the wording mirrors the refresh's duplicate warning.
NOTES (2026-10-12): the invalid-file error reads `id "foo" not found: .capcom/issues/foo.md is invalid`; the invalid file's files row is matched by kind and problems IS NOT NULL in SQL and by path.Base == name+".md" in Go, no LIKE.
NOTES (2026-10-12): consequential edit — internal/index/notes.go: made necessary by the new refusal in resolveID, which ResolveNote's doc comment now states.
NOTES (2026-10-12): consequential edit — docs/format.md: made necessary by the new refusal in resolveID; "Naming a node" now says an exact name of a left-out duplicate or an invalid file is never matched as a prefix.
NOTES (2026-10-12): no internal/cli code changed: a duplicate *tree.Error already falls through resolveFailure to asError (exit 1, and multi-id `show`/`close` stop at it), and dep rm/related rm treat only tree.ErrNotFound as the exact-text case.

**What:** `resolveID` prefix-matches when the exact id is excluded, so `close foo` with a duplicate `foo` mutates `foo-bar`. Covers every index-resolved command (`update`, `close`, `log`, `know update` via `issueResolver`/`ResolveNote`; `dep`, `related`, `show`). Depends on item 5.
**Regression guard.** A duplicate-excluded exact name returns the same kind of error as item 5's refusal (exit 1, `tree.ErrDuplicate`, not `ErrNotFound`, both files relative); `dep rm`, `related rm`, multi-id `show` stop at it. Only the invalid-file case wraps `tree.ErrNotFound`, quoting arg as typed, so `dep rm`/`related rm` drop the exact-text entry and `show a foo b` goes on. Match an invalid file's `files` row by `kind = ? AND problems IS NOT NULL` and base name == name+".md", no LIKE.
**Goal:** an argument equal to an invalid file's id, or a duplicate-excluded node's id or alias, makes `Resolve`/`ResolveNote` (`Index.resolveID`, before the prefix branch) return that error, never a prefix match; otherwise prefix resolution is as `docs/format.md` documents.
**Files:** internal/index/query.go, internal/index/query_test.go, internal/cli/close_test.go, internal/cli/write_test.go, internal/cli/dep_test.go, internal/cli/read_test.go
**Read first:** internal/index/query.go — resolveID; internal/index/notes.go — ResolveNote; internal/index/refresh.go — indexFile, excludeClaims;
internal/index/schema.go — files table; internal/cli/show.go — resolveFailure; internal/cli/dep.go — depRemoval; internal/index/query_test.go — listFixture
**Tests:** `query_test.go`, `foo-bar` valid: dup `foo` → `Resolve("foo")` fails with `tree.ErrDuplicate`, not `ErrNotFound`; invalid `foo.md` → `tree.ErrNotFound`; `Resolve("fo")` resolves. `runCLI`, `foo-bar` valid: `close foo --reason x` fails with `foo-bar` left open, both for an invalid `foo.md` and for a duplicated `foo` (exit 1 naming both files); `update foo --title y` with a duplicated `foo` exits 1, `foo-bar` unchanged; `dep rm x foo` drops the exact `foo` dep (invalid `foo.md`) or exits 1, x unchanged (duplicated `foo`); `show foo` with a duplicated `foo` exits 1 naming both files.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Close|Update|DepRm|Show'`
**Commit:** `fix(index): never prefix-resolve a name owned by an excluded node`

## 7. The Managed-block rewrite of `AGENTS.md` is durable and keeps symlinks — ✅ DONE (2026-10-12)

NOTES (2026-10-12): replaceFile and the now-unused tempPattern constant are gone; the target and mode are resolved by a new resolveGuide helper, and reconcile wraps any failure as `AGENTS.md: <err>` (the inner tree error also names the resolved path). A dangling `AGENTS.md` symlink resolves to ENOENT and is therefore replaced by a regular file, as the Goal's "path on ENOENT" rule says.

**What:** `agents.replaceFile` duplicates the atomic write without fsync and severs a symlinked guide; delete it.
**Regression guard.** A write failure keeps the `AGENTS.md: <err>` prefix: the caller wraps the error as `fmt.Errorf("%s: %w", FileName, err)`, so `capcom agents` and the `refreshAgentsAfterWrite` warning keep their wording.
**Goal:** the guide is written by `tree.WriteFileAtomicVerbatim` to its `filepath.EvalSymlinks` target (`path` on ENOENT), keeping its mode (`0o644` if new) and a symlink a symlink; a failure leaves the bytes and no `.AGENTS.md.tmp-*`.
**Files:** internal/agents/block.go, internal/agents/block_test.go
**Read first:** internal/agents/block.go — reconcile, replaceFile, newFilePermissions; internal/tree/atomic.go — WriteFileAtomicVerbatim;
internal/agents/block_test.go — writeGuide, TestWrite_CRLFGuide_StaysCRLFAndIsThenCurrent; internal/cli/know.go — refreshAgentsAfterWrite
**Tests:** a `0o600` guide keeps its mode; `AGENTS.md -> CLAUDE.md` stays a link, `CLAUDE.md` gets the block; a read-only dir gives an error starting `AGENTS.md: `, bytes intact, no temp file (skip on Windows/root).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/agents/`
- `grep -n 'func replaceFile' internal/agents/block.go` prints nothing
**Commit:** `fix(agents): write the guide atomically through its symlink target`

## 8. `mv --parent ""` is a usage error — ✅ DONE (2026-10-12)

**What:** An empty `--parent` moves a subtree to the top level.
**Regression guard.** `moveDestination` is shared with `know mv` (`know_mv.go`), so the check covers both; `know mv` tests live in `know_mv_test.go`.
**Goal:** `mv`/`know mv` with an empty or whitespace-only `--parent` fail with `UsageErrorf("--parent needs an id or %q", rootParentArg)`, changing nothing; only `--parent .` moves to the top.
**Files:** internal/cli/mv.go, internal/cli/know_mv.go, internal/cli/mv_test.go, internal/cli/know_mv_test.go
**Read first:** internal/cli/mv.go — moveDestination, rootParentArg; internal/cli/know_mv.go — newKnowMvCommand, moveNote; internal/cli/errors.go — UsageErrorf;
internal/cli/mv_test.go — TestMv_Parent_MovesBelowThenToTop, newMoveStore; internal/cli/know_mv_test.go — TestKnowMv_Parent_MovesBelowThenToTop
**Tests:** `mv child --parent ""` and `know mv note --parent " "` exit as usage errors, file untouched; `mv child --parent .` reparents to the top.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Mv|Move'`
**Commit:** `fix(cli): refuse an empty --parent instead of moving to the top`

## 9. `elsewhere.days` above 36500 is a config error

**What:** A large `elsewhere.days` overflows the cutoff and silences the Elsewhere scan.
**Goal:** `elsewhere.days` above `maxElsewhereDays = 36500` fails to load with an error naming the config file, line and bound; 36500 loads; the `docs/format.md` bullet says "0 to 36500".
**Files:** internal/config/config.go, internal/config/config_test.go, docs/format.md
**Read first:** internal/config/config.go — parser.elsewhere, parser.errorAt, defaultElsewhereDays; internal/config/config_test.go — TestParse_InvalidConfig_FailsWithPathAndLine;
docs/format.md — elsewhere.days bullet; internal/elsewhere/elsewhere.go — otherRecentBranches
**Tests:** `config_test.go`: 36500 loads; 36501 and 999999 fail with the file:line error.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/config/`
- `grep -n '36500' docs/format.md`
**Commit:** `fix(config): bound elsewhere.days to keep the scan cutoff in range`

## 10. An unresolvable Ref is a Finding, not a command failure

**What:** A symlink loop, ENAMETOOLONG or EACCES in one Note's `refs` aborts `check` and `know verify`.
**Regression guard.** A loop is `too many links`, not `syscall.ELOOP`, so every non-missing `EvalSymlinks` error on a Ref is the message; the worktree-root `EvalSymlinks` in `Verify` keeps its error return. `checkSymbol`'s `os.Stat`/`os.ReadFile` errors are messages too. Supersedes and rewrites the `Verify` and `checkRef` doc comments ("fails only when a file cannot be read…", "err is set only when the disk cannot be read").
**Goal:** `checkRef` returns the message `cannot be resolved: <err>` for a loop, a too-long name and a permission error, so `check` and `know verify` report one Finding and go on.
**Files:** internal/knowledge/verify.go, internal/knowledge/knowledge_test.go
**Read first:** internal/knowledge/verify.go — checkRef, isMissing, checkSymbol, Verify; internal/knowledge/knowledge_test.go — TestVerify_SymlinkOutOfTheTree_IsFlagged, newWorktree;
internal/check/check.go — checker.checkNote
**Tests:** `refs: [loop/x]` with `loop -> loop` yields a Finding containing `cannot be resolved`, no error; `refs: [secret.txt#sym]` on a mode-000 file yields a Finding (skip both on Windows, the second also as root).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/knowledge/`
**Commit:** `fix(knowledge): report an unresolvable ref as a finding`

## 11. `release prepare` fetches before judging the version against tags

**What:** `checkPrepare` compares against stale local tags.
**Regression guard.** `execRepo.FetchMain` already fetches `--tags`: no interface change. `memRepo.FetchMain` is a no-op, so the test wraps it like `bumpRefusingRepo`, exposing origin's tags to `ReleaseTags` only after `FetchMain`; the bite tag tops `preparedTags`. `prepare`'s `--help` Long text and `docs/releasing.md` bullet say it fetches origin's main and tags first and refuses when that fails, worded as the `status` bullet.
**Goal:** `checkPrepare` calls `checkout.FetchMain(ctx)` before reading `ReleaseTags`, and a failed fetch is a refusal in the form of the other prepare checks.
**Files:** cmd/release/prepare.go, cmd/release/prepare_test.go, cmd/release/repo.go, cmd/release/memrepo_test.go, docs/releasing.md
**Read first:** cmd/release/prepare.go — checkPrepare, newPrepareCommand; cmd/release/repo.go — execRepo.FetchMain; cmd/release/memrepo_test.go — memRepo.FetchMain, memRepo.ReleaseTags;
cmd/release/prepare_test.go — preparedTags, bumpRefusingRepo; docs/releasing.md — verb list
**Tests:** origin holds `v0.25.0`, absent locally → `prepare v0.25.0` refuses with `v0.25.0 is not above the highest release tag v0.25.0`; a failing fetch refuses with its error.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/`
**Commit:** `fix(release): fetch tags before checking the prepared version`

## 12. A failed index commit reports only the commit error

**What:** The deferred rollback joins `sql: transaction has already been committed or rolled back` onto a commit failure.
**Regression guard.** IMMEDIATE transactions make a lock fail `BeginTx`, never `Commit`: add a seam `commitTx = (*sql.Tx).Commit` in `refresh.go`, swapped in `index_test.go`; the fake rolls back then fails, so `sql.ErrTxDone` really arises in the deferred rollback.
**Goal:** when `tx.Commit` fails in `Refresh`, the error wraps the commit error and not `sql.ErrTxDone` (the deferred rollback ignores it).
**Files:** internal/index/refresh.go, internal/index/index_test.go
**Read first:** internal/index/refresh.go — refresh; internal/index/index.go — Open, dsnOptions; internal/index/index_test.go — newStore, openIndex, writeIssue
**Tests:** a commit failure through `commitTx` → `!errors.Is(err, sql.ErrTxDone)` and the message names the commit failure.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/index/`
**Commit:** `fix(index): stop reporting a spurious rollback error after a failed commit`

## 13. A corrupt lock record can be broken and does not hide other locks

**What:** `Break`, `Release`, `Acquire --take-stale` and `List` fail on an unparsable record.
**Regression guard.** `l.read` wraps every parse failure, `{}` included, in one sentinel (e.g. `errCorruptRecord`) that `Break` and `List` test; an I/O error (EACCES, EISDIR) still fails, so `Break` never deletes an unread record. Keep the `invalid lock record` wording. `briefing.readLocks` routes `List` warnings to `b.fail(SectionLocks, …)` like `scanElsewhere`. `Release`/`Acquire` unchanged.
**Goal:** `lock.Break` on an invalid-JSON or zero-length record removes it and returns a Status saying it was corrupt and discarded; `List` returns every parsable lock plus a warning per corrupt key, never an error.
**Files:** internal/lock/lock.go, internal/lock/lock_test.go, internal/cli/lock.go, internal/cli/lock_test.go, internal/briefing/briefing.go
**Read first:** internal/lock/lock.go — Locks.read, Break, List; internal/cli/lock.go — newLockBreakCommand, emitLock; internal/briefing/briefing.go — readLocks, fail;
internal/lock/lock_test.go — TestRead_CorruptRecord_FailsNamingTheFile
**Tests:** zero-length, `{`, `{}` records → `Break` succeeds, file gone; `List` over one corrupt and one valid key → the lock plus one warning; `runCLI` `lock break` on a corrupt record exits 0 with the discard notice.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/lock/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Lock'`
**Commit:** `fix(lock): let break clear a corrupt record and list skip it`

## 14. `Create` places a new node file with an exclusive create

**What:** Two sessions creating one id at once silently overwrite each other.
**Regression guard.** Only Create writes exclusively; Edit and Format keep `Kind.write`'s atomic overwrite (a shared writer takes a mode flag). `tree.WriteFileExclusive` (`atomic.go`): `MkdirAll`, write and fsync a temp, `os.Link` it into place, remove it, `syncDir`; any Link error not `fs.ErrExist` (EPERM, ENOTSUP, ERROR_INVALID_FUNCTION) falls back to `O_CREATE|O_EXCL`.
**Goal:** `nodestore.Create` never replaces an existing file: a file appearing at the target after the id check makes it fail with the id-taken error naming the path, that file's bytes unchanged.
**Files:** internal/tree/atomic.go, internal/tree/atomic_test.go, internal/nodestore/create.go, internal/nodestore/create_test.go
**Read first:** internal/nodestore/create.go — Kind.Create, Kind.write, existsError; internal/nodestore/edit.go — Kind.Edit; internal/nodestore/format.go — Kind.Format;
internal/tree/atomic.go — writeFileAtomic, fillTemp
**Tests:** `atomic_test.go`: an exclusive write onto an existing path → `fs.ErrExist`, bytes unchanged, no temp. `create_test.go`: a file planted between check and write → `Create` fails with the id-taken error.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/tree/ ./internal/nodestore/`
**Commit:** `fix(nodestore): create node files exclusively`

## 15. `Edit` re-checks the etag immediately before replacing the file

**What:** An edit landing between the `--if-match` check and the rename is overwritten. Depends on item 14.
**Regression guard.** Format keeps its plain atomic overwrite. A re-check mismatch passes the first-read etag as stale (`conflictError(path, firstETag, current)`), never the possibly empty `ifMatch`; rewrite `Edit`'s doc step list for the unconditional re-check.
**Goal:** `Edit` re-reads the file just before the atomic write, whether or not `--if-match` was given; if its bytes changed since the first read it fails with the etag-mismatch error and the newer bytes are kept.
**Files:** internal/nodestore/edit.go, internal/nodestore/edit_test.go, internal/nodestore/create.go
**Read first:** internal/nodestore/edit.go — Kind.Edit; internal/nodestore/errors.go — conflictError; internal/nodestore/create.go — Kind.write;
internal/cli/update.go — editNode; internal/nodestore/edit_test.go — TestEdit_Refusals_WriteNothing
**Tests:** a package-level hook between compute and write rewrites the file; `Edit` without an etag fails naming the first-read etag as stale; the rewritten bytes survive.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
**Commit:** `fix(nodestore): re-check the etag before replacing an edited file`

## 16. Test the tag-on-version-bump script

**What:** Recast at the regression check (2026-10-12). Test-only: `.github/scripts/version-bump.sh` decides release tags and no test runs it.
**Regression guard.** Per the header call, add Note `adr-0008/adr-0008-amend-bumptest.md` (free 4-word id; `-amend-1012` is taken) to `adr-0008.md`'s reading list; it supersedes `adr-0008-decision.md` ("Its tests never run `git`, …"). Rewrite the `main.go` package doc and `runner.go` doc saying tests never reach git (line-wrapped; a grep misses it).
Use `newRepo`'s hermetic env (`GIT_CONFIG_GLOBAL=os.DevNull`, `GIT_CONFIG_NOSYSTEM=1`, `GIT_CEILING_DIRECTORIES`, author/committer vars); run the script by absolute path, `Dir` the temp repo; skip without `bash`/`git`; names contain "VersionBump".
**Files:** cmd/release/versionbump_test.go, cmd/release/runner.go, cmd/release/main.go, .capcom/knowledge/decisions/adr-0008.md, .capcom/knowledge/decisions/adr-0008/adr-0008-amend-bumptest.md
**Read first:** .github/scripts/version-bump.sh — read_version, bumps loop, zero-sha fallback; internal/elsewhere/elsewhere_test.go — newRepo; cmd/release/main.go — package doc;
cmd/release/runner.go — runner doc comment; .capcom/knowledge/decisions/adr-0008.md — reading list; .capcom/knowledge/decisions/adr-0008/adr-0008-decision.md — scoped exception to ADR 0007
**Tests:** `GITHUB_OUTPUT` at a temp file: one bump → `bumps=<sha>=v1.2.3`; two bumps in one range → two pairs oldest first; no VERSION change → `bumps=`; VERSION `1.2` → empty, stderr has `refusing to tag`; `before` = 40 zeros → the parent of `after`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run 'VersionBump'`
**Commit:** `test(release): run version-bump.sh against a temp repo`

## 17. Test the production process runner

**What:** Test-only: `execRunner.run` has 0% coverage.
**Regression guard.** The child is the test binary via a `TestMain` helper switch in `main_test.go` that exits only when its own env var is set, else runs `m.Run()`; test names contain "Runner". Compare the child's cwd to `dir` with `os.SameFile`, never as strings (macOS `/private/var/…`).
**Files:** cmd/release/runner_test.go, cmd/release/main_test.go
**Read first:** cmd/release/runner.go — runner, execRunner.run; cmd/release/main_test.go — fakeRunner, execute; internal/elsewhere/elsewhere_test.go — newRepo
**Tests:** an `env` entry overrides an inherited variable; the child's cwd is `dir`; stdout returns verbatim; a non-zero exit → an error containing stderr; a cancelled ctx kills a sleeping child promptly.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run 'Runner'`
**Commit:** `test(release): cover execRunner's env, dir, output and failure contract`

## 18. Test the `mv` guard against overwriting a move target

**What:** Test-only: `ErrMoveTarget` in `renameStep` is never exercised.
**Regression guard.** Create each obstacle after `PlanMove`, before `Apply` (one present at `tree.Load` fails `PlanMove` with `*TakenError`). The children-dir case moves a node with a child (only then is a children directory renamed) and asserts the rollback renames the node file back.
**Files:** internal/tree/move_test.go
**Read first:** internal/tree/plan.go — renameStep, rollBack, ErrMoveTarget; internal/tree/move.go — PlanMove, planRenames;
internal/tree/move_test.go — moveNode, snapshotDir, TestMoveApply_SecondRenameFails_LeavesTreeUnchanged
**Tests:** an untracked file at the destination → `Apply` fails with `tree.ErrMoveTarget`, target bytes unchanged, source in place; a destination children directory → the same, node file renamed back.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/tree/ -run 'Move'`
**Commit:** `test(tree): cover the mv guard against an existing target`

## 19. Test concurrent index refresh

**What:** Test-only: nothing runs two `Open`/`Refresh` at once on one `index.db`.
**Files:** internal/index/index_test.go
**Read first:** internal/index/index.go — Open, dsnOptions; internal/index/refresh.go — refresh, recordedFile.isDirty, racyGranularity; internal/index/index_test.go — newStore, openIndex
**Tests:** two Indexes on one store refresh from goroutines while a third rewrites an issue file; no error mentions `SQLITE_BUSY`/`database is locked`, a final refresh of each reports the same rows, no id twice.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -race ./internal/index/ -run 'Concurrent'`
**Commit:** `test(index): refresh one index from two connections at once`

## 20. Test the diverged local/remote branch pair in the Elsewhere scan

**What:** Test-only: `redundantRef` with diverged `x` and `origin/x` is untested.
**Files:** internal/elsewhere/elsewhere_test.go
**Read first:** internal/elsewhere/elsewhere.go — redundantRef; internal/elsewhere/elsewhere_test.go — newClone, onBranch, commitIssues, entryLines,
TestScan_LocalBranchBehindItsRemote_ReportsTheRemoteOnly
**Tests:** local `x` and `origin/x` each carry a distinct commit touching an issue file; the scan reports both.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/elsewhere/ -run 'Diverged'`
**Commit:** `test(elsewhere): keep both refs of a diverged branch pair`
