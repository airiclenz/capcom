# Audit leftovers and doc tidy plan

**Goal:** Close the three P3 issues the code audit left behind (one bug, two missing tests). Then
bring two Knowledge Notes in line with the code and archive the two superseded handoffs.
**Date:** 2026-10-12
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** f30fa4a

**Sources:**
- `.capcom/issues/resolve-dropped-alias.md`, `.capcom/issues/unread-swap-test.md`,
  `.capcom/issues/exclusive-copy-test.md`
- `docs/skill-runs/refocus/2026-10-12/corrections.md` (the two Note fixes, wording approved)
- `internal/index/refresh.go` (`excludeDuplicates`, `excludeClaims`): the index's duplicate rule
- Notes `release-process`, `adr-0008-amend-bumptest`, `docs-layout`, `cli-test-harness`

**Ratified design calls** (user, 2026-10-12, unless marked author):
- **Dropped aliases:** Resolve refuses an alias of a file dropped as a duplicate id only when another file also claims that name. An alias held by the dropped file alone stays as it is.
- **Load reports the clash** (author): such an alias is a Load problem wrapping `ErrDuplicate`, the problem `Resolve` returns, as Resolve's doc comment already says. As a result, `capcom check` reports it and parses the dropped file through its AliasFunc.
- **Doc fixes:** both corrections are applied as worded in `corrections.md`. Keeping `docs/handoffs/` git-ignored is intended.
- **Issue-index idea:** stays parked in the archived handoff, not filed.
- **Link seam** (author): `placeExclusive` calls a package-level `link` variable (default `os.Link`), swapped through `export_test.go`. This follows the existing `SetMoveRename` precedent.

**Closes:** unread-swap-test

**Standing requirements:**
- skills: coding-standards
- Raspberry Pi host: run one package at a time with `GOMEMLIMIT=2GiB`, never `-race` on `./...`, and never two test commands at once.
- No AI attribution in commits, code or docs. Commit to `main`.

**Out of scope:**
- Cutting a release, the signed-releases decision, and any version or CHANGELOG heading change.
- The index excluding the *first* file of a duplicate id, which tree.Load keeps.
- The `rm`/`mv` warning that says "does not parse" for a refused read.

