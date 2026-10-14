# Architecture review 2026-10-14: membership, validation, reference lists and locality fixes

**Goal:** Land the review's three Strong candidates (one owner for Tree membership and resolution, one validation vocabulary, one reference-list module), the verified defects, and the small locality fixes. Behaviour changes follow the ratified calls below; everything else is a pure refactor.
**Date:** 2026-10-14
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** ca35b73

**Sources:**
- `docs/reviews/architecture-review-2026-10-14-0848.html` (ground truth; candidates 1–3, "Smaller locality fixes", "Defect leads")
- `AGENTS.md`; Notes `coding-conventions`, `error-messages`, `cli-test-harness`, `issue-workflow`, `store-layout`, `adr-0001`, `adr-0002`, `adr-0007`, `adr-0009`

**Ratified design calls (user, 2026-10-14):**
- **Scope:** candidates 1–3, the defect leads, the small fixes except "Multi-argument results"; issues filed first under parent `arch-review-1014`.
- **Alias clash:** both nodes stay usable; only the shared alias is refused, in tree and index.
- **Unusable claims:** a name an unparsed, invalid, orphan or left-out file claims is refused by every verb, never prefix-matched past; only rm/know rm by the exact id of an unparsed live file still removes it.
- **Prefix candidates:** every file's id counts, unusable ones included; ambiguity is refused naming the candidates.
- **Membership depth:** full — pure membership function, index persists its verdicts, one resolver over tree maps and index SQL.
- **Repeated-id finding:** the index wording (one finding per claimant, "both are left out") everywhere; check's count rises by one per pair.
- **Alias findings:** both check and index give the aliases line and the kind noun ("issue"/"note").
- **Superseded-by edits:** follow dep/related: exact id/alias beats text, removal matches every alias spelling, ambiguous or doubly-claimed names refused, names-based already-present; a removal matching nothing stays a silent no-op.
- **know tree orphans:** a Note whose parent is not shown prints at the root, as issues do.
- **Resolution order:** a name that is one node's exact id and another file's alias or unusable claim is refused as a duplicate naming both files, by every verb (tree's order).

**Plan author calls (2026-10-14, no reachable visible change):**
- **Alias reader:** parsed YAML when a file parses; `rawfm` only when it does not.
- **Finding JSON:** `diagnose.Finding` takes check's shape (`code`, `line` always present); check and prime JSON stay byte-identical.
- **Kept as is:** `repeated-alias` folds into check code `duplicate`; conflict markers stay check-only; prime ignores index warnings; both related rm not-found wordings.
- **reflist location:** leaf package `internal/reflist`, importing only `tree`. **Outside-worktree path:** `store.RelativePath` returns the original path, slash-separated.
- **Lone-CR fence:** a closing `---\r` at end of file closes the frontmatter in `frontmatter.Split` too.

**Standing requirements:**
- skills: coding-standards
- Raspberry Pi 5: one package per test command with `GOMEMLIMIT=2GiB`; never `-race ./...`; never two test commands at once.
- Commit to `main`; one commit per item; no AI attribution anywhere. Never tag, bump `VERSION` or edit a CHANGELOG release heading.
- After any hand edit under `.capcom/`, run `capcom check`.

**Regression check (2026-10-14, ca35b73):**
- P1.1: guard folded
- P1.2: guard folded (decision)
- P1.3: guard folded; supersedes the 0.6.0 CHANGELOG "all are left out" entry for aliases
- P1.4: guard folded; yields to the 0.4.0 CHANGELOG entry on mv naming an unparseable moved file
- P1.5: guard folded (decision; adds the Resolution order call)
- P1.6: guard folded
- P1.7: recast (decision)
- P1.8: recast (decision; the index warnings now count unparsed and invalid claimants)
- P1.9: guard folded
- P2.1: guard folded
- P2.2: guard folded
- P2.4: guard folded
- P2.5: guard folded
- P2.6: guard folded
- P2.7: guard folded
- P2.8: guard folded (decision); supersedes the 0.4.0 CHANGELOG check alias wording entry
- P3.1: guard folded
- P3.3: guard folded
- P3.4: guard folded
- P3.5: guard folded; supersedes the 0.6.0 CHANGELOG Fixed entry and file-format.md on ambiguous-prefix removal
- P4.1: guard folded
- P4.2: guard folded (decision)
- P4.3: guard folded
- P4.4: recast (decision); yields to know_tree.go's background-subtree rule
- P4.5: guard folded (decision); yields to know_tree.go's background-filter placement
- P1.8: rejected — the reviewer's "NameClashes renders only clashes among usable claimants; an exact usable id or alias is never refused because an unusable file claims it"; the P1.8 and P1.5 decisions count every file holding an id and refuse an id another file claims unusably
- P1.9: rejected — "an exact usable id or alias wins; otherwise an unusable claim is refused"; the ratified Resolution order (P1.5 decision) refuses an id another file claims as an alias or unusably, tree's order
- P2.4: rejected — related_test.go breaks the build; it compares `problems[0].Code != check.CodeField`, Kind against Kind, which compiles
- Re-check round:
- P1.2: guard folded (decision; each file fact carries its origin, Walk node or claim-only)
- P1.7: guard folded (non-regular orphan .md flip named; per-entry listing errors; rawfm.Aliases grep and agreement_test.go)
- P1.8: guard folded (claim-only walk claims refuse names, never exclude; Goal restated on warning wording)
- P4.4: guard folded (a Note below an unparseable Note prints at the root inside a background or retired subtree)

**Out of scope:**
- Review candidates 4–10 and "Multi-argument results".
- Defect leads "six write sites drop Effects" and "update wraps ReplaceBody" (verified harmless); show's per-argument report of a doubly-claimed name; supersession cycle checks; nodestore `renameRefs`/`dropRefs` sharing `reflist`.

## P0.1 File the review issues — ✅ DONE (2026-10-14)

NOTES (2026-10-14): the review HTML and this plan were already tracked (ccd3f32, 5884f58), so neither file is edited or listed in FILES; the Goal's "tracked" clause holds on the tree as found.
NOTES (2026-10-14): claimed-name-prefix's body cites a reproduction made in a scratch repo: an orphan file .capcom/issues/ghost/foo.md plus a valid foo-bar makes `capcom rm foo` remove foo-bar; an unparseable foo.md did not trigger it (mv refuses with the parse error, create --parent files below foo/).

**What:**
**Goal:** `arch-review-1014` ("Architecture review 2026-10-14") has the nine children below, each with a self-contained body ending in `## Acceptance`; the review file and this plan are tracked; `capcom check` is clean.
**Approach (assumed at the header base):**
- `capcom create --id <id> --title '<title>' --parent arch-review-1014 --type <type> --priority <n> --plan '<this plan basename without .md>' --body-file <tmp>/<id>.md [--dep <id>]`; bodies in a `mktemp -d` outside the repo, per Note `issue-workflow`; symbols, never line numbers; last line `Source: docs/reviews/architecture-review-2026-10-14-0848.html, <section>.` Parent: type task, priority 2, Acceptance "every child issue is closed".
- Children (id — type, priority — title — deps):
  - `claimed-name-prefix` — bug, 1 — mv, rm and --parent prefix-match past a name a broken file claims
  - `membership-owner` — task, 2 — One owner for Tree membership and resolution — `claimed-name-prefix`
  - `validation-one-vocab` — task, 2 — One finding type and code set for check and the index — `membership-owner`
  - `supersede-dup-name` — bug, 1 — know update removes a superseded-by name two files claim
  - `reflist-module` — task, 2 — One module for reference-list edits — `supersede-dup-name`
  - `know-tree-orphans` — bug, 2 — know tree drops the children of an unparseable Note
  - `outline-visibility` — task, 3 — One visible-subtree outline for issues and Notes — `know-tree-orphans`
  - `fence-scanner` — task, 3 — One frontmatter fence predicate
  - `worktree-relpath` — task, 3 — Store owns worktree-relative paths
- Bodies state the ratified calls from this plan's header, not the review's options.
**Files:** .capcom/issues/arch-review-1014.md; .capcom/issues/arch-review-1014/<id>.md for the nine ids; docs/reviews/architecture-review-2026-10-14-0848.html; docs/plans/2026-10-14 - 00 - architecture-review-fixes-plan.md
**Read first:** .capcom/config.yaml — types (bug, task declared), plans dirs; .capcom/issues/code-review-1013.md — parent body shape, ## Acceptance, Source: line, plan: basename with spaces; capcom create --help — --parent, --dep, --plan, --body-file, --priority
**Tests:** none (no code).
**Acceptance:**
- `capcom check` exits 0
- `ls .capcom/issues/arch-review-1014/*.md | wc -l` prints 9
- `grep -L '^## Acceptance' .capcom/issues/arch-review-1014.md .capcom/issues/arch-review-1014/*.md` prints nothing
- `capcom dep list validation-one-vocab` lists `membership-owner`
- `git ls-files --error-unmatch docs/reviews/architecture-review-2026-10-14-0848.html` succeeds
**Commit:** `chore(issues): file the architecture review 2026-10-14 issues`

## P1.1 Characterise tree and index membership side by side

**What:**
**Goal:** one test runs every membership fixture through tree (`Load`, `Resolve`, `Claimant`) and the index (`Resolve`, `ResolveNote`) and asserts each answer; every case where they disagree today is listed in an explicit expected-divergence table, each row naming the ratified call that will reconcile it.
**Approach (assumed at the header base):** new `internal/index/agreement_test.go`. Fixtures: repeated id with subtree; alias clash between two files; unparsed live file claiming an alias (`zzz.md` with `aliases: [foo]` and broken YAML beside valid `foo-bar.md`); the same file in an orphan directory; a file that parses but fails validation; an alias listed only by the later file of a repeated id; prefix `dup` over `a/dup-x.md`, `b/dup-x.md`, `dup-y.md`; a non-regular file. Compare canonical id or error sentinel per query name.
**Regression guard.** Run each fixture as its own `t.Run` subtest, so `makeSymlink`'s `t.Skip` (Windows, root) drops only the non-regular-file case; name the top test `TestAgreement_*` so `-run Agreement` selects it. The `dup` fixture also writes `a.md` and `b.md` beside `a/dup-x.md` and `b/dup-x.md`, so `dup-x` is a repeated id (tree counts it through `prefixCandidates` over `LeftOut`, `index.resolveID` does not), not two orphan directories `tree.Walk` leaves out.
**Files:** internal/index/agreement_test.go
**Read first:** internal/index/query.go — resolveID, unusableClaim, duplicateClaim, leftOutPrefixCandidates; internal/tree/resolve.go — Resolve, prefixCandidates; internal/tree/claims.go — Claimant; internal/index/index_test.go — makeSymlink
**Tests:** the new `TestAgreement_*` test, one subtest per fixture.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/index/ -run Agreement`; `go vet ./internal/index/`
**Commit:** `test(index): characterise tree and index membership side by side`

## P1.2 One pure membership function in tree

**What:**
**Goal:** `tree` exports one pure function that, given per-file facts, decides kept nodes, left-out files with their reason, and every name's claimants with its refusal reason; `tree.Load` is built on it; no output changes.
**Approach (assumed at the header base):** one deep module, new `internal/tree/membership.go`. Input per file: an opaque path key (tree passes absolute paths, the index will pass worktree-relative ones), id, enclosing directory, aliases, parse state (parsed, unparsed, unread) and a caller-supplied `usable` flag (tree has no config). Output: structured verdicts, never rendered strings. `indexIDs`, `repeatedIDs`, `addRepeatedIDDirs`, `indexAliases`, `checkDroppedAliases` collapse into it. Depends on P1.1.
**Regression guard.** the membership function stays pure — directory-existence and enclosing-directory facts come from its caller, so Lstat calls (today in addRepeatedIDDirs) stay in tree.Load and the index walk; any tree-side parse takes a caller-supplied parse func, because tree cannot import knowledge. `tree.Load(dir, AliasFunc)` keeps its signature (43 test call sites; check and agents call it). `check.loadTree` and `agents.topNotes` render Load's problems, so `TestRun_DroppedDuplicateIDFiles_AliasClashAndParseFailureReported` and `TestTopNotes_SharedAlias_LeavesBothOut` must stay green: their packages join Acceptance; each file fact carries its origin — a Walk node, or a claim-only file the walk left out (a file under an orphan directory, or a file the walk refused to read) — and the function never makes a claim-only file a kept node; tree.Load (P1.7's orphan listing) and the index (P1.8's facts from claims rows) both set it, so tree and index verdicts agree
**Files:** internal/tree/membership.go; internal/tree/membership_test.go; internal/tree/claims.go; internal/tree/tree.go
**Read first:** internal/tree/tree.go — Load; internal/tree/claims.go — indexIDs, repeatedIDs, addRepeatedIDDirs, indexAliases, checkDroppedAliases; internal/check/problem.go — loadTree; internal/agents/block.go — topNotes
**Tests:** table test of the function over every P1.1 fixture shape, a claim-only file (orphan-directory or walk-refused) among them: it is never a kept node yet refuses the names it claims; existing tree, check and agents tests unchanged.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/tree/`; `GOMEMLIMIT=2GiB go test ./internal/index/ -run Agreement`; `GOMEMLIMIT=2GiB go test ./internal/nodestore/`; `GOMEMLIMIT=2GiB go test ./internal/check/`; `GOMEMLIMIT=2GiB go test ./internal/agents/`
**Commit:** `refactor(tree): one pure membership function under Load`

## P1.3 An alias clash refuses only the shared name

**What:**
**Goal:** when two files list the same alias, `capcom show <id>` of either node succeeds and resolving the alias fails as a duplicate naming both files, through the index and through tree alike.
**Approach (assumed at the header base):** regression from the index's clash rule: `excludeDuplicatesOf` excludes both files for an alias clash. Keep exclusion for repeated ids only; for an alias clash record the name as refused (`duplicateClaim`). The index warning uses the wording check reports for the same clash today. Remove the reconciled row from the P1.1 table. CHANGELOG (Unreleased, Fixed): an alias two nodes list no longer hides both nodes. Depends on P1.2.
**Regression guard.** `resolveID` checks refused (clashed) names before its exact id and alias lookups, as `tree.Resolve` does (the header's Resolution order; P1.5 extends it to unusable claims): for an id/alias clash the id itself is the refused name. `RelatedBy`'s alias subquery leaves refused names out. The warned file is picked as `tree.Load` picks it (nodes ordered by id, then path), or every claimant is warned, so `isIndexWarnedAbout` still covers the tree problem. Alias-clash and alias-of-another-id nodes become usable, one warning per clash: flip `TestResolve_NameOfExcludedNode_RefusesInsteadOfPrefixMatching`'s clash cases, `TestNotes_InvalidAndDuplicateNotes_WarnAndStayOut` and `TestNotes_EachProducer_SetsTheWarningKind`. Supersedes, for an alias, the 0.6.0 CHANGELOG entry ("…; all are left out" for a name several files share); the Fixed entry says so.
**Files:** internal/index/duplicates.go; internal/index/query.go; internal/index/index.go; internal/index/index_test.go; internal/index/query_test.go; internal/index/notes_test.go; internal/index/agreement_test.go; internal/cli/read_test.go
**Read first:** internal/index/duplicates.go — excludeDuplicatesOf, excludeClaims; internal/diagnose/clashes.go — NameClashes; internal/index/query.go — resolveID, duplicateClaim; internal/index/index.go — RelatedBy; internal/cli/errors.go — isIndexWarnedAbout; internal/index/notes_test.go — TestNotes_EachProducer_SetsTheWarningKind
**Tests:** index test of the clash (both ids resolve, the alias refused); a runCLI test: `show <id>` succeeds, `show <alias>` exits 1 naming both files; an id/alias clash refuses the id; `show` of either alias-clash node lists no reverse link through the refused alias; the flipped query and notes tests.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/index/`; `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestShow_'`
**Commit:** `fix(index): an alias clash refuses only the shared name`

## P1.4 tree refuses names that unusable files claim

**What:**
**Goal:** `capcom rm foo`, `capcom mv foo x` and `capcom create x --parent foo` refuse when `foo` is claimed by an unparsed, invalid, orphan or left-out file, with the same error kind and wording `index.Resolve` gives for that name; `rm`/`know rm` by the exact id of an unparsed live file still removes it.
**Approach (assumed at the header base):** fixes `claimed-name-prefix`: `tree.Resolve` never consults claims, so it prefix-matches `foo` to `foo-bar`. nodestore's `Kind.load` supplies the `usable` fact (validation with the store config) to the P1.2 function; `Resolve` and `Claimant` refuse claimed names; the remove path alone admits an exact unparsed live id (`TestKnowRm_UnparseableNote_RemovesItsFile` stays green). An alias listed only by a left-out repeated-id file is refused the same way (flip `TestResolve_AliasOnlyADroppedFileLists_IsNotADuplicate`). Remove reconciled P1.1 rows. CHANGELOG (Unreleased, Fixed). Depends on P1.3.
**Regression guard.** `usable` affects name claims only: `Nodes`, `ChildrenOf` and `LeftOut` are unchanged, so rm still sees an unusable file's deps. The exact id of an unparsed live file passes `Resolve` for mv and `--parent` too, which then fail at the parse with file:line, exit 1, as today — the item yields to the 0.4.0 CHANGELOG entry that mv names an unparseable moved file with its line. A live file that parses but fails validation is refused by its exact id for rm and mv (the ratified exception names unparsed files only); the CHANGELOG entry says so. The refusal is a `*tree.Error` with Path set, so `nodestore.resolveFailure` relativizes it.
**Files:** internal/tree/resolve.go; internal/tree/membership.go; internal/tree/resolve_test.go; internal/nodestore/load.go; internal/index/agreement_test.go; internal/cli/rm_test.go; internal/cli/mv_test.go; internal/cli/write_test.go
**Read first:** internal/tree/resolve.go — Resolve, prefixCandidates; internal/nodestore/load.go — resolveFailure, relativeTreeError; internal/nodestore/move.go — Move, planFailure; internal/nodestore/refs.go — planNodeRemoval; internal/cli/errors.go — kindFailure
**Tests:** runCLI repro for rm, mv and create --parent (exit code, message, `foo-bar.md` untouched); tree unit tests for each unusable kind; stderr names no absolute root; rm and mv of a parseable invalid file refused; `TestMv_UnparseableIssue_FailsInvalidAtItsRelativeFileAndLine`, `TestKnowMv_UnparseableNote_FailsInvalidAtItsRelativeFileAndLine` and `TestRm_UnparseableIssue_WarnsDepsNotChecked` stay green.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/tree/`; `GOMEMLIMIT=2GiB go test ./internal/nodestore/`; `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestRm_|TestMv_|TestCreate_|TestKnowRm_|TestKnowMv_|TestKnowAdd_'`
**Closes:** claimed-name-prefix
**Commit:** `fix(tree): refuse names that unusable files claim`

## P1.5 Index prefix candidates count every file's id

**What:**
**Goal:** through the index, a prefix that matches the id of any file — unparsed, invalid, excluded or left out included — and another id is refused as ambiguous naming every candidate; an alias listed only by a left-out repeated-id file gets the unusable-claim refusal, not a duplicate error.
**Approach (assumed at the header base):** `resolveID` and `leftOutPrefixCandidates` add excluded and claim-only ids to the candidate set; `duplicateClaim` yields to `unusableClaim` for left-out claimants. Remove the reconciled P1.1 rows. CHANGELOG (Unreleased, Changed). Depends on P1.4.
**Regression guard.** add to the header's Ratified design calls the line "**Resolution order:** a name that is one node's exact id and another file's alias or unusable claim is refused as a duplicate naming both files, by every verb (tree's order)." (user, 2026-10-14); the index adopts that order in this item, with a runCLI show test; the unusable-claim refusal keeps the error kind index.Resolve gives today (not-found with the "is invalid" message), so dep rm, related rm and know update still remove such a name by its text (TestDepRm_IDOfInvalidFile_RemovesTheExactEntry stays green). A claim-only candidate is `tree.IDOf(claims.path)`, never `claims.name`; candidates are deduped and sorted as `tree.prefixCandidates` does. The refusal for an alias only the later file of a repeated id lists is built from `aliases` joined to `issues`/`notes` where `excluded = 1` (that file has no claims rows). `TestResolve_NameOfExcludedNode_RefusesInsteadOfPrefixMatching`'s `fo` assertion flips to a `*tree.AmbiguousError` naming `foo` (or `foo-rework`) and `foo-bar`.
**Files:** internal/index/query.go; internal/index/query_test.go; internal/index/agreement_test.go; internal/cli/read_test.go
**Read first:** internal/index/query.go — resolveID, unusableClaim, duplicateClaim, leftOutPrefixCandidates; internal/index/refresh.go — claimedNames; internal/tree/resolve.go — prefixCandidates; internal/cli/dep.go — isRemovedByText; internal/index/query_test.go — TestResolve_NameOfExcludedNode_RefusesInsteadOfPrefixMatching
**Tests:** index tests for the `dup` fixture and the left-out alias; a runCLI `show dup` test naming both candidates; a runCLI `show` test of the resolution order (an id another file lists as an alias or claims unusably refused, naming both files); the flipped `TestResolve_NameOfExcludedNode_RefusesInsteadOfPrefixMatching`.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/index/`; `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestShow_|TestDepRm_|TestRelatedRm_|TestKnowUpdate'`
**Commit:** `fix(index): prefix candidates count every file's id`

## P1.6 Index leaves out repeated-id subtrees through the membership function

**What:**
**Goal:** the index computes repeated-id subtrees by calling tree's membership function; no repeated-id logic remains in `internal/index`; warning text unchanged.
**Approach (assumed at the header base):** `walkIssues` builds facts from `tree.Walk` and calls the P1.2 function; delete `leaveOutRepeatedIDSubtrees`, `enclosingRepeatedIDDirectory`, `repeatedIDDirectoryProblems`. Depends on P1.5.
**Regression guard.** A per-directory Lstat stays after the membership function, shared with Load's `addRepeatedIDDirs`: warn only when it is a directory, and fail the refresh on any Lstat error other than not-exist, naming the directory relative to the worktree. Every key the function returns maps back to an absolute path before Lstat or `WalkDir`; at walk time pass only path, id and directory facts, and state what the walk passes for aliases, parse state and usable. Order claimants by the OS-native relative path (or the absolute path), so "first" is the same file in tree and index.
**Files:** internal/index/walk.go; internal/index/refresh.go; internal/index/refresh_test.go; internal/tree/claims.go
**Read first:** internal/index/walk.go — walkIssues, leaveOutRepeatedIDSubtrees, repeatedIDDirectoryProblems, problemClaims, orphanDirectoryClaims; internal/tree/claims.go — addRepeatedIDDirs; internal/index/refresh_test.go — TestRefresh_DuplicateIDWithChildren_LeavesTheSubtreeOut; internal/index/index_test.go — TestRefresh_DirectoryWarnings_WordedAsTheTreeWalk
**Tests:** existing repeated-id refresh tests and `TestRefresh_DirectoryWarnings_WordedAsTheTreeWalk` stay green; a repeated-id directory holding no `.md` file still warns `directory left out`.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/index/`; `GOMEMLIMIT=2GiB go test ./internal/tree/`; `grep -n 'leaveOutRepeatedIDSubtrees\|enclosingRepeatedIDDirectory' -r internal/` prints nothing
**Commit:** `refactor(index): repeated-id subtrees from tree's membership function`

## P1.7 One orphan listing and one alias reader

**What:** Recast at the regression check (2026-10-14).
**Goal:** tree exports the listing of files under orphan directories and one alias reader (parsed aliases when the file parses, `rawfm` only when it does not); `orphanDirectoryClaims`, `claimedNames`, `RecoverAliases` and nodestore's `planNodeRemoval` all use them.
**Approach (assumed at the header base):** promote `findOrphanFile`/`listNodeFiles` to one exported listing; one `tree` alias reader replaces the three choices. Depends on P1.6.
**Regression guard.** the one orphan listing keeps non-regular .md entries and returns its errors; the index keeps skipping unreadable entries and Claimant keeps failing on them; invalid-id names stay in the listing and the index keeps skipping them as today; Claimant's recover step still reads only unparseable files (a parsed file yields none there), so TestRecoverAliases_ParseableAndUnparseableFiles stays green and internal/nodestore/kind_test.go joins Files; the tree alias reader takes the file's bytes plus a caller-supplied parse func (the kind's codec parse or diagnose's parse), so planNodeRemoval still reads the file once. Claimant now claims a non-regular orphan .md by its id, never its aliases: `TestClaimant_NonRegularFileInOrphan_NeverClaims` flips and Claimant's doc ("only regular files ... are read") is updated; the tree side claims it too, as a claim-only fact (P1.2). The listing reports errors per entry (files listed so far plus the failed paths): the index skips a failed entry and keeps every readable orphan file's claims, Claimant fails on it. walkIssues takes the kind's parse func from walkStore.
**Files:** internal/tree/claims.go; internal/tree/claims_test.go; internal/index/walk.go; internal/index/refresh.go; internal/index/agreement_test.go; internal/nodestore/kind.go; internal/nodestore/kind_test.go; internal/nodestore/refs.go
**Read first:** internal/tree/claims.go — listNodeFiles, Claimant; internal/tree/claims_test.go — TestClaimant_NonRegularFileInOrphan_NeverClaims; internal/index/walk.go — orphanDirectoryClaims;
internal/index/refresh.go — claimedNames, walkStore; internal/nodestore/kind.go — RecoverAliases; internal/nodestore/refs.go — planNodeRemoval
**Tests:** in `internal/index/agreement_test.go`, a test where a parsed file below an orphan directory lists aliases as a multi-line flow list: tree and index claim the same names; a symlinked `.md` below an orphan directory still claims its id (no prefix match past it), and `TestClaimant_NonRegularFileInOrphan_NeverClaims` is rewritten to expect that claim by id and none by alias; an orphan directory with an unreadable (mode-000) subdirectory: the index still claims every readable file's names and Claimant fails; `TestRecoverAliases_ParseableAndUnparseableFiles` stays green.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/tree/`; `GOMEMLIMIT=2GiB go test ./internal/index/`; `GOMEMLIMIT=2GiB go test ./internal/nodestore/`; `grep -rn 'rawfm.Aliases' internal/index internal/nodestore` prints nothing
**Commit:** `refactor(tree): one orphan listing and one alias reader`

## P1.8 The index persists membership verdicts

**What:** Recast at the regression check (2026-10-14).
**Goal:** after each refresh the index stores the membership function's verdicts (kept, left out with reason, refused names) in its database and answers exclusion and claim queries from them; `excludeDuplicates` is gone; warning wording unchanged; a repeated id counts unparsed and invalid files, warning about and excluding both.
**Approach (assumed at the header base):** after `sync`, build facts from stored rows (clean files: the `aliases` table; problem files: `claims`; orphan files: read fresh) and call the function; persist into a verdicts table; bump `schemaVersion` by one (automatic rebuild). `diagnose.NameClashes` renders verdicts or is deleted, output byte-identical. Depends on P1.7.
**Regression guard.** the verdicts count every file holding an id — parsed or not, valid or not — as tree.Load does, so P2.7's single wording holds; the index warnings change accordingly (CHANGELOG Unreleased, Changed). The facts are built after `replaceWalkClaims` from every claims row (a walk-refused file — a top-level symlink, a file over 1 MiB — has only those), and orphan files reuse `r.walkClaims` instead of being read again on every refresh. The verdicts still set `issues`/`notes.excluded`, which every `excluded = 0` filter reads. Only `files` rows (Walk's node files, parsed or not) count as id holders for repeated ids; walk claims (orphan-directory files, walk-refused symlink or over-1-MiB files) are claim-only facts that refuse names, never exclude.
**Files:** internal/index/schema.go; internal/index/refresh.go; internal/index/duplicates.go; internal/index/query.go; internal/index/index_test.go; internal/diagnose/clashes.go; internal/diagnose/clashes_test.go
**Read first:** internal/index/refresh.go — refresh, replaceWalkClaims; internal/index/duplicates.go — excludeDuplicatesOf; internal/index/walk.go — orphanDirectoryClaims; internal/diagnose/clashes.go — NameClashes; internal/index/schema.go — schemaVersion;
internal/tree/tree.go — Load; internal/index/query_test.go — TestResolve_NameOfExcludedNode_RefusesInsteadOfPrefixMatching
**Tests:** a refresh test that an unchanged file is not re-read while verdicts stay correct after a sibling changes; a repeated id with one unparsed file warns and excludes both; valid `login.md` beside orphan `old/login.md` (no `old.md`): `login` stays listed and only the orphan directory is warned about; the `symlinked file` and `file over 1 MiB` resolve cases stay green; the other pinned warning tests stay green.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/index/`; `GOMEMLIMIT=2GiB go test ./internal/diagnose/`; `grep -n 'func excludeDuplicates' -r internal/` prints nothing
**Commit:** `refactor(index): persist tree's membership verdicts`

