# apogee colours, sidebar columns and a version label for capcom ui

**Goal:** `capcom ui` draws in apogee's colours, read from one embedded YAML file per scheme; the
sidebar's status and priority line up in columns at every tree depth; the tab bar's right end
reads `v<version> · capcom`.
**Date:** 2026-10-16
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 2dd6a74

**Sources:**
- `.capcom/issues/capcom-ui/ui-apogee-colors.md`, `ui-sidebar-columns.md`, `ui-version-label.md`
- `~/repos/apogee/internal/scheme/schemes/dark.yaml`, `light.yaml` (apogee's colours and file format)
- Note `adr-0011` (per-user config; `ui.theme`, `ui.colors`)
- `internal/ui/theme.go`, `internal/ui/tree.go`, `internal/ui/issues.go`, `internal/ui/ready.go`, `internal/ui/layout.go`

**Ratified design calls** (user, 2026-10-16):
- **Scheme files:** embedded YAML, `internal/ui/schemes/dark.yaml` and `light.yaml`, apogee's format (`role: "#rrggbb"  # comment`), one key per role of `userconfig.Roles`.
- **Mapping (apogee → capcom):** gauge → accent; chrome → border; muted → muted; skill → link; selection → selection; tool-header → heading; code → code; muted-bright → status-todo; mode-allow-edits → status-doing; mode-auto → status-waiting; success → status-done; chrome → status-dropped.
- **Columns:** id column as wide as the widest connector + arrow + id of the rows; status column as wide as the widest status; two spaces between columns; Ready list gets the same columns; Knowledge rows unchanged.
- **Version label:** right end of the tab bar, its last character in the frame's last column (flush with the rule's right end); dropped when it does not fit.
- **Selection:** the selected row is a Background(selection) fill with its text in its usual colours, the status keeping its category colour; Reverse only when the palette has no selection colour (NO_COLOR).

**Standing requirements:**
- skills: coding-standards
- Tests per `AGENTS.md`: single packages with `GOMEMLIMIT=2GiB`, never `./...` with `-race`, never two test commands at once.
- Goldens are rewritten with `go test ./internal/ui/ -run <Test> -update`, and every rewritten golden is read before committing.
- Any authorized deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- Switching schemes at run time, a user schemes folder, new colour roles or `ui.theme` values.
- Aligning the CLI's `capcom list --tree` and `capcom ready` output.
- Any change to the status bar.

**Regression check (2026-10-16, 2dd6a74):**
- 1: guard folded (scheme-file test pins hex case-insensitively).
- 2: decided (user, 2026-10-16): selection trade-off settled; the selected row's drawing moves to new item 3.
- 3: new item (user, 2026-10-16): selected row drawn as a Background(selection) fill keeping its colours; Reverse under NO_COLOR.
- 4: guard folded (dep view stays unaligned; lead width; Ready assertion; CLI-mirroring comments superseded).
- 5: guard folded (Acceptance checks that capcom ui passes the version).
- 2 (re-check): decided: `drag-divider-120x30.golden` added to Files and to the goldens rewritten (it carries accent, border, heading and selection SGRs).
- 3 (re-check): guard folded (`drag-divider-120x30` golden; `knowledge.go` and `tree_test.go` in Files; the fill starts after the connector, yields to `docs/design/tui.md:51`; the Issues-row check as its own test; widened comment grep).
- 4 (re-check): decided: column padding built on item 3's styled-segment row contract.

## 1. Move the presets into embedded scheme files — ✅ DONE (2026-10-16)

NOTES (2026-10-16): added TestTheme_LoadSchemes_UndecodableFile_PanicsNamingIt beside the two planned tests (loadSchemes takes an fs.FS so the panic-naming-the-file contract is pinned with an fstest.MapFS).

