# Select text in the detail pane with the mouse and copy it

**Goal:** in `capcom ui`, a left press and drag in the detail pane selects rendered text,
highlighted in the `selection` role, and release copies it through OSC 52, the native clipboard
and tmux, with a `copied N chars` notice. It is built the way apogee builds it.
**Date:** 2026-10-16
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 38fe2cb
**Closes:** ui-select-copy

**Sources:**
- `.capcom/issues/capcom-ui/ui-select-copy.md` (the authority for every behaviour below)
- `docs/design/tui.md` (Keys → Mouse, Layout, the apogee-borrowing section)
- apogee (`/home/airic/repos/apogee`): `internal/tui/mouse.go` (design comment, `transcriptSel`,
  `spanUnchanged`, `shadeCells`, `transcriptSelectionText`), `internal/tui/clipboard.go`,
  `internal/present/clipboard.go`, `internal/security/execsafety.go`,
  `.capcom/knowledge/decisions/adr-0030.md` (one width authority)

**Ratified design calls** (user, 2026-10-16):
- **Windows clipboard:** `github.com/atotto/clipboard` `WriteAll`, promoted to a direct dependency.
- **Tabs:** selection works in the detail pane on the Ready, Issues and Knowledge tabs.
- **Notice:** `copied N chars` is the first notice in the status bar, after the lock and the filter.
- **Help:** the help screen gains `copy  drag over the detail text` after the resize line.

**Standing requirements:**
- skills: coding-standards
- Tests per `AGENTS.md`: single packages with `GOMEMLIMIT=2GiB`, never `./...` with `-race`, never two test commands at once.
- No test touches the real clipboard, runs tmux, or sleeps on a real tick.
- Any authorized deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- Selection in the sidebar, the dependency view, the help screen, the status picker and the prompts.
- Keyboard selection; double- and triple-click; a copy key.

