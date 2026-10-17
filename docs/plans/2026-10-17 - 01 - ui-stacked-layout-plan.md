# capcom ui stacks the panes on a narrow, tall terminal

**Goal:** below 90 columns, a terminal tall enough for both panes shows the sidebar on top, a
horizontal divider, and the detail pane below; the divider moves by drag and `<` `>`, and its
position is remembered as `ui.stacked_ratio` in `state.yaml`, apart from `ui.sidebar_ratio`.
**Date:** 2026-10-17
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 5186d6d
**Closes:** ui-stacked-layout

**Sources:**
- `.capcom/issues/ui-stacked-layout.md`
- Notes `adr-0014`, `adr-0014-decision`
- `internal/ui/layout.go`, `internal/ui/mouse.go`, `internal/ui/keys.go`, `internal/ui/model.go`, `internal/userconfig/state.go`

**Ratified design calls** (user, 2026-10-17):
- **Thresholds:** stacked needs the sidebar ≥ 8 rows and the detail pane ≥ 12, plus the divider row: a body of ≥ 21 rows (terminal ≥ 25 rows) below 90 columns. 80×24 keeps the one-pane layout. The same minimums bound the divider.
- **Default position:** the sidebar takes a third of the body's rows, rounded down, clamped to the minimums.
- **Keys:** `<` and `>` move whichever divider is on screen; on the horizontal one `<` shrinks the sidebar (divider up) and `>` grows it, 2 rows a press.
- **Divider:** one full-width `─` row in the border style, drawn in `dividerActive` while dragged; that row alone is the hit area; a double-click resets it to the default and clears `stacked_ratio`.
- **Layout switch:** live on every resize, from the terminal's size alone; resizing never changes either stored ratio.

**Standing requirements:**
- skills: coding-standards
- Tests per `AGENTS.md`: single packages with `GOMEMLIMIT=2GiB`, never `./...` with `-race`, never two test commands at once.
- No test writes under the real `os.UserConfigDir`: test options set `UserStatePath` to a temp dir.
- Any authorized deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- Stacking on a wide terminal (≥ 90 columns), however tall.
- A per-worktree divider position; moving either ratio out of `state.yaml`.
- Changing the help screen, the one-pane layout or the side-by-side layout.

**Regression check (2026-10-17, 5186d6d):**
- 1: guard folded
- 2: guard folded
- 3: guard folded (writer decision on narrow test heights; supersedes the in-code one-pane-below-90 comments)
- 4: guard folded (writer decision on the save notice and abandoned divider changes; supersedes the CHANGELOG 0.8.0 resize-keys entry and adr-0014-decision.md:29)
- 5: guard folded

## 1. `ui.stacked_ratio` in the state file — ✅ DONE (2026-10-17)

NOTES (2026-10-17): the shared parser is `parser.ratio(node, key, measure, target)` (was `sidebarRatio`); its error noun is named `measure` and comes from the constants `sidebarRatioMeasure` ("terminal's width") and `stackedRatioMeasure` ("body's height"); SaveState and encodeState share one `storedRatios` list (sidebar_ratio first) so each names or writes both ratios from one place.

**What:**
**Goal:** `userconfig.UIState` carries `StackedRatio float64` (the sidebar's rows over the body's, 0 for none); `LoadState` reads `ui.stacked_ratio` and `SaveState` writes it, each beside `ui.sidebar_ratio` and validated as it is; a file without the key loads with `StackedRatio` 0.

