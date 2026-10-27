# capcom ui lets the user choose the stacked layout on a terminal that fits both

**Goal:** on a terminal of at least 90 columns and 25 rows, `v` and a status-bar button switch
`capcom ui` between the split and the stacked layout; the choice is stored as `ui.layout` in
`state.yaml` and applies again whenever both layouts fit.
**Date:** 2026-10-27
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 7b79bd4
**Closes:** ui-layout-toggle

**Sources:**
- `.capcom/issues/human-ergonomics/ui-layout-toggle.md` (binding for every behaviour below)
- Notes `adr-0014`, `adr-0014-decision`
- `docs/plans/archived/2026-10-17 - 01 - ui-stacked-layout-plan.md` (the stacked layout's own plan)

**Ratified design calls** (user, 2026-10-27, unless marked writer):
- **Click with filter line or status picker open:** a left click on the button toggles the layout; both stay open. `v` stays theirs (typed into the filter, taken by the picker).
- **Button colour:** a new colour role `layout-button`, overridable under `ui.colors`; the fill is that role (presets: dark `#4a4a4a`, light `#d0d7de`), the glyph the `muted` colour; under `NO_COLOR` the button is drawn in reverse.
- **Stored value:** `ui.layout: split | stacked`; any other value, or a non-string, loads as no preference silently (no notice, the ratios still load) and a save drops it.
- **Save notice (writer):** `stateSaveNoticePrefix` becomes `cannot save the layout: `, since a save now carries the layout choice and both divider positions.
- **Button position (writer, from the issue text):** blanks pad the status bar between the hints and the button, so the button fills the row's last 3 columns; without the button the row is unpadded as today.

**Standing requirements:**
- skills: coding-standards
- Tests per `AGENTS.md`: single packages with `GOMEMLIMIT=2GiB`, never `./...` with `-race`, never two test commands at once.
- No test writes under the real `os.UserConfigDir`: test options set `UserStatePath` to a temp dir.
- Glossary terms: "split layout", "stacked layout", "one-pane layout"; "layout choice" for the stored preference.
- Any authorized deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- Changing either threshold (`narrowWidth`, `stackBodyMinRows`), the one-pane layout, or either divider's ratio handling.
- A per-worktree layout choice (`ui-state.yaml`).
- A flash notice on toggle; any key other than `v`.

**Regression check (2026-10-27, 7b79bd4):**
- 2: guard folded
- 3: guard folded
- 4: guard folded (writer decision: the save-notice pin; reviewer guard: helpLines stays at 20 rows)
- 5: recast (writer decision: the button fills the row's last 3 columns)
- 6: guard folded (writer decision: the hit test rests on item 5's padding; reviewer guard: no toggle under help or a prompt)
- 7: guard folded
- 8: guard folded (writer decision: tui.md notice quote and help copy; reviewer guards: Mouse sections, state-file prose)

## 1. `ui.layout` in the state file — ✅ DONE (2026-10-27)

NOTES (2026-10-27): the ui.layout reader is a plain function `layout(node, target)` rather than a `parser` method, since it never errors and needs no parser state; a stored value must be a string scalar (quoted or plain) equal to split or stacked, so a wrong-case `Stacked` also loads as no choice.

**What:**
**Goal:** `userconfig.UIState` carries `Layout string`, one of `userconfig.LayoutSplit` (`"split"`), `userconfig.LayoutStacked` (`"stacked"`) or `""` for none; `LoadState` reads `ui.layout`, loading any other value or node kind as `""` with no error; `SaveState` writes `ui.layout` when non-empty, after both ratios, and refuses any value but those three.

**Approach (assumed at the header base):**
- Extends `internal/userconfig/state.go`; no new file. Add `keyLayout = "layout"` and the two exported constants; `stateUI` dispatches the key to a new `parser.layout` that stores only a scalar string equal to a constant.
- `encodeState` appends `  layout: <value>` after the ratios, inside the one `ui:` mapping; an all-empty state still writes only the header.
- `SaveState` errors `save the state file: layout "<v>" is not split or stacked` for an unknown non-empty value and leaves the file untouched.
- Update the `State`/`UIState` doc comments and `LoadState`/`SaveState` docs to name the layout choice.

**Files:** internal/userconfig/state.go, internal/userconfig/state_test.go
**Read first:** internal/userconfig/state.go — UIState, stateUI, encodeState, SaveState, LoadState; internal/userconfig/userconfig.go — eachKey; internal/userconfig/state_test.go — TestSaveState_LoadState_RoundTrips, writeState

**Tests:** round trip of `Layout` alone, with both ratios, and empty; a file with `layout: auto`, `layout: 3`, `layout: [a]` loads `Layout ""`, no error, ratios intact; `SaveState` with `Layout "auto"` errors and leaves the file untouched; encoded order is `sidebar_ratio`, `stacked_ratio`, `layout`.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/userconfig/`
- `go vet ./internal/userconfig/`

**Commit:** `feat(userconfig): store the layout choice as ui.layout`

## 2. The `layout-button` colour role and theme style — ✅ DONE (2026-10-27)

NOTES (2026-10-27): the theme builds `layoutButton` through a new helper `layoutButtonStyle` in internal/ui/theme.go (beside `badgeStyle`, which it does not use since its text is muted, not black/white by luminance); tests add `TestTheme_Presets_LayoutButton_FillsWithItsRoleAndMutedGlyph`, `TestTheme_NoColor_LayoutButtonIsReverse` and the test helper `isSameColour`.
NOTES (2026-10-27): the role lists in docs/site/reference/file-format.md (template quote and roles prose), docs/site/getting-started/terminal-ui.md and docs/design/tui.md do not yet name `layout-button`; item 8 owns them (its text greps `lock-free` in docs/design and docs/site).

**What:**
**Goal:** `layout-button` is a colour role of `userconfig.Roles` (after `lock-free`), has a colour in every scheme file (dark `#4a4a4a`, light `#d0d7de`), and the theme has a `layoutButton` style: background the `layout-button` colour, foreground the `muted` colour; with no colour for the role, `lipgloss.NewStyle().Reverse(true)`.

**Regression guard.** No listed test checks the Goal's fill and glyph colours: `TestTheme_Presets_DrawInApogeesColours` reads `newPalette`, not `theme.layoutButton`. Tests add: `newTheme(userconfig.UI{Theme: dark}, false, false).layoutButton` has `GetBackground()` `#4a4a4a` and `GetForeground()` the muted `#8a8a8a`; light `#d0d7de` and `#656d76`.

**Approach (assumed at the header base):**
- `internal/userconfig/userconfig.go` `Roles` gains the role; `internal/userconfig/template.go` names it in the colours comment ("the status bar's layout button: layout-button").
- `internal/ui/schemes/dark.yaml` and `light.yaml` gain the key with an apogee-role comment like their neighbours ("apogee chrome: the status bar's layout button"); the file headers listing what the UI fills name the button.
- `internal/ui/theme.go`: `roleLayoutButton` constant; `theme.layoutButton` field built in `newTheme` beside `lockHeld`/`lockFree`. Not `badgeStyle`: its text colour is muted, not black/white by luminance.

**Files:** internal/userconfig/userconfig.go, internal/userconfig/template.go, internal/userconfig/userconfig_test.go, internal/ui/schemes/dark.yaml, internal/ui/schemes/light.yaml, internal/ui/theme.go, internal/ui/theme_test.go
**Read first:** internal/ui/theme.go — newTheme, badgeStyle; internal/userconfig/userconfig.go — Roles; internal/userconfig/template.go — template; internal/ui/schemes/dark.yaml, light.yaml — lock-free; internal/ui/theme_test.go — TestTheme_Presets_DrawInApogeesColours, TestTheme_NoColor_DrawsNoColourSequences; internal/userconfig/userconfig_test.go — TestLoad_CreatedTemplate_NamesEveryRoleAndUncommentsCleanly

**Tests:** the preset-colour table in `theme_test.go` gains `layout-button` for dark and light; an override under `ui.colors` of `layout-button` is accepted by `userconfig.Load`; under `NO_COLOR` `layoutButton` renders reverse; `newTheme(...).layoutButton` has `GetBackground()` `#4a4a4a` / `GetForeground()` `#8a8a8a` in dark and `#d0d7de` / `#656d76` in light.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/userconfig/`
- `GOMEMLIMIT=2GiB go test -count=1 -run 'Theme|Scheme|Preset' ./internal/ui/`
- `go vet ./internal/userconfig/ ./internal/ui/`

**Commit:** `feat(ui): add the layout-button colour role`

## 3. The layout choice picks the layout when both fit — ✅ DONE (2026-10-27)

NOTES (2026-10-27): all new tests live in internal/ui/layout_test.go beside the existing state-file and layout tests (TestModel_StoredLayoutChoice_PicksTheLayoutOnlyWhenBothFit, TestModel_TerminalResize_KeepsTheStoredLayoutChoiceUnsaved, TestModel_DividerSave_WritesTheLayoutChoiceHeld); internal/ui/model_test.go needed no change. The prose guard also rewrote the comments on body, pointAt, isSinglePane, isSplit and the layout constants in layout.go, and the stateSaver/save/write comments in model.go, to name the layout choice.

Depends on item 1.

**What:**
**Goal:** the model starts on the stored `Layout`; `canChooseLayout()` reports a terminal of at least `narrowWidth` columns whose body has at least `stackBodyMinRows` rows; on such a terminal the stacked layout applies when the choice is `stacked`, the split layout otherwise; below either threshold the layout is chosen by size exactly as at the base; every state-file save writes the choice held, unchanged by any resize.

**Regression guard.** The prose guard's grep also turns up comments outside the listed files that go false once a stored `stacked` stacks the panes on a wide terminal: `internal/ui/keys.go` (`handleKey`: "on a narrow screen tall enough to stack the panes, Enter keeps both") and `internal/ui/detail.go` (`detailWidth`: "the whole width on a narrow terminal"). Both join **Files:**, and every other file the grep turns up joins it too.

**Approach (assumed at the header base):**
- `internal/ui/layout.go`: add `canChooseLayout()`; `hasFullWidthPanes()` becomes `m.width < narrowWidth || (m.canChooseLayout() && m.layoutChoice == userconfig.LayoutStacked)`; `canStack`, `isSinglePane`, `isStacked`, `isSplit`, `detailLeft`, `sidebarPaneWidth` keep deriving from it, so no other predicate is rewritten.
- `internal/ui/model.go`: field `layoutChoice string` (the stored choice, `""` for none), set from `userState.Layout` in `newModel`; `saveState` passes it as `Layout`. Nothing on `tea.WindowSizeMsg` changes it.
- Prose guard: every comment in `internal/ui` that says the layout follows the terminal's size alone, or that the panes stack only on a narrow terminal, names the choice; find them with `grep -n 'narrow\|wide terminal\|90 columns\|side by side' internal/ui/*.go`.

**Files:** internal/ui/layout.go, internal/ui/model.go, internal/ui/filter.go, internal/ui/keys.go, internal/ui/detail.go, internal/ui/layout_test.go, internal/ui/model_test.go
**Read first:** internal/ui/layout.go — hasFullWidthPanes, canStack, isStacked, detailLeft; internal/ui/model.go — newModel, saveState; internal/ui/filter.go — sidebarPaneWidth; internal/ui/layout_test.go — TestModel_TerminalSize_PicksTheLayout

**Tests:** 120×30 with stored `stacked` renders the stacked body (horizontal divider, full-width panes); 120×24 (body too short) with stored `stacked` renders split; 80×40 with stored `split` renders stacked; 80×24 one pane either way; a resize 120×30 → 80×24 → 120×30 with `stacked` returns to stacked and issues no save; a divider save with `layoutChoice` `stacked` writes `layout: stacked` to the temp `UserStatePath`, with `""` writes no `layout` key.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'Layout|Stack|Split|State|Ratio|Resize|Divider' ./internal/ui/`
- `go vet ./internal/ui/`

**Commit:** `feat(ui): apply the stored layout choice when both layouts fit`

## 4. `v` toggles the layout — ✅ DONE (2026-10-27)

NOTES (2026-10-27): the help screen gets no new row: `layout   v` is merged into the copy row (`  layout   v        copy     drag over the detail text`), so helpLines stays at 20 rows; the plan's suggested row text "split or stacked, when both fit" is not shown.
NOTES (2026-10-27): internal/ui/keys_test.go did not exist; it is created for the v tests (toggle and save, no-op where only one layout fits, help/confirm prompt/text prompt/filter line/status picker open, failed save notice).
NOTES (2026-10-27): docs/design/tui.md:75 still quotes `cannot save the divider position: `; left for item 8, whose text owns the tui.md notice quote and the help copy.

Depends on item 3.

**What:**
**Goal:** while `canChooseLayout()` and no help screen, prompt, text prompt, status picker or filter line is open, `v` sets the layout choice to the layout not on screen, repaints, and returns the command saving the state file; elsewhere `v` changes nothing and saves nothing; the help screen lists `v`; a failed save reads `cannot save the layout: <err>`.

**Regression guard.** The pinned save-notice text in internal/ui/layout_test.go (near line 1146, `cannot save the divider position: `) is updated to `cannot save the layout: `; name that test in Tests and layout_test.go in Files. `helpLines` already holds 20 rows, the whole `bodyHeight` at 80×24, so a 21st fails `TestModel_View_HelpScreen_FitsTheSmallTerminal`: `layout v` is merged into an existing row, or a named blank row is dropped, so `helpLines` stays at 20 rows.

**Approach (assumed at the header base):**
- `internal/ui/layout.go`: `toggleLayout() (model, tea.Cmd)` — returns `m, nil` unless `canChooseLayout()`; sets `layoutChoice` to `LayoutSplit` when `isStacked()`, else `LayoutStacked`; then `abandonDragOffLayout().relayout()` and `saveState()`. Item 6's click calls this same method.
- `internal/ui/keys.go`: `keyLayout = "v"`; a `case keyLayout:` in `handleKey`'s switch. Help, prompts, picker and filter already take keys before that switch (`update`, `handleKey`), so no extra guard.
- `internal/ui/help.go`: one row `  layout   v        split or stacked, when both fit`, kept within `bodyHeight` at 80×24; regenerate `help-120x30.golden` with `-update`.
- `internal/ui/model.go`: `stateSaveNoticePrefix = "cannot save the layout: "` and its comment; update tests pinning the old text.

**Files:** internal/ui/layout.go, internal/ui/keys.go, internal/ui/help.go, internal/ui/model.go, internal/ui/keys_test.go, internal/ui/layout_test.go, internal/ui/testdata/golden/help-120x30.golden
**Read first:** internal/ui/keys.go — handleKey; internal/ui/model.go — update, stateSaveNoticePrefix; internal/ui/help.go — helpLines; internal/ui/mouse.go — abandonDragOffLayout; internal/ui/layout_test.go — TestModel_View_HelpScreen_FitsTheSmallTerminal, TestModel_View_HelpScreen_MatchesGolden, TestModel_UnwritableStateFile_InTheStackedLayout_ShowsTheDividerSaveNotice

**Tests:** `v` at 120×30 toggles split → stacked → split, each returning a save whose file holds the new `layout`; `v` at 80×40, 120×24 and 80×24 changes nothing and returns no command; `v` with help open, a prompt open, the filter line open (it types `v`) and the picker open leaves the layout; a failed save shows `cannot save the layout: `, and `TestModel_UnwritableStateFile_InTheStackedLayout_ShowsTheDividerSaveNotice` expects that prefix; help golden matches; `TestModel_View_HelpScreen_FitsTheSmallTerminal` passes with `helpLines` at 20 rows.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'Key|Layout|Help|State|Toggle' ./internal/ui/`
- `go vet ./internal/ui/`

**Commit:** `feat(ui): toggle between the split and stacked layout with v`

## 5. The status-bar layout button — ✅ DONE (2026-10-27)

NOTES (2026-10-27): the stacked-or-split label is picked by `isStacked()` (the label is ` │ ` when it reports true, ` ─ ` otherwise); `layoutButton()` returns "" while a confirm or text prompt is open as well, though statusBar's prompt paths already return before it.
NOTES (2026-10-27): every regenerated golden differs from the working tree only in its last row (line 30); the new tests sit in statusbar_test.go beside a `withLayoutButton` helper that the two layout_test.go pins use.

Depends on items 2 and 4.

**What:** Recast at the regression check (2026-10-27).
**Goal:** while `canChooseLayout()`, the help screen is closed and no prompt or text prompt is open, the status bar ends in a 3-column button painted in `theme.layoutButton` after the muted key hints: ` ─ ` while the split layout is on screen, ` │ ` while the stacked one is; its columns are reserved like the hints', so the status text is cut before them; otherwise the row is byte-identical to the base.

**Regression guard.** The status row is unpadded at the base (`fit(text, room) + painted`); when the button is drawn, statusBar pads blanks (unstyled) between the painted hints and the button so the button occupies exactly the row's last 3 columns (columns width-3 .. width-1), per the issue's "at the right edge of the status bar, last, after the key hints"; when the button is not drawn the row stays byte-identical to the base (no padding). The exact status-row pins in internal/ui/layout_test.go at 120×30 (e.g. near lines 722 and 824) are updated to the new row and named in Tests/Files.

**Approach (assumed at the header base):**
- `internal/ui/statusbar.go`: constants `layoutButtonToStacked = " ─ "`, `layoutButtonToSplit = " │ "`; `layoutButton() string` returns the unstyled label or `""` when not drawn; `statusBar` reserves `ansi.StringWidth(hints)+ansi.StringWidth(button)` and appends `m.theme.layoutButton.Render(button)` after the painted hints, in the `room > 0` and `room <= 0` paths alike. The prompt paths return before it, so they never draw it.
- It is drawn with the filter line or the status picker open (their hints differ; the button does not).
- Regenerate every golden the button reaches with `-update`; each regenerated file may differ from the base only in its last row.

**Files:** internal/ui/statusbar.go, internal/ui/statusbar_test.go, internal/ui/layout_test.go, internal/ui/testdata/golden/
**Read first:** internal/ui/statusbar.go — statusBar, keyHints; internal/ui/statusbar_test.go — paintedStatusRow, TestStatusBar_LockBadge_NarrowRowCutsBeforeTheHints; internal/ui/layout_test.go — TestModel_FirstStart_CreatesTheUserConfig, TestModel_StatusBar_ShowsTheTreeLock; internal/ui/harness_test.go — statusRow, compareGolden

**Tests:** at 120×30 the stripped last row is the hints, unstyled blanks, then ` ─ ` (split) or ` │ ` (stacked) in its last 3 columns (width-3 .. width-1), 120 columns in all; `TestModel_StatusBar_ShowsTheTreeLock` ("free") and `TestModel_FirstStart_CreatesTheUserConfig` expect that padded row; at 89×30 and 120×24 no button, row as at the base; hidden with help or either prompt open, shown with the filter line and the picker; a long notice is cut before the hints and the button; in dark, light and `NO_COLOR` the button's ANSI carries the role's background (reverse under `NO_COLOR`).

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/`
- `go vet ./internal/ui/`
- `git diff -U0 7b79bd4 -- internal/ui/testdata/golden/` shows only status-bar rows changed, apart from `help-120x30.golden` (item 4) and new files.

**Commit:** `feat(ui): show a layout button at the end of the status bar`

## 6. A click on the layout button toggles the layout — ✅ DONE (2026-10-27)

Depends on item 5.

**What:**
**Goal:** a left click on any of the button's three columns, on the status bar's row, toggles the layout exactly as `v` does, also while the filter line or the status picker is open (both stay open); a click there while the button is not drawn, and any other button, does what it did at the base.

**Regression guard.** The hit test `x >= m.width-3 && x < m.width` on row `m.height-1` matches item 5's position (button in the row's last 3 columns); it relies on item 5's padding. The click is taken before `isMouseIgnored`, and only `statusBar`'s early prompt returns keep the button off a prompt row, so `isOnLayoutButton` returns false while `m.isHelpOpen || m.prompt.isOpen || m.textPrompt.isOpen`. A click working with the filter line or status picker open supersedes `docs/site/getting-started/terminal-ui.md` (Mouse, near line 281) and `docs/design/tui.md` (Mouse, near line 220), per the ratified click call; item 8 amends both.

**Approach (assumed at the header base):**
- `internal/ui/mouse.go`: `isOnLayoutButton(x, y int) bool` — `m.layoutButton() != ""`, `y == m.height-1`, `x >= m.width-ansi.StringWidth(button)` and `x < m.width`. In `mouseClick`, before the `isMouseIgnored` check: a `tea.MouseLeft` press on it clears `m.sel`, resets `m.lastDividerPress` and returns `m.toggleLayout()`.
- Update `mouseClick`'s and `isMouseIgnored`'s doc comments to name the exception.

**Files:** internal/ui/mouse.go, internal/ui/mouse_test.go
**Read first:** internal/ui/mouse.go — mouseClick, isMouseIgnored, pressDivider; internal/ui/statusbar.go — statusBar; internal/ui/layout.go — pointAt; internal/ui/mouse_test.go — click, TestMouse_ClickWithHelpOpen_DoesNothing; internal/ui/harness_test.go — statusRow

**Tests:** click at the button's first and last column at 120×30 toggles and saves; with the filter line open and with the picker open it toggles and they stay open; a right click, a click one column left of it, a click at 89×30, with help open, with a confirmation prompt open and with the reason prompt (`c`) open change nothing.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'Mouse|Click|Layout|Toggle' ./internal/ui/`
- `go vet ./internal/ui/`

**Commit:** `feat(ui): toggle the layout with a click on the status-bar button`

## 7. ADR 0014 records the layout choice

Depends on item 1.

**What:**
**Goal:** Notes `adr-0014` and `adr-0014-decision` record, as a dated 2026-10-27 amendment, `ui.layout` (`split` | `stacked`), when it applies (both layouts fit), that it is written only on a toggle (and carried by every save), that a forced fallback never rewrites it, and that an unknown value loads as none silently.

**Regression guard.** The Approach's list of sections is not closed: an unusable file now also drops the layout choice (`LoadState` returns the zero `State`), so the decision Note's motivation ("set by dragging the divider or pressing a key"), "Deleting the file ... only forgets the divider positions" and "An unusable file is a notice ... starts on the default width and height" go stale too. A rule replaces the list: every sentence of both Notes that enumerates what `state.yaml` holds, when it is written, or what `capcom ui` falls back to without it, found by `grep -n 'ratio\|width\|height\|divider\|default'` on both files.

**Approach (assumed at the header base):**
- Edit `.capcom/knowledge/decisions/adr-0014.md` (status line, summary) and `.capcom/knowledge/decisions/adr-0014/adr-0014-decision.md` (the file's keys, when it is written, "Missing means the defaults", unknown values) in place; then `capcom check`.

**Files:** .capcom/knowledge/decisions/adr-0014.md, .capcom/knowledge/decisions/adr-0014/adr-0014-decision.md
**Read first:** .capcom/knowledge/decisions/adr-0014/adr-0014-decision.md — The file, Only capcom ui reads and writes it, Missing means the defaults, Unknown keys are ignored, An unusable file is a notice; .capcom/knowledge/decisions/adr-0014.md — Status line, summary; internal/userconfig/state.go — LoadState

**Tests:** none (docs).

**Acceptance:**
- `capcom check`
- `capcom know show adr-0014-decision | grep -n 'ui.layout'`

**Commit:** `docs(adr): record the layout choice in ADR 0014`

## 8. User docs for the layout toggle

Depends on items 4, 5 and 6.

**What:**
**Goal:** the manual and the TUI design note describe the toggle: `v`, the button and its glyphs, when it is offered, `ui.layout` in `state.yaml`, and the `layout-button` colour role; each copy of the help screen matches `helpLines`; the copy of the config template matches `internal/userconfig/template.go`.

**Regression guard.** docs/design/tui.md's quote of `cannot save the divider position: ` (near line 75) becomes `cannot save the layout: `; tui.md's help-screen copy is brought in line with helpLines as a whole (it already differs at the base), not only the new `v` row. Both Mouse sections (`terminal-ui.md` near line 281, `tui.md` near line 220) say the layout button's click works while the filter line or the status picker is open. Every sentence saying what `state.yaml` holds, when `capcom ui` writes it, how a bad value loads, or quoting the save notice names the layout choice; find them with `grep -rn 'divider position\|both ratios\|neither key\|writes the file\|written once per\|cannot save\|Deleting the file' docs/site docs/design`.

**Approach (assumed at the header base):**
- `docs/site/getting-started/terminal-ui.md`: layouts section, status bar section, help-screen copy, colour roles list.
- `docs/site/reference/file-format.md`: `state.yaml` keys (`ui.layout`), the config-template copy and the colours reference.
- `docs/design/tui.md`: layout, status bar, help-screen copy and roles.
- Find every help-screen copy with `grep -rn 'resize   < >' docs/design docs/site`; every roles list with `grep -rn 'lock-free' docs/design docs/site`.

**Files:** docs/site/getting-started/terminal-ui.md, docs/site/reference/file-format.md, docs/design/tui.md
**Read first:** docs/site/reference/file-format.md — The state file, The user config; docs/site/getting-started/terminal-ui.md — Status bar bullet, Mouse; docs/design/tui.md — Divider, Mouse; internal/ui/help.go — helpLines; internal/userconfig/template.go

**Tests:** none (docs).

**Acceptance:**
- `grep -rn 'layout   v' docs/design/tui.md docs/site/getting-started/terminal-ui.md`
- `grep -rn 'layout-button' docs/design/tui.md docs/site/getting-started/terminal-ui.md docs/site/reference/file-format.md`
- `grep -n 'ui.layout\|layout:' docs/site/reference/file-format.md`
- `! grep -rn 'the filter line, the status picker or a prompt is open' docs/site docs/design`
- `! grep -rn 'cannot save the divider position' docs/site docs/design`

**Commit:** `docs(ui): describe the layout toggle`
