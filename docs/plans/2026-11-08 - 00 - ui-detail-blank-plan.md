# Plan: the detail pane never goes blank silently

**Goal:** `capcom ui` never leaves the detail pane empty without saying why. Every detail read is
tracked until it answers, a slow one is reported with what holds the session, the frame can be
repainted on demand, and an opt-in trace pins the cause the next time it happens.
**Date:** 2026-11-08
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 9c2d2b2
**Closes:** ui-detail-blank

**Sources:**
- `.capcom/issues/human-ergonomics/ui-detail-blank.md` (the research findings)
- `docs/design/tui.md`; ADR 0012, ADR 0015 (`capcom know show adr-0012`, `adr-0015`)
- `docs/site/getting-started/terminal-ui.md`

**Ratified design calls** (Airic, 2026-11-08):
- **Pending read:** a dim `loading <id>…` line only where the pane has nothing to show (tab switch, start); on a selection move the old issue stays until the new read applies.
- **Slow read:** after 2s pending, the status bar says `reading <id>: waiting <N>s on <holder>`, naming what holds the session; it clears when the read applies or is superseded.
- **Repaint:** ctrl+l clears the screen and repaints the frame (listed on the help screen); the editor's return repaints the same way.
- **Trace:** `CAPCOM_UI_TRACE=<file>` appends one line per detail-read event and per long session-mutex wait; off when unset.

**Standing requirements:**
- skills: coding-standards
- Run single packages with `GOMEMLIMIT=2GiB`; never `./...` with `-race`; never two test commands at once (AGENTS.md).
- UI tests go through the harness in `internal/ui/harness_test.go`; never `t.Parallel()`.
- Any deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- Making the index poll cheaper or moving it off the session mutex (issue `warm-reads-large-store`).
- Changing Bubble Tea or its renderer.
- Loading lines or watchdogs for the tab reads, the dependency view or the status picker.

**Regression check (2026-11-08, 9c2d2b2):**
- 1: guard folded
- 2: recast
- 2 (re-check): guard folded (a pending re-read of the shown id survives landing on it; loading line style named; the A->B->A test added, not replaced; reverse-order tests made same-id)
- 3: guard folded
- 4: guard folded
- 5: guard folded
- 5 (re-check): guard folded (ui.Run sets the tracer on the model and its session after newModel, newModel's signature unchanged; the tracer is closed beside session.close; poll's waits are timed; the drawn signature is built only with a tracer)
- 6: guard folded
- 7: guard folded

## 1. The session records what holds it — ✅ DONE (2026-11-08)

NOTES (2026-11-08): the mutex is taken through acquire/release helpers in session.go that record and clear the holder; poll and close use them too, so close also records `another read` and holder never reports free while the mutex is held.
NOTES (2026-11-08): the regression guard's test hook is the unexported field `session.onHeld func(purpose string)`, nil in production, called by acquire (so by doFor and poll) once the holder is recorded; the poll sub-test asserts `the index poll` through it.

**What:**
**Goal:** `session` reports, while its mutex is held, the purpose of the holder and when it took
the mutex, and reports nothing while it is free; the index poll, a write and the editor's check
each hold it under their own purpose, every other access under a generic read purpose.
**Approach (assumed at the header base):** in `internal/ui/session.go` add `doFor(purpose string,
access)`; `do` stays as is and calls `doFor(purposeRead, access)`. The purpose and start time sit
in fields guarded by their own small mutex (never `mu`), so `holder() (purpose string, since
time.Time, isHeld bool)` answers while `mu` is held. `poll` (`internal/ui/refresh.go`) records
`purposePoll`; the write run in `internal/ui/write.go` uses `doFor(purposeWrite, …)`; `checkEdit`
in `internal/ui/editor.go` uses `doFor(purposeCheck, …)`. Purposes are user-visible text, constants
in `session.go`: `the index poll`, `a write`, `the editor's check`, `another read`.
**Regression guard.** `poll` (`refresh.go`) takes `mu` with no point a test can observe, so a
goroutine blocked behind a test-held `doFor` sees `the index poll` only by a race. Give `session` an
unexported hook (nil in production) that `doFor` and `poll` call once the holder is recorded, and
assert `purposePoll` from it; or drop that sub-test and keep only the `doFor`-based `holder()` tests.
**Files:** internal/ui/session.go, internal/ui/session_test.go, internal/ui/refresh.go, internal/ui/write.go, internal/ui/editor.go
**Read first:** internal/ui/session.go — session, do; internal/ui/refresh.go — poll, pollIndex;
internal/ui/write.go — runWrite, pendingWrite.land; internal/ui/editor.go — checkEdit;
internal/ui/session_test.go — TestSession_Do_ConcurrentCalls_RunOneAtATime
**Tests:** `session_test.go`: `holder()` called from inside an access reports its purpose and a
`since` no later than now; after the access returns it reports `isHeld == false`; `poll` reports
`the index poll` through the session's test hook (or that sub-test is dropped, per the guard).
**Acceptance:** `cd /home/airic/repos/capcom && go vet ./internal/ui/ && GOMEMLIMIT=2GiB go test -count=1 -run 'Session' ./internal/ui/`
**Commit:** `feat(ui): record what holds the session's mutex`

