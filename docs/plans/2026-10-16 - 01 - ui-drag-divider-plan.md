# Resize the sidebar by dragging the divider

**Goal:** In `capcom ui`'s split layout the user sets the sidebar's width by dragging the divider
with the mouse, or with `<` and `>`, within limits; a double-click on the divider resets it, and
the width persists across runs in a machine-written state file beside the user config.
**Date:** 2026-10-16
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** f13aeee
**Closes:** ui-drag-divider

**Sources:**
- `.capcom/issues/capcom-ui/ui-drag-divider.md`
- Notes `adr-0011`, `adr-0011-decision` (per-user config; `config.yaml` is never rewritten)
- `internal/ui/layout.go`, `internal/ui/mouse.go`, `internal/userconfig/userconfig.go`

**Ratified design calls** (user, 2026-10-16):
- **Limits:** sidebar ≥ 20 columns; detail pane (from `detailLeft` to the right edge, scrollbar included) ≥ 40 columns.
- **Persistence:** across restarts, in a separate machine-written `<os.UserConfigDir>/capcom/state.yaml`; `config.yaml` stays hand-owned and is never written.
- **Unit:** columns; clamped to the limits at every terminal size, the stored value never rewritten by a clamp.
- **Hit area:** the `│` column and the margin column either side, on every body row.
- **Feedback:** panes reflow on every motion; the divider and its `┬`/`┴` joints draw in an accent, bold style while the button is held.
- **Keys:** `<` shrinks, `>` grows the sidebar by 4 columns; persisted like a drag.
- **Reset:** a double-click on the divider returns to the default width (a third, 28–48) and clears the stored width.
- **Write time:** once per finished drag (release, when the width changed), per resize key and per reset; a failed write is a status-bar notice and the width still applies.
- **Help:** the help screen names the keys, the drag and the double-click.

**Standing requirements:**
- skills: coding-standards
- Tests per `AGENTS.md`: single packages with `GOMEMLIMIT=2GiB`, never `./...` with `-race`, never two test commands at once.
- No test may write under the real `os.UserConfigDir`: every test model sets both user-file paths to a temp dir.
- Any authorized deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- Resizing in the narrow layout (below `narrowWidth`), or of anything but the sidebar/detail split.
- A `ui.sidebar_width` key in `config.yaml`; a share-of-width unit.
- Keeping unknown keys of `state.yaml` across a save.

