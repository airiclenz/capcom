# capcom init --apogee-hooks

**Goal:** `capcom init --apogee-hooks` adds a reaction running `capcom prime` on `session-started`
and `compaction-finished` to the repository's `.apogee/config.yaml`, opt-in and independent of
`--claude-hooks` and `--codex-hooks`. Execution waits until a released apogee ships both moments.
**Date:** 2026-10-20 (revised 2026-10-21 against apogee main 4b290a8b)
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 8de8312
**Closes:** apogee-hooks

**Sources:**
- `.capcom/issues/agent-ergonomics/apogee-hooks.md`
- `~/repos/apogee/docs/manual/reactions.md` (entry shape, `advise:`, `session-started`, `compaction-finished`, "Where an `advise:` or `gate:` command runs")
- `~/repos/apogee/docs/manual/configuration.md` § "A project's own config" (adoption, ADR 0104)
- `~/repos/apogee/internal/domain/reaction.go` (advise Moment validation), `~/repos/apogee/internal/agent/sessionadvise.go`
- `~/repos/apogee/CHANGELOG.md` [Unreleased]: "`advise:` on `session-started` is admitted", "`compaction-finished` joins the Reaction vocabulary"
- Notes `cli-test-harness`, `error-messages`, `coding-conventions`, `docs-layout`

**Regression check (2026-10-21, 8de8312):**
- 1: writer decision folded (old-apogee line quoted as an excerpt; PATH probe only on a release-tag `apogee --version`)
- 2: guard folded (`...` refused by a line scan; yaml.v3 line parsed into Error.Line; doc-comment scope widened)
- 3: guard folded (`primeLong` gives apogee its own "after compaction" clause; no-apogee assertions name the apogee lines, not the word)
- 4: guard folded (writer decision: never quote the old-apogee line; scope grep widened to "Codex hooks", adds `docs/site/agents/briefing.md`)

**Ratified design calls:**
- **File:** the project's `.apogee/config.yaml`, committed like `.claude/settings.json`; apogee asks each user to adopt the entry once (the user, 2026-10-20).
- **Independence:** `--apogee-hooks` mirrors `--codex-hooks`: own flag, writes only its own file, combines freely; plain `capcom init` and the existing flags stay exactly as they are (the user, 2026-10-20).
- **Moments:** the entry fires on `session-started` and `compaction-finished`, apogee's pair for Claude's `SessionStart` and `PreCompact` (the user, 2026-10-21).
- **Old apogee:** no version detection or subprocess; init's output and the docs name the minimum apogee release read by item 1 (the user, 2026-10-20).

**Standing requirements:**
- skills: coding-standards
- CLI tests go through `runCLI`, never `t.Parallel()` (Note `cli-test-harness`).
- Tests on this host: `GOMEMLIMIT=2GiB`, one package at a time, never `./...` with `-race`.
- A help-text or flag change regenerates the committed reference: `go run ./cmd/gendocs`.

**Out of scope:**
- Writing to the user-level `~/.apogee/config.yaml`.
- Any change to plain `capcom init`, `--claude-hooks` or `--codex-hooks` output.
- Running or probing apogee from capcom.

## 1. Gate: a released apogee ships session-start and compaction advice

**What:**
**Goal:** a released apogee accepts an `advise:` entry with `on: [session-started,
compaction-finished]` in a project `.apogee/config.yaml`, and item 1 records the first such
release as a dated NOTES line `minimum apogee: <version>`. This item changes no file; when the
check fails, stop the run.

**Approach (assumed at the header base):** read `~/repos/apogee/CHANGELOG.md` release headings and
`git -C ~/repos/apogee tag`. The minimum is the first release heading (with its `v*` tag) whose
section carries both "`advise:` on `session-started` is admitted" and "`compaction-finished` joins
the Reaction vocabulary" (at the base both sit under [Unreleased]). The apogee issue
`session-start-advise` is not a gate: it stays open only for the deferred gap
`advise-run-after-close`. An apogee before the minimum ignores just the capcom-prime entry, with
a start-up line containing `ignoring reactions entry "capcom-prime"`; the rest of its project
layer still loads.

