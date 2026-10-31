# Plan: batch apply and the MCP decision

**Goal:** Record the MCP decision as a proposed ADR that recommends against `capcom mcp`, and ship
`capcom apply <file|->`, which applies a YAML list of Issue operations all or none, with
`--dry-run`. Writes go through `internal/ops`, staged in memory and committed as one `tree.Plan`.
**Date:** 2026-10-31
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** e2bd2ec
**Closes:** batch-apply
**Sources:** `.capcom/issues/agent-ergonomics/batch-apply.md`; `.capcom/issues/agent-ergonomics/mcp-server-design.md`; Notes `adr-0012`, `adr-0017`, `adr-0010`, `decisions`; `docs/reviews/agent-eval-2026-10-29-qwen3.8-27b-ud-q4-k-m.md`; `docs/agent-eval.md`; `internal/tree/plan.go`

**Ratified design calls** (Airic Lenz, 2026-10-31):
- **MCP verdict:** ADR status Proposed, recommends not building `capcom mcp` now.
- **MCP evidence:** the existing 27B run plus the field report; no new eval run.
- **MCP revisit:** reopen when an agenteval run of any model fails a task, or repeats a failed call, because of flag syntax or shell quoting.
- **Atomicity:** stage, then commit. Ops run against an in-memory overlay of files and index; commit is one `tree.Plan`; `--dry-run` is the same run without the commit.
- **Format:** a YAML list of flat maps with an `op:` key; the other keys are CLI flag names without `--`.
- **Op set:** create, update, dep add, close, log. Nothing else.
- **File keys:** inline text only; `body-file` and `body-append-file` are refused.
- **Lock warning:** once per batch (ADR 0017 wording), only after a real commit.
- **Advertise:** one line in the generated agent guide block.
- **now ready:** names issues that existed before the batch and became Ready by its end.
- **if-match:** allowed on update, dep add, close and log (apply-only on the last three); compared with the file as it was on disk before the batch; create takes none, and if-match on an issue created earlier in the batch is refused, since it has no file before the batch.
- **Re-close:** closing an already-Closed issue fails the whole batch.

**Standing requirements:**
- skills: coding-standards
- Tests on the Pi: single packages with `GOMEMLIMIT=2GiB`; never `./...` with `-race`; never two test commands at once.
- CLI tests go through `runCLI` (Note `cli-test-harness`); error messages name the file and line (Note `error-messages`).
- After changing a command, flag or help text run `go run ./cmd/gendocs` and commit the result.

**Out of scope:** an MCP server or its issues; Notes in apply files; ops beyond the five; `capcom ui` changes; a release.

