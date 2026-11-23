# Plan: move the domain logic out of internal/cli

**Goal:** The domain logic still inside `internal/cli` moves into domain packages, following
`internal/ops` and `internal/briefing`, one green commit per move and with no user-visible change.
Each moved piece gets direct unit tests in its new package; `internal/cli` keeps the command wiring.
**Date:** 2026-11-23
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** f09536a
**Regression check (2026-11-23, f09536a):**
- 1: guard folded (ratified design calls; `idWordSeparator` moves too, `suggestionDistance` stays referenced from root.go; staticcheck and the -w grep in Acceptance)
- 2: guard folded (briefing's package comment updated, ready.go in Files; staticcheck and the -w grep)
- 3: guard folded (staticcheck and the -w grep)
- 4: recast (the caller chain read lazily; three runCLI journey cases kept); yields to root.go:128-129 and root.go:504 ("The caller's ancestry is read only for a record that names a holder process") and the archived plan `2026-11-22 - 00 - lock-holder-and-agent-guide-plan` ("Read the chain lazily"); guard folded (Acceptance greps root.go for `HeldBy|TouchHolder|.Check(`; a failed renewal carries TouchHolder's error unchanged, with a direct test)
- 5: guard folded (the two failure tests kept as runCLI journey cases; a failed `issue.MergeFile` returns no path and cli words it with `args[mergeArgPath]`; staticcheck and the -w grep)
- 6: recast (the indexed Dependency graph moves into `internal/index`, not a new `internal/depgraph`; reviewer 2's guard folded: case 1 kept in cli, `dep_test.go` in Files)
- 7: guard folded (cli comments naming the moved names restated; staticcheck and the -w grep)
- 8: guard folded (staticcheck and the -w grep)

**Sources:**
- Issue `split-cli-package` (the analysis brief) and its move Issues `cli-split-*` under `code-maturity`
- Notes `adr-0012`, `cli-test-harness`, `coding-conventions`, `docs-layout`
- `internal/cli/doc.go` — the file map `TestDocFileMap_MatchesDirectory` checks

**Ratified design calls** (Airic, 2026-11-23):
- **Scope:** all 8 moves go ahead (only 431 of 10,822 lines are domain logic; the issue's 26k figure was stale).
- **Behaviour:** pure moves; every message, hint, exit code, `--json`/`--kv` field and help text stays byte-identical; no CHANGELOG entry.
- **Wording stays:** failure wording (`output.Error`, `output.Match*`), rendering and their constants stay in `internal/cli`; a moved function returns data or a plain error that cli words.
- **Adapters stay:** a function under ~20 lines that only adapts store config or an index entry to an existing domain API is wiring (`reachingGuides`, `syncAgentsBlock`, `checkNewRefs`, `readiness`, `progressOf`, `finishLock`).
- **New packages:** `internal/nearest` and `internal/mergedriver` only; the indexed Dependency graph extends `internal/index` (no `internal/depgraph`); every other move extends `index`, `briefing`, `recap`, `lock`, `ops` or `knowledge`.
- **Tests:** a moved function's direct tests move with it; a runCLI test replaced by a direct test is deleted, except a runCLI test that is the only check of wiring staying in cli, which stays as a journey case.

**Standing requirements:**
- skills: coding-standards
- Single packages with `GOMEMLIMIT=2GiB`; never `./...` with `-race`; never two test commands at once (AGENTS.md).
- CLI tests go through `runCLI`; never `t.Parallel()` (Note `cli-test-harness`); domain tests use real temp files and git (ADR 0007).
- A new package opens with a `// Package` comment; a file added to or removed from `internal/cli` updates the `doc.go` file map.
- Any deviation from item text lands as a dated NOTES line under the item.
- (a) Every item's Acceptance runs staticcheck at the version `.github/workflows/ci.yml` pins (`go run honnef.co/go/tools/cmd/staticcheck@<STATICCHECK_VERSION> <the item's packages>`), since CI's U1000 fails an orphaned const or helper left in cli.
- (b) Every comment in `internal/cli` naming a moved symbol is restated, and each item's grep uses `grep -nwE '<moved names>' internal/cli/*.go` (word match, comments included) rather than only `^func`.

**Out of scope:**
- `internal/ui` and `internal/cli/output` (the output contract, 568 non-test lines): both stay as they are.
- Any behaviour change, renamed flag or reworded message; `internal/check`'s own guide assembly.
- runCLI tests no item names.

## Classification

Measured at the base with `wc -l` on every non-test file: 51 files, 10,822 lines, 367 functions.
**Wiring** builds a cobra command, reads or checks flags, words a failure, renders text, JSON or KV,
opens the store or index, or adapts an existing domain API. **Domain** decides something about
Issues, Notes, Locks, refs or names from data alone, with no cobra and no output wording. 25
functions (431 lines) are domain logic; the rest is wiring (5,798 lines of functions; the remainder
is types, help text and comments). Each function is listed with its line count; `→n` names the move
item that takes it.

| File | Lines | Wiring (stays) | Domain (moves) |
|---|---|---|---|
| `agents.go` | 304 | newAgentsGuideData 7, newAgentsCommand 20, writeAgentsBlock 28, missingAGENTSMdWarnings 9, syncAgentsBlock 5, reachingGuides 14, guidesText 12, checkAgentsBlock 34, guidesFailure 36, agentsFailure 7 | — |
| `apply.go` | 333 | newApplyCommand 20, apply 35, readApplyFile 22, applyFileName 6, operationFailure 17, operationCause 19, commitBatch 19, batchWarnings 20, applyResult 38 | — |
| `check.go` | 81 | newCheckCommand 33, checkProblems 14 | — |
| `close.go` | 232 | newCloseCommand 36, closeResult 49, issueNodes 3, closeOne 40, alreadyClosed 9, requireReason 9 | — |
| `complete.go` | 306 | isCompletionRequest 3, completeFlag 6, completeIssueIDs 3, completeNoteIDs 3, completeIDValue 3, completeIDs 10, idCandidates 6, completeStatuses 13, isOpenCategory 3, isAnyCategory 3, completeTypes 11, completeCategories 9, commaSeparated 14, completeUITarget 25, withCompletionIndex 7, withCompletionStore 10, readCompletionStore 13, readCompletionIndex 15, issueCandidates 20, noteCandidates 14, appendCandidates 14 | — |
| `context.go` | 370 | newContextCommand 25, contextResult 40, contextDepsText 6, splitRows 6, contextSection 14, contextPlan 15, contextNotes 34, noteBodyText 12 | applyingNotes 40 →2, mentionsPath 18 →2, endsPath 10 →2, isPathByte 10 →2 |
| `create.go` | 204 | newCreateCommand 42, create 15, newIssue 24, createFailure 29, missingPlanWarnings 6 | — |
| `dep.go` | 129 | newDepCommand 13, newDepAddCommand 19, newDepRmCommand 23, depWritten 7 | — |
| `dep_graph.go` | 181 | newDepGraphCommand 29, depGraph.flowchartResult 34 | loadDepGraph 11 →6, depGraph.names 6 →6, depGraph.shownBy 16 →6 |
| `dep_list.go` | 186 | newDepListCommand 29, depGraph.listResult 33, newDepSection 3, depSection.add 20 | depBranches 11 →6 |
| `doc.go` | 63 | — | — |
| `errors.go` | 161 | kindFailure 42, showCommand 6, refusalFailure 19, worktreeRelativeText 3, treeProblemWarnings 7, unwarnedTreeProblems 9, isIndexWarnedAbout 12 | — |
| `export.go` | 81 | newExportCommand 17, exportHTML 18 | — |
| `finish.go` | 277 | newFinishCommand 24, finish 20, finishLock 22, keepFinishLock 15, notHoldingReason 9, unreadableKeptToken 11, unreadableFinishLock 11, nextAfterFinish 14, finishResult 47 | — |
| `flag_once.go` | 69 | onceValue.Set 4, guardSingleValueFlags 13, isAccumulating 6, refuseRepeatedFlags 11 | — |
| `flags.go` | 104 | invalidErrorf 3, valueFailure 13, checkCategory 12 | — |
| `fmt.go` | 114 | newFmtCommand 33, formatRun.emit 21, errorStrings 7 | — |
| `init.go` | 266 | newInitCommand 69, installHooks 11, installApogeeHooks 11, hooksFailure 12, installMergeDriver 11, mergeDriverConfigText 4, agentsHint 3, initText 12, claudeHooksSnippet 4 | — |
| `issue_render.go` | 322 | newIssueData 20, issueListData 7, issueLines 11, missingIssueLine 10, childProgress 7, progressOf 12, newShowData 15, issueText 42, standingText 6, childrenText 6, readyText 10, issueKV 15, flowList 6, formatTimestamp 6, flatten 3, nonNil 6 | — |
| `know_add.go` | 224 | newKnowAddCommand 31, addNote 24, checkNewRefs 15, refWarnings 8, noteWriteResult 7, notes 3, syncAgents 7, addSyncEffects 17, newNote 16, noteBody 7 | — |
| `know.go` | 89 | newKnowCommand 17, resolveNote 8, newNoteData 13 | — |
| `know_search.go` | 94 | newKnowSearchCommand 57 | — |
| `know_show.go` | 227 | newKnowShowCommand 38, shownNotes 31, noteShowResult 32, noteFromFile 13, noteText 31, retiredText 6 | — |
| `know_tree.go` | 131 | newKnowTreeCommand 37, noteOutlineLine 3, outline 27 | — |
| `know_update.go` | 377 | newKnowUpdateCommand 71, readNoteInputs 18, successorList 12, resolveNoteID 4, updateNote 21, noteResolver.Resolve 7, applyNoteUpdate 56, knowUpdateOptions.bodyEdits 3 | noteNames 23 →7, editSuccessors 30 →7 |
| `know_verify.go` | 116 | newKnowVerifyCommand 29, verifyNotes 20, verifyFailure 18 | — |
| `list.go` | 427 | newListCommand 51, listFlaggedResult 6, checkTreeFlags 16, listResult 22, listNamedResult 21, checkNamedFlags 12, lookupNamedIssue 20, listFilter 15, rowFilter 24, listOptions.hasRowFilter 4, checkListValues 27, issueTreeResult 23, issueOutline 18, issueTree.add 18 | — |
| `lock.go` | 657 | newLockCommand 15, newLockAcquireCommand 36, newLockTouchCommand 22, newLockReleaseCommand 22, newLockBreakCommand 18, newLockCheckCommand 31, newLockStatusCommand 23, presentedToken 14, heldLockHint 13, keptResumeCommand 9, takeOverCommand 3, corruptLockFailure 8, openLockTarget 3, emitLock 42, emitDiscardedLock 10, failedLockResult 10, ownLockResult 15, allLocksResult 27, lockStateName 14, endedState 6, newLockData 16, lockPairs 10, lockText 20 | — |
| `log.go` | 48 | newLogCommand 23 | — |
| `merge_driver.go` | 169 | newMergeDriverCommand 15, failedMerge 3, mergeLabelsFrom 8, mergeLabel 6 | mergeIssueFile 32 →5, gitTextMerge 22 →5 |
| `mv.go` | 144 | moveDestination 13, movedWarnings 23, movedPath 6, moveResult 16 | — |
| `next.go` | 88 | newNextCommand 36, emitNothingReady 13 | — |
| `node_edit.go` | 244 | ops 3, editIssue 8, editNode 11, written 10, conflictMarkerWarnings 18, noteKindWrite 7, warnDroppedComments 5, writeFailure 19, bodyEdits.checkPairs 12, bodyEdits.apply 6, bodyEdits.replacements 7, replacementFailure 10, readTextInput 22, writeResult 8 | — |
| `node_verbs.go` | 362 | newMvCommand 42, move 28, newRmCommand 42, remove 23, unreadWarnings 7, movedUnreadWarnings 15, unreadWarning 3, joinKeys 6, createNode 20 | — |
| `outline.go` | 37 | renderOutline 17 | — |
| `output.go` | 27 | addElsewhere 14 | — |
| `prime.go` | 300 | newPrimeCommand 12, primeResult 9, renderBriefing 40, primeWarnings 15, addCappedList 9, addLocks 32, worktreeLockLine 8, worktreeLockAdvice 20, addHealth 21, indentLines 6 | — |
| `ready.go` | 248 | newReadyCommand 43, readyOptions.listOptions 18, readyListData 17, readyLines 15, notReadyReasons 20, droppedDepWarnings 11, allHiddenWarnings 13, allHiddenWarning 10 | — |
| `recap.go` | 255 | newRecapCommand 27, unknownSinceError 10, recapResult 21, closedReason 3, newStatus 3, writeRecapGroup 24, indentRecapEntry 7, withEmptyGroups 16 | recapBoundary 17 →3, sinceTime 14 →3 |
| `reindex.go` | 51 | newReindexCommand 19 | — |
| `related.go` | 189 | newRelatedCommand 11, newRelatedAddCommand 41, newRelatedRmCommand 44, relatedListHint 3, linkHolder 22 | — |
| `reopen.go` | 94 | newReopenCommand 39, targetStatus 26 | — |
| `rm.go` | 190 | issueRemovalFailure 11, issueRemoveResult 28, noteRemoveResult 16, cleanupWarnings 7, refsUncheckedWarnings 4 | brokenRefWarnings 29 →8 |
| `root.go` | 620 | Run 22, Command 3, newRootCommand 49, withSubcommandDispatch 17, unknownCommandError 15, unknownCommandOnFlagError 10, namesSubcommand 8, prepare 9, prepareIndex 43, closeIndex 6, withKV 4, selectRenderer 14, runE 8, emit 24, noteWrite 3, callerAncestry 7, holderProcess 7, newVersionCommand 18, scanFormat 14 | closestName 10 →1, editDistance 21 →1, lockWarning 29 →4, renewHolderLock 13 →4, writesOnly 8 →4 |
| `search.go` | 142 | newSearchCommand 46, searchText 6, searchExcerpts 11, withExcerpts 13 | searchEveryWordFirst 8 →2 |
| `show.go` | 368 | newShowCommand 23, bodyView.body 10, emitShown 9, showResult 64, failureKV 7, newArgumentFailures 3, argumentFailures.add 7, argumentFailures.addFailure 8, argumentFailures.failure 7, messageWithSuggestions 6, issueFromFile 22, loadShowGraph 10, newShowGraph 7, showGraph.facts 7, showLinks 13, resolveFailure 10, resolveIssue 8 | — |
| `start.go` | 325 | newStartCommand 17, start 43, refuseUnstartable 40, readiness 16, readinessResolvers 13, refuseLock 42, startLockFailure 15, startHeldHint 10, claimForStart 16, emitStarted 24 | — |
| `suggest.go` | 237 | unknownFlagError 11, closestFlagName 7, nearFlagNames 16, synonymSubcommand 7, notFoundFailure 16, closestIDs 14 | isNearName 4 →1, closestNames 28 →1, sharedWordCount 12 →1, letterCount 9 →1 |
| `ui.go` | 125 | newUICommand 39, uiTarget 17, terminal 11 | — |
| `update.go` | 332 | newUpdateCommand 55, updateIssue 14, issueUpdate 38, updateOptions.bodyEdits 3, statusChangeFailure 48, hasAnyFlag 8, readNodeFile 12, parseNodeFile 10, absoluteIssuePath 3, frontmatterFailure 12, parseFailure 8 | — |
| `upgrade.go` | 102 | newUpgradeCommand 29, upgradeResult 17 | — |

**Move order.** Each item is one self-contained move that builds and passes on its own, so every
prefix of the items leaves the tree green. Items 1 and 4 both edit `root.go`, and items 2 and 6 both
edit `context.go`: they run in plan order, never in one wave. Each item closes its own move Issue.

## 1. Move name suggestion ranking to internal/nearest — ✅ DONE (2026-11-23)

**What:**
**Goal:** a new leaf package `internal/nearest` holds the edit distance and the ranking of the names
closest to a mistyped one, with direct unit tests; no file of `internal/cli` defines `editDistance`,
`closestName`, `closestNames`, `sharedWordCount`, `letterCount` or `isNearName`, and every suggestion
capcom prints stays byte-identical.
**Approach (assumed at the header base):** move `editDistance` and `closestName` from `root.go`, and
`closestNames`, `sharedWordCount`, `letterCount`, `isNearName` with `nameCandidate`,
`suggestionDistance`, `maxIDSuggestions` and `minSharedWordLetters` from `suggest.go`, into
`internal/nearest`, exported (for example `EditDistance`, `Closest`, `Ranked`, `IsNear`); `suggestionDistance`
is exported too, since `unknownCommandError` also hands it to cobra. The callers
stay in cli and call the package: `unknownCommandError`, `closestFlagName`, `nearFlagNames`,
`closestIDs`. The package imports only the standard library.
**Regression guard.** `idWordSeparator` (suggest.go:119), which the moved `sharedWordCount` reads, moves to
`internal/nearest` too and is deleted from `suggest.go`; `suggestionDistance` comes from `root.go:221`, not
`suggest.go`, and stays referenced there by `unknownCommandError`. Acceptance gains staticcheck on
`./internal/cli/ ./internal/nearest/` and the -w grep (standing requirements (a) and (b)).
**Files:** internal/nearest/nearest.go, internal/nearest/nearest_test.go, internal/cli/root.go, internal/cli/suggest.go, internal/cli/suggest_test.go
**Read first:** internal/cli/suggest.go — closestNames, sharedWordCount, idWordSeparator, closestIDs;
internal/cli/root.go — editDistance, closestName, suggestionDistance; .github/workflows/ci.yml — staticcheck
**Tests:** `TestEditDistance_CountsEditsIgnoringCase` and `TestClosestNames_SuggestsCloseNamesInRankOrder`
move from `suggest_test.go` to `internal/nearest`. Become direct: none. Stay runCLI: every
`TestSuggest_*`; `TestClosestFlagName_PicksTheClosestVisibleFlag` stays in cli.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/nearest/` passes
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Suggest|ClosestFlag|Unknown'` passes
- `grep -nwE 'editDistance|closestName|closestNames|sharedWordCount|letterCount|isNearName|nameCandidate|suggestionDistance|maxIDSuggestions|minSharedWordLetters|idWordSeparator' internal/cli/*.go` prints nothing
- `go vet ./internal/nearest/ ./internal/cli/` is clean
- `go run honnef.co/go/tools/cmd/staticcheck@<STATICCHECK_VERSION> ./internal/cli/ ./internal/nearest/` is clean
**Closes:** cli-split-closest-names
**Commit:** `refactor(nearest): move name suggestion ranking out of internal/cli`

## 2. Move the selection of an Issue's Notes to internal/briefing — ✅ DONE (2026-11-23)

NOTES (2026-11-23): search and know search map index.SearchEveryWordFirst's bool to output.MatchAll/MatchAny through one small cli helper, searchMatch in search.go, rather than repeating the if at each caller; the Notes selection ignores the bool as before.

NOTES (2026-11-23): the briefing tests index Notes through a new indexNoteFiles helper that reuses indexIssueFiles' store and refreshes after writing the Note files; the direct ApplyingNotes tests cover ref-then-title order, the limit, a Note found by both ref and title, and the any-word fallback.

**What:**
**Goal:** `internal/briefing` decides which Notes apply to an Issue — the Notes whose refs name a path
the Issue's body mentions, then the Notes its title finds, capped at a given limit — with direct unit
tests, and `internal/index` holds the every-word-then-any-word search fallback; no file of
`internal/cli` defines `applyingNotes`, `mentionsPath`, `endsPath`, `isPathByte` or
`searchEveryWordFirst`; `context`, `search` and `know search` print byte-identical output.
**Approach (assumed at the header base):** `searchEveryWordFirst` (`search.go`) moves to `index` as an
exported generic function that reports whether the every-word search matched as a bool; its three
cli callers (`search.go`, `know_search.go`, the moved Notes selection) map that bool to
`output.MatchAll`/`output.MatchAny` where they need the word. `applyingNotes` with `mentionsPath`,
`endsPath`, `isPathByte` and `applyingNote` moves to `briefing` as a function taking the
`*index.Index`, the title, the body and the limit, returning an exported entry-and-ref pair;
`contextNoteLimit`, `contextNotes`, `contextPlan` and the rendering stay in `context.go`.
**Regression guard.** Also update `internal/briefing`'s package comment (`internal/briefing/ready.go:1-4`,
which enumerates its API) to name the Notes selection. Acceptance gains staticcheck on
`./internal/cli/ ./internal/briefing/ ./internal/index/` and the -w grep over the moved names.
**Files:** internal/index/search.go, internal/index/search_test.go, internal/briefing/context.go, internal/briefing/context_test.go, internal/briefing/ready.go, internal/cli/context.go, internal/cli/context_test.go, internal/cli/search.go, internal/cli/know_search.go
**Read first:** internal/cli/context.go — applyingNotes, applyingNote, contextNotes, mentionsPath; internal/cli/search.go — searchEveryWordFirst;
internal/index/notes.go — SearchNotes, NoteRefs; internal/briefing/ready_test.go — indexIssueFiles
**Tests:** `TestMentionsPath_WholePathOrDirectory_Counts` moves to `internal/briefing`. Become direct: the
Notes selection that `TestContext_FullIssue_PrintsShowThenEverySection` sets up (ref-named, prefix-only,
title hit) gains a direct test in `briefing`; the runCLI test stays whole as the journey. New: a
direct test of the index fallback (every word first, else any word).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/briefing/` and `./internal/index/` pass
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Context|Search'` passes
- `grep -nwE 'applyingNotes|applyingNote|mentionsPath|endsPath|isPathByte|searchEveryWordFirst' internal/cli/*.go` prints nothing
- `go run honnef.co/go/tools/cmd/staticcheck@<STATICCHECK_VERSION> ./internal/cli/ ./internal/briefing/ ./internal/index/` is clean
**Closes:** cli-split-applying-notes
**Commit:** `refactor(briefing): move the selection of an Issue's Notes out of internal/cli`

## 3. Move the recap boundary to internal/recap — ✅ DONE (2026-11-23)

NOTES (2026-11-23): the exported sentinel recap.Boundary fails with is the existing recap.ErrUnknownBoundary (wrapped with the --since value), so cli maps both Boundary's and Compute's unknown boundary to unknownSinceError through one errors.Is check; no new sentinel was added.

NOTES (2026-11-23): consequential edit — internal/recap/recap.go: made necessary by Boundary also failing with ErrUnknownBoundary (its doc comment now names Compute and Boundary).

NOTES (2026-11-23): boundary_test.go adds a "before the first commit" row (40d → "") and a direct unknown-since test ("no-such-ref", "-x") beside the days, date and ref rows the plan names.

**What:**
**Goal:** `internal/recap` resolves a `--since` value (a date, `<n>d`, or a git ref) to the boundary
commit, with direct unit tests; no file of `internal/cli` defines `recapBoundary` or `sinceTime`; an
unknown `--since` still fails with the same message and hint.
**Approach (assumed at the header base):** `sinceTime`, `sinceDaysPattern`, `sinceDateLayout`,
`sinceOptionPrefix` and the body of `app.recapBoundary` move to `recap` as a function of the worktree
root, the value and the current time; where `recapBoundary` returns `unknownSinceError`, the moved
function returns an exported sentinel error, which cli maps to `unknownSinceError` (its wording
stays). `withEmptyGroups`, `recapResult` and the group rendering stay: they shape output.
**Regression guard.** Acceptance gains staticcheck on `./internal/cli/ ./internal/recap/` and the -w grep.
**Files:** internal/recap/boundary.go, internal/recap/boundary_test.go, internal/cli/recap.go, internal/cli/recap_test.go
**Read first:** internal/cli/recap.go — recapBoundary, sinceTime, unknownSinceError; internal/recap/recap.go — Compute, ErrUnknownBoundary;
internal/gitx/history.go — CommitBefore, ResolveCommit; internal/recap/recap_test.go — newStore
**Tests:** Become direct: the `days`, `date` and `ref` cases of
`TestRecap_EachSinceForm_ListsTheChangesAfterTheBoundary` become a table test of the moved function
in `internal/recap` (a real temp git repo); the runCLI test keeps its `default` and `days` cases.
Stay runCLI: `TestRecap_UnknownSince_IsInvalidWithTheGitLogHint` and the other `TestRecap_*`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/recap/` passes
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run Recap` passes
- `grep -nwE 'recapBoundary|sinceTime|sinceDaysPattern|sinceDateLayout|sinceOptionPrefix' internal/cli/*.go` prints nothing
- `go run honnef.co/go/tools/cmd/staticcheck@<STATICCHECK_VERSION> ./internal/cli/ ./internal/recap/` is clean
**Closes:** cli-split-recap-boundary
**Commit:** `refactor(recap): move the --since boundary out of internal/cli`

## 4. Move the foreign-lock write decision to internal/lock — ✅ DONE (2026-11-23)

NOTES (2026-11-23): the decision is `lock.Worktree.CheckWrites(chain func() []proc.Process, writes [][]string) (*HeldError, error)`; a failed renewal comes back as the new `*lock.RenewalError` (Error gives TouchHolder's text, Unwrap returns it), any other error is Check's read error unchanged; the chain func is called at most once and its result reused for TouchHolder, matching cli's cached callerAncestry. `renewHolderLock` became the unexported `Worktree.renewForHolder`; `writesOnly` the unexported `lock.writesOnly`.

NOTES (2026-11-23): `lockWarnApply` in internal/cli/lock_warn_test.go was used only by the deleted `TestLockWarn_WriteUnderTheHolderProcess_IsSilentAndOtherProcessesWarn` and is removed with it (staticcheck U1000); the renewal-failure test masks the random `.tmp-N` suffix of the record's temp file when comparing with TouchHolder's error text, as two failed writes never name the same temp file.

**What:** Recast at the regression check (2026-11-23).
**Goal:** `internal/lock` decides whether a command's writes must warn of another session's Tree lock
(ADR 0017: the holder's own process chain renews the lock and stays silent, writes of only the
lock's purpose Issue stay silent, a stale lock is reported stale), with direct unit tests; no file of
`internal/cli` defines `renewHolderLock` or `writesOnly`, `app.lockWarning` only words the decision,
and every warning text stays byte-identical.
**Approach (assumed at the header base):** the body of `app.lockWarning` after its early returns —
`Check`, `HeldBy` over the caller ancestry, `renewHolderLock`'s `TouchHolder` handling, `writesOnly` —
moves to a method of `lock.Worktree` taking the caller chain lazily (`func() []proc.Process`) and the writes
(`[][]string`), returning the foreign `*HeldError` (nil when silent) and an error that tells an
unreadable record from a failed renewal. The early returns (no writes, `isLockExempt`, no store),
`callerAncestry`, `holderProcess` and the texts `unreadableLockWarning`, `unrenewedLockWarning`,
`foreignLockAdvice`, `foreignStaleLockAdvice` stay in `root.go`.
**Regression guard.** Take the caller chain lazily as `func() []proc.Process` (cli passes `a.callerAncestry`,
as `primeResult` does), called only when the lock record names a holder process, so the ancestry is never
read for a free lock or a holder-less record (root.go's "The caller's ancestry is read only for a record
that names a holder process" and the archived plan `2026-11-22 - 00 - lock-holder-and-agent-guide-plan`'s
"Read the chain lazily" stand; this item yields to them). A direct test in `internal/lock` proves the chain
func is not called when the record names no holder process. Keep `TestLockWarn_WriteOfThePurposeIssue_IsSilent`,
`TestLockWarn_NoteWhoseIDIsThePurpose_Warns` and `TestLockWarn_ResumeUnderAnotherProcess_MovesTheHolder` in cli
as journey cases (they are the only checks of mv noting old+new id, Note writes noting no id, and acquire's
resume recording the new holder); the decision table becomes new direct tests in `internal/lock`; only
`TestLockWarn_WriteUnderTheHolderProcess_IsSilentAndOtherProcessesWarn` is deleted once covered. Acceptance
gains staticcheck on `./internal/cli/ ./internal/lock/` and the -w grep. `Check`, `HeldBy` and `TouchHolder` leave
`root.go` (lines 510, 519, 541 today; `cli/lock.go:374`'s `HeldBy` is acquire's and stays), so `lockWarning` only
words the decision. The failed-renewal error carries `TouchHolder`'s error unchanged (unwrappable, same text), so
`unrenewedLockWarning`'s `TrimPrefix(err.Error(), recordPath+": ")` still strips; a direct test proves it.
**Files:** internal/lock/write.go, internal/lock/write_test.go, internal/cli/root.go, internal/cli/lock_warn_test.go
**Read first:** internal/cli/root.go — app.lockWarning, renewHolderLock, writesOnly, callerAncestry, unrenewedLockWarning;
internal/lock/worktree.go — Worktree.Check, Worktree.TouchHolder; internal/lock/lock_test.go — TestReadCheck_RecordWithNoGuardInReadOnlyDir_Succeed
**Tests:** Become direct (deleted from cli once covered in `internal/lock`):
`TestLockWarn_WriteUnderTheHolderProcess_IsSilentAndOtherProcessesWarn`. New in `internal/lock`: the decision
table as direct tests, a test that the chain func is not called when the record names no holder process, and a
test of the renewal-failure branch (lock dir made read-only after acquire, as
`TestReadCheck_RecordWithNoGuardInReadOnlyDir_Succeed` sets up) asserting the error's kind and that it
`errors.Is` / matches `TouchHolder`'s error text.
Stay runCLI as journey cases: `TestLockWarn_WriteOfThePurposeIssue_IsSilent`,
`TestLockWarn_NoteWhoseIDIsThePurpose_Warns`, `TestLockWarn_ResumeUnderAnotherProcess_MovesTheHolder`, and every
other `TestLockWarn_*`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/lock/` passes
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'LockWarn|Lock'` passes
- `grep -nwE 'renewHolderLock|writesOnly' internal/cli/*.go` prints nothing
- `grep -nE 'HeldBy|TouchHolder|\.Check\(' internal/cli/root.go` prints nothing
- `go run honnef.co/go/tools/cmd/staticcheck@<STATICCHECK_VERSION> ./internal/cli/ ./internal/lock/` is clean
**Closes:** cli-split-lock-warning
**Commit:** `refactor(lock): move the foreign-lock write decision out of internal/cli`

## 5. Move the Issue-file merge to internal/mergedriver

**What:**
**Goal:** a new package `internal/mergedriver` merges an Issue file's base, ours and theirs versions
into ours — the Log by entry, the rest by git's text merge with the given conflict labels — and reports
a conflict or the path that failed, with direct unit tests; no file of `internal/cli` defines
`mergeIssueFile` or `gitTextMerge`; merge-driver exit codes, failures and hints stay byte-identical.
**Approach (assumed at the header base):** from `mergeIssueFile`, the reading of the three inputs,
`issue.MergeFile` and `nodefile.ReplaceFileVerbatim`, plus `gitTextMerge` with
`mergeTempDirPattern` and `mergeInputPermissions`, move to `mergedriver` as a function of the three
paths and a `gitx.MergeLabels`, returning whether it conflicted, the path of a failed read or write,
and the error. cli keeps the argument positions, `mergeLabelsFrom`, `mergeLabel`, `failedMerge`,
`conflictMessage` and builds the same `output.Error`s from what the function returns.
**Regression guard.** Keep `TestMergeDriver_MissingInput_FailsNamingItAndKeepsOurs` and
`TestMergeDriver_OursInReadOnlyDirectory_FailsInvalidAndKeepsOurs` in cli as journey cases (cli still words those
failures from the returned path); their direct counterparts in `internal/mergedriver` assert the returned path and
error. Only the success-path tests (BothAppendOutsideAnyStore, CRLFInputs_KeepCRLF) are deleted once covered.
Acceptance gains staticcheck on `./internal/cli/ ./internal/mergedriver/` and the -w grep. A failed
`issue.MergeFile` returns no path, so cli keeps wording that failure with `args[mergeArgPath]`; the moved
function's return must let cli tell a read/write failure (with its path) from a merge failure (no path), and the
kept runCLI journey cases stay byte-identical.
**Files:** internal/mergedriver/mergedriver.go, internal/mergedriver/mergedriver_test.go, internal/cli/merge_driver.go, internal/cli/merge_driver_test.go
**Read first:** internal/cli/merge_driver.go — mergeIssueFile, gitTextMerge, failedMerge; internal/issue/merge.go — MergeFile, TextMerge;
internal/gitx/merge.go — MergeLabels; internal/nodefile/atomic.go — ReplaceFileVerbatim; internal/cli/merge_driver_test.go — writeDriverInputs
**Tests:** Become direct (deleted from cli once covered in `mergedriver`):
`TestMergeDriver_BothAppendOutsideAnyStore_WritesBothEntriesToOurs`, `TestMergeDriver_CRLFInputs_KeepCRLF`.
Stay runCLI as journey cases, each with a direct counterpart in `mergedriver` asserting the returned path and
error: `TestMergeDriver_MissingInput_FailsNamingItAndKeepsOurs`,
`TestMergeDriver_OursInReadOnlyDirectory_FailsInvalidAndKeepsOurs` (with its platform skip). Stay
runCLI: `TestMergeDriver_ConflictingTitles_*`, `TestMergeDriver_LabelsAbsentOrUnexpanded_FallBackToPath`
and `merge_property_test.go`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/mergedriver/` passes
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run MergeDriver` passes
- `grep -nwE 'mergeIssueFile|gitTextMerge|mergeTempDirPattern|mergeInputPermissions' internal/cli/*.go` prints nothing
- `go run honnef.co/go/tools/cmd/staticcheck@<STATICCHECK_VERSION> ./internal/cli/ ./internal/mergedriver/` is clean
**Closes:** cli-split-merge-driver
**Commit:** `refactor(mergedriver): move the Issue-file merge out of internal/cli`

## 6. Move the indexed Dependency graph to internal/index

**What:** Recast at the regression check (2026-11-23).
**Goal:** `internal/index` holds the Dependency graph of the indexed Issues — each
Issue's index entry by id and in tree order, whether a dep entry names an id, which Issues `dep graph`
shows from a start, and `dep list`'s branches by first step — with direct unit tests; no file of
`internal/cli` defines the `depGraph` type, `loadDepGraph` or `depBranches`; `dep graph`, `dep list`,
`related` and `context` print byte-identical output.
**Approach (assumed at the header base):** the `depGraph` struct (`graph`, `entries`, `ordered`) becomes
an exported type in `internal/index` with accessors; `loadDepGraph` becomes a constructor over `*index.Index`; `names` and
`shownBy` become its methods; `depBranches` moves beside it. `flowchartResult` and `listResult` return
`output.Result`, so they stay in cli as functions taking the graph; `depSection`, `linkHolder` and
`showGraph` stay. Callers to update: `dep_graph.go`, `dep_list.go`, `related.go`, `context.go`.
**Regression guard.** Recast to the ratified home: the indexed Dependency graph moves into `internal/index`,
not a new `internal/depgraph` package: an exported type in `index` (its name must not be confused with
`issue.DepGraph`; the implementer picks a clear name) with a constructor over `*Index`, the names and shown-by
methods, and `depBranches` beside it. Keep case 1 of `TestDepGraph_IDAndAll_LimitTheNodes` (an id without
`--all`: the closed dependency hidden and its edge left out, the only check of `flowchartResult` dropping that
edge) in the runCLI test beside the id-and-`--all` case; only the closed-named-by-id case becomes direct.
**Files:** internal/index/depgraph.go, internal/index/depgraph_test.go, internal/cli/dep_graph.go, internal/cli/dep_list.go, internal/cli/related.go, internal/cli/context.go, internal/cli/doc.go, internal/cli/dep_test.go
**Read first:** internal/cli/dep_graph.go — depGraph, loadDepGraph, depGraph.shownBy, flowchartResult; internal/cli/dep_list.go — depBranches;
internal/index/query.go — Index.List; internal/index/index_test.go — newStore; internal/cli/dep_test.go — TestDepGraph_IDAndAll_LimitTheNodes
**Tests:** Become direct: the closed-named-by-id case of `TestDepGraph_IDAndAll_LimitTheNodes` becomes a
direct test of the shown-by method in `internal/index`; the runCLI test keeps case 1 (an id without `--all`,
the closed dependency's edge left out) and the case with an id and `--all`. Stay
runCLI: every `TestDepList_*`, `TestDepGraph_NoID_*`, `TestDepGraph_JSON_*` and `related_test.go`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/` passes
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Dep|Related|Context|DocFileMap'` passes
- `grep -nwE 'depGraph|loadDepGraph|depBranches' internal/cli/*.go` prints nothing
- `go run honnef.co/go/tools/cmd/staticcheck@<STATICCHECK_VERSION> ./internal/cli/ ./internal/index/` is clean
- the `doc.go` line for `dep_graph.go` names `internal/index` as the home of the Dependency graph
**Closes:** cli-split-dep-graph
**Commit:** `refactor(index): move the indexed Dependency graph out of internal/cli`

## 7. Move Note successor-list edits to internal/ops

**What:**
**Goal:** `internal/ops` resolves Note names (an id, or an alias exactly one Note claims) and applies
the add and remove edits to a Note's superseded-by list, beside the Issue-side `namesIssue` and
`ReferenceList`, with direct unit tests; no file of `internal/cli` defines `noteNames` or
`editSuccessors`; `know update`'s output and refusals stay byte-identical.
**Approach (assumed at the header base):** `noteNames` moves to `ops` as an exported function over
`[]index.NoteEntry` returning a `reflist.NamesFunc`; the add-then-remove loop of `app.editSuccessors`
(`reflist.List.Add`, `Remove`, `Removal.Drop`, `ops.WithoutEntries`) moves to `ops`, returning the
raw `reflist` error so cli keeps wrapping it with `refusalFailure(err, noteIDs, "", successorWords)`.
`successorList` stays in cli (its resolution failure is wording) and calls the moved `noteNames`.
**Regression guard.** Restate the cli comments naming `noteNames`/`editSuccessors` (`know_update.go:181,314`).
Acceptance gains staticcheck on `./internal/cli/ ./internal/ops/` and the -w grep.
**Files:** internal/ops/notes.go, internal/ops/notes_test.go, internal/cli/know_update.go, internal/cli/know_update_test.go
**Read first:** internal/cli/know_update.go — editSuccessors, noteNames, successorList, applyNoteUpdate;
internal/ops/deps.go — namesIssue, ReferenceList, WithoutEntries; internal/reflist — NamesFunc
**Tests:** Become direct (deleted from cli once covered in `ops`):
`TestKnowUpdate_AddThenRemoveTheSameSuccessor_RemovesIt`,
`TestKnowUpdate_AddSupersededByBesideItsAliasSpelling_AddsNothing`. Stay runCLI:
`TestKnowUpdate_SupersededByAliasOrCitation_StoresTheID` (the journey) and every refusal test.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ops/` passes
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run KnowUpdate` passes
- `grep -nwE 'noteNames|editSuccessors' internal/cli/*.go` prints nothing
- `go run honnef.co/go/tools/cmd/staticcheck@<STATICCHECK_VERSION> ./internal/cli/ ./internal/ops/` is clean
**Closes:** cli-split-note-successors
**Commit:** `refactor(ops): move Note successor-list edits out of internal/cli`

## 8. Move the refs a removal breaks to internal/knowledge

**What:**
**Goal:** `internal/knowledge` selects, from a set of Notes, the refs that name a removed file or its
children directory — the removed Note itself excluded, each Note keeping only its naming refs — with
direct unit tests; `app.brokenRefWarnings` only lists the Notes from the index, calls that selection
and `knowledge.Verify`, and words the warnings; `rm` and `know rm` warnings stay byte-identical.
**Approach (assumed at the header base):** the loop of `app.brokenRefWarnings` over the index's Notes
(skip the removed path, `tree.ChildrenDir`, `knowledge.RefsNaming`, build a `knowledge.NoteFile`) moves
to `knowledge` as a function over `[]knowledge.NoteFile` and the removed path; cli turns each
`index.NoteEntry` into a `NoteFile` first. `refsUncheckedWarnings` and the wording stay in `rm.go`.
**Regression guard.** Acceptance gains staticcheck on `./internal/cli/ ./internal/knowledge/` and the -w grep
over the moved names.
**Files:** internal/knowledge/removal.go, internal/knowledge/removal_test.go, internal/cli/rm.go
**Read first:** internal/cli/rm.go — brokenRefWarnings, refsUncheckedWarnings; internal/knowledge/verify.go — RefsNaming, NoteFile, Verify;
internal/tree — ChildrenDir; internal/cli/node_verbs.go — caller of brokenRefWarnings;
internal/knowledge/verify_test.go — TestRefsNaming_FileDirectoryAndBelow_ReturnsOnlyThoseRefs
**Tests:** Become direct: none; a new direct test covers the selection (a ref to the file, one to its
children directory, one to a sibling, the removed Note's own ref). Stay runCLI:
`TestRm_FileANoteRefNames_RemovesAndWarnsLikeCheck`, `TestKnowRm_FileANoteRefNames_RemovesAndWarnsLikeCheck`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/knowledge/` passes
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Rm|KnowRm'` passes
- `grep -nwE 'RefsNaming' internal/cli/*.go` prints nothing
- `go run honnef.co/go/tools/cmd/staticcheck@<STATICCHECK_VERSION> ./internal/cli/ ./internal/knowledge/` is clean
**Closes:** cli-split-removal-refs
**Commit:** `refactor(knowledge): move the refs a removal breaks out of internal/cli`
