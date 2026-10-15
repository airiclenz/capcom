# Pre-release fixes plan

**Goal:** Close the last open issues before a release cut. The Managed block teaches the work
loop. tree and the index give the same answer for every membership case, and the index holds no
membership logic of its own. know-tree-orphans is closed at the scope ADR 0002 allows.
**Date:** 2026-10-15
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 24b4687
**Closes:** arch-review-1014

**Sources:**
- `.capcom/issues/block-work-loop.md`
- `.capcom/issues/arch-review-1014/membership-owner.md` (its ratified calls and Log)
- `.capcom/issues/arch-review-1014/know-tree-orphans.md`
- `.capcom/knowledge/decisions/adr-0002/adr-0002-membership.md`
- `.capcom/knowledge/glossary/glossary-agent-interface/managed-block.md`

**Ratified design calls** (user, 2026-10-15):
- **Orphans scope:** keep ADR 0002. A Note or issue below a repeated-id or invalid-id parent stays left out with a warning; pin that with tests and close know-tree-orphans.
- **Claim status:** the block renders the configured doing status (`config.DefaultStatus(CategoryDoing)`); `agents.Sync` receives it.
- **Block shape:** the work loop is folded into the existing command bullets in loop order (ready, show, claim, log, close, reopen), then the other bullets, then one `--help` line.
- **Membership scope:** full membership-owner acceptance in this plan, tree side authoritative.

**Standing requirements:**
- skills: coding-standards
- Tests: `GOMEMLIMIT=2GiB`, one package at a time, never `-race` on `./...` (AGENTS.md).
- A deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- Changing ADR 0002's left-out-subtree rule.
- A generated work-loop skill (rejected in block-work-loop).
- Any version, tag or CHANGELOG heading change.

