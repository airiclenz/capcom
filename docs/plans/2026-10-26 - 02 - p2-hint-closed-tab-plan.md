# Plan: missing body-file hint and the UI Closed tab

**Goal:** Close the ready P2 issues. A missing `--body-file` / `--body-append-file` hints a stdin retry
instead of `capcom search`, and `capcom ui` gains a fourth tab, Closed, for browsing finished issues.

**Date:** 2026-10-26
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 1e5e65a
**Closes:** ui-closed-tab

**Sources:**
- `.capcom/issues/agent-ergonomics/body-file-hint.md`
- `.capcom/issues/agent-ergonomics/error-next-command.md` (its Log: only the body-file hint is left)
- `.capcom/issues/human-ergonomics/ui-closed-tab.md`
- ADR 0015 (`capcom know show adr-0015-decision`), ADR 0012, Note `error-messages`, Note `cli-test-harness`
- `docs/design/tui.md`, `docs/site/getting-started/terminal-ui.md`

**Ratified design calls** (user, 2026-10-26):
- **Missing input file hint:** `capcom <command> --<flag> -`, e.g. `hint: capcom create --body-file -`.
- **`L` on Closed:** no-op, like `a`, `c` and `s`; `R` is the one write on the tab.
- **Open `status:` on Closed:** a problem row, as `status:` on Knowledge, not an empty match.
- **Persistence and CLI:** ui-state.yaml stores `tab: "closed"` and a `closed` selection;
  `capcom ui closed [id]` opens the tab, with completion; ADR 0015 gets a dated amendment.
- **Flat-list code** (writer): Ready and Closed share one flat entry-list type; no copy of `readyTab`.
- **Row date** (writer): the Closed time as `2006-01-02` in UTC; ties in Closed time sort by id.

**Standing requirements:**
- skills: coding-standards
- Tests on the Pi host: one package at a time, `GOMEMLIMIT=2GiB`, never `-race` on `./...`.
- CLI tests go through `runCLI` (Note `cli-test-harness`); UI goldens regenerate with `-update`,
  and a regenerated golden may differ only where the item says it does.

