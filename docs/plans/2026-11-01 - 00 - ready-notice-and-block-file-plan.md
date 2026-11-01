# Plan: ready's hidden-issue notice and a configurable block file

**Goal:** `capcom ready` and `capcom next` say when nothing shows only because ready.hide hides
every Ready issue, and capcom can keep its managed block in a file other than `AGENTS.md`, so a
workspace whose `AGENTS.md` is immutable passes `capcom check`.

**Date:** 2026-11-01
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** a253c5f

**Sources:**
- `.capcom/issues/agent-ergonomics/ready-hidden-notice.md`
- `.capcom/issues/agent-ergonomics/agents-block-file.md`
- ADR 0009 (Notes `adr-0009`, `adr-0009-decision`): store format gating
- `docs/site/agents/managed-block.md`

**Ratified design calls** (Airic Lenz, 2026-11-01):
- **Notice channel:** a result warning (a `warning: ` line on stdout in text mode, the `warnings` array in `--json`); ready's data shape stays unchanged.
- **Notice scope:** both `capcom ready` and `capcom next`; next keeps its nothing-ready exit code.
- **Format:** `agents.file` needs store format 6, as `agents` needs format 4.
- **Reach rule:** with `agents.file` set, `AGENTS.md` must import the block file; the other guides keep reaching `AGENTS.md`.

**Standing requirements:**
- skills: coding-standards
- Tests through `runCLI` (Note `cli-test-harness`); single packages with `GOMEMLIMIT=2GiB`, never `-race` on `./...`.

**Out of scope:**
- A notice in `capcom prime` or `capcom ui`.
- Moving guides (`CLAUDE.md`, `GEMINI.md`) to import the block file directly.
- Raising this repository's own store format.
- `capcom finish`'s next line (it also calls ReadyList).