**Regression guard.** Ignore the retracted v1.0.0–v1.8.0 tags and any [1.x] CHANGELOG heading: the
minimum is the first non-retracted (0.x) release that ships both, noted with "the retracted 1.x
series predates it". Items 3 and 4 use the recorded value, never one pinned in this plan.

**Files:** none
**Read first:** ~/repos/apogee/CHANGELOG.md — [Unreleased], [0.27.0]; ~/repos/apogee/internal/domain/reaction.go — MomentCompactionFinished, advise Moment check; ~/repos/apogee/internal/config/execlayer.go — repoReactionRefusedNotice, repoReactionConfigs;
~/repos/apogee/internal/reactions/hooks.go — ParseEvent; ~/repos/apogee/docs/manual/probe.md — apogee probe config (notices section)
**Tests:** none
**Acceptance:**
- `grep -c 'pending admission' ~/repos/apogee/docs/manual/reactions.md` prints `0`.
- A non-retracted release heading in `~/repos/apogee/CHANGELOG.md`, with its `v*` tag, carries both
  CHANGELOG lines above, and item 1 carries the NOTES line `minimum apogee: <version>`; none → the
  gate fails and the run stops.
- Only when `apogee --version` prints exactly a release tag (no `+<n>.g<sha>` dev suffix) at or
  past that version: in a scratch git repo holding the item 2 entry, `apogee probe config` prints no
  `ignoring reactions entry` line. Otherwise item 1 records a dated NOTES line
  `probe skipped: apogee on PATH is <version>`, and the gate still passes on the CHANGELOG/tag check.

**Commit:** none (gate only)

## 2. Merge the capcom prime reaction into .apogee/config.yaml

Depends on item 1.

**What:**
**Goal:** package `hooks` can add the capcom prime reaction to a project `.apogee/config.yaml`,
creating the file when absent, keeping every other byte of an existing file, reporting `Present`
on a second run, and refusing an unusable file with path and line.

**Approach (assumed at the header base):** extends `internal/hooks` (it owns agent hook files); no
new package. A new `apogee.go` holds `ApogeePath = ".apogee/config.yaml"`,
`MergeApogee(content []byte) ([]byte, Status, error)` and `InstallApogee(worktreeRoot string)
(Status, error)`, reusing `Status`, `*Error{Path, Line, Message}`, `PrimeCommand`,
`nodefile.ReadRegularFile`, `nodefile.WriteFileAtomic` and `nodefile.ReplaceFileVerbatim` as
`Install` does. The JSON `Target`/`Merge` path is untouched. The entry, exactly (an argv list:
apogee refuses a bare-string `advise:`):

```yaml
  - id: capcom-prime
    on: [session-started, compaction-finished]
    advise: ["capcom", "prime"]
```

Edit bytes, not a re-encoded tree (precedent: `config.RaiseFormat` / `insertLine`): parse with
`gopkg.in/yaml.v3` into a `yaml.Node` only to locate things. No `reactions:` key → append
`reactions:` plus the entry at the end (adding a missing final newline). A block-sequence
`reactions:` → insert the entry after its last item. Present when any item has `id: capcom-prime`
or an `advise:` argv equal to `capcom prime`. Refuse, writing nothing: YAML that does not parse
(`invalid config: …` with its line), a non-mapping document, `reactions` that is not a block
sequence (`reactions must be a block list; add the capcom-prime entry by hand`, at its line). Keep
the file's line endings.

**Regression guard.** Refuse a second YAML document (precedent `internal/config/parse.go`
`parser.document`, whose second-Decode check covers only `a: 1\n---\nb: 2\n`, Line 2) and a `...`
end marker found by a line scan (a regex like `format.go` `documentStartLine`:
`^\.\.\.([ \t]+(#.*)?)?$`, Error.Line = that line: yaml.v3 returns io.EOF after it), and a
flow-style root mapping (precedent `format.go` `isPlaceable`). Broken YAML: parse the
`yaml: line N: ` prefix into Error.Line and strip it from Message (a yaml error with no line keeps
Line 0; `parser.yamlError` leaves Line 0). Indent the new `- ` at the sequence node's Column, never
the item node's. Every doc comment on the package or on a symbol `apogee.go` reuses (`PrimeCommand`,
`Status`, `Added`, `Present`, `Error`) covers `.apogee/config.yaml` too:
`grep -n 'settings\|JSON\|Claude Code and Codex\|event\|of Install' internal/hooks/settings.go`.

