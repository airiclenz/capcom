# Close styles at each wrap break, and pad the lock badge

**Goal:** a styled span the detail pane's markdown wraps over lines (link, `<u>`, bold), at a
hyphen or a space, ends its style at each line's end and reopens it on the next, so no underline
bleeds past it. The status bar's lock badge fills ` lock ` with a space each side, and no colon
follows the word.
**Date:** 2026-10-16
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 34085bb

**Regression check (2026-10-16, 34085bb):**
- 1: guard folded
- 2: guard folded

**Sources:**
- `.capcom/issues/capcom-ui/ui-link-underline-bleed.md` (authority for item 1)
- `.capcom/issues/capcom-ui/ui-lock-badge-padding.md` (authority for item 2)
- `docs/design/tui.md` (status bar), `docs/plans/archived/2026-10-16 - 04 - ui-lock-badge-plan.md`

**Ratified design calls** (user, 2026-10-16):
- **No-badge padding:** the lock segment always opens with the padded word ` lock ` and one plain space, badge or not, so ` lock  invalid` lines up with a painted ` lock  free`.
- **Docs:** item 2 updates the status-bar examples in `docs/site/getting-started/terminal-ui.md` and `docs/design/tui.md`.

**Standing requirements:**
- skills: coding-standards
- Tests per `AGENTS.md`: single packages with `GOMEMLIMIT=2GiB`, never `./...` with `-race`, never two test commands at once.
- A bite check is reverted before the commit; never committed.
- Any authorized deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- The write prompt `lock: %s, write anyway? (y/n)` (`lockReadFormat` in `internal/ui/write.go`): a question, not the badge.
- `capcom lock` CLI output (`lock: free` in its results).
- Changing x/ansi or lipgloss; the fix lives in this repo's wrap.

## 1. Close a style at each wrap break and reopen it on the next line

**Closes:** ui-link-underline-bleed

**What:**
**Goal:** every line `wrapText` (`internal/ui/markdown/wrap.go`) returns ends with no SGR style in
effect, and a style in effect at a break is re-emitted at the start of the next line, whether the
break fell at a hyphen, a space or a hard break. In `capcom ui`, a link wrapped at a hyphen paints
no underline past its visible text and none on the next screen row's list pane.

**Approach (assumed at the header base):** fix of a defect: `Measure.Wrap` (x/ansi v0.11.7 `wrap`)
passes SGR escapes through as word bytes and never closes them at a break; at a hyphen the reset
following the hyphen is carried onto the next line, so the first line ends with the underline
open. The package doc in `markdown.go` and the `wrapTableCell` comment already claim the wrap
carries an SGR run across a break; this item makes that true. Add one pass in `wrap.go`, run by
`wrapText` on its final lines beside `balanceLinkMarkers` (same shape: rewrites lines in place):
it scans each line's CSI `m` sequences, drops the carried set on a reset (`\x1b[m`, `\x1b[0m`)
and appends every other sequence; a line ending with a non-empty set gets a reset appended, and
the next line is prefixed with the carried sequences. Link markers (APC) are not SGR and are left
alone; link regions are measured on stripped text and stay unchanged. Extends `wrap.go`; no new
file. Every caller of `wrapText` (`hangingPrefixes`, `wrapTableCell`, `renderHeadingLine`,
paragraph wrap in `markdown.go`) gets the balanced lines without change.

**Regression guard.** A view row cannot carry the detail test: `barredRows` appends a
`scrollbarCells` thumb or track cell ending `\x1b[m` (`internal/ui/scrollbar.go`), so every row
ends closed even without the fix. Keep the fixture body within the pane height (blank scrollbar
cell) or assert on `m.detail` lines, and name the test `TestDetailPane_…` so `-run 'Detail'` runs it.

**Files:** internal/ui/markdown/wrap.go, internal/ui/markdown/wrap_test.go, internal/ui/detail_test.go
**Read first:** internal/ui/markdown/wrap.go — wrapText; internal/ui/markdown/links.go — balanceLinkMarkers; internal/ui/markdown/table.go — wrapTableCell; internal/ui/markdown/wrap_test.go — paintMethods
internal/ui/markdown/markdown_test.go — testTheme, underlineSGR; internal/ui/scrollbar.go — scrollbarCells
internal/ui/detail_test.go — TestDetailPane_IssueWithPlanAndRelated_ShowsBothLines

