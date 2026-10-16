# Colour the word lock in the status bar by the Tree lock state

**Goal:** the word `lock` that opens the `capcom ui` status bar is a badge filled in the new
`lock-held` role while another session holds the Tree lock (live or stale) and in `lock-free`
while it is free; every other state, and the rest of the segment, stays plain.
**Date:** 2026-10-16
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 7d7c01a
**Closes:** ui-lock-badge

**Regression check (2026-10-16, 7d7c01a):**
- 2: guard folded

**Sources:**
- `.capcom/issues/capcom-ui/ui-lock-badge.md` (the authority for every behaviour below)
- `docs/design/tui.md` (status bar, theme), ADR 0013 (the UI holds no token)
- `internal/ui/schemes/dark.yaml`, `internal/ui/schemes/light.yaml` (apogee colours)

**Ratified design calls** (user, 2026-10-16):
- **Badge foreground:** picked by the fill's relative luminance: black (`#000000`) on a light fill, white (`#ffffff`) on a dark one, so overrides stay readable.
- **Padding:** none; only the four letters `lock` are filled, the row's visible text is unchanged.

**Standing requirements:**
- skills: coding-standards
- Tests per `AGENTS.md`: single packages with `GOMEMLIMIT=2GiB`, never `./...` with `-race`, never two test commands at once.
- A bite check is reverted before the commit; never committed.
- Any authorized deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- The lock prompt before writes, `lockText`'s wording, and any other status bar segment.
- Taking, renewing or freeing the Tree lock from the UI.

## 1. Add the lock-held and lock-free colour roles — ✅ DONE (2026-10-16)

NOTES (2026-10-16): the role lists in docs/design/tui.md and docs/site/getting-started/terminal-ui.md do not name the new roles yet; both files are in item 2's Files, so left to it.

