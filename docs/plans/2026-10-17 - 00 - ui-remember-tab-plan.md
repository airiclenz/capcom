# capcom ui reopens on the last tab and selection, per worktree

**Goal:** `capcom ui` remembers, per worktree, the tab it was on and the issue or Note selected on
each tab, in `<git-dir>/capcom/ui-state.yaml`, and reopens there; command-line arguments win.
**Date:** 2026-10-17
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** ba503ff
**Closes:** ui-remember-tab

**Sources:**
- `.capcom/issues/ui-remember-tab.md`
- Notes `adr-0014`, `adr-0014-decision` (global `state.yaml`), `store-layout`
- `internal/ui/model.go`, `internal/ui/refresh.go`, `internal/userconfig/state.go`, `internal/cli/ui.go`

**Ratified design calls** (user, 2026-10-17):
- **Scope:** the tab plus one selected id per tab (Ready, Issues, Knowledge); the divider ratio stays in the global `state.yaml`.
- **Location:** `<git-dir>/capcom/ui-state.yaml`, beside `index.db`: per worktree, never committed.
- **Arguments:** `capcom ui <tab>` opens that tab and still restores each tab's stored selection; `capcom ui <tab> <id>` selects that id on that tab; only no argument opens the stored tab.
- **Fallback:** a stored id that is gone or not on its tab selects the first row silently; a stored tab name this binary does not know opens Ready.
- **Closed or retired:** a stored id still held by the Issues or Knowledge tree is selected and revealed, as a command-line id is.
- **Write time:** a tab change saves at once; a selection change saves 1 s after the last change (debounced); quitting flushes a pending save.
- **Failure:** a failed load or save is one status-bar notice per session; the UI runs on as today.
- **Record:** a new ADR 0015; ADR 0014 gains a pointer to it.

**Standing requirements:**
- skills: coding-standards
- Tests per `AGENTS.md`: single packages with `GOMEMLIMIT=2GiB`, never `./...` with `-race`, never two test commands at once.
- No test writes under the real `os.UserConfigDir` or a real repository's git dir: test options set every state path to a temp dir.
- Any authorized deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- Remembering filter text, scroll offsets, tree expansion or the dependency view.
- Moving `ui.sidebar_ratio` out of the global `state.yaml`.
- Keeping unknown keys of `ui-state.yaml` across a save.

**Regression check (2026-10-17, ba503ff):**
- 1: guard folded (quoted ids, invalid-value rows nested under `ui:`, package doc, Go-comment prose rule)
- 3: guard folded (absent file or `tab:` keeps `options.Tab`, Go-comment prose rule)
- 4: guard folded (injectable debounce tick, named quit flush, worktree test on each `GitDir`, `doc.go` file map)
- 5: guard folded (Knowledge prose rule with grep, adds `adr-0011-decision`, depends on item 4)
- 6: guard folded (Acceptance also runs `HelpText`)

## 1. Worktree state file in userconfig — ✅ DONE (2026-10-17)

NOTES (2026-10-17): the read-and-parse prelude LoadState had (missing file → zero, unreadable → *Error, parser.document) moved into a shared readDocument helper in state.go that LoadState and LoadWorktreeState both call, instead of being copied; LoadState's behaviour is unchanged and its tests pass.
NOTES (2026-10-17): LoadWorktreeState accepts only a `!!str` scalar (quoted, or plain text YAML resolves to no other type) and errors with "expected a quoted string"; a null value stores nothing, as in state.yaml.

**What:**
**Goal:** `internal/userconfig` exposes `WorktreeState{Tab string; Ready, Issues, Knowledge string}`, `LoadWorktreeState(path)` and `SaveWorktreeState(path, WorktreeState)`; a saved state loads back equal, and `state.yaml`'s API, file and behaviour are unchanged.