## P1.9 One resolver over tree maps and index SQL

**What:**
**Goal:** `tree.Resolve` is the only resolution algorithm, written against a small names interface that tree's in-memory maps and the index's SQL snapshot both implement; the P1.1 test asserts full agreement with no divergence table.
**Approach (assumed at the header base):** interface: exact name → claimants with usability; prefix → candidate ids. `resolveID`, `unusableClaim`, `duplicateClaim` shrink to the SQL adapter inside the read snapshot. Prose guard: every comment in `internal/index` that says it mirrors tree (`grep -rn 'tree\.Load does\|tree\.Resolve' internal/index`) is reworded or removed. Add the membership rules (ratified calls above) as a section Note under `adr-0002` via `capcom know add --parent adr-0002`. Depends on P1.8.
**Regression guard.** The resolver follows the header's Resolution order call. `internal/index/query.go` keeps no `unusableClaim`, `duplicateClaim` or `leftOutPrefixCandidates` method and `agreement_test.go` holds no divergence table; Acceptance greps both.
**Files:** internal/tree/resolve.go; internal/tree/resolve_test.go; internal/index/query.go; internal/index/agreement_test.go; .capcom/knowledge/decisions/adr-0002/<new note>.md
**Read first:** internal/tree/resolve.go — Resolve, prefixCandidates, Lookup; internal/index/query.go — Resolve, resolveID; internal/index/notes.go — ResolveNote; internal/cli/know_update.go — isResolveMiss; internal/nodestore/load.go — resolveFailure
**Tests:** agreement test with every fixture agreeing.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/tree/`; `GOMEMLIMIT=2GiB go test ./internal/index/`; `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestShow_|TestClose_|TestRm_|TestMv_'`; `grep -n 'func (x \*Index) \(unusableClaim\|duplicateClaim\|leftOutPrefixCandidates\)' internal/index/query.go` prints nothing; `grep -in 'diverg' internal/index/agreement_test.go` prints nothing; `capcom check`
**Closes:** membership-owner
**Commit:** `refactor(tree): one resolver over tree maps and index SQL`