**Tests:**
- `wrap_test.go`: a Link-styled `agents-md-no-block`, a `<u>` span and a bold span, each wrapped at
  a hyphen and at a space, under both measures; assert no returned line ends with a style in
  effect and the continuation line re-opens it; an unstyled text wraps byte-identical to before.
- `detail_test.go`: a `TestDetailPane_…` test with a detail body holding
  `[agents-md-no-block](<existing id>)` at a width that breaks it at the hyphen, the body within the
  pane height (blank scrollbar cell) or the assertion on `m.detail` lines; assert no row ends with
  an SGR style in effect.
- Bite check: both new tests fail with the new pass removed from `wrapText`.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/markdown/`
- `GOMEMLIMIT=2GiB go test ./internal/ui/ -run 'Detail'`
- `go vet ./internal/ui/...`

**Commit:** `fix(ui): close a style at each wrap break and reopen it on the next line`

## 2. Pad the lock badge and drop the colon after it

**Closes:** ui-lock-badge-padding

**What:**
**Goal:** the status bar's lock segment reads ` lock ` + one plain space + its state: ` lock  free`,
` lock  held by <holder>: <purpose> (heartbeat <age> ago)`, ` lock  stale: <purpose> (…)`,
` lock  invalid`, ` lock  unreadable`, ` lock  reading`. With a badge (held, stale, free) the fill,
or the Reverse fallback without colour, covers exactly ` lock `; the space and text after it are
unstyled. Without a badge the same text shows plain. No state has a colon directly after `lock`.
The row is still cut before the hints by visible width.

**Approach (assumed at the header base):** in `internal/ui/statusbar.go`, the badge word constant
`lockWord` becomes the padded ` lock ` and `lockPrefix` becomes `lockWord + " "`; `lockText`,
`lockReadingText` and `paintedLock` (which paints `lockWord` and appends the rest) follow from
them. Update the `lockText` doc comment's spellings. Rewrite `statusbar_test.go` expectations
(`assertBadge`, `assertPlainLock` callers, the cut-before-hints test's `lock: held by` prefix) to
the new text, asserting the fill spans the padded word. Regenerate goldens with
`go test ./internal/ui/ -run Golden -update` and check each diff touches only the lock segment.
Update the status-bar example line and the status bar bullet in
`docs/site/getting-started/terminal-ui.md` and `docs/design/tui.md` to the new spelling.

**Regression guard.** Tests outside `statusbar_test.go` pin `lock: `: `layout_test.go`
(`TestModel_StatusBar_ShowsTheTreeLock`, `TestLockText_OverviewError_IsUnreadable`, the user-config
status tests) and `refresh_test.go` (`TestRefresh_LockAcquiredOnDisk_ShowsAfterOneTick`). Rewrite
them to ` lock  free` / ` lock  held by` / ` lock  unreadable` etc.; `statusRow` strips ANSI, so the row starts with a space.

**Files:** internal/ui/statusbar.go, internal/ui/statusbar_test.go, internal/ui/layout_test.go, internal/ui/refresh_test.go, internal/ui/testdata/golden/*.golden, docs/site/getting-started/terminal-ui.md, docs/design/tui.md
**Read first:** internal/ui/statusbar.go — lockWord, lockText, paintedLock; internal/ui/statusbar_test.go — assertBadge; internal/ui/theme_test.go — sgrBefore
internal/ui/layout_test.go — TestModel_StatusBar_ShowsTheTreeLock; internal/ui/refresh_test.go — TestRefresh_LockAcquiredOnDisk_ShowsAfterOneTick
internal/ui/harness_test.go — statusRow

**Tests:**
- `statusbar_test.go`: held, stale, free each paint ` lock ` in the badge with the text after it
  plain from one space on; invalid and reading show ` lock  invalid` / ` lock  reading` plain; no
  colour paints ` lock ` in Reverse; no state's text holds `lock:`.
- `layout_test.go`, `refresh_test.go`: every `lock: ` expectation rewritten to the padded
  ` lock  <state>` spelling, the status row starting with a space.
- Golden files regenerated; `git diff --stat internal/ui/testdata/golden` shows only lock rows.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/ -run 'Status|Lock|Golden'`
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
- `grep -rn 'lock: free\|lock: held\|lock: stale' docs/site/getting-started/terminal-ui.md docs/design/tui.md` prints nothing

**Commit:** `feat(ui): pad the lock badge and drop the colon after it`