**Regression check (2026-10-31, e2bd2ec):**
- 2: guard folded (writer's decision plus the verified guards: CreateResult carries Written and tree problems, unwrapped refusals, self-dep is the unknown-dep refusal, checks before the body read)
- 3: guard folded
- 4: guard folded
- 5: recast (WithRaise beside WithFS, one Ops-owned Kind helper; the FS routing narrowed)
- 6: guard folded
- 7: guard folded; yields to the `store.Store.RaiseFormat` doc (internal/store/store.go:139-146)
- 8: recast (`ops.Staged(s, fsys, raise, c)`, ops never imports stage)
- 9: recast (if-match on every op)
- 10: guard folded
- 11: recast (noteWrite once per written id, gendocs moves here); yields to ADR 0017's decision (`adr-0017-decision`)
- 12: recast (no gendocs run; the Acceptance checks the page and the kind)
- 13: guard folded
- header: if-match call reworded (none on create; refused on an issue created in the batch)
- 5: guard folded (Goal names what WithFS and WithRaise route; "stay on disk" clause dropped; ReadContent keeps UnreadFile)
- 8: guard folded (four-argument Goal; the twin refreshes its index); recast (end-to-end test in package stage)
- 9: guard folded (`kv` an unknown key; an update with no changing key refused); recast (if-match per the header)
- 10: recast (only `at` is shared; no cached author)
- 11: guard folded (an exported nodestore helper words a commit-time `tree.ErrChanged`)
- 12: recast (the page states the header's if-match rule)
- all items: trimmed for length

## 1. ADR 0018: capcom does not serve its operations over MCP for now — ✅ DONE (2026-10-31)

NOTES (2026-10-31): adr-0018-rejected carries `labels: [background]` like all 16 existing ADR rejected Notes, so plain `capcom know tree adr-0018` lists only adr-0018-decision; `capcom know tree adr-0018 --background` lists both children (the Acceptance's literal command omits the flag).
NOTES (2026-10-31): the record lives in Knowledge, not as a file under docs/adr/ as the mcp-server-design Acceptance words it; docs/adr/README.md routes new records to Knowledge in ADR 0010's shape.

**What:**
**Goal:** Note `adr-0018` (status "Proposed") with children `adr-0018-decision` and `adr-0018-rejected`, listed in Note `decisions`; `capcom check` reports no problems.
**Approach (assumed at the header base):** ADR 0010's shape (Status line, summary, reading order; children in `.capcom/knowledge/decisions/adr-0018/`), via `capcom know add --parent` or by hand plus `capcom check`. The decision child holds, as binding content: the tools (one per `internal/ops` write, plus `next`, `ready`, `show`, `context`, `search`, `know show`, `know tree`); schemas derived from the cobra flags as `cmd/gendocs` reads them, enums from config.yaml statuses and types; the cost (an MCP Go SDK or a hand-written JSON-RPC stdio loop, a third front end); the Tree lock token and ETag as tool arguments, ADR 0017's warning in the tool result; the numbers (Qwen3.8 27B: 12/12 tasks, 3 of 74 calls failed, 0 repeated; call count, not flag syntax, was the cost, which `capcom apply` addresses); not now, with the header's revisit trigger. The rejected child: build it now, MCP for reads only, a generated tool list file without a server.
**Files:** .capcom/knowledge/decisions/adr-0018.md; .capcom/knowledge/decisions/adr-0018/adr-0018-decision.md; .capcom/knowledge/decisions/adr-0018/adr-0018-rejected.md; .capcom/knowledge/decisions.md
**Read first:** .capcom/knowledge/decisions.md — the records list; .capcom/knowledge/decisions/adr-0010.md — Status line, summary, reading order; .capcom/knowledge/decisions/adr-0017.md, adr-0017/adr-0017-decision.md, adr-0017/adr-0017-rejected.md — child titles and shape;
docs/adr/README.md — new record takes ADR 0010's shape in Knowledge; .capcom/issues/agent-ergonomics/mcp-server-design.md — Acceptance;
docs/reviews/agent-eval-2026-10-29-qwen3.8-27b-ud-q4-k-m.md — Total row
**Closes:** mcp-server-design
**Tests:** none (docs only).
**Acceptance:**
- `capcom check` exits 0.
- `capcom know show adr-0018` prints `Status: Proposed.`
- `capcom know tree adr-0018` lists `adr-0018-decision` and `adr-0018-rejected`.
- `capcom know show decisions` lists `adr-0018`.
**Commit:** `docs(adr): propose ADR 0018, no MCP server for now`

## 2. Move issue creation and the value checks into ops — ✅ DONE (2026-10-31)

NOTES (2026-10-31): ops.ValueError carries a fourth field, Err (the reason: ErrUndeclaredStatus, ErrUndeclaredType, ErrPriorityRange, or ValidatePlanName's error), so the plan check is a ValueError too (Field "plan", Valid nil) and the CLI's one wording function, valueFailure, rebuilds today's messages unchanged; category stays the CLI check checkCategory in flags.go.
NOTES (2026-10-31): the closed-status refusal is an *issue.StatusError wrapping the new sentinel ops.ErrCreatedClosed, raised at build time as before (after Kind.Create's id and tree checks); the CLI's createFailure words it with today's hint. ops.MissingPlan(plan) is exported so update keeps its plan warning without a CLI copy of the rule; the CLI keeps only missingPlanWarnings(path, message) to word it.
NOTES (2026-10-31): internal/cli/know_update.go named in Files holds none of the moved symbols, so it is untouched; createNode now serves know add only, so issueVerbs' writeResult field is no longer read (left in place, the struct is shared with noteVerbs).
NOTES (2026-10-31): consequential edit — internal/ops/ops.go: made necessary by CreateIssue and the value checks (package doc said every write takes an if-match)
NOTES (2026-10-31): consequential edit — internal/cli/node_verbs.go: made necessary by create no longer running createNode (nodeVerbs and issueVerbs comments)
NOTES (2026-10-31): consequential edit — internal/cli/doc.go: made necessary by valueChecker's removal from flags.go and create going through internal/ops

**What:**
**Goal:** `internal/ops` offers `CreateIssue` and typed value checks; the CLI's `create` keeps only flags and output, its behaviour, messages and tests unchanged.
**Approach (assumed at the header base):** `ops.CreateIssue(NewIssue) (CreateResult, error)` in `internal/ops/create.go` takes over from `internal/cli/create.go` `checkCreateValues`, `checkPlanValue`, `resolveDeps`, `namesItself`, the closed-status refusal and the defaults of `newIssue`, `defaultType` and `newIssueBody` (pure over body, why, acceptance). The `valueChecker` checks of `internal/cli/flags.go` (status, type, priority) become `ops.ValueError{Field, Value, Valid}` in `internal/ops/values.go`. `missingPlanWarnings` and the empty-body warning become `CreateResult` fields; priority is `*int` (0 valid); deps resolve through the shared resolver. The CLI keeps no copy of create's rules.
**Regression guard.** Created comes from NewIssue, never the clock; value, closed-status and standing refusals return unwrapped (no `ChangeError`, no Path); every caller of a moved helper (internal/cli update.go, list.go, reopen.go, know_update.go, any the grep finds) uses the ops version.
- One CLI function words `ops.ValueError`: usage for list (read_test.go:254-269), invalid for writers; category stays a CLI check; closed-status keeps today's hint, standing goes through `statusChangeFailure(id, err)`.
- `CreateResult` carries `nodestore.Written` (with Effects) and the `tree.Errors` problems; the CLI keeps `unwarnedTreeProblems`, `warnRaisedFormat`, `noteKindWrite`; warnings ordered tree problems, empty body, missing plan.
- A self-dep is the unknown-dep refusal (`*reflist.Refusal`, Holder unset; no ReasonSelf). `NewIssue`'s body func runs inside build: `--body-file` is read after value checks, dep resolution and `Kind.Create`'s id checks.
**Files:** internal/ops/create.go; internal/ops/values.go; internal/ops/create_test.go; internal/cli/create.go; internal/cli/flags.go; internal/cli/update.go; internal/cli/list.go; internal/cli/reopen.go; internal/cli/know_update.go; any other file the grep for the moved symbols finds
**Read first:** internal/cli/create.go — create, checkCreateValues, resolveDeps, newIssue, newIssueBody; internal/cli/node_verbs.go — createNode, issueVerbs; internal/nodestore/create.go — Kind.Create; internal/cli/flags.go — valueChecker, invalidValues, usageValues;
internal/cli/update.go — checkUpdateValues, statusChangeFailure; internal/cli/list.go — checkListValues; internal/cli/write_test.go — TestCreate_MissingIDOrClosedStatus_IsRefused, TestCreateUpdateCloseReopen_InvalidFlagValue_ListsTheValidValuesAndWritesNothing;
internal/ops/ops_test.go — newOps, issueText
**Tests:** `internal/ops/create_test.go`: defaults, an undeclared type, a closed status (unwrapped, no path), a self-dep, an unknown dep, Created from the input. The `internal/cli` create tests pass unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ops/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/`
- `go vet ./internal/ops/ ./internal/cli/`
**Commit:** `refactor(ops): move issue creation and value checks out of the CLI`

## 3. Move issue update into ops — ✅ DONE (2026-10-31)

NOTES (2026-10-31): IssueUpdate carries IsStanding *bool (true sets, false clears) and AppendText func() (string, error) for --body-append-file, run after the value checks and before the ref resolves, so `--status doing --body-append-file missing.md` stays the invalid-status refusal; its text is appended after Appends, as before.
NOTES (2026-10-31): the at-least-one-flag rule stays flag-based in the CLI (hasAnyFlag); ops refuses an empty IssueUpdate with ErrNoChange, and the CLI rewrites the issue unchanged through editIssue when the flags give only no-op values (--no-standing=false), so ops never refuses what hasAnyFlag accepts. standingChange is folded into the CLI's issueUpdate flag mapping.
NOTES (2026-10-31): checkPairs and bodyEdits stay in internal/cli/node_edit.go as the flag adapter both update and know update share (the uneven-pair usage wording is about flag positions; []ops.Replacement cannot be uneven); bodyEdits.apply now runs ops.EditBody, whose *ops.ReplacementError writeFailure words with the unchanged `--body-replace-old #%d %q: ...` text via replacementFailure. internal/cli/know_update.go needed no change.

**What:**
**Goal:** `internal/ops` offers `UpdateIssue`; the CLI's `update` keeps only flags and output, its behaviour, messages and tests unchanged.
**Approach (assumed at the header base):** `ops.UpdateIssue(ref, ifMatch string, u IssueUpdate) (nodestore.Written, error)` in `internal/ops/update.go`; `IssueUpdate` has pointer fields for title, status, type, priority, plan, standing, label add/remove lists, body appends and `[]Replacement{Old, New}`. `applyUpdate`, `standingChange`, `bodyEdits.apply`, `checkPairs`, `checkUpdateValues` and the "at least one change" rule leave `internal/cli/update.go` and `node_edit.go`; reuse item 2's `ValueError`; `statusChangeFailure` stays in the CLI. Depends on item 2.
**Regression guard.**
- The "at least one flag" check stays flag-based (or `IssueUpdate` counts a given no-op flag): ops never refuses what `hasAnyFlag` accepts (TestUpdate_NoStandingFalse_LeavesTheMarkAlone).
- `know update` shares `bodyEdits`, `checkPairs`, `apply`; both keep the wording `--body-replace-old #%d %q: ...` and the uneven-pair usage error.
- Value and pair checks run unwrapped before ref resolves and ahead of `readTextInput`: `--status doing --body-append-file missing.md` stays the invalid-status refusal.
**Files:** internal/ops/update.go; internal/ops/update_test.go; internal/cli/update.go; internal/cli/node_edit.go; internal/cli/know_update.go
**Read first:** internal/cli/update.go — newUpdateCommand, checkUpdateValues, applyUpdate, standingChange, statusChangeFailure; internal/cli/node_edit.go — bodyEdits, checkPairs, apply, writeFailure, editIssue; internal/cli/know_update.go — bodyEdits, the RunE checkPairs call;
internal/ops/ops.go — EditIssue, ChangeError; internal/cli/write_test.go — TestUpdate_NoStandingFalse_LeavesTheMarkAlone, TestUpdate_RepeatedBodyEdits_AreRefusedWhole, TestUpdate_StatusIntoOrOutOfClosed_IsRefused;
internal/cli/know_update_test.go — TestKnowUpdate_RepeatedBodyEdits_AreRefusedWhole
**Tests:** `update_test.go`: no change refused, a replace pair not found, a closed status refused, standing set and cleared, a value refusal unwrapped before the ref resolves. The update and know update tests pass unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ops/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/`
**Commit:** `refactor(ops): move issue update out of the CLI`

## 4. A file-system seam in nodefile, threaded through tree — ✅ DONE (2026-10-31)

NOTES (2026-10-31): leftOutFileOf takes no fsys: it keeps reading the size from entry.Info(), the DirEntry the FS's own ReadDir returned, so the Disk path stays byte-for-byte unchanged on every platform (Windows serves Info from the directory read, not a fresh Lstat); an overlay FS supplies its sizes through the DirEntries it lists.
NOTES (2026-10-31): Tree keeps the FS it was loaded with, so Claimant's orphan-directory listing (findOrphanFile) reads through the same FS as the load; Load and the exported Disk wrappers (Walk, ClaimOnlyFacts, ListOrphanFiles, RepeatedIDDirProblems) pass a tree-local diskFS whose ReadDir calls the readDir var, so SetReadDir still reaches them.
NOTES (2026-10-31): consequential edit — internal/tree/doc.go: made necessary by adding LoadContentFS to tree.go and diskFS to walk.go (the file map names each file's contents)
NOTES (2026-10-31): consequential edit — internal/nodefile/doc.go: made necessary by adding FS and Disk to the package (the package comment lists what it provides)

**What:**
**Goal:** `nodefile.FS` (Stat, Lstat, ReadDir, ReadRegularFile, ReplaceFile, CreateFile) with a `nodefile.Disk` implementation, and every `tree` load path reads through a given `FS`; behaviour unchanged.
**Approach (assumed at the header base):** `internal/nodefile/fs.go`: `Disk` wraps `os.Stat`, `os.Lstat`, `os.ReadDir`, `ReadRegularFile`, `ReplaceFileAtomic`, `WriteFileExclusive`. `tree.LoadContentFS(fsys, ...)` passes `fsys` through `Walk`, `ClaimOnlyFacts`, `ListOrphanFiles`, `appendOrphanFiles`, `RepeatedIDDirProblems`, `leftOutFileOf`; `tree.LoadContent` calls it with `Disk`; the index keeps the Disk-backed functions.
**Regression guard.**
- Exported `Walk(dir)`, `ClaimOnlyFacts`, `ListOrphanFiles`, `RepeatedIDDirProblems` keep their signatures as Disk wrappers; `fsys` threads only through unexported walkers or new `*FS` siblings, so index/walk.go, index/duplicates.go, nodestore/refs.go and tree tests build unchanged.
- The Disk entry points pass a tree-local FS whose ReadDir calls the `readDir` var, honoured by the walk (walk.go:54) and `appendOrphanFiles` (claims.go:474): TestWalk_ChildDirectoryGoneBeforeItsRead_WalksTheRest stays green.
**Files:** internal/nodefile/fs.go; internal/nodefile/fs_test.go; internal/tree/walk.go; internal/tree/claims.go; internal/tree/tree.go
**Read first:** internal/tree/walk.go — Walk, treeWalker.walk, leftOutFileOf, readDir; internal/tree/claims.go — ClaimOnlyFacts, ListOrphanFiles, appendOrphanFiles, RepeatedIDDirProblems, findOrphanFile; internal/tree/tree.go — LoadContent, load;
internal/tree/export_test.go — SetReadDir; internal/tree/tree_test.go — TestWalk_ChildDirectoryGoneBeforeItsRead_WalksTheRest; internal/index/walk.go — tree.Walk and ClaimOnlyFacts callers;
internal/nodefile/read.go — ReadRegularFile; internal/nodefile/atomic.go — ReplaceFileAtomic, WriteFileExclusive
**Tests:** `fs_test.go` covers `Disk`; the tree tests pass unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodefile/ ./internal/tree/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/`
**Commit:** `refactor(tree): read the node tree through a nodefile.FS`

## 5. nodestore reads and writes through an injected FS — ✅ DONE (2026-10-31)

NOTES (2026-10-31): re-derived from the assumption that Kind.load and every nodestore.Issues( in internal/ops sit only in the files the item lists: load lives in internal/nodestore/load.go (now tree.LoadContentFS through the Kind's FS) and the second ops call site is internal/ops/create.go (CreateIssue, added by item 2), both now routed through the Ops-owned helper Ops.issues.
NOTES (2026-10-31): Reader gains ReadFS(fsys, path, relPath), which Read calls with nodefile.Disk; ReadContent uses it, so diagnose.ReadRefusal still maps a FIFO or a file over 1 MiB to tree.UnreadFile before Decode.
NOTES (2026-10-31): Ops gains fsys and raise fields, set by New to nodefile.Disk and s.RaiseFormat; Ops.issues applies them with WithFS and WithRaise. Nothing sets them otherwise until item 8.
NOTES (2026-10-31): Format, Move and Remove are not routed (out of the Goal): they load through the Kind's FS but read, and Move and Remove write, node files on disk; Format writes through place, so through the FS. WithFS's doc says a Kind with a non-Disk FS is for Create and Edit only.
NOTES (2026-10-31): TestReadContent_ParseableUnparseableAndInvalidFiles gains a file over 1 MiB in its table and the makeFIFO case as its own subtest, so a skip on Windows or as root skips only the FIFO.

**What:** Recast at the regression check (2026-10-31).
**Goal:** `nodestore.Kind.WithFS(fsys)` routes load, `ReadContent`, `RecoverAliases`, `Create` and `Edit` through `fsys`, and `Kind.WithRaise` routes the format raise; the default Kind uses `nodefile.Disk` and behaves as before.
**Approach (assumed at the header base):** Like `WithSync` in `internal/nodestore/kind.go`: `Kind.load` (via `tree.LoadContentFS`), `ReadContent` (read through `fsys`, then `Reader.Decode`), `RecoverAliases`, `Edit`'s read and `recheck`, `Create`'s parent `Lstat` and `place` go through `fsys`; `beforeRecheck` keeps working. Depends on item 4.
**Regression guard.** `Kind.WithRaise(func(version int) (bool, error)) *Kind[T]` sits beside `WithFS` (`raiseFormat` stays the seam); every `nodestore.Issues(` in internal/ops goes through one Ops-owned helper applying the Ops's FS and raise; `Claimant` uses the loaded tree.
- `ReadContent` keeps `diagnose.ReadRefusal` mapping to `tree.UnreadFile` before `Reader.Decode`: a FIFO or a file over 1 MiB stays unread, not a `*tree.Error`.
**Files:** internal/nodestore/kind.go; internal/nodestore/edit.go; internal/nodestore/create.go; internal/nodestore/write.go; internal/nodestore/read.go; internal/nodestore/kind_test.go; internal/ops/ops.go
**Read first:** internal/nodestore/kind.go — Kind, WithSync, Issues, ReadContent, RecoverAliases, raise; internal/nodestore/edit.go — Edit, recheck, beforeRecheck; internal/nodestore/create.go — Create, newNodePath;
internal/nodestore/write.go — place, encode; internal/nodestore/read.go — Reader.Read, Reader.Decode; internal/nodestore/load.go — load; internal/nodestore/format.go — Format; internal/ops/ops.go — EditIssue
**Tests:** `kind_test.go`: a recording FS sees an Edit's read and write and a Create's write, nothing changing on disk; a recorded Create of a standing issue below the standing format calls the `WithRaise` func and leaves config.yaml unchanged; `TestReadContent_ParseableUnparseableAndInvalidFiles` gains a `makeFIFO` case read as `tree.UnreadFile`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodestore/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ops/`
**Commit:** `refactor(nodestore): read and write through an injected nodefile.FS`

## 6. ops resolves through a Catalog interface — ✅ DONE (2026-10-31)

NOTES (2026-10-31): internal/ops/update.go is listed under Files but holds no index use (its only `index` is a loop variable), so it is untouched; create.go changed only in its CreateIssue doc ("resolved through the Catalog").
NOTES (2026-10-31): ops.New now takes `x Catalog` instead of `*index.Index`; every caller (internal/cli/node_edit.go, internal/ui/write.go and the ui tests) passes an *index.Index and compiles unchanged.

**What:**
**Goal:** `ops` resolves and lists through `ops.Catalog { Resolve(string) (index.Entry, error); List(index.Filter) ([]index.Entry, error) }`, which `*index.Index` satisfies; `index.EntryOf` builds an `index.Entry` from a parsed issue; `ops.New(s, x)` callers are unchanged.
**Approach (assumed at the header base):** The `*index.Index` field's uses in `internal/ops/ops.go` and `deps.go` (`issueResolver.Resolve`, `AddDep`, `RemoveDep`, `ResolveIssueID`, `IssueNodes`, `NewCloser`) go through the interface. `index.EntryOf(path, id, parentID string, i issue.Issue, cfg config.Config) Entry` in `internal/index/query.go` fills every field `Entry.Node` reads (ParentID, Category, IsStanding, Labels, Deps, Aliases).
**Regression guard.**
- `grep -n 'o\.index\|r\.index' internal/ops/*.go` finds no concrete `*index.Index` use, create.go and update.go included.
- `EntryOf` fills fields as `insertIssue` stores them: Category from `cfg.CategoryOf`, times `UTC().Truncate(time.Second)`, path worktree-relative with forward slashes.
- Supersedes the ops package and `Ops` docs ("through the index", internal/ops/ops.go:1-4, 55-56): reword both to `Catalog`.
**Files:** internal/ops/ops.go; internal/ops/deps.go; internal/ops/ops_test.go; internal/ops/create.go; internal/ops/update.go; internal/index/query.go; internal/index/query_test.go
**Read first:** internal/ops/ops.go — Ops, New, EditIssue, issueResolver.Resolve, IssueNodes, NewCloser; internal/ops/deps.go — AddDep, RemoveDep, ResolveIssueID, DepGraph; internal/index/query.go — Index.Resolve, Index.List, Entry.Node; internal/index/refresh.go — insertIssue;
internal/index/index.go — Entry, scanIssues, loadIssues, formatTime; internal/index/query_test.go — TestEntryNode_IndexedIssue_CarriesWhatTheReadyRulesRead;
internal/index/index_test.go — newStore, writeIssue, openIndex, refresh; internal/ops/ops_test.go — newOps
**Tests:** `query_test.go`: `EntryOf` equals the entry the index stores after a refresh; ops tests pass with a fake Catalog for one write.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/ ./internal/ops/`
**Commit:** `refactor(ops): resolve through a Catalog interface`

## 7. The staging overlay and its commit plan — ✅ DONE (2026-10-31)

NOTES (2026-10-31): consequential edit — internal/tree/export_test.go: made necessary by the new Plan.create seam (SetPlanCreate), which the test of a write failing after its mkdir injects through
NOTES (2026-10-31): consequential edit — internal/tree/doc.go: made necessary by adding AddCreate to plan.go (the file map names plan.go's contents)
NOTES (2026-10-31): Overlay.Plan returns *tree.Plan with no error: RaiseFormat computes and keeps the raised config.yaml bytes when it stages the raise, so Plan has nothing left to fail on. The config.yaml raise is an AddStep (verbatim rewrite with its own unchanged check, or AddCreate when missing) placed first among the steps; tree.Plan still runs every AddRewrite before it, as Apply always orders rewrites first.
NOTES (2026-10-31): stage.New takes *store.Store (config path and its worktree-relative name); OriginalETag returns (string, error), failing with fs.ErrNotExist for a file only a staged create made, so item 10 can refuse if-match on an issue created in the batch.
NOTES (2026-10-31): RaiseFormat reports false for a version at or below one already staged, as store.RaiseFormat would after the first raise landed; a staged read over 1 MiB is refused with ErrTooLarge, as the disk read would refuse it.

**What:**
**Goal:** `internal/stage.Overlay` implements `nodefile.FS` over the disk with every write in memory; `Overlay.Plan()` returns one `tree.Plan` applying all staged writes or none; `tree.Plan.AddCreate` creates a file and its missing directories and undoes both.
**Approach (assumed at the header base):** `overlay.go`: a map from absolute path to `{data, original []byte, isNew bool}`, `original` taken on the first disk read; `ReadDir` merges staged new files and directories as synthesized `fs.DirEntry` values. `plan.go`: `Plan()` adds an `AddRewrite{Original}` per changed file, an `AddCreate` per new file in staged order, and a config.yaml step when the format was raised; `OriginalETag(path)` is the pre-batch ETag, for if-match. `tree.Plan.AddCreate(path, content)` uses `WriteFileExclusive`, records with `Lstat` the directories it made, and undoes the file and them. Depends on items 4 and 5.
**Regression guard.** `Overlay.RaiseFormat(version int) (bool, error)` records the highest version asked for and writes config.yaml only in `Plan().Apply()`.
- Yields to `store.Store.RaiseFormat`'s doc (internal/store/store.go:139-146, store_test.go:469,554): `config.RaiseFormat` over the disk bytes, isRaised false when disk already declares it, `AddCreate` for a missing config.yaml, else a verbatim rewrite (CRLF and mode kept).
- `AddCreate` removes the directories it made when its own write fails (WriteFileExclusive runs MkdirAll first).
**Files:** internal/stage/overlay.go; internal/stage/plan.go; internal/stage/overlay_test.go; internal/tree/plan.go; internal/tree/plan_test.go
**Read first:** internal/tree/plan.go — Plan, AddRewrite, AddStep, Apply, rollBack, checkUnchanged; internal/tree/move.go — Rewrite; internal/nodefile/atomic.go — WriteFileExclusive, writeTemp;
internal/store/store.go — RaiseFormat; internal/config/format.go — RaiseFormat; internal/tree/walk.go — Walk, leftOutFileOf; internal/nodestore/write.go — place
**Tests:** a staged create and edit are invisible before `Apply`, which lands both; a file changed after staging fails `Apply` with `tree.ErrChanged`, every file as before; `AddCreate`'s undo, and a write failing after its mkdir, remove the directories it made; a staged raise writes nothing before `Apply`, creates a missing config.yaml, keeps a CRLF one's line endings and mode.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/stage/ ./internal/tree/`
**Commit:** `feat(stage): stage node writes in memory and commit them as one plan`

## 8. Staged ops: a Catalog that sees staged issues — ✅ DONE (2026-10-31)

NOTES (2026-10-31): internal/stage/overlay.go gains a `store *store.Store` field, set in stage.New from the store it already takes: Overlay.Catalog(x) needs the issue tree's directory, the working tree root (RelativePath) and the config (IssueReader, EntryOf), and its Goal signature passes only the index.
NOTES (2026-10-31): Catalog.Resolve also adds staged creates to the candidates of an index *tree.AmbiguousError (same error type, fuller candidate list); a staged file that does not decode as usable resolves as a *tree.UnusableError and is left out of List. List queries the index without Limit, applies the Filter to staged entries in Go (every Filter field, not only IsClosedIncluded), then caps at Limit.
NOTES (2026-10-31): a name that only an alias a staged edit removed still resolves through the index to that issue; no op of the apply set removes aliases, so it is left as is.

**What:** Recast at the regression check (2026-10-31).
**Goal:** `ops.Staged(s, fsys, raise, c)`, `c` from `Overlay.Catalog(x)`, returns an `*Ops` writing into the overlay and resolving issues created or edited earlier in the stage, so ops behave as on disk with nothing written until `Plan().Apply()`.
**Approach (assumed at the header base):** `internal/stage/catalog.go`: `Overlay.Catalog(x *index.Index) ops.Catalog`. `Resolve` matches staged ids and aliases exactly, then asks the index, keeping its refusals; on its not-found it tries a prefix of staged new ids. `List` returns index entries overridden by staged ones, plus new ones, built with `index.EntryOf`. Depends on items 2, 3, 6 and 7.
**Regression guard.** `ops.Staged(s *store.Store, fsys nodefile.FS, raise func(int) (bool, error), c Catalog) *Ops`, Kind `WithFS(fsys).WithRaise(raise)`; stage imports ops, so the end-to-end test is `internal/stage/staged_test.go` with its own helpers, its twin running the same ops calls via `ops.New` at fixed times, `t.Setenv` GIT_AUTHOR_NAME, `x.Refresh()` after each write.
- Ambiguity is `*tree.AmbiguousError`; staged ids and aliases match on `tree.NormalizeArg(ref)`; staged new ids are also checked when the index resolved ref only as a prefix; fall back only on `errors.As` `*tree.NotFoundError`, keeping `*tree.UnusableError`.
- Staged Paths are `store.RelativePath(s.WorktreeRoot, abs)`; `List` merges by Path in byte order (SQLite BINARY), staged entries filtered by the same Filter (at least `IsClosedIncluded`).
**Files:** internal/stage/catalog.go; internal/stage/catalog_test.go; internal/stage/staged_test.go; internal/ops/ops.go
**Read first:** internal/ops/ops.go — Ops, New, EditIssue, issueResolver, IssueNodes, NewCloser, Closer.Close; internal/ops/deps.go — AddDep, DepGraph, ResolveIssueID; internal/index/query.go — Index.Resolve, snapshotNames.Match, PrefixCandidates, Index.List, Filter;
internal/tree/resolve.go — ResolveIn, prefixMatch, NormalizeArg, AmbiguousError, NotFoundError, UnusableError; internal/store/store.go — RelativePath; internal/ops/ops_test.go — newOps, readIssue, closedAt;
internal/cli/create.go — resolveDeps
**Tests:** `staged_test.go`: create A; create B with parent A and dep A; dep add; log on B; close A; commit; every file matches the twin's; a cycle across two staged dep adds is refused; a stage dropped uncommitted leaves the store untouched. `catalog_test.go`: a prefix matching an index id and a staged id is `*tree.AmbiguousError`; `List` merges in path order.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/stage/ ./internal/ops/`
**Commit:** `feat(ops): run writes against a staged overlay`

## 9. The apply file format and its parser — ✅ DONE (2026-10-31)

NOTES (2026-10-31): Op carries ops.NewIssue (create, Created left zero for the runner; Body set only for a non-empty body, as capcom create does) and ops.IssueUpdate (update), so item 10 hands them to ops unchanged; `no-standing: false` alone passes the changing-key check and leaves Update empty, as capcom update does.
NOTES (2026-10-31): batch.Error mirrors frontmatter.Error (Name, Line, Position, Message; Error() renders `name:line: message`) so item 11 can word it as an output.Error with Path and Line; Parse itself maps the name `-` to `stdin`.
NOTES (2026-10-31): a null scalar on a string key reads as "" (so `plan: ~` clears like `plan: ""`) rather than its literal text; every other scalar keeps its text whatever its tag, as specified.
NOTES (2026-10-31): comma-split keys (label, dep, add-label, remove-label) split every list item too, each item standing for one flag repetition as pflag's StringSlice splits each, and trim white space around each value; an empty scalar gives no values, as pflag's empty value does.
NOTES (2026-10-31): a missing or unknown op is worded `operation <n>: ...` without `(<op>)`, since there is no known op to name; file-level refusals (empty file, not a list, YAML syntax, a second document) carry Position 0; a missing required key, an update with no changing key and uneven body-replace pairs are refused at the operation's line.
NOTES (2026-10-31): parse.go is ~610 lines, past the ~400-line guideline, kept in the one file the item's Files line names; its yamlError repeats the line-lifting helper userconfig and frontmatter each keep privately (pre-existing duplication, not folded here).

**What:** Recast at the regression check (2026-10-31).
**Goal:** `internal/batch.Parse(name string, data []byte) ([]Op, error)` turns an apply file into typed operations with their position and line, and refuses an invalid file naming the operation's position and line.
**Approach (assumed at the header base):** `internal/batch/parse.go`, `gopkg.in/yaml.v3` `Node` for lines. A YAML sequence of mappings; `op` is `create`, `update`, `dep add`, `close` or `log`; other keys are flag names without `--`, plus `id` (target of update, dep add, close, log), `dep` (dep add's dependency) and `text` (log's text).
- create: id, title, body, why, acceptance, dep, label, parent, plan, priority, standing, status, type.
- update: id, if-match, title, status, type, priority, plan, standing, no-standing, add-label, remove-label, body-append, body-replace-old, body-replace-new.
- dep add: id, dep, if-match. close: id, reason, status, if-match. log: id, text, if-match.
A repeatable flag takes a scalar or a list; a boolean true or false. Refused at its line: an empty file or list; an unknown `op`; an unknown key (`if-match` on create, `kv`); `body-file`, `body-append-file` ("apply files take text inline"); a missing required key; a wrong YAML type. Messages: `<name>:<line>: operation <n> (<op>): <problem>`, `<name>` `stdin` for `-`.
**Regression guard.** if-match follows the header (apply-only on dep add, close, log; an unknown key on create). An update op with none of the keys `hasAnyFlag` checks is refused at its line: "update needs at least one flag that changes the issue".
- A string key takes any ScalarNode's Value whatever its tag (`title: 2026`); a wrong type is a wrong node Kind (mapping, sequence, alias), a non-int priority, or a boolean not true/false.
- A repeated key, and `standing` with `no-standing`, are refused at the second key's line. `label`, `dep`, `add-label`, `remove-label` split a scalar on commas; `acceptance` and `body-append` never split.
**Files:** internal/batch/parse.go; internal/batch/parse_test.go
**Read first:** internal/cli/create.go — newCreateCommand, createOptions; internal/cli/update.go — newUpdateCommand, standingChange, hasAnyFlag, checkUpdateValues; internal/cli/node_edit.go — bodyEdits.checkPairs;
internal/cli/flag_once.go — isAccumulating, refuseRepeatedFlags; internal/cli/close.go — newCloseCommand, requireReason; internal/cli/log.go — newLogCommand; internal/cli/dep.go — newDepAddCommand;
internal/cli/root.go — withKV
**Tests:** table tests, one valid file per op and each refusal above, asserting position and line; rows: `title: 2026` accepted, a repeated key, `label: a,b` as two labels, `standing` with `no-standing`, `kv`, an update with only `id`, `if-match` accepted on update, dep add, close and log, refused on create.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/batch/`
**Commit:** `feat(batch): parse apply files with operation positions and lines`

## 10. Run a batch against staged ops

**What:** Recast at the regression check (2026-10-31).
**Goal:** `batch.Run(o *ops.Ops, ov *stage.Overlay, ops []Op, at time.Time) (Outcome, error)` runs the operations in order against staged ops and returns per-op outcomes and the newly Ready issues, or the first failure as `*OpError{Index, Line, Op, Err}`; it never commits.
**Approach (assumed at the header base):** `internal/batch/run.go` maps each `Op` to `ops.CreateIssue`, `ops.UpdateIssue`, `AddDep`, a `Closer` close or `AppendLog`, with one `at` for the batch; `gitx.AuthorName` gives a dry run and a real run the same name, so only `at` is shared, and no same bytes across separate runs are promised. `if-match` is checked against `ov.OriginalETag` of the issue's file, then the op runs with if-match "". Re-closing is an `OpError`. `Outcome.NowReady`: ids that existed before the batch, were not Ready then, and are Ready after. A format raise is reported once. Depends on items 8 and 9.
**Regression guard.**
- A close without `status` closes to `cfg.DefaultStatus(config.CategoryDone)`; an undeclared or non-closed status gets the CLI's wording (`targetStatus` / `ValueError`). Log entries take `at.Local()`, create and close `at` UTC-truncated to the second; Created comes from `NewIssue`.
- A mismatch is `*nodestore.ConflictError` (Path, IfMatch, ETag, "etag X does not match the file's etag Y; show the issue again"); if-match on an issue created earlier in the batch is refused.
- Each op's warnings (empty body, missing plan, dropped comments, the format raise once) ride on `Outcome` for item 11.
**Files:** internal/batch/run.go; internal/batch/run_test.go
**Read first:** internal/ops/ops.go — Closer, NewCloser, Closer.Close, AppendLog, IssueNodes; internal/cli/reopen.go — targetStatus; internal/cli/close.go — closeResult, closeOne; internal/issue/ready.go — Ready;
internal/issue/mutate.go — Close, AppendLog; internal/nodestore/errors.go — ConflictError, conflictError; internal/cli/log.go — newLogCommand
**Tests:** three creates with deps; a failing third op stays uncommitted and returns its position; a second touch's if-match against the pre-batch ETag fails as `*nodestore.ConflictError`; if-match on a batch-created issue refused; re-close refused; a status-less close gets the first done status; a log dated as `capcom log` dates it; an empty-body warning on `Outcome`; NowReady excludes created issues.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/batch/`
**Commit:** `feat(batch): run apply operations against staged ops`

## 11. The capcom apply command

**What:** Recast at the regression check (2026-10-31).
**Goal:** `capcom apply <file|->` applies an apply file all or none, `--dry-run` reports the same outcome and writes nothing, and output comes in text, `--json` (kind `apply`) and `--kv`.
**Approach (assumed at the header base):** `internal/cli/apply.go`, wired in `root.go`: read the file (stdin for `-`), `batch.Parse`, `stage.Overlay` and `ops.Staged`, `batch.Run`, then unless `--dry-run` `ov.Plan().Apply()`. Text: `<n>  <op>  <id>  <outcome>` per op, then `now ready: a, b` if any; a dry run opens with `dry run: nothing written`. JSON `data`: `{"dry_run": bool, "ops": [{"index","line","op","id","outcome","etag"}], "now_ready": []}`; `--kv`: a record per op. An `OpError` is an `output.Error` at the apply file, the op's line, the op's own code. After a real commit `a.noteWrite(id)` per written issue; a dry run never warns. Run `go run ./cmd/gendocs`. Depends on item 10.
**Regression guard.** Yields to `adr-0017-decision` (a holder writing a different Issue is warned too): noteWrite per id, still one warning per batch. An OpError keeps the issue file's `path:line: message` after `operation <n> (<op>): ` via the verbs' failure mappers.
- A commit-time `tree.ErrChanged` is worded by a new exported nodestore helper over `applyChangedFailure` (errors.go:120), mapped through `kindFailure`; a create whose file now exists maps to exists.
**Files:** internal/cli/apply.go; internal/cli/apply_test.go; internal/cli/root.go; internal/nodestore/errors.go; docs/site/reference/cli/
**Read first:** internal/cli/root.go — newRootCommand, emit, noteWrite, lockWarning, writesOnly, withKV; internal/cli/node_edit.go — written, writeFailure, readTextInput; internal/nodestore/errors.go — applyChangedFailure, changedError; internal/cli/errors.go — kindFailure;
internal/cli/write_test.go — runCLIWith; internal/cli/lock_warn_test.go — newLockWarnStore, lockWarning; internal/cli/help_text_test.go — TestHelpText_EveryCommand_SetsLongAndExample, TestHelpText_EveryCommand_PrintsExamplesBeforeShortRules;
internal/clidoc/reference_test.go — TestReferenceUpToDate
**Tests:** `apply_test.go` through `runCLI`: three creates with deps in one call; an invalid op anywhere leaves `.capcom/` byte-identical, naming its position and line; an issue-file failure keeps its `path:line: message`; `--dry-run` text (or JSON without etag) equals the real run's but for the dry-run line, writing nothing; stdin via `-` through `runCLIWith`; lock purpose a with a batch writing a and b warns once.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'Apply' ./internal/cli/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`
**Commit:** `feat(cli): add capcom apply for all-or-none batches of issue writes`

## 12. Document the apply file and the apply kind

**What:** Recast at the regression check (2026-10-31).
**Goal:** `docs/site/reference/apply-file.md` documents the apply file format, `docs/site/reference/json-output.md` documents kind `apply`, and the generated CLI reference includes `capcom apply`.
**Approach (assumed at the header base):** A page with the site frontmatter (`title`, one-sentence `description`, `sidebar: { order: <n> }`, no body H1): ops and keys, scalar-or-list rules, inline-only bodies, all-or-none, `--dry-run`, the if-match rule, the error form, a three-issue example. json-output.md gains `apply` in the kind list and a `data` example. Depends on item 11.
**Regression guard.** No gendocs run (item 11 runs it). The page states: if-match is allowed on update, dep add, close and log (apply-only on the last three), compared with the file as it was on disk before the batch; create takes none, and if-match on an issue created earlier in the batch is refused.
- ``grep -q '`apply`'`` checks the kind (the base has "apply" only in prose).
**Files:** docs/site/reference/apply-file.md; docs/site/reference/json-output.md
**Read first:** docs/site/reference/json-output.md — envelope table kind row, Errors, The --kv format; docs/site/reference/file-format.md — frontmatter; .capcom/knowledge/docs-layout.md — site page rules;
internal/clidoc/reference_test.go — TestReferenceUpToDate; docs/site/index.md — Next steps
**Tests:** `TestReferenceUpToDate` in `internal/clidoc`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`
- ``grep -q '`apply`' docs/site/reference/json-output.md``
- `test -f docs/site/reference/apply-file.md`
- `grep -q '^sidebar:' docs/site/reference/apply-file.md`
**Commit:** `docs(reference): document apply files and the apply output kind`

## 13. Advertise apply in the agent guide

**What:**
**Goal:** The Managed block capcom writes into agent guides lists `capcom apply <file>: file a linked set of issues in one call, all or none`, and this repo's `AGENTS.md` carries the refreshed block.
**Approach (assumed at the header base):** Add the line after `capcom create` in `internal/agents/block.go`'s command list; update `block_test.go`; refresh `AGENTS.md` with `go run ./cmd/capcom agents`. Depends on item 11.
**Regression guard.**
- Every doc copy changes with it (`grep -rn 'file a new issue' docs/site plugins`): docs/site/agents/managed-block.md, docs/site/reference/file-format.md's Managed block list, plugins/capcom/skills/capcom/SKILL.md's command list, as 3f3e68d and d81a036 did.
- Installed capcom 0.12.1 renders the old block (`capcom check` flags it; an installed `know add/update/mv/rm` rewrites it, as 0d87142 reverted d81a036): from here run check and Note writes as `go run ./cmd/capcom …` until a release; re-run `grep -q 'capcom apply <file>' AGENTS.md` at closeout.
**Files:** internal/agents/block.go; internal/agents/block_test.go; AGENTS.md; docs/site/agents/managed-block.md; docs/site/reference/file-format.md; plugins/capcom/skills/capcom/SKILL.md
**Read first:** internal/agents/block.go — loopClosing, render; internal/agents/block_test.go — goldenBlock, TestRender_Notes_MatchesGolden, preContextBlock (historical blocks stay as rendered); internal/check/agents.go — agentsBlock; internal/cli/know_add.go — syncAgents;
internal/cli/agents_test.go — TestAgents_RenamedDoingStatus_ClaimsWithStart; docs/site/agents/managed-block.md — verbatim block; docs/site/reference/file-format.md — The Managed block in AGENTS.md;
plugins/capcom/skills/capcom/SKILL.md — command list
**Tests:** block_test asserts the line.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/agents/`
- `grep -q 'capcom apply <file>' AGENTS.md`
- `grep -q 'capcom apply' docs/site/agents/managed-block.md`
**Commit:** `feat(agents): list capcom apply in the agent guide`