**Regression check (2026-10-16, f13aeee):**
- 3: guard folded (one serialized saver; parent-is-a-file failure test only)
- 4: guard folded (single-run rule/body output unless dragging; no existing golden regenerated)
- 5: guard folded (fake clock for item 4's chained drags; a finished drag clears `lastDividerPress`)
- 6: recast (one help row, help-fits-80x24 test, widened docs prose rules)
- 6 (second round): guard folded into the Approach (one key row with no note line, fit test, widened docs/site finder, tui.md Keys block and Mouse paragraph); guard trimmed to the one-row bound

## 1. State file for capcom ui in userconfig — ✅ DONE (2026-10-16)

NOTES (2026-10-16): internal/userconfig/userconfig.go: Path() now delegates to a new unexported capcomFile(name) helper shared with StatePath() (the plan's "sharing Path()'s directory logic"); the "locate the user config directory: " wording is unchanged.
NOTES (2026-10-16): consequential edit — internal/userconfig/userconfig.go: made necessary by adding state.go (the package doc now names the state file and that only capcom ui may call LoadState/SaveState).
NOTES (2026-10-16): beyond the listed tests, SaveState rejects a negative SidebarWidth with an error, a quoted or fractional sidebar_width is an *Error like a non-integer, and tests also cover replacing an existing file (no temp file left behind) and a parent-is-a-file save failure.

**What:**
**Goal:** `internal/userconfig` exposes `State{UI UIState}`, `UIState{SidebarWidth int}` (0 = none stored), `StatePath()`, `LoadState(path)` and `SaveState(path, State)`; a saved state loads back equal, and `config.yaml`'s API and behaviour are unchanged.

**Approach (assumed at the header base):**
- New `internal/userconfig/state.go` in the existing package (it owns capcom ui's per-user files; no new package). `StatePath()` returns `<os.UserConfigDir>/capcom/state.yaml`, sharing `Path()`'s directory logic.
- File shape: a first comment line `# capcom ui state: written by capcom ui, not meant for editing (ADR 0014).`, then `ui:` / `  sidebar_width: <n>`; `SidebarWidth` 0 omits the key (and an empty `ui:` mapping is omitted too).
- `LoadState`: a missing file is the zero `State` and no error, and is never created. Malformed YAML, a non-integer or a value < 1 is an `*Error` (path and line, as `parse` reports) with the zero `State`. Unknown keys are ignored silently.
- `SaveState`: creates the directory (`directoryMode`), writes a temp file in the same directory and renames it over `state.yaml` (`fileMode`), so a reader never sees a half-written file.

**Files:** internal/userconfig/state.go, internal/userconfig/state_test.go
**Read first:** internal/userconfig/userconfig.go — Path, Load, parser.document, parser.yamlError, Error, writeNew, directoryMode, fileMode; internal/userconfig/userconfig_test.go — TestPath_UserConfigDir_EndsInCapcomConfig, configPath; internal/ui/layout_test.go — TestModel_NoUserConfigDirectory_WarnsAndRunsOnDefaults (pins Path's "locate the user config directory: " wording, so a shared helper must keep it)

**Tests:** round trip of widths 0 and 36; missing file → zero, no file created; malformed YAML, `sidebar_width: wide`, `sidebar_width: 0` → `*Error` naming the path and line; unknown key ignored; save into a missing directory creates it; `StatePath` ends in `capcom/state.yaml`.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/userconfig/`
- `go vet ./internal/userconfig/`

**Commit:** `feat(userconfig): add the capcom ui state file`

## 2. Adjustable sidebar width with the < and > keys — ✅ DONE (2026-10-16)

**What:**
**Goal:** In the split layout the sidebar's width is a stored column count when one is set, the default (a third, 28–48) otherwise, always clamped to [20, width − 3 − 40]; `<` and `>` change it by 4 within those limits and every pane, the rules' joints, the filter line and mouse hit-testing follow the new width.

**Approach (assumed at the header base):**
- Extends `layout.go`; no new module. New constants beside `sidebarMinWidth`: `sidebarDragMinWidth = 20`, `detailMinWidth = 40`, `sidebarStep = 4`. Model field `sidebarColumns int` (0 = default).
- `sidebarWidth()` becomes: desired = `sidebarColumns` or today's third-within-28–48 formula, clamped to `[sidebarDragMinWidth, m.width − 2*dividerMargin − ansi.StringWidth(paneSeparator) − detailMinWidth]`. Every reader (`dividerColumn`, `detailLeft`, `pointAt`, `sidebarPaneWidth`, `rule`, `detailWidth`) keeps calling it.
- Extract the repaint in `Update`'s `tea.WindowSizeMsg` case (filter line width, `paintSidebar`, `renderDetail`, `scrollDetail(0)`, `paintDeps`) into one `relayout()` used by the size message and by every width change; the size message still sets the text prompt's width itself.
- `keys.go`: `keyShrinkSidebar = "<"`, `keyGrowSidebar = ">"` in `handleKey`; on a narrow terminal they do nothing. New width = clamp(current `sidebarWidth()` ± `sidebarStep`), stored in `sidebarColumns`. No persistence in this item.

**Files:** internal/ui/layout.go, internal/ui/model.go, internal/ui/keys.go, internal/ui/layout_test.go
**Read first:** internal/ui/layout.go — sidebarWidth, dividerColumn, detailLeft, pointAt, rule, body; internal/ui/model.go — Update (tea.WindowSizeMsg case), model; internal/ui/keys.go — handleKey; internal/ui/filter.go — sidebarPaneWidth; internal/ui/detail.go — detailWidth, renderDetail; internal/ui/depview.go — depViewKey, paintDeps; internal/ui/mouse_test.go — smallWidth, detailLeft, click

**Tests:** default width unchanged at 90, 120 and 200 columns (existing goldens still pass); `>` and `<` move `dividerColumn()` by 4; repeated `<` stops at 20 and `>` at width − 43; a stored 80 at width 100 clamps to 57 and `sidebarColumns` stays 80; keys do nothing at `smallWidth`; a click on a sidebar row after `>` still selects that row.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
- `go vet ./internal/ui/`

**Commit:** `feat(ui): resize the sidebar with < and >`

## 3. Persist the sidebar width in the state file — ✅ DONE (2026-10-16)

NOTES (2026-10-16): the saver is a stateSaver held by pointer in model.go: each save command writes the latest width asked for under its mutex and one finding that save already written writes nothing; stateSavedMsg carries the save's number and the model drops a report older than the last one applied.
NOTES (2026-10-16): a `<`/`>` press that leaves sidebarColumns unchanged (narrow terminal, or already at a limit) builds no save command, so the file is written only when the width changed.
NOTES (2026-10-16): a state path that cannot be resolved (no user config directory) shows a `ui state: ` load notice, and every save then reports that error instead of writing anywhere; internal/cli/ui.go needed no change (empty UserStatePath resolves the default).
NOTES (2026-10-16): internal/ui/model.go grows to 453 lines, past the ~400 guideline; the saver stayed there because the item's Files list names no new file — a later split of the user-file loading (loadUserConfig, loadUserState, stateSaver) into its own file would bring it back under.

Depends on items 1 and 2.

**What:**
**Goal:** `capcom ui` starts on the width stored in `state.yaml`, and every width change made by a key writes it there; an unusable or unwritable state file is a status-bar notice and the UI runs on.

**Regression guard.** bubbletea v2.0.8 runs each `tea.Cmd` in its own goroutine (`tea.go` event loop), so per-press save commands can land out of order. Route every save through one saver the model holds by pointer (a mutex plus the latest width, written under the lock) or a sequence number that drops stale writes and stale `stateSavedMsg`s; test it by running two built save commands in reverse order and reading back the later width. The save-failure test uses only a state path whose parent is a regular file (fails `MkdirAll` on every OS, root included), never a read-only directory.

**Approach (assumed at the header base):**
- `Options` gains `UserStatePath string`; empty → `userconfig.StatePath()`, resolved in `newModel` as `loadUserConfig` resolves `UserConfigPath`. `newModel` loads it into `sidebarColumns`.
- A new `stateNotice` model field joins the status bar's notice list in `statusbar.go`. Load error → `stateNotice` naming the file; the width stays default.
- Writes run off `Update` as a `tea.Cmd` (`saveState`) returning `stateSavedMsg{err}`; an error sets `stateNotice` ("cannot save the sidebar width: …"), success clears it. One model method builds that command from `sidebarColumns`; the `<`/`>` handler returns it.
- `newTestOptions` in `harness_test.go` sets `UserStatePath` to a temp-dir path.

**Files:** internal/ui/ui.go, internal/ui/model.go, internal/ui/keys.go, internal/ui/statusbar.go, internal/ui/harness_test.go, internal/ui/layout_test.go
**Read first:** internal/ui/model.go — newModel, loadUserConfig, Update; internal/ui/ui.go — Options, Run; internal/ui/statusbar.go — statusBar; internal/ui/harness_test.go — newTestOptions, startModel, press, drive, statusRow; internal/ui/layout_test.go — TestModel_NoUserConfigDirectory_WarnsAndRunsOnDefaults, writeTestFile; internal/cli/ui.go — newUICommand

**Tests:** a model over a state file holding 36 starts with `sidebarWidth()` 36 at 120 columns; `>` then a drive of its command leaves `sidebar_width: 40` in the file; two built save commands run in reverse order leave the later width in the file; a malformed state file shows its path in `statusRow`; a state path whose parent is a regular file shows the save notice and the width still changes.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
- `go vet ./internal/ui/ ./internal/cli/`

**Commit:** `feat(ui): remember the sidebar width across runs`

## 4. Drag the divider with the mouse

Depends on item 3.

**What:**
**Goal:** In the split layout a left press on the divider's hit area starts a drag; each motion moves the divider with the pointer within the limits and reflows the panes; the release ends it and saves the width when it changed. While dragging, the divider and its rule joints draw in the theme's `dividerActive` style (accent foreground, bold).

**Regression guard.** `rule()` paints the whole rule as one `border.Render` run (`layout.go`), and the theme goldens pin those SGR bytes (`TestTheme_Presets_MatchANSIGoldens`). `rule()` and `body()` keep today's single-run output unless `drag.isActive`; the item adds one golden (the mid-drag frame) and regenerates no existing golden — never run `-update` over the package's goldens.

**Approach (assumed at the header base):**
- Extends `mouse.go` and `layout.go`. `isOnDivider(x, y)`: `isSplit()`, a body row, and `sidebarWidth() ≤ x < detailLeft()`. In `mouseClick`, a left press there (and not `isMouseIgnored`) starts the drag ahead of the pane switch: model field `drag` holding `isActive`, `grabOffset = x − dividerColumn()` (so the divider never jumps) and the width at press.
- `Update` gains `tea.MouseMotionMsg` → while dragging, `sidebarColumns` = clamp(`msg.X − grabOffset − dividerMargin`), `relayout()` when it changed; and `tea.MouseReleaseMsg` → ends the drag, returning the save command from item 3 when the width differs from the press. Motion and release without an active drag do nothing.
- `theme.go` gains `dividerActive`; `body()` and `rule()` use it for `paneSeparator` and the joint while `drag.isActive`.

**Files:** internal/ui/mouse.go, internal/ui/layout.go, internal/ui/model.go, internal/ui/theme.go, internal/ui/mouse_test.go, internal/ui/testdata/golden/ (the one new mid-drag golden only)
**Read first:** internal/ui/mouse.go — mouseClick, isMouseIgnored; internal/ui/layout.go — pointAt, body, rule, dividerColumn, detailLeft, isSplit; internal/ui/model.go — Update, View; internal/ui/theme.go — theme, newTheme; internal/ui/theme_test.go — TestTheme_Presets_MatchANSIGoldens, newThemeModel; internal/ui/mouse_test.go — click, smallWidth, TestMouse_ClickOnTheDividerOrItsMargins_SelectsNothing; internal/ui/harness_test.go — press, goldenANSI, newTestOptions

**Tests:** press on each of the three hit columns, move right 10, release → width +10 and the state file holds it; a move past either limit clamps; press and release without motion writes nothing; a press on the `┬` rule row or the tab bar starts nothing; motion with no drag changes nothing; an ANSI golden of the frame mid-drag shows the styled divider and joints; the three theme goldens pass unchanged; no drag at `smallWidth` or with the help screen open.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
- `go vet ./internal/ui/`

**Commit:** `feat(ui): drag the divider to resize the sidebar`

## 5. Double-click the divider to reset its width

Depends on item 4.

**What:**
**Goal:** A second left press on the divider's hit area within 500 ms of the first returns the sidebar to the default width, clears the stored width in `state.yaml` and starts no drag.

**Regression guard.** Item 4's drag tests run on the real clock, so once this lands a test chaining two divider presses on one model (the clamp-past-each-limit test) would read the second as a double-click: give those tests a fake clock that moves on at least `doubleClickWindow` per press, or start every item-4 drag from a fresh model. A release that ends a drag which changed the width zeroes `lastDividerPress`, so a fresh grab right after a quick drag is not a reset.

**Approach (assumed at the header base):**
- Model field `now func() time.Time` (default `time.Now`; tests replace it, as `tick` is replaced) and `lastDividerPress time.Time`. In the press branch from item 4: within `doubleClickWindow = 500 * time.Millisecond` of `lastDividerPress` → `sidebarColumns = 0`, `relayout()`, zero `lastDividerPress`, return the save command; otherwise record the press and start the drag.

**Files:** internal/ui/mouse.go, internal/ui/model.go, internal/ui/mouse_test.go
**Read first:** internal/ui/mouse.go — mouseClick; internal/ui/model.go — model (tick field), newModel, Update; internal/ui/harness_test.go — startModel, noTick, press; internal/ui/mouse_test.go — click, TestMouse_ClickOnTheDividerOrItsMargins_SelectsNothing

**Tests:** with a fake clock: drag to 60, advance the clock past `doubleClickWindow`, then two presses 200 ms apart → default width and no `sidebar_width` key in the file; a drag that changes the width, then a press within 500 ms of the drag's first press → a drag, no reset; two presses 600 ms apart → no reset; a press on the divider then one on a sidebar row → no reset; item 4's chained drag tests still pass.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
- `go vet ./internal/ui/`

**Commit:** `feat(ui): reset the sidebar width with a double-click on the divider`

## 6. Help screen and user docs

Depends on item 5.

**What:**
Recast at the regression check (2026-10-16).
**Goal:** The help screen, the terminal-UI guide, the TUI design note and the file-format reference describe the resize keys, the drag, the double-click reset, the limits and `state.yaml`.

**Regression guard.** The help screen gains exactly one line (wording may be tightened, still one row): `helpLines` holds 19 rows and `bodyHeight` at 80x24 is 20, so a second new line pushes "? or Esc closes this help." off screen.

**Approach (assumed at the header base):**
- `help.go`'s `helpLines`: a single new key row naming `< >`, the drag and the double-click reset, e.g. `resize   < >  drag, double-click resets` (no separate note line); regenerate the `help-120x30` golden; add the test that every `helpLines` row fits within `bodyHeight` at 80x24 (`smallWidth` x `smallHeight`).
- `docs/site/getting-started/terminal-ui.md`: the mouse and keys sections. `docs/design/tui.md`: the layout section's width rule (its **Divider** bullet), its Keys block copying `helpLines`, and its **Mouse** paragraph — `grep -rn "help     ?\|a click selects" docs/site docs/design` finds the copies.
- `docs/site/reference/file-format.md`: a `state.yaml` section (path per OS, shape, written only by capcom ui, safe to delete). Rule: every sentence stating capcom ui keeps or reads one file outside / belonging to no repository now names both `config.yaml` and `state.yaml` — `grep -rnE "outside the repository|belongs to no repository|one file" docs/site` finds them, frontmatter `description` included.

**Files:** internal/ui/help.go, internal/ui/testdata/golden/help-120x30.golden, internal/ui/layout_test.go, docs/site/getting-started/terminal-ui.md, docs/design/tui.md, docs/site/reference/file-format.md
**Read first:** internal/ui/help.go — helpLines; internal/ui/layout.go — body, bodyHeight, frameRows; internal/ui/layout_test.go — TestModel_View_HelpScreen_MatchesGolden, TestModel_HelpKeys_ToggleTheHelpScreen;
internal/ui/mouse_test.go — smallWidth, smallHeight; internal/ui/harness_test.go — golden, isGoldenUpdate; docs/site/reference/file-format.md — frontmatter description, intro paragraph, The user config;
docs/site/getting-started/terminal-ui.md — Keys, Mouse, Theme; docs/design/tui.md — Layout (Divider bullet), Keys (key-map block, **Mouse**), Theme, Docs

**Tests:** the help golden; a test (name containing `Help`, so the acceptance `-run Help` picks it up) that every `helpLines` row fits within `bodyHeight` at `smallWidth` x `smallHeight` (80x24).

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/ -run Help`
- `grep -n "state.yaml" docs/site/reference/file-format.md docs/site/getting-started/terminal-ui.md`

**Commit:** `docs(ui): document resizing the sidebar and the state file`

## 7. Decision record ADR 0014 for the state file

Depends on item 1.

**What:**
**Goal:** Note `adr-0014` (with children `adr-0014-decision` and `adr-0014-rejected`) records why capcom ui keeps a machine-written `state.yaml` beside the hand-owned `config.yaml`; `decisions`, `adr-0011-decision` and `docs-layout` point to it, and `capcom check` is clean.

**Approach (assumed at the header base):**
- Add the Notes with `capcom know add` under `decisions`, shaped as `adr-0013`: Status line and summary in `adr-0014`; the decision (path, written only by capcom ui at the write times above, missing = defaults, unknown keys dropped on save, unusable = notice) in `-decision`; rejected: rewriting `config.yaml` in place, a share-of-width unit, writing on quit — in `-rejected`.
- Append `adr-0014` to the list in `decisions`; add one sentence to `adr-0011-decision` naming ADR 0014; reword `docs-layout`'s "the one file capcom keeps outside the repository" to cover both files.

**Files:** .capcom/knowledge/decisions/adr-0014.md, .capcom/knowledge/decisions/adr-0014/adr-0014-decision.md, .capcom/knowledge/decisions/adr-0014/adr-0014-rejected.md, .capcom/knowledge/decisions.md, .capcom/knowledge/decisions/adr-0011/adr-0011-decision.md, .capcom/knowledge/docs-layout.md
**Read first:** .capcom/knowledge/decisions.md — records list; .capcom/knowledge/decisions/adr-0013.md — Status line shape; .capcom/knowledge/decisions/adr-0013/adr-0013-rejected.md — frontmatter labels; .capcom/knowledge/decisions/adr-0011/adr-0011-decision.md — The file, Only capcom ui reads it; .capcom/knowledge/docs-layout.md — file-format.md bullet

**Tests:** none (Knowledge only).

**Acceptance:**
- `capcom check`
- `capcom know show adr-0014 && capcom know tree adr-0014`

**Commit:** `docs(decisions): record the capcom ui state file as ADR 0014`