**Files:** internal/hooks/apogee.go, internal/hooks/apogee_test.go, internal/hooks/settings.go
**Read first:** internal/hooks/settings.go — Install, Error, PrimeCommand; internal/config/format.go — insertLine, documentStartLine;
internal/config/parse.go — parser.document, parser.yamlError; internal/hooks/settings_test.go — TestMerge_UnusableContent_FailsWithPathAndLine
**Tests:** table tests: empty/absent file; file without `reactions:`; existing reactions (entry
appended, others byte-identical, comments kept; the result re-parses and its reactions list grew
by one, the new item's `on` equal to `[session-started, compaction-finished]`); second merge →
`Present`, equal bytes; flow `reactions: []`, scalar `reactions`, broken YAML (Line from the
`yaml: line N: ` prefix, which Message no longer repeats), `allow: [go test]\n...\n` (Line 2),
`a: 1\n---\nb: 2\n` (Line 2), flow root `{allow: [go test]}\n` → `*Error` with path and line, file
unchanged; `InstallApogee` creates `.apogee/` and keeps an existing file's mode.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/hooks/`
- `go vet ./internal/hooks/`

**Commit:** `feat(hooks): merge a capcom prime reaction into .apogee/config.yaml`

## 3. capcom init --apogee-hooks

Depends on item 2.

**What:**
**Goal:** `capcom init --apogee-hooks` installs the reaction and prints
`added the capcom prime reaction to .apogee/config.yaml`, then
`apogee asks to adopt reaction capcom-prime at the next session start`, then
`needs apogee <minimum apogee from item 1's NOTES> or later; an older apogee ignores the capcom-prime entry`
(the last two on the present path too, after `.apogee/config.yaml already runs capcom prime`);
`--json` carries `apogee_hooks` (`added` or `present`); an unusable file fails exit 1 as
`capcom: .apogee/config.yaml:<line>: …` (`capcom: .apogee/config.yaml: …` where no line applies),
the file untouched. Without `--apogee-hooks`, init's text, `--json` data and written files name
nothing of apogee and create no `.apogee/`; `--apogee-hooks` alone still prints the Claude snippet.

**Approach (assumed at the header base):** in `internal/cli/init.go`, `newInitCommand` gains
`flagApogeeHooks = "apogee-hooks"` (usage: `also add a reaction running capcom prime at session
start and after compaction to .apogee/config.yaml`) beside `flagCodexHooks`; a sibling of
`installHooks` calls `hooks.InstallApogee` and maps `*hooks.Error` through `hooksFailure`.
`initData` gains `apogee_hooks` (omitempty). Update `initLong`, add an `initExample` line
(`capcom init --apogee-hooks  # ...and from apogee's session-started and compaction-finished reaction`).
`primeLong` in `internal/cli/prime.go` gives `init --apogee-hooks` its own clause (see the guard).
Regenerate `docs/site/reference/cli/` with `go run ./cmd/gendocs`.

**Regression guard.** Tests pin the exact `needs apogee … ignores the capcom-prime entry` line on
both paths. For plain, `--claude-hooks` and `--codex-hooks`, assert the output lacks the three apogee
lines and `.apogee/config.yaml`, the `--json` data lacks `apogee_hooks`, and no `.apogee/` exists — never
a bare "apogee" check (the temp-dir root carries `t.Name()`), as `TestInitCodexHooks_WithoutClaudeHooks_StillPrintsTheClaudeSnippet`
checks `codexFeatureNote` and `codexHooksPath`. `primeLong` keeps "init --claude-hooks and init --codex-hooks
run it at session start and before compaction" and adds its own clause, "init --apogee-hooks runs it at
session start and after compaction" (apogee's `compaction-finished` fires after the fold), pinned in the
regenerated `capcom_prime.md`. Every init.go comment listing init's flags or hook targets names
`--apogee-hooks` too: `grep -n 'three flags\|Codex hooks\|CodexHooks and' internal/cli/init.go`.

**Files:** internal/cli/init.go, internal/cli/init_test.go, internal/cli/prime.go, docs/site/reference/cli/capcom_init.md, docs/site/reference/cli/capcom_prime.md
**Read first:** internal/cli/init.go — newInitCommand, installHooks, hooksFailure, initData, initLong; internal/cli/prime.go — primeLong;
internal/cli/init_test.go — TestInitCodexHooks_NoHooksFile_CreatesIt, TestInitCodexHooks_WithoutClaudeHooks_StillPrintsTheClaudeSnippet
**Tests:** in `init_test.go`, mirroring the Codex tests: no file → created, `apogee_hooks` `added`
in `--json`; existing file → other entries kept, second run prints the exact `already runs` line;
the exact `needs apogee` line on both paths; unusable file → exit 1 with path and line, plus a
no-line row; `--apogee-hooks` alone still prints the Claude snippet; all three flags together
write all three files; the no-apogee path above.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'TestInit'`
- `GOMEMLIMIT=2GiB go test ./internal/clidoc/`
- `grep -c 'init --apogee-hooks runs it at session start and after compaction' docs/site/reference/cli/capcom_prime.md` prints `1`
- `go vet ./internal/cli/`

**Commit:** `feat(cli): add capcom init --apogee-hooks`

## 4. Document apogee hooks

Depends on item 3.

**What:**
**Goal:** the agent docs name `--apogee-hooks` wherever they name `--codex-hooks` as a way to
brief a session, and explain the one-time adoption in apogee.

**Approach (assumed at the header base):** `docs/site/agents/agent-setup.md`: a "## apogee hooks"
section after "## Codex hooks" showing the command, its output and the entry it writes. It says:
the entry briefs the model at session start and again after every compaction fold (apogee's
`compaction-finished` fires after the fold, where Claude's `PreCompact` fires before it); apogee
proposes it until each user adopts it (start-up pane, `/settings` or `apogee project adopt`); the
command runs in the Project root, inside apogee's workspace box when `confine-to-workspace` is on,
with a 10 s default timeout and its output capped at 8 KiB. Update the page's `description` and
the "sets up both agents at once" line to all three flags. `README.md`: the "Works with any coding
agent" hooks bullet and the FAQ agents answer. `docs/site/getting-started/faq.md`: the agents
answer. `llms.txt`: the Agent setup line's description, matching the new frontmatter.

**Regression guard.** The apogee section states the minimum apogee recorded by item 1, that an
older apogee ignores the capcom-prime entry with one start-up line and loads the rest of the
project config, and that the retracted apogee 1.x releases predate it despite their higher
numbers. Never quote the old-apogee start-up line verbatim; describe it as one start-up line naming
the ignored `capcom-prime` entry. Scope: every place that names the Codex hooks or `--codex-hooks`
as briefing a session, `grep -rn 'Codex hooks\|--codex-hooks' README.md docs/site | grep -v /reference/`,
except `docs/site/getting-started/migrating-from-beads.md` (so `docs/site/agents/briefing.md` too).

**Files:** docs/site/agents/agent-setup.md, README.md, docs/site/getting-started/faq.md, llms.txt, docs/site/getting-started/quickstart.md, docs/site/agents/briefing.md
**Read first:** docs/site/agents/agent-setup.md — frontmatter description, "## Codex hooks", "sets up both agents at once"; README.md — "Claude Code and Codex hooks" bullet;
docs/site/agents/briefing.md — "When to run it"; ~/repos/apogee/docs/manual/reactions.md — advise: row, "Advising the model"; ~/repos/apogee/docs/manual/configuration.md — Adoption
**Tests:** none (docs only)
**Acceptance:**
- `grep -c 'apogee-hooks' docs/site/agents/agent-setup.md README.md docs/site/getting-started/faq.md docs/site/getting-started/quickstart.md` prints a non-zero count for each
- `grep -c 'apogee' docs/site/agents/briefing.md` prints a non-zero count
- `grep -c 'compaction-finished' docs/site/agents/agent-setup.md` prints a non-zero count
- `capcom check`

**Commit:** `docs(agents): document capcom init --apogee-hooks`