**Regression check (2026-10-26, 1e5e65a):**
- 1: guard folded (`--kv` asserted on the `create` and `update` rows only)
- 3: guard folded (`doc.go` file map; Ready's names and signatures kept)
- 4: guard folded with the writer's decision (not-on-Closed notice, width-80 tab bar, existing tab tests)
- 5: guard folded (help stays 20 rows; per-key no-op checks; stale tab comments)
- 6: guard folded with the writer's decision (clidoc check; completion test extended)
- 7: guard folded with the writer's decision (`go run ./cmd/capcom`; widened grep; ADR Status line)

**Out of scope:**
- Other error hints; the P3 issues (`batch-apply`, `tree-status-filter`, `html-export`, `shared-ref-repository`).
- Any change to the Issues tab's `/status:closed` behaviour.
- CHANGELOG release headings, VERSION, tags.

## 1. A missing body input file hints a stdin retry — ✅ DONE (2026-10-26)

**What:**
**Goal:** a `--body-file` or `--body-append-file` naming a file that does not exist fails with code
`not_found` and the hint `capcom <cmd> --<flag> -` (the failing command's path and the flag the
caller used), in text and `--json` on `create`, `update`, `know add` and `know update`, and in
`--kv` on `create` and `update`.
Fix: these failures took `not_found`'s default hint `capcom search <text>`, which does not fit a
missing input file (deferred from plan `2026-10-26 - 01 - p1-p2-ergonomics-plan`, item 7).
**Approach (assumed at the header base):** `readTextInput` in `internal/cli/node_edit.go` takes the
flag name and, on `os.ErrNotExist`, sets `Hint: "capcom <command> --" + flag + " -"`; the
`capcom <command>` placeholder is filled by `WithCommand` in `root.go`. Every caller passes its flag
constant (`flagBodyFile`, `flagBodyAppendFile`). Other read errors keep their hints.
**Regression guard.** `know add` and `know update` register no `--kv` (only `create` and `update`
take it, through `withKV`; elsewhere cobra fails `unknown flag: --kv`, pinned by
`TestList_KV_FailsAsUnknownFlag`) and stay without it: assert the `--kv` `HINT=` pair only on the
`create` and `update` rows, text and `--json` on all five.
**Files:** internal/cli/node_edit.go, internal/cli/create.go, internal/cli/update.go, internal/cli/know_add.go, internal/cli/know_update.go, internal/cli/body_file_test.go
**Read first:** internal/cli/node_edit.go — readTextInput, stdinPath; internal/cli/output/errors.go — Error.Hint, hintText, WithCommand, defaultHints; internal/cli/root.go — Run (InCommand), withKV, scanFormat; internal/cli/create.go — newIssueBody; internal/cli/update.go — newUpdateCommand
internal/cli/know_update.go — readNoteInputs; internal/cli/know_add.go — newNote; internal/cli/cli_test.go — lastLine, decodeEnvelope, TestRun_Failure_EndsWithItsHintInEveryFormat
**Tests:** new `internal/cli/body_file_test.go`: table over `create --body-file`, `update --body-append-file`,
`know add --body-file`, `know update --body-file`, `know update --body-append-file`, each naming a
missing path; assert the text output's last line is `hint: capcom <cmd> --<flag> -` with the real
command path, the `--json` error envelope's hint field, and, on the `create` and `update` rows
only, the `--kv` hint pair.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'BodyFile|KnowUpdate|Know'`
- `go vet ./internal/cli/`
**Closes:** body-file-hint; error-next-command
**Commit:** `fix(cli): hint a stdin retry when a body input file is missing`

## 2. The worktree state holds a Closed tab selection

**What:**
**Goal:** `userconfig.WorktreeState` carries a `Closed` selection saved and loaded under
`ui.selected.closed`, and `tab: "closed"` round-trips like the other tab names.
**Approach (assumed at the header base):** in `internal/userconfig/worktree_state.go` add the `Closed`
field and `keyClosed = "closed"`, and list it in `selections()`; save and load already walk that list.
Unknown keys stay ignored on read, so no other change.
**Files:** internal/userconfig/worktree_state.go, internal/userconfig/worktree_state_test.go
**Read first:** internal/userconfig/worktree_state.go — WorktreeState, selections, encodeWorktreeState, selected; internal/userconfig/worktree_state_test.go — TestSaveWorktreeState_LoadWorktreeState_RoundTrips ("every field" pins the file content), TestLoadWorktreeState_UnknownKeys_IgnoredAndTheRestKept
internal/ui/uistate.go — worktreeSnapshot, trackedSelection; internal/ui/model.go — newModel (storedIDs from WorktreeState)
**Tests:** extend `TestSaveWorktreeState_LoadWorktreeState_RoundTrips` with a `Closed` id and `Tab: "closed"`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/userconfig/`
**Commit:** `feat(userconfig): store the Closed tab's selection in ui-state.yaml`

## 3. Ready's flat list becomes a shared entry list

**What:**
**Goal:** one type in `internal/ui/entrylist.go` owns a flat issue list's state (all and matched
entries, cursor, scroll top, read flag, error) and its behaviour shared by any flat tab: narrowing by
a filter, keeping the selection across a reload, seating a wanted id, cursor moves, click and wheel.
The Ready tab is built on it. Behaviour is unchanged: every existing test and golden passes untouched.
**Approach (assumed at the header base):** extract from `readyTab` (`ready.go`), `applyReady`,
`keepReadySelection` (`refresh.go`), `refilterReady` (`filter.go`), the Ready cases of `tabCount`
(`layout.go`) and of `clickSidebar` / `wheelSidebar` (`mouse.go`). Per-tab parts stay per tab: the
loader, the row painter and the no-match text. Pure refactor; no new behaviour.
**Regression guard.** `doc.go`'s file map gains an `entrylist.go` line (`TestDocFileMap_MatchesDirectory`
fails without it; reword the `ready.go` line if its responsibility moves). The tests' names stay
reachable: `m.ready.entries` / `.all` as `[]index.Entry` (embed the entry list in `readyTab` or keep
the field names), and `readySelection`, `readyMsg{err:}` and `loadReady(s, tabRead)` keep their names and signatures.
**Files:** internal/ui/entrylist.go, internal/ui/ready.go, internal/ui/refresh.go, internal/ui/filter.go, internal/ui/layout.go, internal/ui/mouse.go, internal/ui/model.go, internal/ui/doc.go
**Read first:** internal/ui/ready.go — readyTab, applyReady, readySelection, paintReadySidebar; internal/ui/refresh.go — keepReadySelection, replacementID, withShownReloaded; internal/ui/filter.go — refilterReady, emptySidebarText
internal/ui/layout.go — tabCount; internal/ui/mouse.go — clickSidebar, wheelSidebar; internal/ui/listcursor.go — listCursor; internal/ui/doc_test.go — TestDocFileMap_MatchesDirectory
**Tests:** no new behaviour; existing `internal/ui` tests and goldens are the guard.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/` (no `-update`; `git status internal/ui/testdata` clean)
- `go vet ./internal/ui/`
**Commit:** `refactor(ui): share the Ready tab's flat list as an entry list`

