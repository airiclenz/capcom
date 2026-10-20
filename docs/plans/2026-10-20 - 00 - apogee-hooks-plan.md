# capcom init --apogee-hooks

**Goal:** `capcom init --apogee-hooks` adds a `session-started` reaction running `capcom prime` to
the repository's `.apogee/config.yaml`, opt-in and independent of `--claude-hooks` and
`--codex-hooks`. Execution waits until apogee accepts `advise:` on `session-started`.
**Date:** 2026-10-20
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** db9c48b
**Closes:** apogee-hooks

**Sources:**
- `.capcom/issues/agent-ergonomics/apogee-hooks.md`
- `~/repos/apogee/docs/manual/reactions.md` (entry shape, `advise:`, `session-started`)
- `~/repos/apogee/docs/manual/configuration.md` § "A project's own config" (adoption, ADR 0104)
- apogee issue `session-start-advise` (in `~/repos/apogee/.capcom/`)
- Notes `cli-test-harness`, `error-messages`, `coding-conventions`, `docs-layout`

**Ratified design calls** (the user, 2026-10-20):
- **File:** the project's `.apogee/config.yaml`, committed like `.claude/settings.json`; apogee
  asks each user to adopt the entry once.
- **Independence:** `--apogee-hooks` mirrors `--codex-hooks`: its own flag, writes only its own
  file, combines freely; plain `capcom init` and the existing flags stay exactly as they are
  (the Claude snippet still prints unless `--claude-hooks` is given).
- **Old apogee:** an apogee that refuses session-start advice skips the whole project layer;
  init's output and the docs name the minimum apogee release (read by item 1), with no version
  detection or subprocess (the user, 2026-10-20).

**Standing requirements:**
- skills: coding-standards
- CLI tests go through `runCLI`, never `t.Parallel()` (Note `cli-test-harness`).
- Tests on this host: `GOMEMLIMIT=2GiB`, one package at a time, never `./...` with `-race`.
- A help-text or flag change regenerates the committed reference: `go run ./cmd/gendocs`.

**Out of scope:**
- A compaction hook for apogee: apogee has no compaction notice.
- Writing to the user-level `~/.apogee/config.yaml`.
- Any change to plain `capcom init`, `--claude-hooks` or `--codex-hooks` output.

