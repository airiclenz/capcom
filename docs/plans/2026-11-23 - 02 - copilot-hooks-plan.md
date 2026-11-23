# capcom init --copilot-hooks

**Goal:** a GitHub Copilot session (Copilot CLI, VS Code agent mode, the cloud agent) starts with
the capcom Briefing: `capcom prime --additional-context` prints it as Copilot's `sessionStart` JSON,
and `capcom init --copilot-hooks` wires that command into `.github/hooks/capcom.json`.
**Date:** 2026-11-23
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 35858d1
**Closes:** copilot-hooks

**Sources:**
- `.capcom/issues/copilot-hooks.md`
- https://docs.github.com/en/copilot/reference/hooks-reference (file shape, `sessionStart` output, `command` field)
- https://code.visualstudio.com/docs/agent-customization/hooks; https://github.com/github/copilot-cli/blob/main/changelog.md (1.0.11, 1.0.22)
- `docs/plans/archived/2026-10-20 - 00 - apogee-hooks-plan.md` (the precedent flag)
- Notes `cli-test-harness`, `error-messages`, `coding-conventions`, `docs-layout`

**Ratified design calls:**
- **Prime output:** a new `capcom prime --additional-context` prints `{"additionalContext": "<text Briefing>"}`, named for the hook field, not the agent (the user, 2026-11-23).
- **Hook file:** `.github/hooks/capcom.json`, read by Copilot CLI, VS Code and the cloud agent; merged when present (the user, 2026-11-23).
- **Double run:** a repo with `--claude-hooks` also runs plain `capcom prime` under Copilot (its text is dropped); accepted and documented, no detection (the user, 2026-11-23).
- **Cloud agent:** docs only on installing capcom in `copilot-setup-steps.yml`; init writes no workflow (the user, 2026-11-23).
- **Events:** only `sessionStart`; Copilot's `preCompact` output never reaches the model and it has no post-compaction event (writer, from the hooks reference).
- **Minimum:** GitHub Copilot CLI 1.0.22, the first to fire `sessionStart` once per session (writer, from the CLI changelog).

**Standing requirements:**
- skills: coding-standards
- CLI tests go through `runCLI`, never `t.Parallel()` (Note `cli-test-harness`).
- Tests on this host: `GOMEMLIMIT=2GiB`, one package at a time, never `./...` with `-race`.
- A help-text or flag change regenerates the committed reference: `go run ./cmd/gendocs`.

**Out of scope:**
- Writing `.github/workflows/copilot-setup-steps.yml`, `.github/copilot/settings.json` or any user-level Copilot file.
- Any change to the Claude Code, Codex or apogee hooks, or to plain `capcom init` output.
- `preCompact` or any Copilot event other than `sessionStart`.

**Regression check (2026-11-23, 35858d1):**
- 1: recast; yields to `internal/cli/root.go` `rootLong`
- 2: guard folded
- 3: recast
- 4: guard folded
- 1 (re-check): guard folded; yields to `internal/cli/root.go` `withKV` doc comment
- 3 (re-check): guard folded

## 1. capcom prime --additional-context — ✅ DONE (2026-11-23)

NOTES (2026-11-23): the text is rendered by swapping app.renderer for a text renderer into a buffer and calling app.emit itself (writeAdditionalContext), so emit's warnings are added by emit rather than copied; primeLong's new paragraph names the JSON as "the additionalContext string of one JSON object" instead of spelling `{"additionalContext": "<text>"}`, because gendocs copies Long into the Markdown reference as prose, where `<text>` would read as an HTML tag.

**What:** Recast at the regression check (2026-11-23).
**Goal:** `capcom prime --additional-context` writes exactly one JSON object and a line break to
stdout, `{"additionalContext": "<s>"}`, where `<s>` equals byte for byte what plain `capcom prime`
writes to stdout in the same state (warning lines, Briefing, elsewhere lines); it exits 0 whenever
plain prime does. `--additional-context --json` is a usage error (exit 2) naming both flags.

