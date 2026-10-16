# Selection-fill tests, a file map for internal/ui, and the full detail header

**Goal:** the selection fill is covered by tests on the dependency view and the Knowledge tab, not
only on Ready and Issues; `internal/ui` carries a `doc.go` file map checked by a structural test;
the detail pane's header shows every frontmatter key of an Issue or Note.
**Date:** 2026-10-16
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 101ce08

**Regression check (2026-10-16, 101ce08):**
- 1: recast — the writer's decision applied (fix the "issues" subcase of `TestTheme_SelectedRow_FillsTheBackgroundKeepingItsColours` to press '1'; the helper takes the store files); `internal/ui/mouse_test.go` added to **Files:** for the helper's caller there
- 2: guard folded — the writer's decision applied (open the Knowledge tab with '3'; reuse item 1's helper unchanged)
- 1 (second round): recast — the helper also takes a `func(*Options)` tweak (nil for none); the test seats Tab=TabIssues, ID="alpha", presses 'd', then moves onto base; mouse_test.go's caller passes nil
- 2 (second round): recast — the Note is seated through item 1's helper (Tab=TabKnowledge, ID="relic"); "retired Notes revealed" dropped; faint parsed as its own SGR parameter with a second bite on `.Faint(true)` in `selectionStyles`; connector check on a nested non-retired Note
- 1 (third round): guard folded — no key after 'd': base (waitsOn[0]) is already the selected row once the view opens; read the row from m.detail and pin m.deps.cursor.highlight(...) == 0 before asserting
- 4: guard folded — the writer's decision applied (issueFile gives every test Issue `type: task` and a fixed `created:`, so every golden showing an Issue detail is rewritten); the "only those" qualifier dropped from **Files:**
- 5: guard folded — `docs/site/getting-started/terminal-ui.md` added to **Files:**; every prose naming the detail header's fields or its id links, in tui.md and terminal-ui.md, names the full set

**Sources:**
- `.capcom/issues/capcom-ui/ui-depview-fill-test.md`, `.capcom/issues/capcom-ui/ui-knowledge-fill-test.md`, `.capcom/issues/ui-doc-file-map.md`
- `docs/design/tui.md` (the selection and muted highlights start after the tree connectors)
- `internal/ui/theme_test.go` — `TestTheme_SelectedRow_FillsTheBackgroundKeepingItsColours` (the pattern items 1 and 2 extend)
- `internal/tree/doc.go`, `internal/tree/doc_test.go` (the file-map pattern item 3 follows)
- `.capcom/issues/capcom-ui/ui-detail-full-header.md` (items 4 and 5)

**Ratified design calls** (user, 2026-10-16):
- **Header fields:** every frontmatter key the file can hold, plus parent, each line left out when empty; `capcom show`'s derived facts (path, category, ready, etag) stay out.
- **Notes too:** the Note header adds retired, superseded-by and aliases.
- **Layout:** one `key: value` line per key in the file's key order; status, priority and type share the first line; id-valued keys are code spans (links); dates `2006-01-02 15:04` UTC; `retired` only when true.

**Standing requirements:**
- skills: coding-standards
- Tests per `AGENTS.md`: single packages with `GOMEMLIMIT=2GiB`, never `./...` with `-race`, never two test commands at once.
- A bite check (temporarily breaking the render path) is reverted before the commit; never committed.
- Any authorized deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- Any change to how the selection is rendered (`theme.markOf`, `rowMark.render`, the scheme files).
- File maps for packages other than `internal/ui`; its subpackages `internal/ui/markdown` and `internal/ui/schemes`.
- `capcom show`'s output and the index schema; the detail header reads only what the index and the file already carry.

## 1. Test the selection fill in the dependency view — ✅ DONE (2026-10-16)

NOTES (2026-10-16): newThemeModel now takes (t, files, configText, isNoColor, tweak func(*Options)); every existing caller passes themeFiles and nil. The "issues" subcase of TestTheme_SelectedRow_FillsTheBackgroundKeepingItsColours now presses '1' and asserts m.tab (TabIssues; TabReady for the "ready" subcase) before checking the row.
NOTES (2026-10-16): bite check run — depview.go's selected row painted with markOf(false, false) made TestDepView_SelectedRow_FillsTheBackgroundKeepingItsColours fail on base, open and Base; reverted (depview.go is unchanged).

