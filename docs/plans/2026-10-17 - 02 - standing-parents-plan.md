# A standing Parent is never offered as ready work

**Goal:** an Issue can carry `standing: true`, the mark of a standing Parent that only groups
children. A standing Issue is never Ready, never claimed or closed while marked, and `check`
reports one that is closed or doing; the docs and the managed block say so.
**Date:** 2026-10-17
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** a875d33
**Closes:** standing-parents

**Sources:**
- `.capcom/issues/standing-parents.md`
- Notes `adr-0003`, `adr-0009`, `adr-0012`, `error-messages`, `cli-test-harness`
- `internal/knowledge/note.go` (`retired`, the boolean key to imitate)

**Ratified design calls** (user, 2026-10-17):
- **Mark:** a frontmatter boolean `standing: true`, set by `create --standing` and `update --standing` / `--no-standing`; modelled on a Note's `retired`; the new key raises the store format by one.
- **Override:** no force flag. The refusal names the mark and says to run `capcom update <id> --no-standing` first; `update <id> --no-standing --status <doing>` in one call works.
- **Marking:** `--standing` on a closed Issue or one in a doing status is refused.
- **ui:** the detail pane shows the mark; claim, status and close on a standing Issue show the refusal as a notice; no ui toggle.

**Standing requirements:**
- skills: coding-standards
- Tests per `AGENTS.md`: single packages with `GOMEMLIMIT=2GiB`, never `./...` with `-race`, never two test commands at once.
- Any authorized deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- A ui key to set or clear the mark.
- Migrating `ready.hide` labels such as `container` to the mark.
- Any rule tied to an Issue `type`.

**Regression check (2026-10-17, a875d33):**
- 1: guard folded
- 2: guard folded
- 4: guard folded
- 5: guard folded (round 2: `issueFromFile` copies `IsStanding`; show.go added to Files)
- 6: guard folded
- 7: recast; guard folded (round 2: Tests JSON clause reworded; `issueFromFile` copy moved to item 5; standing fixtures declare the store format)
- 8: guard folded
- 9: guard folded
- 10: guard folded
- 11: guard folded

## 1. The `standing` key in the Issue file and the store format — ✅ DONE (2026-10-17)

NOTES (2026-10-17): the descriptor's format row names the key through a new unexported const `keyStanding` in internal/nodestore/descriptor.go (beside `keyRetired`) instead of the literal "standing" the approach shows, per the no-magic-strings rule.
NOTES (2026-10-17): DefaultFile keeps a literal (`format: 5`); TestParse_DefaultFile_DeclaresCurrentFormat already pins it to FormatVersion, and a new TestFormatVersion_IsTheNewestNamedFormat pins FormatVersion to FormatStanding.
NOTES (2026-10-17): docs/site/reference/file-format.md and docs/site/agents/managed-block.md still show `format: 4` and list formats up to 4; left for item 11, whose Files name both.