## P2.1 Pin the validation surface

**What:**
**Goal:** tests pin the closed set of check code strings, the JSON element shape of check `data` and prime `broken_refs` (`line: 0` present), and a Note-tree twin of the per-file problem wording pin in check and the index.
**Approach (assumed at the header base):** twins of `TestRun_FileProblems_PinCodesAndWording` and `TestRefresh_FileProblems_PinKindsAndWording` for Notes; a closed-set test over every `check.CodeX`; a prime JSON element test (exact key set, nonzero line).
**Regression guard.** `line: 0` is pinned only in check `data` (an orphan-directory or not-regular problem): a prime `broken_refs` element always has a line ≥ 2 (`Field.List` lines every ref), so its pin is the exact key set {path, line, code, message} with that nonzero line. The new cli tests are named `TestCheck_*`/`TestPrime_*` so the Acceptance `-run` selects them.
**Files:** internal/check/check_test.go; internal/index/notes_test.go; internal/cli/check_test.go; internal/cli/prime_test.go
**Read first:** internal/check/check_test.go — TestRun_FileProblems_PinCodesAndWording; internal/index/index_test.go — TestRefresh_FileProblems_PinKindsAndWording; internal/index/notes_test.go — TestNotes_EachProducer_SetsTheWarningKind; internal/cli/check_test.go — TestCheck_JSONWithProblems_IsKindCheckWithProblemList; internal/cli/prime_test.go — TestPrime_JSON_CarriesEverySection; internal/briefing/briefing.go — readHealth
**Tests:** the new pins.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/check/`; `GOMEMLIMIT=2GiB go test ./internal/index/ -run 'Notes|FileProblems'`; `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestCheck_|TestPrime_'`
**Commit:** `test(check): pin the validation codes and JSON shapes`