**Regression check (2026-10-12, f30fa4a):**
- 1: guard folded (path-keyed AliasFunc for dropped-file tests, `**Closes:** resolve-dropped-alias`, Goal reworded to the first recorded duplicate; What states that check also reports a dropped file's parse failures, per writer decision)
- 2: guard folded (mv cases are renames; each swap removes the file first)
- 3: guard folded (mv cases are renames; each swap removes the file first; path from `notes.Nodes()`)
- 5: guard folded (writer decision: reflow the release-process paragraph)

## 1. Resolve refuses an alias a dropped duplicate-id file shares — ✅ DONE (2026-10-12)

NOTES (2026-10-12): TestResolve_DuplicatedName_FailsNamingTwoClaimants now builds its AliasFunc with aliasesByPath for every case (its existing keys are top-level files, whose relative path is their base name, so the existing cases behave as before); aliasesByFile stays for every other existing test.
NOTES (2026-10-12): indexAlias's two duplicate-alias errors now come from shared helpers (claimedAlias, newDuplicateAliasError) that the dropped-file check reuses; the error text is unchanged.

**What:** Fixes bug `resolve-dropped-alias`. Load reads aliases only for the files it keeps, so a name that a kept file and a dropped duplicate-id file both claim still resolves. Reading a dropped file's aliases makes `capcom check` also report that file's frontmatter parse failures and conflict markers; this is intended, and `check_test.go` pins one such case alongside the `CodeDuplicate` case.
**Regression guard.** Every dropped-file test builds its AliasFunc with a new path-keyed helper, `aliasesByPath(dir, map[relpath][]string)` in `tree_test.go`: `aliasesByFile` keys on `filepath.Base`, and a dropped duplicate-id file shares its kept twin's base name. `aliasesByFile` stays for the existing tests. When N is already a recorded duplicate, `recordDuplicate` keeps the earlier problem, so `Resolve(N)` fails with the first duplicate problem Load recorded for N.
**Goal:** Suppose file A claims name N (as id or alias), and file B is dropped for a duplicate id but lists N as an alias. Then Load reports N as a duplicate problem (`ErrDuplicate`, naming B and A), and `Resolve(N)` fails with the first duplicate problem Load recorded for N. If only the dropped file lists an alias, it gets no new problem and resolves as before. Two dropped files sharing an alias is also a duplicate.
**Approach (assumed at the header base):** `indexIDs` keeps the nodes it drops (a slice on `Tree`, or one handed to `indexAliases`). Once the kept nodes' aliases are indexed, `indexAliases` calls the AliasFunc for each dropped node and runs `CheckAliases` on the result. A passing alias that a kept node holds in `byID`/`byAlias`, or that an earlier dropped node lists, becomes an `*Error` with Path set to the dropped file and Other to the claimant. It is appended to problems and passed to `recordDuplicate`. Dropped aliases are never added to `byAlias`. Problems that `CheckAliases` rejects for a dropped file are appended as they are for kept files.
**Files:** internal/tree/tree.go, internal/tree/resolve_test.go, internal/tree/tree_test.go, internal/check/check_test.go
**Read first:** internal/tree/tree.go — indexIDs, indexAliases, recordDuplicate; internal/tree/resolve.go — Resolve; internal/tree/tree_test.go — aliasesByFile; internal/check/check.go — loadTree;
internal/tree/resolve_test.go — TestResolve_DuplicatedName_FailsNamingTwoClaimants;
internal/check/check_test.go — TestRun_DuplicateIDAndAlias_NameBothFilesRelativeToTheWorktree
**Closes:** resolve-dropped-alias
**Tests:**
- `resolve_test.go`: add cases to `TestResolve_DuplicatedName_FailsNamingTwoClaimants`: the kept file's id, or the kept file's alias, is also an alias of a dropped duplicate-id file. A new test pins that an alias only the dropped file holds is still not refused as a duplicate.
- `tree_test.go`: add `aliasesByPath`; Load returns the new problem.
- `check_test.go`: `check` reports the clash as `CodeDuplicate`, and reports a dropped duplicate-id file's frontmatter parse failure.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `GOMEMLIMIT=2GiB go test ./internal/check/`
- `go vet ./internal/tree/ ./internal/check/`
**Commit:** `fix(tree): refuse an alias a dropped duplicate-id file shares`

## 2. Issue rm and mv: test a node file swapped after Load

**What:** Covers the issue half of `unread-swap-test`.
**Regression guard.** The mv cases are a rename (the new id differs from the old), so `planRewrites` reads the swapped file; a re-parent skips every other issue file. Each symlink or FIFO swap first `os.Remove`s the issue file, then creates the symlink or FIFO at its path (`Mkfifo` and `os.Symlink` fail with EEXIST otherwise); the >1 MiB case overwrites the file in place.
**Approach (assumed at the header base):** each test calls `tree.Load`, then swaps another issue's file, then calls `issue.PlanRemove` or `issue.PlanMove` (both take the loaded tree). It uses table cases: symlink, FIFO, and a regular file over 1 MiB. Each asserts that the swapped path joins the plan's `Unread`, that its file is left exactly as swapped (`os.Lstat` mode and size), and that the plan returns no error. Add `internal/issue/fifo_unix_test.go` (`//go:build unix`, `syscall.Mkfifo`, skips as root) and `fifo_windows_test.go` (skips), copied from `internal/check`. Symlink cases skip on Windows and as root, the same way `internal/tree`'s `makeSymlink` does.
**Files:** internal/issue/remove_test.go, internal/issue/move_test.go, internal/issue/fifo_unix_test.go, internal/issue/fifo_windows_test.go
**Read first:** internal/issue/remove.go — planDrops, isRefusedRead; internal/issue/move.go — planRewrites; internal/issue/move_test.go — newMoveTree, loadMoveTree;
internal/issue/remove_test.go — TestPlanRemove_UnparseableFiles_RemovesOwnAndListsOthersAsUnread;
internal/check/fifo_unix_test.go — makeFIFO; internal/tree/tree_test.go — makeSymlink
**Tests:** new tests in `remove_test.go` and `move_test.go` (`package issue_test`).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/issue/`
- `go vet ./internal/issue/`
**Commit:** `test(issue): cover a node file swapped after Load in rm and mv`

## 3. Note rm and mv: test a node file swapped after Load

**What:** Covers the Note half of `unread-swap-test`.
**Regression guard.** The mv cases are a rename (the new id differs from the old): `knowledge.PlanMove` returns before `planSuccessorRewrites` on a re-parent. Each symlink or FIFO swap first `os.Remove`s the Note file, then creates the symlink or FIFO at its path; the >1 MiB case overwrites in place. Take the Note's path from `notes.Nodes()` (`loadNotes`/`newNoteTree` return only the tree; `Tree.Dir` also exists).
**Approach (assumed at the header base):** same as item 2, but for `knowledge.PlanRemove` (`planDrops`) and `knowledge.PlanMove` (`planSuccessorRewrites`). The swapped file is another Note that names the removed or moved Note, so the plan has to read it. Add FIFO helper files in `package knowledge`. Reuse the existing `loadNotes`/`newNoteTree` helpers.
**Files:** internal/knowledge/remove_test.go, internal/knowledge/move_test.go, internal/knowledge/fifo_unix_test.go, internal/knowledge/fifo_windows_test.go
**Read first:** internal/knowledge/remove.go — planDrops, isRefusedRead; internal/knowledge/move.go — PlanMove, planSuccessorRewrites;
internal/knowledge/move_test.go — loadNotes; internal/knowledge/knowledge_test.go — newNoteTree; internal/tree/tree.go — Tree.Nodes;
internal/tree/tree_test.go — makeSymlink
**Tests:** new tests in `remove_test.go` and `move_test.go` (`package knowledge`).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/knowledge/`
- `go vet ./internal/knowledge/`
**Commit:** `test(knowledge): cover a node file swapped after Load in rm and mv`

## 4. WriteFileExclusive: test the copy fallback

**What:** Closes the gap in `exclusive-copy-test`.
**Goal:** `internal/tree/atomic_test.go` forces the hard link to fail and covers the copy fallback. A free path ends up holding the whole content with LF endings and no temp file left behind. An existing path fails with `fs.ErrExist` and keeps its old bytes.
**Approach (assumed at the header base):** in `atomic.go`, `placeExclusive` calls a package-level `var link = os.Link`. `export_test.go` adds `SetLink(t *testing.T, fn func(string, string) error)`, which swaps the variable and restores it in `t.Cleanup`. The injected link returns `&os.LinkError{Op: "link", Err: errors.ErrUnsupported}`, which is not `fs.ErrExist`, so the code takes the copy path. Tests that swap the variable never call `t.Parallel`.
**Files:** internal/tree/atomic.go, internal/tree/export_test.go, internal/tree/atomic_test.go
**Read first:** internal/tree/atomic.go — placeExclusive, copyExclusive, WriteFileExclusive; internal/tree/export_test.go — SetMoveRename; internal/tree/tree_test.go — dirEntries, writeNodes;
internal/tree/atomic_test.go — TestWriteFileExclusive_FreePath_CreatesFileAsLFAndLeavesNoTempFile,
TestWriteFileExclusive_PathExists_FailsWithErrExistAndLeavesFileAndNoTempFile
**Closes:** exclusive-copy-test
**Tests:** two new tests in `atomic_test.go`, mirroring the existing `TestWriteFileExclusive_FreePath_…` and `…_PathExists_…` tests on the copy path.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -run WriteFileExclusive ./internal/tree/`
- `go vet ./internal/tree/`
**Commit:** `test(tree): cover WriteFileExclusive's copy fallback without hard links`

## 5. Note fixes and handoff archive

**What:** apply the two approved Note corrections and archive the superseded handoffs.
**Regression guard.** The release-process.md CURRENT sentence wraps across lines; replace it by reflowing the paragraph so the PROPOSED wording lands, keeping the file's line width.
**Goal:**
- `.capcom/knowledge/release-process.md` names `versionbump_test.go` (Note `adr-0008-amend-bumptest`) as the exception to "never run git, gh, go or komac".
- `.capcom/knowledge/docs-layout.md` says `docs/handoffs/` is git-ignored and local to each clone.
- `docs/handoffs/` holds no handoff outside `archived/`.
**Approach (assumed at the header base):** replace each CURRENT sentence in `corrections.md` with its PROPOSED one. Move `docs/handoffs/2026-10-12 - 00 - maturity-feedback-follow-ups.md` and `… - 01 - knowledge-organization-review.md` into `docs/handoffs/archived/` with the same names. They are git-ignored, so the move changes the working tree only and stays out of the commit.
**Files:** .capcom/knowledge/release-process.md, .capcom/knowledge/docs-layout.md
**Read first:** .capcom/knowledge/release-process.md — the ADR 0007 exception sentence; .capcom/knowledge/docs-layout.md — the docs/handoffs/ bullet;
docs/skill-runs/refocus/2026-10-12/corrections.md — CURRENT/PROPOSED 1 and 2; .capcom/knowledge/decisions/adr-0008/adr-0008-amend-bumptest.md;
docs/handoffs/.gitignore; cmd/release/versionbump_test.go — header comment
**Tests:** none (docs only).
**Acceptance:**
- `grep -n versionbump_test .capcom/knowledge/release-process.md`
- `grep -n "git-ignored\|ignores everything" .capcom/knowledge/docs-layout.md`
- `ls docs/handoffs/` lists only `archived`
- `go run ./cmd/capcom check`
**Commit:** `docs(knowledge): note the bump-test exception and local-only handoffs`