**What:**
**Goal:** an Issue file may carry `standing: true`; it parses into `Issue.IsStanding`, round-trips byte for byte, is written only when true, and its presence needs the new store format, which `config.FormatVersion` names.
**Approach (assumed at the header base):** add `keyStanding` to the key constants in `internal/issue/issue.go` after `plan` (before `aliases`), an `IsStanding bool` field and a `StandingLine()` accessor beside `PlanLine`. Add a `frontmatter.Raw` field to `issueSchema` in `internal/issue/file.go` whose decoder accepts `true`, `false` or empty and whose encoder writes only `true`, as `retired` does in `internal/knowledge/note.go`; update the key order in the `Serialize` doc comment. Raise `FormatVersion` in `internal/config/config.go` by one with a named constant `FormatStanding` (update its doc comment), and add `{Key: "standing", Format: config.FormatStanding, Line: issue.Issue.StandingLine}` to `IssueDescriptor` `Formats` in `internal/nodestore/descriptor.go`, as `related` does.
**Regression guard.** `DefaultFile`'s `\nformat: 4\n` literal becomes the new format (or is built from `FormatVersion`), and `TestParse_NewerFormat_FailsAsTooNewWithLine` moves to format 6 / "up to 5"; `internal/cli/read_test.go` derives the "newer" format as `config.FormatVersion+1` and "up to %d" from `FormatVersion` (`prime_test.go` shares the const). The unknown-key hint gains `standing` after `plan`: update every pin (`grep -rn 'an issue has' internal`). The issue fixture in `internal/nodestore/descriptor_test.go` carries a `standing` line and expects its format row.
**Files:** internal/issue/issue.go, internal/issue/file.go, internal/issue/file_test.go, internal/issue/testdata/standing.md, internal/config/config.go, internal/config/config_test.go, internal/nodestore/descriptor.go, internal/nodestore/create_test.go, internal/nodestore/descriptor_test.go, internal/nodestore/read_test.go, internal/index/index_test.go, internal/check/check_test.go, internal/cli/read_test.go
**Read first:** internal/issue/file.go — issueSchema, Serialize; internal/knowledge/note.go — retired; internal/issue/issue.go — PlanLine; internal/config/config.go — FormatVersion, DefaultFile; internal/nodestore/descriptor.go — IssueDescriptor; internal/cli/read_test.go — newerFormatMessage
**Tests:** round trip of `testdata/standing.md`; `standing: false` parses as not standing and is dropped on write; `standing: maybe` is a parse error naming the line; the format constant test; a create/edit writing `standing` raises the store format; `TestDescriptor_PerKind_DescribesNounRefKeysAndFormats` expects the `standing` row; the newer-format tests take their numbers from `FormatVersion`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/issue/ ./internal/config/ ./internal/nodestore/ ./internal/index/ ./internal/check/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `go vet ./internal/issue/ ./internal/config/ ./internal/nodestore/`
**Commit:** `feat(issue): add the standing key to the Issue file`

## 2. Status rules refuse a standing Issue — ✅ DONE (2026-10-17)

NOTES (2026-10-17): consequential edit — internal/ops/ops.go: made necessary by the new ErrStanding refusal in issue.SetStatus and issue.Close (doc comments of Ops.SetStatus and Closer.Close name it; no logic change)
NOTES (2026-10-17): SetStanding takes the shape of the file's other setters, SetStanding(i Issue, cfg config.Config, isStanding bool) (Issue, error); in Close the standing check follows both ErrAlreadyClosed and ErrNotClosed

Depends on item 1.