## P2.2 Drop the index Warning re-exports

**What:**
**Goal:** `internal/index` declares no `Warning`, `WarningKind` or `WarningX` names; producers, cli and tests use `diagnose.Finding` and `diagnose` kinds; output unchanged.
**Approach (assumed at the header base):** delete the re-export block in `index.go`; `Report.Warnings` becomes `[]diagnose.Finding`; update `cli/root.go`, `cli/mv.go` (`movedWarnings`), `cli/errors.go` (`isIndexWarnedAbout`). Depends on P2.1.
**Regression guard.** `excludeClaims` (duplicates.go), `parseIssue` and `aliasProblems` (problems.go) and `parseNote` (notes.go) name `Warning` too and change with the block. Acceptance also greps the package's own declarations, so no `type Warning =` alias survives.
**Files:** internal/index/index.go; internal/index/refresh.go; internal/index/duplicates.go; internal/index/problems.go; internal/index/notes.go; internal/index/index_test.go; internal/index/notes_test.go; internal/index/refresh_test.go; internal/cli/root.go; internal/cli/mv.go; internal/cli/errors.go
**Read first:** internal/index/index.go — WarningKind, Warning, Report.Warnings; internal/index/duplicates.go — excludeClaims; internal/index/problems.go — parseIssue, aliasProblems; internal/index/notes.go — parseNote; internal/cli/mv.go — movedWarnings; internal/cli/errors.go — isIndexWarnedAbout
**Tests:** existing.
**Acceptance:** `go build ./...`; `GOMEMLIMIT=2GiB go test ./internal/index/`; `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestMv_|TestReindex|TestErrors_'`; `grep -rn 'index\.Warning' internal/` prints nothing; `grep -nE 'type Warning|\bWarningKind\b|\bWarning[A-Z][A-Za-z]* *=' internal/index/*.go` prints nothing
**Commit:** `refactor(index): drop the Warning re-exports`