## 4. The Closed tab lists the closed issues

**What:** Depends on items 2 and 3.
**Goal:** `ui.TabClosed` exists, named `closed`, labelled `Closed`, last in the tab bar after Knowledge
on key `4`; `1` to `3` are unchanged and Tab cycles all four. Its sidebar lists exactly the issues in a
closed category, newest Closed time first, rows `<closed date>  <id>  <title>` (date `2006-01-02` UTC,
ids padded to one column). The label shows `Closed [n]` (or `[matched/total]` while filtered). The
detail pane shows the selected issue in full. A click on the label opens the tab. The tab and its
selection are saved to and restored from the worktree state file. Every write reloads it.
**Approach (assumed at the header base):** `ui.go`: append `TabClosed` to the `Tab` consts, `tabOrder`,
`tabNames`, `tabLabels`. New `closed.go`: `closedTab` on item 3's entry list, `loadClosed` via
`issueIndex.List(index.Filter{Categories: config.ClosedCategories(), IsClosedIncluded: true})` sorted
by `Entry.Closed` desc then id; `applyClosed`, `paintClosedSidebar`, `closedRow`. Wire the Closed case
into `model.go` (field, `storedIDs`, `Init`, `showSelection`, `paintSidebar`, `sidebarSelection`),
`refresh.go` (`reloadTabs`, `withShownReloaded`), `layout.go` (`tabCount`), `filter.go`
(`noMatchTexts`; a missing entry panics), `uistate.go` (`worktreeSnapshot`, `trackedSelection`,
`markTabRead`), `doc.go`. Every golden's tab bar gains the Closed label; nothing else in it may change.
**Regression guard.** when the command line names an id the Closed list does not hold (`capcom ui closed <open-id>`), applyClosed sets the notice `<id> is not on the Closed tab` (a `notOnClosedFormat` constant beside ready.go's `notInReadyFormat`, same mechanism) and selects the first row; a closed_test.go case pins it. The tab bar dropping the path/version label when the four labels leave less than versionLabelMinGap (layout.go) is the existing narrow-bar fallback and is accepted; add a closed_test.go case at width 80 asserting all four tab labels render in full.
Existing tab tests follow the fourth tab: in `layout_test.go` the "tab wraps" case takes three Tabs
(or wants `TabClosed`) and `TestModel_TabBar_CountsFollowCloseAndCreate`'s wants gain ` Closed [n] `
(n+1 after the close); in `issues_test.go` the `2`, Tab, Tab round trip takes one more Tab.
**Files:** internal/ui/ui.go, internal/ui/closed.go, internal/ui/closed_test.go, internal/ui/model.go, internal/ui/refresh.go, internal/ui/layout.go, internal/ui/filter.go, internal/ui/uistate.go, internal/ui/doc.go, internal/ui/layout_test.go, internal/ui/issues_test.go, internal/ui/testdata/golden/
**Read first:** internal/ui/ui.go — tabOrder, tabNames, tabLabels, ParseTab; internal/ui/model.go — newModel storedIDs, Init, showSelection, paintSidebar, sidebarSelection; internal/ui/uistate.go — worktreeSnapshot, trackedSelection, markTabRead
internal/ui/refresh.go — reloadTabs, withShownReloaded; internal/ui/layout.go — tabCount, tabBar, tabAt; internal/ui/filter.go — noMatchTexts; internal/index/query.go — Filter.sql, List; internal/ui/mouse_test.go — tabColumn, click
**Tests:** `closed_test.go`: list holds only closed-category issues in Closed-time order with the row
shape; `4`, Tab cycling and a label click open it; the label count; tab and selection restored from
ui-state.yaml; the detail pane shows the selected closed issue's body; closing an issue on Ready
(`c` + reason) puts it on Closed and bumps the label count; `ui closed <open-id>` sets the
not-on-Closed notice and selects the first row; at width 80 all four tab labels render in full.
`layout_test.go` and `issues_test.go` updated as the guard says.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/ -update` once, then `GOMEMLIMIT=2GiB go test ./internal/ui/`
- `git diff internal/ui/testdata/golden` shows only tab-bar lines changed
- `go vet ./internal/ui/`
**Commit:** `feat(ui): add a Closed tab listing the closed issues`

## 5. Keys, reopen and filtering on the Closed tab

**What:** Depends on item 4.
**Goal:** on the Closed tab, cursor keys, mouse click and wheel move the selection; `R` reopens the
selected issue through the reason prompt, after which it leaves Closed, shows on Issues (and Ready
when ready) and every count updates; `/` filters by words, `label:` and `status:`, where a `status:`
naming a non-closed status is a problem row (`status: on the Closed tab takes a closed status`);
`d` opens the dependency view; `e` opens the editor on the issue's file; `a`, `c`, `s` and `L` do
nothing. The help screen lists the four tabs and `1 2 3 4`.
**Approach (assumed at the header base):** `keys.go`: Closed case in `sidebarKey`; gate `a`, `c`, `s`,
`L` in `handleKey` when `m.tab == TabClosed`; `R` keeps `openTextPrompt(reopenWrite())`. `mouse.go`:
Closed in `clickSidebar` and `wheelSidebar` through the entry list. `editor.go`: Closed in
`selectedPath`. `filter.go`: Closed in `withFilter`, and a Closed branch in `matchFilter` raising the
problem row, as `statusOnNotesText` does for Knowledge. `help.go`: the tabs row in `helpLines`
becomes `  tabs     Tab  1 Issues  2 Ready  3 Knowledge  4 Closed`; no row is added.
**Regression guard.** `helpLines` stays at most 20 rows (the 80x24 body is 20 rows;
`TestModel_View_HelpScreen_FitsTheSmallTerminal`): the tab names go on the existing tabs row, under 80
columns. Update every `internal/ui` doc comment naming the tabs a key acts on (`keys.go` handleKey,
`depview.go` :131 and :184-185, …), found with `grep -nE "Ready (and|or) Issues|on the Knowledge tab" internal/ui/*.go`.
**Files:** internal/ui/keys.go, internal/ui/mouse.go, internal/ui/editor.go, internal/ui/filter.go, internal/ui/help.go, internal/ui/depview.go, internal/ui/closed_test.go, internal/ui/testdata/golden/help-120x30.golden, and any other `internal/ui` file the comment grep names
**Read first:** internal/ui/keys.go — handleKey, sidebarKey; internal/ui/filter.go — matchFilter, matchIssues, matchNotes, withFilter, refilterReady; internal/ui/mouse.go — clickSidebar, wheelSidebar; internal/ui/editor.go — selectedPath
internal/ui/depview.go — depViewKey, openDeps, revealIssue; internal/ui/help.go — helpLines; internal/ui/layout_test.go — TestModel_View_HelpScreen_FitsTheSmallTerminal; internal/ui/write_close_test.go — submitPrompt, newClosedSelectedModel
**Tests:** in `closed_test.go`: `R` with a reason reopens and the issue moves to Issues and Ready with
counts updated; after each of `a`/`c`/`s`/`L` no prompt or picker opens (`m.textPrompt.isOpen` and
`m.picker.isOpen` false), for `a` the issue file's bytes are unchanged, and the store is unchanged; word, `label:` and closed `status:` filters
narrow the list; an open `status:` shows the problem row; `d` opens the dependency view; `e` names the
issue's path. Help golden regenerated.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
- `go vet ./internal/ui/`
**Commit:** `feat(ui): reopen, filter and browse on the Closed tab`

## 6. `capcom ui closed [id]` opens the Closed tab

**What:** Depends on item 4.
**Goal:** `capcom ui closed` and `capcom ui closed <id>` are accepted, shell completion offers `closed`
as a tab, and the command's usage, long help and the generated reference page name the four tabs.
**Approach (assumed at the header base):** `internal/cli/ui.go`: `Use` becomes
`ui [issues|ready|knowledge|closed] [id]`, `uiLong` and `uiExample` name Closed; `internal/cli/complete.go`:
add `ui.TabClosed` to `uiTabs`; `ParseTab` already walks `tabOrder`. Regenerate the reference with
`go run ./cmd/gendocs`.
**Regression guard.** add `GOMEMLIMIT=2GiB go test ./internal/clidoc/` to Acceptance (TestReferenceUpToDate fails on any Use/help change without the regenerated docs/site/reference/cli page).
`TestComplete_UITab_ListsTabNamesInTabBarOrder` pins the tab list exactly: extend its want to
`issues`, `ready`, `knowledge`, `closed` (tab-bar order) rather than adding a separate test.
**Files:** internal/cli/ui.go, internal/cli/complete.go, internal/cli/ui_test.go, internal/cli/complete_test.go, docs/site/reference/cli/capcom_ui.md
**Read first:** internal/cli/ui.go — newUICommand, uiLong, uiExample, uiTarget; internal/cli/complete.go — uiTabs, completeUITarget, issueCandidates; internal/ui/ui.go — ParseTab, tabOrder
internal/cli/ui_test.go — TestUITarget_ValidArguments_ReportsTabIDAndWhetherATabWasGiven, TestUI_MalformedInvocation_ExitsUsage; internal/cli/complete_test.go — TestComplete_UITab_ListsTabNamesInTabBarOrder; internal/clidoc/reference_test.go — TestReferenceUpToDate; cmd/gendocs/main.go — defaultOutDir
**Tests:** a `closed` row in the `uiTarget` table of `ui_test.go`; `TestComplete_UITab_ListsTabNamesInTabBarOrder`'s
want extended with `closed`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'UI|Complete'`
- `GOMEMLIMIT=2GiB go test ./internal/clidoc/`
- `go run ./cmd/gendocs && git diff --exit-code docs/site/reference/cli/` after the commit
**Commit:** `feat(cli): open the Closed tab with capcom ui closed`

## 7. Docs and ADR 0015 name the Closed tab

**What:** Depends on items 5 and 6.
**Goal:** every doc that enumerates the UI's tabs, their keys or the ui-state tab names includes Closed,
and ADR 0015 carries a dated amendment: `closed` is a fourth tab name in `ui.tab` and `ui.selected`.
**Approach (assumed at the header base):** the rule: every passage listing the tabs, `1 2 3`, or
`ready`, `issues` or `knowledge` as tab names, or naming which tabs a key or feature works on (`grep -rnE "Knowledge tab|1 2 3|three tabs|Three tabs|\`knowledge\`|Ready (and|or) Issues|Issues, Ready|.1., .2. and .3" README.md docs/ --include=*.md`,
excluding `docs/plans/` and `docs/handoffs/`). Known: `README.md`, `docs/site/getting-started/terminal-ui.md`
(tab bar, "The tabs", Keys block, filter text, Writes preamble), `docs/design/tui.md`,
`docs/site/reference/file-format.md` (ui-state example and tab-name list). Add the amendment with
`go run ./cmd/capcom know add --parent adr-0015 --id adr-0015-amend-1026 ...` in the shape of `adr-0002-amend-1012`,
and list it in the reading order of `adr-0015`'s body; run `go run ./cmd/capcom check`.
**Regression guard.** every capcom command in the item runs as `go run ./cmd/capcom ...` (the capcom on PATH is older than the base), including the `know add` of the amendment.
The passages naming which tabs a key works on count too (`docs/design/tui.md` :126 and :167,
`terminal-ui.md` :168 and :193, the `` `1`, `2` and `3` `` keys line); a hit that rightly leaves Closed
out (`terminal-ui.md` :131, the Status and priority columns Closed rows lack) stays as is.
`adr-0015`'s `Status: Accepted.` becomes `Status: Accepted; amended 2026-10-26.`, as `adr-0002.md` reads.
**Files:** README.md, docs/site/getting-started/terminal-ui.md, docs/design/tui.md, docs/site/reference/file-format.md, .capcom/knowledge/decisions/adr-0015.md, .capcom/knowledge/decisions/adr-0015/adr-0015-amend-1026.md
**Read first:** docs/site/getting-started/terminal-ui.md — Tab bar, Keys, Dependencies, Writes; docs/design/tui.md — Tab bar, Dependency view, Keys, writes; docs/site/reference/file-format.md — ui.tab, ui.selected; README.md — Three tabs
.capcom/knowledge/decisions/adr-0015.md — reading order, Status; .capcom/knowledge/decisions/adr-0002.md, adr-0002/adr-0002-amend-1012.md — amendment shape; internal/nodestore/kind.go — WithSync
**Tests:** docs only.
**Acceptance:**
- `go run ./cmd/capcom check` exits 0
- `go run ./cmd/capcom know show adr-0015-amend-1026`
- the Approach grep finds no tab list without Closed
**Commit:** `docs: name the Closed tab in the UI docs and ADR 0015`