**Regression check (2026-11-01, a253c5f):**
- 2: guard folded (writer decision: text-mode warnings print on stdout; Notice channel line, Tests and Out of scope corrected)
- 3: guard folded
- 4: recast (writer decision: `Guides` keeps its AGENTS.md rule; item 4 updates the Sync callers so its commit builds)
- 5: recast (writer decision: item 4 passes `Config.Agents.File` at the Sync call sites)
- 6: guard folded
- 2 (re-check): guard folded (writer decision: the `--kv` WARNING= line and its test)
- 4 (re-check): recast (writer decision: `Guides`' has-block gate reads the configured block file, its reach target stays AGENTS.md; internal/agents errors name the block file; supersedes the round-one line for 4)
- 5 (re-check): guard folded (writer decision: Note-write refresh line via node_verbs.go/know_update.go, CLI reference regeneration, stale-comment rule; agents --check's unreached message keeps naming AGENTS.md)
- 5 (writer): takes the orphaned AgentsBlock/AgentsGuide doc comments in internal/diagnose/diagnose.go
- 4: second recast not re-checked (one re-check round only)

## 1. Count the Ready issues ready.hide hides — ✅ DONE (2026-11-01)

NOTES (2026-11-01): the shared private helper is `judgeFiltered`, which returns every todo issue the label and plan filters keep as `ReadyItem`s (Ready ones with their Unblocks); `ReadyList` applies `keeps`, the sort and the limit to it, `ReadyHidden` counts the Ready+IsHidden ones. `ReadyHidden` returns the zero `Hidden` (Labels nil) before reading the index when `IsHiddenIncluded` is set.
NOTES (2026-11-01): the package doc comment in internal/briefing/ready.go now also names ReadyHidden.

**What:**
**Goal:** `internal/briefing` reports, for the same options `ReadyList` takes, how many Ready
issues the ready.hide rule alone leaves out and which ready.hide labels they carry (sorted,
unique), zero when `IsHiddenIncluded` is set.
**Approach (assumed at the header base):** extend `internal/briefing/ready.go`; no new package.
Add `ReadyHidden(x, cfg, options) (Hidden, error)` with `Hidden{Count int; Labels []string}`.
It shares one judging pass with `ReadyList` (extract the selection — index list, label filter,
`issue.Ready` verdicts, `keepsPlan` — into one private helper both call), so the two never
disagree on which issues the other filters keep. A Ready issue counts when `verdict.IsHidden`
and it passes the label and plan filters; `Limit` and `IsNotReadyIncluded` do not change the count.
**Files:** internal/briefing/ready.go, internal/briefing/ready_test.go
**Read first:** internal/briefing/ready.go — ReadyList, ReadyOptions.keeps, ReadyOptions.keepsPlan, ReadyOptions.filter;
internal/issue/ready.go — Ready, Readiness.IsHidden; internal/briefing/ready_test.go — readyFixture, hiddenLabel
**Tests:** table test: no hidden issues → 0; all Ready hidden under two labels → count and both
labels; a hidden issue that is not Ready is not counted; `--label`/plan filters narrow the count;
`IsHiddenIncluded` → 0.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/briefing/` passes; `go vet ./internal/briefing/` clean.
**Commit:** `feat(briefing): count the Ready issues ready.hide hides`

## 2. Warn in ready and next when every Ready issue is hidden

**What:**
**Goal:** when `capcom ready` lists no Ready issue (with `--all`, none of the listed is Ready) and
`ReadyHidden` counts at least one, the result carries exactly one warning
`nothing shown: <n> Ready issues carry a ready.hide label (<label>, <label>); --show-hidden lists them`
(`1 Ready issue carries` for one); `capcom next` with nothing Ready carries the same warning and
keeps its nothing-ready exit code. No warning when nothing is Ready, when a Ready issue is shown,
or with `--show-hidden`. Closes the defect that the silent empty output is byte-identical to
"nothing Ready".
**Approach (assumed at the header base):** in `internal/cli/ready.go` (`newReadyCommand`) append
the warning to `Result.Warnings` beside `droppedDepWarnings`; in `internal/cli/next.go` pass it
into `emitNothingReady`. One formatting function, used by both, in `ready.go`. Update both
commands' Long help with one sentence naming the warning.
**Regression guard.** Factual correction of the ratified notice channel — result warnings render on
stdout in text mode as `warning: <text>` (textRenderer in internal/cli/output/output.go), not stderr.
`capcom next --kv` with every Ready issue hidden prints one `WARNING=nothing shown: ...` line and no STATUS line
(kvRenderer writes warnings first; emitNothingReady passes an empty KV), where stdout is empty today.
**Files:** internal/cli/ready.go, internal/cli/next.go, internal/cli/ready_test.go, internal/cli/next_test.go, docs/site/reference/cli/capcom_ready.md, docs/site/reference/cli/capcom_next.md
**Read first:** internal/cli/ready.go — newReadyCommand, droppedDepWarnings; internal/cli/next.go — emitNothingReady,
nothingReadyMessage; internal/cli/output/output.go — textRenderer.Render, kvRenderer.Render;
internal/cli/ready_test.go — newReadyStore; internal/cli/next_test.go — TestNext_NothingReady_PrintsSoAndExitsNothingReady
**Tests:** via `runCLI` with `ready.hide` configured: all Ready hidden → stdout holds exactly the
`warning: ` line and nothing else; `--json` warnings array holds it; genuinely empty → no warning;
one shown + one hidden → no warning; `--show-hidden` → no warning; `next` all hidden → the
warning and the nothing-ready exit code; `next --kv` all hidden → stdout exactly the one `WARNING=` line, exit
ExitNothingReady (beside TestNext_NothingReadyKV_ExitsNothingReady, which stays green: its fixture has no ready.hide).
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Ready|Next'` passes;
`go run ./cmd/gendocs` leaves `git diff --exit-code docs/site/reference/cli` clean after commit.
**Closes:** ready-hidden-notice
Depends on item 1.
**Commit:** `fix(cli): warn when ready and next show nothing only because every Ready issue is hidden`

## 3. Parse agents.file at store format 6

**What:**
**Goal:** config.yaml accepts `agents.file`, a local slash-separated path relative to the
repository root, defaulting to `AGENTS.md`; it is refused below format 6 with the message shape
the `agents` key uses (`agents.file needs store format 6, but the store declares format <n>: set format: 6`),
refused when it names one of `agents.guides` or escapes the root; `capcom init` writes format 6.
**Approach (assumed at the header base):** in `internal/config`: add `FormatBlockFile = 6`,
raise `FormatVersion` to 6, write `format: 6` in `DefaultFile`, add `Agents.File` (default
`agentsFile`) parsed beside `keyGuides` in `parser.agents`, reusing the path validation of
`parser.guides`. Add a commented `#   file: AGENTS.md` line under the template's `agents:`
comment (format 6). Update `FormatVersion`'s doc comment and the format table in
`docs/site/reference/file-format.md`. Every test pinning `format: 5` as the init default moves to 6.
**Regression guard.** Move TestParse_NewerFormat_FailsAsTooNewWithLine's too-new fixture to `FormatVersion+1` "(up to 6)"
and add `FormatBlockFile` to TestFormatVersion_IsTheNewestNamedFormat's list. Run the file-vs-guides clash once the whole
agents section is read. Store `Agents.File` `path.Clean`ed, any EqualFold match of `AGENTS.md` folded to `AGENTS.md`. Update every doc or
Note stating init's format or each format's additions (`grep -rn 'format: 5\|format 5 add' .capcom/knowledge docs/site`); add an `agents.file` bullet beside `agents.guides` in file-format.md's Config list.
**Files:** internal/config/config.go, internal/config/parse.go, internal/config/config_test.go, internal/config/parse_test.go, docs/site/reference/file-format.md, .capcom/knowledge/store-layout/store-config.md, .capcom/knowledge/glossary/glossary-structure/format-version.md
**Read first:** internal/config/parse.go — parser.agents, parser.guides; internal/config/config.go — FormatVersion, DefaultFile,
Default; internal/config/config_test.go — TestFormatVersion_IsTheNewestNamedFormat, TestParse_NewerFormat_FailsAsTooNewWithLine;
docs/site/reference/file-format.md — Config
**Tests:** parse: default `AGENTS.md`; set at format 6; refused at format 5 with the exact
message; refused naming a guide (any case); refused `../x.md` and an absolute path; `file: CLAUDE.md` placed before
`guides: []` accepted; `agents.md` and `./AGENTS.md` stored as `AGENTS.md`; the too-new fixture at `FormatVersion+1`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/config/` passes; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Init'` passes.
**Commit:** `feat(config): add agents.file at store format 6`

## 4. Keep the managed block in the configured file

**What:**
Recast at the regression check (2026-11-01).
**Goal:** `internal/agents` reads and writes the managed block in a block file its callers name
(a path relative to the worktree root, in slash form); when that file is not `AGENTS.md`,
`Guides`-style inspection also reports whether `AGENTS.md` reaches it by an `@<rel>` import line,
and ModeWrite appends that line to an `AGENTS.md` lacking it. ModeWrite creates a missing block
file, and its missing directories, holding the block alone.
**Approach (assumed at the header base):** thread a `blockFile string` parameter through `Sync`,
`reconcile`, `inspect` and `write` (replacing `guidePath`/`FileName` uses; `FileName`
stays as the default's name). `Guides` takes `blockFile` for its has-block gate only (see the regression guard).
Add `AgentsGuide(worktreeRoot, blockFile, mode) (Guide, error)` reusing the import-line logic of
`Guides` for `AGENTS.md`, returning nothing to report when `blockFile` is `AGENTS.md`.
**Regression guard.** Guides' block-present gate inspects the configured block file (Guides takes blockFile for that gate only), so CLAUDE.md and GEMINI.md stay checked when agents.file is set; their reach target stays AGENTS.md and Guide.Problem still names AGENTS.md. Every internal/agents error or message naming the block's home names blockFile, in this item.
Concretely: findBlockHome reads markers from blockFile while home.target/rootPath stay the root AGENTS.md; findBlock/markerError,
outsideError and reconcile's `%s: %w` wraps name blockFile. When blockFile is not AGENTS.md, AgentsGuide also reports capcom markers
left in AGENTS.md, so item 5 can flag them with a hint to remove them. Item 4 updates every caller of the changed signatures so its
commit builds, passing Config.Agents.File at the Sync and Guides calls in internal/cli/agents.go and internal/check/agents.go and
agents.FileName at internal/check/agents_test.go's writeCurrentBlock.
**Files:** internal/agents/block.go, internal/agents/guides.go, internal/agents/block_test.go, internal/agents/guides_test.go, internal/cli/agents.go, internal/check/agents.go, internal/check/agents_test.go
**Read first:** internal/agents/guides.go — Guides, findBlockHome, reachBlock; internal/agents/block.go — Sync,
reconcile, markerError, outsideError; internal/check/agents_test.go — writeCurrentBlock
**Tests:** block written to and read from `.capcom/AGENTS-capcom.md`; `AGENTS.md` untouched by a
Refresh; AGENTS.md without the import → unreached, ModeWrite appends `@.capcom/AGENTS-capcom.md`;
a missing `.capcom/AGENTS-capcom.md` (and directory) created holding the block alone; block in the `.capcom` file, CLAUDE.md
without `@AGENTS.md` → GuideUnreached with `@AGENTS.md`, with it → GuideReaches and untouched by ModeWrite; tangled markers
and an outside link in the block file name `.capcom/AGENTS-capcom.md`; capcom markers left in AGENTS.md reported by AgentsGuide;
existing `Sync` and `Guides` callers in tests pass `FileName` with their assertions unchanged.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/agents/` passes; `go vet ./internal/agents/` clean; `go build ./... && go vet ./internal/cli/ ./internal/check/` clean.
Depends on item 3.
**Commit:** `feat(agents): keep the managed block in a named file`

## 5. Use agents.file in agents, the Note writes and check

**What:**
Recast at the regression check (2026-11-01).
**Goal:** `capcom agents`, `capcom agents --check`, every Note write that refreshes the block and
`capcom check` use `agents.file`; every message naming the block's home names the configured
file; `capcom check` reports an `AGENTS.md` that does not import a non-default block file. With
`agents.file` set and `AGENTS.md` in a directory capcom may not write, `capcom know add` of a
top-level Note prints no warning and `capcom check` exits 0.
**Approach (assumed at the header base):** item 4 already passes `Config.Agents.File` at the
Sync call sites; replace `agents.FileName` in output text and diagnostics (`agents.go`, `know_add.go`
`addSyncEffects`, `init.go` `agentsHint`, `check/agents.go`) with the configured file; add the
AGENTS.md reach result to both `capcom agents` paths and to check under `diagnose.AgentsGuide`.
**Regression guard.** Item 4 already passes Config.Agents.File at the Sync call sites; item 5 owns the messages naming the
configured file, reporting the AGENTS.md reach result in capcom agents, agents --check and capcom check, and the CLI tests.
addSyncEffects has no store: give noteVerbs' afterWrite (node_verbs.go) and know_update.go's call the file name (afterWrite takes
*app, or it rides on nodestore.Effects from app.syncAgents), so know update/mv/rm print the configured file's refresh line too.
agents --check's unreached-guide message (agents.go, `does not reach the capcom block in`) keeps naming agents.FileName, the
guides' reach target, matching Guide.Problem; only messages about the block itself name the configured file. check and
agents --check flag the capcom markers AgentsGuide reports left in AGENTS.md, hinting to remove them. Item 5 changes only
internal/cli, internal/check and the AgentsBlock/AgentsGuide doc comments in internal/diagnose/diagnose.go
(internal/agents messages are item 4's). Doc ownership: item 6 owns agentsLong/agentsExample and
capcom_agents.md; item 5 rewords the know help naming AGENTS.md as the block's home (know_add.go knowAddLong, know.go knowLong)
and regenerates every other generated CLI reference file its help-text changes touch, in its own commit. Every comment naming
AGENTS.md as where the block lives is reworded (`grep -rn 'AGENTS.md' internal/cli/agents.go internal/cli/know_add.go internal/cli/node_verbs.go internal/check internal/diagnose`).
**Files:** internal/cli/agents.go, internal/cli/know_add.go, internal/cli/know.go, internal/cli/node_verbs.go, internal/cli/know_update.go, internal/cli/init.go, internal/check/agents.go, internal/check/check.go, internal/cli/agents_test.go, internal/cli/know_test.go, internal/cli/know_update_test.go, internal/cli/know_mv_test.go, internal/cli/know_rm_test.go, internal/check/agents_test.go, internal/diagnose/diagnose.go, docs/site/reference/cli/capcom_know.md, docs/site/reference/cli/capcom_know_add.md
**Read first:** internal/cli/agents.go — syncAgentsBlock, checkAgentsBlock, agentsData; internal/cli/know_add.go — app.syncAgents,
addSyncEffects; internal/cli/node_verbs.go — nodeVerbs.afterWrite; internal/check/agents.go — checker.agentsBlock, checker.agentsGuides
**Tests:** via `runCLI`: format 6, `agents.file: .capcom/AGENTS-capcom.md`, `AGENTS.md` holding
the import: `capcom agents` writes the block there and prints its path; `know add` refreshes it
and prints `refreshed: .capcom/AGENTS-capcom.md`; `know update`, `know mv` and `know rm` print the
same refresh line; `check` exits 0; AGENTS.md without the import → check exits 1 naming it; CLAUDE.md
without `@AGENTS.md` → `agents --check` names AGENTS.md in its message; capcom markers left in AGENTS.md
→ check and `agents --check` flag them with the remove hint. Unix only (skip on Windows and as root): repo root chmod 555 after
setup → `know add` no warning, `check` exits 0.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Agents|Know|Init'`, `GOMEMLIMIT=2GiB go test -count=1 ./internal/check/` and `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/` pass; `go run ./cmd/gendocs` leaves `git diff --exit-code docs/site/reference/cli` clean after commit.
Depends on item 4.
**Commit:** `feat(agents): honour agents.file in capcom agents, Note writes and check`

## 6. Document agents.file

**What:**
**Goal:** the user docs say how to keep the block outside `AGENTS.md`: the key, its format,
the `@<file>` import AGENTS.md needs, and the immutable-AGENTS.md use case; the generated CLI
reference matches the help text.
**Approach (assumed at the header base):** a section in `docs/site/agents/managed-block.md`, a
line in `docs/site/agents/agent-setup.md`; regenerate with `go run ./cmd/gendocs`.
**Regression guard.** Every docs/site page listing the agents keys or naming AGENTS.md as the block's home
(`grep -rn 'agents.guides\|Managed block' docs/site`) gains agents.file: file-format.md's Agent guides section and quickstart.md
included (item 3 takes file-format.md's Config list). No other item rewords `agentsLong`/`agentsExample` (internal/cli/agents.go):
item 6 rewords them to name agents.file and regenerates capcom_agents.md in the same commit.
**Files:** docs/site/agents/managed-block.md, docs/site/agents/agent-setup.md, docs/site/reference/cli/capcom_agents.md, docs/site/reference/file-format.md, docs/site/getting-started/quickstart.md, internal/cli/agents.go
**Read first:** docs/site/agents/managed-block.md — agents.guides section; docs/site/agents/agent-setup.md — agents.guides line;
docs/site/reference/file-format.md — Config, Agent guides; internal/cli/agents.go — agentsLong, agentsExample;
internal/clidoc/reference_test.go — TestReferenceUpToDate
**Tests:** none beyond `TestReferenceUpToDate` (internal/clidoc), which pins the regenerated page.
**Acceptance:** `go run ./cmd/gendocs && git diff --exit-code docs/site/reference/cli` after commit; `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/` passes; `grep -n 'agents.file' docs/site/agents/managed-block.md` matches.
**Closes:** agents-block-file
Depends on item 5.
**Commit:** `docs(agents): document agents.file`
