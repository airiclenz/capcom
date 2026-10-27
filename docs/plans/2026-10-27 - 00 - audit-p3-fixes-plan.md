# Plan: the rest of the 2026-10-26 audit, and one shared-Ref chore

**Goal:** Close the eleven remaining `audit-1026` issues and `shared-ref-repository`: two Tree lock
P2s (keep-lock resume, warn on foreign-lock writes), three lock record fixes, a clear busy-index
error, batched list and excerpt reads, and six write, Ref and docs fixes.

**Date:** 2026-10-27
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 8982eca

**Regression check (2026-10-27, 8982eca):**
- 3: guard folded
- 4: guard folded; yields to `internal/cli/flags.go:67-70` (valueChecker) and `json-output.md:164,174`: exit 1 `invalid`
- 5: guard folded (decision); supersedes the "plan field set" Planned in `file-format.md:139`, `glossary-work/plan.md:6`, `migrating-from-beads.md:109`
- 6: guard folded (decision); supersedes `nodestore/refs.go:66` and `nodestore/move.go:37-45`
- 7: guard folded (decision)
- 8: guard folded (decision)
- 9: guard folded (decision)
- 10: guard folded
- 11: guard folded
- 12: guard folded (decision)
- 13: guard folded
- 14: guard folded
- 15: guard folded
- 16: guard folded (decision); supersedes `file-format.md:9` and `store-layout.md:9` for `worktree-id`
- 17: guard folded (decision)
- 18: guard folded

**Sources:**
- `.capcom/issues/audit-1026/*.md`, `.capcom/issues/shared-ref-repository.md` (each item names its issue)
- ADR 0016 and amendment `adr-0016-amend-1026`, ADR 0017, ADR 0005, ADR 0012, ADR 0007
- Notes `cli-test-harness`, `error-messages`, `store-layout`, `docs-layout`, `issue-workflow`
- `docs/site/reference/file-format.md`, `docs/site/reference/json-output.md`

**Ratified design calls** (user, 2026-10-27):
- **Scope:** the audit-1026 cluster plus `shared-ref-repository`; features out.
- **Busy index:** a typed `ErrBusy` and a clear error; no stale-read fallback.
- **Token at rest:** the record's `token` key holds `sha256:<hex>`; legacy cleartext records still match and are rewritten on touch or resume.
- **Worktree identity:** a random id in `<gitDir>/capcom/worktree-id`, copied into the record as `worktree`; orphans are reported and taken over, never pruned by a new verb.
- **keep-lock next line:** `next: <id>  <title>; capcom start <id> --token <token>`.
- **Comment warning:** every CLI write that rewrites a file warns: the Edit path and the mv/rm rewrites of other files; UI and `fmt` out.
- **--planned:** `ready --planned`/`--unplanned` and `next` agree with `prime`: planned means the plan resolves to a file.
- (writer) **mv Refs:** a Ref is rewritten when its path part equals the moved file, its children directory, or lies below that directory; `#symbol` kept.
- (writer) **Busy error code:** `invalid` (exit 1); `locked` stays the Tree lock's.
- (writer) **Orphan state:** `State()` stays `StateFree`; a new `Status.IsOrphaned` marks it, so no consumer of the state enum changes.
- (writer) **Lock warning sites:** explicit `noteWrite` calls at each write, not a cobra annotation.

**Standing requirements:**
- skills: coding-standards
- Tests on the Pi host: one package at a time, `GOMEMLIMIT=2GiB`, never `./...` with `-race`.
- CLI tests go through `runCLI` (Note `cli-test-harness`), never `t.Parallel()`.
- Every new error or warning names the file, and the line when known (Note `error-messages`).
- A changed help text is followed by `go run ./cmd/gendocs`; `TestReferenceUpToDate` must pass.

**Out of scope:**
- `batch-apply`, `tree-status-filter`, `html-export`, `agent-eval-harness`.
- Batched reads for Notes (`noteList`/`readNote`) and the per-file `DELETE FROM fts` in refresh.
- UI notices for dropped comments; a prune verb for orphaned lock records.
- CHANGELOG release headings, VERSION, tags.

## 1. One constructor for a store's knowledge.Repository — ✅ DONE (2026-10-27)

**What:**
**Goal:** `nodestore.RefRepository(s *store.Store) knowledge.Repository` is the only place a
`knowledge.Repository` literal is built outside tests; `know add`/`know update`, `know verify` and
`capcom check` all call it. No behaviour change.
**Approach (assumed at the header base):** add the func beside `Issues`/`Notes` in
`internal/nodestore/kind.go` (WorktreeRoot, GitDir, GitCommonDir from the store, IgnoredPaths:
`gitx.IgnoredPaths`). Replace the literals in `checkNewRefs` (`internal/cli/know_add.go`),
`internal/cli/know_verify.go` and `checkRefs` (`internal/check/notes.go`). knowledge and store gain
no import; the only new edge is nodestore→gitx.
**Files:** internal/nodestore/kind.go, internal/cli/know_add.go, internal/cli/know_verify.go, internal/check/notes.go
**Read first:** internal/nodestore/kind.go — Issues, Notes; internal/knowledge/verify.go — Repository, IgnoreFunc, Verify; internal/cli/know_add.go — checkNewRefs;
internal/cli/know_verify.go — verifyNotes; internal/check/notes.go — checker.checkRefs; internal/check/check.go — checker (store field);
internal/gitx/ignore.go — IgnoredPaths
**Closes:** shared-ref-repository
**Tests:** existing tests pass unchanged.
**Acceptance:**
- `grep -rn 'knowledge.Repository{' internal --include=*.go | grep -v _test.go` prints only `internal/nodestore/kind.go`
- `go vet ./internal/nodestore/ ./internal/cli/ ./internal/check/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodestore/` then `./internal/check/` then `./internal/knowledge/` then `./internal/cli/`
**Commit:** `refactor(knowledge): build a store's Ref repository in one place`

## 2. Search help says a CJK run is one word — ✅ DONE (2026-10-27)

**What:**
**Goal:** `capcom search --help` and `capcom know search --help` each say that a run of CJK
characters with no spaces counts as one word, with an example: `登录超时问题` is found by
`登录超时问题`, not by `登录`. The CLI reference is regenerated.
**Approach (assumed at the header base):** one paragraph in `searchLong`
(`internal/cli/search.go`) and `knowSearchLong` (`internal/cli/know_search.go`), within the
paragraph limits `internal/cli/help_text_test.go` enforces; then `go run ./cmd/gendocs`.
**Files:** internal/cli/search.go, internal/cli/know_search.go, docs/site/reference/cli/capcom_search.md, docs/site/reference/cli/capcom_know_search.md
**Read first:** internal/cli/search.go — searchLong; internal/cli/know_search.go — knowSearchLong; internal/clidoc/reference_test.go — TestReferenceUpToDate;
internal/cli/help_text_test.go — TestHelpText_EveryCommand_PrintsExamplesBeforeShortRules, helpRules, wrappedLineCount; internal/index/index.go — Index.Search, searchIDs
**Closes:** search-cjk-substring
**Tests:** `internal/cli/help_text_test.go` and `TestReferenceUpToDate` cover it.
**Acceptance:**
- `go run ./cmd/capcom search --help | grep -F '登录超时问题'` and the same for `know search --help`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Help'` and `./internal/clidoc/`
**Commit:** `docs(search): say a run of CJK characters counts as one word`

