# Plan: list --tree takes the row filters

**Goal:** `capcom list --tree` combines with every row filter but `--limit`, printing the matching
Issues and, marked as context, the ancestors that keep the outline readable. One call then lists a
Parent's descendants by status at every depth, in text and in `--json`.

**Date:** 2026-10-27
**Status:** unexecuted
**Sized for:** ~200k-context host
**base:** 0d87142
**Closes:** tree-status-filter

**Regression check (2026-10-27, 0d87142):**
- 1: guard folded; decision folded (Matching keeps filteredOutline's shown-rows-only rule)
- 2: recast; recast again (Approach base tree is `outline.Issues(entries, true)` under a filter); guard folded (Acceptance greps `[context]` in `--help`); supersedes CHANGELOG.md:265

**Sources:**
- `.capcom/issues/agent-ergonomics/tree-status-filter.md`
- `docs/site/reference/json-output.md`
- ADR 0012 (`capcom know show adr-0012`): the CLI and UI share logic below `internal/cli`

**Ratified design calls** (user, 2026-10-27):
- **Filters:** `--status`, `--category`, `--label`, `--no-label`, `--type`, `--priority`, `--plan`, `--planned`, `--unplanned` combine with `--tree`; only `--limit` stays refused.
- **Context marker:** a context row's text line ends `  [context]`, after any progress count; `--json` carries `"context": true` on it.
- **Closed rule:** as in the flat list, `--status`/`--category` decide by themselves whether Closed Issues are listed; other filters follow `--all`. A Closed ancestor of a match prints as context.

**Standing requirements:**
- skills: coding-standards
- Matching is the index's: the CLI asks `index.List` for the matching ids; no second matcher in Go.
- CLI tests go through `runCLI` and never call `t.Parallel()` (Note `cli-test-harness`).
- Tests on this host: one package at a time with `GOMEMLIMIT=2GiB`.

**Out of scope:**
- Marking context rows in `capcom ui` (it keeps its own rendering).
- Filters on `capcom know tree`.
- `--limit` with `--tree`.

## 1. outline: a filtered Tree that marks context rows — ✅ DONE (2026-10-27)

NOTES (2026-10-27): Matching copies the matched set (true entries only) and records an empty, non-nil set for a nil argument, so a Tree narrowed to nothing still marks its start row as context; added two tests beyond the plan's list (TestOutline_Matching_ChildrenStillIncludeEveryChild, TestOutline_NewTree_RowsAreNeverContext); the now-unused "cmp" import left internal/ui/filter.go.

**What:**
**Goal:** `internal/outline` offers one method that narrows a Tree to a set of matched ids plus
their shown ancestors, and its `Row` reports whether a row is context (shown only as an ancestor
of a match). `capcom ui` builds its filtered Issues and Knowledge trees through that method, with
no outline-filtering function left in `internal/ui`.
**Approach (assumed at the header base):** add `func (t *Tree[E]) Matching(matched map[string]bool) *Tree[E]`
to `internal/outline/outline.go`: it walks `t.Rows(nil, UnlimitedDepth)`, keeps every matched row
and its ancestors (the walk in `ui.filteredOutline`), and returns a Tree sharing `t`'s `children`,
`parentOf` and `idOf` (groups already ordered, so no compare is needed), with `isShown` = kept and
the matched set recorded. Add `IsContext bool` to `Row`: true when the Tree has a matched set and
the row's id is not in it, the start row of `Rows(start, …)` included. A Tree built by `NewTree`
has no matched set, so its rows are never context. Replace both calls of `filteredOutline` in
`internal/ui/filter.go` (`issuesTree`, `knowledgeTree`) with `.Matching(filter.matched)` and delete
`filteredOutline`; the UI ignores `IsContext`. `Children` still returns every child, so progress
counts are unchanged.

**Regression guard.** The receiver of `.Matching` is `filteredOutline`'s first argument:
`outline.Issues(entries, filter.isClosedShown).Matching(filter.matched)` in `issuesTree` (not its
unfiltered `outline.Issues(entries, false)`) and `noteOutline(notes).Matching(filter.matched)` in
`knowledgeTree`. Delete `entryParentID`, `noteParentID` and `compareNoteIDs` with `filteredOutline`; `entryID`/`noteID` stay.
Matching keeps filteredOutline's rule that only rows the receiver Tree shows can match; it must not widen or narrow that rule (item 2 and the UI depend on it).

**Files:** internal/outline/outline.go, internal/outline/outline_test.go, internal/ui/filter.go
**Read first:** internal/ui/filter.go — filteredOutline, issuesTree, knowledgeTree; internal/outline/outline.go — Tree, Rows,
walkOutline, Row; internal/ui/knowledge.go — noteOutline

**Tests:**
- `TestOutline_Matching_KeepsMatchesAndAncestors`: a three-level tree; matching one leaf keeps it and its ancestors, drops its siblings, marks the ancestors `IsContext`, the leaf not.
- `TestOutline_Matching_StartRowIsContextUnlessMatched` and `TestOutline_Matching_DepthCutCountsKeptChildren` (`More` counts kept children only).
- `TestOutline_Matching_IgnoresMatchesTheTreeHides`: a matched id the receiver Tree does not show (it or an ancestor hidden) is not kept, and neither are its ancestors on its account.
- The existing `internal/ui` filter tests pass unchanged.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/outline/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/`
- `grep -n "func filteredOutline" internal/ui/*.go` prints nothing.

**Commit:** `refactor(outline): narrow a Tree to its matches and mark context rows`

## 2. list --tree combines with the row filters — ✅ DONE (2026-10-27)

NOTES (2026-10-27): treeRowFilters was removed rather than shrunk and renamed: with --limit the only refused flag, checkTreeFlags checks it directly instead of looping over a one-entry slice.
NOTES (2026-10-27): the row-filter half of listFilter moved into a new rowFilter (no --parent, no --limit), which listFilter and the new issueOutline share; issueOutline asks index.ListIDs (the same query as index.List, ids only) for the matched ids.

Depends on item 1.

**What:** Recast at the regression check (2026-10-27).
**Goal:** `capcom list --tree` accepts every row filter but `--limit` and prints the matching
Issues with their ancestors; a non-matching row ends `  [context]` in text and carries
`"context": true` in `--json`. `(closed/total closed)` still counts every direct child, `--depth`
still cuts with `... <n> more` counting only rows the filter keeps, and `--limit` with `--tree`
still fails `--limit cannot be combined with --tree`. `capcom list --help` says how the filters
apply under `--tree`, and the committed CLI reference matches it.
**Approach (assumed at the header base):** in `internal/cli/list.go`, shrink `treeRowFilters` to
`--limit` alone (rename it to fit) and keep `checkTreeFlags` refusing it. In `issueTreeResult`,
when any of the nine filters is set: run `checkListValues`, build the index filter as `listFilter`
does but with `ParentID` empty and no limit (`--parent` keeps meaning "start the outline here"),
take the matched ids from `a.issueIndex.List`; with any row filter set, the base tree is
`outline.Issues(entries, true)` (every Issue shown) narrowed with `.Matching(matched)` (item 1);
which Closed Issues match is decided only by the index filter's IsClosedIncluded (--all, or
--status/--category given); without a filter the outline is today's
`outline.Issues(entries, isAll)`. In `issueTree.add`, append
`"  [context]"` to the text when `row.IsContext`, and add `Context bool \`json:"context,omitempty"\``
to `issueTreeItem`. Update `listLong`, `listExample` (one filtered tree example) and the `--tree`
flag help; then `go run ./cmd/gendocs` to refresh `docs/site/reference/cli/capcom_list.md`.

**Regression guard.** When any row filter is set, build the base tree with every Issue shown (outline.Issues(entries, true)) before narrowing it with Matching, so a match under a Closed Parent prints with that Parent as context (header Closed rule); which Closed Issues count as matches is still decided by the index filter (IsClosedIncluded = --all, or --status/--category given). Without a filter the outline is today's. TestListTree_BadFlags_UsageErrorNamesTheFlag in internal/cli/read_test.go is narrowed to the --limit case and the nine filter cases move to passing tests.
The `--help` acceptance greps text only the new help carries (`[context]`), since `--tree` already
appears in today's help. This supersedes CHANGELOG.md:265 ("A row filter ... with `--tree` ... is a
usage error"), reversed by the header's ratified Filters call; the item's `[Unreleased]` entry says so.

**Files:** internal/cli/list.go, internal/cli/read_test.go, docs/site/reference/cli/capcom_list.md
**Read first:** internal/cli/list.go — issueTreeResult, issueTree.add, issueTreeItem, checkTreeFlags, treeRowFilters, listFilter;
internal/ui/filter.go — filteredOutline; internal/outline/outline.go — Issues; internal/index/query.go — Filter.IsClosedIncluded;
internal/cli/read_test.go — TestListTree_BadFlags_UsageErrorNamesTheFlag, TestListTree_ClosedParent_LeavesOutItsSubtree

**Tests** (in `internal/cli/read_test.go`, through `runCLI`):
- `--tree --parent <p> --status open` prints every open descendant at every depth and their non-open ancestors with `  [context]`, and nothing else; a closed ancestor of an open grandchild prints as context.
- The same with `--json`: matches lack `context`, context rows have `"context": true`; parent `children` counts every child.
- `--tree --label <l>` without `--all` leaves out a closed labelled Issue; with `--all` it prints.
- `--tree --depth 1 --status open` prints `... <n> more` counting kept rows only.
- `--tree --limit 2` fails `--limit cannot be combined with --tree`; `--tree --status bogus` lists the declared statuses.
- `--tree --type <t>` without `--all`: an open match under a Closed (dropped) Parent prints, with that Parent as `  [context]`.
- `TestListTree_BadFlags_UsageErrorNamesTheFlag` is narrowed to the `--limit` case (its `--depth` cases stay); the nine filter cases each exit 0 in a passing test.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`
- `go run ./cmd/capcom list --help | grep -n -- '\[context\]'` prints a line.

**Commit:** `feat(list): combine --tree with the row filters, marking context rows`

## 3. Document the filtered outline

Depends on item 2.

**What:**
**Goal:** the user docs describe a filtered `list --tree`: `docs/site/reference/json-output.md`
documents the `context` field of `list --tree --json` and when it is left out, and
`docs/site/concepts/issues.md` shows one filtered outline with a `[context]` row after the plain
`capcom list --tree` example.
**Approach (assumed at the header base):** extend the `list --tree --json` paragraph of
`json-output.md` (the one naming `depth`, `children`, `more`) with `context`; add the example to
`issues.md` under the existing `capcom list --tree` console block, its output produced by running
the built binary on a matching throwaway store, never hand-written.

**Files:** docs/site/reference/json-output.md, docs/site/concepts/issues.md
**Read first:** docs/site/reference/json-output.md — the `list --tree --json` paragraph (`depth`, `children`, `more`, "left out when the text prints none of them"); docs/site/concepts/issues.md — the `capcom list --tree` console block under the Tree section; internal/cli/list.go — issueTreeItem

**Tests:** none (docs only).

**Acceptance:**
- `grep -n '"context"' docs/site/reference/json-output.md` prints a line.
- `grep -n "\[context\]" docs/site/concepts/issues.md` prints a line.

**Commit:** `docs(list): describe the filtered --tree outline and its context rows`