**What:**
**Goal:** `lock-held` and `lock-free` are roles of `userconfig.Roles`, named in the user config
template and coloured in both scheme files: dark `lock-held: "#f0883e"`, `lock-free: "#43d389"`;
light `lock-held: "#bc4c00"`, `lock-free: "#115e59"`; a `ui.colors` override of either loads
without a warning.
**Approach (assumed at the header base):** append `"lock-held"`, `"lock-free"` to `Roles` in
`internal/userconfig/userconfig.go` after the status roles; extend the template's `# Roles:` comment
in `internal/userconfig/template.go` (keep each comment line ≤ 100 columns) and mirror that template
text in the copy in `docs/site/reference/file-format.md`, plus its **ui.colors** role list (one
sentence: `lock-held` and `lock-free`, the status bar's lock badge). In each scheme file add the two
keys after `status-dropped`, each with an apogee-role comment (`apogee mode-auto` / `apogee success`)
saying what it paints. Add both roles for both themes to `TestTheme_Presets_DrawInApogeesColours`.
**Files:** `internal/userconfig/userconfig.go`, `internal/userconfig/template.go`, `internal/ui/schemes/dark.yaml`, `internal/ui/schemes/light.yaml`, `internal/ui/theme_test.go`, `docs/site/reference/file-format.md`
**Read first:** internal/userconfig/userconfig.go — Roles; internal/userconfig/template.go — template; internal/userconfig/userconfig_test.go — TestLoad_CreatedTemplate_NamesEveryRoleAndUncommentsCleanly, uncomment;
internal/ui/theme_test.go — TestTheme_Presets_DrawInApogeesColours, TestTheme_SchemeFiles_ColourEveryRoleInHex, TestTheme_Presets_ColourEveryRole; internal/ui/schemes/dark.yaml, internal/ui/schemes/light.yaml — status-dropped;
internal/ui/theme.go — newPalette, presets; docs/site/reference/file-format.md — template copy, ui.colors
**Tests:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/userconfig/` then `GOMEMLIMIT=2GiB go test -count=1 -run TestTheme ./internal/ui/`
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/userconfig/` passes (`TestLoad_CreatedTemplate_NamesEveryRoleAndUncommentsCleanly` included).
- `GOMEMLIMIT=2GiB go test -count=1 -run TestTheme ./internal/ui/` passes (`TestTheme_SchemeFiles_ColourEveryRoleInHex`, `TestTheme_Presets_ColourEveryRole`).
- `go vet ./internal/userconfig/ ./internal/ui/` is clean.
**Commit:** `feat(ui): add the lock-held and lock-free colour roles`

## 2. Paint the status bar's lock badge — ✅ DONE (2026-10-16)

NOTES (2026-10-16): the badge's black-or-white text switches at relative luminance 0.179 (`lightFillLuminance` in internal/ui/theme.go), the point where both contrast equally by the WCAG ratio; dark's presets get black text, light's white.

Depends on item 1.

**What:**
**Goal:** with the lock held or stale the status bar's leading `lock` carries a `lock-held`
background fill, with it free a `lock-free` fill, its foreground black or white by the fill's
luminance; the text after the word is unstyled. Invalid, unreadable and the `reading` placeholder
leave the row unstyled. Without a colour for the role (NO_COLOR, or a palette lacking it) a held or
stale badge is Reverse and a free one plain. The row's visible text and width are unchanged.
**Approach (assumed at the header base):** the badge state travels beside the text, never parsed
out of it: a small `lockBadge` kind (none, held, free; zero value none) computed by one function
from the same `(lock.Overview, error)` `lockText` takes — producers `readLock`
(`internal/ui/statusbar.go`, `lockMsg` gains the kind) and `session.poll` (`internal/ui/refresh.go`,
`refreshedMsg` gains it); consumers `model.Update`'s `lockMsg` case and `applyRefreshed`, which store
it on a new `model` field beside `lockText` (`internal/ui/model.go`; the initial value is none).
`theme` (`internal/ui/theme.go`) gains the two badge styles, built in `newTheme` by one helper
taking the role and the no-colour fallback (Reverse for held, plain for free), plus a luminance
helper returning the readable foreground. `statusBar` renders the badge style over `lock` and
appends the rest of `m.lockText` unchanged; `fit`/`ansi.Truncate` keeps measuring visible width.
Docs: `docs/design/tui.md` (status bar bullet, role list) and
`docs/site/getting-started/terminal-ui.md` (status bar, `colors` role list) describe the badge.
**Regression guard.** Both scheme files' headers (`internal/ui/schemes/dark.yaml:3-4`,
`light.yaml:3-4`) say the UI fills only the selected row's background; once the badge lands that is
false. Every comment or doc line saying the selection is the only background fill is amended to
name the lock badge too — find them with `grep -rn "background alone" internal/ docs/`.
**Files:** `internal/ui/statusbar.go`, `internal/ui/refresh.go`, `internal/ui/model.go`, `internal/ui/theme.go`, `internal/ui/statusbar_test.go` (new), `internal/ui/schemes/dark.yaml`, `internal/ui/schemes/light.yaml`, `internal/ui/testdata/golden/theme-dark-120x30.golden`, `internal/ui/testdata/golden/theme-light-120x30.golden`, `internal/ui/testdata/golden/drag-divider-120x30.golden`, `docs/design/tui.md`, `docs/site/getting-started/terminal-ui.md`
**Read first:** internal/ui/statusbar.go — statusBar, lockText, readLock, lockMsg, lockReadingText; internal/ui/refresh.go — session.poll, refreshedMsg, applyRefreshed; internal/ui/model.go — newModel, model.Update (lockMsg case);
internal/ui/theme.go — theme, newTheme, selectionStyles, applyTheme; internal/ui/layout.go — fit; internal/ui/layout_test.go — TestModel_StatusBar_ShowsTheTreeLock, acquireTestLock, ageTestLock, writeTestLockRecord;
internal/ui/theme_test.go — newThemeModel (tweak gets *Options before start: arrange the lock via &testStore{store: o.Store}), sgrBefore, colourSequences, TestTheme_NoColor_DrawsNoColourSequences; internal/ui/harness_test.go — statusRow, startModel, goldenANSI
**Tests:** new `internal/ui/statusbar_test.go`, over `newThemeModel` and the lock helpers of
`layout_test.go` (`acquireTestLock`, `ageTestLock`, `writeTestLockRecord`): held, stale and free
under theme dark assert the SGR before `lock` carries the role's `48;2;…` fill and a contrasting
`38;2;…` and the text after `lock` carries no colour; a `ui.colors.lock-held`/`lock-free` override
changes the fill; invalid and the `reading` placeholder carry no colour sequence; NO_COLOR held is
Reverse (`7`), free plain; a narrow width still cuts before the hints with them intact. Golden
updates via `-update`, reviewed: only the status row's `lock` changes.
After the edit, `grep -rn "background alone" internal/ docs/` finds no line that omits the badge.
`GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/`
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/` passes, `TestModel_StatusBar_ShowsTheTreeLock` unchanged.
- `go vet ./internal/ui/` is clean.
- Bite check: `statusBar` without the badge fails the new held and free tests; reverted.
**Commit:** `feat(ui): colour the status bar's lock badge by the Tree lock state`