## 3. No pinned version in docs, and a --ref example that resolves — ✅ DONE (2026-10-27)

NOTES (2026-10-27): terminal-ui.md writes the version as `vX.Y.Z`, not `v<version>`: it is as wide as `v0.7.0`, so the sample screen's tab bar keeps its 77-column alignment with the rule below it.
NOTES (2026-10-27): consequential edit — internal/cli/testdata/help/know-add.txt: made necessary by the knowAddExample change (golden help file of TestHelp_NodeVerbs_MatchGolden).
NOTES (2026-10-27): the --ref scan also trims trailing sentence punctuation (`.,;:)` and backtick) from a value before the path check, and a table test (TestRefPaths_ProseAndExamples_KeepsOnlyPaths) pins that prose values are skipped.

**What:**
**Goal:** no hand-written page under `docs/site/` pins a capcom version; the `--ref` example in
`README.md` and in `capcom know add --help` names `docs/releasing.md`; a test fails when a `--ref`
or `--add-ref` value in `README.md` or in any command's `Long`/`Example` names a path missing from
this repository.
**Approach (assumed at the header base):** in `docs/site/getting-started/terminal-ui.md` write the
sample screen's version generically (`v<version>`). Fix `README.md` and `knowAddExample`
(`internal/cli/know_add.go`), regenerate. New test in `internal/cli/help_text_test.go`: walk
`availableCommands()` plus `../../README.md`, match `--(add-)?ref[ =]("[^"]*"|\S+)`, strip
`#symbol`, `os.Stat` from the repo root. docs/site pages are not scanned (they carry other
projects' examples); no version-regex test (the Note `docs-layout` holds that rule).
**Regression guard.** The value pattern also matches prose in each `Long` (`knowAddLong`: "--ref set", "A --ref
inside"; `knowUpdateLong`: "--add-ref and", "--add-ref inside"), so the test keeps scanning `Long`
as the Goal says but accepts a match only when its value holds a `/` or a `.` (a path); "set",
"inside" and "and" are never stat'ed.
**Files:** docs/site/getting-started/terminal-ui.md, README.md, internal/cli/know_add.go, internal/cli/help_text_test.go, docs/site/reference/cli/capcom_know_add.md
**Read first:** internal/cli/know_add.go — knowAddExample, knowAddLong; internal/cli/know_update.go — knowUpdateLong, knowUpdateExample; README.md — know add example (line with --ref docs/release.md);
internal/cli/help_text_test.go — availableCommands, TestHelpText_EveryCommand_SetsLongAndExample; docs/site/reference/cli/capcom_know_add.md — Examples;
docs/site/getting-started/terminal-ui.md — The screen (sample screen and Tab bar bullet)
**Closes:** docs-show-stale-version
**Depends on:** item 1 (same file)
**Tests:** the new test; it must fail against the pre-item `knowAddExample` and pass on the
prose `--ref`/`--add-ref` mentions in `knowAddLong`/`knowUpdateLong`.
**Acceptance:**
- `grep -rnE 'v0\.[0-9]+\.[0-9]+' docs/site/getting-started/` prints nothing
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Help|Ref'` and `./internal/clidoc/`
**Commit:** `fix(docs): unpin the UI page's version and point the --ref example at a real file`

## 4. --plan refuses a path and warns on a missing plan — ✅ DONE (2026-10-27)

NOTES (2026-10-27): the name rule is `Config.ValidatePlanName` (a method, as the Approach allowed) and the shared message `Config.MissingPlanMessage`; `create.go` gains `checkPlanValue` and `missingPlanWarnings`, which update reuses; `flags.go`'s valueChecker is untouched, the refusal goes through `output.Errorf(output.CodeInvalid, ...)` as invalidValues does.
NOTES (2026-10-27): the existing `--plan login-plan` test (`TestCreate_UnderParent_CreatesParentDirectory`) now writes `docs/plans/login-plan.md` so its plan resolves, rather than asserting the warning; the warning itself is asserted by the new `TestCreateAndUpdate_MissingPlan_WritesAndWarnsLikeCheck`.
NOTES (2026-10-27): file-format.md gains the refusal and warning wording under the config `plans` key, not in the issue table's `plan` row, which item 5 owns.

**What:**
**Goal:** `create --plan` and `update --plan` refuse, exit 1 (`invalid`), a value that is absolute or
contains `/`, `\`, or is `.` or `..`, with a message naming the plans directories; `--plan ""`
still clears. A basename with no plan file is written with a warning
`<issue path>: plan "<x>" is in none of the plans directories [<dirs>]`, the same text
`capcom check` reports. `config.PlanPath` uses the same name rule.
**Approach (assumed at the header base):** `config.ValidPlanName(c Config, plan string) error` (or
a method) in `internal/config/config.go`, used by `PlanPath`, `checkCreateValues`
(`internal/cli/create.go`) and `checkUpdateValues` (`internal/cli/update.go`). The missing-plan
message moves to one exported func in `internal/config` called from `internal/check/issues.go`
and both commands; warnings go to `result.Warnings` like `refWarnings` in `know_add.go`.
**Regression guard.** The refusal is `invalid`, exit 1, through `invalidValues`/`output.CodeInvalid` like
`--status` on create/update, never a usage error: the item yields to the rule in `valueChecker`'s
comment (`internal/cli/flags.go:67-70`) and to `docs/site/reference/json-output.md:164,174`, whose
exit-2 row is only an unknown command or flag, a repeated flag or a wrong arg count. In
`docs/site/reference/file-format.md` this item edits only the refusal and warning wording; item 5
owns the plan row's Planned definition.
**Files:** internal/config/config.go, internal/config/config_test.go, internal/cli/create.go, internal/cli/update.go, internal/check/issues.go, internal/cli/write_test.go, docs/site/reference/file-format.md
**Read first:** internal/config/config.go — Config.PlanPath, Default (Plans); internal/cli/create.go — checkCreateValues, create, newIssue; internal/cli/update.go — checkUpdateValues, newUpdateCommand, applyUpdate;
internal/check/issues.go — checker.checkIssue; internal/cli/flags.go — valueChecker, invalidValues, usageValues; internal/cli/know_add.go — refWarnings;
internal/config/config_test.go — TestPlanPath_PathInsteadOfBasename_IsRefused; internal/cli/write_test.go — TestCreate_UnderParent_CreatesParentDirectory, mustRun
Part of issue `plan-accepts-traversal`; the later item closes it.
**Tests:** refusal (exit 1, code `invalid`) of `../../etc/passwd`, `/etc/passwd`, `a/b`, `..`; warning for `no-such-plan`;
no warning for a plan whose file exists; existing `--plan login-plan` tests updated for the warning.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/config/` then `./internal/check/` then `./internal/cli/ -run 'Create|Update|Plan'`
**Commit:** `fix(plan): refuse a plan that is not a bare name and warn on a missing one`

## 5. Planned means the plan resolves, in prime, ready and next — ✅ DONE (2026-10-27)

NOTES (2026-10-27): consequential edit — internal/cli/finish.go: made necessary by readyOptions.listOptions now taking the worktree root (nextAfterFinish calls it; it passes a.store.WorktreeRoot, its plan option stays PlanAny)
NOTES (2026-10-27): consequential edit — internal/index/query.go: made necessary by the new Planned definition; the PlanState/PlanSet/PlanUnset doc comments said "Planned" and now say "plan field set", which is what the index (and capcom list) still selects on
NOTES (2026-10-27): the planned check lives in one unexported helper, briefing.isPlanned (config.PlanPath under the worktree root), used by readWork and ReadyOptions.keepsPlan; ready's long help gained one sentence on --planned/--unplanned besides the flag text
NOTES (2026-10-27): ready_test.go's indexIssueFiles, readyFixture and unblockFixture now also return the temp root (and indexIssueFiles writes the alpha-plan file), so the planned cases can set ReadyOptions.Root

**What:**
**Goal:** an issue counts as planned only when `config.PlanPath` resolves its plan to a file:
`prime` lists an unresolvable plan under `ready, unplanned`; `ready --planned`/`next --planned`
leave it out and `--unplanned` keeps it.
**Approach (assumed at the header base):** `readWork` (`internal/briefing/briefing.go`) splits on
`s.Config.PlanPath(s.WorktreeRoot, plan)`. `briefing.ReadyList` (`internal/briefing/ready.go`) applies
the planned/unplanned option itself through the same check after the index query, instead of
passing `index.PlanSet`/`PlanUnset` to the index; `index.PlanState` stays for other callers.
`ready`/`next` help says "a plan that resolves to a file".
**Regression guard.** the function is `briefing.ReadyList` (not `briefing.Ready`); item 5 owns the "Planned" definition in file-format.md's plan row (reworded to "planned when the plan resolves to a file") and adds docs/site/reference/file-format.md to its Files, while item 4 edits only the refusal and warning wording there.
`ReadyList` takes no worktree root: carry it in `ReadyOptions` (`Root string`, needed only when
`Plan != PlanAny`, set by `readyOptions.listOptions`), so its signature and callers (`finish.go`
`nextAfterFinish`, `ui/ready.go` `loadReady`, `ui/ready_test.go`, the `readyList` stub in
`briefing_test.go`) stay. `capcom list --planned`/`--unplanned` keep "plan field set" and their
help says so; `capcom_list.md` is regenerated. The "plan field set" definitions in the glossary
Note `plan` (`glossary-work/plan.md:6`) and `migrating-from-beads.md:109` are superseded and
reworded; `grep -rn -i 'planned' docs/site .capcom/knowledge` (generated `cli/` excepted) finds any other.
**Files:** internal/briefing/briefing.go, internal/briefing/ready.go, internal/briefing/briefing_test.go, internal/cli/ready.go, internal/cli/next.go, internal/cli/prime_test.go, docs/site/agents/briefing.md, docs/site/reference/cli/capcom_ready.md, docs/site/reference/cli/capcom_next.md, docs/site/reference/file-format.md, internal/briefing/ready_test.go, internal/cli/ready_test.go, internal/cli/list.go, docs/site/reference/cli/capcom_list.md, .capcom/knowledge/glossary/glossary-work/plan.md, docs/site/getting-started/migrating-from-beads.md
**Read first:** internal/briefing/ready.go — ReadyList, ReadyOptions, ReadyOptions.filter; internal/briefing/briefing.go — Briefing.readWork, readyList; internal/cli/ready.go — readyOptions.listOptions, newReadyCommand;
internal/cli/next.go — newNextCommand; internal/ui/ready.go — loadReady, readyOptions; internal/cli/ready_test.go — newReadyStore, TestReady_Flags_SelectReadyIssuesMostUrgentFirst;
internal/briefing/ready_test.go — readyFixture, indexIssueFiles, TestReadyList_Options_SelectAndSortStablyByPriority
**Closes:** plan-accepts-traversal
**Depends on:** item 4
**Tests:** a ready issue with `plan: no-such-plan` is unplanned in `prime` text and `--json`, absent
from `ready --planned`, present in `ready --unplanned`.
`TestReady_Flags_SelectReadyIssuesMostUrgentFirst` and
`TestReadyList_Options_SelectAndSortStablyByPriority` keep their wants: `newReadyStore` writes the
`alias-plan` file and `indexIssueFiles` writes `docs/plans/2026-10-12 - 00 - alpha-plan.md` (or
returns the root).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/briefing/` then `./internal/cli/ -run 'Prime|Ready|Next|List'` then `./internal/clidoc/`
- `go run ./cmd/capcom check`
**Commit:** `fix(prime): count an issue as planned only when its plan resolves`

## 6. A move rewrites the Note Refs that point at what it moved — ✅ DONE (2026-10-27)

NOTES (2026-10-27): Moved gains UnreadNotes (Notes whose Refs could not be checked, or the Knowledge dir when it cannot be walked) and RefsRewritten (Notes whose Refs were rewritten), both at post-move paths; the CLI does not render them yet (item 7).
NOTES (2026-10-27): the Note list comes from tree.Walk(knowledgeDir) in refs.go (planNoteRefs), not a loader in load.go; a walk failure joins UnreadNotes as the Knowledge directory's path.
NOTES (2026-10-27): the path pass treats every Note read failure as an UnreadNotes warning for a Note move too (not only an issue move); a Note rename's existing superseded-by pass keeps its fatal read failures, and a Note it already lists in Unread is not repeated in UnreadNotes.
NOTES (2026-10-27): Moved.Unread is now also relocated to post-move paths (files inside the moved children directory), per the item's regression guard; Kind.movedPaths reuses knowledge.RewriteRefs for the mapping.

**What:**
**Goal:** `Kind.Move` of an issue or a Note (rename or re-parent) rewrites, in every Note file, each
Ref whose path part equals the moved file, its children directory, or a path below that
directory, to the new path, keeping `#symbol`; Notes inside the moved subtree are rewritten too.
**Approach (assumed at the header base):** a deep func in `internal/knowledge`
(`RewriteRefs(refs []string, oldFile, newFile, oldDir, newDir string) ([]string, bool)`) owns the
Ref grammar (reuse `splitRef`). `planNodeMove` (`internal/nodestore/refs.go`) runs a path-refs pass
whenever `NewPath != node.Path`, over every Note under `s.KnowledgeDir()` (give `Kind` the Note
loader in `Issues()`/`Notes()`). One `tree.Rewrite` per path: merge with a superseded-by rewrite of
the same Note and with the moved Note's own content. Unparseable Notes go to their own unread list.
**Regression guard.** any path surfaced from a rewrite inside the moved subtree is reported at its post-move path, relocated as movedWarnings does.
For an issue move, a Note that cannot be read (any read failure, not only a refusal) or a Knowledge
walk failure (`internal/nodestore/load.go`) joins the Note unread list as a warning and never fails
the move; only the issues' own read failures stay fatal. This supersedes "a re-parent alone
rewrites no file" (`internal/nodestore/refs.go:66`) and `Kind.Move`'s doc (`move.go:37-45`):
rewrite both and every `grep -rn "re-parent alone\|rewrites no" internal/nodestore internal/cli` hit.
**Files:** internal/knowledge/verify.go, internal/knowledge/verify_test.go, internal/nodestore/refs.go, internal/nodestore/move.go, internal/nodestore/kind.go, internal/nodestore/move_refs_test.go
**Read first:** internal/nodestore/refs.go — planNodeMove, planRefRewrite, renameOwnRefs; internal/nodestore/move.go — Kind.Move, Moved; internal/nodestore/kind.go — Issues, Notes, Kind;
internal/knowledge/verify.go — splitRef; internal/tree/move.go — Rewrite, Move.planRenames; internal/tree/plan.go — Plan.AddRewrite;
internal/nodestore/move_refs_test.go — newRefIssues, newRefNotes, writeRefTree, TestMove_ReparentIssue_RewritesNoOtherFile, TestMove_ReparentNote_LeavesTheFileBytes
Part of issue `mv-stale-note-refs`; the later item closes it.
**Tests:** `RewriteRefs` table test (file, dir, below dir, `#symbol`, unrelated); nodestore: rename
and re-parent of an issue a Note refs, a `know mv` of a Note another Note refs and supersedes.
`TestMove_Reparent*_RewritesNo*` updated to the new meaning. An issue move with an unreadable
Note succeeds and warns; a rewrite inside the moved subtree is reported at its post-move path.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/knowledge/` then `./internal/nodestore/`
**Commit:** `fix(mv): rewrite Note Refs that point at a moved file or directory`

## 7. mv reports the Ref rewrites and its help says so — ✅ DONE (2026-10-27)

NOTES (2026-10-27): consequential edit — internal/cli/testdata/help/mv.txt: made necessary by the new issueMvHelp text (TestHelp_NodeVerbs_MatchGolden pins the help byte for byte; regenerated from `capcom mv --help`)
NOTES (2026-10-27): consequential edit — internal/cli/testdata/help/know-mv.txt: made necessary by the new noteMvHelp text (regenerated from `capcom know mv --help`)
NOTES (2026-10-27): nodeVerbs gains hasRefs (true for Notes), so an unparseable bystander of a Note move warns "superseded-by and refs" while an issue bystander keeps "deps and related"; moveResult builds on verbs.writeResult (kind, text, KV) and swaps in a moveData payload, leaving writeResult/noteWriteResult untouched.
NOTES (2026-10-27): an UnreadNotes entry that is the Knowledge directory (item 6's walk failure) warns "<dir>: refs not checked for <id>: the directory cannot be read" instead of "the file does not parse"; a Note read failure keeps the "does not parse" wording, as moved.Unread already does.

**What:**
**Goal:** `capcom mv` and `capcom know mv` name rewritten Notes in their output like deps today;
the unread-Note warning names Refs for Notes it could not parse; both `--help` texts say Note Refs
to the moved path are rewritten. A CLI test renames an issue a Note refs by path and
`capcom check` then exits 0.
**Approach (assumed at the header base):** `move()` and `unreadWarnings` in
`internal/cli/node_verbs.go`; `issueMvHelp`/`noteMvHelp` in `internal/cli/mv.go`; regenerate.
**Regression guard.** same rule as 6 for every path mv/know mv prints.
A mv-only result (never `writeResult`/`noteWriteResult`, which other writes share) names the
rewritten Notes by id: text `refs_rewritten: [<note ids>]`, JSON `refs_rewritten`, KV
`REFS_REWRITTEN=` before `STATUS=`; both "It prints the ... new id, path and etag" sentences say so.
An unparseable file gets one warning naming only the keys left unchecked (superseded-by on a Note
rename only, refs on every move), e.g. `<path>: superseded-by and refs not checked for release-flow:
the file does not parse`; `know_mv_test.go:338` is updated on purpose to that wording.
**Files:** internal/cli/node_verbs.go, internal/cli/mv.go, internal/cli/mv_test.go, internal/cli/know_mv_test.go, docs/site/reference/file-format.md, docs/site/concepts/knowledge.md, docs/site/reference/cli/capcom_mv.md, docs/site/reference/cli/capcom_know_mv.md
**Read first:** internal/cli/node_verbs.go — move, unreadWarnings, issueVerbs, noteVerbs; internal/cli/mv.go — issueMvHelp, noteMvHelp, movedWarnings, movedPath; internal/cli/node_edit.go — writeResult;
internal/cli/know_add.go — noteWriteResult; internal/cli/rm.go — issueRemoveResult; internal/cli/mv_test.go — newMoveStore, TestMv_UnparseableBystander_WarnsDepsAndRelatedNotChecked;
internal/cli/know_mv_test.go — TestKnowMv_UnparseableBystander_WarnsAndMoves
**Closes:** mv-stale-note-refs
**Depends on:** item 6
**Tests:** `mv beta-new beta-final` with a Note `refs: [.capcom/issues/beta-new.md]`, then `check`
exit 0; `mv --parent`; `know mv`; `refs_rewritten` in text, `--json` and `--kv`; one unread warning
per file; `TestKnowMv_UnparseableBystander_WarnsAndMoves` updated.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Mv|Move'` then `./internal/clidoc/`
**Commit:** `feat(mv): report rewritten Note Refs`

## 8. An edit warns when it drops frontmatter comments — ✅ DONE (2026-10-27)

NOTES (2026-10-27): carrier for item 9 — nodestore.Effects.DroppedComments of type nodestore.CommentDrops, a newline-separated string of worktree-relative paths (Paths() lists them, the unexported add appends one), so Effects stays comparable with ==; app.warnDroppedComments in internal/cli/node_edit.go renders one warning per path.

**What:**
**Goal:** every write through `Kind.Edit` (update, close, log, reopen, dep, related, start's claim,
know update) of a file whose frontmatter holds a YAML comment adds the warning
`<path>: frontmatter comments are not kept; capcom writes frontmatter in canonical form`, in text,
`--json` and `--kv`; `capcom update --help` says comments in frontmatter are not kept.
**Approach (assumed at the header base):** `frontmatter.HasComments(data []byte) bool` in
`internal/frontmatter` walks the parsed `yaml.Node` for Head/Line/Foot comments. `Kind.Edit`
(`internal/nodestore/edit.go`) checks the original bytes and records the path in the
comparable carrier the Regression guard names. `app.written` (`internal/cli/node_edit.go`) renders it beside `warnRaisedFormat`.
`updateLong` gains one sentence; regenerate.
**Regression guard.** dropped-comment paths travel in one carrier chosen in item 8 that keeps nodestore's Effects comparable with == (tests compare Effects with ==), and item 9 reuses that carrier.
**Files:** internal/frontmatter/decode.go, internal/frontmatter/decode_test.go, internal/nodestore/edit.go, internal/nodestore/kind.go, internal/nodestore/edit_test.go, internal/cli/node_edit.go, internal/cli/update.go, internal/cli/close_test.go, docs/site/reference/file-format.md, docs/site/reference/cli/capcom_update.md
**Read first:** internal/nodestore/edit.go — Kind.Edit; internal/nodestore/kind.go — Effects, Written; internal/frontmatter/decode.go — decoder.document, Decode; internal/frontmatter/frontmatter.go — Split;
internal/cli/node_edit.go — app.written; internal/cli/related.go — app.warnRaisedFormat; internal/cli/root.go — app.emit; internal/nodestore/sync_test.go — TestWithSync (Effects{} comparisons)
Part of issue `write-strips-frontmatter-comments`; the later item closes it.
**Depends on:** item 6 (kind.go)
**Tests:** an issue with `# why: customer outage` above `status:`, `close a --reason fixed`, the
warning present and exit 0; no warning for a comment-free file; a `#` inside a quoted value is no comment.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/frontmatter/` then `./internal/nodestore/` then `./internal/cli/ -run 'Close|Update|Comment'` then `./internal/clidoc/`
**Commit:** `fix(write): warn when a write drops frontmatter comments`

## 9. mv and rm warn for the comments their rewrites drop — ✅ DONE (2026-10-27)

NOTES (2026-10-27): the comment check runs once over each plan's finished rewrite set (refs.go commentedPaths, called from planNodeMove and planNodeRemoval) rather than separately in planRefRewrite, planRefDrop and the path-refs pass, so a file two passes rewrite is checked once against the bytes first read; the paths travel in item 8's Effects.DroppedComments carrier, moved paths mapped through movedPaths.
NOTES (2026-10-27): the CLI rm test lives in internal/cli/rm_test.go, beside the other rm tests, a file the plan's Files line did not name.
NOTES (2026-10-27): consequential edit — docs/site/reference/file-format.md: made necessary by mv/know mv/rm/know rm now dropping comments with the warning, which the paragraph's command list named only for edits.

**What:**
**Goal:** the deps, related, superseded-by and Ref rewrites `mv`, `know mv`, `rm` and `know rm`
make in other files add the same dropped-comments warning for each rewritten file that held one.
**Approach (assumed at the header base):** `planRefRewrite`, `planRefDrop` and item 6's path-refs
pass (`internal/nodestore/refs.go`) record rewritten paths whose original held comments via
`frontmatter.HasComments`; `Move`/`Remove` carry them in item 8's carrier; `move()`/`remove()` in
`internal/cli/node_verbs.go` render them.
**Regression guard.** same rule as 6 for the dropped-comment warning paths.
The moved node's own rewrite (a rename's alias and reference-field rewrite, `refs.go:91-94`, and
item 6's own-Refs rewrite) is in the comment check too, named by its new path. The paths travel in
item 8's carrier, never a slice on `Effects`.
**Files:** internal/nodestore/refs.go, internal/nodestore/move.go, internal/nodestore/remove.go, internal/nodestore/move_refs_test.go, internal/cli/node_verbs.go, internal/cli/mv_test.go
**Read first:** internal/nodestore/refs.go — planNodeMove, planRefRewrite, planNodeRemoval, planRefDrop; internal/nodestore/move.go — Kind.Move, Moved; internal/nodestore/remove.go — Removed, Kind.Remove;
internal/cli/node_verbs.go — move, remove; internal/cli/mv.go — movedWarnings, movedPath; internal/nodestore/move_test.go — movedEqual; internal/nodestore/sync_test.go
**Closes:** write-strips-frontmatter-comments
**Depends on:** items 7, 8
**Tests:** mv of an issue whose dependent carries a comment warns naming the dependent's path; a
commented child dependent inside the moved subtree is named at its post-move path; a rename of a
commented issue warns naming its new path.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodestore/` then `./internal/cli/ -run 'Mv|Rm|Comment'`
**Commit:** `fix(mv): warn when a rewrite drops another file's frontmatter comments`

## 10. A busy index fails with a clear message — ✅ DONE (2026-10-27)

NOTES (2026-10-27): the busy failure carries no hint of its own, so the `invalid` code's default hint (`capcom <command> --help`) still follows it in text and `--json`; the CLI test asserts only that the hint is not `capcom reindex`.
NOTES (2026-10-27): the busy classification wraps every failure of `refresh` (its deferred wrapper is registered before `BeginTx`, so the begin failure is covered) and the `Ping` in `openFile`, so `Index.Refresh` callers (start, finish, the UI) get `ErrBusy` too; `dsnOptions` became a variable built by `connectionOptions`, the busy-timeout seam the index tests shorten to 100 ms.

**What:**
**Goal:** a command whose index open or refresh meets SQLite BUSY or LOCKED past the busy timeout
fails `invalid` with `<index path>: the index is busy: another capcom command, such as capcom
reindex, is rebuilding or writing it; retry when it finishes`, in text and `--json`; never the raw
SQLITE_BUSY text alone, and never deleted as corrupt.
**Approach (assumed at the header base):** `ErrBusy` and a `busyError` classifier beside
`ErrCorrupt`/`corruptError` in `internal/index/index.go` (primary codes `SQLITE_BUSY`,
`SQLITE_LOCKED`), wrapped where `openFile`/`refresh` (`internal/index/refresh.go`, `BeginTx`) fail.
`prepareIndex` (`internal/cli/root.go`) maps it beside `ErrCorrupt`, without the `capcom reindex`
hint. A package-level busy-timeout seam builds the DSN so tests wait ~100 ms, and
`TestOpen_BusyIndex_IsNotCorruptAndIsNeverDeleted` uses it too.
**Regression guard.** prime's `readWork` (`internal/briefing/briefing.go:133`) omits "; run capcom reindex" when
`errors.Is(err, index.ErrBusy)` and keeps it for other failures (`briefing_test.go:209`). The CLI
test pays the 5 s busy wait (no exported seam); the index seam is swapped only in non-parallel tests
with `t.Cleanup` restore, never package-wide (`TestRefresh_Concurrent...` is `t.Parallel()`). A
`dsnOptions` identifier stays for `deleteOnFirstEntryRead` (`query_test.go:68`) to concatenate.
**Files:** internal/index/index.go, internal/index/refresh.go, internal/index/index_test.go, internal/cli/root.go, internal/cli/reindex_test.go, internal/briefing/briefing.go
**Read first:** internal/index/index.go — Open, openFile, isCorruptIndex, corruptError, dsnOptions; internal/index/refresh.go — refresh, databaseError; internal/cli/root.go — prepareIndex, reindexHint;
internal/briefing/briefing.go — readWork; internal/index/index_test.go — TestOpen_BusyIndex_IsNotCorruptAndIsNeverDeleted, TestRefresh_ConcurrentIndexesAndAnEdit_NeitherFailsAndBothSeeTheSameRows, buildIndex;
internal/index/query_test.go — deleteOnFirstEntryRead; internal/cli/reindex_test.go — TestReindex_CorruptIndex_ListFailsWithReindexHintAndReindexRepairsIt, newIssueStore
**Closes:** index-busy-timeout-scale
**Tests:** index: held `BEGIN EXCLUSIVE` → `errors.Is(err, ErrBusy)`, not `ErrCorrupt`, file kept.
CLI: hold the lock on the index, `runCLI list --json` → code `invalid`, message names `capcom reindex`;
`list` in text prints the busy message and no `run capcom reindex` hint line.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/` then `./internal/cli/ -run 'Reindex|Busy'`
**Commit:** `fix(index): name the rebuild when a command meets a busy index`

## 11. list reads its rows in a fixed number of queries — ✅ DONE (2026-10-27)

NOTES (2026-10-27): readIssue (Issue, and so Resolve) now goes through the same batch loader with one id, so single and batch reads share one scan path; Resolve was already a fixed count.

**What:**
**Goal:** `index.List`, the issue search funcs and `Resolve` load N entries in a constant number of
statements (one row query plus one per list field), whatever N; `BenchmarkList` in
`internal/index` runs over a generated store of about 3000 issues.
**Approach (assumed at the header base):** replace the per-id `readIssue` loop in `issues(q, ids)`
(`internal/index/query.go`) with a batch loader: ids passed as one JSON array,
`IN (SELECT value FROM json_each(?))` (no bound-variable limit), list fields grouped in Go, the
caller's id order kept, a missing id still `tree.ErrNotFound`. `beforeEntryRead` still fires once
between reading the ids and the batch. Test helpers `newStore`/`writeIssue`/`openIndex` take
`testing.TB`.
**Regression guard.** A test passes `x.issues` a counting `querier` wrapper and asserts the same statement count (one
row query plus the four list queries: labels, deps, related, aliases) for one id and for many ids;
it fails against the per-id `readIssue` loop.
**Files:** internal/index/index.go, internal/index/query.go, internal/index/index_test.go, internal/index/query_test.go, internal/index/bench_test.go
**Read first:** internal/index/query.go — issues, resolved, List, searchIssues, Resolve, beforeEntryRead; internal/index/index.go — readIssue, snapshot, strings, querier;
internal/index/query_test.go — deleteOnFirstEntryRead, TestList_RowsDeletedMidRead_ReturnsOneSnapshot, listFixture; internal/index/index_test.go — newStore, writeIssue, openIndex
Part of issue `list-per-row-queries`; the later item closes it.
**Depends on:** item 10 (same files)
**Tests:** existing query tests, `TestList_RowsDeletedMidRead_ReturnsOneSnapshot`; order and
list-field equality between batch and single reads; the counting-`querier` statement count.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/`
- `GOMEMLIMIT=2GiB go test -run '^$' -bench BenchmarkList -benchtime 3x ./internal/index/`
**Commit:** `perf(index): load list rows in a fixed number of queries`

## 12. search --excerpt reads its bodies in one query — ✅ DONE (2026-10-27)

NOTES (2026-10-27): item 11 left no generated-store helper (its store was built inline in BenchmarkList), so that setup moved into `benchIndex` in internal/index/bench_test.go, which BenchmarkList and BenchmarkSearchExcerpt both call before b.ResetTimer.
NOTES (2026-10-27): `bodies` reuses `pathValues` for its one query; `Index.Body` now has no production caller (only tests), kept as the plan says.

**What:**
**Goal:** `index.Bodies(paths []string) (map[string]string, error)` reads every hit's body in one
statement; `searchExcerpts` (shared by `search` and `know search`) calls it once;
`BenchmarkSearchExcerpt` in `internal/index` covers it.
**Approach (assumed at the header base):** `Bodies` beside `Body` in `internal/index/index.go` with
`json_each`; `searchExcerpts` in `internal/cli/search.go`.
**Regression guard.** BenchmarkSearchExcerpt reuses item 11's generated-store helper in internal/index/bench_test.go, building the store once per benchmark before b.ResetTimer.
`Bodies` delegates to `bodies(q querier, paths)`, and a test asserts one statement through item
11's counting querier; the benchmark covers `Bodies` plus `Excerpt` (`internal/index` cannot call
cli's unexported `searchExcerpts`). `Body` stays: `excerpt_test.go:115` uses it.
**Files:** internal/index/index.go, internal/index/index_test.go, internal/index/bench_test.go, internal/cli/search.go
**Read first:** internal/cli/search.go — searchExcerpts, newSearchCommand; internal/cli/know_search.go — newKnowSearchCommand (second searchExcerpts caller); internal/index/index.go — Body, querier, snapshot;
internal/index/excerpt_test.go — TestBody_IndexedIssueAndNote_ReturnsTheirBodies; internal/index/schema.go — fts (path UNINDEXED: no index, so `path IN (json_each)` scans fts once)
**Closes:** list-per-row-queries
**Depends on:** items 2, 11
**Tests:** `Bodies` returns each body and omits unknown paths; one statement through the counting
`querier`; CLI excerpt tests unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/` then `./internal/cli/ -run 'Search'`
- `GOMEMLIMIT=2GiB go test -run '^$' -bench BenchmarkSearchExcerpt -benchtime 3x ./internal/index/`
**Commit:** `perf(search): read excerpt bodies in one query`

## 13. finish --keep-lock names the start that resumes the lock — ✅ DONE (2026-10-27)

**What:**
**Goal:** `finish <id> --keep-lock` prints the session's token when it holds the lock, and its next
line reads `next: <next>  <title>; capcom start <next> --token <token>` (KV gains `TOKEN=`);
running that start exits 0 and leaves one token file, for `<next>`. With no token of this session
(lock held by another purpose or session) the plain next line stays. A bare `start <other>`
refused `locked` has hint `capcom start <other> --token <token>, or capcom lock status`.
**Approach (assumed at the header base):** in `finishLock` (`internal/cli/finish.go`), the
keep-lock branch gets the token via `worktree.SessionToken(token, id)` then `worktree.Check(t)`;
never from `Record.Token` (item 14 removes it). `finishResult` builds the line.
`startLockFailure` (`internal/cli/start.go`) takes the id. `finishLong`/`startLong`, regenerate.
**Regression guard.** A kept token that cannot be read leaves keep-lock's note "kept (--keep-lock)" with the plain next
line and no warning (the "keep lock" row of `TestFinish_UnreadableKeptToken_...` stays). `TOKEN=` is
written only under `--keep-lock` when a session token holds the lock (`TestFinish_KV_GivesOneRecord`
unchanged). The note stays on the lock line and `token <token>` follows it; `finish --json` carries
`lock.token` under keep-lock and `finishData`'s "never with a token" comment says so.
**Files:** internal/cli/finish.go, internal/cli/start.go, internal/cli/finish_test.go, internal/cli/start_test.go, docs/site/concepts/tree-locks.md, .capcom/knowledge/issue-workflow.md, docs/site/reference/cli/capcom_finish.md, docs/site/reference/cli/capcom_start.md
**Read first:** internal/cli/finish.go — finishLock, finishResult, finishLockOutcome, unreadableKeptToken, finishLong; internal/cli/start.go — startLockFailure, refuseLock, start;
internal/cli/finish_test.go — TestFinish_KeepLock_LeavesTheLockHeld, TestFinish_UnreadableKeptToken_LeavesTheLockAndSaysSo, TestFinish_KV_GivesOneRecord, startToken;
internal/cli/start_test.go — TestStart_RefusedIssue_WritesNothingAndNamesTheResolver, lockRecordPath; internal/cli/lock.go — lockText, newLockData, keptTokenFiles (lock_test.go)
**Closes:** keep-lock-blocks-next
**Tests:** finish `--keep-lock`, run the start its next line names → exit 0, one token file; bare
`start other` → exit 3, hint names `--token`; the `start_test.go` row "lock held by another" is
updated to `capcom start login-flow --token <token>, or capcom lock status`; keep-lock text and
`--json` show the token after the kept note.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Finish|Start'` then `./internal/clidoc/`
- `go run ./cmd/capcom check`
**Commit:** `fix(finish): name the start that resumes a kept lock`

## 14. The lock record holds a token digest, not the token — ✅ DONE (2026-10-27)

**What:**
**Goal:** a lock record written by capcom holds `"token": "sha256:<hex>"`, never the cleartext;
acquire, resume, touch, release and check match a presented token by digest (constant-time); a
legacy cleartext record still matches and is rewritten as a digest on touch or resume; another
user's `lock status` still names the holder.
**Approach (assumed at the header base):** one `tokenMatches(stored, presented)` and a digest func
in `internal/lock/lock.go`, used by `Acquire`, `Touch`, `Release`, `Check`. `Acquire` returns the
cleartext in a new `Status.Token`; `internal/cli/lock.go` (acquire) and `internal/cli/start.go`
switch from `Record.Token` to it. Token files stay cleartext at 0600.
**Regression guard.** `docs/site/reference/file-format.md` and the item's CHANGELOG sidecar entry say that an older
capcom in the same clone compares the cleartext, so it sees a record written by this one as held by
another session, and that `capcom lock break` clears it.
**Files:** internal/lock/lock.go, internal/lock/lock_test.go, internal/lock/token_test.go, internal/lock/worktree_test.go, internal/cli/lock.go, internal/cli/start.go, internal/cli/lock_test.go, docs/site/reference/file-format.md
**Read first:** internal/lock/lock.go — Acquire, Touch, Release, Check, read, Record, Status; internal/lock/token.go — removeTokenFiles, SessionToken, writeTokenFile; internal/cli/lock.go — newLockAcquireCommand, emitLock;
internal/cli/start.go — start, claimForStart, emitStarted; internal/lock/worktree_test.go — TestWorktree_AcquireThenTouch_WritesTheRecordAtRecordPath; internal/lock/lock_test.go — openTest, mustAcquire, writeRecord;
docs/site/reference/file-format.md — lock record token bullet
**Closes:** lock-record-token-readable
**Depends on:** item 13
**Tests:** the record JSON does not contain the token acquire returned; a legacy cleartext record
resumes and is rewritten; a wrong token is refused.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/lock/` then `./internal/cli/ -run 'Lock|Start|Finish'`
**Commit:** `fix(lock): keep only a digest of the token in the record`

## 15. A lock record from a removed or re-created worktree is orphaned — ✅ DONE (2026-10-27)

NOTES (2026-10-27): the orphan judgement and the worktree-id helpers (linkedGitDir, isOrphaned, ensureWorktreeID, readWorktreeID) live in internal/lock/worktree.go; Locks gains a commonDir field so read() derives `<common>/worktrees/<name>` from the key; Acquire was split into issueToken and keepToken to stay small, and newToken now draws its bits from a shared randomHex.
NOTES (2026-10-27): an orphaned Status keeps the orphaned Record (IsHeld false, IsOrphaned true) so item 16 can report it; every CLI consumer already gates Record on IsHeld. Release and Touch treat an orphan as free (Touch fails not-held; Release leaves the record); Break removes it as before.
NOTES (2026-10-27): internal/lock/lock_test.go needed no change; the docs for the `worktree` record key and `worktree-id` file are left to item 16, whose Files name docs/site/reference/file-format.md and .capcom/knowledge/store-layout.md.
NOTES (2026-10-27): gofmt re-aligned the `name`/`ttl` fields of the "free beside a held lock" case in internal/lock/overview_test.go after its arrange became multi-line.

**What:**
**Goal:** a `wt-` record whose admin dir `<common>/worktrees/<name>` is gone, or whose `worktree`
id differs from `<gitDir>/capcom/worktree-id` (missing marker included), reads as free with
`Status.IsOrphaned`; `Check` treats it as free; `Acquire` takes it over with a new token and drops
its token files. Legacy records without `worktree` are orphaned only when the admin dir is missing;
`_main` is never orphaned.
**Approach (assumed at the header base):** `Acquire` on a `wt-` key writes a random id to
`<gitDir>/capcom/worktree-id` when missing; `Record` gains `Worktree string` (`json:"worktree,omitempty"`).
`read(key)` in `internal/lock/lock.go` decides orphaning; `Worktree` (`internal/lock/worktree.go`)
supplies the git dir. `State()` stays `StateFree`.
**Regression guard.** Orphaning is judged only after a record reads and parses with a token; one that cannot be read or
holds no lock stays invalid or unreadable. `_main` is never orphaned: its git dir is the common dir.
`Acquire` never `MkdirAll`s a missing admin dir; lock tests get a helper creating
`<common>/worktrees/<name>` for `foreignKey`. The CLI re-add is `runGit` `worktree add -q <path>
branch-<name>` then `mustRun` init, not a second `addWorktree`.
**Files:** internal/lock/lock.go, internal/lock/worktree.go, internal/lock/lock_test.go, internal/lock/worktree_test.go, internal/cli/lock_test.go, internal/lock/overview_test.go, internal/lock/token_test.go
**Read first:** internal/lock/lock.go — read, Acquire, Check, Status, Record, WorktreeKey; internal/lock/worktree.go — ForWorktree, Worktree; internal/lock/overview.go — Overview;
internal/lock/overview_test.go — foreignKey, TestOverview_TwoWorktrees_HoldIndependentLocks, TestOverview_DirectoryAtAForeignGuardBesideAHeldLock_ListsBothLocks;
internal/cli/lock_test.go — addWorktree, runGit, writeLockRecord, TestLockStatus_ForeignInvalidRecord_PrintsNoWarning; internal/cli/prime_test.go — TestPrime_LockHeldInMainWorktree_AdvisesALinkedWorktreeItIsFree
Part of issue `lock-orphan-worktree-record`; the later item closes it.
**Depends on:** item 14
**Tests:** CLI: acquire in a linked worktree (`addWorktree`), `git worktree remove --force`, add
again at the same path, `lock acquire` there exits 0. Lock: marker mismatch and missing admin dir; `{`/`{}` records with no admin dir stay invalid;
`_main` never orphaned.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/lock/` then `./internal/cli/ -run 'Lock'`
**Commit:** `fix(lock): treat a removed or re-created worktree's record as orphaned`

## 16. status and prime report orphaned lock records — ✅ DONE (2026-10-27)

NOTES (2026-10-27): consequential edit — docs/site/agents/briefing.md: made necessary by prime listing orphaned records (its `locks` JSON row said "one object per held lock" and its locks bullet did not name orphans).
NOTES (2026-10-27): Overview gains `Orphans []Status` (ordered by key, apart from Locks); an orphaned own record leaves State free and sets Own to the orphaned Status. lockData gains `orphaned` (omitempty); --kv carries no orphan marker, an orphan's record is LOCK=free.
NOTES (2026-10-27): `lock status --all` lists orphans after the held locks; with none held it still prints "no locks held" first, and --kv writes the lone STATUS line only when there is neither a lock nor an orphan. prime's JSON `locks` array carries the orphan objects after the held ones (orphaned: true), mirroring the text, while the text count stays the held count.
NOTES (2026-10-27): file-format.md also gains the `worktree` record key and the `worktree-id` file, which item 15 left to this item.

**What:**
**Goal:** `lock status`, `lock status --all` and `prime` show an orphaned record as
`free (orphaned record of a removed worktree)`, never counted under `locks:`; `--json` carries
`"orphaned": true`. Docs say a deleted `<gitDir>/capcom` orphans a live lock.
**Approach (assumed at the header base):** `lockStateName`/`allLocksResult` in
`internal/cli/lock.go`, `Overview` (`internal/lock`), `addLocks`/`worktreeLockAdvice` in
`internal/cli/prime.go`; `lockStatusLong`, regenerate.
**Regression guard.** orphaned records are listed in Overview apart from Locks, so prime's "locks:" count is unchanged.
The docs clause covers every sentence that calls `<git-dir>/capcom` disposable or safe to delete
(`file-format.md:9` "deleting them loses nothing", `.capcom/knowledge/store-layout.md:9`), found with
`grep -rn 'disposable\|loses nothing\|git-dir>/capcom' docs/site .capcom/knowledge`; for
`worktree-id` those sentences are superseded.
**Files:** internal/lock/overview.go, internal/lock/overview_test.go, internal/cli/lock.go, internal/cli/prime.go, internal/cli/lock_test.go, docs/site/concepts/tree-locks.md, docs/site/reference/file-format.md, docs/site/reference/cli/capcom_lock_status.md, .capcom/knowledge/store-layout.md
**Read first:** internal/lock/overview.go — Overview, Status.State; internal/cli/lock.go — ownLockResult, allLocksResult, lockData, lockText, lockStatusLong; internal/cli/prime.go — addLocks, worktreeLockAdvice, primeData;
internal/briefing/briefing.go — readLocks; internal/ui/statusbar.go — lockBadgeOf, lockText; docs/site/reference/file-format.md — intro, lock record keys
**Closes:** lock-orphan-worktree-record
**Depends on:** item 15
**Tests:** after remove/re-add, `lock status --all` text and `--json` show the orphan; `prime`'s
`locks:` excludes it.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/lock/` then `./internal/cli/ -run 'Lock|Prime'` then `./internal/clidoc/`
- `go run ./cmd/capcom check`
**Commit:** `feat(lock): report orphaned lock records`

## 17. A CLI write warns while another session holds the lock

**What:**
**Goal:** per ADR 0017, a CLI write while this worktree's lock is held under a purpose other than
the written Issue's id adds one warning naming holder, purpose and heartbeat age (stale marked),
still exit 0; an unreadable or invalid record warns and names `capcom lock status`. Warning
commands: create, update, log, close, reopen, dep add/rm, related add/rm, mv, rm, know add, know
update, know mv, know rm, fmt and agents without `--check`. Never: start, finish, lock verbs, ui,
init, reindex, merge-driver, read-only commands, a failed write. No file is created under
`.git/capcom/locks/`.
**Approach (assumed at the header base):** `app.noteWrite(ids ...string)` called from `written`
(`internal/cli/node_edit.go`) and the bypass sites: `createNode`/`move`/`remove`
(`internal/cli/node_verbs.go`, mv passes old and new id), related, `editNode`, fmt, agents.
`app.emit` (`internal/cli/root.go`) runs `lock.ForWorktree(a.store).Check("")` once when a write
was noted and appends to `writeWarnings`, reusing `HeldError.Error()` plus ADR 0017's advice. Not in `internal/ops`
(the UI asks already).
**Regression guard.** finish stays silent after item 13's keep-lock change: its exemption is an app flag set by finish, never derived from the token or purpose.
The warning reads `<record path>: locked by ... ; if it is yours, carry on` (`HeldError.Error()`
plus ADR 0017's advice), pinned in `lock_warn_test.go`. Note writes, `fmt` and `agents` note a write
with no Issue id (always foreign). Rows: finish under a foreign lock and `--keep-lock` under another
purpose silent; `start` silent; a Note whose id is the purpose warns.
**Files:** internal/cli/root.go, internal/cli/node_edit.go, internal/cli/node_verbs.go, internal/cli/related.go, internal/cli/fmt.go, internal/cli/agents.go, internal/cli/lock_warn_test.go, internal/cli/finish.go
**Read first:** internal/cli/node_edit.go — app.written, editIssue, editNode; internal/cli/root.go — app.emit, app.writeWarnings; internal/cli/node_verbs.go — createNode, move, remove; internal/cli/finish.go — finish, finishLock;
internal/cli/start.go — claimForStart, start; internal/cli/close.go — closeOne; internal/lock/lock.go — Locks.Check, HeldError.Message, ErrCorruptRecord;
internal/cli/lock_test.go — acquireToken, writeLockRecord, newLockStore
Part of issue `lock-warn-on-write`; the later item closes it.
**Depends on:** items 13, 16
**Tests:** table over every warning command under a foreign lock (`acquireToken`): exit 0, file
written, warning; `log` on the purpose's Issue silent (lock taken with `start <id>` or
`lock acquire --purpose <id>`, not `acquireToken`, whose purpose is "item 12"); the finish, start
and Note-id rows of the guard; free lock silent; `writeLockRecord` invalid
record warns; no lock dir created on a fresh repo.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'LockWarn|Lock'`
**Commit:** `feat(lock): warn when a CLI write meets another session's lock`

## 18. The managed block and docs say the lock is signalled, not enforced

**What:**
**Goal:** the agents managed block, the repo's `AGENTS.md`, `docs/site/agents/managed-block.md`,
`docs/site/concepts/tree-locks.md` and the `issue-workflow` Note say a CLI write warns while
another session holds the lock and is never refused.
**Approach (assumed at the header base):** `lockClosing` in `internal/agents/block.go` and its
golden copies in `internal/agents/block_test.go`; regenerate `AGENTS.md` with
`go run ./cmd/capcom agents`.
**Regression guard.** The docs rule is every page or skill that teaches the Tree lock to an agent or lists what reads the
record (`grep -rn "Tree lock\|lock check\|Reading the lock" docs/site plugins README.md
.capcom/knowledge`), `plugins/capcom/skills/capcom/SKILL.md` "4. Tree lock" and file-format.md's
lock readers included. The CHANGELOG sidecar entry carries "An existing block reads as out of date
after upgrading: run `capcom agents` once ..." as `CHANGELOG.md:75` does.
**Files:** internal/agents/block.go, internal/agents/block_test.go, AGENTS.md, docs/site/agents/managed-block.md, docs/site/concepts/tree-locks.md, .capcom/knowledge/issue-workflow.md, plugins/capcom/skills/capcom/SKILL.md, docs/site/reference/file-format.md
**Read first:** internal/agents/block.go — lockClosing, render; internal/agents/block_test.go — goldenBlock, preContextBlock, keepTheTokenBlock, TestInspect_States (keep the old goldenBlock as a new historical stale input; preContextBlock is a past render and must not change);
docs/site/agents/managed-block.md; docs/site/concepts/tree-locks.md — Tree lock; plugins/capcom/skills/capcom/SKILL.md — 4. Tree lock; docs/site/reference/file-format.md — The lock record; .capcom/knowledge/issue-workflow.md
**Closes:** lock-warn-on-write
**Depends on:** item 17
**Tests:** block golden tests.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/agents/`
- `go run ./cmd/capcom agents --check` and `go run ./cmd/capcom check`
**Commit:** `docs(lock): say a CLI write warns under another session's lock`