**What:**
**Goal:** the dark and light presets are read from `internal/ui/schemes/dark.yaml` and
`light.yaml`, embedded in the binary; no Go file of `internal/ui` outside tests names a hex
colour; every golden renders byte-identical to the base.
**Approach (assumed at the header base):** write both files with the colours `presets` in
`theme.go` holds today, one `role: "#rrggbb"` line per role of `userconfig.Roles` in that order,
each with a short comment naming what the role paints (from `theme`'s field docs), and a header
comment in apogee's style (what a scheme is, quoted hex, one key per role). Embed them with
`//go:embed schemes/*.yaml`; build `presets` (same shape, keyed by `userconfig.ThemeDark` /
`ThemeLight`, the file's basename) by decoding each with `gopkg.in/yaml.v3` into
`map[string]string`. The files are compiled in, so a decode failure is a programmer error: the
loader panics naming the file, and a test makes that unreachable. `newPalette` and every caller
stay unchanged. In `docs/design/tui.md`'s Theme section add one sentence naming the scheme files
as where the preset colours live.
**Regression guard.** The scheme-file test matches each value case-insensitively, `^#[0-9a-fA-F]{6}$`
(as `userconfig.hexColor`, `internal/userconfig/userconfig.go:72`), since item 2 writes apogee's
`code: "#70BBFF"` verbatim; a lower-case-only pattern would turn item 1's test red on item 2's run.
**Files:** `internal/ui/schemes/dark.yaml`, `internal/ui/schemes/light.yaml`, `internal/ui/theme.go`, `internal/ui/theme_test.go`, `docs/design/tui.md`
**Read first:** `internal/ui/theme.go` — presets, newPalette, presetName; `internal/ui/theme_test.go` — TestTheme_Presets_ColourEveryRole, TestTheme_Presets_MatchANSIGoldens;
`internal/userconfig/userconfig.go` — Roles, hexColor; `version.go` — //go:embed VERSION
**Tests:** in `theme_test.go`, a test that each embedded scheme file names exactly the roles of
`userconfig.Roles` (none missing, none extra), each value matching `^#[0-9a-fA-F]{6}$`; and that `presets` holds
exactly the dark and light schemes.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/` passes with no golden rewritten (`git status --short internal/ui/testdata` empty).
- `grep -nE '"#[0-9a-fA-F]{3,6}"' internal/ui/*.go | grep -v _test.go` prints nothing.
- `go vet ./internal/ui/`
**Commit:** `refactor(ui): read the colour presets from embedded scheme files`

## 2. Draw the presets in apogee's colours — ✅ DONE (2026-10-16)

**What:**
**Goal:** `dark.yaml` and `light.yaml` hold apogee's colours by the header's mapping, so each
role renders in the colour of its apogee role from apogee's scheme file of the same name.
**Approach (assumed at the header base):** set each role's value from the apogee role the
mapping names, read from `~/repos/apogee/internal/scheme/schemes/<name>.yaml` (dark: accent
`#c396ff`, border `#4a4a4a`, muted `#8a8a8a`, link `#40b0ff`, selection `#3a5fcd`, heading
`#aea580`, code `#70BBFF`, status-todo `#b2b2b2`, status-doing `#58a6ff`, status-waiting
`#f0883e`, status-done `#43d389`, status-dropped `#4a4a4a`; light: `#8250df`, `#d0d7de`,
`#656d76`, `#6639ba`, `#b3d1ff`, `#1f6feb`, `#bc4c00`, `#424a53`, `#0969da`, `#bc4c00`,
`#115e59`, `#d0d7de`, same order). Each line's comment also names its apogee role; the header
comment says the colours are apogee's. Rewrite the `theme-dark`, `theme-light` and `drag-divider` goldens.
Depends on item 1.
**Regression guard.** The selected row's drawing changes in the new item 3 (below); this item leaves `selectedRow` / `mutedSelectedRow` as they are.
**Files:** `internal/ui/schemes/dark.yaml`, `internal/ui/schemes/light.yaml`, `internal/ui/theme_test.go`, `internal/ui/testdata/golden/theme-dark-120x30.golden`, `internal/ui/testdata/golden/theme-light-120x30.golden`, `internal/ui/testdata/golden/drag-divider-120x30.golden`
**Read first:** `internal/ui/theme.go` — newTheme, selectedRow, mutedSelectedRow, newPalette; `internal/ui/theme_test.go` — TestTheme_Presets_MatchANSIGoldens,
TestTheme_NoColor_DrawsNoColourSequences; `internal/ui/harness_test.go` — goldenANSI; `~/repos/apogee/internal/scheme/schemes/dark.yaml` — selection
**Tests:** in `theme_test.go`, a table test that `newPalette` under theme dark and light gives
accent, selection, status-doing and status-done the mapped values above; the two theme goldens and
`drag-divider-120x30` (`TestMouse_MidDrag_MatchesANSIGolden`) rewritten;
`theme-no-color-120x30` unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/`
- `grep -c '#c396ff' internal/ui/schemes/dark.yaml` prints 1; `grep -c '#8250df' internal/ui/schemes/light.yaml` prints 1.
**Commit:** `feat(ui): draw capcom ui in apogee's colours`

## 3. Draw the selected row as a background fill

**What:**
**Goal:** under a palette with a selection colour, the selected row of every list painted with
`theme.selectedRow` / `mutedSelectedRow` (Issues and Knowledge trees via `treeWidget.paint` /
`rowStyle`, the Ready list in `paintReadySidebar`, the dep view in `depview.go`, the status picker
via `picker.lines` in `layout.go`) is drawn with the selection colour as its background across the
whole row text, the text keeping its usual colours: an issue row's status in its category colour,
the tree connector in the border colour. A muted selected tree row is the same fill with faint
text. Under NO_COLOR (a palette without a selection colour) the selected row is drawn in Reverse
exactly as at the base, so `theme-no-color-120x30.golden` stays byte-identical; non-selected
muted rows are unchanged.
**Approach (assumed at the header base):** a reset inside a styled span ends an outer background
early, so each coloured segment of the selected row carries the background itself (e.g. the
status style composed with `Background(selection)`), or the fill is re-applied after each inner
reset. The `rowText(row, isMarked)` contract of `treeWidget.paint`, `issueTreeRow` and `readyRow`'s
"empty style for a marked row" convention change accordingly, and their doc comments are updated
(rule: every comment saying a marked row's text must carry no styling of its own; find with
`grep -n "reset inside\|isMarked\|carry no styling" internal/ui/*.go`). Rewrite the `theme-dark`
and `theme-light` goldens and read each.
Depends on item 2.
**Regression guard.** The fill starts after the connector, as today: the connector stays plain `th.border`, outside the fill (the item yields to `docs/design/tui.md:51`). Also rewrite `drag-divider-120x30.golden` with `go test ./internal/ui/ -run TestMouse_MidDrag_MatchesANSIGolden -update` and read it.
The `rowText` change reaches `noteTreeRow` (`knowledge.go:176`, unstyled: the Knowledge selected row keeps its fill, by `noteTreeRow` getting it or `paint` applying it) and the `paintIDs` / `:181` closures in `tree_test.go`, whose
`TestTreeWidget_Paint_MarksTheSelectionAfterTheConnector` want is rebuilt from the new fill styles. The comment grep is widened to
`grep -n -i "marked\|reset inside\|carry no styling" internal/ui/*.go` plus `grep -n "highlights start" docs/design/tui.md` (it catches `issues.go:157`).
**Files:** `internal/ui/theme.go`, `internal/ui/tree.go`, `internal/ui/issues.go`, `internal/ui/ready.go`, `internal/ui/depview.go`, `internal/ui/layout.go`, `internal/ui/knowledge.go`, `internal/ui/theme_test.go`, `internal/ui/tree_test.go`, `internal/ui/testdata/golden/*.golden`
**Read first:** `internal/ui/tree.go` — treeWidget.paint, rowStyle; `internal/ui/theme.go` — newTheme, theme.status; `internal/ui/issues.go` — issueTreeRow; `internal/ui/ready.go` — paintReadySidebar, readyRow;
`internal/ui/depview.go` — depRow.text; `internal/ui/tree_test.go` — TestTreeWidget_Paint_MarksTheSelectionAfterTheConnector, paintIDs; `internal/ui/theme_test.go` — newThemeModel, sgrSequence;
`internal/ui/mouse_test.go` — TestMouse_MidDrag_MatchesANSIGolden
**Tests:** an Issues test of its own (the theme goldens open on Ready): `newThemeModel` under theme
dark, switch to the Issues tab (press its tab key), then `m.sidebar[0]` carries 48 + the selection
colour on its id, 38 (status-todo) + 48 on `open`, and the fill on its title; on the ANSI output
(`goldenANSI` / theme goldens) the same for the selected Ready row; the NO_COLOR golden unchanged;
the theme-dark, theme-light and `drag-divider-120x30` goldens rewritten;
`TestTreeWidget_Paint_MarksTheSelectionAfterTheConnector`'s want rebuilt from the new fill styles.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/`
- `git diff --quiet internal/ui/testdata/golden/theme-no-color-120x30.golden`
- `go vet ./internal/ui/`
**Closes:** ui-apogee-colors
**Commit:** `feat(ui): draw the selected row as a background fill that keeps its colours`

## 4. Align status and priority in sidebar columns

**What:**
**Goal:** on the Issues tab every tree row's status starts at one column and every `P<n>` at
one column, at any depth; the same on the Ready tab; Knowledge rows render as before.
**Approach (assumed at the header base):** columns are measured over every row the list holds
(`treeWidget.rows` for Issues, `ready.entries` for Ready), not only the rows on screen, so they
do not shift while scrolling. Issues: in `paintIssuesSidebar` compute the id column as the max
over rows of `ansi.StringWidth(connector) + ansi.StringWidth(treeArrow(row)) +
ansi.StringWidth(ID)` and the status column as the max status width; `issueTreeRow` pads the id
to (id column − that row's connector and arrow width) and the status to the status column, then
`P<n>`, the title and the progress suffix as before. Ready: id column = widest id. Extend
`readyRow` with the two pad widths (one formatter for both tabs, as now); pad with spaces
outside the status style, so a coloured status keeps its reset where it is. The column padding is built on the styled-segment row contract item 3 leaves for `readyRow` / `issueTreeRow` (each segment carries its own style; padding spaces of a selected row carry the selection background so the fill stays unbroken). Two spaces still
separate the columns. The row is still cut by the pane width as today. Rewrite every golden the
change alters; read each.
Depends on item 3 (shared goldens).
**Regression guard.** `depRow.text` (`depview.go:63`) also calls `readyRow`: it passes zero pad widths, so the dep view and `deps-both-120x30` stay byte-identical. A row's lead is `row.Depth*connectorWidth` plus an arrow 2 columns wide (`tree.go:17-32`); `paint`'s `rowText` signature stays as it is.
`TestReadyTab_Sidebar_ListsTheReadyListInItsOrder` (`ready_test.go:71`) gets the padded row `beta   open  P2  Beta task`. Supersedes the
comments "as capcom ready prints it" (`ready.go:195`) and "as capcom list --tree prints it" (`issues.go:155`): reword both, since the CLI stays unaligned.
**Files:** `internal/ui/issues.go`, `internal/ui/ready.go`, `internal/ui/tree.go`, `internal/ui/depview.go`, `internal/ui/issues_test.go`, `internal/ui/ready_test.go`, `internal/ui/testdata/golden/*.golden`
**Read first:** `internal/ui/issues.go` — paintIssuesSidebar, issueTreeRow; `internal/ui/ready.go` — readyRow, paintReadySidebar; `internal/ui/depview.go` — depRow.text;
`internal/ui/tree.go` — treeWidget.paint, treeArrow, connectorWidth
**Tests:** an Issues test with an expanded three-level tree and ids of different lengths that
asserts, on `ansi.Strip`ped sidebar rows, one start column for the status and one for `P`; a
Ready test with ids of different lengths asserting the same; a test that a scrolled Issues tree
keeps the columns of its unscrolled view; `TestReadyTab_Sidebar_ListsTheReadyListInItsOrder`'s wanted
row rewritten to `beta   open  P2  Beta task`; the rewritten goldens (`issues-expanded`, `ready`, …);
`deps-both-120x30` unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/`
- `git diff --stat` names no golden of the Knowledge tab (`knowledge-*`).
**Closes:** ui-sidebar-columns
**Commit:** `feat(ui): align status and priority in sidebar columns`

## 5. Show the version and app name at the tab bar's right end

**What:**
**Goal:** the tab bar's row ends in `v<version> · capcom`, its last character in the frame's last
column, painted in the muted hints style; it is left out when it would leave fewer than two
columns between the last tab and the label, and the tabs are never cut for it; `capcom ui` passes
the version of the VERSION file.
**Approach (assumed at the header base):** add `Version string` to `ui.Options`, carried into the
model; `internal/cli/ui.go` sets it from `capcom.Version()`. An empty Version draws no label.
`tabBar` takes the frame width: tabs as now, then spaces, then `m.theme.hints.Render(label)`,
so the row is exactly `m.width` wide. `tabAt` is unchanged: a click on the label or the gap
selects no tab. `newTestOptions` sets a fixed fixture version (not the VERSION file's), so the
goldens are stable across releases; rewrite every golden the tab bar change alters and read each.
Add the label to the Tab bar bullet of `docs/design/tui.md` and of
`docs/site/getting-started/terminal-ui.md`.
Depends on item 4 (shared goldens).
**Regression guard.** Both CLI UI tests (`internal/cli/ui_test.go`) exit before `ui.Run`, so nothing listed catches an unset
Version: Acceptance greps `internal/cli/ui.go` for it. `internal/cli/ui.go` imports `github.com/airiclenz/capcom` for
`capcom.Version()`, as `internal/cli/root.go:9` already does.
**Files:** `internal/ui/ui.go`, `internal/ui/model.go`, `internal/ui/layout.go`, `internal/ui/layout_test.go`, `internal/ui/harness_test.go`, `internal/ui/mouse_test.go`, `internal/cli/ui.go`, `internal/ui/testdata/golden/*.golden`, `docs/design/tui.md`, `docs/site/getting-started/terminal-ui.md`
**Read first:** `internal/ui/layout.go` — tabBar, frame, tabAt; `internal/ui/ui.go` — Options; `internal/ui/model.go` — newModel;
`internal/cli/ui.go` — newUICommand; `version.go` — Version; `internal/ui/harness_test.go` — newTestOptions
**Tests:** in `layout_test.go`: at 120 columns the stripped tab bar ends in `v<fixture> ·
capcom` and is 120 columns wide; at a width where the label does not fit the tab bar equals the
tabs alone; with an empty Version no label. In `mouse_test.go`: a click on the label's columns
changes no tab. The rewritten goldens.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/`
- `GOMEMLIMIT=2GiB go test -count=1 -run UI ./internal/cli/`
- `grep -n 'Version: *capcom.Version()' internal/cli/ui.go` prints one line.
- `go vet ./internal/ui/ ./internal/cli/`
**Closes:** ui-version-label
**Commit:** `feat(ui): show the version and app name at the tab bar's right end`