**Regression check (2026-10-20, db9c48b):**
- 1: recast — the gate also finds and records the minimum apogee release (writer's decision)
- 2: guard folded — refuse multi-document and flow-root files; indent at the sequence's dash
  column; settings.go doc comments widened
- 3: recast — third "needs apogee" line and primeLong (writer's decision); guard folded —
  no-line failures
- 4: guard folded — minimum apogee in agent-setup (writer's decision); quickstart.md in scope
- 1 (re-check): guard folded — the minimum ignores the retracted v1.0.0–v1.8.0 tags and [1.x]
  headings (writer's decision)
- 3 (re-check): guard folded — without --apogee-hooks init names nothing of apogee, pinned by a
  test; init.go comments listing init's flags name --apogee-hooks
- 4 (re-check): guard folded — agent-setup says the retracted apogee 1.x releases do not qualify
  (writer's decision)

## 1. Gate: apogee accepts advise on session-started

**What:**
Recast at the regression check (2026-10-20).
**Goal:** apogee's released behaviour accepts an `advise:` entry with `on: [session-started]` in a
project `.apogee/config.yaml`. Writing that entry before then makes apogee skip the whole project
layer (`apogee: project config <path>: …; the project layer is skipped`), so this plan must not
run until it holds. This item changes no file; when the check fails, stop the run.

**Regression guard.** Besides the existing checks, the gate reads from ~/repos/apogee (its
CHANGELOG release headings and git tags) the first apogee release that ships session-start advise
(issue session-start-advise closed), and records it as a dated NOTES line under item 1 in the form
"minimum apogee: <version>"; items 3 and 4 use that recorded value, never one pinned in this plan.
If no released version ships it yet, the gate fails and the run stops.
When reading the minimum apogee release, ignore the retracted v1.0.0–v1.8.0 tags and the [1.x]
CHANGELOG headings; the minimum is the first non-retracted release (the 0.x series) that ships
session-start advise, recorded as "minimum apogee: <version>" with a note that the retracted 1.x
series predates it.

**Files:** none
**Read first:** ~/repos/apogee/docs/manual/reactions.md — `on:` row, "pending admission" refusal paragraph; ~/repos/apogee/.capcom/issues/session-start-advise.md — Acceptance, deps (blocked by non-tool-advise-slot, session-advise-admission-arm);
~/repos/apogee/CHANGELOG.md — [Unreleased], [0.27.0] headings, retracted [1.0.0]–[1.7.0] headings; ~/repos/apogee/docs/manual/configuration.md — "A project's own config" (project layer skipped)
**Tests:** none
**Acceptance:**
- `cd ~/repos/apogee && capcom show session-start-advise` reports `category: done`.
- `grep -c 'pending admission' ~/repos/apogee/docs/manual/reactions.md` prints `0`.
- A released apogee version (a CHANGELOG release heading with its `v*` git tag in
  `~/repos/apogee`) ships session-start advise, and item 1 carries the dated NOTES line
  `minimum apogee: <version>`; no such release → the gate fails and the run stops. A retracted
  v1.0.0–v1.8.0 tag or [1.x] heading never counts.

**Commit:** none (gate only)

## 2. Merge the capcom prime reaction into .apogee/config.yaml

Depends on item 1.

**What:**
**Goal:** package `hooks` can add the capcom prime reaction to a project `.apogee/config.yaml`,
creating the file when absent, keeping every other byte of an existing file, reporting `Present`
on a second run, and refusing an unusable file with path and line.

**Approach (assumed at the header base):** extends `internal/hooks` (it owns agent hook files);
no new package. A new `apogee.go` holds `ApogeePath = ".apogee/config.yaml"`,
`MergeApogee(content []byte) ([]byte, Status, error)` and
`InstallApogee(worktreeRoot string) (Status, error)`, reusing `Status` (`Added`/`Present`),
`*Error{Path, Line, Message}`, `PrimeCommand`, `nodefile.ReadRegularFile`,
`nodefile.WriteFileAtomic` and `nodefile.ReplaceFileVerbatim` as `Install` does. The JSON
`Target`/`Merge` path is untouched. The entry, exactly:

```yaml
  - id: capcom-prime
    on: [session-started]
    advise: ["capcom", "prime"]
```

Edit bytes, not a re-encoded tree (precedent: `config.RaiseFormat` / `insertLine`): parse with
`gopkg.in/yaml.v3` into a `yaml.Node` only to locate things. No `reactions:` key → append
`reactions:` plus the entry at the end (adding a missing final newline). A block-sequence
`reactions:` → insert the entry after its last item, its `- ` at the sequence node's Column (the dash column of its items). Present when
any item has `id: capcom-prime` or an `advise:` argv equal to `capcom prime`. Refuse, writing
nothing: YAML that does not parse (`invalid config: …` with its line), a non-mapping document,
`reactions` that is not a block sequence (`reactions must be a block list; add the capcom-prime
entry by hand`, at its line). Keep the file's line endings.

**Regression guard.**
- Refuse, writing nothing, a second YAML document or a `...` end marker (precedent
  `internal/config/parse.go` `parser.document`: "only one YAML document is allowed") and a
  flow-style root mapping (precedent `internal/config/format.go` `isPlaceable`,
  `root.Style&yaml.FlowStyle`); otherwise the entry lands outside the document apogee reads.
- Indent the new `- ` at the sequence node's Column (the dash column of its items), never the
  item node's Column, which yaml.v3 puts past the dash (`  - id: x` → sequence 3, item 5).
- Every doc comment in `internal/hooks` that names JSON/settings files as the package's or a
  shared type's only subject (package doc, `Status`, `Error.Line`) covers `.apogee/config.yaml`
  too: `grep -n 'settings file\|JSON' internal/hooks/settings.go`.

**Files:** internal/hooks/apogee.go, internal/hooks/apogee_test.go, internal/hooks/settings.go
**Read first:** internal/hooks/settings.go — Install, readWorktreeFile, Error; internal/config/format.go — RaiseFormat, insertLine, isPlaceable;
internal/config/parse.go — parser.document; internal/hooks/helpers_test.go — unusableFiles
**Tests:** table tests for: empty/absent file; file without `reactions:`; existing reactions
(entry appended, others byte-identical, comments kept; the result re-parses and its reactions
list grew by one); second merge returns `Present` and equal bytes; flow `reactions: []`, a scalar
`reactions`, broken YAML, `allow: [go test]\n...\n`, `a: 1\n---\nb: 2\n` and a flow root
`{allow: [go test]}\n` → `*Error` with path and line, file unchanged;
`InstallApogee` creates `.apogee/` and keeps an existing file's mode.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/hooks/`
- `go vet ./internal/hooks/`

**Commit:** `feat(hooks): merge a capcom prime reaction into .apogee/config.yaml`

## 3. capcom init --apogee-hooks

Depends on item 2.

**What:**
Recast at the regression check (2026-10-20).
**Goal:** `capcom init --apogee-hooks` installs the reaction and prints
`added the capcom prime reaction to .apogee/config.yaml` then
`apogee asks to adopt reaction capcom-prime at the next session start`, or
`.apogee/config.yaml already runs capcom prime`; `--json` carries `apogee_hooks` (`added` or
`present`); an unusable file fails exit 1 as `capcom: .apogee/config.yaml:<line>: …`
(`.apogee/config.yaml: …` where no line applies), the file untouched.
Without `--apogee-hooks` (plain, `--claude-hooks`, `--codex-hooks`), init's text, `--json` data
and written files name nothing of apogee and create no `.apogee/`;
`--apogee-hooks` alone still prints the Claude snippet, as `--codex-hooks` alone does.

**Approach (assumed at the header base):** in `internal/cli/init.go`, `newInitCommand` gains
`flagApogeeHooks = "apogee-hooks"` (usage: `also add a session-started reaction running capcom
prime to .apogee/config.yaml`) beside `flagCodexHooks`; a sibling of `installHooks` calls
`hooks.InstallApogee` and maps `*hooks.Error` through `hooksFailure`. `initData` gains
`apogee_hooks` (omitempty). Update `initLong` and add an `initExample` line
(`capcom init --apogee-hooks  # ...and from apogee's session-started reaction`). Regenerate
`docs/site/reference/cli/capcom_init.md` with `go run ./cmd/gendocs`.

**Regression guard.** init --apogee-hooks prints a third line after the adoption line, `needs
apogee <minimum apogee from item 1's NOTES> or later; an older apogee skips .apogee/config.yaml`,
on both the added and the present paths, and a test pins it. Item 3 also extends primeLong in
internal/cli/prime.go to name `init --apogee-hooks` beside --claude-hooks and --codex-hooks
(session start only, no compaction hook) and regenerates docs/site/reference/cli/capcom_prime.md
with go run ./cmd/gendocs; add both paths to Files.
A failure with no single line (not a regular file, larger than 1 MiB) reads
`capcom: .apogee/config.yaml: …`, as the Codex table's no-line rows do.
A test pins the no-apogee path: plain, `--claude-hooks` and `--codex-hooks` each print no "apogee",
carry no `apogee_hooks` key and create no `.apogee/`. Every comment in init.go that lists init's
flags or hook targets names `--apogee-hooks` / the apogee reaction too (the "Flags of init" const
comment and `grep -n 'three flags\|Codex hooks\|CodexHooks and' internal/cli/init.go`).

**Files:** internal/cli/init.go, internal/cli/init_test.go, docs/site/reference/cli/capcom_init.md,
internal/cli/prime.go, docs/site/reference/cli/capcom_prime.md
**Read first:** internal/cli/init.go — newInitCommand, installHooks, hooksFailure, initData, initLong;
internal/cli/init_test.go — TestInitCodexHooks_WithoutClaudeHooks_StillPrintsTheClaudeSnippet, TestInitCodexHooks_UnusableHooksFile_FailsWithPath;
internal/cli/prime.go — primeLong
**Tests:** in `init_test.go`, mirroring the Codex tests: no file → created and `apogee_hooks`
`added` in `--json`; existing file → other entries kept and second run prints the exact
`already runs` line; the exact `needs apogee <minimum> or later; …` line on both the added and
the present paths; unusable file → exit 1 with path and line, and a no-line row
(`capcom: .apogee/config.yaml: `); `--apogee-hooks` alone still prints the Claude snippet; all
three flags together write all three files; plain, `--claude-hooks` and `--codex-hooks` each
print no "apogee", carry no `apogee_hooks` key and create no `.apogee/` (mirroring
`TestInitCodexHooks_WithoutClaudeHooks_StillPrintsTheClaudeSnippet`).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestInit'`
- `GOMEMLIMIT=2GiB go test ./internal/clidoc/`
- `go vet ./internal/cli/`

**Commit:** `feat(cli): add capcom init --apogee-hooks`

## 4. Document apogee hooks

Depends on item 3.

**What:**
**Goal:** the agent docs name `--apogee-hooks` wherever they name `--codex-hooks` as a way to
brief a session, and explain the one-time adoption in apogee.

**Approach (assumed at the header base):** `docs/site/agents/agent-setup.md`: a "## apogee
hooks" section after "## Codex hooks" showing the command, its output and the entry it writes,
saying apogee proposes it until each user adopts it (start-up pane, `/settings` or
`apogee project adopt`) and that there is no compaction hook; update the page's `description`
and the "sets up both agents at once" line to all three flags. `README.md`: the "Works with any
coding agent" hooks bullet and the FAQ agents answer. `docs/site/getting-started/faq.md`: the
agents answer. `llms.txt`: the Agent setup line's description, matching the new frontmatter.

**Regression guard.** The apogee section in docs/site/agents/agent-setup.md states the minimum
apogee release recorded by item 1 and that an older apogee skips the project's .apogee/config.yaml
entirely. Scope is every hand-written page naming `--codex-hooks` as a briefing,
`grep -rn -- '--codex-hooks' README.md docs/site | grep -v /reference/`, except
`docs/site/getting-started/migrating-from-beads.md` (it replaces bd hooks); that includes quickstart.md.
The apogee section in docs/site/agents/agent-setup.md also says the retracted apogee 1.x releases
predate session-start advice and do not qualify, despite their higher numbers. init's output line
from item 3 stays as it is.

**Files:** docs/site/agents/agent-setup.md, README.md, docs/site/getting-started/faq.md, llms.txt,
docs/site/getting-started/quickstart.md
**Read first:** docs/site/agents/agent-setup.md — frontmatter description, "## Codex hooks", "sets up both agents at once"; README.md — "Works with any coding agent" hooks bullet, FAQ "Which coding agents";
docs/site/getting-started/faq.md — "Which coding agents does capcom work with?"; docs/site/getting-started/quickstart.md — init hooks paragraph;
~/repos/apogee/docs/manual/configuration.md — Adoption (project adopt, /settings)
**Tests:** none (docs only)
**Acceptance:**
- `grep -c 'apogee-hooks' docs/site/agents/agent-setup.md README.md docs/site/getting-started/faq.md docs/site/getting-started/quickstart.md` prints a non-zero count for each
- `capcom check`

**Commit:** `docs(agents): document capcom init --apogee-hooks`
