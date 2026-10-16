# capcom ui plan

**Goal:** Build `capcom ui [issues|ready|knowledge] [id]`, the terminal UI of `docs/design/tui.md`:
Ready, Issues and Knowledge tabs with markdown detail, live refresh, light writes through a shared
ops package guarded by ETag and the Tree lock, mouse, theme from a per-user config, and its docs.
**Date:** 2026-10-16
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 9aa5428

**Sources:**
- `docs/design/tui.md`
- `.capcom/issues/capcom-ui.md` and `.capcom/issues/capcom-ui/*.md` (each child's `## Acceptance`)
- `.capcom/knowledge/decisions/adr-0011/`, `adr-0012/`, `adr-0013/` (and their parent Notes)
- `/home/airic/repos/apogee/internal/tui/` (`markdown.go`, `mdtable.go`, `wrap.go`, `width.go`, `listsurface.go`)

**Ratified design calls** (user, 2026-10-16):
- **Outline code:** `visibleTree` and the issue/Note outline builders move from `internal/cli` to a new `internal/outline`; CLI and UI both call it.
- **Links:** followable links are issue/Note ids only: markdown links whose target is an id, inline-code spans exactly matching an existing id, and parent/deps/refs naming an id. URLs and file refs are styled but inert.
- **Dep view:** `d` swaps the detail pane to a "waits on" / "blocks" list with its own cursor; Enter selects that issue in the Issues tab; Esc or `d` returns. Body links are mouse-only.
- **Conflict UX:** reload, then prompt `changed on disk: <fields> — run <action> again? (y/n)` naming the changed frontmatter fields plus `body`/`log`; y re-runs on the new ETag, n cancels.
- **Status roles:** colour override keys name status categories: `status-todo`, `status-doing`, `status-waiting`, `status-done`, `status-dropped`.
- **Detail scroll:** PgUp/PgDn and ctrl+u/ctrl+d scroll the detail pane from anywhere; the sidebar keeps focus; in the narrow detail view j/k/arrows also scroll.
- **Tree start:** trees open collapsed to the top level; ancestors of a selected id are expanded.
- **Claim status:** `a` sets `config.DefaultStatus(config.CategoryDoing)` (ratified 2026-10-15 for the Managed block).
- **Closed issues:** a `status:<name>` filter naming a closed-category status (e.g. `status:done`) includes closed issues, shown with their ancestors; text and label filters keep default visibility; no toggle key.
- **Refresh writes:** `Index.Refresh` runs the full incremental refresh every call, as `Open` does, membership writes included; no write-skipping.

**Standing requirements:**
- skills: coding-standards
- Tests: `GOMEMLIMIT=2GiB`, one package at a time; `-race` only on a single package, never `./...` (AGENTS.md).
- `internal/ui` tests never import `internal/cli` (import cycle); they build stores with their own harness.
- After any change to a command's help or flags: `go run ./cmd/gendocs` and commit the result.
- A deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- Everything under "Out of scope for v1" in `docs/design/tui.md`.
- New `--if-match` flags on close, reopen, log or dep (ops takes one; the CLI passes "").
- Closing `capcom-ui` itself: its real-terminal check on macOS and Windows is manual.
- Any version, tag or CHANGELOG heading change.

**Regression check (2026-10-16, 9aa5428):**
- 1: guard folded (ops ChangeError/located-Written contract, cumulative NewlyReady, update via EditIssue, cli wrappers kept); yields to adr-0012-decision
- 2: guard folded (item 1's contract, doc.go file map, helper callers, comment rule)
- 3: recast (no-change Refresh writes nothing to index.db; racy-clean reading of the Goal)
- 3 (re-check): recast (ratified Refresh writes call: full refresh every call, no write-skipping, supersedes the first round's skip; changed includes walk-claims differences); yields to internal/index/index.go Report doc (warnings on every refresh)
- 4: guard folded (outline tests moved, know show/progressOf, start/maxDepth/More API, doc.go, unfiltered cli test)
- 6: guard folded (doc.go, Long/Example, Go 1.25 docs, hermetic cli tests, unfiltered cli test)
- 7: guard folded (lock via Overview, user config path resolved in internal/ui, unfiltered cli test)
- 8: recast (observe kept, ModeReportMsg forwarded; ported-test scope; glyphBullet, testTheme)
- 8 (re-check): guard folded (exported Measure with Observe/Method, vs16Warning and paintMethods ported, apogee doc citations rewritten, x/ansi direct)
- 9: guard folded (ported tests moved to the new Render signature)
- 10: guard folded (IsID over a session.do snapshot, IssueReader, parse failure shown)
- 11: guard folded (hidden-id reveal rule, entries with closed included, progress from all children)
- 12: guard folded (hidden Note follows item 11's rule)
- 13: guard folded (closed dep follows item 11's rule, missing row message)
- 14: recast (ratified Closed issues call; filter line takes every key; Esc and dep-view Enter scope)
- 15: recast (config.yaml reload, schema-version stop, filter re-applied on reload)
- 15 (re-check): guard folded (Index and Tree lock re-opened on the new store, harness backdates files, failed config re-open keeps the old session)
- 16: guard folded (full frontmatter diff with `file` fallback, failed lock read asks, grep pattern)
- 17: guard folded (closed route via status filter, UI messages spelled, blank reason/log refused); yields to internal/cli/update.go:145-147
- 18: guard folded (edited file's check problem first, fake editor argv)
- 19: guard folded (mouse gated by prompts, hidden-id links, wheel moves selection, Files)
- 20: guard folded (textinput styles themed, heading role test, NoColor option, goldenANSI)
- 21: guard folded (shift-drag and closed route documented, every tui.md claim checked, file-format and docs-layout intro)

## 1. ops package: status, claim, close, reopen, log — ✅ DONE (2026-10-16)

NOTES (2026-10-16): internal/cli/update.go is listed in Files but needed no edit: `capcom update` already writes through a.editIssue, which now wraps ops.EditIssue with applyUpdate as its change (one write, one if-match).
NOTES (2026-10-16): the batch closer is `ops.Closer` (`Ops.NewCloser`, `Closer.Close`); `Ops.Close` is a one-issue Closer. Closer.Close's NewlyReady leaves out ids an earlier close of the batch already named, so the CLI loop no longer dedupes.
NOTES (2026-10-16): the CLI types a `*ops.ChangeError` by statusChangeFailure when it wraps an `*issue.StatusError`, else by kindFailure, then fills the path; for close this rewords a non-ErrAlreadyClosed StatusError, which is unreachable since targetStatus only passes closed-category statuses.
NOTES (2026-10-16): `a.appendLog` in log.go deleted as dead code; its author-plus-AppendLog logic now lives in ops.AppendLog.
NOTES (2026-10-16): consequential edit — internal/cli/root.go: made necessary by the format-raise warning moving from editNode to app.written
NOTES (2026-10-16): consequential edit — internal/cli/related.go: made necessary by the format-raise warning moving from editNode to app.written
NOTES (2026-10-16): consequential edit — internal/cli/doc.go: made necessary by node_edit.go now holding written, the CLI side of the ops writes

**What:**
**Goal:** `internal/ops` offers SetStatus, Claim, Close, Reopen and AppendLog on one issue, each
taking an if-match ETag and returning a typed result (Close also returns the ids it made ready);
`capcom update --status`, `close`, `reopen` and `log` run through it with unchanged output and exit
codes, and `internal/ops` imports neither `internal/cli` nor `internal/cli/output`.
**Approach (assumed at the header base):** one deep module: `ops.New(s *store.Store, x *index.Index)
*Ops`. Move the core of `editNode[T]`/`editIssue` and `issueResolver` from
`internal/cli/node_edit.go` into ops (`EditIssue(ref, ifMatch, change)` returning
`nodestore.Written`). Methods: `SetStatus(ref, ifMatch, status)` (`issue.SetStatus`);
`Claim(ref, ifMatch)` = SetStatus with `cfg.DefaultStatus(config.CategoryDoing)`, an error when none
is configured; `Close(ref, ifMatch, status, reason, at) (CloseResult{Written, NewlyReady []string},
error)` (`issue.Close` plus `issue.NewlyReady`, the computation `closeOne`/`issueNodes` do today);
`Reopen(ref, ifMatch, status, reason, at)` (`issue.Reopen`, `gitx.AuthorName`); `AppendLog(ref,
ifMatch, text, at)`. Errors return unwrapped (`*nodestore.ConflictError`, `*issue.StatusError`, tree
errors). The CLI keeps flag parsing, the multi-arg loop, `alreadyClosed`, `requireReason`,
`targetStatus`, the `beforeCloseEdit` hook, `kindFailure`/`resolveFailure` mapping and
`warnRaisedFormat` (fed from `Written.Effects`).
**Regression guard.** ops' edit contract returns the located `nodestore.Written` (path known even on failure) and wraps an error returned by the change func in a typed `*ops.ChangeError{Path, Err}` (Unwrap to Err), distinct from store errors, so the CLI keeps today's wording and path for change refusals exactly as editNode does; item 2 reuses this contract unchanged. (This replaces the Approach's "Errors return unwrapped": the CLI applies `statusChangeFailure` plus the path fill to `*ops.ChangeError` only, `kindFailure` to the rest.) NewlyReady is cumulative across one batch: ops offers a closer taking the nodes read once plus the ids already closed in the batch, and the CLI's multi-arg loop uses it, so `close a b` output is unchanged (one copy). `capcom update` calls `ops.EditIssue` with `applyUpdate` as its change (one write, one if-match); SetStatus and Claim exist for the UI. `a.editIssue`, `editNode` and `a.issueNodes` stay in cli as wrappers over ops in this item, so `know_update.go`, `related.go`, `dep.go` and `show.go` build untouched. Yields to ADR 0012 (`adr-0012-decision`: "A rule added to closing (a reason check, a new side effect) lands in one place"): ops.Close and ops.Reopen refuse a blank (TrimSpace) reason and ops.AppendLog a blank text with typed errors; the CLI keeps its up-front flag checks (`requireReason`, log's empty-text check) for today's wording and exit codes.
**Files:** internal/ops/ops.go, internal/ops/ops_test.go, internal/cli/node_edit.go, internal/cli/update.go, internal/cli/close.go, internal/cli/reopen.go, internal/cli/log.go
**Read first:** internal/cli/node_edit.go — editNode, editIssue; internal/nodestore/edit.go — Kind.Edit; internal/cli/close.go — closeResult, closeOne;
internal/cli/update.go — applyUpdate, statusChangeFailure; internal/cli/close_test.go — TestClose_SeveralIDs_ReportsEachReadiedIDOnce
**Tests:** existing `internal/cli` tests pass unedited; `internal/ops/ops_test.go` only for cases the
CLI cannot reach: Close, Reopen and AppendLog with a stale if-match return `*nodestore.ConflictError`
and leave the file byte-identical; Claim with no doing-category status configured errors; a change-func
refusal returns `*ops.ChangeError` carrying the file's path; a batch close's NewlyReady counts the batch's
earlier closes; a blank reason or log text is refused with the file byte-identical.
**Acceptance:**
- `go build ./... && go vet ./internal/ops/ ./internal/cli/`
- `GOMEMLIMIT=2GiB go test ./internal/ops/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `go list -deps ./internal/ops/ | grep -c 'internal/cli'` prints 0
- this item's diff touches no `internal/cli/*_test.go`
**Commit:** `refactor(ops): move the issue write glue out of internal/cli into internal/ops`

## 2. ops package: dependencies and node edits — ✅ DONE (2026-10-16)

NOTES (2026-10-16): re-derived from the assumption that ops' generic edit was still the unexported cli editNode: item 1 had exported it as `ops.Edit`, which the Acceptance grep `\.Edit(` matches at node_edit.go, so it is renamed to the item's named `ops.EditNode` (EditIssue and cli editNode call it); related add/rm and know update already wrote through ops (a.editIssue / editNode) and needed only the helper moves.
NOTES (2026-10-16): referenceList, resolveIssueID and withoutEntries moved to ops as `Ops.ReferenceList`, `Ops.ResolveIssueID` and `WithoutEntries` (related.go, know_update.go and create.go call them, no copies); loadDepGraph stays in cli for dep list, dep graph and related, while ops builds its own graph via the new `Ops.DepGraph` over `IssueNodes`.
NOTES (2026-10-16): dep add/rm type ops' outcome through a new cli helper `depWritten`: any `*reflist.Refusal` (argument or edit, the not-a-dep one arriving inside a `*ops.ChangeError`) goes to refusalFailure at the dependent's located path, everything else through app.written.
NOTES (2026-10-16): internal/cli/root.go and internal/cli/close.go are listed in Files but needed no edit: no comment there names editNode, editIssue or loadDepGraph.
NOTES (2026-10-16): updateNote's comment now names ops.EditNode and attributes the format-raise warning to app.written (item 1 moved it there from editNode).

**What:**
**Goal:** `capcom dep add`/`dep rm` run through `ops.AddDep`/`ops.RemoveDep` (if-match taking,
typed result), and every remaining `internal/cli` edit of an issue or Note (`related add/rm`,
`know update`) goes through ops' generic edit; no non-test file in `internal/cli` calls
`nodestore.Kind.Edit` directly. Output and exit codes unchanged. Depends on item 1.
**Approach (assumed at the header base):** move the dep-edit glue of `internal/cli/dep.go`
(`resolveIssue` use, the `loadDepGraph` cycle/self checks, `referenceList(...).Add/Remove`,
`withoutEntries`) into `ops.AddDep(ref, ifMatch, depRef)` / `RemoveDep`, returning the refusals as
typed errors the CLI maps through `refusalFailure`; `loadDepGraph` stays in cli where `dep list`
and `show` need it, or moves to ops if both sides share it (one copy only). Export ops' generic
`EditNode[T]` and call it from `related.go` and `know_update.go` (`updateNote`), keeping
`noteResolver` and the `WithSync(a.syncAgents)` Kind the CLI builds.
**Regression guard.** reuse item 1's ChangeError/located-Written contract; any item that adds, removes or renames a non-test internal/cli/*.go updates the `internal/cli/doc.go` file map, and its Acceptance runs the unfiltered `GOMEMLIMIT=2GiB go test ./internal/cli/` (TestDocFileMap_MatchesDirectory, TestHelpText_*). ops.AddDep/RemoveDep return the resolved dependent's located Written beside a reflist refusal, so the CLI words and places it as today (TestDepRm_NotADep_FailsNotFound). Before moving `withoutEntries`, `referenceList`, `loadDepGraph` or `resolveIssueID`, grep every caller: a helper still called in cli (related.go, know_update.go, dep_list.go, dep_graph.go, create.go) is exported from ops and called there, never copied; create.go is edited if `resolveIssueID` moves. Every cli comment naming editNode, editIssue or loadDepGraph (`grep -n 'editNode\|editIssue\|loadDepGraph' internal/cli/*.go`) states the ops path.
**Files:** internal/ops/ops.go, internal/ops/deps.go, internal/ops/ops_test.go, internal/cli/dep.go, internal/cli/dep_graph.go, internal/cli/related.go, internal/cli/know_update.go, internal/cli/node_edit.go, internal/cli/doc.go, internal/cli/root.go, internal/cli/close.go, internal/cli/create.go (if `resolveIssueID` moves)
**Read first:** internal/cli/dep.go — newDepAddCommand, newDepRmCommand, withoutEntries; internal/cli/dep_graph.go — loadDepGraph, referenceList, resolveIssueID;
internal/cli/errors.go — refusalFailure; internal/cli/doc_test.go — TestDocFileMap_MatchesDirectory
**Closes:** ui-ops-package
**Tests:** existing `internal/cli` tests pass unedited; an ops test that AddDep with a stale if-match
returns `*nodestore.ConflictError`; a RemoveDep refusal carries the dependent's path.
**Acceptance:**
- `go build ./... && go vet ./internal/ops/ ./internal/cli/`
- `GOMEMLIMIT=2GiB go test ./internal/ops/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `grep -n '\.Edit(' internal/cli/*.go | grep -v _test.go` prints nothing
**Commit:** `refactor(ops): move dependency and node edits into internal/ops`

## 3. Export an incremental index refresh — ✅ DONE (2026-10-16)

NOTES (2026-10-16): the walk-claims comparison lives in replaceWalkClaims (now returning isChanged) via a new storedWalkClaims read before the delete; Report gains an unexported walkClaimsChanged field and an isChanged method that Refresh returns.
NOTES (2026-10-16): no CHANGELOG entry: Refresh is an internal API with no user-facing change until capcom ui ships.
**What:** Recast at the regression check (2026-10-16).
**Goal:** `(*index.Index).Refresh() (bool, error)` re-syncs an open index incrementally and reports
true when any issue or Note was added, changed or removed; with no file changed it reports false
and re-parses no indexed file (Report.parsed empty).
**Approach (assumed at the header base):** wrap the unexported `refresh(false)` in
`internal/index/refresh.go`; changed = rebuilt, or any file parsed (`indexFile`) or forgotten
(`forget`) per the `Report`'s unexported `rebuilt`/`parsed`/`removed`, or the walk claims differ from the stored ones. Keep the racy-clean rule
(`isDirty`, `racyGranularity`) as is. Update the `Index` doc comment: still not safe for concurrent
use; callers serialise.
**Regression guard.** Ratified design call (user, 2026-10-16), **Refresh writes:** `Index.Refresh` runs the full incremental refresh every call, as `Open` does, membership writes included; no write-skipping. This supersedes the first round's decision to skip the membership/claims rewrite on a no-change Refresh. Refresh reports changed=true when any file was parsed or removed, the index was rebuilt, or the walk claims (claim-only files: oversize, non-regular, below an orphan dir) differ from the stored ones; otherwise false. `Report.Warnings` keep today's behaviour: the item yields to the `Report` doc in internal/index/index.go (a problem is reported on every refresh until its file changes). The Goal's "no file changed" means no file changed since a refresh that recorded it clean: a racy-clean file is re-read once and counts as changed; the no-change tests open the index with `laterClock()` (index_test.go), as TestRefresh_OneFileEdited_ReparsesOnlyThatFile does.
**Files:** internal/index/refresh.go, internal/index/index.go, internal/index/index_test.go
**Read first:** internal/index/refresh.go — (*Index).refresh, refresher.sync, refresher.replaceWalkClaims, recordedFile.isDirty; internal/index/walk.go — walkIssues;
internal/index/index.go — Report; internal/index/index_test.go — laterClock, TestRefresh_DuplicateName_WarnsAndExcludesBoth
**Closes:** ui-index-refresh
**Tests:** in `index_test.go` beside `TestRefresh_*`: an added, a changed and a removed issue file
and a changed Note each make Refresh return true; an added oversize issue file makes Refresh return true; no change (index opened with `laterClock()`) returns false with `parsed` empty;
membership warnings (e.g. a duplicate name) survive a no-change Refresh; a
same-second edit (`withClock`, as `TestRefresh_SameSizeEditInOneMtimeTick_IsSeen`) is re-parsed.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/index/`
**Commit:** `feat(index): export Refresh on an open index, reporting whether anything changed`

## 4. Shared outline package — ✅ DONE (2026-10-16)

NOTES (2026-10-16): `outline.Issues(entries, isAll)` and `outline.Notes(notes, isBackgroundIncluded)` return a `*outline.Tree[E]` (generic `NewTree`, `Children` = the all-children accessor, `Shown`, `Rows(start, maxDepth) []Row[E]`) rather than `[]Row` directly, so the same value serves the rows and the all-children accessor that list's progressOf and know show --with-children call; Row is generic over its Entry (index.Entry or index.NoteEntry) with ID, ParentID (the outline parent, "" at the top level), Depth, HasChildren (has shown children), More, MoreDepth; `outline.UnlimitedDepth = 0` beside cli's unchanged unlimitedDepth.
NOTES (2026-10-16): internal/cli/outline_test.go is deleted: all three of its tests (and newTestTree) moved to internal/outline/outline_test.go under the same names, adapted to Children/Shown/Rows; new tests cover closed subtrees hidden unless --all, a hidden start shown, a depth cut's More/MoreDepth, Note id order and background subtrees hidden.
NOTES (2026-10-16): internal/cli/issue_render.go is listed in Files but needed no edit: progressOf stays as it is and list.go now passes it `tree.Children(id)`. The cli helpers issueID, issueParentID, noteID, noteParentID and newNoteTree are removed (their only caller was the moved grouping).
NOTES (2026-10-16): consequential edit — internal/cli/doc.go: made necessary by moving the grouping out of know.go and the tree and walker out of outline.go (both file-map lines reworded).
**What:**
**Goal:** the rows, order, depth and visibility of the `capcom list --tree` issue outline and the
`capcom know tree` Note outline are computed in `internal/outline` as data, and both commands render
from it with unchanged output.
**Approach (assumed at the header base):** move `visibleTree[E]` (`newVisibleTree`, `walk`, `all`)
from `internal/cli/outline.go`, the row selection of `issueTreeResult` (`internal/cli/list.go`,
default visibility `isAll || !Category.IsClosed()`, progress counts from `progressOf`) and
`newNoteTree` / `(a *app) outline` (`know.go`, `know_tree.go`, background Notes hidden via
`knowledge.IsBackground`) into `internal/outline`: `outline.Issues(entries, isAll)` and
`outline.Notes(notes, ...)` return `[]Row{ID, ParentID, Depth, HasChildren, ...}` plus the
entry. Rendering (`renderOutline`, text lines) stays in cli. No new rule: one copy of each.
**Regression guard.** same internal/cli doc.go file-map rule as item 2 if outline.go is removed or renamed, and run the unfiltered cli package test. `internal/cli/outline_test.go`'s TestWalkOutline_DepthCutsAndCountsShownChildren, TestOutline_VisibleTree_GroupsAllAndShown and TestOutline_VisibleTreeWalk_StartsAtTopOrAtStart (with `newTestTree`) move to `internal/outline/outline_test.go`. `know show --with-children` (know_show.go) keeps every child, hidden included, and `progressOf` (issue_render.go) stays in cli: outline exposes an all-children accessor both call. The outline API takes the start (shown even when hidden) and maxDepth, and Row carries More and MoreDepth; outline has its own unlimited-depth constant, 0 like cli's `unlimitedDepth`.
**Files:** internal/outline/outline.go, internal/outline/outline_test.go, internal/cli/outline.go, internal/cli/list.go, internal/cli/know.go, internal/cli/know_tree.go, internal/cli/outline_test.go, internal/cli/know_show.go, internal/cli/issue_render.go, internal/cli/doc.go
**Read first:** internal/cli/outline.go — visibleTree, newVisibleTree, walk, walkOutline; internal/cli/list.go — issueTreeResult;
internal/cli/know_show.go — newKnowShowCommand; internal/cli/issue_render.go — progressOf; internal/cli/outline_test.go — newTestTree
**Tests:** existing `list --tree`, `know tree`, `know show` and `show` cli tests pass unedited, except the three
`internal/cli/outline_test.go` tests, which move to `internal/outline/outline_test.go`; `outline_test.go`: a hidden
node hides its subtree, sibling order, `--all` shows closed issues, a hidden start is shown, a depth cut sets More/MoreDepth.
**Acceptance:**
- `go build ./... && go vet ./internal/outline/ ./internal/cli/`
- `GOMEMLIMIT=2GiB go test ./internal/outline/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
**Commit:** `refactor(outline): move the issue and Note outlines out of internal/cli`

## 5. Per-user config package — ✅ DONE (2026-10-16)

NOTES (2026-10-16): consequential edit — docs/design/tui.md: made necessary by the Roles list (the Theme example named the role `status-active`, which Load now warns on as unknown; it now names `status-doing`)
NOTES (2026-10-16): Load returns Default() alongside any *Error, so the caller runs on it directly; a null `ui`, `theme` or `colors` value means the default, while a null colour (e.g. an unquoted `#fff`, which YAML reads as a comment) is an error whose message says so
NOTES (2026-10-16): malformed-YAML errors carry the line yaml.v3 reports, lifted out of its message into `path:line: message`; an unreadable existing file is an *Error (line 0)
NOTES (2026-10-16): added TestPackage_ImportedOnlyByUI, which walks the module and fails when any package but internal/ui imports internal/userconfig, guarding the item's "no package but internal/ui imports it" clause
**What:**
**Goal:** `internal/userconfig` loads `<os.UserConfigDir>/capcom/config.yaml` per ADR 0011: the `ui`
settings with defaults filled in (theme `auto`, no colour overrides); a missing file is created with
every key as a commented default; an unknown key or colour role is a warning; malformed YAML, a
theme outside `auto|dark|light` or a colour that is not `#rgb`, `#rrggbb` or an integer 0-255 is an
error printed `path:line: message`; an unwritable directory is a warning. No package but
`internal/ui` imports it, and no existing command touches the path.
**Approach (assumed at the header base):** `Path() (string, error)`; `Load(path) (Config,
[]Warning, error)` where `Config{UI{Theme string; Colors map[string]string}}`; decode into a
`yaml.v3` `yaml.Node` and walk known keys to get line numbers. Exported `Roles`: `accent`,
`border`, `muted`, `link`, `selection`, `heading`, `code`, `status-todo`, `status-doing`,
`status-waiting`, `status-done`, `status-dropped`. On error the caller runs on defaults. A second
Load leaves the file byte-identical.
**Files:** internal/userconfig/userconfig.go, internal/userconfig/template.go, internal/userconfig/userconfig_test.go, internal/cli/userconfig_test.go
**Read first:** .capcom/knowledge/decisions/adr-0011/ — adr-0011-decision; internal/cli/testharness_test.go — runCLI, setHermeticGitEnv;
docs/design/tui.md — Theme section; go.mod — gopkg.in/yaml.v3
**Closes:** ui-user-config
**Tests:** defaults; first Load creates the all-commented file, second leaves it unchanged; unknown
key and unknown role warn; bad YAML and wrong-typed values error with the line; unwritable directory
warns (skipped on Windows and as root). `internal/cli/userconfig_test.go`: with `XDG_CONFIG_HOME`,
`HOME` and `AppData` set to temp dirs, `init`, `create`, `update`, `close`, `list`, `ready`, `show`,
`know tree` and `check` via `runCLI` leave those dirs empty.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/userconfig/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run UserConfig`
**Commit:** `feat(userconfig): add the per-user config file for capcom ui`

## 6. capcom ui command, session and test harness — ✅ DONE (2026-10-16)

NOTES (2026-10-16): the concurrent `session.do` tests live in a new `internal/ui/session_test.go` (not in Files), following the `{source}_test.go` rule; `-run Session` selects them.
NOTES (2026-10-16): `session` holds the store and the index only; the store config travels as `store.Config` on the store, so there is no separate config field behind the mutex.
NOTES (2026-10-16): the TTY check uses `github.com/charmbracelet/x/term` (in the module graph through bubbletea) and the golden harness `github.com/charmbracelet/x/ansi`; `go mod tidy` made both direct requirements, and bubbletea's graph also raised `golang.org/x/sync` (indirect) to v0.21.0.
NOTES (2026-10-16): `newTestStore` keys its files by their path relative to `.capcom/` (`issues/a.md`, `knowledge/n.md`) rather than relative to the issues directory, so later tabs can seed Notes too; it returns a `*testStore` (root, store, index) that `newTestModel` takes.
NOTES (2026-10-16): `Options.ID` is carried into the model but not yet acted on (selection lands with the tab items); `capcom ui` prints no index warnings, since it renders no result through emit.

**What:**
**Goal:** `capcom ui [issues|ready|knowledge] [id]` exists: a non-TTY stdin or stdout exits 1 with an
error, `--json` and an unknown tab are usage errors (exit 2), an id is accepted only after a tab
name; on a terminal it runs a Bubble Tea v2 program from `internal/ui` that quits on `q` and ctrl+c.
`go.mod` requires `charm.land/bubbletea/v2` and `charm.land/lipgloss/v2`, and the six release targets
build with `CGO_ENABLED=0`.
**Approach (assumed at the header base):** `go get charm.land/bubbletea/v2@v2.0.8
charm.land/lipgloss/v2@v2.0.4` (in the module cache), `go mod tidy`; the `go` directive and
`golang.org/x/sys` move only as far as those require. `internal/cli/ui.go` `newUICommand`, annotated
`indexRefresh`, registered in `newRootCommand`; TTY check on stdin and stdout via a term package
already in the module graph. `ui.Run(ui.Options{Store, Index, Tab, ID, In, Out}) error`.
`internal/ui/session.go`: store, index and config behind one `sync.Mutex`; every store/index access
from any `tea.Cmd` goes through `session.do(func(*store.Store, *index.Index) error)` — binding: no
index access outside it. `harness_test.go`: `newTestStore(t, files)` (temp git repo with the
hermetic env `setHermeticGitEnv` uses, the config `capcom init` writes, `store.Open`, `index.Open`),
`newTestModel(t, s, w, h)` sending `tea.WindowSizeMsg`, key helpers building `tea.KeyPressMsg`, and
`golden(t, name, view)` comparing `ansi.Strip` of the view with `testdata/golden/<name>.golden`,
`-update` rewriting.
**Regression guard.** same doc.go file-map rule (ui.go is added) and the unfiltered `GOMEMLIMIT=2GiB go test ./internal/cli/` replaces the `-run UI` line. The `ui` command sets Long and an Example whose lines start `  capcom ui ` (TestHelpText_EveryCommand_SetsLongAndExample). bubbletea v2.0.8, lipgloss v2.0.4 and x/sys v0.46.0 declare go 1.25.0, so README.md and docs/site/getting-started/install.md change "Go 1.24 or later" to "Go 1.25 or later". The cli ui tests run in a temp store (`newIssueStore`, reindex_test.go), never in capcom's own repo; `--json` is rejected in the Args validator (before PersistentPreRunE refreshes the index); the TTY check tests `cmd.InOrStdin()` and `a.stdout` as `*os.File`, never os.Stdin/os.Stdout.
**Files:** go.mod, go.sum, internal/cli/ui.go, internal/cli/ui_test.go, internal/cli/root.go, internal/ui/ui.go, internal/ui/session.go, internal/ui/model.go, internal/ui/harness_test.go, internal/ui/model_test.go, internal/ui/testdata/golden/, docs/site/reference/cli/, internal/cli/doc.go, README.md, docs/site/getting-started/install.md
**Read first:** internal/cli/root.go — prepare, prepareIndex, indexRefresh; internal/cli/doc_test.go — TestDocFileMap_MatchesDirectory;
internal/cli/help_text_test.go — TestHelpText_EveryCommand_SetsLongAndExample; internal/cli/testharness_test.go — runCLI, setHermeticGitEnv;
internal/cli/reindex_test.go — newIssueStore
**Tests:** cli (in a `newIssueStore` temp store): `capcom ui` under `runCLI` exits 1 naming the terminal; `ui --json` and `ui bogus`
exit 2. ui: `q` returns `tea.Quit`; golden of the bare frame at 120x30; concurrent `session.do`.
**Acceptance:**
- `go build ./... && go vet ./internal/ui/ ./internal/cli/`
- `GOMEMLIMIT=2GiB go test ./internal/ui/` and `GOMEMLIMIT=2GiB go test -race ./internal/ui/ -run Session`
- `GOMEMLIMIT=2GiB go test ./internal/cli/` and `GOMEMLIMIT=2GiB go test ./internal/clidoc/`
- `for o in linux darwin windows; do for a in amd64 arm64; do GOOS=$o GOARCH=$a CGO_ENABLED=0 go build -o /dev/null ./cmd/capcom || exit 1; done; done`
**Commit:** `feat(ui): add the capcom ui command on Bubble Tea v2`

## 7. Layout, keys, help and the lock status bar — ✅ DONE (2026-10-16)

NOTES (2026-10-16): re-derived from the assumption that `ui.Options` lives in a listed file — it is in `internal/ui/ui.go`, which gains `UserConfigPath`; `Run` builds the model with `newModel(options)`, which loads the user config.
NOTES (2026-10-16): `internal/cli/ui.go` is unchanged: per the item's regression guard the CLI passes no `UserConfigPath` and `internal/ui` resolves `userconfig.Path()` itself.
NOTES (2026-10-16): the sidebar holds no rows yet, so arrows and `j k h l` move nothing there; up/down/j/k scroll the detail pane in the narrow detail view, and sidebar movement and h/l expand/collapse arrive with the sidebar widgets (items 10–12).
NOTES (2026-10-16): item 6's bare-frame golden and its test are replaced by the `layout-120x30` and `layout-80x24` goldens; `AgeText` gets its own `internal/lock/age_test.go` (not in Files), following the `{source}_test.go` rule.

**What:**
**Goal:** the UI draws the tab bar, sidebar, detail pane and status bar at 90 columns and wider;
below 90 the sidebar fills the screen, Enter opens the detail full width and Esc returns. Keys:
arrows and `j k h l`, Tab and `1 2 3` switch tabs, Enter, Esc, PgUp/PgDn and ctrl+u/ctrl+d scroll
the detail pane, `?` toggles a help screen with the full key map of `docs/design/tui.md` plus
`d deps`, `q` quits. The status bar shows this worktree's Tree lock — `lock: free`,
`lock: held by <holder>: <purpose> (heartbeat <age> ago)`, `lock: stale: <purpose> (heartbeat <age>
ago)` — and key hints. At start the UI loads the user config (creating it) and shows its warnings or
error in the status bar, running on defaults after an error. Depends on items 5 and 6.
**Approach (assumed at the header base):** lock via `lock.ForWorktree(s).Read()` and
`Status.State()` inside `session.do`; the UI holds no token, so any held lock is another session's.
Age text: move `ageText` from `internal/cli/prime.go` into `internal/lock` (exported) and call it
from both. `ui.Options` gains `UserConfigPath` (cli passes `userconfig.Path()`, tests a temp path).
**Regression guard.** run the unfiltered `GOMEMLIMIT=2GiB go test ./internal/cli/` instead of `-run 'Prime|UI'`. The lock is read with `lock.ForWorktree(s).Overview()` (not `Read`, which fails on a corrupt record and creates the lock dir), rendering `Overview.State`/`Overview.Own`, plus the texts `lock: invalid` and `lock: unreadable`; item 15's per-tick read uses the same call. `internal/ui` resolves the path itself with `userconfig.Path()` when `Options.UserConfigPath` is empty (tests set it; cli passes nothing), so item 5's "no package but internal/ui imports it" holds; a Path error becomes a status-bar warning and the UI runs on defaults.
**Files:** internal/ui/model.go, internal/ui/layout.go, internal/ui/keys.go, internal/ui/help.go, internal/ui/statusbar.go, internal/ui/layout_test.go, internal/ui/testdata/golden/, internal/lock/age.go, internal/cli/prime.go, internal/cli/ui.go
**Read first:** internal/lock/overview.go — Locks.Overview, Overview, Status.State; internal/lock/worktree.go — ForWorktree, Worktree.Overview;
internal/cli/prime.go — ageText, addLocks; internal/cli/prime_test.go — TestPrime_Locks_ShowAgeAndStaleFlag
**Closes:** ui-skeleton
**Tests:** goldens: empty layout at 120x30 and 80x24, the help screen; status bar text for a free, a
held and a stale lock (acquired through `internal/lock` in the test store); a corrupt `_main.json` shows
`lock: invalid`; a config with an unknown key shows its warning; first start creates the config file; with
`UserConfigPath` empty and HOME/XDG_CONFIG_HOME unset (Unix only) the status bar warns and the UI runs on defaults.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
- `GOMEMLIMIT=2GiB go test ./internal/lock/` and `GOMEMLIMIT=2GiB go test ./internal/cli/`
**Commit:** `feat(ui): layout, keys, help screen and Tree lock status bar`

## 8. Markdown renderer ported from apogee — ✅ DONE (2026-10-16)

NOTES (2026-10-16): go.mod left unchanged — github.com/charmbracelet/x/ansi is already a direct require (no `// indirect`), so the item's go.mod edit had nothing to do.
NOTES (2026-10-16): clampInt not copied — in apogee it serves only expandTabsInSpans, which is not part of the port, so it would be dead code here; withMarker and its test also stay behind (apogee's chat-marker helper, not in the copy list).
NOTES (2026-10-16): widthAuthority is ported as exported `Measure` with no constructor (its zero value is the painter's WcWidth start); the ported tests are renamed TestWidthAuthority* -> TestMeasure*, and TestTableStreamingDegradesToParagraphs -> TestTableHeaderAloneDegradesToParagraphs; apogee's Model-level width tests stay behind, replaced by the model_test.go mode-2027 test.
NOTES (2026-10-16): ported functions restructured to the coding standards' size/nesting limits without behaviour change — renderInline is table-driven (inlineSpans, matchInlineSpan), Render's fence and table branches moved into fenceBody and renderTableAt, fitColumns' level scan into shrinkLevel; testTheme lives in markdown_test.go; the package doc is in markdown.go (5 files, no doc.go).
NOTES (2026-10-16): table_test.go fixture cell "layout.md" changed to "layout.go" (same width) so the item's `layout\.md` grep prints nothing; the model's markdownTheme is the zero Theme (unstyled roles) until the theme items set its styles.

**What:** Recast at the regression check (2026-10-16).
**Goal:** `internal/ui/markdown` renders headings, fenced code, bullet and numbered lists, GFM pipe
tables, inline code and bold to a width measured in terminal cells (wide characters and ANSI
sequences handled), with apogee's markdown, table, wrap and width tests ported and passing.
Depends on item 6.
**Approach (assumed at the header base):** copy from `/home/airic/repos/apogee/internal/tui/`:
`markdown.go`, `mdtable.go`, the helpers `wrapText`, `hangingPrefixes`, `hangCollapses`,
`expandTabs` (`wrap.go`), `widthAuthority` (`width.go`, without `observe`), `blankLine`, `clampInt`
and the table glyph constants; their tests from `markdown_test.go`, `mdtable_test.go`,
`wrap_test.go`, `width_test.go`. Replace apogee's `theme` with `markdown.Theme{Measure, Bold,
Underline, Heading, Code, CodeBlock, Rule}`. Exported entry: `Render(th Theme, text string, width
int) []string`. No dependency on apogee packages.
**Regression guard.** keep apogee's `widthAuthority.observe(tea.ModeReportMsg)` so the measure switches to grapheme width on mode-2027 terminals; the UI model forwards `tea.ModeReportMsg` to the theme's measure, and the link regions of items 9 and 19 use that same measure. (This replaces the Approach's "without `observe`".) The Goal's ported tests are the tests of the copied functions (TestWrapTextHoldsTheWidthCap, TestWrapTextBreaksInThePaintersMeasure, the widthAuthority tests, observe included); the rails, clip, guttered, hanging-wrap and Model-rendering tests stay behind. `glyphBullet` (apogee theme.go) joins the copied constants, and a test helper `testTheme()` builds `markdown.Theme` from plain lipgloss styles in place of apogee's `newTheme(scheme.X)`.
Re-check (2026-10-16): the measure is exported, `type Measure` with `Observe(tea.ModeReportMsg) Measure` and `Method()`, carried as `Theme.Measure`; the model test reads `Measure.Method()`. The ported test helpers (wrap_test.go) add `vs16Warning` and `paintMethods` (apogee paint_test.go). Every copied comment citing an apogee ADR, layout.md, render.go, doc.go or transcript is rewritten or dropped. go.mod: `github.com/charmbracelet/x/ansi` becomes a direct require (drop `// indirect`).
**Files:** go.mod, internal/ui/markdown/markdown.go, internal/ui/markdown/table.go, internal/ui/markdown/wrap.go, internal/ui/markdown/width.go, internal/ui/markdown/theme.go, internal/ui/markdown/markdown_test.go, internal/ui/markdown/table_test.go, internal/ui/markdown/wrap_test.go, internal/ui/model.go, internal/ui/model_test.go
**Read first:** /home/airic/repos/apogee/internal/tui/width.go — widthAuthority, observe; .../markdown.go — renderMarkdownBody, renderInline;
.../wrap.go — wrapText, hangingPrefixes; .../theme.go — glyphBullet; .../paint_test.go — vs16Warning, paintMethods
**Tests:** the ported tests of the copied functions (width tests, observe included, folded into `wrap_test.go`);
`internal/ui/model_test.go`: a mode-2027 `tea.ModeReportMsg` sent to the model switches the markdown measure to grapheme width (read through `Measure.Method()`).
**Acceptance:**
- `go vet ./internal/ui/markdown/ && GOMEMLIMIT=2GiB go test ./internal/ui/markdown/`
- `grep -rn 'apogee' internal/ui/markdown/*.go` prints only provenance comments
- `grep -nE 'ADR [0-9]|layout\.md|render\.go|doc\.go|transcript' internal/ui/markdown/*.go` prints nothing apogee-specific
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
**Commit:** `feat(ui): port apogee's markdown renderer`

## 9. Markdown links and blockquotes — ✅ DONE (2026-10-16)

NOTES (2026-10-16): re-derived from the assumption that wrap_test.go calls the renderer — it has no Render call at this tree, so it is untouched; markdown_test.go's 13 and table_test.go's 25 Render calls (and the unexported renderInline/renderTable calls, which now take a nil *linkSet) moved to the new signature instead.
NOTES (2026-10-16): table.go and wrap.go edited though not in **Files:** — renderTable must pass the link set to renderInline so table cells carry links, and wrapText closes and reopens a link's zero-width APC markers at each break (balanceLinkMarkers) so a wrapped link records one region per line; regions are read off the finished lines and the markers stripped before Render returns.
NOTES (2026-10-16): link text is shown literally (no inline markup inside it), an inert link shows its text only (the URL is not printed beside it), `<scheme://...>` autolinks render Inert, and bare URLs in prose stay plain text; blockquote lines take heading, list or paragraph text (no fenced code or table inside a quote) and drop their bars when the width cannot hold them plus one column.
NOTES (2026-10-16): no CHANGELOG entry — the renderer is not wired into the detail pane until item 10. Pre-existing: a Theme whose Bold underlines would make lipgloss style a table header rune by rune, breaking any escape inside it (inline SGR and now link markers alike).

**What:**
**Goal:** `markdown.Render` returns the lines plus each followable link's target and screen region
(line, column and width in cells): markdown links whose target is an id and inline-code spans whose
text is an id, as decided by an `IsID func(string) bool` option, are link-styled and recorded; a link
wrapped over lines records one region per segment; URLs and other targets render styled but are not
recorded. `> ` blockquotes render with a bar prefix and wrap inside it. Depends on item 8.
**Approach (assumed at the header base):** `Render(th, text, width, Options{IsID}) Rendered{Lines
[]string; Links []Link{Target string; Line, Col, Width int}}`; `Theme` gains `Link`, `Inert` and
`Quote` styles; regions measured with `Theme.Measure`, never `len`.
**Regression guard.** the ported tests that call the renderer (`table_test.go`'s 26 calls and `wrap_test.go`'s one, apogee's `renderMarkdownBody`) move to the new `Render` signature in this item, so `go test ./internal/ui/markdown/` compiles.
**Files:** internal/ui/markdown/markdown.go, internal/ui/markdown/links.go, internal/ui/markdown/theme.go, internal/ui/markdown/links_test.go, internal/ui/markdown/markdown_test.go, internal/ui/markdown/table_test.go, internal/ui/markdown/wrap_test.go
**Read first:** /home/airic/repos/apogee/internal/tui/markdown.go — renderInline, renderMarkdownBody; .../wrap.go — wrapText; .../mdtable.go — renderTable, wrapTableCell;
.../width.go — widthAuthority.Width; .../mdtable_test.go — renderMarkdownBody callers
**Closes:** ui-markdown
**Tests:** link region after wide (CJK) characters; a link wrapped over two lines; an inline-code id;
a URL rendered but unrecorded; a nested blockquote with a list.
**Acceptance:**
- `go vet ./internal/ui/markdown/ && GOMEMLIMIT=2GiB go test ./internal/ui/markdown/`
**Commit:** `feat(ui): links and blockquotes in the markdown renderer`

## 10. Ready tab and issue detail pane — ✅ DONE (2026-10-16)

NOTES (2026-10-16): re-derived from "the key handling lives in model.go" — item 7 put it in internal/ui/keys.go (handleKey, openTab), so the sidebar's cursor keys and openTab's detail reload landed there; openTab now returns a tea.Cmd and clears the panes on a tab switch.
NOTES (2026-10-16): re-derived from "internal/cli/show.go — issueFromFile reads the file": show reads through parseNodeFile/readNodeFile (update.go); the UI reads with nodestore.IssueReader(cfg).Read and takes Title/Status/Priority/Labels/Deps from the parsed file and ParentID from the index entry, as issueFromFile does.
NOTES (2026-10-16): listCursor.key ported with capcom's move keys (up/k, down/j) and only the listSwallowed/listUnclaimed verdicts: Enter and Esc stay the model's layout keys (narrow detail open/back, item 7), so apogee's listCloses/listAccepts and ctrl+p/ctrl+n are not ported. wheel is ported per the plan but has no caller until item 19; clampInt is added here (item 8 left it out of the markdown package).
NOTES (2026-10-16): the sidebar list stops at its ends (listStopsAtEnds) and scrolls to keep the selection on screen (readyTab.top); the selected row is marked with reverse video.
NOTES (2026-10-16): unspecified edge filled: `capcom ui ready <id>` naming an id that resolves to no issue, or to one the queue does not hold, keeps the first row selected and shows `select <id>: <error>` or `<id> is not in the Ready queue` in the status bar (new model.notice, rendered by statusbar.go).
NOTES (2026-10-16): the header is rendered through markdown.Render with the body (`# <title>`, `status: <s> · P<n>`, `labels:`, `parent:`, `deps:`), parent and deps as code spans so existing ids become links; a parse failure shows the diagnose finding (`<path>:<line>: <message>`) under a header from the index entry. A tea.ModeReportMsg drops the cached rendering, since a new width measure can wrap differently.
NOTES (2026-10-16): harness_test.go: startModel now drives every command Init returns (tea.Batch included) and the messages they return, with new press/drive helpers, made necessary by Init batching the lock read with the Ready load.
NOTES (2026-10-16): consequential edit — internal/ui/layout_test.go: made necessary by the Ready tab now filling the detail pane (comment on the scroll test's 100 synthetic rows).
NOTES (2026-10-16): layout-120x30 / layout-80x24 goldens regenerated: their one-issue store now shows that issue in the Ready sidebar and detail pane.
NOTES (2026-10-16): internal/ui now has 10 non-test files with its package map still in ui.go's package doc only; a doc.go file map with a structural test may be due (coding-standards), not added here.

**What:**
**Goal:** the Ready tab lists the queue `capcom ready` prints with no flags, in its order, each row
`id  status  P<n>  title`; an empty queue says so. The detail pane shows the selected issue's status,
priority, labels, parent and deps, then its body through `markdown.Render` with ids as links.
`capcom ui` and `capcom ui ready <id>` open here, the latter selecting that issue. Depends on items 7
and 9.
**Approach (assumed at the header base):** `briefing.ReadyList(x, cfg, briefing.ReadyOptions{Plan:
index.PlanAny})`, the options `readyOptions` in `internal/cli/ready.go` builds with no flags. Port
`listCursor` (`move`, `clampSelection`, `highlight`, `seat`, `key`, `wheel`) with `listVerdict`,
`listWrap` and `clampInt` from apogee's `listsurface.go` into `internal/ui/listcursor.go`. The detail
loader reads the file (body, ETag) the way `show` does and keeps the ETag for item 16. `IsID` checks
an exact issue or Note id in the index.
**Regression guard.** the detail loader collects the index's id set inside `session.do`; IsID reads that snapshot, never the index, and the rendered detail is cached per (body, width). The file is read with `nodestore.IssueReader(cfg)` (`nodestore.NoteReader()` in item 12) and `Reading.ETag`; a file that does not parse shows `<path>: <error>` in the detail pane in place of the body, with no ETag kept.
**Files:** internal/ui/ready.go, internal/ui/detail.go, internal/ui/listcursor.go, internal/ui/model.go, internal/ui/ready_test.go, internal/ui/testdata/golden/
**Read first:** internal/briefing/ready.go — ReadyList, ReadyOptions; internal/cli/ready.go — readyOptions.listOptions; internal/cli/show.go — issueFromFile;
internal/cli/update.go — parseNodeFile; internal/index/index.go — Index.Issue; /home/airic/repos/apogee/internal/tui/listsurface.go — listCursor, listVerdict
**Closes:** ui-ready-tab
**Tests:** sidebar ids equal `briefing.ReadyList` with those options; goldens of a populated queue
with a selected issue (header and body) and of an empty queue; `ready <id>` selects; an id in the body renders as a link with IsID reading the snapshot;
an unparsable issue file shows its path and error in the detail pane.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
**Commit:** `feat(ui): Ready tab with the issue detail pane`

## 11. Tree widget and Issues tab — ✅ DONE (2026-10-16)

NOTES (2026-10-16): re-derived from "model.go holds the key routing and the detail apply" — item 7/10 put them in internal/ui/keys.go (sidebarOrScrollKey, openTab) and internal/ui/detail.go (applyDetail), so the tab dispatch lands there: new model.showSelection / paintSidebar / sidebarSelection (model.go) and sidebarKey (keys.go); applyDetail now checks the open tab's selection instead of the Ready queue's only.
NOTES (2026-10-16): consequential edit — internal/ui/ready.go: made necessary by capcom ui issues reusing selectFailureFormat (its comment named only capcom ui ready).
NOTES (2026-10-16): progress is counted in internal/ui/issues.go (closed/total over outline.Tree.Children), since internal/ui cannot import internal/cli's progressOf; the row text reuses readyRow's `id  status  P<n>  title`, the format issueLines prints.
NOTES (2026-10-16): reveal rule as implemented: a revealed hidden node shows muted with no arrow and no children of its own (l does nothing on it); hidden ancestors on its path show muted, expanded only along the path; moving onto a muted ancestor makes it the revealed node, and moving onto a shown node drops the reveal at once (the rebuild keeps the selection on the same node).
NOTES (2026-10-16): unspecified edges filled: an empty outline shows "No open issues."; a read failure shows `issues: <error>`; an id that resolves but is not in the tree reports `<id> is not in the issue tree`; Init now loads the Issues tree alongside the Ready queue on every start.
NOTES (2026-10-16): the parent-whose-children-are-all-closed golden is drawn at 150x30 so its `(2/2 closed)` is not cut by the 40-column sidebar of a 120-column terminal.
NOTES (2026-10-16): internal/ui now has 17 non-test files with its package map still in ui.go's package doc only; a doc.go file map with a structural test may be due (coding-standards), not added here.

**What:**
**Goal:** the Issues tab shows the `capcom list --tree` outline (same nodes, same order, closed issues
hidden as by default) as a collapsible tree, each row with its arrow, id, status, priority, title and
child progress; it opens collapsed to the top level; `→`/`l` expands, `←`/`h` collapses or moves to
the parent; expanded state survives tab switches; `capcom ui issues <id>` selects that issue (aliases
resolved as `show` does) with its ancestors expanded. Depends on items 4 and 10.
**Approach (assumed at the header base):** `internal/ui/tree.go`: one tree widget over
`outline.Row` with an expanded set keyed by id and a `listCursor` over the visible rows, reused by
the Knowledge tab. Rows from `outline.Issues(entries, false)`; `index.Resolve` for the id.
**Regression guard.** one rule for selecting an id the tree hides (a closed issue under default visibility, a background Note): the tree reveals that node and its ancestors as extra visible rows (the node marked muted) for as long as it stays selected; when the selection moves away it hides again on the next rebuild; items 12, 13 (dep-view Enter on a closed dep) and 19 (link click on a closed issue) use this rule. Entries come from `Index.List(index.Filter{IsClosedIncluded: true})`, as `issueTreeResult` (list.go) does: the arrow from shown children, progress from all children (`progressOf` over item 4's all-children accessor).
**Files:** internal/ui/tree.go, internal/ui/issues.go, internal/ui/model.go, internal/ui/tree_test.go, internal/ui/issues_test.go, internal/ui/testdata/golden/
**Read first:** internal/cli/list.go — issueTreeResult; internal/cli/outline.go — newVisibleTree, visibleTree.shown, visibleTree.all; internal/cli/issue_render.go — progressOf;
internal/index/query.go — Index.List, Filter.IsClosedIncluded, Index.Resolve
**Closes:** ui-issue-tree
**Tests:** goldens of a nested tree collapsed and expanded; row order equals `outline.Issues`;
expand/collapse/parent keys; expansion kept across a tab switch; `issues <alias>` selects; `issues <closed-id>` reveals it and its
ancestors (muted) and they hide again once the selection moves away; a golden of a parent whose children are all closed.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
**Commit:** `feat(ui): Issues tab with a collapsible tree`

## 12. Knowledge tab — ✅ DONE (2026-10-16)

NOTES (2026-10-16): re-derived from "rows from outline.Notes over Index.Notes(false)": a retired Note named by `capcom ui knowledge <id>` must be in the tree to be revealed (item 11's rule), so the tab reads Index.Notes(true) and builds the outline with outline.NewTree using outline.Notes' grouping and id order and a shown rule that hides retired as well as background subtrees; a test asserts the fully expanded rows equal outline.Notes over Index.Notes(false).
NOTES (2026-10-16): re-derived from "key routing lives in model.go": the tab's sidebar key routing (sidebarKey) lives in internal/ui/keys.go, so the TabKnowledge case is added there.
NOTES (2026-10-16): consequential edit — internal/ui/keys.go: made necessary by the Knowledge tree keeping its expanded nodes across a tab switch (openTab doc comment).
NOTES (2026-10-16): consequential edit — internal/ui/ready.go: made necessary by capcom ui knowledge reusing selectFailureFormat (its doc comment named only ready and issues).
NOTES (2026-10-16): detailMsg gained isNote and sidebarSelection now returns the selected id, so a Note read applies only on the Knowledge tab and an issue read only on the others (an issue and a Note may share an id).

**What:**
**Goal:** the Knowledge tab shows the `capcom know tree` outline (retired and background Notes hidden
as by default) in the tree widget, collapsed to the top level; the detail pane shows the Note's
title, labels, refs (refs naming a Note or issue id are links, file refs inert) and rendered body;
`capcom ui knowledge <id>` selects that Note. Depends on item 11.
**Approach (assumed at the header base):** rows from `outline.Notes` over `Index.Notes(false)`;
`index.ResolveNote` for the id; the Note file read for body and ETag as in item 10.
**Regression guard.** `capcom ui knowledge <id>` naming a retired or background Note (ResolveNote resolves both) follows item 11's rule for an id the tree hides: it and its ancestors show as extra rows, the Note muted, while it stays selected.
**Files:** internal/ui/knowledge.go, internal/ui/detail.go, internal/ui/model.go, internal/ui/knowledge_test.go, internal/ui/testdata/golden/
**Read first:** internal/cli/know_tree.go — (a *app) outline; internal/cli/know.go — newNoteTree, resolveNote; internal/index/notes.go — Index.Notes, Index.ResolveNote, retiredFilter;
internal/knowledge/note.go — IsBackground
**Closes:** ui-knowledge-tab
**Tests:** goldens of a nested Note tree and a Note with refs; row order equals `outline.Notes`;
`knowledge <id>` selects; `knowledge <retired-id>` reveals and selects it.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
**Commit:** `feat(ui): Knowledge tab`

## 13. Dependency view — ✅ DONE (2026-10-16)

NOTES (2026-10-16): the dependency view's keys are intercepted in model.Update ahead of handleKey (keys.go untouched); a tab key closes the view and then switches tabs as before. The waits-on side lists the file's deps verbatim with graph.Resolve (dep list's depth-1 output); the blocks side walks issue.WalkDependents at depth 1. A side with no issue shows "none"; an issue with neither shows "<id> waits on no issue and blocks none."
NOTES (2026-10-16): retry fix: with the view open, a key the dependency cursor does not take but the sidebar does (h l and the left and right arrows on the Issues tree) closes the view first and then moves the sidebar, so no issue's dependencies stay beside another's selection; covered by TestDepView_SidebarKey_ClosesTheViewBeforeMovingTheSelection.

**What:**
**Goal:** on the Issues and Ready tabs `d` swaps the detail pane to the selected issue's "waits on"
and "blocks" lists, matching `capcom dep list <id>` with no flags, each row with its own cursor;
Enter on a row switches to the Issues tab and selects that issue with ancestors expanded; Esc or `d`
returns to the body. An issue with neither side says so. Depends on item 11.
**Approach (assumed at the header base):** build `issue.NewDepGraph` from all index entries, closed
included (as `loadDepGraph`), and walk `issue.WalkDeps` / `issue.WalkDependents` at the depth
`dep list` uses by default, missing deps marked as it marks them.
**Regression guard.** Enter on a closed dep (or one under a closed parent) follows item 11's rule for an id the tree hides; Enter on a `missing` row shows `<id> is missing` in the status bar and leaves the selection unchanged.
**Files:** internal/ui/depview.go, internal/ui/model.go, internal/ui/depview_test.go, internal/ui/testdata/golden/
**Read first:** internal/cli/dep_list.go — depGraph.listResult, depBranches, defaultDepListDepth, depMissingLabel; internal/cli/dep_graph.go — loadDepGraph;
internal/issue/deps.go — NewDepGraph, WalkDeps, WalkDependents
**Closes:** ui-dep-view
**Tests:** an issue with both sides (golden), one with neither, Enter selecting a dependent; Enter on a `done` dep
reveals and selects it; Enter on a `missing` row shows the message, selection unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
**Commit:** `feat(ui): dependency view for the selected issue`

## 14. Sidebar filter

**What:** Recast at the regression check (2026-10-16).
**Goal:** `/` opens a filter line over the current tab's sidebar; Enter keeps it, Esc clears it, and
the status bar shows `filter: <text>` while one is in effect. Terms AND together: `status:<name>`
and `label:<name>` narrow by field, the other words match title and body exactly as `capcom search`
(issues) and `capcom know search` (Notes) match them. In a tree a match keeps its ancestors, expanded.
An undeclared status or a label no node carries shows a message naming it; `status:` on the
Knowledge tab says it applies to issues. A `status:<name>` filter naming a closed-category status (e.g.
`status:done`) includes closed issues, shown with their ancestors; text and label filters keep default
visibility. Depends on items 12 and 13.
**Approach (assumed at the header base):** text terms through `Index.SearchIssues(text)` /
`Index.SearchNotes(text, nil, false)`, intersected with the tab's rows; `charm.land/bubbles/v2`
`textinput` for the line (added with `go get charm.land/bubbles/v2@v2.1.0`), wrapped in a small
`lineInput` reused by item 17's prompts. One filter per tab.
**Regression guard.** Ratified design call (user, 2026-10-16), **Closed issues:** a `status:<name>` filter naming a closed-category status (e.g. `status:done`) includes closed issues, shown with their ancestors; text and label filters keep default visibility; no toggle key. The open filter line takes every key (Enter and Esc included) before any global binding, so typing `q`, `1` or `d` into it quits nothing and switches nothing. Esc clears a kept filter only from the sidebar with no dep view or narrow detail open; Enter from the dep view on an issue the Issues filter hides clears that filter.
**Files:** go.mod, go.sum, internal/ui/filter.go, internal/ui/lineinput.go, internal/ui/model.go, internal/ui/filter_test.go, internal/ui/testdata/golden/
**Read first:** internal/index/query.go — Index.SearchIssues, Filter.IsClosedIncluded; internal/index/notes.go — Index.SearchNotes;
internal/index/index.go — searchIDs, matchExpression; internal/cli/search.go — newSearchCommand; internal/config/config.go — Config.CategoryOf;
charm.land/bubbles/v2@v2.1.0 textinput/textinput.go — Model
**Closes:** ui-filter
**Tests:** text, status, label, combined terms, ancestors kept in the Issues and Knowledge trees, the
Ready list; text matches equal `SearchIssues`/`SearchNotes` for the same words; unknown status and
unknown label messages; `status:done` lists a closed issue with its ancestors and the row is selectable (`R` on it
is tested in item 17); typing `q`, `1` and `d` into the open line quits and switches nothing; Esc with the dep view
open leaves the filter kept.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
**Commit:** `feat(ui): filter the sidebar with /`

## 15. Live refresh

**What:** Recast at the regression check (2026-10-16).
**Goal:** about once a second the UI calls `Index.Refresh` and re-reads the Tree lock; when anything
changed it reloads the tabs and redraws, keeping selection, expanded nodes and scroll position; a
vanished selected node moves the selection to its parent, else the next sibling, else the previous
one. A tick with no change reloads nothing. Depends on items 3 and 14.
**Approach (assumed at the header base):** a `tickMsg` scheduled with `tea.Tick(time.Second)`; its
Cmd runs Refresh and the lock read in `session.do` and returns `refreshedMsg{changed, lock}`;
selection kept by id. Tests send `tickMsg` directly and run the returned Cmd synchronously (the fake
clock); no real sleep.
**Regression guard.** each tick, before Refresh, the session compares `.capcom/config.yaml` (mtime and size) with what it loaded: on a change it re-opens the store (`store.Open`) and swaps the session's store/config so Refresh never rebuilds index.db under a stale config, and newly declared statuses appear; if the index's stored schema version differs from this binary's (another capcom version rebuilt it), polling stops and the status bar shows `index changed by another capcom version: restart capcom ui` — never a rebuild from the UI; tests: a config.yaml edit plus one tick shows a newly declared status without a rebuild loop, and a schema-version mismatch stops polling with the message. Also: the filter in effect is applied again on every reload; test: filter set, an issue edited on disk, one tick, rows still filtered.
Re-check (2026-10-16): on a config.yaml change the session closes the Index and re-opens it on the new store (`index.Open(newStore, ModeRefresh)`, one rebuild), rebuilds `lock.ForWorktree(newStore)` (TTL from config) and passes the new store to ops; swapping only the session's store does not reach the Index. On a failed re-open (e.g. a `*config.Error`) it keeps the old store and index, shows the config error in the status bar, and retries only when mtime or size changes again; a missing config.yaml is its own stat state. The ui test harness backdates every store file with `os.Chtimes` (e.g. now-1h) before opening the index, so later edits are not racy-clean.
**Files:** internal/ui/harness_test.go, internal/ui/refresh.go, internal/ui/model.go, internal/ui/session.go, internal/ui/refresh_test.go, internal/index/schema.go, internal/index/index_test.go
**Read first:** internal/index/refresh.go — Index.refresh, recordedFile.isDirty; internal/index/schema.go — ensureSchema, isSchemaCurrent;
internal/index/index.go — Open, Index; internal/store/store.go — Open; internal/lock/worktree.go — ForWorktree
**Closes:** ui-live-refresh
**Tests:** an issue file edited on disk shows after one tick; the selected node removed moves to its
parent; a no-change tick leaves the tab data untouched (no reload); a lock acquired on disk shows in
the status bar after one tick; a config.yaml edit plus one tick shows a newly declared status without a rebuild
loop; a schema-version mismatch stops polling with the message; filter set, an issue edited on disk, one tick,
rows still filtered; a config.yaml with a YAML error plus one tick keeps the old session and shows the error, and a fix plus one
tick re-opens. Files are backdated by the harness before the index opens.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/` and `GOMEMLIMIT=2GiB go test ./internal/index/`
**Commit:** `feat(ui): live refresh by polling the index`

## 16. Writes: lock prompt, conflicts, claim and status

**What:**
**Goal:** on the Issues and Ready tabs `a` claims the selected issue and `s` opens a picker of the
configured statuses outside the closed categories, both through `internal/ops` with the on-screen
ETag as if-match. While another session holds this worktree's lock, live or stale (read at the
moment of the write), a write first asks `locked by <purpose>, write anyway? (y/n)`; y or Enter
writes, n or Esc cancels with nothing written. A `*nodestore.ConflictError` reloads the issue and
asks `changed on disk: <fields> — run <action> again? (y/n)`, fields from title, status, priority,
labels, parent, deps, body, log; y re-runs on the new ETag. The UI never acquires, touches or
releases the lock. Depends on items 2 and 15.
**Approach (assumed at the header base):** one write pipeline in `internal/ui/write.go`: lock check
→ confirm → `ops` call in `session.do` → refresh, or conflict → diff → confirm → retry; actions are
values so item 17 adds close, reopen and log to the same pipeline.
**Regression guard.** the conflict prompt diffs every frontmatter field of `issue.Issue` (type, related, plan, aliases, closed and reason included, replacing the Goal's field list) plus `body`/`log`; when nothing differs (a reformat) it names `file`. A lock read that fails at write time (an invalid or unreadable record) shows the read error and asks as for a held lock. The no-lock-call check is `grep -nE '\.(Acquire|Touch|Release)\(' $(ls internal/ui/*.go | grep -v _test.go)`, empty.
**Files:** internal/ui/write.go, internal/ui/prompt.go, internal/ui/statuspick.go, internal/ui/model.go, internal/ui/write_test.go, internal/ui/testdata/golden/
**Read first:** internal/nodestore/errors.go — ConflictError; internal/lock/worktree.go — Worktree.Read; internal/lock/overview.go — Status.State;
internal/issue/mutate.go — SetStatus, StatusError; internal/cli/update.go — applyUpdate, statusChangeFailure; internal/config/config.go — DefaultStatus
**Tests:** `a` and `s` change the file as `capcom update --status` does (read the file back); a file
edited between display and write is refused, the prompt names the changed fields (golden), y lands
the write; held and stale locks ask, n leaves the file byte-identical; a free lock asks nothing;
a type-only change (golden) names `type`; a hand-written invalid lock record asks with the read error;
`grep -nE '\.(Acquire|Touch|Release)\(' $(ls internal/ui/*.go | grep -v _test.go)` prints nothing.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
**Commit:** `feat(ui): claim and status writes with lock prompt and conflict retry`

## 17. Writes: close, reopen and log

**What:**
**Goal:** `c` prompts for a reason and closes the selected issue with the status `capcom close` uses
without `--status`, then shows `closed <id>; now ready: <ids>` (no "now ready" part when none);
`R` prompts for a reason and reopens with the status `capcom reopen` uses without `--status`; `L`
prompts for a line and appends it as `capcom log` does. An empty reason is refused as the CLI refuses
it; closing a closed issue or reopening an open one shows the CLI's message. Prompts submit on Enter
and cancel on Esc with nothing written. All go through item 16's pipeline. Depends on item 16.
**Approach (assumed at the header base):** `ops.Close`, `ops.Reopen`, `ops.AppendLog`; the default
statuses via `config.DefaultStatus` of the categories `newCloseCommand` / `newReopenCommand` pass to
`targetStatus`; prompts through `lineInput`.
**Regression guard.** the route to a closed issue is item 14's `status:<closed status>` filter (ratified call **Closed issues**); `R` and close-of-closed are tested through it. Close of a closed issue shows `<id> is already closed`; reopen of an open issue shows `<id> is not closed; only a closed issue can be reopened`; both are the UI's own wording, pinned in tests, and the item yields to internal/cli/update.go:145-147 (the `statusChangeFailure` wording stays in cli). A blank (TrimSpace) reason is refused with `the reason is empty; say why` and a blank `L` line with `the log text is empty` (item 1's ops refusals); both leave the file byte-identical.
**Files:** internal/ui/write.go, internal/ui/prompt.go, internal/ui/model.go, internal/ui/write_close_test.go, internal/ui/testdata/golden/
**Read first:** internal/cli/close.go — closeResult, alreadyClosed, requireReason; internal/cli/reopen.go — targetStatus; internal/cli/log.go — newLogCommand;
internal/issue/mutate.go — Close, Reopen, ErrNotClosed
**Closes:** ui-writes
**Tests:** each write read back from the file; close naming the newly ready issue; Esc on each prompt
writes nothing; the reason prompt golden; a conflict and a lock prompt on `L`; `R` on a closed issue
reached through `status:done`; close of a closed and reopen of an open issue show their messages; a blank reason
and a blank log line are refused, the file byte-identical.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
**Commit:** `feat(ui): close, reopen and log writes`

## 18. Open in $EDITOR

**What:**
**Goal:** `e` opens the selected Issue or Note file in `$VISUAL`, else `$EDITOR` (split on spaces,
the path appended), suspending the UI and restoring it after; afterwards it runs `check.Run` on the
store and shows the first problem and the count in the status bar, then refreshes. No editor set
shows a message. While another session holds the lock it asks first, as writes do. Depends on
item 17.
**Approach (assumed at the header base):** `tea.ExecProcess` behind a model field (`execFn`) that
tests replace with a synchronous runner; the fake editor is the test binary re-executed through
`TestMain` with an env flag, so it works on every OS.
**Regression guard.** after the edit the status bar shows the first `check.Run` problem in the edited file's path when there is one, else the first overall, plus the total count. The `execFn` fake runner re-executes `os.Args[0]` with the argv tail it is handed, so `$VISUAL` in tests is a fixed word (e.g. `fake-editor --flag`); the space split gets its own unit test.
**Files:** internal/ui/editor.go, internal/ui/model.go, internal/ui/editor_test.go, internal/ui/main_test.go
**Read first:** internal/check/check.go — Run, Problem; internal/cli/check.go — checkProblems; charm.land/bubbletea/v2@v2.0.8 exec.go — ExecProcess, ExecCallback;
internal/cli/testharness_test.go — setHermeticGitEnv
**Closes:** ui-editor
**Tests:** the fake editor edits the file and the change shows; an edit breaking frontmatter surfaces
the `check` problem; no `$VISUAL`/`$EDITOR` shows the message; the lock prompt appears; with a pre-existing problem in another file
the edited file's problem shows first, with the count; the space split unit test.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
**Commit:** `feat(ui): open the selected file in $EDITOR`

## 19. Mouse

**What:**
**Goal:** with mouse reporting on, a click selects a tab, a sidebar row, a dep-view row or a detail
link (an issue id selects it in the Issues tab, a Note id in the Knowledge tab); a click on a tree
arrow expands or collapses; the wheel scrolls the sidebar or the detail pane, whichever is under the
pointer. Hit-testing is correct in the narrow layout (only the visible pane responds). Depends on
items 9, 13 and 18.
**Approach (assumed at the header base):** `View` records each region (tabs, rows, arrows, detail
links from `Rendered.Links` offset by the pane origin and scroll) in a geometry value the mouse
handler reads; `tea.View.MouseMode = tea.MouseModeCellMotion`; rows use `listCursor.wheel`. Patterns
from apogee's `mouse.go`, no code copied.
**Regression guard.** clicks and wheel are ignored while a prompt, status picker or help screen is open. A link to an id the tree hides (a closed issue, a retired or background Note) follows item 11's rule. The wheel over the sidebar moves the selection one row per notch (`listCursor.wheel`), which is what the Goal's "scrolls the sidebar" means; over the detail pane it scrolls the pane.
**Files:** internal/ui/mouse.go, internal/ui/layout.go, internal/ui/model.go, internal/ui/mouse_test.go, internal/ui/tree.go, internal/ui/depview.go, internal/ui/detail.go
**Read first:** /home/airic/repos/apogee/internal/tui/mouse.go — handleMouseClick; .../listsurface.go — listCursor.wheel, listCursor.move;
charm.land/bubbletea/v2@v2.0.8 tea.go — View.MouseMode, MouseModeCellMotion; .../mouse.go — MouseClickMsg, MouseWheelMsg; internal/cli/list.go — isShown
**Closes:** ui-mouse
**Tests:** `tea.MouseClickMsg` / `tea.MouseWheelMsg` at fixed coordinates for each target, at 120x30
and 80x24, including a link after scrolling; a click during the reason prompt, then Enter writes only the prompted
issue; a link to a closed issue reveals and selects it; a wheel notch over the sidebar moves the selection.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
**Commit:** `feat(ui): mouse clicks, wheel and links`

## 20. Theme presets and colour overrides

**What:**
**Goal:** the UI's colours come from `dark` and `light` presets over the roles in `userconfig.Roles`;
`theme: auto` (the default) picks one from the detected terminal background (dark until it answers);
a colour override changes that role everywhere it is used; with `NO_COLOR` set to anything non-empty
the view contains no colour sequences whatever the file says. Depends on items 5 and 19.
**Approach (assumed at the header base):** `internal/ui/theme.go` builds every lipgloss style of the
UI and `markdown.Theme` from a role → colour map; status colours by `config.Category`;
`tea.RequestBackgroundColor` in `Init` and `tea.BackgroundColorMsg.IsDark()`.
**Regression guard.** `theme.go` also builds `lineInput`'s `textinput.Styles` and every style in filter.go, prompt.go and statuspick.go (bubbles' default textinput styles carry colours). The `accent` override is tested in the tab bar and a `heading` override in the markdown heading. `ui.Options` gains `NoColor` (internal/cli/ui.go sets it from `os.Getenv("NO_COLOR")`); tests set it explicitly, never from the process env. `goldenANSI(t, name, view)` in harness_test.go (same `-update` flag, no strip) keeps the ANSI goldens.
**Files:** internal/ui/theme.go, internal/ui/model.go, internal/ui/statusbar.go, internal/ui/theme_test.go, internal/ui/testdata/golden/, internal/ui/lineinput.go, internal/ui/filter.go, internal/ui/prompt.go, internal/ui/statuspick.go, internal/ui/harness_test.go, internal/ui/ui.go, internal/cli/ui.go
**Read first:** bubbletea/v2@v2.0.8/color.go — RequestBackgroundColor, BackgroundColorMsg.IsDark; lipgloss/v2@v2.0.4/color.go — Color; bubbles/v2@v2.1.0/textinput/styles.go — DefaultStyles;
internal/config/config.go — CategoryOf, DefaultStatus; adr-0011-decision.md — Precedence; docs/design/tui.md — Theme
**Closes:** ui-theme
**Tests:** goldens kept with ANSI (no strip) for the dark preset, the light preset and `NO_COLOR`; a
`BackgroundColorMsg` with a light colour switches `auto` to light; an `accent` override appears in
the tab bar and a `heading` override in the markdown heading; `NO_COLOR` output has no `38;`/`48;` SGR parameters,
a frame with the filter line open included.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
**Commit:** `feat(ui): theme presets and colour overrides`

## 21. Document capcom ui

**What:**
**Goal:** `docs/site/` has a manual page for `capcom ui` (tabs, keys including `d` and PgUp/PgDn,
mouse, writes, the lock prompt, conflicts, the theme config) with the frontmatter the site requires
(`title`, one-sentence `description`, `sidebar: { order: <n> }`, no body H1);
`docs/site/reference/file-format.md` describes the per-user config file; the CLI reference is
regenerated; `docs/design/tui.md` matches the build (status-role example `status-doing`, `d` key,
detail scroll keys, tree start). Depends on item 20.
**Approach (assumed at the header base):** a new page `docs/site/getting-started/terminal-ui.md`, linked with relative `.md` links, glossary terms, no version
number; `go run ./cmd/gendocs`.
**Regression guard.** the manual page says that with mouse reporting on, terminal text selection needs shift-drag (or the terminal's own modifier), and documents the `status:<closed status>` route to closed issues. Every claim in docs/design/tui.md is checked against the build (`grep -n 'working name\|refresh\|go 1\.\|x/sys\|status-\|PgUp\|^## ' docs/design/tui.md`); the four sites the Goal names are examples, not the scope. file-format.md's description and opening paragraph name the per-user config as the one file outside `.capcom/`, and so does the docs-layout Note's file-format line.
**Files:** docs/site/getting-started/terminal-ui.md, docs/site/reference/file-format.md, docs/site/reference/cli/, docs/design/tui.md, .capcom/knowledge/docs-layout.md
**Read first:** internal/clidoc/reference_test.go — TestReferenceUpToDate, checkFrontmatter; cmd/gendocs/main.go — defaultOutDir; docs/design/tui.md — Live refresh, Theme;
docs/site/reference/file-format.md — Config section; .capcom/knowledge/docs-layout.md — docs/site rules; docs/site/getting-started/quickstart.md — frontmatter
**Closes:** ui-docs
**Tests:** none beyond the reference check.
**Acceptance:**
- `go run ./cmd/gendocs && git diff --exit-code docs/site/reference/cli/`
- `GOMEMLIMIT=2GiB go test ./internal/clidoc/`
- `grep -n 'status-active' docs/design/tui.md` prints nothing
- `capcom check`
**Commit:** `docs(ui): document capcom ui and the per-user config`
