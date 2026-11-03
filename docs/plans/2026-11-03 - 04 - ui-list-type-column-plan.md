# UI list type column

**Goal:** Show each issue's type as an aligned column in the three capcom ui issue lists (Ready, Issues, Closed), so a reviewer can tell the kind of work without opening the issue.
**Date:** 2026-11-03
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** d3855d2
**Closes:** ui-list-show-type

**Regression check (2026-11-03, d3855d2):**
- 1: guard folded (writer's decision) — the dependency view keeps `capcom dep list`'s row (no type cell at zero type width); raw-file fixtures with a config.yaml declaring a >10-cell type; `docs/design/tui.md` added to Files. Yields to tui.md:141-142 and depview.go:17,57.
- 2: guard folded — `docs/design/tui.md` added to Files and the comment rule widened to it; raw-file fixtures with a config.yaml declaring the long type; the pinned Closed sidebar test rewritten.

**Sources:**
- `.capcom/issues/ui-list-show-type.md`
- Notes `coding-conventions`, `cli-test-harness`

**Ratified design calls:**
- **Type column width:** the widest type in the list, capped at 10 cells; a longer label is cut to the width ending in `…` (Airic, 2026-11-03).
- **Closed tab:** gets the type column too, after the id: `<closed date>  id  type  title` (Airic, 2026-11-03).

**Standing requirements:**
- skills: coding-standards
- Tests on this host: single package, `GOMEMLIMIT=2GiB`, no `-race` on `./...`.

**Out of scope:**
- The detail pane header, the CLI list output, the Knowledge tab.
- Colouring the type.

## 1. Type column in the Ready and Issues lists

**What:**
**Goal:** Every Ready and Issues sidebar row reads `id  status  P<n>  type  title` (plus the Issues tree's progress suffix), the type column starting at one character position across all rows of the list; its width is the widest type in the list, at most 10 cells, and a longer type is cut to that width ending in `…`, the row's other columns unchanged.
**Approach (assumed at the header base):**
Extends `columnWidths` in `internal/ui/ready.go` with a type width: `widen` takes `min(max(c.type, StringWidth(entry.Type)), typeColumnMax)` where `typeColumnMax = 10` is a named const beside `readyColumnGap`. Add one helper `typeCell(entry, width) string` returning `fit(entry.Type, width) + padding(fitted, width)` (reuse `fit` from `layout.go` and `padding`; no new truncation code) — item 2 reuses it. `readyRow` inserts `typeCell(...) + readyColumnGap` between the `P<n>` and the title. `issueTreeRow` inherits it through `readyRow`; `columns.after` keeps narrowing only the id. Update the doc comments on `paintReadySidebar`, `paintIssuesSidebar`, `readyRow`, `issueTreeRow` and `columnWidths` that spell the row as `id  status  P<n>  title` (rule: every comment in `internal/ui` naming that row shape; `grep -n 'P<n>  title' internal/ui/*.go`).
**Regression guard.** The dependency view (depRow.text in internal/ui/depview.go) keeps today's row exactly as `capcom dep list` prints it, with no type column; readyRow omits the type cell and its gap when columns.type == 0 (the zero columnWidths pads nothing and adds nothing), and the deps-both-120x30 golden must not change. Fold the reviewer's other item-1 guards as written (raw-file fixtures with a config.yaml declaring a >10-cell type; docs/design/tui.md added to Files with its rule and grep). The item yields to the documented dependency-view row (`docs/design/tui.md:141-142`, `internal/ui/depview.go:17,57`). Docs rule: every `docs/design/tui.md` sentence naming a sidebar row's columns (`grep -n 'priorit' docs/design/tui.md`).
**Files:** `internal/ui/ready.go`, `internal/ui/issues.go`, `internal/ui/ready_test.go`, `internal/ui/issues_test.go`, `internal/ui/testdata/golden/*.golden` (those the change rewrites), `docs/design/tui.md`
**Read first:** `internal/ui/ready.go` — readyRow, columnWidths.widen; `internal/ui/depview.go` — depRow.text; `internal/ui/layout.go` — fit; `internal/ui/issues.go` — issueTreeRow;
`internal/ui/ready_test.go` — issueFile; `internal/ui/refresh_test.go` — reviewConfig; `docs/design/tui.md` — Issues tab
**Tests:**
- Fixtures: raw issue files with explicit types (not `issueFile`, which writes `type: task`), plus a `files["config.yaml"]` declaring a type longer than 10 cells (pattern: `reviewConfig`, `refresh_test.go`); an undeclared type is never indexed.
- `ready_test.go`: rows with types of different widths put the title at one column; a type longer than 10 cells renders as 9 cells + `…` and the row's title still starts at the shared column.
- `issues_test.go`: nested rows at different depths have their type at one column.
- Regenerate goldens with `go test ./internal/ui/ -run <golden tests> -update`, then inspect the diff: only the inserted type column changes; `deps-both-120x30.golden` does not change.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/`
- `go vet ./internal/ui/`
**Commit:** `feat(ui): show the issue type in the Ready and Issues lists`

## 2. Type column in the Closed list

**What:**
Depends on item 1 (`typeCell`, `typeColumnMax`).
**Goal:** Every Closed sidebar row reads `<closed date>  id  type  title`, the type and the title each starting at one character position across all rows; the type width follows item 1's rule (widest, at most 10 cells, cut with `…`).
**Approach (assumed at the header base):**
In `internal/ui/closed.go`, `paintClosedSidebar` measures a type width beside `idWidth` (capped at `typeColumnMax`) and passes it to `closedRow`, which inserts `typeCell(entry, typeWidth) + closedColumnGap` after the padded id. Update the doc comments naming `<closed date>  id  title` (`grep -n 'id  title' internal/ui/closed.go`).
**Regression guard.** `docs/design/tui.md:161` spells the Closed row `<closed date>  id  title`: update it with the closed.go comments, the rule widened to `grep -rn 'id  title' internal/ui/closed.go docs/design/tui.md`. The type-width tests use raw issue files and a `config.yaml` declaring the long type (`closedFiles` builds every row with `issueFile`, all `type: task`; undeclared types are not indexed), and `TestClosedTab_Sidebar_ListsTheClosedIssuesNewestFirst`, which pins today's rows, is rewritten with the type column.
**Files:** `internal/ui/closed.go`, `internal/ui/closed_test.go`, `internal/ui/testdata/golden/*.golden` (those the change rewrites), `docs/design/tui.md`
**Read first:** `internal/ui/closed.go` — paintClosedSidebar, closedRow, closedColumnGap; `internal/ui/closed_test.go` — closedFiles, TestClosedTab_Sidebar_ListsTheClosedIssuesNewestFirst;
`internal/ui/ready.go` — padding (and item 1's typeCell); `internal/ui/ready_test.go` — issueFile; `docs/design/tui.md` — Closed tab
**Tests:**
- Fixtures: raw issue files with explicit types plus a `files["config.yaml"]` declaring a type longer than 10 cells, as for item 1.
- `closed_test.go`: rows with types of different widths put the title at one column; an over-long type is cut with `…`.
- `closed_test.go`: rewrite `TestClosedTab_Sidebar_ListsTheClosedIssuesNewestFirst`'s expected rows to `<closed date>  id  type  title`.
- Regenerate any affected golden with `-update` and inspect the diff.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/`
- `go vet ./internal/ui/`
**Commit:** `feat(ui): show the issue type in the Closed list`
