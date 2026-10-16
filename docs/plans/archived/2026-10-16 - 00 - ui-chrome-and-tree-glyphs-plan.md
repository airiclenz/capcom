# capcom ui chrome and tree glyphs plan

**Goal:** Give `capcom ui` breathing room and clearer structure: margins around the pane divider, a
rule under the tab bar, apogee's scrollbar on both panes, and a tree drawn with ▼ ▶ · glyphs and
gray ┝ ┕ │ connectors.
**Date:** 2026-10-16
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** e3440c9

**Regression check (2026-10-16, e3440c9):**
- 1: guard folded
- 2: guard folded (writer's decision plus the stale-comment guard)
- 3: guard folded (writer's decision)
- 4: guard folded (writer's decision plus the stale-pair and row-cut guards)
- 5: guard folded
- 6: guard folded

**Sources:**
- `docs/design/tui.md`
- `/home/airic/repos/apogee/internal/tui/model.go` (`Model.renderScrollbar`, `Model.layout`), `boxdraw.go` (`scrollbarThumb`, `Model.joinScrollbar`), `theme.go` (`glyphScrollThumb`, `glyphScrollTrack`, `bodyRightGutter`)

**Ratified design calls** (user, 2026-10-16):
- **Divider:** the pane divider is ` │ ` — one blank column each side of `│`.
- **Tab rule:** the row under the tab bar is a `─` rule in the border colour with `┬` above the divider, mirroring the `┴` rule above the status bar.
- **Scrollbar panes:** both the sidebar and the detail pane get apogee's scrollbar.
- **Tree colour:** connectors use the existing `border` theme role; no new role.
- **Nesting:** a non-last ancestor continues as a gray `│`; a last ancestor leaves blanks.
- **Selection:** the selection and muted highlight start after the connectors; connectors stay gray.

**Standing requirements:**
- skills: coding-standards
- Tests: `GOMEMLIMIT=2GiB`, one package at a time; never `./...` with `-race` (AGENTS.md).
- Goldens regenerate with `GOMEMLIMIT=2GiB go test ./internal/ui/ -update`; the implementer reads the golden diff and confirms it shows only the item's change before committing.
- A deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- A setting to hide the scrollbar; clicking or dragging the scrollbar.
- `capcom list --tree` and `capcom dep` glyphs (CLI output is unchanged).
- The Ready tab's flat list glyphs; the help screen gets no scrollbar.
- Any version, tag or CHANGELOG heading change.

## 1. Margins around the pane divider — ✅ DONE (2026-10-16)

NOTES (2026-10-16): the divider helpers are `model.dividerColumn()` (the `│` column, also the rule's `┴` column, for item 2's `┬`) and `model.detailLeft()` (0 on a narrow terminal), with a `dividerMargin` constant; `paneSeparator` stays the bare `│`. `detailWidth()` is now `m.width - m.detailLeft()`. The mouse_test `detailLeft(m)` helper keeps an independent `sidebarWidth() + 3` oracle rather than calling the method.

**What:**
**Goal:** in the split layout every body row reads `<sidebar> │ <detail>` with exactly one blank
column each side of `│`; the detail pane's content (and markdown wrap width) starts 3 columns after
the sidebar; a click on either margin cell or the divider selects nothing; the `┴` joint in the rule
above the status bar sits under the `│`.
**Approach (assumed at the header base):** in `internal/ui/layout.go` replace `paneSeparator` use in
`body()` with a ` │ ` divider (only `│` styled by `theme.border`). The "+1 divider column" is spelled
in several places — `body()`, `rule()`, `pointAt()` in layout.go, `detailWidth()` in detail.go — so
introduce one helper (e.g. `detailLeft()` = `sidebarWidth()` + divider width) in layout.go and route
every site through it; no site keeps a literal `+1`/`-1` for the divider. `rule()` puts `┴` at the
`│` column. `pointAt` maps the three divider cells to `paneNone`.
**Regression guard.** The margin-click test clicks all three cells `sidebarWidth()`..`+2` on an
open dep-view row (`depView.rowAtLine` ignores the column), so it fails on the base tree, where the
`+1`/`+2` clicks select the row; a plain click on a cell with no link proves nothing.
**Files:** internal/ui/layout.go, internal/ui/detail.go, internal/ui/layout_test.go, internal/ui/mouse_test.go, internal/ui/testdata/golden/*
**Read first:** internal/ui/layout.go — body, rule, pointAt, sidebarWidth; internal/ui/detail.go — detailWidth;
internal/ui/mouse_test.go — detailLeft, TestMouse_ClickOnDepViewRow_SelectsItForEnter; internal/ui/depview.go — depView.rowAtLine
**Tests:** layout_test: a split view's body rows hold ` │ ` at columns `sidebarWidth()`..`+2` and
the rule's `┴` at `sidebarWidth()+1`; mouse_test: update the `detailLeft()` helper, a click at
`detailLeft()` lands on detail column 0, clicks on each of the three cells `sidebarWidth()`..`+2`
of an open dep-view row select nothing; regenerate goldens.
**Acceptance:**
- `go build ./... && go vet ./internal/ui/`
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
**Commit:** `feat(ui): pad the pane divider with a margin on each side`

## 2. Rule under the tab bar — ✅ DONE (2026-10-16)

NOTES (2026-10-16): `rule()` now takes its joint glyph (`rule(topRuleJoint)` for `┬` under the tab bar, `rule(ruleJoint)` for `┴` under the panes), its `┬` column coming from item 1's `dividerColumn()`; the existing `ruleJoint` name is kept for the bottom joint rather than renamed. A new `bodyTop = 2` constant names the first body row; `pointAt` returns `paneNone` for `row := y - bodyTop` below 0, and mouse_test's `linkCell` uses it.
NOTES (2026-10-16): consequential edit — internal/ui/theme.go: made necessary by the new top rule (the `border` field's comment now names the rules above and under the panes).
NOTES (2026-10-16): docs/design/tui.md and the manual's screen samples are left to item 6, which owns them.

Depends on item 1.

**What:**
**Goal:** row 1 of the screen is a `─` rule in the border colour under the tab bar, with `┬` in the
divider's `│` column whenever the split layout shows (no joint in the narrow or help views, the
same cases where the bottom rule has no `┴`); the panes start on row 2; `bodyHeight()` is the
screen height minus 4; a click or wheel on row 1 does nothing.
**Approach (assumed at the header base):** in `internal/ui/layout.go` bump `frameRows` to 4 (update
its comment), add the rule in `frame()` after the tab bar, and share one rule painter between the
top and bottom rules, parameterised by the joint glyph (`┬`/`┴`), so the joint-placement logic of
`rule()` is not copied. `pointAt` treats `y == 1` as `paneNone` and body rows as `y-2`. Every
body-row offset in mouse_test (`click(x, 1)` for the first body row, `linkCell()`'s `1 + …`) moves
by one.
**Regression guard.** the top rule's ┬ column comes from item 1's divider helper (the │ column), never from sidebarWidth() or a literal offset; every hard-coded body-row y in mouse_test (clicks and wheels, and comments naming a row) shifts by one.
Every comment in internal/ui that places the body or a rule relative to the tab bar or top row is
updated (`grep -n 'tab bar\|top row\|under the panes' internal/ui/*.go`: `bodyHeight`, `body`,
`rule`, `pointAt`, not only `frameRows`).
**Files:** internal/ui/layout.go, internal/ui/layout_test.go, internal/ui/mouse_test.go, internal/ui/testdata/golden/*
**Read first:** internal/ui/layout.go — frame, frameRows, bodyHeight, rule, pointAt; internal/ui/mouse_test.go — click, linkCell;
internal/ui/layout_test.go — TestModel_ScrollKeys_MoveTheDetailPaneWithinItsLines
**Tests:** layout_test: row 1 of a split view is all `─` but `┬` at the divider column; row 1 of a
narrow view has no `┬`; `bodyHeight()` == height-4; `TestModel_ScrollKeys_MoveTheDetailPaneWithinItsLines`
expectations follow the new body height; mouse_test: a click on row 1 changes nothing and the
first body row is `y == 2`; every hard-coded body-row y (clicks, wheels, row-naming comments such
as "Row 6") moves down by one; regenerate goldens.
**Acceptance:**
- `go build ./... && go vet ./internal/ui/`
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
**Commit:** `feat(ui): set the tabs off with a rule`

## 3. Scrollbar on the detail pane — ✅ DONE (2026-10-16)

NOTES (2026-10-16): the scrollbar glyphs are named `scrollThumbGlyph`/`scrollTrackGlyph` (beside layout.go's frame glyph names) rather than apogee's `glyphScrollThumb`/`glyphScrollTrack`; the row painter is `model.scrolledPaneRows` with `squareRow` cutting and padding in `m.markdownTheme.Measure`, and `scrollbarCells` paints the bar, all in scrollbar.go, so item 4 can reuse them for the sidebar.
NOTES (2026-10-16): internal/ui/layout_test.go (not in Files) — `TestModel_SplitLayout_PadsTheDividerWithAMarginEachSide` asserted `detailWidth() == 120-sidebarWidth-3`; the item's own `detailWidth()` shrink by 2 changes that value, so the assertion now subtracts `scrollbarColumns`.
NOTES (2026-10-16): the regenerated goldens differ only in trailing blanks (`git diff --ignore-space-at-eol` over them is empty): every golden's detail body fits, so the bar column is reserved and blank; the visible thumb and track are asserted by scrollbar_test instead. layout-80x24 and help-120x30 are unchanged (no detail pane on screen).
NOTES (2026-10-16): docs/design/tui.md and the manual's screen samples are left to item 6, which owns them.

Depends on item 2.

**What:**
**Goal:** the detail pane's last column is a scrollbar ported from apogee: 1 column wide, `┃` thumb
(muted colour) on a `│` track (border colour), with one blank gutter column between the content and
the bar; when the shown lines fit, the column stays reserved and blank; the thumb is at least one
row, flush top at offset 0 and flush bottom at the last offset. This holds in the split and narrow
detail views, for the markdown body, the dep view and the status picker alike.
**Approach (assumed at the header base):** new `internal/ui/scrollbar.go` holding a port of apogee's
`scrollbarThumb(start, window, total, height) (pos, size)` and a painter returning `height` cells for
(offset, total, height) using new `theme` styles `scrollThumb` (muted fg) and `scrollTrack` (border
fg) in theme.go. `body()` reserves gutter+bar (2 columns) from the detail pane and appends the bar
cell to each detail row, padding (and cutting) rows in the painter's width measure
(`m.markdownTheme.Measure`, as apogee's `joinScrollbar` pads in its theme's measure), not
`ansi.StringWidth` and not `lipgloss.JoinHorizontal`. `detailWidth()` shrinks by 2 so markdown wraps short of the gutter.
Total/offset come from the lines `body()` draws in the detail pane and the offset it slices them at
(`m.detailOffset`). Clicks on the gutter or bar select nothing; the wheel still scrolls the detail.
**Regression guard.** add internal/ui/mouse.go to Files — the gutter/bar column check lives in the click handlers (clickDetail), not in pointAt, so pointAt still routes the wheel to the detail pane; rows are padded to the bar with the same width measure the painters truncate with, so the bar holds one column on every row
**Files:** internal/ui/scrollbar.go, internal/ui/scrollbar_test.go, internal/ui/theme.go, internal/ui/layout.go, internal/ui/detail.go, internal/ui/mouse.go, internal/ui/mouse_test.go, internal/ui/testdata/golden/*
**Read first:** internal/ui/layout.go — body, detailRows, paneRows, pad; internal/ui/detail.go — detailWidth;
internal/ui/mouse.go — clickDetail; internal/ui/markdown/width.go — Measure.Width, Measure.Truncate
**Tests:** scrollbar_test: `scrollbarThumb` table (fits → 0,0; min size 1; flush bottom at max
offset; flush top at 0); a long detail body paints only `┃`/`│` in the last column with both
present, a short one paints blanks there; detail text never reaches the last 2 columns; narrow
detail view likewise; a body line holding `⚠️` under the default WcWidth measure keeps its bar cell
in the last column; mouse_test: a wheel on the bar column scrolls the detail, a click on a dep
row's gutter or bar cell selects nothing; regenerate goldens (theme goldens keep ANSI and change).
**Acceptance:**
- `go build ./... && go vet ./internal/ui/`
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
**Commit:** `feat(ui): paint a scrollbar beside the detail pane`

## 4. Scrollbar on the sidebar — ✅ DONE (2026-10-16)

NOTES (2026-10-16): re-derived from the assumption that item 3's painter could take the sidebar's pre-sliced rows as is: `scrolledPaneRows` slices its lines at the bar's offset, so internal/ui/scrollbar.go splits out `barredRows(lines, cells, width)` (squaring rows in `m.markdownTheme.Measure`, then gutter and cell), which `scrolledPaneRows` and the new `sidebarRows` in layout.go both call; no second copy of the painter.
NOTES (2026-10-16): top/total live on the model as `sidebarTop`/`sidebarTotal`, set with `sidebar` through one `withSidebar` helper in model.go; `treeWidget.paint` is unchanged (the paint sites read `tree.top` and `len(tree.rows)` after painting), so internal/ui/tree.go is not touched.
NOTES (2026-10-16): internal/ui/keys.go (not in Files) — `openTab` assigns `m.sidebar = nil`; per the guard that every assignment sets the pair, it now calls `withSidebar(nil, 0, 0)`.
NOTES (2026-10-16): `pad` in layout.go is deleted: its one caller, the split join, now takes the sidebar rows already squared by `barredRows`.
NOTES (2026-10-16): the tab-switch test switches with `3` (Ready to an empty Knowledge tab) rather than Tab, since Tab from a long Ready list lands on an equally long Issues tree.
NOTES (2026-10-16): the regenerated goldens differ only in four sidebar rows cut two columns earlier (`(2/2 clos…` becomes `(2/2 cl…`) and in trailing blanks (layout-80x24's narrow rows are now padded to the bar); theme goldens are unchanged since their lists fit. docs/design/tui.md and the manual's samples are left to item 6.

Depends on item 3.

**What:**
**Goal:** the sidebar (Ready list, Issues and Knowledge trees, split and narrow views) ends in the
same gutter+scrollbar as the detail pane, so a split row reads `<sidebar text> ┃ │ <detail text> ┃`;
the bar spans the list's rows (below the filter line while it is open, whose bar cell is blank) and
reflects the list's top row and total rows; sidebar text never reaches its last 2 columns.
**Approach (assumed at the header base):** the sidebar is pre-sliced at paint time, so the painters
(`paintReadySidebar` in ready.go via `m.ready.top`/`len(m.ready.entries)`, and `treeWidget.paint` in
tree.go via `w.top`/`len(w.rows)`) must hand top and total to `body()` alongside the rows — store them
on the model next to `m.sidebar`. The list rows are cut in `body()`: `paneRows` at
`sidebarWidth()-2` (split) and `m.width-2` (narrow); `sidebarPaneWidth()` in filter.go only sizes
the filter line, and returns the width minus 2 to keep it short of the gutter. Reuse item 3's painter; no second copy. Clicks on the sidebar's gutter
or bar select nothing; the wheel still scrolls the sidebar.
**Regression guard.** add internal/ui/mouse.go to Files — the sidebar gutter/bar column check lives in clickSidebar, not in pointAt, so the wheel still scrolls the sidebar; reuse item 3's row padding for the sidebar's join, replacing the sidebar's own pad so both panes measure width the same way
Every branch that assigns `m.sidebar` sets the top/total pair too — the error and empty branches
of `paintReadySidebar`, `paintIssuesSidebar` and `paintKnowledgeSidebar` (single-row text: total 1,
top 0) — so a thumb never outlives the list it measured.
**Files:** internal/ui/layout.go, internal/ui/filter.go, internal/ui/ready.go, internal/ui/issues.go, internal/ui/knowledge.go, internal/ui/tree.go, internal/ui/model.go, internal/ui/mouse.go, internal/ui/mouse_test.go, internal/ui/scrollbar_test.go, internal/ui/testdata/golden/*
**Read first:** internal/ui/layout.go — body, paneRows, pointAt; internal/ui/ready.go — paintReadySidebar;
internal/ui/tree.go — treeWidget.paint; internal/ui/filter.go — sidebarPaneWidth; internal/ui/mouse.go — clickSidebar, wheelSidebar
**Tests:** a Ready list longer than the body shows `┃`/`│` in the sidebar's last column and the
thumb flush bottom with the last entry selected; a short list shows blanks there; with the filter
open the filter row's bar cell is blank; a tree likewise; Tab from a long Ready list to a tab still
loading or empty paints no thumb; mouse_test: a click on the sidebar's gutter or bar cell selects
nothing and a wheel there still moves the selection; regenerate goldens.
**Acceptance:**
- `go build ./... && go vet ./internal/ui/`
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
**Commit:** `feat(ui): paint a scrollbar beside the sidebar`

## 5. Tree glyphs and connectors — ✅ DONE (2026-10-16)

NOTES (2026-10-16): internal/ui/mouse_test.go (in Files) is unchanged: the arrow and text click tests click depth-0 rows, whose columns the change keeps, and pass as they are.
NOTES (2026-10-16): `addRows` gains an `ancestors` parameter and filters the visible children first through a new `visibleChildren`, so "last visible child" accounts for hidden and revealed siblings; `rowConnector` and `childAncestors` build the cells, and `connectorWidth` (2) replaces `treeIndent` as the arrow's column step in `click`.
NOTES (2026-10-16): the regenerated goldens differ only in the tree rows' glyphs and connectors (every row stays 120/150 columns, the divider column unchanged); the theme ANSI goldens are unchanged. docs/design/tui.md and docs/site/getting-started/terminal-ui.md still show ▾ ▸ and are left to item 6.

**What:**
**Goal:** Issues and Knowledge trees draw an expanded node as `▼ `, a collapsed node with children
as `▶ `, a leaf as `· `; a row at depth d ≥ 1 is prefixed by d-1 ancestor cells (`│ ` when that
ancestor has a later visible sibling, else two blanks) and then `┝ ` (a later visible sibling
follows) or `┕ ` (last visible child); depth-0 rows have no connector. Connectors render in the
`border` style and sit outside the selection/muted highlight, which covers glyph and text only.
Example (expanded root with an expanded last child):
`▼ main item` / `┝ ▶ collapsed sub item` / `┕ ▼ expanded sub item` / `  ┝ · leaf one` / `  ┕ · leaf two`.
The glyph keeps column `2*depth`: a click on columns `2*depth` and `2*depth+1` of a row with
children expands or collapses it.
**Approach (assumed at the header base):** in `internal/ui/tree.go` change `arrowExpanded`,
`arrowCollapsed`, `arrowLeaf`; drop `treeIndent` as the indent source in `paint()`. Compute each
row's unstyled connector prefix in `addRows()` (it walks `w.children(parentID)` after muted/reveal
filtering, so "last visible child" is known there) and keep it on `treeRow`. `paint()` emits
`th.border.Render(prefix)` then the marked style around glyph+`rowText` only. `click()` keeps the
glyph at `Depth*2` with the glyph's width.
**Regression guard.** `paint()` renders the border prefix only when it is non-empty (`Render("")`
emits an empty SGR pair), so depth-0 rows and the ANSI theme goldens change only by the glyph. The
paint test compares `ansi.Strip(painted[i])` exactly, not `strings.Contains`, against a new outline
fixture with the example's shape. The `border` field's comment in theme.go names the tree connectors.
**Files:** internal/ui/tree.go, internal/ui/tree_test.go, internal/ui/theme.go, internal/ui/mouse_test.go, internal/ui/testdata/golden/*
**Read first:** internal/ui/tree.go — addRows, paint, click, treeRow; internal/ui/tree_test.go — TestTreeWidget_Paint_IndentsAndMarksTheArrows, treeEntries;
internal/ui/theme.go — theme.border; internal/ui/theme_test.go — TestTheme_Presets_MatchANSIGoldens
**Tests:** tree_test: replace `TestTreeWidget_Paint_IndentsAndMarksTheArrows` expectations, compared
exactly on `ansi.Strip` rows, with the example above on a new outline fixture of that shape (`treeEntries`
cannot form it) plus a `│ ` continuation case (non-last expanded child with children); a muted last
child changes which sibling gets `┕`; the selected row's ANSI shows the connector outside the
selection style; mouse_test: arrow/text click tests still pass at their columns; regenerate goldens.
**Acceptance:**
- `go build ./... && go vet ./internal/ui/`
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
**Commit:** `feat(ui): draw trees with ▼ ▶ · glyphs and gray connectors`

## 6. Docs: layout samples and glyphs — ✅ DONE (2026-10-16)

NOTES (2026-10-16): the Layout diagram in docs/design/tui.md lost its outer ┌┐└┘ frame, which the UI never drew; both samples now mirror the golden rendering (no leading column, ─┬─ tab rule, ─┴─ status rule, ` │ ` divider, ┃/│ scrollbars on both panes, ▼ ▶ · with ┝ ┕ connectors)
NOTES (2026-10-16): added Divider and Scrollbars bullets and glyph sentences to both Layout/The screen lists, and a "Ported: the scrollbar" line under tui.md's "Taken from apogee", which item 3's port left out

Depends on items 1–5.

**What:**
**Goal:** every doc sample or sentence describing the `capcom ui` screen shows the ` │ ` divider,
the `─┬─` tab rule, the scrollbars, and the ▼ ▶ · glyphs with ┝ ┕ │ connectors; none mentions `▾`
or `▸`.
**Approach (assumed at the header base):** update the Layout diagram in `docs/design/tui.md` and the
layout sample in `docs/site/getting-started/terminal-ui.md`; find every other site with
`grep -rn '▾\|▸\|paneSeparator' docs/ README.md --exclude-dir=archived --exclude-dir=plans --exclude-dir=skill-runs` and fix each.
**Regression guard.** Both greps exclude `plans` and `skill-runs` as well as `archived`: this plan
and its run sidecars quote `▾`/`▸` and stay unarchived while item 6 runs, so an unnarrowed check
can never print nothing.
**Files:** docs/design/tui.md, docs/site/getting-started/terminal-ui.md
**Read first:** docs/design/tui.md — Layout, Mouse; docs/site/getting-started/terminal-ui.md — The screen, Mouse;
docs/plans/2026-10-16 - 00 - ui-chrome-and-tree-glyphs-plan.md — item 6 Acceptance
**Tests:** none (docs only).
**Acceptance:**
- `grep -rn '▾\|▸' docs/ README.md --exclude-dir=archived --exclude-dir=plans --exclude-dir=skill-runs` prints nothing
- `grep -c '┝' docs/design/tui.md docs/site/getting-started/terminal-ui.md` prints ≥ 1 for each
**Commit:** `docs(ui): show the new divider, tab rule, scrollbars and tree glyphs`