**What:** Recast at the regression check (2026-10-16). a test-only item. Add a test that opens the dependency view (as
`TestDepView_IssueWithBothSides_MatchesGolden` does, over `depFiles` with `alpha` selected and `d`
pressed) under the user config `ui:\n  theme: dark\n`, moves the selection onto a dependency row
whose issue is open (`base`, status `open`, category todo), and asserts with `sgrBefore` that every
span of that row (id, status, title) carries dark's selection fill `48;2;58;95;205` while its
status also keeps dark's `status-todo` foreground `38;2;178;178;178`. The dep view's rows are
painted through `theme.markOf` in `internal/ui/depview.go`.
To open a coloured model over `depFiles`, generalise `newThemeModel` in `internal/ui/theme_test.go`
to take the store files (one helper; `themeFiles` callers pass it explicitly) — never a second
copy of its config-writing body. The new test lives in `internal/ui/depview_test.go`.
Bite check: render the selected dep row with `markOf(false, false)` and confirm the test fails, then revert.
**Regression guard.** tabOrder is Issues, Ready, Knowledge (internal/ui/ui.go tabOrder), so '1' opens Issues, '2' Ready and '3' Knowledge. The existing "issues" subcase of TestTheme_SelectedRow_FillsTheBackgroundKeepingItsColours presses '2' and so re-tests Ready; item 1 fixes it to press '1' (and confirms the subcase then renders the Issues sidebar), since item 1 already touches internal/ui/theme_test.go. The generalised newThemeModel helper takes the store files as a parameter so item 2 reuses it unchanged.
The generalised helper takes the store files and a func(*Options) tweak (nil for none), so callers can seat a tab and an id; item 1 seats Tab=TabIssues, ID="alpha" through it, presses 'd', and base is then already the selected row; mouse_test.go's caller passes nil.
No key after 'd': base is waitsOn[0] (depFiles deps [base, shipped, ghost]) and the first load puts the cursor on row 0 (applyDeps); a 'j' lands on the done shipped and the todo foreground fails. Read the row from m.detail (paintDeps sets it) and pin m.deps.cursor.highlight(len(m.deps.rows())) == 0 before asserting.
**Files:** `internal/ui/depview_test.go`, `internal/ui/theme_test.go`, `internal/ui/mouse_test.go` (its `newThemeModel` call in `TestMouse_MidDrag_MatchesANSIGolden` passes `themeFiles`)
**Read first:** `internal/ui/theme_test.go` — newThemeModel, sgrBefore, TestTheme_SelectedRow_FillsTheBackgroundKeepingItsColours; `internal/ui/mouse_test.go` — TestMouse_MidDrag_MatchesANSIGolden;
`internal/ui/depview_test.go` — depFiles; `internal/ui/depview.go` — applyDeps, paintDeps; `internal/ui/harness_test.go` — newTestOptions
**Closes:** ui-depview-fill-test
**Tests:** `GOMEMLIMIT=2GiB go test -count=1 -run 'TestDepView|TestTheme|TestMouse_MidDrag_MatchesANSIGolden' ./internal/ui/`
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/` passes.
- `go vet ./internal/ui/` is clean.
**Commit:** `test(ui): cover the selection fill in the dependency view`

## 2. Test the selection fill on the Knowledge tab

**What:** Recast at the regression check (2026-10-16). a test-only item. Depends on item 1 (it uses the generalised `newThemeModel`). Add a
test in `internal/ui/knowledge_test.go` that opens the Knowledge tab over a store holding Notes
(`knowledgeFiles`, or a smaller fixture with a nested Note and a retired one) under
`ui:\n  theme: dark\n`, and asserts:
- every span of the selected row carries the selection fill `48;2;58;95;205`;
- with the selection on a nested Note, the tree connector before it carries no `48;2;58;95;205`
  (the fill starts after the connector, per `docs/design/tui.md`);
- with the selection on a retired (muted) Note, the row is faint (SGR
  `2`) over the fill.
The Knowledge rows are painted by `treeWidget.paint` in `internal/ui/tree.go` with `noteTreeRow`
(`internal/ui/knowledge.go`) through `theme.markOf`.
Bite check: paint the selected row with `markOf(row.isMuted, false)` and confirm the test fails, then revert.
**Regression guard.** open the Knowledge tab with '3' (never copy the '2' keypress); reuse item 1's generalised newThemeModel helper without changing its signature.
Seat the Note through item 1's helper with Tab=TabKnowledge and ID="relic" (nested under the retired old: revealed, muted and selected) — drop the phrase "retired Notes revealed"; the faint assertion checks that faint (SGR param 2) is a parameter in its own right of the SGR before the row's text — parse the parameters skipping the 38/48 colour sub-parameters — never a substring "2" match; add a second bite: drop .Faint(true) from the muted selected style in internal/ui/theme.go (selectionStyles / mutedSelectedRow) and confirm the faint assertion fails, then revert. The connector check uses a nested non-retired Note (e.g. ID "setup" under guide).
**Files:** `internal/ui/knowledge_test.go`
**Read first:** `internal/ui/tree.go` — treeWidget.paint; `internal/ui/theme.go` — theme.markOf, selectionStyles; `internal/ui/knowledge.go` — noteTreeRow;
`internal/ui/knowledge_test.go` — knowledgeFiles, TestKnowledgeTab_OpenedOnAHiddenNote_RevealsAndSelectsIt; `internal/ui/theme_test.go` — newThemeModel, sgrBefore
**Closes:** ui-knowledge-fill-test
**Tests:** `GOMEMLIMIT=2GiB go test -count=1 -run 'TestKnowledge|TestTheme' ./internal/ui/`
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/` passes.
- `go vet ./internal/ui/` is clean.
**Commit:** `test(ui): cover the selection fill on the Knowledge tab`

