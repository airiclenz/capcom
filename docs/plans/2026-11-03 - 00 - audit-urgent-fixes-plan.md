# Audit urgent fixes plan

**Goal:** Fix the P1 and P2 bugs from the 2026-11-03 code audit, plus the UI row sanitising that
shares their helper, so that repository text cannot crash, spoof or escape capcom.
**Date:** 2026-11-03
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 1b73606

**Sources:**
- docs/reviews/code-audit-2026-11-03.md
- .capcom/issues/ (the ids on each item's `**Closes:**` line)
- Note `coding-conventions` and its child `error-messages`; Note `cli-test-harness`

**Ratified design calls:**
- **UI control bytes:** the UI renders control bytes as visible `\xHH` text, through the same helper `capcom show` uses, moved into a shared package (user, 2026-11-03).
- **Scope:** include `ui-row-control-bytes` and sanitise every title and lock-record string the UI draws, with no exploitability probe first (user, 2026-11-03).

**Standing requirements:**
- skills: coding-standards
- Tests on this host: one package at a time, `GOMEMLIMIT=2GiB`, never `-race` on `./...`.
- Any deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- The P3 audit bugs other than `ui-row-control-bytes`, and every child of `audit-2026-11-03`.
- Changing what `capcom show` prints for control bytes.

**Regression check (2026-11-03, 1b73606):**
- 1: guard folded
- 2: guard folded
- 3: guard folded
- 4: guard folded
- 5: guard folded
- 6: guard folded
- 8: guard folded

## 1. Move the terminal sanitiser into a shared package — ✅ DONE (2026-11-03)

**What:**
**Goal:** a package `internal/termsafe` exports `Escape(text string) string`, which rewrites C0
controls other than TAB and LF, DEL, C1 controls and each byte of invalid UTF-8 as visible
`\xHH` text and returns everything else unchanged; `internal/cli/output` calls it and keeps no
copy of the logic.
**Approach (assumed at the header base):** move `sanitizeTerminal`, `isTerminalControl` and their
constants out of `internal/cli/output/output.go` into `internal/termsafe/termsafe.go` as
`Escape`; every `sanitizeTerminal` call in `internal/cli/output` calls `termsafe.Escape`. Move the
sanitiser's own unit tests with it. One deep function, no options: both the CLI and the UI need
exactly this rule. Pure refactor: CLI output is byte-identical.
**Regression guard.** The `Result` doc comment (`output.go:55`) and every other comment naming
`sanitizeTerminal` is reworded to name `termsafe.Escape`; Acceptance greps every mention, comments
included, not only the `func` line.
**Files:** internal/termsafe/termsafe.go, internal/termsafe/termsafe_test.go, internal/cli/output/output.go, internal/cli/output/output_test.go
**Read first:** internal/cli/output/output.go — sanitizeTerminal, isTerminalControl, firstPrintableRune, textRenderer.Render, textRenderer.RenderError, Result;
internal/cli/output/output_test.go — TestSanitizeTerminal_EscapesControlsOnly, TestTextRenderer_ControlCharacters_EscapedInEveryStream
**Tests:** `internal/termsafe` table test: ESC, BEL, CR, DEL, U+0085, invalid UTF-8, TAB and LF
kept, plain and multi-byte text unchanged. The existing `internal/cli/output` tests pass unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/termsafe/ ./internal/cli/output/`
- `go vet ./internal/termsafe/ ./internal/cli/output/`
- `grep -rn 'sanitizeTerminal' internal/` prints nothing (every mention, comments included)
**Commit:** `refactor(output): move the terminal sanitiser into internal/termsafe`

## 2. A forged link marker in a body no longer crashes capcom ui

**What:** fixes the `capcom ui` panic: a body holding `ESC _capcom-link:-5 ESC \` indexes
`targets[-5]`, and a forged in-range index spoofs a followable link.
**Goal:** `markdown.Render` never panics on any body text, reports a Link only for a link it
rendered itself, and its output holds no control byte from the source other than TAB and LF; a
source control byte shows as `\xHH`.
**Approach (assumed at the header base):** at the top of `Render` in
`internal/ui/markdown/markdown.go`, pass `text` through `termsafe.Escape` before splitting it, so
only the renderer can write markers. As a second guard, `nextLinkMarker` in `links.go` treats a
negative index as a foreign marker (resume the scan after it), and `stripLinkMarkers` skips any
`open` outside `0 <= open < len(targets)`.
Depends on item 1.
**Regression guard.** The forged in-range test body also holds a real followable link (`[x](id)`
with `knownIDs`) and asserts exactly one Link, whose `regionText` is the real link's text, the
forged text visible as `\x1b_capcom-link...`. The `stripLinkMarkers` out-of-range line is built
with `linkOpenMarker(-5)`, plus a second case with a positive index: no panic, no Link.
**Files:** internal/ui/markdown/markdown.go, internal/ui/markdown/links.go, internal/ui/markdown/links_test.go, internal/ui/markdown/markdown_test.go
**Read first:** internal/ui/markdown/markdown.go — Render; internal/ui/markdown/links.go — nextLinkMarker, stripLinkMarkers, linkOpenMarker, noLink;
internal/ui/markdown/links_test.go — knownIDs, regionText, TestMalformedLinksStayLiteral
**Tests:** `Render` with a forged open marker around visible text, once with index -5 and once
with 99: no panic, no Link reported, `\x1b` visible in the output. A forged in-range index in a
body that also holds a real followable link (`knownIDs`): exactly one Link, whose `regionText` is
the real link's text, the forged text visible as `\x1b_capcom-link...`.
A body with an OSC sequence (`"\x1b]0;x\x07"`) renders as visible text. A
`stripLinkMarkers` test on a line holding renderer-written markers built with `linkOpenMarker(-5)`,
and a second case with a positive out-of-range index, does not panic and returns no Link.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/markdown/`
- `go vet ./internal/ui/markdown/`
**Closes:** ui-link-marker-panic
**Commit:** `fix(ui): render forged link markers and control bytes in bodies as text`

## 3. Sanitise titles and lock records drawn in UI rows

**What:** fixes control bytes in issue and Note titles and in lock Holder and Purpose reaching
the terminal raw through `capcom ui` rows, the status bar and notices.
**Goal:** every title, lock Holder, lock Purpose and error text `capcom ui` draws shows a control
byte as `\xHH`, never raw.
**Approach (assumed at the header base):** `oneLine` in `internal/ui/statusbar.go` applies
`termsafe.Escape` before joining fields, which covers every caller (`closed.go`, `knowledge.go`,
`ready.go`, `detail.go`, `depview.go`, `write.go`, `statuspick.go`, `editor.go`, `statusbar.go`).
The lock `record.Holder` drawn in `statusbar.go` goes through `oneLine` too. Escaping is
idempotent for this purpose: the detail title passes `oneLine` and then item 2's `Render`, and a
`\xHH` sequence holds no control byte.
Depends on item 1.
**Regression guard.** `oneLine` escapes the joined result, `termsafe.Escape(strings.Join(strings.Fields(text), " "))`,
so CR, VT, FF and U+0085 still collapse to a space, as today. `statusBar` also passes each notice
through `termsafe.Escape` in its notice loop (idempotent on `oneLine` output), covering the error
text of `refresh.go`, `model.go`, `uistate.go` and `depview.go` (`row.missing`) with no edit there.
**Files:** internal/ui/statusbar.go, internal/ui/statusbar_test.go, internal/ui/ready_test.go
**Read first:** internal/ui/statusbar.go — oneLine, lockText, statusBar; internal/ui/refresh.go — applyRefreshed;
internal/ui/statusbar_test.go — arrangeHeld, paintedStatusRow; internal/ui/harness_test.go — newTestStore, startModel
**Tests:** `oneLine` unit test with ESC, BEL and CR; CR comes out as a space. A model test: an
issue whose title holds `"\x1b]0;x\x07"`, a lock record whose Holder and Purpose hold ESC, and a
`refreshNotice` holding ESC; `m.View().Content` holds no `\x1b]` and no `\x07`, and shows `\x1b`
as text in the Ready row and the status bar.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/`
- `go vet ./internal/ui/`
**Closes:** ui-row-control-bytes
**Commit:** `fix(ui): show control bytes in titles and lock records as text`

## 4. Refuse a children directory that is not a real directory

**What:** fixes `capcom mv` and child creation writing through a committed symlink
`issues/foo -> /elsewhere` beside `foo.md`, landing the node outside the store.
**Goal:** moving a node below, or creating a child of, a parent whose children path exists as
anything but a real directory (a symlink, a file) fails with an error naming that path, and
writes nothing.
**Approach (assumed at the header base):** `Tree.PathFor` in `internal/tree/tree.go`, the one
place both `PlanMove` (`move.go`) and `nodestore`'s `newNodePath` (`create.go`) get a node's
path, `os.Lstat`s the parent's children directory; a path that exists and is not a directory
(`ModeSymlink` included) fails with an error that names it. A missing directory is fine.
`planRenames` keeps its `os.Stat(newDir)` existence check. No new module.
**Regression guard.** `PathFor` returns the refusal as a `*tree.Error{Path: <children dir>}`, so
create's `resolveFailure` relativizes it. The symlink cases call `tree.Load`, accept the
`tree.Errors` it returns beside the tree (walk reports the symlink as `ErrNotRegular`) and use that
tree, never `mustLoad`; the regular-file case loads clean. The issue's Acceptance needs create.
**Files:** internal/tree/tree.go, internal/tree/tree_test.go, internal/tree/move_test.go, internal/nodestore/create_test.go
**Read first:** internal/tree/tree.go — PathFor, ChildrenDir; internal/tree/move.go — PlanMove, planRenames;
internal/nodestore/create.go — newNodePath; internal/nodestore/load.go — resolveFailure; internal/tree/tree_test.go — makeSymlink, mustLoad
**Tests:** with `issues/foo.md` and `issues/foo` a symlink to a temp directory outside the
store: `PathFor("bar", "foo")` fails with a `*tree.Error` naming `issues/foo`; `PlanMove` of `bar`
below `foo` fails and nothing is written in the outside directory. Same for a regular file at
`issues/foo`. A nodestore `Create` test (`create_test.go`) of a child of `foo` fails naming the
relative `.capcom/issues/foo`. Symlinks come from `makeSymlink`, which skips on Windows and as root;
the symlink cases load with `tree.Load` and accept its `tree.Errors`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/tree/ ./internal/nodestore/`
- `go vet ./internal/tree/`
**Closes:** tree-symlinked-children-dir
**Commit:** `fix(tree): refuse a children path that is not a real directory`

## 5. Check config plans entries stay inside the repository

**What:** fixes `plans: [../../../etc, /srv/x, 'a\b']` being accepted, which lets `PlanPath`
stat and report files outside the worktree.
**Goal:** loading a config whose `plans` list holds an absolute path, a path leaving the
repository or a backslash path fails, naming the line and `plans`; relative slash-separated
entries still load.
**Approach (assumed at the header base):** in `topLevel` in `internal/config/parse.go`, the
`keyPlans` case runs `p.localPath` on each item of the list after `nameList`, as `guides` does.
**Regression guard.** A `plans` entry outside the repository (`../shared/plans`, `/srv/plans`) that
loads today fails `config.Parse` after this item, and with it every command. The `plans` bullet in
`docs/site/reference/file-format.md` says each entry is a slash-separated path inside the
repository, relative to its root, as the `agents.guides` bullet does.
**Files:** internal/config/parse.go, internal/config/config_test.go, docs/site/reference/file-format.md
**Read first:** internal/config/parse.go — topLevel, nameList, localPath, guides; internal/config/config_test.go — TestParse_InvalidConfig_FailsWithPathAndLine, TestParse_DefaultFile_EqualsDefault;
internal/store/store.go — Open config parse; docs/site/reference/file-format.md — plans bullet
**Tests:** invalid-config table rows for `plans: [../x]`, `plans: [/abs]` and `plans: ['a\b']`,
each failing at its line with the `must be a slash-separated path inside the repository`
message; the default `plans` list still loads.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/config/`
- `go vet ./internal/config/`
**Closes:** config-plans-local-path
**Commit:** `fix(config): require plans entries to be paths inside the repository`

## 6. Count only the maintainer's own winget pull requests

**What:** fixes `probeWinget` counting any open winget-pkgs pull request with a matching title
as the release's submission, so a stranger's PR makes publish skip the winget step.
**Goal:** the winget probe reports a step done or setup-pending only for a pull request the
signed-in account authored; a matching title from any other author leaves the step pending.
**Approach (assumed at the header base):** `searchPulls` in `cmd/release/channelrepos.go` adds
`--author @me` to its `gh pr list` call; `OpenPulls` keeps merging the viewer's own pull
requests. Update the `OpenPulls` doc comment and any test fake that asserts the argument vector.
**Regression guard.** No probe test: `memChannelRepos.OpenPulls` returns every pull request
whatever the query and `probeWinget` filters by title only, so such a test passes before and after.
The author filter is pinned where it lives: `wantCalls` in
`TestExecChannelReposOpenPullsReadsTheViewersPullsBesideTheSearch` includes `--author @me`.
**Files:** cmd/release/channelrepos.go, cmd/release/channelrepos_test.go, cmd/release/memchannels_test.go
**Read first:** cmd/release/channelrepos.go — searchPulls, OpenPulls, viewerOpenPulls; cmd/release/winget.go — probeWinget;
cmd/release/channelrepos_test.go — TestExecChannelReposOpenPullsReadsTheViewersPullsBesideTheSearch, pullsAnswering; cmd/release/memchannels_test.go — memChannelRepos.OpenPulls
**Tests:** `wantCalls` in `TestExecChannelReposOpenPullsReadsTheViewersPullsBesideTheSearch`
asserts the `gh pr list` argument vector includes `--author @me`. The `memChannelRepos.OpenPulls`
doc comment says the author filter lives in the production adapter. No probe test.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./cmd/release/`
- `go vet ./cmd/release/`
**Closes:** winget-probe-any-author
**Commit:** `fix(release): count only the signed-in account's winget pull requests`

## 7. An unclosed code fence no longer hides conflict markers

**What:** fixes `ConflictMarkers` treating every line after an unclosed fence opener as code,
so a body with a stray fence line and a later conflict passes `capcom check`.
**Goal:** `ConflictMarkers` and `ClosedCodeLines` share one definition of a code line: markers
inside a closed fenced block are skipped, and a fence opener no later line closes hides nothing.
**Approach (assumed at the header base):** in `internal/rawfm/conflict.go`, build the line list
with one trailing `\r` trimmed, compute `ClosedCodeLines(trimmed, bodyStart)` once, and skip a
line only where it marks code, dropping the streaming `CodeFence`.
**Files:** internal/rawfm/conflict.go, internal/rawfm/conflict_test.go
**Read first:** internal/rawfm/conflict.go — ConflictMarkers, bodyStartIndex; internal/rawfm/fence.go — ClosedCodeLines, CodeFence.IsCode;
internal/rawfm/conflict_test.go — TestConflictMarkers_FileContent_ReturnsTheMarkerLines, TestConflictMarkers_CRLFContent_TrimsTheCarriageReturnFromTheText;
internal/check/check.go — checkConflictMarkers; internal/issue/mutate.go — ClosedCodeLines caller
**Tests:** a body of an unclosed three-backtick line, text, then `<<<<<<< HEAD`, `x`, `=======`,
`y`, `>>>>>>> b` reports all three markers; the same block inside a closed fence reports none;
CRLF content gives the same markers.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/rawfm/ ./internal/check/`
- `go vet ./internal/rawfm/`
**Closes:** conflict-scan-unclosed-fence
**Commit:** `fix(rawfm): let an unclosed fence hide no conflict markers`

## 8. ReplaceFileVerbatim never recreates a removed file

**What:** fixes `ReplaceFileVerbatim` recreating a node file another session removed between its
existence check and the rename, the case `ReplaceFileAtomic` already refuses.
**Goal:** `ReplaceFileVerbatim` on a path missing at the rename fails with an error wrapping
`fs.ErrNotExist` and creates no file or directory; an ordinary replace still keeps CRLF and the
old mode.
**Approach (assumed at the header base):** in `internal/nodefile/atomic.go`,
`ReplaceFileVerbatim` keeps its `os.Stat` for the mode, then calls `writeFileAtomic(path,
content, false, info.Mode().Perm(), true)`, so `checkReplaceTarget` and the no-mkdir rule apply.
`WriteFileAtomicVerbatim` is unchanged.
**Regression guard.** The target is removed mid-write by a reader that deletes it while it is read
(pattern: `failingReader`, `atomic_test.go`); off Windows no rename or link hook reaches the replace
path. No missing-parent sub-assertion: `writeTemp`'s MkdirAll runs before the content is read, and
`TestReplaceFileVerbatim_MissingPath_FailsNamingItAndCreatesNothing` already covers a missing target.
**Files:** internal/nodefile/atomic.go, internal/nodefile/atomic_test.go
**Read first:** internal/nodefile/atomic.go — ReplaceFileVerbatim, writeFileAtomic, checkReplaceTarget, writeTemp;
internal/nodefile/atomic_test.go — TestReplaceFileVerbatim_MissingPath_FailsNamingItAndCreatesNothing, TestReplaceFileVerbatim_CRLFFile_KeepsBytesAndModeAndLeavesNoTempFile, failingReader
**Tests:** remove the target after the Stat through a reader that deletes the file while it is read
(pattern: `failingReader`) and assert `errors.Is(err, fs.ErrNotExist)` and no file at the path.
Existing CRLF and mode tests pass.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodefile/`
- `go vet ./internal/nodefile/`
**Closes:** replace-verbatim-resurrects-file
**Commit:** `fix(nodefile): never recreate a file ReplaceFileVerbatim replaces`