**Regression check (2026-10-16, 38fe2cb):**
- 1: guard folded
- 2: guard folded; recast (maps `selection.go` in `internal/ui/doc.go`)
- 3: guard folded
- 4: guard folded; recast (maps `clipboard.go` in `internal/ui/doc.go`)
- 5: guard folded; recast (no file-map entries for `selection.go` or `clipboard.go`)
- 4 (re-check): guard folded (re-record item 3's post-release goldens)
- 5 (re-check): guard folded (supersedes `docs/design/tui.md` "borrows their patterns, not their code")

## 1. Add the clipboard package: native and tmux writes behind a working-tree fence

**What:**
**Goal:** a new package `internal/clipboard` writes text to the host clipboard and to tmux.
Every external program is resolved on `PATH` and refused when it resolves inside the working
tree. Tests swap the lookup, the runner and the Windows writer, so no real program runs.
**Approach (assumed at the header base):** port apogee's `present.Clipboard` and slim it down,
with no `security` package. Use one deep type, `clipboard.Writer{GOOS string; Getenv func(string) string;
Root string; LookPath func(string) (string, error); Run func(ctx context.Context, stdin, program
string, args ...string) error; WriteWindows func(string) error}`. A nil field means the production
default: `exec.LookPath`, run via `exec.CommandContext` with stdin and null stdout/stderr, and
`clipboard.WriteAll` from atotto.
- `WriteNative(text)`:
  - On windows it calls `WriteWindows`.
  - On darwin it tries `pbcopy`.
  - On the unix family it tries `wl-copy` (only with `WAYLAND_DISPLAY` set), then
    `xclip -in -selection clipboard`, `xsel --input --clipboard`, `termux-clipboard-set`, `clip.exe`.
  - The first program that resolves is run. A refused program stops the walk and returns
    `ErrRefused`. Nothing installed returns `ErrNoProgram`.
- `WriteTmux(text)` is a no-op unless `TMUX` is set. Otherwise it runs `tmux load-buffer -w -`.
- Both run under a 2 s timeout.
- The fence:
  - A lookup error wrapping `exec.ErrDot`, or a non-absolute result, is refused.
  - Any other lookup error means the program is not available.
  - Otherwise `filepath.EvalSymlinks` the result. It is refused when
    `store.RelativePath(Root, resolved)` reports it inside. An empty `Root` fences nothing.
- Add `doc.go` with the package comment. Promote atotto in `go.mod` with `go mod tidy`.

**Regression guard.** When `filepath.EvalSymlinks` fails on the lookup result, fence the cleaned
path instead, as apogee's `EvalRealPath` does. Resolve `Root` with `filepath.EvalSymlinks` before
`store.RelativePath`. Tests' fake `LookPath` answers real files made under an outside
`t.TempDir()`, never literal unix paths, so the rows hold on macOS and Windows CI.
**Files:** `internal/clipboard/clipboard.go`, `internal/clipboard/doc.go`, `internal/clipboard/clipboard_test.go`, `go.mod`, `go.sum`
**Read first:** `internal/store/store.go` — RelativePath; `internal/gitx/gitx.go` — Discover; `internal/store/store_test.go` — skipSymlinkTest; `internal/elsewhere/elsewhere_test.go` — newRepo;
`/home/airic/repos/apogee/internal/present/clipboard.go` — WriteSystem, systemCandidates; `/home/airic/repos/apogee/internal/security/execsafety.go` — ResolveProgram;
`/home/airic/repos/apogee/internal/security/pathsafety.go` — EvalRealPath
**Tests:** a table over GOOS and env for the candidate order; a program under `Root` (and a
symlink into it) refused with nothing run; `ErrDot` refused; tmux skipped without `TMUX`, argv
pinned with it; Windows calls `WriteWindows` only. The fake `LookPath` answers real files under
an outside `t.TempDir()`; a lookup result that does not resolve falls back to its cleaned path.
The symlink row skips on Windows and as root, as `skipSymlinkTest` does.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/clipboard/` passes.
- `go vet ./internal/clipboard/` is clean; `GOOS=windows go build ./internal/clipboard/` and `GOOS=darwin go build ./internal/clipboard/` succeed.
**Commit:** `feat(clipboard): write to the native clipboard and tmux behind a working-tree fence`

## 2. Add the detail selection: its state, painting and text

**What:**
Recast at the regression check (2026-10-16).
**Goal:** `internal/ui/selection.go` holds a value type for a selection in content coordinates.
Functions over it paint the selected cells of the rendered lines and extract the copied text.
They measure only through `markdown.Measure`.
**Approach (assumed at the header base):** port apogee's `transcriptSel` and `contentCell` as
`detailSel{isActive bool; anchor, head contentCell}` and `contentCell{line, col int}`, with these methods:
- `isCollapsed()`.
- `ordered()` returns the selection in reading order.
- `spanUnchanged(old, new []string) bool` keeps apogee's semantics: inactive is false, collapsed
  is true, otherwise every spanned line must be equal and in bounds.
- `shade(measure, line, c0, c1, style)`: `Cut(0,c0)` + `style.Render(ansi.Strip(Cut(c0,c1)))` +
  `Cut(c1,width)`. A `c1 <= c0` returns the line unchanged.
- `paint(measure, lines, offset, rows, width, style) []string` shades the visible rows and returns
  a new slice; it never mutates `lines`. The first spanned line runs from `top.col` and the last
  to `bottom.col`; others run `0..width`.
- `text(measure, lines, width) string` uses the same `c0`/`c1`. Each line is
  `strings.TrimRight(ansi.Strip(Cut(...)), " ")`, and lines are joined with `\n`.
- Never call `ansi.Cut`, `ansi.StringWidth` or `lipgloss.Width` here (ADR 0030).
- No caller in this item; item 3 wires it.

**Regression guard.** `paint` returns a copy of `len(lines)` rows with only the spanned rows in
`[offset, offset+rows)` shaded, so `scrolledPaneRows` still takes `len(lines)` for the thumb and
slices from `offset` itself. Each item that adds a non-test file to `internal/ui` maps it in
`internal/ui/doc.go` in that same item, so `TestDocFileMap_MatchesDirectory` stays green: this
item adds the `selection.go` entry.
**Files:** `internal/ui/selection.go`, `internal/ui/selection_test.go`, `internal/ui/doc.go`
**Read first:** `internal/ui/markdown/width.go` — Measure.Cut; `/home/airic/repos/apogee/internal/tui/mouse.go` — transcriptSel.spanUnchanged, shadeCells, transcriptSelectionText;
`internal/ui/scrollbar.go` — scrolledPaneRows; `internal/ui/layout.go` — detailRows; `internal/ui/theme.go` — theme.selectedRow; `internal/ui/doc.go` — file map
**Tests:** the text over two styled lines has no ANSI codes or trailing spaces; a reversed drag
equals the forward one; a line with CJK and emoji cuts whole runes identically in `paint` and
`text`; collapsed paints nothing; `spanUnchanged` keeps a change outside the span and drops one
inside; `len(paint(...)) == len(lines)` with `offset > 0`, and only rows in the visible window
are shaded.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'TestSelection|TestDocFileMap' ./internal/ui/` passes.
- `go vet ./internal/ui/` is clean.
**Commit:** `feat(ui): add the detail selection's state, painting and text`

## 3. Select with press and drag in the detail pane; follow links on a bare click

Depends on item 2.

**What:**
**Goal:**
- A left press in the detail pane's text anchors a selection and a left drag extends it. A press on
  the scrollbar does not. Motion off the pane does not extend it.
- The selection is painted in `m.theme.selectedRow` over the rendered lines and survives a wheel
  scroll.
- A bare click (press and release on one cell) follows a link at release and copies nothing.
- A keypress, a press elsewhere, another issue or Note, the dep view opening, a tab switch, or a
  re-render that changes a spanned line clears the selection.
- The dep view, the picker and the divider drag behave as before.
**Approach (assumed at the header base):**
- Add `model.sel detailSel`.
- In `mouseClick`, a left press at `pointAt` → `paneDetail` with `col < detailWidth()` and the dep
  view closed sets `sel` collapsed at `(detailOffset+row, col)`. Any other press clears it.
- The link follow moves out of `clickDetail` to `mouseRelease`, which gains its msg. On a collapsed
  selection it calls `issue.linkAt(anchor.line, anchor.col)` → `followLink`, then clears.
- The dep view's `rowAtLine` click stays press-time.
- `mouseMotion` with the left button and an active `sel` sets `head` from `pointAt` when it lands
  on `paneDetail`, the col clamped to `[0, detailWidth()]`. The divider-drag path is unchanged.
- Paint in `detailRows()` (or right before `scrolledPaneRows` in `body`) via `sel.paint`. Keep it
  out of the sidebar and the scrollbar.
- Clearing:
  - At the top of key handling in `Update`.
  - In `renderDetail` when `!sel.spanUnchanged(old, new)`.
  - At every site that resets `m.issue` or opens the dep view (`grep -n 'issueDetail{}' internal/ui/*.go`, plus `depview.go` open).
- Use the existing `release(x, y)` helper (mouse_test.go). The four `TestMouse_ClickOn*Link*` tests send
  click then release and keep their assertions.

**Regression guard.** Also clear `sel` in `applyDetail` when `!isShown` (the id changed): a
sidebar wheel or a live refresh switches issues through `applyDetail` without a key or a press.
The `issueDetail{}` resets in `issues.go`, `ready.go` and `knowledge.go` are in scope.
**Files:** `internal/ui/mouse.go`, `internal/ui/model.go`, `internal/ui/layout.go`, `internal/ui/detail.go`, `internal/ui/keys.go`, `internal/ui/depview.go`, `internal/ui/issues.go`, `internal/ui/ready.go`, `internal/ui/knowledge.go`, `internal/ui/mouse_test.go`
**Read first:** `internal/ui/mouse.go` — mouseClick, mouseRelease, mouseMotion, clickDetail; `internal/ui/detail.go` — applyDetail, renderDetail;
`internal/ui/layout.go` — pointAt; `internal/ui/mouse_test.go` — release
**Tests:**
- A drag over two body lines paints exactly those cells, verified by an ANSI golden or by
  checking the fill on the stripped columns.
- A drag on a Note and over the header lines.
- A wheel mid-drag keeps the anchor line.
- Each clear trigger listed in the Goal; the other-issue trigger is a sidebar wheel notch, not a
  key.
- A press-drag that starts on a link follows nothing.
- All existing `TestMouse_*` divider, link and wheel tests pass.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'TestMouse|TestSelection|TestDetail' ./internal/ui/` passes.
- `go vet ./internal/ui/` is clean.
**Commit:** `feat(ui): select text in the detail pane with press and drag`

## 4. Copy the selection on release and show `copied N chars`

Depends on items 1 and 3.

**What:**
Recast at the regression check (2026-10-16).
**Goal:**
- Releasing a non-collapsed selection whose text is not blank copies that text. It goes through
  `tea.SetClipboard`, the native clipboard (`clipboard.Writer.WriteNative`) and tmux
  (`WriteTmux`), and the highlight stays.
- The status bar's first notice shows `copied N chars` (`char` for one, N = runes) for about two
  seconds, even when the native or tmux write fails.
- A blank selection copies nothing and is cleared.
**Approach (assumed at the header base):**
- Port apogee's `copyFlash`/`showFlash`: `model.flash string`, `flashGen int`,
  `flashClearMsg{gen int}`, `flashDuration = 2 * time.Second`.
- `Update` clears `flash` only when the gen matches.
- `statusBar` puts `m.flash` first in its notices slice.
- Seams on `model`, set in `newModel`, swapped in `startModel` (beside `m.tick = noTick`):
  - `setClipboard func(string) tea.Cmd` (default `tea.SetClipboard`)
  - `writeNative, writeTmux func(string) error` (default: a `clipboard.Writer` with
    `runtime.GOOS`, `os.Getenv` and `Root` read from `options.Store.WorktreeRoot` at
    `newModel`)
  - `flashTick` (default `tea.Tick`)
- Native and tmux run as Cmds that drop their error and return nil.
- `mouseRelease` returns `tea.Batch` of the four.
- Put the clipboard code in `internal/ui/clipboard.go`.

**Regression guard.** Swap `setClipboard`, `writeNative`, `writeTmux` and `flashTick` in
`startModel`, not `newTestModel`, so `newIssuesModel` and `newKnowledgeModel` never run real
xclip, tmux or a real tick. The `flashTick` fake schedules nothing, like `noTick`. This item adds
the `clipboard.go` entry to the `internal/ui/doc.go` file map. `goldenANSI` pins the whole View
with its status row, so re-record any item-3 golden captured after a selection release with
`-update -run TestMouse` and review that its only diff is the `copied N chars` notice.
**Files:** `internal/ui/clipboard.go`, `internal/ui/clipboard_test.go`, `internal/ui/mouse.go`, `internal/ui/model.go`, `internal/ui/statusbar.go`, `internal/ui/harness_test.go`, `internal/ui/doc.go`, `internal/ui/testdata/golden/` (item 3's post-release drag golden, if any)
**Read first:** `internal/ui/mouse.go` — mouseRelease; `internal/ui/harness_test.go` — startModel, noTick, goldenANSI; `internal/ui/model.go` — newModel;
`internal/ui/statusbar.go` — statusBar; `internal/ui/doc.go` — file map; `internal/store/store.go` — Store.WorktreeRoot
**Tests:**
- A two-line drag on an issue body records the exact `setClipboard` text and native/tmux calls,
  and `statusRow` shows `copied N chars`.
- One char shows `copied 1 char`.
- A drag over blank cells calls nothing.
- Failing native and tmux fakes still show the notice.
- A stale `flashClearMsg` keeps a newer flash; the test sends `flashClearMsg{gen}` itself.
- A wide-rune line copies the highlighted runes.
- Any item-3 golden captured after a release is re-recorded and reviewed: only the status row's
  `copied N chars` notice differs.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'TestClipboard|TestMouse|TestStatusBar|TestDocFileMap' ./internal/ui/` passes.
- `go vet ./internal/ui/` is clean.
**Commit:** `feat(ui): copy the detail selection on release`

## 5. Document selecting and copying

Depends on item 4.

**What:**
Recast at the regression check (2026-10-16).
**Goal:**
- The design, the user guide, the help screen and the package file map describe selecting and
  copying in the detail pane.
- No doc still says text selection needs shift-drag without saying the detail pane selects on its
  own.
**Approach (assumed at the header base):**
- `docs/design/tui.md`:
  - The Mouse paragraph: press-drag selects, release copies through OSC 52, native and tmux with
    the working-tree fence, and a bare click follows a link.
  - The apogee-borrowing section names the selection and the clipboard routes.
- `docs/site/getting-started/terminal-ui.md`:
  - The `## Mouse` section: shift-drag stays for the sidebar and the rest of the screen.
  - The key table gains the copy line.
- `internal/ui/help.go` `helpLines`: `  copy     drag over the detail text`, aligned under the resize
  line.
- `internal/ui/doc.go` file map: reword the `mouse.go` entry if it describes it without selection.
- `internal/clipboard` stays out of `internal/ui/doc.go`.
- Find every site with `grep -rn -i 'shift' docs/ internal/ui/help.go`.

**Regression guard.** Item 5 no longer adds file-map entries for `selection.go` or
`clipboard.go`: it only rewords the `doc.go` entry for `mouse.go` if it still describes `mouse.go`
as clicks, divider drags, links and wheel without selection. Keep `helpLines` at 20 rows (80x24
fit): add the copy row after resize and merge or drop another row (e.g. `deps d` shares a row,
or a blank row goes), and state which in the item's NOTES. Every key block mirroring `helpLines`
(`grep -rn 'resize   < >' docs/design docs/site`) gains the copy line. Items 2 and 4 port apogee's
`mouse.go` code, superseding `docs/design/tui.md:266-267` ("borrows their patterns, not their
code"): in `### Taken from apogee`, move the selection (from `mouse.go`) and the clipboard routes
under **Ported** and reword **Written new** so it no longer says no `mouse.go` code was borrowed.
**Files:** `docs/design/tui.md`, `docs/site/getting-started/terminal-ui.md`, `internal/ui/help.go`, `internal/ui/doc.go`, `internal/ui/testdata/golden/help-120x30.golden`
**Read first:** `internal/ui/help.go` — helpLines; `internal/ui/layout_test.go` — TestModel_View_HelpScreen_FitsTheSmallTerminal, TestModel_View_HelpScreen_MatchesGolden;
`internal/ui/doc.go` — file map (`mouse.go` entry); `docs/design/tui.md` — ## Keys, **Mouse**, ### Taken from apogee;
`docs/site/getting-started/terminal-ui.md` — ## Mouse
**Tests:** the help golden `help-120x30.golden` is updated with `-update -run 'HelpScreen'` and
reviewed; `TestModel_View_HelpScreen_FitsTheSmallTerminal` stays green at 80x24.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'HelpScreen|TestDoc' ./internal/ui/` passes.
- `go vet ./internal/ui/` is clean.
- `grep -rn -i 'shift' docs/design/tui.md docs/site/getting-started/terminal-ui.md` is reviewed: no hit says selection needs shift-drag without saying the detail pane selects on its own.
**Commit:** `docs(ui): describe selecting and copying in the detail pane`