## P2.3 check adds parse failures from the finding directly

**What:**
**Goal:** check records a parse failure from its `diagnose.Finding` without rebuilding an `issue.Error`/`knowledge.Error`; output unchanged.
**Approach (assumed at the header base):** `addParseFailure` takes a `diagnose.Finding`; `readAliases` in `issues.go`/`notes.go` passes `findings[0]`. Depends on P2.2.
**Files:** internal/check/check.go; internal/check/issues.go; internal/check/notes.go
**Read first:** internal/check/check.go — addParseFailure, checkConflictMarkers; internal/check/issues.go — checker.issues (readAliases); internal/check/notes.go — checker.notes (readAliases); internal/diagnose/files.go — ParseIssue, ParseNote, parseFinding; internal/check/check_test.go — TestRun_ConflictMarkersInFrontmatter_ReportedOnceWithoutAFrontmatterProblem
**Tests:** existing, incl. `TestRun_ConflictMarkersInFrontmatter_ReportedOnceWithoutAFrontmatterProblem`.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/check/`
**Commit:** `refactor(check): add parse failures from the finding directly`

## P2.4 One code set, part 1: diagnose owns every code

**What:**
**Goal:** every check code is a `diagnose.Kind`; `check.Problem.Code` is typed `diagnose.Kind`; check JSON unchanged.
**Approach (assumed at the header base):** add the ten check-only kinds (config, conflict, missing-dep, missing-related, missing-superseded-by, format-too-old, cycle, missing-plan, broken-ref, agents-block) to `diagnose`; `check.CodeX` become aliases of them. `repeated-alias` still folds into `duplicate` for check (one mapping function kept). Depends on P2.3.
**Regression guard.** `check_test.go` (`locations`, every `problem.Code+" "+…` and `check.CodeX + "…"` want) and `agents_test.go` concatenate codes with strings; they convert with `string(problem.Code)` / `string(check.CodeX)` in this item, so `go test ./internal/check/` builds.
**Files:** internal/diagnose/diagnose.go; internal/diagnose/diagnose_test.go; internal/check/problem.go; internal/check/check.go; internal/check/issues.go; internal/check/notes.go; internal/check/agents.go; internal/check/check_test.go; internal/check/agents_test.go
**Read first:** internal/check/problem.go — Code constants, Problem, kindCode, checker.loadTree; internal/diagnose/diagnose.go — Kind, TreeKind; internal/check/check.go — checker.add; internal/check/check_test.go — locations
**Tests:** P2.1 closed-set pin stays green.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/diagnose/`; `GOMEMLIMIT=2GiB go test ./internal/check/`
**Commit:** `refactor(diagnose): own every check code`

## P2.5 One code set, part 2: delete check's code constants

**What:**
**Goal:** no `check.CodeX` constants remain; callers and tests use `diagnose` kinds.
**Approach (assumed at the header base):** mechanical rename at every site (`grep -rn 'check\.Code\|\bCode[A-Z]' internal/check internal/briefing`), then delete the aliases. Depends on P2.4.
**Regression guard.** The rename covers `internal/check` and `internal/briefing` only — cli's `output.Code*` and `rawfm.CodeFence` are unrelated — including the unqualified sites in check.go, issues.go, notes.go and agents.go. Acceptance also greps that no `CodeX` declaration remains in `internal/check`.
**Files:** internal/check/problem.go; internal/check/check.go; internal/check/issues.go; internal/check/notes.go; internal/check/agents.go; internal/check/check_test.go; internal/check/related_test.go; internal/check/agents_test.go; internal/briefing/briefing.go; internal/briefing/briefing_test.go
**Read first:** internal/check/problem.go — Code constants, kindCode; internal/check/issues.go — checkIssue, checkCycles; internal/check/notes.go — checkNote; internal/check/agents.go — agentsBlock; internal/check/check.go — ConfigProblems; internal/briefing/briefing.go — readHealth
**Tests:** existing.
**Acceptance:** `go build ./...`; `GOMEMLIMIT=2GiB go test ./internal/check/`; `GOMEMLIMIT=2GiB go test ./internal/briefing/`; `grep -rn 'check\.Code' internal/` prints nothing; `grep -n '\bCode[A-Z]' internal/check/*.go` prints nothing
**Commit:** `refactor(check): use diagnose kinds for every code`

## P2.6 One finding type

**What:**
**Goal:** `check.Problem` and `diagnose.Finding` are one type with check's JSON shape (`path`, `line` always present, `code`, `message`); check and prime JSON are byte-identical to before.
**Approach (assumed at the header base):** `diagnose.Finding` renames its `kind` JSON key to `code` and drops `omitempty` on `line`; `check.Problem` is removed or aliased; the index's stored `files.problems` changes shape, so bump `schemaVersion` by one. Depends on P2.5.
**Regression guard.** `diagnose.Finding`'s fields are ordered Path, Line, Code, Message, as `check.Problem` has them; P2.1/P2.6 pin the raw bytes of one check `--json` element, not a decoded map. The item names the one Go field kept (`Code` or `Kind`) and converts every site by grep (`grep -rn '\.Code\b\|Code:' internal/` or `'\.Kind\b\|Kind:'`), not a file list; `check.Problem` stays as an alias of `diagnose.Finding`. Acceptance adds `go vet ./...` and the briefing tests.
**Files:** internal/diagnose/files.go; internal/diagnose/files_test.go; internal/check/problem.go; internal/check/check.go; internal/index/schema.go; internal/briefing/briefing.go; internal/cli/prime.go; internal/cli/check_test.go
**Read first:** internal/diagnose/files.go — Finding, Finding.String; internal/check/problem.go — Problem, kindCode; internal/index/refresh.go — rewarn, indexFile; internal/briefing/briefing.go — readHealth; internal/cli/check_test.go — TestCheck_JSONWithProblems_IsKindCheckWithProblemList
**Tests:** P2.1 JSON pins stay green; a raw-bytes pin of one check `--json` element; `TestRefresh_BadStoredProblems_FailsNamingTheFile` stays green.
**Acceptance:** `go build ./...`; `go vet ./...`; `GOMEMLIMIT=2GiB go test ./internal/diagnose/`; `GOMEMLIMIT=2GiB go test ./internal/check/`; `GOMEMLIMIT=2GiB go test ./internal/index/`; `GOMEMLIMIT=2GiB go test ./internal/briefing/`; `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestCheck_|TestPrime_'`
**Commit:** `refactor(diagnose): one finding type for check and the index`