**What:**
**Goal:** the pure Issue rules refuse closing a standing Issue, moving it into a doing status, and marking a closed or doing Issue standing, each with a sentinel error; `ops.Claim`, `ops.SetStatus` and `Closer.Close` pass the refusal on (the CLI and ui paths are items 5, 6 and 8).
**Approach (assumed at the header base):** in `internal/issue/mutate.go` add `ErrStanding` (refused close or doing status on a standing Issue) and `ErrStandingNotOpen` (marking a closed or doing Issue), returned as `*StatusError` like `ErrEntersClosed`. `SetStatus` refuses a target in the doing category when `IsStanding`; `Close` refuses when `IsStanding`; add `SetStanding(is bool, category)` (or the shape the file's other setters use) that refuses `true` when the Issue is closed or in a doing status. Moving a standing Issue between todo-category statuses stays allowed. `ops.Claim`, `ops.SetStatus` and `Closer.Close` in `internal/ops/ops.go` need no new logic; add a test that each wraps the refusal as `*ChangeError` at the file.
**Regression guard.** `SetStanding(true)` refuses only `IsClosed()` or `CategoryDoing` (a waiting Issue may be marked), as the ratified Marking call says. `Close` keeps `ErrAlreadyClosed` first and `SetStatus` keeps its same-status no-op and `ErrLeavesClosed` first; the standing check comes after them. The todo→todo test parses a config declaring a second todo status (`config.Parse` with a statuses map); `config.Default()` declares only `open`.
**Files:** internal/issue/mutate.go, internal/issue/mutate_test.go, internal/ops/ops_test.go
**Read first:** internal/issue/mutate.go — SetStatus, Close, StatusError; internal/ops/ops.go — Claim, Closer.Close; internal/cli/update.go — applyUpdate; internal/issue/mutate_test.go — lifecycleIssue; internal/ops/ops_test.go — TestSetStatus_ChangeRefused_IsAChangeErrorAtTheFile
**Tests:** table tests for each refusal, for marking a waiting Issue, and for the allowed todo→todo move on a config with two todo statuses; `Close` of a standing Issue already `done` is `ErrAlreadyClosed`; ops tests for Claim, SetStatus and Close on a standing Issue returning `*ChangeError` wrapping `ErrStanding`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/issue/ ./internal/ops/`
- `go vet ./internal/issue/ ./internal/ops/`
**Commit:** `feat(issue): refuse closing or claiming a standing Issue`

## 3. The index carries the mark — ✅ DONE (2026-10-17)

Depends on item 1.

**What:**
**Goal:** the SQLite index stores whether each Issue is standing, `index.Entry.IsStanding` reads it back, and `Entry.Node()` sets `issue.Node.IsStanding`; an index built by the previous binary is rebuilt.
**Approach (assumed at the header base):** add `standing INTEGER NOT NULL DEFAULT 0` to the `issues` table in `internal/index/schema.go` and bump `schemaVersion`; write it in `insertIssue` (`refresh.go`), read it in `readIssue` (`index.go`), add `Entry.IsStanding`, and set it in `Entry.Node()` (`query.go`). Add `IsStanding bool` to `issue.Node` in `internal/issue/ready.go` (the field only; the rule is item 4).
**Files:** internal/index/schema.go, internal/index/refresh.go, internal/index/index.go, internal/index/query.go, internal/index/index_test.go, internal/issue/ready.go
**Read first:** internal/index/refresh.go — insertIssue; internal/index/index.go — Entry, readIssue; internal/index/query.go — Entry.Node; internal/index/schema.go — schemaVersion, schemaStatements; internal/issue/ready.go — Node; internal/index/index_test.go — writeIssue
**Tests:** an index over a store with one standing Issue returns `IsStanding` true for it and false for the rest, and `Node()` carries it.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/index/ ./internal/issue/`
- `go vet ./internal/index/ ./internal/issue/`
**Commit:** `feat(index): store the standing mark`

## 4. A standing Issue is never Ready — ✅ DONE (2026-10-17)

NOTES (2026-10-17): `NewlyReady` gets no separate skip: it names only issues `Ready` judges Ready, and `Ready` never judges a standing Issue Ready, so its doc comment says so instead of a redundant check; the unit test covers the closing-last-child case.
NOTES (2026-10-17): until item 7 lands, `capcom ready --all` prints a standing Issue with an empty `-- not ready:` reason, since `notReadyReasons` does not yet know the mark.

Depends on item 3.

**What:**
**Goal:** `issue.Ready` judges a standing Issue not Ready whatever its children, with `Readiness.IsStanding` set; `NewlyReady` never names a standing Issue; its non-closed children keep the usual rules, so `ready`, `prime`, the ui ready list and close's `now ready:` all drop standing Issues.
**Approach (assumed at the header base):** in `internal/issue/ready.go` add `Readiness.IsStanding` and AND `!node.IsStanding` into the `IsReady` expression of `Ready()`; in `internal/issue/unblock.go` `NewlyReady` skips standing nodes; `unblocksByCloser` in `internal/briefing/ready.go` skips standing verdicts (see the guard). `ReadyList` with `IsNotReadyIncluded` keeps standing Issues.
**Regression guard.** `unblocksByCloser` skips `verdict.IsStanding` unconditionally and its "becomes Ready exactly when" doc comment says so, so the last open child of a standing Parent never gets `Unblocks` naming the Parent (no `-- unblocks 1`, no JSON `unblocks`).
**Files:** internal/issue/ready.go, internal/issue/ready_test.go, internal/issue/unblock.go, internal/issue/unblock_test.go, internal/briefing/ready.go, internal/briefing/ready_test.go
**Read first:** internal/issue/ready.go — Ready, Readiness; internal/briefing/ready.go — unblocksByCloser, soleCloser, ReadyList; internal/issue/unblock.go — NewlyReady; internal/cli/ready_test.go — TestReady_All_JSONCarriesWhatAReadyIssueUnblocks, TestReady_Text_ReadyLineEndsWithWhatItUnblocks
**Tests:** a standing Parent with all children closed and one with no children are not Ready and `IsStanding`; an open child of a standing Parent is Ready; closing the last child of a standing Parent does not list the Parent as newly ready; `ReadyList` default omits it and `--all` options keep it; the last open child of a standing Parent has empty `Unblocks`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/issue/ ./internal/briefing/`
- `go vet ./internal/issue/ ./internal/briefing/`
**Commit:** `feat(ready): never count a standing Issue as Ready`

## 5. `create --standing` and `update --standing` / `--no-standing`

Depends on items 2 and 3.

**What:**
**Goal:** `capcom create --standing` writes `standing: true`; `capcom update <id> --standing` sets it and `--no-standing` removes it (the two are mutually exclusive); `update <id> --no-standing --status <doing>` succeeds in one call; `show` text and JSON carry the mark; `--help` of both commands describes it.
**Approach (assumed at the header base):** flag-name constants in `internal/cli/flags.go`; `createOptions` / `newIssue` in `internal/cli/create.go`; in `internal/cli/update.go` declare the pair with `MarkFlagsMutuallyExclusive`, add both to the `hasAnyFlag` list, and in `applyUpdate` apply the standing change through item 2's setter BEFORE the status change, so an unmark-and-claim call is judged on the unmarked Issue (marking and a doing status in one call is refused). Map `ErrStandingNotOpen` in `statusChangeFailure`. Add `standing` to the show field list and `issueData` in `internal/cli/issue_render.go`, in the same JSON form a Note's `retired` uses. Update `createLong`/`createExample`, `updateLong`/`updateExample` and `internal/cli/testdata/help/create.txt`.
**Regression guard.** regenerate docs/site/reference/cli/ with `go run ./cmd/gendocs` in this item, add docs/site/reference/cli/ to Files and `GOMEMLIMIT=2GiB go test ./internal/clidoc/` to Acceptance; show tests live in internal/cli/read_test.go (there is no show_test.go) — fix Files and Tests. In `newIssue`, after the status is chosen, apply item 2's setter to the built Issue and word its refusal with `statusChangeFailure`; the issue create path calls `a.warnRaisedFormat(written.Effects)` (or `issueVerbs` gains an `afterWrite` that does). Every issue JSON output built from `issueData` (list, list --tree, dep list, prime, ready, search, show) carries `standing`, omitted when false. `issueFromFile` in internal/cli/show.go copies `IsStanding` onto the Entry in THIS item, so `show` text and JSON carry `standing`.
**Files:** internal/cli/flags.go, internal/cli/create.go, internal/cli/update.go, internal/cli/issue_render.go, internal/cli/show.go, internal/cli/write_test.go, internal/cli/read_test.go, internal/cli/testdata/help/create.txt, docs/site/reference/cli/
**Read first:** internal/cli/update.go — applyUpdate, statusChangeFailure, hasAnyFlag; internal/cli/create.go — newIssue; internal/cli/node_edit.go — written; internal/cli/issue_render.go — issueData; internal/cli/node_verbs.go — createNode, issueVerbs
**Tests:** through `runCLI`: create with `--standing` then `show --json`; update `--standing` then `--no-standing` round trip; both flags together refused; `--standing` on a closed and a doing Issue refused; `--no-standing --status active` in one call claims it; `create --standing --status active` is refused and writes nothing; `create --standing` in a store below the new format warns with `formatRaisedWarning`; the show tests in `read_test.go`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Create|Update|Show|Help'`
- `GOMEMLIMIT=2GiB go test ./internal/clidoc/`
- `go vet ./internal/cli/`
**Commit:** `feat(cli): mark an Issue standing with create and update`

## 6. `close` and `update --status` refusal messages

Depends on item 5.

**What:**
**Goal:** `capcom close <id>` and `capcom update <id> --status <doing>` on a standing Issue exit non-zero with a message that names the file, says the Issue is standing, and says to run `capcom update <id> --no-standing` first; `close` with several ids still closes the others.
**Approach (assumed at the header base):** map `ErrStanding` where the id is known (see the guard), for `update` (`internal/cli/update.go`) and for close's per-argument failure path (`closeOne` / `closeResult` in `internal/cli/close.go`); mention the refusal in `closeLong` and `updateLong`.
**Regression guard.** regenerate docs/site/reference/cli/ with `go run ./cmd/gendocs` in this item, add docs/site/reference/cli/ to Files and `GOMEMLIMIT=2GiB go test ./internal/clidoc/` to Acceptance. Word `ErrStanding` in `a.written` / `writeFailure` from `written.ID` (ops keeps the ID on failure), with update's edit closure returning the raw `*issue.StatusError` rather than pre-wording it; close already reaches the same path through `closeOne` → `a.written`.
**Files:** internal/cli/update.go, internal/cli/close.go, internal/cli/node_edit.go, internal/cli/close_test.go, internal/cli/write_test.go, docs/site/reference/cli/
**Read first:** internal/cli/node_edit.go — writeFailure, written; internal/cli/update.go — statusChangeFailure, applyUpdate; internal/cli/close.go — closeOne, closeLong; internal/ops/ops.go — EditNode, ChangeError
**Tests:** through `runCLI`, assert exit code and the exact message for close, `update --status active`, and close of a standing and a plain Issue together.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Close|Update'`
- `GOMEMLIMIT=2GiB go test ./internal/clidoc/`
- `go vet ./internal/cli/`
**Commit:** `feat(cli): explain why a standing Issue cannot be closed or claimed`

## 7. `ready`, `show` and `prime` output

Depends on items 4 and 5.

**What:** Recast at the regression check (2026-10-17).
**Goal:** `capcom ready` and the Ready lists of `capcom prime` omit a standing Issue; `capcom ready --all` lists it with the reason `standing: holds children` (in text; the JSON entry carries item 5's `standing` field); `show` prints `ready: no (standing: holds children)`; its open children are listed as usual.
**Approach (assumed at the header base):** add the reason to `notReadyReasons` in `internal/cli/ready.go` (used by `readyText` in `issue_render.go` too), and no field to `readyData`; describe it in `readyLong`. Prime needs no code change beyond what item 4 gives; add the test.
**Regression guard.** regenerate docs/site/reference/cli/ with `go run ./cmd/gendocs` in this item, add docs/site/reference/cli/ to Files and `GOMEMLIMIT=2GiB go test ./internal/clidoc/` to Acceptance; readyData gains NO `standing` field of its own — the `standing` field item 5 adds to the embedded issueData carries the mark in `ready --all --json`, so the Goal's JSON clause reads "the JSON entry carries item 5's `standing` field"; the reason `standing: holds children` is added wherever readyData already lists not-ready reasons, if it does; the `issueFromFile` copy is item 5's work — item 7 relies on it and drops internal/cli/show.go from its Files unless it still edits it for another reason (show's ready line comes from the index Entry via `loadShowGraph`); test fixtures that write a standing Issue declare the store format (`format:` in config.yaml at the new FormatStanding value) so `check`/`prime` do not report format-too-old; show tests live in internal/cli/read_test.go — fix Files and Tests. `notReadyReasons` puts the standing reason first (`standing: holds children; open children: x`); the tests build their own fixture rather than editing `newReadyStore`. readyData carries no reason string, so `ready --all` text shows the reason and `ready --all --json` gives the Parent `ready: false` and `standing: true`, with no new reason key.
**Files:** internal/cli/ready.go, internal/cli/ready_test.go, internal/cli/prime_test.go, internal/cli/read_test.go, docs/site/reference/cli/
**Read first:** internal/cli/ready.go — notReadyReasons, readyListData, readyData, readyLong; internal/cli/issue_render.go — readyText;
internal/cli/ready_test.go — readyConfig; internal/cli/read_test.go — TestShow_Text_OpenChildren_WordedAsReadyAll;
internal/clidoc/reference_test.go — TestReferenceUpToDate
**Tests:** through `runCLI`: a store with a standing Parent (children closed) and a standing Parent with an open child: `ready` lists only the child, `ready --all` text shows the reason; `ready --all --json` gives the Parent `ready: false` and `standing: true`, with no new reason key; `ready --all` prints the combined line `standing: holds children; open children: <child>` for the Parent with an open child, `ready --all --json` carries `standing: true` from `issueData`, `prime` lists neither Parent, `show` prints the reason (in `read_test.go`).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Ready|Prime|Show'`
- `GOMEMLIMIT=2GiB go test ./internal/clidoc/`
- `go vet ./internal/cli/`
**Commit:** `feat(ready): list a standing Issue under --all with its reason`

## 8. capcom ui shows the mark and the refusal

Depends on items 2 and 3.

**What:**
**Goal:** the ui detail pane shows `standing: yes` for a standing Issue; claim, status change and close on one show a status-bar notice naming the mark and `capcom update <id> --no-standing`, and write nothing.
**Approach (assumed at the header base):** a header line via `writeHeaderLine` in `internal/ui/detail.go`; `changedFields` in `internal/ui/write.go` gains `standing`; map `ErrStanding` in `claimAction`, `statusAction` and `closeAction` the way `closeAction` maps `ErrAlreadyClosed` to `alreadyClosedFormat`.
**Regression guard.** `issueSource` in internal/ui/detail.go copies `IsStanding` onto the Entry it builds.
**Files:** internal/ui/detail.go, internal/ui/write.go, internal/ui/detail_test.go, internal/ui/write_test.go
**Read first:** internal/ui/write.go — claimAction, statusAction, closeAction, changedFields; internal/ui/detail.go — issueHeader, issueSource, writeHeaderLine; internal/ui/write_close_test.go — TestWriteClose_Refusals_ShowTheMessageAndWriteNothing
**Tests:** detail render of a standing Issue; each of the three actions on a standing Issue yields the notice and leaves the file unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
- `go vet ./internal/ui/`
**Commit:** `feat(ui): show the standing mark and refuse claiming or closing it`

## 9. `check` reports a standing Issue that is closed or doing

Depends on item 1.

**What:**
**Goal:** `capcom check` reports, under a new diagnose kind `standing`, every standing Issue whose status is in the closed or doing category, at the `standing` line, naming the status; a standing todo Issue passes.
**Approach (assumed at the header base):** add `Standing Kind = "standing"` to `internal/diagnose/diagnose.go`; in `checker.checkIssue` (`internal/check/issues.go`) use `c.store.Config.CategoryOf(entry.issue.Status)` and `c.add(path, entry.issue.StandingLine(), diagnose.Standing, ...)`, as the missing-plan rule does. Not in `issue.Validate`: a Validate problem drops the file from the index.
**Regression guard.** The check fixtures declare `format: <FormatStanding>` in `.capcom/config.yaml`, as the `related` cases declare `format: 2`, so no `format-too-old` problem joins the exact problem list; `TestKind_EachKind_PinsItsString` pins the new kind.
**Files:** internal/diagnose/diagnose.go, internal/diagnose/diagnose_test.go, internal/check/issues.go, internal/check/check_test.go, internal/cli/check_test.go
**Read first:** internal/check/issues.go — checkIssue, addKeysFormatTooOld; internal/config/config.go — Config.CategoryOf; internal/diagnose/diagnose.go — MissingPlan; internal/diagnose/diagnose_test.go — TestKind_EachKind_PinsItsString; internal/check/check_test.go — issueFile
**Tests:** hand-written files: standing+closed and standing+doing are reported at the right line; standing+todo is clean; one `runCLI` test of the text output and exit code; `TestKind_EachKind_PinsItsString` pins `standing`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/check/ ./internal/diagnose/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run Check`
**Commit:** `feat(check): report a standing Issue that is closed or doing`

## 10. The managed block tells agents never to claim a standing Issue

**What:**
**Goal:** the managed block capcom writes into agent guides says a standing Issue only groups children and is never claimed or closed; this repo's `AGENTS.md` block matches the template, so `capcom check` is clean.
**Approach (assumed at the header base):** one sentence, a paragraph of its own in `blockGuidance` in `internal/agents/block.go`; update the goldens in `internal/agents/block_test.go`; resync `AGENTS.md` with `go run ./cmd/capcom agents`.
**Regression guard.** The sentence is its own paragraph in `blockGuidance` (not in `claimPurpose`'s claim line, not inside the store-map or Tree-lock paragraph) and never says `capcom update` (e.g. "a standing Issue only groups its children: never claim or close it"). Resync with `go run ./cmd/capcom agents`, not the installed binary. The item's CHANGELOG entry (sidecar) ends "An existing block reads as out of date after upgrading: run `capcom agents` once to refresh it (`capcom check` and `capcom agents --check` report it until you do)."
**Files:** internal/agents/block.go, internal/agents/block_test.go, internal/cli/agents_test.go, AGENTS.md
**Read first:** internal/agents/block.go — blockGuidance, claimPurpose, render; internal/agents/block_test.go — goldenBlock, TestRender_NoDoingStatus_LeavesOutTheClaimLine; internal/cli/agents_test.go — TestCheck_AgentsBlockWithoutStoreMap_IsStaleUntilAgentsRefreshesIt; AGENTS.md — the BEGIN/END CAPCOM block
**Tests:** the block goldens; `TestRender_NoDoingStatus_LeavesOutTheClaimLine` and the `internal/cli` agents tests stay green unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/agents/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run Agents`
- `go run ./cmd/capcom check`
**Commit:** `feat(agents): tell agents that standing Issues are never claimed`

## 11. Glossary, Knowledge and manual

Depends on items 5, 6, 7, 9 and 10.

**What:**
**Goal:** the glossary has a Standing Parent term and the Ready term names the exclusion; the manual's issue concepts, file-format fields table, JSON output reference, briefing and managed-block pages describe the mark, its refusals and its check finding; the generated CLI reference is current.
**Approach (assumed at the header base):** new Note `.capcom/knowledge/glossary/glossary-structure/standing-parent.md` via `go run ./cmd/capcom know add` (list it in `glossary-structure.md` if that Note lists children); edit `glossary-work/ready.md`, `glossary-structure/parent.md`, `glossary-structure/format-version.md` (if it lists formats) and `store-layout/store-config.md` (`format`); `docs/site/concepts/issues.md` (Parent, Ready), `docs/site/reference/file-format.md` (Fields table, and the Config section's format sample, list and raise paragraph), `docs/site/reference/json-output.md`, `docs/site/agents/briefing.md`, `docs/site/agents/managed-block.md`; regenerate `docs/site/reference/cli/` with `go run ./cmd/gendocs`.
**Regression guard.** Create the Note with `go run ./cmd/capcom know add --id standing-parent ...` below `glossary-structure` (there is no `know create`). The quoted block in `docs/site/agents/managed-block.md` matches what `go run ./cmd/capcom agents` writes into a fresh `AGENTS.md`, character for character. Every sentence that lists the format raises or names the format init writes is updated (`grep -rn 'format 4\|format: 4\|format 3 adds' docs/site .capcom/knowledge`); `managed-block.md:153` may keep `format: 4`. This item still regenerates docs/site/reference/cli/ as the final check (items 5–7 already did so for their own help changes).
**Files:** .capcom/knowledge/glossary/glossary-structure/standing-parent.md, .capcom/knowledge/glossary/glossary-structure.md, .capcom/knowledge/glossary/glossary-work/ready.md, .capcom/knowledge/glossary/glossary-structure/parent.md, .capcom/knowledge/glossary/glossary-structure/format-version.md, .capcom/knowledge/store-layout/store-config.md, docs/site/concepts/issues.md, docs/site/reference/file-format.md, docs/site/reference/json-output.md, docs/site/agents/briefing.md, docs/site/agents/managed-block.md, docs/site/reference/cli/
**Read first:** docs/site/agents/managed-block.md — the quoted block; docs/site/reference/file-format.md — Fields, Config; .capcom/knowledge/glossary/glossary-structure/format-version.md; docs/site/concepts/issues.md — Parent, Ready; internal/clidoc/reference_test.go — TestReferenceUpToDate
**Tests:** `TestReferenceUpToDate`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/clidoc/`
- `go run ./cmd/capcom check`
**Commit:** `docs: describe standing Parents`