**Approach (assumed at the header base):**
- Extends `internal/userconfig` (it owns the files only capcom ui reads and writes; `TestPackage_ImportedOnlyByUI` keeps it ui-only); new file `worktree_state.go`, no new package. The path is not computed here: the caller passes it.
- File shape: first line `# capcom ui worktree state: written by capcom ui, not meant for editing (ADR 0015).`, then `ui:` / `  tab: <name>` / `  selected:` / `    ready: <id>` / `    issues: <id>` / `    knowledge: <id>`; empty fields omit their key, an empty mapping is omitted.
- `LoadWorktreeState`: missing file → zero value, no error, nothing created. Malformed YAML or a non-string value → `*Error` with path and line (through the existing `parser`, `eachKey`, `errorAt`), zero value. Unknown keys ignored silently. The tab name is returned as written; validating it is the caller's job.
- `SaveWorktreeState`: `os.MkdirAll` the directory, then the atomic `replaceFile`, which takes its temp-file pattern as a parameter (`.state-*.yaml.tmp` for `state.yaml`, `.ui-state-*.yaml.tmp` here); errors wrapped `save the worktree state file: %w`.

**Regression guard.** `SaveWorktreeState` writes each id double-quoted (`strconv.Quote`) and `LoadWorktreeState` accepts a quoted `!!str` scalar, so valid ids such as `2026`, `null` or `true` (`idPattern`, `internal/tree/ids.go`) load back as strings, not `!!int`, `!!null` or `!!bool`. The invalid-value test rows sit nested under `ui:` (`ui:\n  tab: [a]`, `ui:\n  selected: 3`), laid out as in `TestLoadState_InvalidFile_ErrorsWithLineOnZero`: written top-level they are unknown keys, ignored. The package doc in `userconfig.go` names the git-dir `ui-state.yaml` and `LoadWorktreeState`/`SaveWorktreeState` in its ownership and "only capcom ui" sentences. Prose rule — every Go comment in internal/userconfig saying what capcom ui remembers or which files it keeps (e.g. the State type's doc "State is what capcom ui remembers", the package doc in userconfig.go) names the per-worktree file too; `grep -rnE "remembers|state file|state\.yaml" internal/userconfig` finds them.

**Files:** internal/userconfig/worktree_state.go, internal/userconfig/worktree_state_test.go, internal/userconfig/state.go, internal/userconfig/userconfig.go
**Read first:** internal/userconfig/state.go — replaceFile, encodeState, stateTopLevel; internal/userconfig/userconfig.go — eachKey, errorAt, package doc;
internal/userconfig/state_test.go — TestLoadState_InvalidFile_ErrorsWithLineOnZero; internal/tree/ids.go — idPattern

**Tests:** round trip of the zero value, of all four fields and of ids `2026`, `null` and `true`; missing file → zero, no file created; malformed YAML, `ui:\n  tab: [a]`, `ui:\n  selected: 3` → `*Error` naming path and line; unknown keys ignored; save into a missing directory creates it; replacing an existing file leaves no temp file; existing `state.yaml` tests still pass.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/userconfig/`
- `go vet ./internal/userconfig/`

**Commit:** `feat(userconfig): add the per-worktree capcom ui state file`

## 2. Tell an explicit tab from no argument; keep other commands off the file — ✅ DONE (2026-10-17)

**What:**
**Goal:** `ui.Options` carries `IsTabGiven`, true exactly when `capcom ui` got a tab argument; no command other than `capcom ui` creates `<git-dir>/capcom/ui-state.yaml`.

**Approach (assumed at the header base):**
- `internal/cli/ui.go`: `uiTarget` also returns whether a tab was given; `RunE` sets `Options.IsTabGiven`. `internal/ui/ui.go`: add the field (documented: false means the stored tab may apply). No behaviour change in `internal/ui` yet.
- Extend `TestUserConfig_EveryOtherCommand_LeavesUserConfigDirUntouched` (`internal/cli/userconfig_test.go`) to assert `<root>/.git/capcom/ui-state.yaml` does not exist after its commands.

**Files:** internal/cli/ui.go, internal/cli/ui_test.go, internal/cli/userconfig_test.go, internal/ui/ui.go
**Read first:** internal/cli/ui.go — uiTarget, newUICommand; internal/ui/ui.go — Options, ParseTab; internal/cli/ui_test.go — TestUI_MalformedInvocation_ExitsUsage;
internal/cli/userconfig_test.go — TestUserConfig_EveryOtherCommand_LeavesUserConfigDirUntouched; internal/cli/init_test.go — newGitRepo; internal/index/index.go — Path

**Tests:** `uiTarget` table: no args → Ready, not given; `ready`, `issues`, `knowledge <id>` → given; existing malformed-invocation cases unchanged.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'UI|UserConfig'`
- `go vet ./internal/cli/ ./internal/ui/`

**Commit:** `feat(cli): tell capcom ui whether a tab was given`

## 3. Open on the stored tab and selections

Depends on items 1 and 2.

**What:**
**Goal:** At start `capcom ui` reads `ui-state.yaml` once and opens on its tab (unless `IsTabGiven`) with each tab's stored id selected (the command-line id wins on its tab); a gone or off-tab stored id, or an unknown tab name, falls back silently; an unusable file is one status-bar notice and the defaults.

**Approach (assumed at the header base):**
- `internal/index/index.go`: add exported `Dir(s *store.Store) string` (`<git-dir>/capcom`); `Path` builds on it.
- `internal/ui/ui.go`: `Options.UIStatePath` (empty → `filepath.Join(index.Dir(options.Store), "ui-state.yaml")`), resolved once in `newModel`.
- `newModel` loads it beside `loadUserState`; the stored tab goes through `ParseTab`, an error → Ready, silently. `initialIDFor(tab)` returns the command-line id for the opened tab when one was given, else the stored id for that tab.
- Stored ids load quietly: a flag on `tabRead` (e.g. `isStored`) carried to the loaded message suppresses `selectFailureFormat` and the not-in-tab notices (`notInReadyFormat`, `notInTreeFormat`, `notInKnowledgeFormat`); a command-line id keeps today's notices. Fallback is the existing zero cursor (first row).
- A load error is a notice `ui worktree state: <err> (running on defaults)`, shown through the existing notice slot order in `statusbar.go`.
- `newTestOptions` (`harness_test.go`) sets `UIStatePath` under `t.TempDir()`.

**Regression guard.** An absent file or an absent `tab:` key keeps `options.Tab`; only a present tab name goes through `ParseTab` (an error → Ready), so `newIssuesModel`, `newKnowledgeModel` and the depview, editor, filter and write tests, which set `options.Tab` without `IsTabGiven` over a fresh `UIStatePath`, still open on their tab. Prose rule — every Go comment in internal/ui and internal/index naming the state file(s) or what capcom ui remembers (the Options doc in ui.go naming state.yaml, index.Path's doc) names ui-state.yaml where it now applies; `grep -rnE "state\.yaml|state file|remembers" internal/ui/*.go internal/index/*.go` finds them.

**Files:** internal/index/index.go, internal/ui/ui.go, internal/ui/model.go, internal/ui/refresh.go, internal/ui/statusbar.go, internal/ui/harness_test.go, internal/ui/uistate_test.go
**Read first:** internal/ui/model.go — newModel, loadUserState, initialIDFor; internal/ui/refresh.go — tabRead; internal/ui/ready.go — selectFailureFormat, notInReadyFormat;
internal/ui/statusbar.go — statusBar; internal/ui/harness_test.go — newTestOptions

**Tests:** missing file with `options.Tab` Issues and `IsTabGiven` false → Issues tab; stored `tab: knowledge` + note id → Knowledge tab, that Note selected; `IsTabGiven` Issues + stored Ready tab → Issues, stored Issues id selected; command-line id beats the stored one on its tab; stored id unknown / not in Ready → first row, empty status bar; `tab: bogus` → Ready, no notice; a closed issue stored for Issues is selected; malformed file → notice naming the path, defaults.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `go vet ./internal/ui/ ./internal/index/`

**Commit:** `feat(ui): open on the stored tab and selections`

## 4. Save the tab and selections as they change

Depends on item 3.

**What:**
**Goal:** `ui-state.yaml` holds the open tab and each tab's selected id: a tab change saves at once, a selection change 1 s after the last one, quitting flushes a pending save; a tab whose first read has not applied keeps its stored id; a failed save is one notice per session; two linked worktrees of one clone keep separate files.

**Approach (assumed at the header base):**
- New `internal/ui/uistate.go` owns the snapshot, debounce and saver; `model.go` calls it, no logic duplicated in key or mouse handlers.
- One hook: `Update` compares a snapshot `(tab, readySelection, issuesSelection, knowledgeSelection)` before and after each message; a differing tab returns a save command, a differing id only re-arms a 1 s `tea.Tick` carrying a sequence number (a stale tick saves nothing). Until a tab's first read applies, its snapshot field is the stored id.
- Saves are serialized like `stateSaver` (one mutex, stale requests dropped). `Run` flushes synchronously with the final model returned by `program.Run()` when a save is pending.
- The first failed save sets the notice `cannot save the tab and selection: <err>`; later failures are silent.
- Worktree test: a helper running `git worktree add` in the test repo (mirror `addWorktree` in `internal/cli/lock_test.go`); each worktree's `index.Dir` gets its own file.

**Regression guard.** The debounce goes through an injectable `tickFunc` field (default `tea.Tick`, like `flashTick` in `model.go`) that `startModel` stubs to nil — a real `tea.Tick` blocks `drive` 1 s per selection change; the debounce tests capture the scheduled message themselves and send it. The quit flush is a named model/saver method that `Run` calls with the final model; the quit test presses `q`, then calls that method on the model. The worktree test clears `UIStatePath` (or sets it to `index.Dir(store)+"/ui-state.yaml"`) for both models and asserts the two files under each worktree's `GitDir`; its helper does a commit, `git worktree add`, `store.Init`, `store.Open` and `index.Open` with a `Close` cleanup (the ui package has no `mustRun`/`runGit`). internal/ui/doc.go's file map gains a uistate.go entry and its model.go entry's "state file" wording names both files.

**Files:** internal/ui/uistate.go, internal/ui/model.go, internal/ui/ui.go, internal/ui/uistate_test.go, internal/ui/harness_test.go, internal/ui/doc.go
**Read first:** internal/ui/model.go — Update, stateSaver; internal/ui/ui.go — Run; internal/ui/ready.go — readySelection;
internal/ui/harness_test.go — startModel, drive, noFlashTick; internal/cli/lock_test.go — addWorktree

**Tests:** `2` (tab change) → file holds `tab: issues` after driving the command; `j` then no tick → file unchanged, then the captured tick message sent → new id; two moves within 1 s → one save of the last id; `q` with a pending selection, then the flush method → file holds it; a reload before Knowledge's first read does not blank its stored id; an unwritable path → one notice, a second failure adds none; two worktrees with `UIStatePath` cleared → a file under each worktree's `GitDir`, each with its own tab.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
- `go vet ./internal/ui/`

**Commit:** `feat(ui): remember the tab and selections per worktree`

## 5. ADR 0015 and the store-layout Note

Depends on item 4.

**What:**
**Goal:** A Note `adr-0015` (with children `adr-0015-decision`, `adr-0015-rejected`) under `decisions` records the per-worktree `ui-state.yaml`; `adr-0014` points to it for what lives in the repository's git dir; `store-layout` lists the file; `capcom check` is clean.

**Approach (assumed at the header base):**
- Mirror the shape of `adr-0014` and its children (hand-edited Notes, then `capcom check`). Decision: the file, its keys, who reads and writes it, write times, fallbacks, the once-per-session notice. Rejected: a global file keyed by repository path (grows without bound, lost when a repository moves); folding it into `state.yaml`; writing only on quit (lost on a killed UI).
- `adr-0014` and `adr-0014-decision`: where they say capcom ui's state is never inside a repository, add that the per-worktree tab and selection live under the git dir (ADR 0015). Add the entry to the decisions index Note.
- `store-layout`: name `<git-dir>/capcom/ui-state.yaml` beside `index.db`, and add a `refs` entry for `internal/ui/uistate.go`.

**Regression guard.** Prose rule: every Knowledge sentence saying that what capcom ui remembers goes in `state.yaml`, or that its files stay outside the repository, names ADR 0015 — `grep -rnE "remembers|never inside a repository|outside the repository" .capcom/knowledge` finds them, among them `adr-0011-decision` ("what it remembers between runs ... goes in a machine-written `state.yaml`") and `adr-0014-decision`'s title. The item runs after item 4: until `internal/ui/uistate.go` exists, the `store-layout` ref to it is a broken ref and `capcom check` exits 1.

**Files:** .capcom/knowledge/decisions.md, .capcom/knowledge/decisions/adr-0015.md, .capcom/knowledge/decisions/adr-0015/adr-0015-decision.md, .capcom/knowledge/decisions/adr-0015/adr-0015-rejected.md, .capcom/knowledge/decisions/adr-0014.md, .capcom/knowledge/decisions/adr-0014/adr-0014-decision.md, .capcom/knowledge/decisions/adr-0011/adr-0011-decision.md, .capcom/knowledge/store-layout.md
**Read first:** .capcom/knowledge/decisions/adr-0014/adr-0014-decision.md — The file, Only capcom ui reads and writes it; .capcom/knowledge/decisions/adr-0014.md — summary;
.capcom/knowledge/decisions/adr-0011/adr-0011-decision.md — Only capcom ui reads it; .capcom/knowledge/store-layout.md — refs; .capcom/knowledge/decisions.md — records list;
docs/adr/README.md — new-record shape (ADR 0010); internal/check/notes.go — checkNote

**Tests:** none (docs only).

**Acceptance:**
- `capcom check`
- `capcom know show adr-0015-decision | grep -n "ui-state.yaml"`
- `capcom know show store-layout | grep -n "ui-state.yaml"`
- `capcom know show adr-0014-decision | grep -n "ADR 0015"`

**Commit:** `docs(decisions): record ADR 0015, the per-worktree capcom ui state`

## 6. User docs, help text and generated reference

Depends on item 4.

**What:**
**Goal:** The help text, the generated CLI reference and the user and design docs say that `capcom ui` with no argument reopens on the last tab and selection of this worktree, that arguments win, and where the file lives.

**Approach (assumed at the header base):**
- `internal/cli/ui.go`: `uiLong` and the first `uiExample` line stop saying no argument opens the Ready tab; regenerate `docs/site/reference/cli/capcom_ui.md` with `go run ./cmd/gendocs`.
- Prose rule: every sentence saying no argument opens Ready, that capcom ui keeps its files only outside the repository, or listing what capcom ui remembers, names the new behaviour — `grep -rnE "Ready tab|opens the Ready|outside the repository|state\.yaml|remembers" docs/site docs/design .capcom/knowledge/docs-layout.md internal/cli/ui.go` finds them.
- `docs/site/reference/file-format.md`: a section for `ui-state.yaml` (path, keys, fallbacks) beside "The state file". `docs/site/getting-started/terminal-ui.md` "Start it". `docs/design/tui.md` "Command", the tab-bar bullet, "Architecture" and "Docs".

**Regression guard.** The Acceptance runs `-run 'UI|HelpText'`: `TestHelpText_EveryCommand_SetsLongAndExample` (`internal/cli/help_text_test.go`) pins every `uiExample` line to the prefix `"  capcom ui "`, and `-run UI` does not select it.

**Files:** internal/cli/ui.go, docs/site/reference/cli/capcom_ui.md, docs/site/reference/file-format.md, docs/site/getting-started/terminal-ui.md, docs/design/tui.md, .capcom/knowledge/docs-layout.md
**Read first:** internal/cli/ui.go — uiLong, uiExample; internal/cli/help_text_test.go — TestHelpText_EveryCommand_SetsLongAndExample; internal/clidoc/reference_test.go — TestReferenceUpToDate;
docs/site/reference/file-format.md — The state file; docs/site/getting-started/terminal-ui.md — Start it; docs/design/tui.md — Command, Architecture

**Tests:** `TestReferenceUpToDate` passes after regeneration; `TestHelpText_EveryCommand_SetsLongAndExample` passes with the reworded examples.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/clidoc/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'UI|HelpText'`
- `grep -n "ui-state.yaml" docs/site/reference/file-format.md docs/site/getting-started/terminal-ui.md docs/design/tui.md`
- `capcom check`

**Commit:** `docs(ui): describe reopening on the last tab and selection`
