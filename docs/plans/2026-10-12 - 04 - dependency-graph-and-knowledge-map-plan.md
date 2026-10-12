# Dependency graph and knowledge map plan

**Goal:** Make dependency facts reach agents through the output they already read: `close`, `show`,
`ready`, `create`, bounded transitive views, plus an issue outline and a Mermaid export. Also land the knowledge-organization review's findings, sized for 25B-class
local models.
**Date:** 2026-10-12
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 06f5327

**Sources:**
- `docs/handoffs/2026-10-12 - 01 - knowledge-organization-review.md`
- `docs/format.md`; Notes `adr-0001`, `adr-0003`, `adr-0009`, `adr-0010`, `store-layout`, `docs-layout`,
  `cli-test-harness`, `error-messages`

**Ratified design calls** (user, 2026-10-12, unless marked author):
- **Scope:** proposal items 1–7 plus review findings #1–#8; one capcom issue per item.
- **Rejected ADR options:** label `background`; `know tree` hides such Notes unless `--background`.
- **Who works on what:** `prime` already lists the doing issues; the issue-workflow Note documents the convention (doing status plus Tree lock purpose). No new field.
- **Many issues:** `capcom list --tree [--depth N]`, the same shape as `know tree`.
- **No stub issues:** guidance in the AGENTS block and a Note; `create` without a body warns.
- **close:** text ends `now ready: a, b`. **show:** prints `ready:`, `dependents:` and `children:` lines.
- **dep list:** `--depth N` with a hint where depth cut. **ready:** priority ties go to the issue that unblocks more, shown as `-- unblocks N`.
- **Unblocks (author):** an issue "unblocks" exactly the todo issues that become Ready when it alone
  is closed; `close` uses the same rule over the closed set.
- **dep graph (author):** Mermaid only, no format flag; edges point dependency → dependent.
- **Output (author):** JSON additions only (`SchemaVersion` stays 1); `--kv` output unchanged.
- **Item 14 dropped** (user, 2026-10-12): prime already lists the doing issues; review #8 is covered by the issue-workflow Note.

**Standing requirements:**
- skills: coding-standards
- CLI tests go through `runCLI`/`runCLIWith`, never `t.Parallel()` (Note `cli-test-harness`).
- Tests on this host: one package at a time with `GOMEMLIMIT=2GiB`; never `./...` with `-race`.
- Every error names the file, and the line when known (Note `error-messages`).
- A new non-test file in `internal/cli` gets its line in the `doc.go` file map.
- After any change under `.capcom/`: `capcom check` and `capcom fmt --check` exit 0.
- Every `capcom` command an item runs uses the in-tree build (`go run ./cmd/capcom …`), never the `capcom` on PATH.

**Regression check (2026-10-12, 06f5327):**
- 1: guard folded (NewlyReady leaves out hidden issues; neighbour dedupe; repeat steps for item 9)
- 2: guard folded (`now_ready` never null)
- 3: guard folded (one comparator, `--all` want change, incremental re-judge); yields to glossary Note `ready` (_Avoid_: unblocked)
- 4: guard folded (`Ready *bool` on `showData`; whole-package Acceptance)
- 7: guard folded (bodiless `create --kv` test gains `--body-file`)
- 8: guard folded (pinned `--depth 1` cases; direct-children count; retired case)
- 9: guard folded (dep-list wrapper type; repeat steps; depth-1 file order)
- 10: guard folded (list wrapper type; orphan rows as roots; `--parent` at depth 0)
- 11: guard folded (doc.go map and `newDepCommand` comment)
- 12: guard folded (filter outside `childrenByParent`); supersedes `adr-0010-rejected.md:17-18`
- 13: guard folded (in-tree build; format.md left-out sentences; adr-0010-rejected amended in place)
- old 14 (prime lists the issues in progress): dropped by the user; old items 15–18 renumbered 14–17
- 14 (was 15): guard folded (in-tree build; CHANGELOG upgrade note; Depends adds 2 and 4)
- 15 (was 16): guard folded (in-tree build; content greps in Acceptance)
- 15 (was 16): rejected — `grep -ci ambiguit .capcom/knowledge/glossary.md` as an Acceptance check; it already prints 1 at 06f5327 (the `glossary-ambiguities` section line)
- 16 (was 17): guard folded (in-tree build; `know add --id/--title --body-file`; body grep)
- 17 (was 18): guard folded (show line order in format.md; explicit Acceptance greps)

**Out of scope:**
- Typed edges, stored reverse edges, critical path, weights, DOT export.
- An issue assignee key, `lock acquire --issue`, any config or front-matter key (no format raise).

## 1. Unblock query and bounded dependency walk — ✅ DONE (2026-10-12)