**Regression check (2026-10-15, 24b4687):**
- 1: guard folded; supersedes "rendered from the Note files alone" (`docs/site/reference/file-format.md:685`, `docs/site/agents/managed-block.md:91`), rewritten by item 2
- 2: guard folded
- 3: guard folded
- 4: guard folded
- 5: recast (the below-repeated-id parse state and its fixture move to item 6)
- 6: recast (carries them); yields to membership-owner.md:39 ("Index warning wording stays as it is")
- 7: guard folded
- 5 (re-check): guard folded (duplicate-alias warning names the first clean claimant; one conflict-file comparison over `*tree.Error` and `*tree.UnusableError`)
- 6 (re-check): guard folded (RelatedBy subqueries stay unfiltered; only BelowRepeatedID leaves `isClean`; reuses item 5's comparison)

## 1. Managed block teaches the work loop

**What:**
**Goal:** `render` in `internal/agents` emits the work loop in its command bullets, naming the
configured doing status in the claim line, and this repository's AGENTS.md matches it.
**Approach (assumed at the header base):** `Sync(worktreeRoot, knowledgeDir, mode)` gains the
doing status (a `string` plus ok, or the empty string for none) and passes it through `reconcile`,
`statusOf`, `splice` and `render`, and the test wrappers `inspect`/`write`. Callers pass
`DefaultStatus(config.CategoryDoing)` from the config they hold: `writeAgentsBlock` and the
`--check` path in `internal/cli/agents.go`, `syncAgents` in `internal/cli/know_add.go`, the check
in `internal/check/agents.go`. `blockIntro` stops being one const: the command bullets become, in
this order and wording:
`capcom ready` (as now); `capcom show <id>` (as now); ``capcom update <id> --status <doing>`: claim
it before you start``; ``capcom log <id> <text>`: note progress as work lands``;
`capcom close <id> --reason <text>` (as now); ``capcom reopen <id> --reason <text>`: undo a
close``; then the create, dep, list and know bullets as now; then the line
``Each command's flags are in `capcom <cmd> --help`.`` With no doing status configured the claim
line is left out. The Tree lock paragraph is unchanged and the claim line does not mention it.
Refresh AGENTS.md with `go run ./cmd/capcom agents`.
**Regression guard.** The runCLI test goes in `internal/cli/agents_test.go` beside
`TestAgents_AppendsThenRefreshesInPlace`, writing `.capcom/config.yaml` as `close_test.go:102`
does. Every comment in `internal/agents` that says the block is rendered from the Knowledge top
level alone (`block.go:2-3`, `Sync`'s doc at `block.go:263`) names the doing status too
(`grep -n 'rendered deterministically\|top level of the Knowledge' internal/agents/*.go`). The
item's sidecar CHANGELOG entry follows the existing "An existing block reads as out of date after
upgrading: run capcom agents once" precedent and adds that renaming the doing status in
config.yaml now also stales the block until `capcom agents` runs. This supersedes "rendered from
the Note files alone" (`docs/site/reference/file-format.md:685`,
`docs/site/agents/managed-block.md:91`); item 2 rewrites both.
**Files:** internal/agents/block.go, internal/agents/block_test.go, internal/agents/guides_test.go, internal/cli/agents.go, internal/cli/agents_test.go, internal/cli/know_add.go, internal/check/agents.go, internal/check/agents_test.go, AGENTS.md
**Read first:** internal/agents/block.go — blockIntro, render, Sync, reconcile; internal/cli/agents.go — writeAgentsBlock, checkAgentsBlock;
internal/cli/know_add.go — syncAgents; internal/check/agents.go — agentsBlock
**Tests:** update `goldenBlock`; add `TestRender_WorkLoop_NamesTheDoingStatus` (a config whose
doing status is `wip` renders `--status wip`, never `active`) and
`TestRender_NoDoingStatus_LeavesOutTheClaimLine`; a runCLI test in `internal/cli/agents_test.go`
that `capcom agents` in a store with a renamed doing status writes that name into AGENTS.md.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/agents/`
- `GOMEMLIMIT=2GiB go test ./internal/check/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Agents|KnowAdd|Check'`
- `go run ./cmd/capcom agents --check` exits 0; `grep -c 'capcom reopen' AGENTS.md` prints 1
**Commit:** `feat(agents): Managed block teaches the work loop with the configured doing status`
**Closes:** block-work-loop

## 2. Manual copy of the Managed block matches the new text

Depends on item 1.
**What:**
**Goal:** `docs/site/agents/managed-block.md` quotes the block exactly as item 1 renders it for the
default config, and its prose summary names the work loop and the `--help` pointer.
**Approach (assumed at the header base):** replace the ```markdown sample with the block from
AGENTS.md (this repository's Knowledge list may be swapped for the page's sample list, as the page
does now); add one sentence that the claim line names the repository's own doing status.
**Regression guard.** Edit `docs/site/reference/file-format.md` too: every sentence describing
the block's contents or its inputs, there and in managed-block.md, names the work-loop commands,
the `--help` line and the configured doing status, and the stale-block sentences name a doing-status
rename in config.yaml beside a hand Note edit (`grep -n 'Note files alone\|handful of commands\|refreshes nothing' docs/site/reference/file-format.md docs/site/agents/managed-block.md`).
**Files:** docs/site/agents/managed-block.md, docs/site/reference/file-format.md
**Read first:** docs/site/agents/managed-block.md — "Writing it: capcom agents" sample, "The Knowledge tree in the block", "Keeping it current";
docs/site/reference/file-format.md — "The Managed block in AGENTS.md"; AGENTS.md — the capcom block; internal/config/config.go — Default (doing status `active`)
**Tests:** none (docs).
**Acceptance:**
- `grep -c 'capcom reopen <id> --reason <text>' docs/site/agents/managed-block.md` prints 1
- `grep -c 'capcom <cmd> --help' docs/site/agents/managed-block.md` prints at least 1
- `sed -n '1,13p' docs/site/agents/managed-block.md | grep -c 'work loop'` prints at least 1
**Commit:** `docs(site): Managed block page shows the work loop`

## 3. Prefix of an unusable node's id resolves as the index does

**What:**
**Goal:** a prefix that matches only the id of a node that is not usable (left out as a duplicate
or unparsed) gives the same answer from `tree.ResolveIn` over a loaded Tree and from the index; an
agreement fixture covers it.
Regression from b1ff6b8: `prefixMatch` in `internal/tree/resolve.go` treats the id as a match
because `byID` holds unusable nodes, so for broken `zzz.md` plus `ghost/zzz.md` the query `zz` gives
`id zzz` from tree and a duplicate refusal from the index. `ResolveIn`'s doc ("unless a usable node
has the id") is the ratified rule; the index side is right.
**Approach (assumed at the header base):** in tree's `NameReader` implementation (`*Tree.Match` /
`PrefixCandidates` in `internal/tree/tree.go`), or in `prefixMatch`, count an id as a node match
only when its node is usable; the refusal comes from `refusals` as on the index side.
**Regression guard.** `*Tree.Match` (`internal/tree/resolve.go:185`) is the only site: it sets
`match.ID` for an id only when the kept node is usable, and `NameMatch.ID`'s doc says "usable node".
`prefixMatch` and its unit test stay as they are (`NameMatch` carries no usability bit). `newTree`
records per-node usability from `facts[i].IsUsable` (it folds in rejected aliases, `claims.go:74`),
not from `unparsedIDs`; the new agreement fixture covers a `formBroken` and a `formInvalid` node.
**Files:** internal/tree/resolve.go, internal/tree/tree.go, internal/tree/resolve_test.go, internal/index/agreement_test.go
**Read first:** internal/tree/resolve.go — (*Tree).Match, prefixMatch, ResolveIn; internal/tree/tree.go — newTree, Tree.unparsedIDs;
internal/index/query.go — snapshotNames.Match; internal/tree/resolve_test.go — unusableClaimsFixture; internal/index/agreement_test.go — agreementFixtures
**Tests:** a `resolve_test.go` case for the prefix over a loaded Tree (the `prefixMatch` table
unchanged); a new `agreementFixtures` entry for both kinds, with a `formBroken` unusable node
(`zzz.md` plus `ghost/zzz.md`, query `zz`) and a `formInvalid` one (`bad.md` plus `ghost/bad.md`,
query `ba`).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `GOMEMLIMIT=2GiB go test ./internal/index/ -run Agreement`
**Commit:** `fix(tree): a prefix of an unusable node's id resolves as the index resolves it`

## 4. One claim-fact builder for files the walk refuses

Depends on item 3.
**What:**
**Goal:** the claims of orphan-directory files and of files the walk refuses (not regular, too
large, unreadable) are built by one exported `tree` function that `tree.Load` and the index refresh
both call; an unreadable orphan file keeps its id claim on both sides; `internal/index` defines no
`orphanDirectoryClaims`, `problemClaims` or `nodeFileID`.
Regression from b1ff6b8: `orphanDirectoryClaims` in `internal/index/walk.go` drops a file whose
alias read fails (`continue`), id included, while tree's `claimOnlyFacts` keeps the id, so a name
shared with another unusable file is "not found" in the index and a duplicate in tree.
**Approach (assumed at the header base):** export the fact building of `claimOnlyFacts` /
`claimOnlyFact` in `internal/tree/claims.go` (one deep function over `ListOrphanFiles` and the
walk's refused files, returning the per-file facts `DecideMembership` takes); the index's
`walkIssues`/`replaceWalkClaims` path writes its `claims` rows from those facts.
**Regression guard.** The builder takes every orphan-directory problem it is given, and the index
passes its repeated-id directory problems (`leaveOutBelowRepeatedIDs`, `walk.go:43-47`) too: they
are the only aliases its left-out nodes get, so "repeated id with subtree" `kid-alias` stays a
duplicate. Fact Keys go through `store.RelativePath` to the slash paths `membershipFacts` matches
on; `withoutKeptFiles` stays. The unreadable-file case is its own test function, not an
`agreementFixtures` entry (that table cannot skip and checks `Claimant`, which fails on an
unreadable orphan file): shaped like `TestAgreement_UnreadableDirectoryInOrphan_ReadableFilesClaimAndClaimantFails`,
it chmods the file and asserts only `ResolveIn` and the index's resolve.
internal/index/refresh.go (walkStore, insertClaimQuery, replaceWalkClaims) joins the Files line
when the change reaches it.
**Files:** internal/tree/claims.go, internal/tree/claims_test.go, internal/index/walk.go, internal/index/duplicates.go, internal/index/agreement_test.go; internal/index/refresh.go when the change reaches it
**Read first:** internal/index/walk.go — walkIssues, leaveOutBelowRepeatedIDs, orphanDirectoryClaims, withoutKeptFiles;
internal/tree/claims.go — claimOnlyFacts, claimOnlyFact; internal/tree/tree.go — load; internal/index/duplicates.go — membershipFacts
**Tests:** an agreement test function with an unreadable orphan file and another unusable file
sharing its id (skipped on Windows and root like `TestAgreement_UnreadableDirectoryInOrphan_...`,
asserting only `ResolveIn` and the index's resolve); a fixture where two unusable files share a
name with no node row.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `grep -nE 'func (orphanDirectoryClaims|problemClaims|nodeFileID)' internal/index/*.go` prints nothing
**Commit:** `fix(index): claims of refused files come from tree's one fact builder`

## 5. Index facts carry the parse state tree reads

Depends on item 4.
**What:**
Recast at the regression check (2026-10-15).
**Goal:** every fact the index hands `tree.DecideMembership` carries the same parse state tree
reads for that file, so a recovered alias of an unparsed file below a repeated id only claims (an
unusable claim, as in tree), and a file that parses but fails validation has its aliases mapped on
both sides; agreement fixtures cover both.
Regression from b1ff6b8: `membershipFacts` in `internal/index/duplicates.go` builds left-out nodes
with `walkFact(..., tree.ParsedFile)` and every problem file as `UnparsedFile`, while tree's
`readContentFacts` takes the parse state from `ReadContent`.
**Approach (assumed at the header base):** persist the parse state with each claim-bearing file
(a column on `claims` or `file_verdicts` in `internal/index/schema.go`, with `schemaVersion`
bumped) and read it back in `membershipFacts`; `claimedAliases` stops re-running
`tree.CheckAliases` when the persisted facts already decide it.
**Regression guard.** item 5 builds the persisted parse state only for problem files and
claim-only files (refresh.go joins its Files line); the parse state of files below a repeated id
and the agreement fixture "unparsed file with a recovered alias below a repeated id" move to item
6, which builds them once on its one-decision path, so no item writes code a later item deletes.
The parse state is a column on `claims` (or `files`), never `file_verdicts`, which
`persistMembership` clears before `membershipFacts` reads it; it comes from `Reading.IsRead`/`IsParsed`
in `indexFile` and from item 4's tree facts for walk-left-out files. A problem file's claims rows
hold its id plus raw aliases (`refresh.go:325-336`): keep `CheckAliases` on read-back, or write
only `CheckAliases`-passed aliases (id excluded) at both claims writers, so invalid `foo.md` still
answers "not found" (`TestClose_IDOfInvalidFile_...`). The parsed-invalid fixture pins the invalid
file to sort first (`aaa.md` invalid `[shared]`, `zzz.md` valid `[shared]`). Item 5 builds the
agreement test's conflict-file comparison once (covering *tree.Error Path/Other and
*tree.UnusableError.Path, index paths joined onto the worktree root).
Duplicate-alias warnings stay a function of the clean files: when a conflict's Key is clean and
Other is not, `persistMembershipOf` warns naming the first clean claimant before Key in
`membership.Names[Name].Claims` (if any).
**Files:** internal/index/schema.go, internal/index/duplicates.go, internal/index/refresh.go, internal/index/walk.go, internal/index/agreement_test.go, internal/index/refresh_test.go
**Read first:** internal/index/duplicates.go — membershipFacts, claimedAliases, persistMembershipOf; internal/index/refresh.go — indexFile, insertClaims;
internal/index/schema.go — schemaVersion; internal/tree/membership.go — mapKeptAliases; internal/index/agreement_test.go — agreementAnswer
**Tests:** agreement fixture: a parsed, invalid file (`aaa.md`) whose alias another node
(`zzz.md`) also lists. Compare the conflict's file, not only the answer category, through the one
comparison helper. A refresh test with `aaa.md` invalid `aliases: [s]`, `mmm.md` and `zzz.md` valid
`[s]` still warns `zzz.md: duplicate alias "s": also an alias of mmm.md`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestClose_|TestShow_'`
**Commit:** `fix(index): membership facts keep the parse state tree reads`

## 6. The index runs DecideMembership once

Depends on item 5.
**What:**
Recast at the regression check (2026-10-15).
**Goal:** one refresh calls `tree.DecideMembership` once per kind and uses that verdict both to
split the walk and to persist verdicts; `internal/index` holds no repeated-id logic of its own and
membership-owner's Acceptance holds.
**Approach (assumed at the header base):** `leaveOutBelowRepeatedIDs` in `internal/index/walk.go`
runs a second `DecideMembership` over `UnreadFile` facts to drop the files below a repeated id
before indexing; replace it with the verdicts of the one call `persistMembershipOf` uses
(`LeftOut{Reason: BelowRepeatedID}` marks them). Remove `withoutKeptFiles` if nothing else needs it.
**Regression guard.** Item 6 carries the below-repeated-id parse state and that fixture ("unparsed
file with a recovered alias below a repeated id"), per the item-5 decision. Files below a repeated
id are now indexed: only `membership.LeftOut` entries with Reason `BelowRepeatedID` leave
`isClean` (`RepeatedIDFile` files keep their clean-file warnings), and their own file findings
leave `r.warnings` once the verdict is known, so no new "duplicate alias" warning appears; the item
yields to membership-owner.md:39 ("Index warning wording stays as it is") and `duplicates.go:87-88`.
`refresher` holds the worktree root and joins it to the slash Keys before `RepeatedIDDirProblems`,
reports the problem path relative, and wraps its failure with `relativePathError` as today.
`RelatedBy` (`index.go:225-227`): leave both NOT EXISTS subqueries unfiltered (a below-repeated
file now counting there matches tree's `checkLeftOutAliases` refusal), say so in a NOTES line, and
pin it with a RelatedBy test on an alias a below-repeated file shares.
Item 6 reuses item 5's conflict-file comparison and does not build its own.
**Files:** internal/index/walk.go, internal/index/duplicates.go, internal/index/refresh.go, internal/index/refresh_test.go, internal/index/agreement_test.go, internal/index/index_test.go
**Read first:** internal/index/walk.go — walkIssues, leaveOutBelowRepeatedIDs, withoutKeptFiles; internal/index/duplicates.go — persistMembershipOf, membershipFacts;
internal/index/refresh.go — refresh; internal/tree/claims.go — RepeatedIDDirProblems; internal/index/index.go — RelatedBy
**Tests:** existing index and agreement tests; the agreement fixture "unparsed file with a
recovered alias below a repeated id" (from item 5), comparing the conflict's file through item 5's
comparison; a RelatedBy test in which a below-repeated file shares the target's alias, so the
holder linking by it is not listed; a refresh test that a Note and an issue below a repeated id are
left out with the directory warning and add no warning of their own, the below-repeated file
written with a field problem so the filtering is exercised. It is a no-new-warning pin without a
bite (walk.go:43-47 never reads those files today); the bite is the agreement fixture.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestShow_|TestRm_|TestMv_|TestCheck_'`
- `grep -n 'func leaveOutBelowRepeatedIDs' internal/index/*.go` prints nothing
- `go vet ./...` is clean
- `grep -n 'tree.DecideMembership(' internal/index/*.go` prints one line
- `grep -nE 'excludeDuplicates|ListOrphanFiles|rawfm' internal/index/*.go` prints nothing
**Commit:** `refactor(index): one membership decision per refresh`
**Closes:** membership-owner

## 7. Left-out subtrees are pinned in know tree and list --tree

**What:**
**Goal:** runCLI tests pin that `capcom know tree` and `capcom list --tree` leave out a node below a
repeated-id or an invalid-id parent and print the directory-left-out warning; a node below an
unparseable parent still prints at the root; ADR 0002's membership Note states the invalid-id
subtree rule beside the repeated-id one.
**Approach (assumed at the header base):** extend `TestKnowTree_ChildOfInvalidParent_PrintsAsRoot`
(`internal/cli/know_test.go`) and the list test in `internal/cli/read_test.go`; rename the list test
to `TestListTree_ChildOfUnparseableParent_PrintsAsRoot` (and the know one alike) so the names match
the case. Add one sentence to `adr-0002-membership.md`: a node file with an invalid id has its
directory left out with every file below it, with a warning. Run `capcom check` after.
**Regression guard.** Each left-out case also asserts `strings.Contains(stdout, "<slash dir>:
directory left out: its node file <id>.md has a duplicate id\n")` (or "has an invalid id"), worded
as `check_test.go:54` and `mv_test.go:229` do: the `HasPrefix(stdout, want+"warning: ")` check sees
only the first, path-sorted warning. The renamed `..._ChildOfUnparseableParent_PrintsAsRoot` tests
keep the unparseable case only; the repeated-id and invalid-id cases go in new
`TestKnowTree_ChildOfLeftOutParent_LeftOutWithWarning` and
`TestListTree_ChildOfLeftOutParent_LeftOutWithWarning`. The know-tree-orphans close reason cites
the ratified Orphans scope call (user, 2026-10-15): a Note below a repeated-id or invalid-id parent
is left out with a warning by ADR 0002, and a Note below an unparseable parent prints at the root
since b1ff6b8.
**Files:** internal/cli/know_test.go, internal/cli/read_test.go, .capcom/knowledge/decisions/adr-0002/adr-0002-membership.md
**Read first:** internal/cli/know_test.go — TestKnowTree_ChildOfInvalidParent_PrintsAsRoot, writeNoteFiles; internal/cli/read_test.go — TestListTree_ChildOfInvalidParent_PrintsAsRoot, writeIssueFiles;
internal/tree/errors.go — invalidIDDirectoryError; internal/tree/claims.go — DuplicateIDDirectoryError; .capcom/knowledge/decisions/adr-0002/adr-0002-membership.md — Repeated id bullet;
internal/cli/check_test.go — directory-left-out wording
**Tests:** as above, plus the two `..._ChildOfLeftOutParent_LeftOutWithWarning` tests.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestKnowTree_|TestListTree_'`
- `go run ./cmd/capcom check` reports no problems
**Commit:** `test(cli): pin left-out subtrees in know tree and list --tree`
**Closes:** know-tree-orphans