## P2.7 One repeated-id finding for check and the index

**What:**
**Goal:** `capcom check` reports a repeated id with the index's wording, one problem per claimant ("…also the id of P; both are left out"), code `duplicate`.
**Approach (assumed at the header base):** check's `loadTree` renders the membership verdict through the same function the index uses (`diagnose.NameClashes` or its P1.8 successor) instead of tree's `DuplicateIDError` text. Update the pins. CHANGELOG (Unreleased, Changed): check reports both files of a repeated id. Depends on P2.6 and P1.8.
**Regression guard.** Every file of the id counts as a claimant, unparsed and invalid ones included, as `tree.Load` counts them; `TestRun_DroppedDuplicateIDFiles_AliasClashAndParseFailureReported` is repinned to three "and 1 more; all are left out" findings. docs/site/reference/file-format.md (repeated-id bullet) is reworded to one problem per file of the id, naming another claimant.
**Files:** internal/check/problem.go; internal/check/check_test.go; internal/cli/check_test.go; internal/cli/prime_test.go; docs/site/reference/file-format.md
**Read first:** internal/check/problem.go — loadTree, kindCode; internal/diagnose/clashes.go — NameClashes, clashMessage; internal/tree/claims.go — DuplicateIDError; internal/check/check_test.go — TestRun_DroppedDuplicateIDFiles_AliasClashAndParseFailureReported; internal/cli/prime_test.go — TestPrime_UnparseableIssue_CountsItInCheckWithoutAWarningLine; internal/index/duplicates.go — excludeDuplicatesOf
**Tests:** check and runCLI tests of a repeated id: two problems, exact wording, prime count 2; three files of one id, one unparsed: three problems.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/check/`; `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestCheck_|TestPrime_'`
**Commit:** `feat(check): report both files of a repeated id`

## P2.8 Alias findings give the line and the kind noun

**What:**
**Goal:** check and the index report an alias problem with the frontmatter `aliases` line and the noun "issue" or "note", identically.
**Approach (assumed at the header base):** `diagnose.AliasFindings` / `tree.CheckAliases` take the kind noun and always set the line (`AliasesLine`); tree.Load's "node" noun goes. Update pins in check and index. CHANGELOG (Unreleased, Changed). Depends on P2.7.
**Regression guard.** the noun change lands wherever the alias check lives after P1.2 (internal/tree/membership.go if P1.2 moved it there); Files names that file and claims.go. `aliasProblems` (index/problems.go, also used by index/notes.go) changes with `AliasFindings`; every `tree.Load` caller is found by rule (`grep -rn 'tree\.Load(\|= Load(' internal`), or check renders alias findings itself through `diagnose.AliasFindings` with its noun. `TestCreate_IndexWarnedFileSelfAlias_WarnsOnce`'s two wants gain the aliases line. Supersedes the 0.4.0 CHANGELOG entry recording check's `alias "<alias>" repeats the node's own id` wording (ratified Alias findings call).
**Files:** internal/diagnose/files.go; internal/diagnose/files_test.go; internal/tree/claims.go; internal/tree/membership.go; internal/tree/tree.go; internal/tree/tree_test.go; internal/index/problems.go; internal/index/notes.go; internal/check/check_test.go; internal/index/index_test.go; internal/index/notes_test.go; internal/cli/write_test.go
**Read first:** internal/diagnose/files.go — AliasFindings; internal/tree/ids.go — CheckAliases, checkAlias; internal/tree/claims.go — indexAliases, checkDroppedAliases; internal/index/problems.go — aliasProblems; internal/cli/errors.go — isIndexWarnedAbout; internal/cli/write_test.go — TestCreate_IndexWarnedFileSelfAlias_WarnsOnce
**Tests:** P3.60/P2.1 wording pins updated to the one rendering; an assertion both producers emit the same string for one fixture; `TestLoad_RepeatedAliases_DropOnlyTheOffendingAlias` and `TestCreate_IndexWarnedFileSelfAlias_WarnsOnce` repinned.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/diagnose/`; `GOMEMLIMIT=2GiB go test ./internal/tree/`; `GOMEMLIMIT=2GiB go test ./internal/check/`; `GOMEMLIMIT=2GiB go test ./internal/index/`; `GOMEMLIMIT=2GiB go test ./internal/nodestore/`; `GOMEMLIMIT=2GiB go test ./internal/agents/`; `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestCreate_'`
**Commit:** `feat(diagnose): alias findings name the line and the kind`

## P2.9 One per-file verdict replaces the diagnose wrappers

**What:**
**Goal:** `diagnose` exposes one per-file verdict per kind (parse, validate and alias findings in one call); `ParseIssue`, `ParseNote`, `ValidateIssue`, `ValidateNote` are gone; the index uses the whole verdict, check the parse part plus its own validation of loaded nodes; output unchanged.
**Approach (assumed at the header base):** `diagnose.IssueFile` / `NoteFile`; update `index/problems.go`, `index/notes.go`, `check/issues.go`, `check/notes.go`; reword the briefing comment about index warnings. Depends on P2.8.
**Files:** internal/diagnose/files.go; internal/diagnose/files_test.go; internal/index/problems.go; internal/index/notes.go; internal/check/issues.go; internal/check/notes.go; internal/briefing/briefing.go
**Read first:** internal/diagnose/files.go — ParseIssue, ParseNote, ValidateIssue, ValidateNote, parseFinding; internal/index/problems.go — parseIssue; internal/index/notes.go — parseNote; internal/briefing/briefing.go — readWork
**Tests:** a verdict table test in diagnose; existing pins stay green.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/diagnose/`; `GOMEMLIMIT=2GiB go test ./internal/index/`; `GOMEMLIMIT=2GiB go test ./internal/check/`; `grep -rn 'func Parse\(Issue\|Note\)\|func Validate\(Issue\|Note\)' internal/diagnose` prints nothing
**Closes:** validation-one-vocab
**Commit:** `refactor(diagnose): one per-file verdict`

## P3.1 Pin reference-list edits

**What:**
**Goal:** runCLI tests pin today's dep, related and know update reference edits for the cases not yet covered.
**Approach (assumed at the header base):** add: related rm on a doubly-claimed name (refused); dep rm and related rm with an ambiguous prefix and no text match (invalid, candidates named); related rm's `"is not related to <arg>"` message; `NormalizeArg` text removal (`"Gone Issue"`); know update add beside an alias-spelled entry (two entries today).
**Regression guard.** Pins compare full stderr, not `Contains`: dep rm not-found `capcom: .capcom/issues/login-flow.md: login-flow does not depend on db-schema` (exit 3); related rm `capcom: .capcom/issues/login-flow.md: login-flow is not related to absent` (exit 3); an ambiguous prefix with no path, e.g. `capcom: ambiguous id prefix "b": matches build-pipeline, build-plan` (exit 1). Also pin today's: `dep rm a a` removes a hand-written self-dep; `dep add alpha beta` exits 0 when that edge already sits in a hand-written cycle; `create --dep "gamma,,beta"` skips the empty entry; `create --id foo-bar --dep foo-bar` with foo-bar taken fails exists.
**Files:** internal/cli/dep_test.go; internal/cli/related_test.go; internal/cli/know_update_test.go; internal/cli/write_test.go
**Read first:** internal/cli/dep_test.go — TestDepRm_NotADep_FailsNotFound, newDepStore; internal/cli/related_test.go — TestRelatedRm_NotRelated_FailsNotFound, newRelatedStore; internal/cli/rm_test.go — assertDuplicateRefusal; internal/cli/know_test.go — newKnowStore, writeNoteFiles; internal/cli/know_update_test.go — readNoteFile
**Tests:** the new pins.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestDep|TestRelated|TestKnowUpdate|TestCreate'`
**Commit:** `test(cli): pin reference-list edits`

## P3.2 know update refuses a superseded-by name two files claim