## 3. File map in internal/ui/doc.go with a structural test

**What:**
**Goal:** `internal/ui/doc.go` holds the package's only package comment and a `# Files` section
mapping every non-test `.go` file directly in `internal/ui` (doc.go included) with a half-line
role each, in the format of `internal/tree/doc.go`; `TestDocFileMap_MatchesDirectory` in
`internal/ui/doc_test.go` fails when a file is missing from the map or the map names a file that
does not exist.
**Approach (assumed at the header base):** move the package comment from `internal/ui/ui.go` into
`doc.go` unchanged (ui.go keeps a bare `package ui`), append the file map, and copy
`internal/tree/doc_test.go` into `internal/ui/doc_test.go` with `package ui`. Each role is read
from the file's own declarations, never guessed. Subdirectories (`markdown`, `schemes`,
`testdata`) are not mapped.
**Files:** `internal/ui/doc.go`, `internal/ui/doc_test.go`, `internal/ui/ui.go`
**Read first:** `internal/ui/ui.go` — package comment; `internal/tree/doc.go` — # Files; `internal/tree/doc_test.go` — TestDocFileMap_MatchesDirectory, mappedFilePattern;
`internal/ui/main_test.go` — TestMain
**Closes:** ui-doc-file-map
**Tests:** `GOMEMLIMIT=2GiB go test -count=1 -run TestDocFileMap_MatchesDirectory ./internal/ui/`
**Acceptance:**
- `go vet ./internal/ui/` is clean.
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/` passes.
- `grep -c '^// Package ui' internal/ui/*.go | grep -v ':0'` prints only `internal/ui/doc.go:1`.
**Commit:** `docs(ui): map the package's files in doc.go with a structural test`

## 4. Show every frontmatter key in the Issue detail header

**What:**
**Goal:** the Issue detail header (`issueHeader` in `internal/ui/detail.go`) renders, after the
`# <title>` heading, these lines in this order, each left out when its value is empty:
`status: <status> · P<n> · <type>` (the ` · <type>` part left out when type is empty), `labels:`,
`parent:`, `deps:`, `related:`, `plan:`, `aliases:`, `created:`, `closed:`, `reason:`. parent,
deps and related are code spans so ids become links; plan, aliases and reason are plain text
(reason through `oneLine`); created and closed read `2006-01-02 15:04` in UTC. Part of
`ui-detail-full-header`; item 5 completes it.
**Approach (assumed at the header base):** `issueSource` copies the parsed `issue.Issue`'s fields
onto the `index.Entry` before calling `issueHeader`, as it does for labels and deps today, so every
new field reads from the file and an edit shows without a reindex; the failure paths keep using
the index entry, which already holds Type, Related, Plan, Aliases, Created, Closed and Reason.
One helper renders a list of ids as code spans for parent, deps and related (deps' loop today).
**Regression guard.** issueFile (internal/ui/ready_test.go) gives every test Issue `type: task` and the fixed `created: 2026-10-11T09:00:00Z`, so item 4 rewrites every golden that shows an Issue detail (the status line gains " · task" and a `created: 2026-10-11 09:00` line); the Files line names internal/ui/testdata/golden/*.golden without the "only those" qualifier, and each rewritten golden is read before the commit.
**Files:** `internal/ui/detail.go`, `internal/ui/detail_test.go`, `internal/ui/testdata/golden/*.golden`
**Read first:** `internal/ui/detail.go` — issueSource, issueHeader, codeSpan, oneLine; `internal/index/index.go` — Entry;
`internal/ui/ready_test.go` — issueFile, detailText; `internal/ui/harness_test.go` — isGoldenUpdate
**Tests:** a table test over `issueHeader`: an Issue with every key set renders every line in the
order above; one with only title, status and priority renders the heading and the status line
alone. A model test selects an Issue with `plan:` and `related:` set and finds both lines in
`detailText`. `GOMEMLIMIT=2GiB go test -count=1 -run 'Detail|Header' ./internal/ui/`
**Acceptance:**
- `go vet ./internal/ui/` is clean.
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/` passes; every rewritten golden was read.
**Commit:** `feat(ui): show every frontmatter key in the issue detail header`

## 5. Show every frontmatter key in the Note detail header, and document the full header

**What:** Depends on item 4.
**Goal:** the Note detail header (`noteHeader` in `internal/ui/detail.go`) renders, after the
`# <title>` heading, `labels:`, `refs:`, `retired: true` (only when retired), `superseded-by:`,
`aliases:` in this order, each left out when empty; refs and superseded-by are code spans so ids
become links. `docs/design/tui.md`'s "Detail pane" bullet names every field of both headers and
says empty ones are left out.
**Approach (assumed at the header base):** `noteSource` copies the parsed `knowledge.Note`'s
retired, superseded-by and aliases onto the `index.NoteEntry` as it does for labels and refs; the
id-span helper from item 4 renders superseded-by. When `index.NoteEntry` lacks one of these fields,
the failure-path header leaves that line out rather than widening the index.
**Regression guard.** Every prose naming the detail header's fields or its id links names the full set, related and superseded-by included: tui.md's Detail pane bullet and its links sentence ("parent, deps and refs naming an id") and Knowledge tab section ("with its labels and refs"), and terminal-ui.md's Detail pane bullet ("a Note shows its labels and Refs") and mouse links bullet ("the Parent, dependencies and Refs"); `grep -n -i 'labels and refs\|parent, deps\|Parent, dependencies' docs/design/tui.md docs/site/getting-started/terminal-ui.md` finds no stale subset.
**Files:** `internal/ui/detail.go`, `internal/ui/detail_test.go`, `internal/ui/testdata/golden/*.golden` (only those whose Note header gains a line), `docs/design/tui.md`, `docs/site/getting-started/terminal-ui.md`
**Read first:** `internal/ui/detail.go` — noteSource, noteHeader, indexIDs; `internal/index/notes.go` — NoteEntry, readNote;
`internal/ui/knowledge_test.go` — knowledgeFiles, newKnowledgeModel; `internal/ui/mouse_test.go` — linkCell
**Closes:** ui-detail-full-header
**Tests:** a table test over `noteHeader`: a Note with every key renders every line in order; one
without retired, superseded-by and aliases renders none of them. A model test on the Knowledge tab
seats a retired Note with `superseded-by:` naming another Note and finds the link in the detail.
`GOMEMLIMIT=2GiB go test -count=1 -run 'Detail|Header|Knowledge' ./internal/ui/`
**Acceptance:**
- `go vet ./internal/ui/` is clean.
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/` passes; every rewritten golden was read.
- `grep -n 'superseded-by' docs/design/tui.md` finds the Detail pane bullet.
- `grep -n 'superseded-by' docs/site/getting-started/terminal-ui.md` finds the Detail pane bullet.
**Commit:** `feat(ui): show every frontmatter key in the Note detail header`