**Regression guard.** The shared parser's nouns are `terminal's width` / `body's height` (not `width`), so `ui.sidebar_ratio`'s error stays byte-identical and `ratioMessage` (state_test.go) passes unedited. Every comment in state.go naming `sidebar_ratio`/`SidebarRatio` as the only key or checked field (`LoadState`, `SaveState`, `isStorableRatio`, the parser's doc, beside `State`/`UIState`) names both; find them with `grep -n 'sidebar_ratio\|SidebarRatio' internal/userconfig/state.go`.

**Approach (assumed at the header base):**
- Extends `internal/userconfig/state.go`; no new file. Add `keyStackedRatio = "stacked_ratio"`; `stateUI` dispatches both keys; the ratio parser is shared by both keys, taking the key and the error's noun (`width` / `body's height`), never copied.
- Error for a bad value: `ui.stacked_ratio: expected a fraction of the body's height, more than 0 and less than 1`, through `errorAt` (path and line).
- `encodeState` writes `ui:` once with each non-zero key (`sidebar_ratio` first); `SaveState` refuses an unstorable value in either field, naming that field.
- Update the `State`/`UIState` doc comments to name both ratios.

**Files:** internal/userconfig/state.go, internal/userconfig/state_test.go
**Read first:** internal/userconfig/state.go — sidebarRatio, stateUI, encodeState, SaveState, isStorableRatio; internal/userconfig/state_test.go — ratioMessage, TestSaveState_RatioOutOfRange_ErrorsAndLeavesTheFile; internal/userconfig/userconfig.go — parser.errorAt

**Tests:** `ratioMessage` unchanged and green; round trip with only `SidebarRatio`, only `StackedRatio`, both; a file with only `sidebar_ratio` loads `StackedRatio` 0; `stacked_ratio` of `0`, `1`, `2`, `.nan`, `"0.3"` → `*Error` naming path, line and `ui.stacked_ratio`; `SaveState` with `StackedRatio` 1 or NaN errors and leaves the file untouched.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/userconfig/`
- `go vet ./internal/userconfig/`

**Commit:** `feat(userconfig): store the stacked divider's position as ui.stacked_ratio`

## 2. The UI keeps both ratios and saves them together — ✅ DONE (2026-10-17)

Depends on item 1.

**What:**
**Goal:** the model starts on the stored `StackedRatio` as well as `SidebarRatio`, and every state-file save writes both ratios the model holds, so saving one never drops the other.

**Regression guard.** The acceptance run covers every test through the changed `saveState`: `-run 'State|Ratio|Resize|SaveCommands|Divider'` (the reverse-order save test and the drag/double-click save tests in mouse_test.go). Every comment in model.go describing the saver, the load or the save as carrying the sidebar ratio alone (the `sidebarRatio`/`stateSaver` field docs, `loadUserState`, `newModel`, `stateSaver`, `save`, `write`, `saveState`) names both; find them with `grep -n 'sidebar ratio\|sidebarRatio\|ratio' internal/ui/model.go`.

**Approach (assumed at the header base):**
- `internal/ui/model.go`: add `stackedRatio float64` beside `sidebarRatio` (doc: the sidebar's rows over the body's in the stacked layout, 0 for the default). `loadUserState` returns the whole `userconfig.UIState`; `newModel` sets both fields.
- `stateSaver` keeps the latest `userconfig.UIState` asked for instead of one ratio; `save` takes a `UIState`; `model.saveState` passes both fields. The out-of-order guard (`asked`/`written`) is unchanged.
- No layout change yet.

**Files:** internal/ui/model.go, internal/ui/layout_test.go
**Read first:** internal/ui/model.go — stateSaver, stateSaver.save, stateSaver.write, model.saveState, loadUserState, newModel; internal/ui/layout_test.go — TestModel_SaveCommands_RunInReverseOrder_LeaveTheLaterWidth, readStoredRatio

**Tests:** a stored file with both keys starts the model on both; a `>` in the split layout saves a file that still holds the stored `stacked_ratio`; `TestModel_SaveCommands_RunInReverseOrder_LeaveTheLaterWidth` and the divider drag/double-click save tests in mouse_test.go still pass.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/ -run 'State|Ratio|Resize|SaveCommands|Divider'`
- `go vet ./internal/ui/`

**Commit:** `refactor(ui): keep and save both divider ratios together`

## 3. Draw the stacked layout — ✅ DONE (2026-10-17)

NOTES (2026-10-17): consequential edit — internal/ui/model.go: made necessary by isNarrow → isSinglePane; the isDetailOpen field comment now names the one-pane layout and says the stacked layout ignores it.

NOTES (2026-10-17): renamed the local isNarrowDetail in keys.go handleKey to isOnePaneDetail so `grep -n isNarrow internal/ui/*.go` is empty, as the item's approach requires.

NOTES (2026-10-17): beyond the named isSinglePane/isStacked, layout.go adds canStack (narrow and tall enough, help ignored, so sidebarPaneRows/detailPaneRows and repaints under an open help screen keep the stacked sizes), hasFullWidthPanes (narrow; used by sidebarPaneWidth and detailLeft, per the regression guard), stackedPointAt, stackedBody, and the constants stackDividerRows = 1 and stackBodyMinRows = 21.

NOTES (2026-10-17): the help screen's "in the narrow detail view" line (help.go) is left as it is: the help screen is out of scope and the line stays true of the one-pane layout.

Depends on item 2.

**What:**
**Goal:** below 90 columns with a body of ≥ 21 rows the body draws the sidebar (top), one `─` rule row, and the detail pane (bottom), each with its scrollbar, every key and click except `<`, `>` and the divider (item 4) acting as in the side-by-side layout; below 21 body rows the one-pane layout is drawn, and at ≥ 90 columns the side-by-side one, switching live on resize.

**Regression guard.** every existing internal/ui test that means the one-pane layout at a narrow width runs at a height of 24 rows or less (the 30-row narrow sites in layout_test.go, filter_test.go and statusbar_test.go move to 24, found with a grep for narrow widths below 90 at heights of 25 or more), and those test files join the item's Files line.
Overriding the Approach's `isNarrow` → `isSinglePane` list: `resizeSidebar` guards on `!m.isSplit()`, so `>` at 80×40 saves nothing; `sidebarPaneWidth` and `detailLeft` test `m.width < narrowWidth` (or a helper true for single-pane or stacked, not `!isSplit()`, and not matching `grep isNarrow`). The test helpers' `isNarrow` calls (mouse_test.go `detailLeft`, `openDetail`, the test body at :166) move too, `openDetail` to `isSinglePane`; lower-pane click/wheel tests take the detail pane's top as `bodyTop+stackedSidebarRows()+1`, not `linkCell`'s `bodyTop`.
Supersedes the in-code comments stating one pane below 90 columns (layout.go:11-12, :121-123, :301-304; model.go `isDetailOpen`; keys.go:30-40): every comment `grep -n -i narrow internal/ui/*.go` finds naming the narrow layout gains the stacked case.

**Approach (assumed at the header base):**
- `internal/ui/layout.go`: constants `stackSidebarMinRows = 8`, `stackDetailMinRows = 12`, `stackedShare = 3`. `isNarrow` becomes `isSinglePane` (narrow and too short to stack); add `isStacked` (narrow, tall enough, help closed). Every `isNarrow` caller moves to `isSinglePane`: `keys.go` (`keyOpen`, `keyBack`, `sidebarOrScrollKey`, `scrollNarrowDetail`), `filter.go` (`openFilter`'s guard, `sidebarPaneWidth`), `depview.go` (three sites), `statusbar.go` (`keyHints`), `layout.go` (`resizeSidebar`, `body`, `detailLeft`, `pointAt`). `grep -n isNarrow internal/ui/*.go` is empty afterwards.
- `stackedSidebarRows()`: `round(stackedRatio × bodyHeight)` when set, else `bodyHeight/3`, clamped to `[stackSidebarMinRows, bodyHeight-1-stackDetailMinRows]`. `sidebarPaneRows()` and `detailPaneRows()` return the pane heights (the body's outside stacked). `sidebarHeight` builds on `sidebarPaneRows`; `detailRows`, `scrollDetail`, `scrollForKey`'s page and `paintDeps` use `detailPaneRows`, never `bodyHeight`.
- Stacked panes are full width: `sidebarPaneWidth` and `detailLeft` as in the one-pane layout. The rules above and under the panes have no joints (`rule` already joints only `isSplit`).
- `pointAt`: sidebar rows `[0, stackedSidebarRows)`, the divider row → `paneNone`, detail rows below with `row` counted from the detail pane's top.
- `resizeSidebar` stays a no-op outside the split layout (item 4 adds the stacked move).

**Files:** internal/ui/layout.go, internal/ui/keys.go, internal/ui/filter.go, internal/ui/depview.go, internal/ui/statusbar.go, internal/ui/layout_test.go, internal/ui/mouse_test.go, internal/ui/filter_test.go, internal/ui/statusbar_test.go, internal/ui/testdata/golden/layout-80x40.golden
**Read first:** internal/ui/layout.go — isNarrow, isSplit, pointAt, resizeSidebar, detailLeft; internal/ui/filter.go — sidebarPaneWidth; internal/ui/mouse_test.go — openDetail, linkCell

**Tests:** golden `layout-80x40`; 80×24 still matches `layout-80x24`; at 120×40 the split; a `WindowSizeMsg` from 80×24 to 80×40 and back switches layouts; Enter at 80×40 keeps both panes; PgDn scrolls the detail by its own rows; a click and a wheel notch in the lower pane act on the detail pane, on the rule row on nothing; `>` at 80×40 leaves `sidebar_ratio` unwritten; the narrow one-pane cases in layout_test.go, filter_test.go and statusbar_test.go pass at 24 rows.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
- `go vet ./internal/ui/`

**Commit:** `feat(ui): stack the panes on a narrow, tall terminal`

## 4. Move the horizontal divider

Depends on item 3.

**What:**
**Goal:** in the stacked layout a drag on the rule row, `<` and `>` (2 rows a press) move the divider within the pane minimums, each move that changed it saves `stacked_ratio` (leaving `sidebar_ratio` as stored), and a double-click on the rule row resets it to the default and clears `stacked_ratio`.

**Regression guard.** stateSaveNoticePrefix names both dividers, reading "cannot save the divider position: ", with every test pinning the old "cannot save the sidebar width: " wording updated (model.go joins Files); and because a save writes both ratios, every path that ends a divider change without saving it (a release with no net move, a layout change mid-drag) restores the ratio held at the press, so a later save on the other axis never writes a position the user did not keep — with a test: a drag abandoned by a resize into the split layout, then a `>` there, leaves stacked_ratio as stored.
Every internal/ui doc comment saying the divider, `<`/`>`, a drag or a double-click acts only in the split layout or only on the sidebar's width gains the stacked case; find them with `grep -nE "split layout|narrow|sidebar's width|columns|divider" internal/ui/{keys,mouse,layout,model}.go`.
Supersedes the CHANGELOG.md 0.8.0 entry "Below 90 columns the keys do nothing" and its `cannot save the sidebar width: …` (released, never edited; item 5's Unreleased line records the change) and adr-0014-decision.md:29's width-only writes on `<`/`>` (amended in item 5).

**Approach (assumed at the header base):**
- `layout.go`: `stackedStep = 2`; `resizeStacked(delta)` mirrors `resizeSidebar` on rows, storing `stackedRatio` as rows over `bodyHeight`. `isOnDivider` also hits the rule row in the stacked layout; `rule`-style drawing of that row uses `dividerStyle()`.
- `keys.go`: `resizeSidebarKey` dispatches to `resizeStacked(±stackedStep)` when stacked, saving only when `stackedRatio` changed.
- `mouse.go`: `dividerDrag` records the axis at the press; `startDrag`, `dragDivider` and `mouseRelease` follow `msg.Y` and `stackedRatio` on the horizontal axis (grab offset in rows, a move back onto the start rows restores the start ratio); `pressDivider`'s double-click clears whichever ratio its divider owns. A resize that changes the layout mid-drag ends the drag with nothing saved.

**Files:** internal/ui/layout.go, internal/ui/keys.go, internal/ui/mouse.go, internal/ui/model.go, internal/ui/layout_test.go, internal/ui/mouse_test.go, internal/ui/testdata/golden/drag-divider-80x40.golden
**Read first:** internal/ui/mouse.go — dividerDrag, pressDivider, dragDivider, mouseRelease; internal/ui/layout.go — isOnDivider, resizeSidebar; internal/ui/model.go — update (tea.WindowSizeMsg); internal/ui/mouse_test.go — assertNoStoredRatio (hard-codes "sidebar_ratio")

**Tests:** `<`/`>` at 80×40 move the divider 2 rows and stop at 8 sidebar / 12 detail rows; a drag moves it and the release saves `stacked_ratio` with `sidebar_ratio` unchanged in the file; a drag back to the start saves nothing; double-click clears `stacked_ratio`; golden while dragging; `<` at 120×30 still moves the vertical divider and leaves `stacked_ratio`; the stored ratio survives a restart; a drag abandoned by a resize into the split layout, then a `>` there, leaves `stacked_ratio` as stored; a failed save shows `cannot save the divider position: `.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
- `go vet ./internal/ui/`

**Commit:** `feat(ui): move the stacked layout's divider by drag and keys`

## 5. Document the stacked layout and `ui.stacked_ratio`

Depends on item 4.

**What:**
**Goal:** the manual, the design doc and ADR 0014 describe the stacked layout, its thresholds, moving its divider and the `ui.stacked_ratio` key; `capcom check` reports no problems; the CHANGELOG sidecar carries one Unreleased line.

**Regression guard.** The prose rule covers every passage saying what the divider, `<`/`>` or a double-click do, or what state.yaml holds and when it is written (file-format.md:696, :713; adr-0014-decision.md:5, :29; adr-0014.md:8; terminal-ui.md:245; tui.md:191 included); find them with `grep -rnE "90 columns|narrow|drill-down|sidebar_ratio|sidebar width|divider|state\.yaml" docs/site docs/design README.md .capcom/knowledge`. Acceptance checks each file on its own, chained with `&&`, plus the Unreleased line.

**Approach (assumed at the header base):**
- `docs/site/getting-started/terminal-ui.md`: the "Below 90 columns" paragraph gains the stacked case; "Resizing the sidebar" covers the horizontal divider (drag, `<` `>` 2 rows, 8/12 minimums, double-click, its own ratio). Mouse list item on the divider likewise.
- `docs/site/reference/file-format.md` (The state file): the example and a `ui.stacked_ratio` bullet; a file without it uses the default.
- `docs/design/tui.md`: the narrow-terminal and divider passages.
- `adr-0014.md` and `adr-0014-decision.md`: a dated amendment adding the key, its default and why it is separate from `sidebar_ratio`; edit with `capcom` or by hand then `capcom check`.
- Prose rule: every doc passage saying the narrow layout shows one pane at a time, or naming only `sidebar_ratio` as what `state.yaml` holds, gains the stacked case; `grep -rnE "90 columns|narrow|sidebar_ratio" docs/site docs/design README.md .capcom/knowledge` finds them.

**Files:** docs/site/getting-started/terminal-ui.md, docs/site/reference/file-format.md, docs/design/tui.md, .capcom/knowledge/decisions/adr-0014.md, .capcom/knowledge/decisions/adr-0014/adr-0014-decision.md
**Read first:** docs/site/getting-started/terminal-ui.md — "Below 90 columns" paragraph, Resizing the sidebar, Mouse list; docs/site/reference/file-format.md — The state file; docs/design/tui.md — divider bullet, Narrow terminals bullet;
   .capcom/knowledge/decisions/adr-0014/adr-0014-decision.md — The file; CHANGELOG.md — 0.8.0 resize-keys entry (released, never edited)

**Tests:** none (docs only).

**Acceptance:**
- `capcom check`
- `grep -q stacked_ratio docs/site/reference/file-format.md && grep -q stacked_ratio .capcom/knowledge/decisions/adr-0014.md && grep -q stacked_ratio .capcom/knowledge/decisions/adr-0014/adr-0014-decision.md && grep -q stacked docs/site/getting-started/terminal-ui.md && grep -q stacked docs/design/tui.md`

**Commit:** `docs(ui): describe the stacked layout and ui.stacked_ratio`