**What:**
**Goal:** `capcom know update <id> --remove-superseded-by <name>` exits 1 naming both files and writes nothing when two files claim `<name>`, as `dep rm` does.
**Approach (assumed at the header base):** fixes `supersede-dup-name`: `isResolveMiss` counts `tree.ErrDuplicate` as a miss; stop doing so. Update `docs/site/reference/file-format.md` (superseded-by removal). CHANGELOG (Unreleased, Fixed). Depends on P3.1.
**Files:** internal/cli/know_update.go; internal/cli/know_update_test.go; docs/site/reference/file-format.md
**Read first:** internal/cli/know_update.go — resolveRemovals, isResolveMiss; internal/index/query.go — duplicateClaim; internal/cli/output/errors.go — AsError; internal/cli/rm_test.go — assertDuplicateRefusal; internal/cli/know_test.go — newKnowStore, writeNoteFiles; docs/site/reference/file-format.md — superseded-by removal paragraph
**Tests:** runCLI repro: Notes `foo.md` and `testing-rules/foo.md`, `release-flow` superseded by `[foo, cli-tests]`; exit 1, both paths named, file unchanged.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/cli/ -run TestKnowUpdate`
**Closes:** supersede-dup-name
**Commit:** `fix(cli): know update refuses a superseded-by name two files claim`

## P3.3 The reflist package

**What:**
**Goal:** `internal/reflist` decides add and remove on a reference list: given the holder, its entries, the argument, a resolver and an entry-names lookup, it returns the canonical id and whether it is present, the entries to drop, or a typed refusal (self, cycle, not found, ambiguous, duplicate).
**Approach (assumed at the header base):** pure leaf package importing only `tree` (sentinel errors). Rules (binding, from dep/related): exact id/alias beats text, text beats prefix; removal drops every entry naming the target by id or any alias, or matching the argument or `NormalizeArg(arg)` as text; already-present is names-based; cycle check through an optional `FindCycle`; a no-match removal returns a typed not-found the caller may ignore. Depends on P3.2.
**Regression guard.** Self and cycle apply to add only, in the order self → already-present (names-based) → cycle; removal never refuses self (`dep rm a a` removes a hand-written self-dep), and an add already present is a no-op even when the edge closes a hand-written cycle.
**Files:** internal/reflist/reflist.go; internal/reflist/reflist_test.go
**Read first:** internal/cli/dep.go — isRemovedByText, isEntryText, depRemoval, newDepAddCommand; internal/cli/related.go — relatedRemoval; internal/tree/resolve.go — NormalizeArg, AmbiguousError; internal/issue/deps.go — DepGraph.FindCycle
**Tests:** one table test covering every rule × outcome, with rows for removing a self entry and for adding a present edge inside a cycle.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/reflist/`; `go vet ./internal/reflist/`; `go list -deps ./internal/reflist | grep capcom/internal` lists only reflist and tree (and tree's own deps)
**Commit:** `feat(reflist): one module for reference-list edits`

## P3.4 dep, related and create --dep use reflist

**What:**
**Goal:** dep add/rm, related add/rm and create --dep decide through `reflist`; cli maps flags and refusals only; output unchanged.
**Approach (assumed at the header base):** delete `depRemoval`, `isRemovedByText`, `isEntryText`, `relatedRemoval`'s matching and `resolveDeps`' matching; refusals map to `output.Error` in one function in `errors.go`. Collapse the five mirrored dep/related runCLI pairs into one smoke test per command (message, exit code). Depends on P3.3.
**Regression guard.** The argument is resolved before the edit, failing through `resolveFailure` with no path as today; reflist's entry decision is applied to `target.Deps`/`target.Related` inside the edit closure, so not-found keeps its file path and a stale `--if-match` still wins. create --dep passes no holder and no `FindCycle`, uses an identity names lookup (no `loadDepGraph` on create) and skips `""` before calling reflist.
**Files:** internal/cli/dep.go; internal/cli/related.go; internal/cli/create.go; internal/cli/dep_graph.go; internal/cli/errors.go; internal/cli/dep_test.go; internal/cli/related_test.go
**Read first:** internal/cli/dep.go — newDepRmCommand, depRemoval, isRemovedByText; internal/cli/related.go — relatedRemoval; internal/cli/create.go — resolveDeps; internal/cli/dep_graph.go — loadDepGraph; internal/cli/node_edit.go — editNode; internal/cli/show.go — resolveFailure
**Tests:** P3.1 pins and remaining smoke tests stay green.
**Acceptance:** `go build ./...`; `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestDep|TestRelated|TestCreate'`; `grep -rn 'isRemovedByText\|isEntryText\|depRemoval\|relatedRemoval' internal/cli` prints nothing
**Commit:** `refactor(cli): dep, related and create --dep use reflist`

## P3.5 know update superseded-by follows reflist

**What:**
**Goal:** `--add-superseded-by` and `--remove-superseded-by` decide through `reflist` with the dep/related rules (ratified); a removal matching nothing stays a silent no-op.
**Approach (assumed at the header base):** a Note entry-names adapter from the index's Note aliases (`Notes(true)`); delete `isResolveMiss`, `removedSuccessors`, `resolveRemovals`' matching. Visible changes: ambiguous prefix with no text match refused; removal matches alias spellings; add next to an alias-spelled entry adds nothing. Update P3.1 pins, `docs/site/reference/file-format.md`, and regenerate the help reference if flag help changes (`internal/clidoc` test). CHANGELOG (Unreleased, Changed). Depends on P3.4.
**Regression guard.** The "ambiguous prefix matching no entry" case of `TestKnowUpdate_RemoveSupersededBy_MatchesTheTextThenTheResolvedID` flips to the refusal (exit 1, candidates named, file unchanged) and the test is renamed, since exact id/alias now beats text. The entry-names adapter is built from `index.Notes(true)`, every usable Note, retired ones included. reflist's removal decision runs inside `applyNoteUpdate`, against the list after the adds. Supersedes the 0.6.0 CHANGELOG Fixed entry and docs/site/reference/file-format.md's "changes nothing" for an ambiguous prefix; the Changed entry says so.
**Files:** internal/cli/know_update.go; internal/cli/know_update_test.go; docs/site/reference/file-format.md
**Read first:** internal/cli/know_update.go — resolveRemovals, removedSuccessors, applyNoteUpdate, noteResolver; internal/index/notes.go — ResolveNote, Notes; internal/cli/know_update_test.go — TestKnowUpdate_RemoveSupersededBy_MatchesTheTextThenTheResolvedID; internal/knowledge/note.go — validateSupersededBy
**Tests:** runCLI test per visible change; the flipped and renamed ambiguous-prefix case; a retired successor's alias removed; `--add-superseded-by X --remove-superseded-by X` removes X.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/cli/ -run TestKnowUpdate`; `GOMEMLIMIT=2GiB go test ./internal/clidoc/`
**Closes:** reflist-module
**Commit:** `feat(cli): know update superseded-by edits follow the dep rules`

## P4.1 One frontmatter fence predicate

**What:**
**Goal:** `rawfm` exports one fence predicate used by `bodyStartIndex`, `frontmatterEndIndex` and `frontmatter`'s `splitAtFence`; a closing `---\r` at end of file closes the frontmatter in both.
**Approach (assumed at the header base):** `rawfm.IsFrontmatterFence(line)`: `TrimSuffix(line, "\r") == "---"`; `frontmatter` already imports `rawfm`.
**Regression guard.** Every prose site that states the closing-fence rule (`grep -rn 'exactly `---`\|exactly a fence' internal/frontmatter internal/rawfm docs/site`) is reworded — `Split`'s and `splitAtFence`'s doc comments and docs/site/reference/file-format.md.
**Files:** internal/rawfm/conflict.go; internal/rawfm/aliases.go; internal/rawfm/conflict_test.go; internal/frontmatter/frontmatter.go; internal/frontmatter/decode_test.go; docs/site/reference/file-format.md
**Read first:** internal/rawfm/conflict.go — bodyStartIndex, BodyStart; internal/rawfm/aliases.go — frontmatterEndIndex, frontmatterFence; internal/frontmatter/frontmatter.go — Split, splitAtFence, fence; internal/frontmatter/decode_test.go — TestDecode_NoKeys_CallsNothing
**Tests:** table test: CRLF, BOM, trailing-space fence (not a fence), missing close, lone `---`, closing `---\r` at EOF.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/rawfm/`; `GOMEMLIMIT=2GiB go test ./internal/frontmatter/`
**Closes:** fence-scanner
**Commit:** `refactor(rawfm): one frontmatter fence predicate`

## P4.2 store owns worktree-relative paths, part 1

**What:**
**Goal:** `store.RelativePath(root, path)` and `(*Store).Relative(path)` return a slash-separated worktree-relative path, the original path (slash-separated) when outside; a file named `..foo` is shown relative; check and the index use them.
**Approach (assumed at the header base):** `filepath.IsLocal`-based containment; replace `checker.relative` and `index.worktreeRelative`. Not `Store.relative` (joins `.capcom`, a different job).
**Regression guard.** RelativePath also reports whether the path is inside the worktree (e.g. returns the path and an inside bool) so P4.3's IssuesPathspec uses that verdict. `checker.relative`'s callers in check/notes.go, issues.go and problem.go move to the store's helpers; Acceptance greps that neither `checker.relative` nor `index.worktreeRelative` remains.
**Files:** internal/store/store.go; internal/store/store_test.go; internal/check/check.go; internal/check/notes.go; internal/check/issues.go; internal/check/problem.go; internal/index/walk.go
**Read first:** internal/store/store.go — IssuesPathspec, (*Store).relative, locate; internal/check/check.go — (*checker).relative; internal/index/walk.go — worktreeRelative, nodeFileID; internal/store/store_test.go — TestIssuesPathspec_OutsideTheWorktree_IsNil, newRepo
**Tests:** store unit test: inside, outside, a name starting `..foo`, already relative, each with its inside verdict.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/store/`; `GOMEMLIMIT=2GiB go test ./internal/check/`; `GOMEMLIMIT=2GiB go test ./internal/index/`; `grep -rn 'func worktreeRelative\|func (c \*checker) relative(' internal/` prints nothing
**Commit:** `refactor(store): own worktree-relative paths`

## P4.3 store owns worktree-relative paths, part 2

**What:**
**Goal:** nodestore and cli use `store.RelativePath`; no other worktree-relative helper remains.
**Approach (assumed at the header base):** replace `Kind.relative`, `relativeSlashPath` (callers in `errors.go`, `list.go`) and the `..` prefix test in `IssuesPathspec`. Depends on P4.2.
**Regression guard.** `IssuesPathspec` uses P4.2's inside verdict, so it still returns nil outside the worktree (`TestIssuesPathspec_OutsideTheWorktree_IsNil`). `Kind.relative`'s callers across nodestore (errors.go, write.go, claim.go, create.go, format.go, refs.go, load.go, remove.go, move.go, kind.go) change with it. The cli package runs whole, so `frontmatterFailure` (update.go) and `warnRaisedFormat` (related.go) are exercised; the `..foo` clause moved to P4.2's Goal.
**Files:** internal/nodestore/kind.go; internal/nodestore/errors.go; internal/nodestore/write.go; internal/nodestore/claim.go; internal/nodestore/create.go; internal/nodestore/format.go; internal/nodestore/refs.go; internal/nodestore/load.go; internal/nodestore/remove.go; internal/nodestore/move.go; internal/cli/issue_render.go; internal/cli/errors.go; internal/cli/list.go; internal/store/store.go
**Read first:** internal/store/store.go — IssuesPathspec; internal/nodestore/kind.go — (*Kind[T]).relative, relativeText; internal/cli/errors.go — (*app).worktreeRelative, worktreeRelativeText; internal/cli/issue_render.go — relativeSlashPath; internal/briefing/briefing.go — scanElsewhere; internal/cli/output.go — addElsewhere
**Tests:** nodestore's `TestErrors_NameFilesRelativeToTheWorktree` and `*ReturnedRelativeToTheWorktree` stay green.
**Acceptance:** `go build ./...`; `GOMEMLIMIT=2GiB go test ./internal/nodestore/`; `GOMEMLIMIT=2GiB go test ./internal/store/`; `GOMEMLIMIT=2GiB go test ./internal/cli/`; `grep -rn 'func relativeSlashPath\|func worktreeRelative\|func (k \*Kind\[T\]) relative(' internal/` prints nothing
**Closes:** worktree-relpath
**Commit:** `refactor(cli): use the store's worktree-relative paths`

## P4.4 know tree prints the children of an unparseable Note

**What:** Recast at the regression check (2026-10-14).
**Goal:** in `capcom know tree`, a Note whose parent is not indexed (unparseable or left out) prints at the root, as `capcom list --tree` does for issues.
**Approach (assumed at the header base):** fixes `know-tree-orphans`: `childrenByParent` keys such children under the missing id, so the walk from `""` never reaches them; move them to the root as `issuesByParent` does. CHANGELOG (Unreleased, Fixed).
**Regression guard.** Goal becomes "a Note whose parent is not indexed (unparseable or left out) prints at the root"; orphanhood is decided over all indexed Notes before the background filter, as issuesByParent decides over Closed-included entries, so background subtrees stay hidden and TestKnowTree_BackgroundSubtrees_LeftOutUnlessBackground and TestKnowTree_JSON_LeavesOutBackgroundUnlessBackground stay green. The item yields to `know_tree.go`'s documented rule that dropping a background Note drops its subtree. Exception, said in the Fixed entry: a Note below an unparseable Note prints at the root even inside a background or retired subtree (U is not indexed, and retiredSubtrees recurses only through indexed parents), as list --tree does under a Closed issue.
**Files:** internal/cli/know.go; internal/cli/know_tree.go; internal/cli/know_test.go
**Read first:** internal/cli/know_tree.go — (*app).outline; internal/cli/know.go — childrenByParent; internal/cli/list.go — issuesByParent; internal/index/notes.go — Notes, retiredSubtrees;
internal/cli/know_test.go — newBackgroundKnowStore, TestKnowTree_BackgroundSubtrees_LeftOutUnlessBackground; internal/cli/read_test.go — TestListTree_ChildOfInvalidParent_PrintsAsRoot
**Tests:** `TestKnowTree_ChildOfInvalidParent_PrintsAsRoot` (twin of `TestListTree_ChildOfInvalidParent_PrintsAsRoot`), with cases pinning that C prints at the root under background B -> unparseable U -> C without --background and under retired R -> U -> C without --retired; `TestKnowTree_BackgroundSubtrees_LeftOutUnlessBackground` and `TestKnowTree_JSON_LeavesOutBackgroundUnlessBackground` stay green.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestKnowTree|TestKnowShow'`
**Closes:** know-tree-orphans
**Commit:** `fix(cli): know tree prints the children of an unparseable Note`

## P4.5 One visible-subtree outline

**What:**
**Goal:** `list --tree`, `know tree` and `know show` group, hide and start their outlines through one generic outline type in `outline.go` with a per-command visibility rule; output unchanged.
**Approach (assumed at the header base):** `visibleTree[E]` built from entries, id/parent accessors, a sort, `isShown(E)`; gives `shown(parent)`, `all(parent)` (progress counts) and a start helper. Issues hide Closed unless `--all`; `know tree` hides background unless `--background` (moved from `know_tree.go`), `know show --with-children` hides nothing; retired stays filtered in the index. Delete `issuesByParent`, `childrenByParent`. Depends on P4.4.
**Regression guard.** the outline type groups over all entries before visibility filtering, so a hidden parent's children stay hidden. Visibility is per command: `know tree` hides background unless `--background`; `know show --with-children` hides nothing (`all(parent)`), so `TestKnowShow_WithChildren_ShowsBackgroundChildren` stays green; retired stays an index filter. The item yields to `know_tree.go`'s rule that the background filter stays out of the grouping `know show --with-children` shares.
**Files:** internal/cli/outline.go; internal/cli/outline_test.go; internal/cli/list.go; internal/cli/know_tree.go; internal/cli/know.go; internal/cli/know_show.go
**Read first:** internal/cli/outline.go — walkOutline, renderOutline; internal/cli/list.go — issueTree.shownChildren, issuesByParent; internal/cli/know_tree.go — (*app).outline; internal/cli/know_show.go — newKnowShowCommand; internal/cli/outline_test.go — TestWalkOutline_DepthCutsAndCountsShownChildren; internal/cli/know_test.go — TestKnowShow_WithChildren_ShowsBackgroundChildren
**Tests:** unit test of the outline type; `TestListTree_*`, `TestKnowTree_*`, `TestKnowShow_WithChildren_ShowsBackgroundChildren` and `TestWalkOutline_DepthCutsAndCountsShownChildren` stay green.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestListTree|TestKnowTree|TestKnowShow|TestOutline|TestWalkOutline'`
**Closes:** outline-visibility
**Commit:** `refactor(cli): one visible-subtree outline`