## 2. Track the pending detail read and show a loading line

**What:** Recast at the regression check (2026-11-08). Hardening for `ui-detail-blank`: a read could be dropped or never answer with nothing
on screen and no retry, because no state named the read in flight.
**Goal:** the model holds at most one pending detail read (id, Note or issue, sequence number); a
requested read applies only when its sequence is the pending one; a selection is re-read unless it
is the shown or the pending id; whenever the pane has no issue shown and a read is pending, it
draws exactly one line `loading <id>…` in `m.theme.mutedRow` (muted and faint); an empty
selection clears the pending read.
**Approach (assumed at the header base):** one producer of reads: `requestDetail(entry)` and
`requestNoteDetail(note)` on `model` (in `internal/ui/detail.go`) bump `detailSeq`, set
`detailPending` and return `loadDetail`/`loadNoteDetail` with the sequence in `detailMsg`. Every
caller of `loadDetail`/`loadNoteDetail` switches to them: `show{Ready,Issues,Knowledge,Closed}Selection`
and `withShownReloaded` (`refresh.go`). The `selected.ID == m.issue.id` short-circuit becomes
"shown or pending". `applyDetail` applies a read carrying a sequence only when it equals
`detailPending.seq`, then clears the pending read; a read without one (the write conflict's read in
`write.go`, `applyWritten` path) keeps today's selection-and-tab check. `openTab` (`keys.go`) clears
the pending read with the shown issue. `renderDetail` draws the loading line when `m.issue.id == ""`
and a read is pending, before its early return.
**Regression guard.** Landing the selection on the shown id clears only a pending read of another id; a pending re-read of the shown id itself (a live refresh's, from withShownReloaded) stays pending with its watch, so item 3's slow-read notice still covers it. applyDetail keeps its selection-and-tab check for every read; a sequenced read applies only when it also carries detailPending.seq, and a sequenced read the check drops clears the pending read when it was the pending one. All clearing of the pending read goes through one helper (clearPendingDetail) that item 3 extends. Add (no such test exists to replace) the test: shown A, j (read B pending), k back to A before B answers leaves A shown and nothing pending, B's late read is dropped, and a second j re-reads B. requestDetail/requestNoteDetail re-render the pane (renderDetail) after setting the pending read, so the loading line reaches the frame on a tab switch; renderDetail draws it. The loading line uses m.theme.mutedRow (theme.go, muted and faint); the sidebar's empty text is unstyled, so it is no reference. The reverse-order and stale-read tests use two reads of the SAME id (two reloads of shown A, the file edited between, each via m.Update's cmd run by hand): reads of different ids are already dropped by applyDetail's selection check at the base.
**Files:** internal/ui/detail.go, internal/ui/detail_test.go, internal/ui/model.go, internal/ui/issues.go, internal/ui/ready.go, internal/ui/knowledge.go, internal/ui/closed.go, internal/ui/refresh.go, internal/ui/keys.go
**Read first:** internal/ui/detail.go — applyDetail, renderDetail, loadDetail, loadNoteDetail; internal/ui/refresh.go — withShownReloaded; internal/ui/issues.go — showIssuesSelection, issuesKey, applyIssues;
internal/ui/ready.go — showReadySelection (same shape in knowledge.go showKnowledgeSelection, closed.go showClosedSelection); internal/ui/keys.go — openTab; internal/ui/mouse.go — wheelSidebar, clickSidebar;
internal/ui/write.go — applyConflict; internal/ui/harness_test.go — send, press, drive
**Tests:** `detail_test.go`: after a tab switch with the read not yet delivered (use `send`, not
`press`), the frame holds `loading <id>…`; two re-reads of shown A (two reloads, the file edited
between) delivered in reverse order leave the newer body shown; a stale read of the shown id does
not replace a newer pending one; shown A, j (read B pending), k back to A before B answers leaves A
shown and nothing pending, B's late read is dropped, and a second j re-reads B; a reload's re-read
of shown A held (`send`), k at the top row, then delivered shows A's edited body; the conflict path
still applies.
**Acceptance:** `cd /home/airic/repos/capcom && go vet ./internal/ui/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/`
**Commit:** `fix(ui): track the pending detail read and say when the pane is loading`

## 3. Report a detail read that waits too long

**What:** Depends on items 1 and 2.
**Goal:** a detail read still pending 2s after it was requested puts `reading <id>: waiting <N>s
on <holder>` in the status bar, `<holder>` being the session's current holder purpose (left out,
with its ` on`, when the mutex is free), updated each second while that read stays pending, and
gone once the read applies or a newer read supersedes it.
**Approach (assumed at the header base):** `requestDetail`/`requestNoteDetail` also return a tick
from a new injectable `m.detailTick` (like `m.saveTick`; `startModel` in `harness_test.go` sets it
to `noFlashTick`) after `detailWatchDelay` (2s) carrying `detailWatchMsg{seq, requested}`. Its
handler in `model.go` ignores a stale sequence; otherwise it sets a new `detailNotice` from
`m.session.holder()` and schedules the next tick at 1s. `applyDetail` clears `detailNotice`. The
status bar (`statusbar.go`, the notice list in `statusBar`) shows `detailNotice` right after
`refreshNotice`.
**Regression guard.** clearPendingDetail (item 2) also clears detailNotice, so every site that clears or replaces the pending read clears the notice; the "waiting 2s" test sets detailWatchMsg.requested 2s in the past (or replaces the model's clock) so N reads 2.
`TestWorktreeState_ReloadBeforeKnowledgesFirstRead_KeepsItsStoredID` (`uistate_test.go`) builds with
`newModel` and `drive`, so it sets `m.detailTick = noFlashTick`; the read's batch puts the load
ahead of the watch tick.
**Files:** internal/ui/detail.go, internal/ui/model.go, internal/ui/statusbar.go, internal/ui/harness_test.go, internal/ui/detail_test.go, internal/ui/statusbar_test.go, internal/ui/uistate_test.go
**Read first:** internal/ui/model.go — update, model (saveTick, now fields); internal/ui/statusbar.go — statusBar;
internal/ui/harness_test.go — startModel, noFlashTick, statusRow; internal/ui/session.go — holder (item 1);
internal/ui/uistate_test.go — TestWorktreeState_ReloadBeforeKnowledgesFirstRead_KeepsItsStoredID
**Tests:** send `detailWatchMsg` for the pending sequence, its `requested` 2s in the past, while a
test goroutine holds the session via `doFor(purposePoll, …)`: the status row reads `reading <id>:
waiting 2s on the index poll`; a stale sequence changes nothing; delivering the read clears the
notice; a tab switch that clears or replaces the pending read (an empty list, a selection already
shown) clears it too.
**Acceptance:** `cd /home/airic/repos/capcom && go vet ./internal/ui/ && GOMEMLIMIT=2GiB go test -count=1 -run 'Detail|StatusBar' ./internal/ui/`
**Commit:** `feat(ui): report a detail read that waits on the session`

## 4. ctrl+l and the editor's return repaint the frame

**What:**
**Goal:** ctrl+l, among the model's own keys, returns a command whose message is Bubble Tea's
clear-screen message; the help screen lists `repaint  ctrl+l`; the editor's return (`editedMsg`,
success or failure) returns the same command alongside what it returned before.
**Approach (assumed at the header base):** a `keyRepaint = "ctrl+l"` constant in `keys.go`
handled in `handleKey` returning `tea.ClearScreen`; prompts, the picker and the filter line take
their keys first, so ctrl+l there is unchanged. `applyEdited` (`editor.go`) batches
`tea.ClearScreen` with `checkEdit`. Add the help row to `helpLines` (`help.go`) and update the help
golden file with `go test ./internal/ui/ -run <help golden test> -update`.
**Regression guard.** `helpLines` holds 20 rows, the body height at 80x24
(`TestModel_View_HelpScreen_FitsTheSmallTerminal`): fold `repaint  ctrl+l` into an existing row (or
drop the leading "" row) instead of appending one, then regenerate `help-120x30.golden`. A failed
edit returns `tea.ClearScreen` itself (`tea.Batch` of one command compacts to it), not a `tea.BatchMsg`.
**Files:** internal/ui/keys.go, internal/ui/keys_test.go, internal/ui/help.go, internal/ui/editor.go, internal/ui/editor_test.go, internal/ui/testdata/golden/
**Read first:** internal/ui/keys.go — handleKey, the key constants; internal/ui/model.go — update (KeyPressMsg routing: prompt, textPrompt, picker, filter, depViewKey first);
internal/ui/help.go — helpLines; internal/ui/layout_test.go — TestModel_View_HelpScreen_FitsTheSmallTerminal; internal/ui/editor.go — applyEdited;
internal/ui/editor_test.go — TestEdit_EditorFails_ShowsItsError; internal/ui/harness_test.go — ctrlKey
**Tests:** `keys_test.go`: ctrl+l's command yields the clear-screen message; `editor_test.go`: the
returned batch of a successful edit holds it, and a failed edit's command is `tea.ClearScreen`
directly (`cmd() == tea.ClearScreen()`); help golden updated; the help screen still fits 80x24.
**Acceptance:** `cd /home/airic/repos/capcom && go vet ./internal/ui/ && GOMEMLIMIT=2GiB go test -count=1 -run 'Key|Editor|Help|Golden' ./internal/ui/`
**Commit:** `feat(ui): repaint the frame on ctrl+l and after the editor`

## 5. CAPCOM_UI_TRACE writes a detail-read trace

**What:** Depends on items 1, 2 and 3.
**Goal:** with `CAPCOM_UI_TRACE` set to a path, `capcom ui` appends one line per detail-read
event — requested, applied, stale (with the reason), rendered (line count), watch notice — one
per session-mutex wait over 100ms (purpose waited for, duration, holder at the start), and a
`drawn` line whenever the detail pane `View` draws differs from the last one traced (shown id,
non-blank text rows, pane left column, width and rows), so a blank screen over a drawn pane shows
as a terminal fault and a blank drawn pane as a model fault; unset, no
file is touched; a path that cannot be opened fails `capcom ui` before the screen opens with
`CAPCOM_UI_TRACE: <path>: <error>`. Each line: RFC 3339 UTC time with milliseconds, event, fields
as `key=value`.
**Approach (assumed at the header base):** `View` is a value receiver, so the tracer keeps the
last `drawn` signature itself, under its mutex. `traceVariable` beside `noColorVariable` in
`internal/cli/ui.go` fills a new `ui.Options.TracePath`; `ui.Run` (`internal/ui/ui.go`) opens it
append-only, mode 0600, and hands a `tracer` (new `internal/ui/trace.go`; a nil tracer is a no-op)
to the model and the session. `uiLong` gains one paragraph naming the variable; regenerate
`docs/site/reference/cli/capcom_ui.md` with `go run ./cmd/gendocs`.
**Regression guard.** `doc.go`'s file map gains a "`trace.go` — tracer: …" line (`TestDocFileMap_MatchesDirectory`).
The tracer serializes writes, a mutex around format and one Write per line: lines come from Update
and from tea.Cmd goroutines. `ui.Run` opens the trace before `newModel`, and builds the text from the
`*PathError`'s `Err` so the path is printed once.
ui.Run sets the tracer on the model and on its session after newModel returns; newModel's signature stays as it is (it has test callers).
The tracer gets a nil-safe `close`; `Run` closes it after `program.Run` returns, beside `initial.session.close`; writes after close (straggling tea.Cmd goroutines) are dropped silently; every test that opens a tracer closes it in `t.Cleanup` (Windows CI cannot remove an open TempDir file).
`poll` (`refresh.go`) takes `s.mu` itself: time the wait in one `session.go` lock helper that both `doFor` and `poll` call, so a poll's wait over 100ms writes its line too.
`View` runs every frame: build the `drawn` signature only when the tracer is non-nil, from the panes `body()` lays out (no detail pane while help or the one-pane sidebar shows), not from `detailRows` called again.
**Files:** internal/cli/ui.go, internal/ui/ui.go, internal/ui/trace.go, internal/ui/trace_test.go, internal/ui/detail.go, internal/ui/session.go, internal/ui/refresh.go, internal/ui/model.go, internal/ui/layout.go, internal/ui/doc.go, docs/site/reference/cli/capcom_ui.md
**Read first:** internal/ui/ui.go — Options, Run; internal/ui/model.go — newModel, View, model; internal/ui/layout.go — frame, body, detailRows, detailLeft;
internal/ui/detail.go — applyDetail, renderDetail, loadDetail; internal/ui/session.go — do, session; internal/ui/refresh.go — poll;
internal/cli/ui.go — noColorVariable, uiLong, newUICommand; internal/ui/harness_test.go — startModel, send
**Tests:** `trace_test.go`: a model with a tracer over a temp file, driven through a tab switch,
writes requested/applied/rendered lines with the id; two `View` calls over the same pane write one
`drawn` line and a divider move writes another; a stale read writes a stale line; a nil tracer
writes nothing; an unopenable path makes `ui.Run` fail with the stated text, the path named once,
before `newModel` runs; a `poll` waiting over 100ms behind a held `doFor` writes a wait line; a
write after `close` is dropped; every test that opens a tracer closes it in `t.Cleanup`.
**Acceptance:** `cd /home/airic/repos/capcom && go vet ./internal/ui/ ./internal/cli/ ./internal/clidoc/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`
**Commit:** `feat(ui): trace detail reads to the file CAPCOM_UI_TRACE names`

## 6. A randomized interleaving test pins the detail pane

**What:** Depends on item 2. Test-only.
**Goal:** `internal/ui/interleave_test.go` drives a fixed set of seeds (bounded so the test runs
under 15s on the Pi) of random keys (arrows, j k h l, Tab, 1–4, d, Esc, Enter, ?, v), clicks and
wheel notches at random cells, ticks and on-disk edits of issue files, with every returned command
held in a pool and delivered in random order. After every step: a selection with an empty pane
means a pending read for that selection. After draining the pool: with no overlay open, the shown
id equals the selection and the frame contains its detail.
**Approach (assumed at the header base):** a test-local pool runner beside `drive` that expands
`tea.BatchMsg`; models from `newTestModel` at widths 120 and 60 and heights 40 and 12; an issue
tree with a child, a closed issue and a Note sharing an issue's id. No write keys, no `e`, no `q`.
**Regression guard.** At 60x12 the layout is one pane (`isSinglePane`): the sidebar fills the screen
until Enter. Apply the post-drain "frame contains its detail" check only where the detail pane is on
screen (`!m.isSinglePane() || m.isDetailOpen`), help and deps closed; check shown id == selection everywhere.
**Files:** internal/ui/interleave_test.go
**Read first:** internal/ui/harness_test.go — drive, newTestModel, newTestStore; internal/ui/model.go — showSelection;
internal/ui/keys.go — handleKey; internal/ui/mouse.go — mouseClick, mouseWheel; internal/ui/layout.go — isSinglePane
**Tests:** the file itself; it fails with the seed, the size and the step trace.
**Acceptance:** `cd /home/airic/repos/capcom && GOMEMLIMIT=2GiB go test -count=1 -run 'Interleave' ./internal/ui/`
**Commit:** `test(ui): drive randomized interleavings against the detail pane`

## 7. Document the loading line, the slow-read notice, ctrl+l and the trace

**What:** Depends on items 2–5. Owns every doc amendment of this plan.
**Goal:** `docs/site/getting-started/terminal-ui.md` describes the loading line, the slow-read
notice, ctrl+l and `CAPCOM_UI_TRACE`; `docs/design/tui.md` describes the detail read's lifecycle
(one pending read, its sequence, the watch, the trace). Every doc line that says the pane is blank
until a read arrives, or lists the keys without ctrl+l, is updated
(`grep -rn "detail pane\|ctrl+" docs/site docs/design`).
**Regression guard.** The key-map copies in docs/design/tui.md and docs/site/getting-started/terminal-ui.md mirror the row layout item 4 gives helpLines.
`docs/design/tui.md` already holds "loading" (lines 127, 257), so the Acceptance greps
`CAPCOM_UI_TRACE` and `ctrl+l` there instead.
**Files:** docs/site/getting-started/terminal-ui.md, docs/design/tui.md
**Read first:** docs/site/getting-started/terminal-ui.md — key block (scroll row), the PgUp/PgDn bullet, the NO_COLOR environment line;
docs/design/tui.md — ## Keys block, **Detail scroll** paragraph; internal/ui/help.go — helpLines (the key block mirrors it);
docs/site/reference/cli/capcom_ui.md — generated by item 5, not hand-edited
**Tests:** none (docs).
**Acceptance:** `cd /home/airic/repos/capcom && grep -n "CAPCOM_UI_TRACE" docs/site/getting-started/terminal-ui.md && grep -n "ctrl+l" docs/site/getting-started/terminal-ui.md && grep -n "CAPCOM_UI_TRACE" docs/design/tui.md && grep -n "ctrl+l" docs/design/tui.md`
**Commit:** `docs(ui): describe the loading line, slow-read notice, repaint and trace`