**Approach (assumed at the header base):** extends `internal/cli/prime.go`; no new package and no
new `output.Format`. `newPrimeCommand` gains the bool flag `additional-context` (help: "print the
Briefing as the JSON a GitHub Copilot sessionStart hook reads"). With it set, `primeResult`'s result
is rendered through the text renderer (`output.NewRenderer` text format) into a buffer, with the
same warnings `app.emit` would add, and the buffer is written as `{"additionalContext": ...}`
encoded without HTML escaping (as `hooks.encodeJSON` does). `primeLong` gains one sentence, in a paragraph of its own, naming
the flag and what reads it (a GitHub Copilot sessionStart hook); item 3 adds the `init --copilot-hooks` clause.

**Regression guard.** Acceptance's go test -run pattern also covers the help-text tests (TestHelpText); primeLong's new sentence names only `--additional-context` and what reads it (a GitHub Copilot sessionStart hook), never `init --copilot-hooks`, so no commit's help names a flag that does not exist yet; that sentence goes in a new paragraph of its own, not the existing third one, so no paragraph exceeds helpRuleMaxLines.
The `--json` check runs in RunE, where `selectRenderer` (root.go) has set `a.isJSON`; `--kv` stays prime's unknown-flag usage error (`unknownFlagError`, suggest.go), and prime never gains `withKV`, which would put --kv in prime --help and turn plain `prime --kv` into emit's "does not support --kv". The `--json` test asserts the message names both `--additional-context` and `--json`: exit 2 alone also passes on the pre-item tree, where the flag is unknown.
The item yields to `internal/cli/root.go` `rootLong` ("Every command except capcom ui prints plain text, or with --json a versioned JSON envelope"), which it leaves unchanged, and to `withKV`'s doc comment there ("any other command rejects it as an unknown flag").

**Files:** internal/cli/prime.go, internal/cli/prime_test.go, docs/site/reference/cli/capcom_prime.md
**Read first:** internal/cli/prime.go — newPrimeCommand, primeResult, primeLong; internal/cli/root.go — emit, selectRenderer; internal/hooks/settings.go — encodeJSON;
internal/cli/prime_test.go — newPrimeStore; internal/cli/help_text_test.go — TestHelpText_EveryCommand_PrintsExamplesBeforeShortRules
**Tests:** fixture store: `--additional-context` output unmarshals into a struct with one
`additionalContext` string equal to plain `prime`'s stdout from the same store; the raw output is a
single line ending in `\n`; a store whose index fails still exits 0 with the warning inside the
string; `--additional-context --json` exits 2 with a message naming both `--additional-context`
and `--json`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestPrime|TestHelpText'`
- `go vet ./internal/cli/`
- `go run ./cmd/gendocs && git diff --stat docs/site/reference/cli/` shows only `capcom_prime.md`.

**Commit:** `feat(prime): print the Briefing as Copilot sessionStart JSON with --additional-context`

## 2. Merge the capcom sessionStart hook into .github/hooks/capcom.json

**What:**
**Goal:** package `hooks` can add a `sessionStart` entry
`{"type": "command", "command": "capcom prime --additional-context"}` to `.github/hooks/capcom.json`,
creating the file as `{"version": 1, "hooks": {"sessionStart": [entry]}}` when absent, keeping every
other member's bytes and order in an existing file, reporting `Present` on a second run, and
refusing an unusable file with path and line.

**Approach (assumed at the header base):** extends `internal/hooks` in a new `copilot.go`;
`settings.go`'s `Target`/`Merge` stay unchanged (Copilot entries are flat, not matcher groups). It
exports `CopilotPath = ".github/hooks/capcom.json"`, `CopilotPrimeCommand = PrimeCommand + "
--additional-context"`, `MergeCopilot(content []byte) ([]byte, Status, error)` and
`InstallCopilot(worktreeRoot string) (Status, error)`, and reuses the private helpers
`decodeObject`, `encodeObject`, `jsonArray`, `encodeJSON`, `indentJSON`, `settingsFailure`,
`readWorktreeFile` and `writeWorktreeFile` as `Install` does. An object without `version` gets
`"version": 1` as its first member; a `version` other than the number 1 is refused (`version must
be 1`). `hooks.sessionStart` must be an array; Present when any entry's `command` or `bash` equals
`CopilotPrimeCommand`. Entries of another shape are kept verbatim.

**Regression guard.** "Every other member's bytes" means what `Merge` documents and settings_test's "member order and value bytes kept" case pins: other members and foreign entries keep their order and the bytes of their strings and numbers, re-indented two spaces (`indentJSON`).
Every doc comment in `internal/hooks` naming the files or commands the package writes (package doc, `PrimeCommand`, `Status`, `Error`; `grep -n 'apogee\|Codex' internal/hooks/settings.go`) gains the Copilot file.

**Files:** internal/hooks/copilot.go, internal/hooks/copilot_test.go, internal/hooks/settings.go
**Read first:** internal/hooks/settings.go — Install, Merge, decodeObject, settingsFailure, indentJSON; internal/hooks/settings_test.go — TestMerge_UsableContent_AddsTheMissingGroups,
TestInstall_ExistingSettings_KeepTheirMode; internal/hooks/helpers_test.go — unusableFiles
**Tests:** table tests: empty, `null`, absent file; object with other members (kept in order, the
bytes of their strings and numbers equal, re-indented two spaces, as the "member order and value bytes kept" case); existing `sessionStart` entries kept, ours appended; second merge → `Present`, equal bytes;
entry with `bash` equal to the command → `Present`; `version: 2`, `hooks: []`,
`hooks.sessionStart: {}`, broken JSON (Line set) → `*Error` naming `.github/hooks/capcom.json`;
`InstallCopilot` creates `.github/hooks/` and keeps an existing file's mode.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/hooks/`
- `go vet ./internal/hooks/`

**Commit:** `feat(hooks): merge a capcom prime sessionStart hook into .github/hooks/capcom.json`

## 3. capcom init --copilot-hooks

Depends on items 1 and 2.

**What:** Recast at the regression check (2026-11-23).
**Goal:** `capcom init --copilot-hooks` calls `hooks.InstallCopilot`, prints
`added the capcom prime hook to .github/hooks/capcom.json` or
`.github/hooks/capcom.json already runs capcom prime --additional-context`, then
`needs GitHub Copilot CLI 1.0.22 or later\n`; `--json` sets `copilot_hooks` to `added` or
`present`; a refused file exits 1 naming its path. Of the agent hook files it writes only
`.github/hooks/capcom.json`, and `--claude-hooks`, `--codex-hooks` and `--apogee-hooks` write
nothing under `.github/hooks/` (init still creates the store); plain `capcom init` output is unchanged.

**Approach (assumed at the header base):** mirrors `--apogee-hooks` in `internal/cli/init.go`:
`flagCopilotHooks`, `initData.CopilotHooks`, an `installCopilotHooks` beside `installApogeeHooks`
using `hooksFailure`, an `initLong` paragraph and an `initExample` line. The hook-flag count in the
doc comments ("The four flags are independent") is updated.

**Regression guard.** Acceptance's go test -run pattern also covers TestHelpText; item 3 adds to primeLong the clause that init --copilot-hooks runs `capcom prime --additional-context` at session start, in the separate paragraph item 1 added (not the existing third one), and adds internal/cli/prime.go and docs/site/reference/cli/capcom_prime.md to its Files line.
The no-file test also asserts that no `.claude`, `.codex` or `.apogee` exists; the mirror of `TestInit_WithoutApogeeHooks_NamesNothingOfApogee` runs plain, `--claude-hooks`, `--codex-hooks` and `--apogee-hooks` cases and asserts no `.github/` exists. The journey test asserts the command's first word is `capcom` and runs `strings.Fields(command)[1:]` through `runCLI`, which takes no program name.

**Files:** internal/cli/init.go, internal/cli/init_test.go, docs/site/reference/cli/capcom_init.md, internal/cli/prime.go, docs/site/reference/cli/capcom_prime.md
**Read first:** internal/cli/init.go — installApogeeHooks, hooksFailure, initData, initLong, initExample; internal/cli/init_test.go — TestInitApogeeHooks_WithTheOtherHookFlags_WritesAllThreeFiles,
TestInit_WithoutApogeeHooks_NamesNothingOfApogee; internal/cli/testharness_test.go — runCLI; internal/cli/testdata/json/init.golden.json (copilot_hooks must stay omitempty)
**Tests:** `TestInitCopilotHooks_*` mirroring `TestInitApogeeHooks_*`: no file → created, and no
`.claude`, `.codex` or `.apogee`; existing
file kept and idempotent; unusable file → exit 1 with path, file unchanged; combined with
`--claude-hooks --codex-hooks --apogee-hooks` writes all four files; plain init, and init with each
of the other hook flags, names nothing of Copilot and creates no `.github/`. Journey test: read the
`command` of the written `sessionStart` entry, assert its first word is `capcom`, run the words after
it (`strings.Fields(command)[1:]`) through `runCLI`, and assert the stdout unmarshals to a non-empty
`additionalContext`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestInit|TestPrime|TestHelpText'`
- `go vet ./internal/cli/`
- `go run ./cmd/gendocs && git diff --stat docs/site/reference/cli/` shows only `capcom_init.md` and `capcom_prime.md`.

**Commit:** `feat(init): wire capcom prime into GitHub Copilot with --copilot-hooks`

## 4. Document the Copilot hooks

Depends on item 3.

**What:**
**Goal:** `docs/site/agents/agent-setup.md` has a "GitHub Copilot hooks" section after the apogee
one: the `capcom init --copilot-hooks` transcript and the file it writes; the minimum GitHub Copilot
CLI 1.0.22 and VS Code hooks being in preview; that only session start is briefed (no compaction
re-brief); that `.claude/settings.json` hooks also run under Copilot, a harmless extra `capcom prime`
whose text Copilot drops; and that the cloud agent reads the file from the default branch and needs
`capcom` installed by `.github/workflows/copilot-setup-steps.yml`. Every page that lists the hook
flags names `--copilot-hooks` too.

**Approach (assumed at the header base):** find the sites with
`grep -rn 'apogee-hooks' README.md docs/site --include=*.md | grep -v reference/cli`; update each
list (README feature bullet and agents paragraph, quickstart, faq, agent-setup's "combine" line,
its front-matter description). Wording follows the apogee section's shape.

**Regression guard.** The Goal's last sentence reads: every page that names `--apogee-hooks` names `--copilot-hooks` too; migrating-from-beads.md, which lists only beads' replacement flags, stays as it is. Pages listing the hooked agents
without the flag change too: `grep -rn 'Codex and apogee\|all three' README.md docs/site --include=*.md | grep -v reference/cli` prints nothing (briefing.md:84, :118; quickstart.md:56; agent-setup.md:3, :171).
The Goal's specifics are checked by the added Acceptance greps below.

**Files:** docs/site/agents/agent-setup.md, README.md, docs/site/getting-started/quickstart.md, docs/site/getting-started/faq.md, docs/site/agents/briefing.md
**Read first:** docs/site/agents/agent-setup.md — front-matter description, "## Codex hooks" combine line, "## apogee hooks"; docs/site/agents/briefing.md — hooks sentence, Next list;
README.md — feature bullet "Claude Code, Codex and apogee hooks", FAQ "Which coding agents"; docs/site/getting-started/quickstart.md — config snippet paragraph
**Tests:** none (docs only)
**Acceptance:**
- `grep -c 'copilot-hooks' docs/site/agents/agent-setup.md` prints at least 2.
- `grep -rln 'apogee-hooks' README.md docs/site --include=*.md | grep -v reference/cli | xargs grep -L 'copilot-hooks'` prints nothing.
- `grep -rn 'Codex and apogee\|all three' README.md docs/site --include=*.md | grep -v reference/cli` prints nothing.
- `grep -n '^## apogee hooks\|^## GitHub Copilot hooks' docs/site/agents/agent-setup.md` lists `## apogee hooks` first, `## GitHub Copilot hooks` second.
- `grep -c '1\.0\.22' docs/site/agents/agent-setup.md`, `grep -c 'copilot-setup-steps.yml' docs/site/agents/agent-setup.md` and `grep -c '.github/hooks/capcom.json' docs/site/agents/agent-setup.md` each print at least 1.

**Commit:** `docs(agents): document capcom init --copilot-hooks`