NOTES (2026-10-12): the walk API is `DepGraph.Walk(id, WalkDeps|WalkDependents, maxDepth) []WalkStep` with `WalkStep{ID, Via, Depth, Cut, IsRepeat}` and `UnlimitedDepth = 0`; the start issue gets no step (its neighbours are depth 1), and an id first reached at the cap counts as reached, so a later sighting is a repeat step.
NOTES (2026-10-12): `Dependents` now returns a copy of the reverse edges built in `NewDepGraph` (same order and dedupe as before); the `ids` field it alone used was removed as dead.
NOTES (2026-10-12): package `internal/issue` now holds 10 non-test files and has no `doc.go` file map; the plan does not ask for one (pre-existing gap, not added here).

**What:**
**Goal:** package `issue` answers two questions. (a) Which todo issues become Ready when a given set
of issues is closed with given categories. (b) The deps or the dependents of an issue to a maximum
depth, each id once, terminating on cycles, with a count of the neighbours the depth cap left out.
**Approach (assumed at the header base):** in `internal/issue`, add `NewlyReady(nodes []Node, cfg
config.Config, closed map[string]config.Category) []string`. It copies the nodes, sets each closed id's
`Category`, runs `Ready` before and after, and returns the ids Ready after but not before, in node
order, hidden ones left out (see the guard). Add a depth-limited pre-order walk on
`DepGraph`, in both directions (build the reverse edges in `NewDepGraph`), whose steps carry id, the
id it hangs under, depth, and the count of neighbours cut by the cap. One deep module with no CLI
concerns; `maxDepth` 0 means unlimited.
**Regression guard.** `issue.NewlyReady` itself leaves out the issues `Ready` marks `IsHidden`; items 2
and 3 rely on that and do no hidden filtering of their own. The walk dedupes each id's neighbours in
both directions before walking and counting (`depsNodes`' `by-id` names `db-schema` by id and by alias).
Each id is expanded once; an id reached again is a repeat step (flagged, no subtree, not cut) for item 9.
**Files:** internal/issue/unblock.go, internal/issue/unblock_test.go, internal/issue/deps.go, internal/issue/deps_test.go
**Read first:** internal/issue/ready.go — Ready, readyTree.resolve, Readiness.IsHidden, hasAny; internal/issue/deps.go — NewDepGraph, DepGraph.Dependents;
internal/issue/deps_test.go — depsNodes, TestDepsGraph_Dependents_ResolveIDsAndAliasesOnce
**Closes:** unblocks-query
**Tests:** table tests: a sole blocker closed done, then dropped, readies its dependent; one of two
blockers readies nothing; both closed together ready it; closing the last open child readies the
parent; an alias dep resolves; a hidden dependent is not returned; the walk terminates on a cycle; the
depth cap reports the cut counts; on `depsNodes`, `by-id` has one step to `db-schema` (cut count 1 at
a cap that cuts it); a diamond yields one repeat step.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/issue/`
**Commit:** `feat(issue): tell which issues a close makes ready and walk deps to a depth`

## 2. close names the issues it made ready — ✅ DONE (2026-10-12)

**What:**
**Goal:** after closing, `capcom close` text output ends with one line `now ready: <id>, <id>` listing
the todo issues the close made Ready (no line when none; issues hidden by `ready.hide` left out). Each
JSON `closeData` carries `now_ready` (ids, always present): the issues this argument's close readied,
given the earlier arguments' closes, each id reported once across the command.
**Approach (assumed at the header base):** `closeResult` loads every node from the index once, before
writing (the index is not refreshed after writes), and keeps a map of the ids closed so far to the
category of the target status. After each `closed` outcome it calls `issue.NewlyReady` with that map
and subtracts the ids already reported. `already_closed` adds nothing. Depends on item 1.
**Regression guard.** `now_ready` is tagged without `omitempty`, and both `closeData` builds (the failure
branch, `close.go:102`, and the closed/already_closed outcome) set `NowReady` to `[]string{}`, so it is
never `null`. Hidden issues are already left out by `issue.NewlyReady` (item 1); no filtering here.
**Files:** internal/cli/close.go, internal/cli/close_test.go
**Read first:** internal/cli/close.go — closeResult, closeOne, closeData; internal/briefing/ready.go — ReadyList; internal/index/query.go — Index.List, Entry.Node;
internal/cli/dep_test.go — newDepStore; internal/cli/close_test.go — TestClose_SeveralIDsOneAlreadyClosed_ClosesTheRestAndReportsEach
**Closes:** close-now-ready
**Tests:** in `newDepStore`, closing `db-schema` prints `now ready: build-pipeline` (its dep is the alias
`apogee-kk0.4`); `--status dropped` also readies it; two ids closed together report each readied id
once; a hidden issue is not reported; the JSON `now_ready` per argument; a `not_found` argument
carries `now_ready: []`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Close|Reopen'`
**Commit:** `feat(close): name the issues a close made ready`

## 3. ready ranks priority ties by what an issue unblocks — ✅ DONE (2026-10-12)

NOTES (2026-10-12): unblocks are derived from the one `issue.Ready` pass's before-verdicts (`unblocksByCloser`/`soleCloser` in internal/briefing/ready.go: a non-hidden, unheld todo issue becomes Ready exactly when every blocking dep, resolved through `issue.DepGraph.Resolve`, and every open child is one single issue) instead of calling `issue.NewlyReady` per item, per the regression guard; `TestReadyList_Unblocks_MatchNewlyReady` cross-checks the result against `issue.NewlyReady` on both briefing fixtures.
NOTES (2026-10-12): the `--show-hidden` want of `TestReady_Flags_SelectReadyIssuesMostUrgentFirst` changes too (`session-refresh` now precedes `deploy-prod`), a direct consequence of the new order; the README/format reference text is left to item 17.

**What:**
**Goal:** `capcom ready` orders by priority, then by the number of issues each Ready issue unblocks
(more first), then by tree order. A Ready line with N > 0 ends `  -- unblocks N`. JSON `readyData`
carries `unblocks` (ids, omitted when empty). Not-ready lines under `--all` keep their order rule and
gain no suffix. `prime` inherits the order and keeps its line format.
**Approach (assumed at the header base):** in `briefing.ReadyList`, for each Ready item compute
`issue.NewlyReady(nodes, cfg, {id: <first done category>})`, which already leaves out hidden issues. Store it on
`ReadyItem` and extend the `slices.SortStableFunc` key. `readyLines` appends the suffix. Depends on item 1.
**Regression guard.** One comparator: priority, then the unblocks count descending (a not-ready item
counts 0), then tree order. Compute the before-verdicts once per `ReadyList` and re-judge only the closed
issue's dependents and parent (the only verdicts a done close can turn Ready, `internal/issue/ready.go:59-77`),
never two full `Ready` passes per item. Glossary Note `ready` lists "unblocked" under _Avoid_; the item
yields to it: `unblocks` only counts or lists the issues a close makes Ready, never names a state.
**Files:** internal/briefing/ready.go, internal/briefing/ready_test.go, internal/cli/ready.go, internal/cli/ready_test.go
**Read first:** internal/briefing/ready.go — ReadyList, ReadyItem; internal/cli/ready.go — readyLines, readyData;
internal/cli/ready_test.go — newReadyStore, TestReady_Flags_SelectReadyIssuesMostUrgentFirst; internal/briefing/ready_test.go — TestReadyList_Options_SelectAndSortStablyByPriority;
internal/cli/prime_test.go — newPrimeStore
**Closes:** ready-unblock-order
**Tests:** two P2 issues where the later one in tree order unblocks one issue: it is listed first,
with `-- unblocks 1`; a lower priority never outranks a higher one; JSON `unblocks`. The `--all` want of
`TestReady_Flags_SelectReadyIssuesMostUrgentFirst` changes: `session-refresh` (it readies `login-flow`)
moves to second, after `api-tokens`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/briefing/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Ready|Prime'`
**Commit:** `feat(ready): rank priority ties by the issues each one unblocks`

## 4. show prints readiness and dependents

**What:**
**Goal:** `capcom show` prints, for a todo-category issue, `ready: yes` or `ready: no (<reasons>)` right
after `category:`, the reasons worded exactly as `ready --all` words them. For an issue that others
depend on, it prints `dependents: [<id>, <id>]` right after `deps:`. JSON `showData` gains `ready`
(bool, omitted outside todo), `blocking` (as in `readyData`) and `dependents` (ids, always present).
No other command's output changes.
**Approach (assumed at the header base):** `showResult` loads the nodes once (index `List` with
`IsClosedIncluded`), runs `issue.Ready` and `issue.NewDepGraph`, and passes each issue's verdict and
dependents into the text renderer. Reuse `notReadyReasons` from `internal/cli/ready.go`; don't copy it.
**Regression guard.** The field is `Ready *bool` tagged `json:"ready,omitempty"` on `showData`, never on `issueData`
(list and search share it; `related_test.go:289` pins `list --json` without show-only keys), so a blocked
todo issue carries `ready: false`. Acceptance runs the whole `internal/cli` package to check that no
other command's output changes.
**Files:** internal/cli/show.go, internal/cli/issue_render.go, internal/cli/read_test.go
**Read first:** internal/cli/show.go — showResult; internal/cli/issue_render.go — issueText, showData, issueData; internal/cli/ready.go — notReadyReasons, readyData;
internal/cli/dep.go — loadDepGraph; internal/issue/ready.go — Ready; internal/cli/read_test.go — newReadStore
**Closes:** show-readiness
**Tests:** a blocked todo issue shows `ready: no (blocked by …)`; a Ready one shows `ready: yes`; a done
issue has no `ready:` line; a depended-on issue lists its dependents; JSON fields, `ready: false`
present for a blocked todo issue.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Show|Related'`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/`
**Commit:** `feat(show): print readiness and dependents`

## 5. show prints child progress

**What:**
**Goal:** `capcom show` on an issue with direct children prints `children: <closed>/<total> closed`
after `dependents:` (or `deps:`). JSON `showData` gains `children` `{total, closed}`, omitted when
there are none.
**Approach (assumed at the header base):** count with index `ListIDs` (`ParentID`, `IsClosedIncluded`,
then the done and dropped categories), in one helper in `internal/cli/issue_render.go` that item 10
reuses. Depends on item 4.
**Files:** internal/cli/show.go, internal/cli/issue_render.go, internal/cli/read_test.go
**Read first:** internal/cli/issue_render.go — issueText, showData; internal/cli/show.go — showResult; internal/index/query.go — Filter, ListIDs, Filter.sql;
internal/cli/read_test.go — newReadStore, TestShow_Text_PrintsFrontmatterThenBodyVerbatim
**Closes:** show-child-progress
**Tests:** `login-flow` in `newReadStore` with one open child shows `children: 0/1 closed`; after the child
is closed, `children: 1/1 closed`; a leaf has no line.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run Show`
**Commit:** `feat(show): print how many children are closed`

## 6. create takes dependencies

**What:**
**Goal:** `capcom create --dep <id|alias|prefix>` (repeatable, comma-separated allowed) writes each
dependency's canonical id into the new file's `deps`, once each, in the order given. A dependency that
resolves to no issue fails the create with the error `dep add` gives for it, and no file is written.
**Approach (assumed at the header base):** in `newCreateCommand`, resolve each value with
`a.resolveIssue` before `nodestore.Issues(...).Create`, de-duplicate, and set `Deps` in `newIssue`. No
cycle is possible, since nothing depends on a new issue yet.
**Files:** internal/cli/create.go, internal/cli/write_test.go
**Read first:** internal/cli/create.go — newCreateCommand, create, newIssue; internal/cli/show.go — resolveIssue; internal/cli/dep.go — resolveEdge;
internal/nodestore/create.go — Kind.Create; internal/cli/write_test.go — newWriteStore, mustRun
**Closes:** create-dep-flag
**Tests:** a dep given by alias is stored as the canonical id; one given twice is stored once; a
missing dep exits non-zero and leaves no file; `dep list` of the target then lists the new issue.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Create|Dep'`
**Commit:** `feat(create): add --dep to file an issue with its dependencies`

## 7. create warns about an empty body

**What:**
**Goal:** `capcom create` with no body, or only whitespace, still succeeds and adds the warning
`<id> has no body: state what, why and a "## Acceptance" section with capcom update <id> --body-append-file <file>`,
in text (through the warning label) and in JSON `warnings`.
**Approach (assumed at the header base):** append to the `Result.Warnings` that `create` already fills
with `unwarnedTreeProblems`. Tests that pin empty warnings for a bodiless create are updated to give a
body or to expect the warning. Depends on item 6.
**Regression guard.** `TestCreate_KV_BesideUnparseableFile_PrintsWarningBeforeRecord` (`write_test.go:377`)
runs a bodiless `create --kv` and pins exactly one warning: give that create a `--body-file`. Leave
`assertKVWarningFirst` alone; `TestShow_KV_BesideUnparseableFile_PrintsWarningBeforeRecord` shares it.
**Files:** internal/cli/create.go, internal/cli/write_test.go, internal/cli/read_test.go
**Read first:** internal/cli/create.go — create, newIssue; internal/cli/root.go — app.emit; internal/cli/output.go — textRenderer.Render, kvRenderer.Render;
internal/cli/write_test.go — TestCreate_KV_BesideUnparseableFile_PrintsWarningBeforeRecord, TestCreate_Defaults_TaskOpenAndEmptyBody; internal/cli/read_test.go — assertKVWarningFirst
**Closes:** create-empty-body-hint
**Tests:** a bodiless create warns with the exact text; one with `--body-file` does not;
`TestCreate_KV_BesideUnparseableFile_PrintsWarningBeforeRecord` gains a `--body-file`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run Create`
**Commit:** `feat(create): warn when a new issue has no body`

## 8. Shared outline renderer; know tree shows where depth cut

**What:**
**Goal:** under each Note whose children `--depth` cut, `capcom know tree` prints one line, indented
one level deeper: `... <n> more (--depth <d>)`, where `d` is the depth that would show them. Children
hidden as retired are not counted. JSON `outlineData` gains `more` (int, omitted when 0).
**Approach (assumed at the header base):** the indented-outline rendering lives in one renderer that
items 9 and 10 reuse. Move the line building out of `app.outline` /
`newKnowTreeCommand` into a new `internal/cli/outline.go` (rows of depth, text and cut count), and add
its `doc.go` line. Keep `know tree` output byte-identical when nothing is cut.
**Regression guard.** `n` counts direct children only (2 under `testing-rules`, not 3).
`TestKnowTree_DepthAndStart_LimitTheOutline` pins `--depth 1` and `testing --depth 1` byte for byte;
both cases gain the hint (`  ... 2 more (--depth 2)` under testing-rules, `    ... 1 more (--depth 2)`
under cli-tests). A `newRetiredKnowStore` case pins that retired children are not counted.
**Files:** internal/cli/outline.go, internal/cli/doc.go, internal/cli/know.go, internal/cli/know_test.go
**Read first:** internal/cli/know.go — newKnowTreeCommand, app.outline, outlineData, childrenByParent; internal/index/notes.go — retiredFilter;
internal/cli/know_test.go — TestKnowTree_DepthAndStart_LimitTheOutline, newRetiredKnowStore; internal/cli/doc_test.go — TestDocFileMap_MatchesDirectory
**Closes:** outline-depth-hint
**Tests:** `know tree --depth 1` on a store with grandchildren prints the hint with the exact count and
`--depth 2`; the full depth prints no hint; JSON `more`; the two `--depth 1` cases of
`TestKnowTree_DepthAndStart_LimitTheOutline` updated as the guard says; on `newRetiredKnowStore`,
`know tree --depth 1` hints `2 more` under testing-rules (`legacy-runner` not counted).
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Know|DocFileMap'`
**Commit:** `feat(know): show where tree --depth cut children`

## 9. dep list walks to a depth

**What:**
**Goal:** `capcom dep list <id> --depth N` prints both sections, `deps:` and `dependents:`, as
indented trees N levels deep. Default 1 gives today's output byte for byte, 0 is unlimited, and a
negative value is a usage error worded as `know tree`'s. An issue already printed in a section is
printed again as `<id>  (see above)` without its subtree. Cut branches print item 8's hint. Missing
deps stay at depth 1 as today. JSON items gain `depth` and `via` (the id they hang under, "" at depth 1).
**Approach (assumed at the header base):** extend `newDepListCommand` and `depGraph.listResult` with
the walk from item 1 and the renderer from item 8. Depends on items 1 and 8.
**Regression guard.** dep list JSON items use a dep-list-specific wrapper type that embeds `issueData` and
adds `depth` and `via`; `issueData` itself is unchanged, so no other command's JSON changes. `(see above)`
prints at item 1's repeat steps. Depth 1 of `deps:` prints `entry.Deps` verbatim in file order
(duplicates and self-deps included), a missing one in place as `<dep>  missing`; then `dependents:` in tree order.
**Files:** internal/cli/dep.go, internal/cli/dep_test.go
**Read first:** internal/cli/dep.go — newDepListCommand, depGraph.listResult, depListData, loadDepGraph; internal/issue/deps.go — NewDepGraph, DepGraph.Dependents;
internal/cli/dep_test.go — newDepStore, TestDepList_Issue_ShowsDepsAndDependents; internal/cli/issue_render.go — issueData; internal/cli/ready.go — readyData
**Closes:** dep-list-depth
**Tests:** a three-level chain at `--depth 2` shows two levels and the hint; `--depth 0` shows all;
a diamond prints `(see above)`; the default output equals today's; `--depth -1` is a usage error; a
missing dep listed before a resolved one prints first; `list`/`show --json` items carry no `depth` or `via`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run Dep`
**Commit:** `feat(dep): walk dependencies and dependents to a depth`

## 10. list --tree prints the issue outline

**What:**
**Goal:** `capcom list --tree [--depth N] [--parent <id>] [--all]` prints the issue hierarchy as
`<indent>id  status  P<n>  title`, two spaces per level. A parent's line ends `  (<closed>/<total>
closed)`. Without `--all`, closed issues and their subtrees are left out but still counted in the
progress. `--depth` defaults to 0 (unlimited), and a cut prints item 8's hint. Any other row filter
with `--tree`, or `--depth` without `--tree`, is a usage error naming the flag. JSON: the item list
gains `depth`.
**Approach (assumed at the header base):** in `internal/cli/list.go`, build the rows from index entries
grouped by `ParentID` in tree order, the progress from item 5's helper, the text from item 8's
renderer. Depends on items 5 and 8.
**Regression guard.** list --tree JSON items use a list-specific wrapper type that embeds `issueData` and
adds `depth`; `issueData` itself is unchanged, so no other command's JSON changes. A row whose `ParentID`
names no listed entry (parent not indexed) prints as a root, never dropped. The `--parent` issue prints at
depth 0, as `know tree <id>` prints its start; the `--parent` flag help says so for `--tree`.
**Files:** internal/cli/list.go, internal/cli/read_test.go
**Read first:** internal/cli/list.go — newListCommand, listOptions, listFilter, checkListValues; internal/index/query.go — Filter, Index.List;
internal/cli/read_test.go — newReadStore, TestList_NoArgs_ListsOpenIssuesIndentedByDepth, TestList_BadFlags_ExitUsage; internal/cli/issue_render.go — issueLines, issueData
**Closes:** list-tree-outline
**Tests:** `newReadStore` prints `login-flow` with `session-refresh` indented and `(0/1 closed)`;
`--depth 1` prints the hint; `--parent login-flow` prints `login-flow` at depth 0 with its children below;
`--tree --label x` is a usage error; a child whose parent file is invalid prints as a root.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run List`
**Commit:** `feat(list): add --tree for the issue outline`

## 11. dep graph prints a Mermaid flowchart

**What:**
**Goal:** `capcom dep graph [<id>]` prints a Mermaid flowchart: `flowchart LR`, one node per issue
written `n<k>["<id> (<status>)"]` in node order, one edge `n<a> --> n<b>` per resolved dependency from
dependency to dependent. Without an id it covers every issue that is not closed. With an id, it covers
that issue and its transitive deps and dependents. `--all` adds closed issues. Missing deps are left
out. JSON (`Kind` "dep-graph"): `nodes` `[{id, status, category, title}]`, `edges` `[{dependent,
dependency}]`.
**Approach (assumed at the header base):** a `newDepGraphCommand` beside `newDepListCommand`, using
`loadDepGraph` and item 1's unlimited walk. Depends on item 1.
**Regression guard.** The `doc.go` file map line for `dep.go` ("capcom dep add, rm and list") and the
`newDepCommand` comment both name `graph` as well.
**Files:** internal/cli/dep.go, internal/cli/dep_test.go, internal/cli/doc.go
**Read first:** internal/cli/dep.go — newDepCommand, newDepListCommand, loadDepGraph, depGraph.names; internal/issue/deps.go — NewDepGraph, DepGraph.Resolve;
internal/cli/dep_test.go — newDepStore; internal/cli/list.go — flagAll; internal/cli/doc.go — file map
**Closes:** dep-graph-mermaid
**Tests:** `newDepStore` prints the exact flowchart; `<id>` limits it to the reachable issues; a closed
dep appears only with `--all`; JSON nodes and edges.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Dep|DocFileMap'`
**Commit:** `feat(dep): add dep graph, a Mermaid flowchart of the dependencies`

## 12. know tree hides background Notes

**What:**
**Goal:** a Note labelled `background` is left out of `capcom know tree`, together with its subtree, in
text and JSON, unless `--background` is given. A start Note named explicitly is printed even when it
is background, as retired Notes are. `--background` and `--retired` are independent. Hidden background
Notes are not counted in item 8's hints. The AGENTS block's top-level list skips background Notes as
it skips retired ones. `know show` and `know search` are unchanged.
**Approach (assumed at the header base):** a `knowledge.LabelBackground = "background"` constant; the
filter sits beside the retired check in `app.outline`/`childrenByParent` and in `agents.TopNotes`.
Depends on item 8.
**Regression guard.** Filter background Notes only in the know-tree path (`app.outline`, or the notes
list it builds its children map from), never inside `childrenByParent`, which also feeds `know show
--with-children`. This supersedes `adr-0010-rejected.md:17-18` ("nothing hides a labelled Note by
default", ratified call **Rejected ADR options**); item 13 amends that sentence.
**Files:** internal/knowledge/note.go, internal/cli/know.go, internal/cli/know_test.go, internal/agents/block.go, internal/agents/block_test.go
**Read first:** internal/cli/know.go — newKnowTreeCommand, outline, childrenByParent, newKnowShowCommand; internal/agents/block.go — TopNotes;
internal/index/notes.go — retiredFilter; internal/cli/know_test.go — TestKnowShow_WithChildren_RetiredChildrenLeftOutUnlessRetired; internal/agents/block_test.go — TestTopNotes_RetiredTopLevelNote_IsLeftOut
**Closes:** know-tree-background
**Tests:** a background child is hidden, then shown with `--background`; named as the start, it prints;
the hint counts exclude it; `TopNotes` skips a background top-level Note; `know show testing-rules
--with-children` still shows a background child.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run Know`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/agents/ ./internal/knowledge/`
**Commit:** `feat(know): hide background Notes from tree unless asked`

## 13. Mark rejected ADR options as background

**What:**
**Goal:** every `adr-NNNN-rejected` Note carries the label `background`. `adr-0010-decision` states that
a record's rejected-options Note carries it. Glossary term Note `background` (parent
`glossary-knowledge`) reads "Background Note: a Note labelled background, left out of know tree
unless asked for", in the shape of its sibling `retired`. The `## Note files` section of
`docs/format.md` documents the label.
**Approach (assumed at the header base):** `go run ./cmd/capcom know update <id> --add-label background` for
each of the eight; `go run ./cmd/capcom know add` for the term; then `go run ./cmd/capcom check` and
`go run ./cmd/capcom fmt --check`. Depends on item 12.
**Regression guard.** Every `capcom` command in this item's Approach and Acceptance runs the in-tree build
(`go run ./cmd/capcom …`), never the `capcom` on PATH (an older release). Every `docs/format.md` sentence
saying what `know tree` or the Managed block leaves out names background too (`grep -n 'left out\|leave out'
docs/format.md`: :351-352, :515). Amend `adr-0010-rejected.md`'s "Status as labels" sentence in place.
**Files:** .capcom/knowledge/decisions/ (the eight adr-*-rejected.md, adr-0010/adr-0010-decision.md), .capcom/knowledge/glossary/glossary-knowledge/background.md, docs/format.md
**Read first:** .capcom/knowledge/decisions/adr-0010/adr-0010-decision.md — "Retired and Supersession" paragraph; .capcom/knowledge/decisions/adr-0010/adr-0010-rejected.md — "Status as labels";
.capcom/knowledge/glossary/glossary-knowledge/retired.md — title shape; docs/format.md — "### Supersession", "## The Managed block in AGENTS.md"; internal/cli/know_update.go — updateNote
**Closes:** label-rejected-notes
**Tests:** none (content); `go run ./cmd/capcom know tree decisions` no longer lists a `-rejected` Note.
**Acceptance:** `go run ./cmd/capcom check`; `go run ./cmd/capcom fmt --check`; `! go run ./cmd/capcom know tree decisions | grep -q rejected`;
`grep -n background docs/format.md` prints lines; `go run ./cmd/capcom know show background` exits 0
**Commit:** `docs(knowledge): mark rejected ADR options as background`

## 14. AGENTS block: store map, dependency commands, issue-body rule

**What:**
**Goal:** the command list of the block `capcom agents` writes reads exactly:
`capcom ready` (unchanged); `` `capcom show <id>`: one issue in full, with what blocks it ``;
`` `capcom create --id <slug> --title <title> [--dep <id>]`: file a new issue ``;
`` `capcom dep add <id> <dep-id>`: make an issue wait for another; `capcom dep list <id>` shows both sides ``;
`` `capcom close <id> --reason <text>`: close an issue; it names the issues this made ready ``;
`` `capcom list --tree --depth 2`: the outline of the issues ``; the `know tree` line (unchanged).
After the list come two paragraphs. The first: "An issue's body says what is wrong or wanted, why,
and how to tell it is done (`## Acceptance`), readable on its own; a plan says how and in what order."
The second: "Where things are: an issue is `.capcom/issues/<id>.md`, and its children sit in the
directory `.capcom/issues/<id>/` beside it; Notes follow the same shape under `.capcom/knowledge/`.
Prefer capcom commands; after a hand edit run `capcom check`." The repo's `AGENTS.md` is refreshed
with `go run ./cmd/capcom agents`. The managed-block section of `docs/format.md` matches.
**Approach (assumed at the header base):** edit `blockIntro` in `internal/agents/block.go` and the text
pinned in `block_test.go`. Depends on items 2, 4, 6, 9, 10 and 12.
**Regression guard.** The `capcom agents` refresh and every check run the in-tree build (`go run
./cmd/capcom …`), never the `capcom` on PATH. The item's changelog entry (its sidecar) says, as
`CHANGELOG.md:75` did for the install line: an existing block reads as out of date after upgrading; run
`capcom agents` once (`capcom check` and `capcom agents --check` report it until then).
**Files:** internal/agents/block.go, internal/agents/block_test.go, internal/cli/agents_test.go, AGENTS.md, docs/format.md
**Read first:** internal/agents/block.go — blockIntro, Render, TopNotes; internal/agents/block_test.go — goldenBlock, preInstallLineBlock, TestInspect_States;
internal/cli/agents_test.go — TestCheck_AgentsBlockWithoutInstallLine_IsStaleUntilAgentsRefreshesIt; internal/check/check.go — agentsBlock; CHANGELOG.md — install-pointer entry
**Closes:** agents-block-map
**Tests:** the block test pins the new text; `go run ./cmd/capcom agents --check` (or the command's check mode) passes on the repo.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/agents/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run Agents`;
`go run ./cmd/capcom agents --check`; `go run ./cmd/capcom check`
**Commit:** `feat(agents): map the store and the dependency commands in the block`

## 15. Knowledge and docs tidy-ups from the review

**What:**
**Goal:**
- (#2) `docs/format.md` `## The tree` carries a worked example from this repo: `glossary/glossary-knowledge.md`
  with children in `glossary/glossary-knowledge/`, and `decisions/adr-0001/adr-0001-rejected.md`. It
  says a child id repeating its parent's id is a naming habit, not a rule.
- (#4) Note `docs-layout` says that `CONTEXT.md` and `docs/adr/README.md` only point to Notes, and
  that `docs/design/research-notes.md` is the pre-v1 research record, not current design.
- (#5) Note `glossary` says in one line that a flagged ambiguity is a word that means something
  else here.
- (#6) A new Note `store-config` (parent `store-layout`) says what `config.yaml` controls: statuses
  mapped to categories, types, plans, `ready.hide`, `lock.ttl`, `elsewhere.days`, and the
  format-version rule of ADR 0009. It points to `docs/format.md` `## Config` for the reference, with
  refs `docs/format.md` and `internal/config/config.go#FormatVersion`.
**Approach (assumed at the header base):** `go run ./cmd/capcom know update`/`know add`, a hand edit of
`docs/format.md`, then `go run ./cmd/capcom check` and `go run ./cmd/capcom fmt --check`.
**Regression guard.** Every `capcom` command this item runs uses the in-tree build (`go run ./cmd/capcom
…`), never the `capcom` on PATH. Acceptance greps the content: `grep -c 'naming habit' docs/format.md`,
`grep -c research-notes .capcom/knowledge/docs-layout.md` (both non-zero), and `know show store-config` exits 0.
**Files:** docs/format.md, .capcom/knowledge/docs-layout.md, .capcom/knowledge/glossary.md, .capcom/knowledge/store-layout/store-config.md
**Read first:** docs/format.md — `## The tree`, `## Config`, `### Refs`; .capcom/knowledge/store-layout.md — refs; .capcom/knowledge/docs-layout.md; .capcom/knowledge/glossary.md;
internal/cli/know.go — newKnowAddCommand, addNote; internal/config/config.go — FormatVersion
**Closes:** knowledge-docs-tidy
**Tests:** none (content).
**Acceptance:** `go run ./cmd/capcom check`; `go run ./cmd/capcom fmt --check`; `go run ./cmd/capcom know verify` lists no new broken ref;
`grep -c 'naming habit' docs/format.md` and `grep -c research-notes .capcom/knowledge/docs-layout.md` print non-zero; `go run ./cmd/capcom know show store-config` exits 0
**Commit:** `docs(knowledge): document naming, legacy docs and config semantics`

## 16. Note issue-workflow

**What:**
**Goal:** a top-level Note `issue-workflow`, titled "Before filing, starting or closing an issue: its
body, claiming it, dependencies", says:
- an issue's body is self-contained (what, why, `## Acceptance`), and its plan says how, so archiving
  a plan strands no issue;
- claiming work means moving the issue to a doing status (`capcom update <id> --status active` here)
  and holding the Tree lock with `--purpose <id>`, and `prime` lists the doing issues;
- `create --dep` and `dep add` record dependencies, `dep list --depth`, `dep graph` and `list --tree`
  show them, `close` names what it made ready, and `ready` ranks by what an issue unblocks.

`go run ./cmd/capcom agents` then refreshes `AGENTS.md`.
**Approach (assumed at the header base):** `go run ./cmd/capcom know add --id issue-workflow --title "<title>"
--body-file <file>`, then `go run ./cmd/capcom agents`. Depends on items 2, 3, 9, 10, 11 and 14.
**Regression guard.** Every `capcom` command this item runs (`know add`, `agents`, `check`) uses the in-tree
build (`go run ./cmd/capcom …`), never the `capcom` on PATH, whose `know add` refresh would revert item
14's block text. `know add` takes `--id`/`--title` flags, no positional id. Acceptance checks the body.
**Files:** .capcom/knowledge/issue-workflow.md, AGENTS.md
**Read first:** internal/cli/know.go — newKnowAddCommand, addNote, refreshAgentsAfterWrite; internal/agents/block.go — blockIntro, TopNotes; internal/check/check.go — agentsBlock;
internal/cli/lock.go — flagPurpose; .capcom/config.yaml — statuses (active: doing); .capcom/knowledge/glossary/glossary-coordination/tree-lock.md
**Closes:** issue-workflow-note
**Tests:** none (content).
**Acceptance:** `go run ./cmd/capcom check`; `go run ./cmd/capcom fmt --check`; `go run ./cmd/capcom know tree --depth 1 | grep -c issue-workflow` prints 1;
`go run ./cmd/capcom agents --check`; `go run ./cmd/capcom know show issue-workflow | grep -c -e '--purpose' -e 'dep graph' -e 'list --tree'` prints non-zero
**Commit:** `docs(knowledge): add the issue-workflow Note`

## 17. README and format reference for the new output

**What:**
**Goal:** the `## Usage` block of `README.md` shows `create --dep`, `dep list --depth 2`, `dep graph`
and `list --tree`. `docs/format.md` documents, beside its `### Related links` precedent:
- `close`'s `now ready:` line and `now_ready` field;
- `show`'s `ready:`, `dependents:` and `children:` lines and fields;
- `ready`'s order, its `-- unblocks N` suffix and `unblocks` field;
- `dep graph`'s output.
**Approach (assumed at the header base):** hand edits; wording follows the emitted strings. Depends on
items 2–5 and 9–11.
**Regression guard.** Also rewrite every sentence in docs/format.md that pins the order of `capcom show`'s
lines (e.g. that `related:` comes "after `deps:`", docs/format.md around line 148) to the new order, found
by grepping docs/format.md for `deps:` and `after`.
**Files:** README.md, docs/format.md
**Read first:** README.md — `## Usage`; docs/format.md — `### Related links`, `## Issue files`; internal/cli/issue_render.go — issueText, showData; internal/cli/close.go — closeData;
internal/cli/ready.go — readyLines, readyData; internal/cli/dep.go — newDepListCommand
**Closes:** cli-docs-update
**Tests:** none (docs).
**Acceptance:** `grep -n` finds in `README.md` `## Usage` each of `create --dep`, `dep list --depth 2`, `dep graph`,
`list --tree`; each of `now_ready`, `unblocks`, `ready`, `blocking`, `dependents`, `children`, `flowchart LR`,
`-- unblocks` occurs both in docs/format.md and in its emitting file (close.go, ready.go, issue_render.go, dep.go).
**Commit:** `docs: document the dependency output and the issue outline`
