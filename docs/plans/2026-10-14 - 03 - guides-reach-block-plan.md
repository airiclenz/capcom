# Agent guides reach the Managed block plan

**Goal:** Every agent guide a repository keeps beside AGENTS.md (CLAUDE.md, GEMINI.md by default)
reaches the Managed block, by a link to AGENTS.md or an `@AGENTS.md` import line. `capcom agents`
adds the import, and `capcom agents --check` and `capcom check` report a guide that does not reach it.
**Date:** 2026-10-14
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 11eeaa4
**Closes:** guides-reach-block

**Regression check (2026-10-14, 11eeaa4):**
- 1: guard folded (decision 1; cli `NewerStoreFormat` run, config_test too-new case, format-prose rule)
- 2: recast (decision 2: silent on AGENTS.md faults, owns the unreached-guide message; `guidePath` kept, guide-named errors, any-CRLF rule)
- 2: guard folded (decision 2, second round: import write via `nodefile.ReplaceFileVerbatim`, never creates; `reconcile` passes `FileName`, block tests unchanged)
- 2: guard folded (decision 2, correction: regression-3 G: lines kept; guides deduplicated by resolved target; `<rel>` from the resolved target's directory; Approach names `ReplaceFileVerbatim`)
- 3: guard folded (decision 3; Guides only after a current or stale block; unreadable and subdirectory guide tests)
- 4: guard folded (decision 4; first unreached guide named; init console blocks; outside-guide test)
- 5: guard folded (rule widened to `capcom agents` output and commit lines; Acceptance greps)

**Sources:**
- `.capcom/issues/guides-reach-block.md`
- `.capcom/knowledge/decisions/adr-0009/adr-0009-decision.md` (any new config key raises the format)
- `.capcom/knowledge/glossary/glossary-agent-interface/managed-block.md`

**Ratified design calls** (user, 2026-10-14):
- **Single home:** the block lives only in AGENTS.md; other guides import or link it, never copy it.
- **Command name:** keep `capcom agents`; no per-host command such as `capcom claude`.
- **Import spot:** the import line is appended at the end of the guide.
- **Severity:** a guide that does not reach the block is a `capcom check` problem, not a warning.
- **Check scope:** only when AGENTS.md carries the block; no block means no guide problem.
- **Config:** `agents.guides` list, default `[CLAUDE.md, GEMINI.md]`; `guides: []` turns the rule off.
- **Format:** the `agents` key raises FormatVersion to 4; `agents:` in a store declaring format < 4 is an error.
- **init:** `capcom init` writes no guide; its output gains one hint line naming `capcom agents`.

**Standing requirements:**
- skills: coding-standards
- Raspberry Pi host: run single packages with `GOMEMLIMIT=2GiB`; never `-race` on `./...`; never two test commands at once.
- Use the built binary (`go run ./cmd/capcom ...`), never the installed one.

**Out of scope:**
- Creating a CLAUDE.md or GEMINI.md that does not exist.
- Guides other hosts read natively as AGENTS.md (Cursor, Codex, Copilot); `.cursor/rules`, `.clinerules`.
- The `ModeRefresh` path that Note writes run (`syncAgents`): it never touches guides.
- Raising an existing store's format automatically when `agents:` is added by hand.

## 1. config.yaml gains `agents.guides` at format 4 — ✅ DONE (2026-10-14)
NOTES (2026-10-14): the format gate runs in the `agents` case of topLevel, not after the whole mapping is read: format is parsed before every other key, so config.Format is already known there and the error lands at the `agents` key line whatever the section holds.
NOTES (2026-10-14): AGENTS.md is refused in any letter case and after path.Clean (`./AGENTS.md`, `agents.md`), duplicates are compared after path.Clean, and an entry holding a backslash is refused so a guide names the same file on every platform.
NOTES (2026-10-14): internal/config/format_test.go and internal/cli/prime_test.go needed no change: format_test raises to an explicit 3 and prime_test reaches the too-new fixture only through writeNewerFormatConfig/newerFormatMessage in read_test.go.

**What:**
**Goal:** `config.Config` carries `Agents.Guides []string`, default `["CLAUDE.md", "GEMINI.md"]`
when the key is absent. `agents: {guides: [...]}` parses in a store declaring format ≥ 4 and fails
with a `*config.Error` at the `agents` key line in a store declaring less, the message naming
`format: 4` as the fix. `config.FormatVersion` is 4, and `DefaultFile` declares `format: 4`.
**Approach (assumed at the header base):** In `internal/config/config.go` add the `Agents` section
struct and key constants beside `Ready`; `Default()` sets the two guides. In `parse.go`, `topLevel`
gains an `agents` case parsed like `p.ready` (`eachKey`, unknown subkey rejected, `nameList` for
`guides`, empty list allowed). Each guide must be a local slash-separated path (`filepath.IsLocal`
after `filepath.FromSlash`), not `AGENTS.md`, without duplicates; a violation is a `*config.Error`
at its line. The format gate compares the parsed `format` against 4 after the whole mapping is read.
`templateBody` is shared with `KeylessFile` (format 1), so the template documents the section only
as comment lines (`# agents:` / `#   guides: [CLAUDE.md, GEMINI.md]`) with a one-line comment on
what a guide is; never a real key. Update the `## Config` section of
`docs/site/reference/file-format.md` (YAML block, bullet, format list gains format 4) and the
config keys in `.capcom/knowledge/store-layout/store-config.md`.
**Regression guard.** Raising FormatVersion to 4 reaches internal/cli tests that pin the format (read_test, prime_test, init_test and any other grep for `format: 3` / FormatVersion in internal/cli and internal/store tests); item 1 updates them, lists them on **Files:**, and its Acceptance adds `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestRead|TestPrime|TestInit'`.
The read_test.go too-new tests are `TestList_`/`TestCreate_`/`TestCheck_NewerStoreFormat`, so Acceptance also runs `-run 'NewerStoreFormat'`; `newerFormatMessage`/`writeNewerFormatConfig` and `TestParse_NewerFormat_FailsAsTooNewWithLine` move to format 5 and "(up to 4)".
Every sentence listing what each store format adds or which config keys exist names format 4 and `agents.guides`: `grep -rn 'format 3\|elsewhere.days' .capcom/knowledge docs/site internal/config` (the glossary Note format-version.md, the config.go package and `FormatVersion` comments).
**Files:** internal/config/config.go; internal/config/parse.go; internal/config/config_test.go; internal/config/format_test.go; docs/site/reference/file-format.md; .capcom/knowledge/store-layout/store-config.md; .capcom/knowledge/glossary/glossary-structure/format-version.md; internal/cli/read_test.go; internal/cli/prime_test.go; internal/cli/init_test.go
**Read first:** internal/config/parse.go — parser.topLevel, parser.ready, parser.nameList; internal/config/config.go — FormatVersion, templateBody;
internal/config/config_test.go — TestParse_NewerFormat_FailsAsTooNewWithLine; internal/cli/read_test.go — newerFormatMessage; internal/cli/init_test.go — TestInit_WrittenConfig_DeclaresCurrentFormat

**Tests:**
- Absent key → default guides; `guides: []` → empty; `guides: [docs/AI.md]` accepted.
- `agents:` under `format: 3` fails at the key line naming `format: 4`; unknown subkey, `AGENTS.md`, `../x.md`, duplicate fail.
- `DefaultFile` and `KeylessFile` still parse; `DefaultFile` declares format 4.
- `TestParse_NewerFormat_FailsAsTooNewWithLine` and the cli too-new fixture move to format 5, "(up to 4)"; `TestInit_WrittenConfig_DeclaresCurrentFormat` builds its expected line from `config.FormatVersion`.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/config/ ./internal/store/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestRead|TestPrime|TestInit'`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'NewerStoreFormat'`
- `go run ./cmd/capcom check` (this repo stays format 3, no problems)

**Commit:** `feat(config): agents.guides lists the agent guides that must reach the Managed block`

## 2. agents.Guides inspects and imports the block into agent guides — ✅ DONE (2026-10-14)
NOTES (2026-10-14): the unreached-guide message is built once by `Guide.Problem()` (empty for any state but GuideUnreached); `Guide` carries an `ImportLine` field (`@AGENTS.md` / `@../AGENTS.md`) so the method can name it. Items 3 and 4 prefix `Guide.Path` themselves.
NOTES (2026-10-14): `guidePath(root)` kept for AGENTS.md and now delegates to the new `namedGuidePath(root, name)`; the guide's name travels on `guideFile.name`, which `readGuide`/`unreadableError` use, so block tests are unchanged.
NOTES (2026-10-14): the import line is appended by reusing `splice(content, blockSpan{}, importLine)`, which already implements the append rule (CRLF when any CRLF, missing final line ending added, blank line before on a non-empty file).

**What:** Recast at the regression check (2026-10-14).
**Goal:** `agents.Guides(worktreeRoot string, names []string, mode Mode) ([]Guide, error)` reports
each named guide as `GuideMissing`, `GuideReaches`, `GuideUnreached`, `GuideImported` or
`GuideOutside`, and returns nil when AGENTS.md carries no begin marker. A guide reaches the block
when its resolved path equals AGENTS.md's resolved path, or a line, blanks trimmed, is
`@<rel>` or `@./<rel>`, `<rel>` being the root AGENTS.md relative to the directory of the guide's
resolved target (the file the line is written to) in slash form.
`ModeWrite` appends `@<rel>` to each unreached guide; no other mode writes.
**Approach (assumed at the header base):** New file `internal/agents/guides.go`; `Guide{Path, State}`
with `Path` worktree-relative in slash form. Generalise `resolveGuide`/`guidePath` to take the
guide's name so both files resolve one way; the block's begin marker is found with `findBlock` on
AGENTS.md's content. One deep function behind `Guides`; callers never see resolution. Append rule:
keep the file's line ending (CRLF when the content holds any CRLF, as `splice` does); content not
ending in a line ending gets one, a non-empty file gets one blank line before the import; write via
`nodefile.ReplaceFileVerbatim` to the resolved target, keeping its permissions (never
`WriteFileAtomicVerbatim`, which creates a missing file). A guide linking
outside the tree is `GuideOutside`: never read, never written, never an error. An unreadable guide
(not regular, over 1 MiB) is an `*Error` whose `Path` is the guide's name. A missing or dangling
guide is `GuideMissing` and is never created.
**Regression guard.** Guides stays silent (returns nil, nil) whenever AGENTS.md is missing, links outside the tree, is unreadable, or has tangled markers — the block inspection already reports AGENTS.md, so a Guides error only ever names a guide file. Item 2 also owns the unreached-guide message: an exported func or method in internal/agents (e.g. `Guide.Problem()` / `UnreachedMessage(rel)`) builds `does not reach the capcom block in AGENTS.md; run capcom agents to add @<rel>` once, and items 3 and 4 use it rather than spelling the text.
block_test.go calls `guidePath(root)` ~20 times: keep it for AGENTS.md beside a name-taking sibling, or update those callers. `readGuide`, `unreadableError` and the resolve-error wrap in `reconcile` take the guide's name, so a FIFO CLAUDE.md fails with `Path == "CLAUDE.md"`.
The import write never creates a file: use nodefile.ReplaceFileVerbatim (or an equivalent that fails rather than creates when the target is gone), not WriteFileAtomicVerbatim; reconcile keeps passing FileName to the generalised resolveGuide/readGuide so existing block tests stay unchanged.
Guides sharing one resolved target are deduplicated: that target is read and written at most once per run (each name still gets its `Guide`), so GEMINI.md -> CLAUDE.md, both unreached, gains exactly one import line. `<rel>` is taken from the resolved target's directory, never the name's: `docs/AI.md` -> `../CLAUDE.md` writes `@AGENTS.md` into CLAUDE.md, not `@../AGENTS.md`.
**Files:** internal/agents/guides.go; internal/agents/guides_test.go; internal/agents/block.go
**Read first:** internal/agents/block.go — reconcile, resolveGuide, readGuide, findBlock, splice, isInsideTree;
internal/nodefile/atomic.go — ReplaceFileVerbatim; internal/agents/fifo_unix_test.go — makeFIFO

**Tests:**
- Import line present (`@AGENTS.md`, `@./AGENTS.md`, indented) → reaches; CLAUDE.md symlink to AGENTS.md and AGENTS.md symlink to CLAUDE.md → reaches, file byte-identical.
- `ModeWrite` on unreached CRLF / no-final-newline / empty guide → exact bytes; perm kept; second run byte-identical.
- No block in AGENTS.md → nil; missing guide not created; outside link untouched; FIFO CLAUDE.md → `*Error` whose `Path` is `CLAUDE.md`.
- AGENTS.md missing, linked outside the tree, unreadable or with tangled markers → `nil, nil`.
- The unreached-guide message builder gives the exact wording for `@AGENTS.md` and `@../AGENTS.md`.
- Guide in a subdirectory (`docs/AI.md`) imports `@../AGENTS.md`.
- GEMINI.md symlink to CLAUDE.md, both unreached, `ModeWrite` → CLAUDE.md holds exactly one import line.
- `docs/AI.md` symlink to `../CLAUDE.md`, unreached, `ModeWrite` → CLAUDE.md gains `@AGENTS.md` once, no `@../AGENTS.md`.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/agents/`

**Commit:** `feat(agents): agent guides reach the Managed block through an import line`

## 3. capcom check reports a guide that does not reach the block — ✅ DONE (2026-10-14)
NOTES (2026-10-14): the guide check is a separate `checker.agentsGuides` called from `agentsBlock` only after a current or stale block; the store's already-loaded `c.store.Config.Agents.Guides` is the list. Only an `*agents.Error` from `agents.Guides` becomes a problem; any other error (a guide that cannot be resolved or read) fails the run, as it does for AGENTS.md.
NOTES (2026-10-14): `docs/site/reference/file-format.md` still names only `agents-block` among the AGENTS.md check problems; item 5 owns that file's guide-rule wording.

Depends on items 1 and 2.

**What:**
**Goal:** `capcom check` reports each `GuideUnreached` guide as a problem with code `agents-guide`
at the guide's path, line 0, message `does not reach the capcom block in AGENTS.md; run capcom
agents to add @AGENTS.md` (the `<rel>` spelling for a guide in a subdirectory); an unreadable
guide is an `agents-guide` problem with the `*agents.Error` message.
**Approach (assumed at the header base):** Add `AgentsGuide Kind = "agents-guide"` in
`internal/diagnose/diagnose.go` and to both Kind pin maps (`diagnose_test.go`, `check_test.go`).
`checker.agentsBlock` in `internal/check/agents.go` calls `agents.Guides(..., cfg.Agents.Guides,
agents.ModeInspect)` after the block inspection, using the config `Run` already loads. Update the
`internal/check/check.go` package comment.
**Regression guard.** use the item-2 message builder, never a literal copy of the wording.
Call `agents.Guides` only when `agents.Sync` returned no error and `status.State` is `StateCurrent` or `StateStale` (keep the early return at internal/check/agents.go:14-16): an AGENTS.md linking outside the tree, or with tangled or unreadable markers, gets no second, agents-guide verdict.
**Files:** internal/diagnose/diagnose.go; internal/diagnose/diagnose_test.go; internal/check/agents.go; internal/check/agents_test.go; internal/check/check.go; internal/check/check_test.go; internal/cli/check_test.go
**Read first:** internal/check/agents.go — checker.agentsBlock; internal/check/check.go — Run; internal/diagnose/diagnose.go — AgentsBlock;
internal/check/agents_test.go — writeGuide, codedProblems, runCheckWithin; internal/check/check_test.go — TestCodes_ClosedSet_PinsEveryCodeString;
internal/briefing/briefing.go — readHealth

**Tests:**
- `check/agents_test.go`: block + unreached CLAUDE.md → `agents-guide CLAUDE.md: …`; no block → none; `guides: []` (format 4) → none.
- `check/agents_test.go`: outside-linked AGENTS.md + unreached CLAUDE.md → none; FIFO or directory CLAUDE.md → `agents-guide CLAUDE.md: not a regular file`; format-4 `guides: [docs/AI.md]` → a message naming `@../AGENTS.md`.
- `cli/check_test.go`: exact stdout line and non-zero exit for an unreached CLAUDE.md.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/diagnose/ ./internal/check/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestCheck|TestPrime'`

**Commit:** `feat(check): report an agent guide that does not reach the Managed block`

## 4. capcom agents imports and checks the guides; init hints at it

Depends on items 1 and 2.

**What:**
**Goal:** `capcom agents` writes the block, then appends the import to each unreached guide,
printing `added @AGENTS.md to CLAUDE.md` per guide and `skipped CLAUDE.md: links outside the
working tree` for an outside one. `capcom agents --check` exits 1 with
`capcom: CLAUDE.md: does not reach the capcom block in AGENTS.md; run capcom agents to add @AGENTS.md`
when the block is current but a guide is unreached. `--json` data gains `guides: [{path, state}]`.
`capcom init` output gains the line `run capcom agents to write the capcom block into AGENTS.md`.
**Approach (assumed at the header base):** In `internal/cli/agents.go`, `writeAgentsBlock` calls
`agents.Guides(..., ModeWrite)` after `agents.Sync`; `checkAgentsBlock` calls it with
`ModeInspect`, a stale block still reported first. `agentsData` gains `Guides []agentsGuideData`
(`path`, `state` as `missing|reaches|unreached|imported|outside`, `omitempty`). Update
`agentsLong`/`agentsExample`. `internal/cli/init.go` `initText` gains the hint line. Regenerate
`docs/site/reference/cli/` with `go run ./cmd/gendocs`.
**Regression guard.** use the item-2 message builder, never a literal copy of the wording.
`agents --check` returns one error, naming the first unreached guide in `agents.guides` order. Every console block quoting `capcom init` output gains the hint line, found by `grep -rn 'initialized capcom store' docs/site README.md`.
**Files:** internal/cli/agents.go; internal/cli/agents_test.go; internal/cli/init.go; internal/cli/init_test.go; docs/site/reference/cli/capcom_agents.md; docs/site/reference/cli/capcom_init.md; docs/site/getting-started/quickstart.md; docs/site/agents/agent-setup.md
**Read first:** internal/cli/agents.go — writeAgentsBlock, checkAgentsBlock, agentsData, agentsFailure; internal/cli/init.go — initText;
internal/cli/agents_test.go — writeAgentsGuide, TestAgents_GuideLinkedOutsideTheTree_FailsNamingTheLink; internal/clidoc/reference_test.go — TestReferenceUpToDate

**Tests:**
- `capcom agents` on block-less AGENTS.md + CLAUDE.md without import → both written, exact text lines; re-run → CLAUDE.md byte-identical.
- `capcom agents --check` exact stderr and exit 1 for unreached guide; exit 0 when guide reaches or block absent; `--json` guides array.
- `capcom init` output contains the hint line.
- CLAUDE.md and GEMINI.md both unreached → `--check` names CLAUDE.md; CLAUDE.md symlinked outside the tree → `skipped CLAUDE.md: links outside the working tree`, exit 0, outside file untouched (pattern: `TestAgents_GuideLinkedOutsideTheTree_FailsNamingTheLink`).

**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestAgents|TestInit|TestHelp'`
- `GOMEMLIMIT=2GiB go test ./internal/clidoc/`

**Commit:** `feat(agents): capcom agents adds @AGENTS.md to agent guides that miss the block`

## 5. Docs and glossary describe the guide rule

Depends on items 3 and 4.

**What:**
**Goal:** The Managed block glossary Note, `docs/site/agents/managed-block.md`,
`docs/site/agents/agent-setup.md` and the "The Managed block in AGENTS.md" section of
`docs/site/reference/file-format.md` state that AGENTS.md is the block's only home, that
`agents.guides` names the guides that must reach it, how a guide reaches it, and what
`capcom agents` and `capcom check` do about one that does not.
**Approach (assumed at the header base):** Edit the glossary Note with `capcom know update`
(or by hand, then `go run ./cmd/capcom check`); the Note's title becomes "Managed block: the
marked section of AGENTS.md that capcom writes and every agent guide reaches". A new
"Other agent guides" section in managed-block.md with example output from item 4. Every sentence
that says the block is only in AGENTS.md or that CLAUDE.md is the user's job must match:
`grep -rn 'CLAUDE.md\|GEMINI.md' docs/site README.md .capcom/knowledge`.
**Regression guard.** The rule also covers every site that quotes `capcom agents` output or commits after it, found by `grep -rn 'capcom agents\|git add .capcom AGENTS.md' docs/site README.md` with docs/site/reference/cli excluded (quickstart.md "2. Tell your agents" and its commit line, index.md:36, README.md:141): the commit lines take the edited guides too.
**Files:** .capcom/knowledge/glossary/glossary-agent-interface/managed-block.md; docs/site/agents/managed-block.md; docs/site/agents/agent-setup.md; docs/site/reference/file-format.md; README.md; docs/site/getting-started/quickstart.md; docs/site/index.md
**Read first:** .capcom/knowledge/glossary/glossary-agent-interface/managed-block.md — title, body; docs/site/agents/managed-block.md — "AGENTS.md as a link", --check paragraph;
docs/site/agents/agent-setup.md — CLAUDE.md @AGENTS.md paragraph, init console block; docs/site/reference/file-format.md — "The Managed block in AGENTS.md" bullets;
docs/site/getting-started/quickstart.md — "2. Tell your agents", commit line

**Tests:**
- none (docs); the managed block refresh is checked by `capcom agents --check`.

**Acceptance:**
- `go run ./cmd/capcom agents --check`
- `go run ./cmd/capcom check`
- `grep -l 'agents.guides'` and `grep -l '@AGENTS.md'` each list all four of .capcom/knowledge/glossary/glossary-agent-interface/managed-block.md, docs/site/agents/managed-block.md, docs/site/agents/agent-setup.md, docs/site/reference/file-format.md

**Commit:** `docs: agent guides reach the Managed block through AGENTS.md`
