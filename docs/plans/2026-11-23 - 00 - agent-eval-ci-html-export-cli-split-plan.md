# Plan: scheduled agent eval, static HTML export, and the internal/cli split analysis

**Goal:** A weekly and on-demand GitHub workflow runs the agent eval against every configured model
and goes red when a model's pass count drops below its threshold. `capcom export --html <dir>`
writes a deterministic, JavaScript-free static site of the Issues and Notes. A plan and one child
Issue per move step exist for splitting `internal/cli`.
**Date:** 2026-11-23
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 761b9b6

**Regression check (2026-11-23, 761b9b6):**
- 1: guard folded
- 2: guard folded
- 3: guard folded
- 4: recast (guard folded)
- 5: recast
- 6: guard folded
- 7: guard folded
- 4 (re-check): guard folded — deps rendered through `DepGraph.Resolve`; issues.html built with `outline.Issues(entries, true)`

**Sources:**
- Issues `scheduled-agent-eval`, `html-export`, `split-cli-package` (`capcom show <id>`)
- Notes `agent-eval`, `cli-test-harness`, `coding-conventions`, `docs-layout`, `store-layout`, `adr-0012`, `adr-0008`
- `docs/agent-eval.md`, `.github/workflows/ci.yml`, `docs/site/reference/json-output.md`

**Ratified design calls** (Airic, 2026-11-23):
- **split-cli-package scope:** one analysis item writes the move plan and files the child Issues; no code moves in this plan.
- **Export dir:** an export writes a marker file `.capcom-export`; a dir holding it is cleared and rewritten; a non-empty dir without it is refused.
- **Body rendering:** Issue and Note bodies render as HTML-escaped preformatted text (wrapping); no markdown renderer, no new dependency.
- **Exported Issues:** every Issue, closed and dropped included, each with its status.
- **Eval matrix:** repository variable `AGENT_EVAL_MODELS`, a JSON list of `{name, base_url, model, key_secret}`; unset or empty runs no model jobs; a `claude -p` job runs only when secret `ANTHROPIC_API_KEY` is set.
- **Threshold:** `agenteval run --thresholds <file> --name <model>`; below the threshold fails; a name missing from the file fails too, naming the file.

**Standing requirements:**
- skills: coding-standards
- Single packages with `GOMEMLIMIT=2GiB`; never `./...` with `-race`; never two test commands at once (AGENTS.md).
- CLI tests go through `runCLI` (Note `cli-test-harness`); never `t.Parallel()`.
- After changing a command or its help: `go run ./cmd/gendocs`, commit `docs/site/reference/cli/`; never hand-edit it.
- Knowledge Notes change only through `capcom know update`, then `capcom check`.
- Any deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- Moving any code out of `internal/cli` (the child Issues of item 7 carry that).
- Committing eval reports from CI; a markdown renderer; JavaScript in the export.
- The stale capcom block in `AGENTS.md` that `capcom check` reports.

## 1. agenteval run enforces a per-model pass threshold — ✅ DONE (2026-11-23)

NOTES (2026-11-23): checkRunOptions now returns the resolved runThreshold alongside its error (PreRunE stores it in runOptions.threshold), so the file is loaded once, at command-line check time; the pair check, file load and name lookup live in thresholds.go's lookupThreshold, which checkRunOptions calls last.
NOTES (2026-11-23): the met-threshold line is printed by enforceThreshold in thresholds.go, after runRun's `wrote …` line; the "met" run test asserts exit 0 and a passing row only (runNextClaimClose discards stdout); the met line's text is pinned by TestEnforceThreshold_JudgesThePassedCount.

**What:**
**Goal:** `agenteval run --thresholds <file> --name <model>` exits non-zero when the run's passed
count is below `<model>`'s threshold in `<file>`, and fails before any task runs when `<file>` is
unreadable or has no entry for `<model>`; without the flags `run` behaves as before.
**Approach (assumed at the header base):**
- New `cmd/agenteval/thresholds.go`: `loadThresholds(path) (map[string]int, error)` parses a YAML
  map `models: {<name>: <min passed>}` with `go.yaml.in/yaml/v3` (already in go.mod); a negative
  value or an unknown top-level key is an error.
- `newRunCommand` gains `--thresholds` and `--name`; either without the other is a usage error
  (`exitBadUsage`), checked in `checkRunOptions` together with the file load and the name lookup,
  so a misconfigured model fails in seconds, not after the run. Messages name the file path.
- After the report is written, `runRun` compares `countPassed(results)` with the threshold: below
  it prints `threshold <name>: <passed>/<total> passed, need <min>` to stderr and exits with a new
  `exitBelowThreshold = 3`; met, it prints `threshold <name>: met (<min>)` to stdout. The report is
  always written first.
- New `cmd/agenteval/thresholds.yaml` holding a comment on its format and `models: {}`.
- `doc.go`: the `run` usage line names the two flags; the `# Files` map lists `thresholds.go`.
**Regression guard.** `runE` wraps every RunE error in `runError`, so `runRun` prints nothing for a
shortfall: it returns a typed `belowThresholdError` whose `Error()` is the `threshold <name>: …`
line (main prints it once), `exitCodeFor` checks it BEFORE `runError`, and main.go's exit-code
consts document exit 3. run's Long help and doc.go state exit code 3 as "the passed count is below the --name model's threshold"; item 3 reuses that exact wording in docs/agent-eval.md (the Long's "It exits 0 once the report is written, whichever tasks failed." is amended to match).
The exit-3 test calls `executeCommand` directly with an empty script (0 passed) and reads `--out`;
`runNextClaimClose` fails on any non-zero exit, so only the "met" test uses it (flags before `--`).
**Files:** cmd/agenteval/thresholds.go, cmd/agenteval/thresholds_test.go, cmd/agenteval/thresholds.yaml, cmd/agenteval/run.go, cmd/agenteval/main.go, cmd/agenteval/doc.go, cmd/agenteval/run_test.go, cmd/agenteval/main_test.go
**Read first:** cmd/agenteval/main.go — exitCodeFor, runError; cmd/agenteval/run.go — newRunCommand, checkRunOptions, runRun;
cmd/agenteval/run_test.go — executeCommand, runNextClaimClose; cmd/agenteval/main_test.go — TestExitCodeFor_TellsARunFailureFromBadUsage
**Tests:** table tests of `loadThresholds` (valid, negative, unknown key, missing file); run tests: flags in pairs, unknown name exits 2 before any task, below threshold exits 3 with the report written and the `threshold` line printed once (via `executeCommand`), met exits 0; a `belowThresholdError` → 3 row in `TestExitCodeFor_TellsARunFailureFromBadUsage`. No network or model.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./cmd/agenteval/`
- `go vet ./cmd/agenteval/`
**Commit:** `feat(agenteval): fail a run below the model's pass threshold`

## 2. Scheduled agent-eval workflow — ✅ DONE (2026-11-23)

NOTES (2026-11-23): the setup job also validates AGENT_EVAL_MODELS with jq and fails naming the problem when it is not a JSON list, an entry lacks a name (letters, digits, '.', '_', '-'), base_url, model or key_secret (a secret name), or two entries share a name (a name becomes a file and artifact name); unset, empty or `[]` runs no model job.
NOTES (2026-11-23): matrix values reach the run scripts through job env vars (MODEL_NAME, MODEL_BASE_URL, MODEL), never interpolated into `run:` text; the summary also prints the report's table header and alignment rows above the Total row so it renders as a table (`grep -m 2 '^| '`, safe under pipefail).
NOTES (2026-11-23): timeout-minutes is 195 per eval job (12 tasks x 15m = 180m plus builds); concurrency group agent-eval with cancel-in-progress false; weekly cron Monday 04:00 UTC; Claude Code installed unpinned with npm, commented as deliberate.
NOTES (2026-11-23): python3 has no yaml module on this host, so the parse acceptance ran with go.yaml.in/yaml/v3 v3.0.6 (go.mod's version) from a scratch module instead; actionlint v1.7.7 (installed into the scratchpad) reports nothing.

**What:**
**Goal:** `.github/workflows/agent-eval.yml` runs on `workflow_dispatch` and weekly; with neither
`AGENT_EVAL_MODELS` nor `ANTHROPIC_API_KEY` configured it runs no eval job and passes; each eval
job uploads its report and transcripts, writes the report's Total row to `$GITHUB_STEP_SUMMARY`,
and fails when item 1's threshold check fails. Depends on item 1.
**Approach (assumed at the header base):**
- House style of `ci.yml`: `name:` then a `#` comment block (a separate workflow so an eval failure
  never blocks a release: the release gate reads `ci.yml`'s runs only, Note `adr-0008`),
  top-level `permissions: contents: read`, `actions/checkout@v4`, `actions/setup-go@v6` with
  `go-version-file: go.mod`, `actions/upload-artifact@v4`, `concurrency` so runs never overlap.
- Job `setup`: reads `vars.AGENT_EVAL_MODELS` with `jq`, outputs the matrix JSON and
  `has-models`, and `has-claude` from whether `secrets.ANTHROPIC_API_KEY` is non-empty.
- Job `model` (`if: has-models == 'true'`, `fail-fast: false`, matrix from `fromJSON`):
  `go build -o agenteval ./cmd/agenteval` once, env `AGENT_EVAL_API_KEY: ${{ secrets[matrix.key_secret] }}`,
  runs `./agenteval run --out eval/<name>.md --thresholds cmd/agenteval/thresholds.yaml --name <name> -- ./agenteval agent --base-url <base_url> --model <model> --api-key-env AGENT_EVAL_API_KEY`.
- Job `claude` (`if: has-claude == 'true'`): installs the Claude Code CLI with npm, runs with
  `--name claude -- claude -p --allowedTools Bash`.
- Both: upload `eval/` (report and `.transcripts/`) and append the Total row (grep of
  `| **Total** |`) to the summary, each step `if: always()`; `timeout-minutes` covers 12 tasks at
  the default 15m each, with a comment saying so.
**Regression guard.** The leading # comment block follows fuzz.yml/stress.yml/bench.yml (ci.yml has none); actions, setup-go and permissions follow ci.yml.
The matrix is `matrix: { include: ${{ fromJSON(needs.setup.outputs.matrix) }} }` (a bare list is
rejected); each eval job's `if:` reads the setup job's `has-models`/`has-claude` outputs. Artifacts
are named `agent-eval-${{ matrix.name }}` and `agent-eval-claude`, with `if-no-files-found: ignore`
(upload-artifact@v4 refuses a repeated name); the summary step tolerates a missing report
(`[ -f eval/<name>.md ] && grep … || echo "no report"`).
**Files:** .github/workflows/agent-eval.yml
**Read first:** .github/workflows/fuzz.yml — matrix include, upload-artifact per-target name, header comment;
.github/workflows/bench.yml — defaults shell bash, GITHUB_OUTPUT step; cmd/agenteval/run.go — runRun transcriptsDir, resolveAgentArgv;
cmd/agenteval/report.go — renderReport Total row
**Tests:** none in Go; workflow parses and its scripts are checked below.
**Acceptance:**
- `python3 -c 'import yaml,sys; yaml.safe_load(open(sys.argv[1]))' .github/workflows/agent-eval.yml`
- `grep -n 'workflow_dispatch\|schedule\|--thresholds\|GITHUB_STEP_SUMMARY\|upload-artifact\|has-models\|has-claude' .github/workflows/agent-eval.yml`
- `actionlint .github/workflows/agent-eval.yml` when `actionlint` is installed
**Commit:** `ci(agent-eval): run the agent eval weekly against configured models`

## 3. Document the eval workflow and thresholds — ✅ DONE (2026-11-23)

NOTES (2026-11-23): no CHANGELOG entry — items 1 and 2 already carry entries for the flags and the workflow; this item is docs only.
NOTES (2026-11-23): every `capcom know update` rewrites the stale capcom block in AGENTS.md ("refreshed: AGENTS.md"); restored with `git restore AGENTS.md` each time, as that block is out of scope for this plan. `capcom check` still reports only that pre-existing AGENTS.md:31 problem.
NOTES (2026-11-23): `cmd/agenteval/thresholds.yaml` ships `models: {}`, so any model added to AGENT_EVAL_MODELS (and `claude` once ANTHROPIC_API_KEY is set) fails exit 2 until it gets an entry; docs/agent-eval.md says so and shows an illustrative example, without adding entries.

**What:**
**Goal:** `docs/agent-eval.md` documents the `--thresholds`/`--name` flags, the workflow's
triggers, the `AGENT_EVAL_MODELS` variable (with an example value), the `key_secret` and
`ANTHROPIC_API_KEY` secrets, `cmd/agenteval/thresholds.yaml`, the uploaded artifact, and that CI
reports are never committed (copy noteworthy ones into `docs/reviews/`); Note `agent-eval` names
the workflow and the threshold file. Depends on items 1 and 2.
**Approach (assumed at the header base):** a new `## In CI` section after `## The report and the
transcripts`; flags added under `## Running it`; Note via `capcom know update agent-eval`.
**Regression guard.** docs/agent-eval.md describes exit code 3 in the same wording item 1 puts in run's Long help.
The exit-code paragraph under `## Running it` (today "0 once the report is written, whichever tasks
failed; 1 …; 2 …") gains exit 3: the passed count is below the --name model's threshold.
**Files:** docs/agent-eval.md, .capcom/knowledge/agent-eval.md
**Read first:** docs/agent-eval.md — ## Running it flag table and exit-code paragraph, ### With Claude Code, ### With a local model, ## The report and the transcripts;
.capcom/knowledge/agent-eval.md — body, refs; cmd/agenteval/run.go — newRunCommand Long
**Closes:** scheduled-agent-eval
**Tests:** none (docs).
**Acceptance:**
- `grep -n 'AGENT_EVAL_MODELS\|thresholds.yaml\|ANTHROPIC_API_KEY\|--name' docs/agent-eval.md`
- `grep -n 'passed count is below' docs/agent-eval.md` finds the exit-3 line
- `capcom know show agent-eval` names `agent-eval.yml`; `capcom check` reports no new problem
**Commit:** `docs(agent-eval): document the scheduled workflow and the thresholds`

## 4. internal/export: Issue pages, Issue tree and Ready list — ✅ DONE (2026-11-23)

NOTES (2026-11-23): besides WriteHTML the package exports three sentinel errors (ErrInsideStore, ErrForeignDir, ErrHoldsStore) so item 6 and the tests can tell the refusals apart with errors.Is.
NOTES (2026-11-23): added a refusal the item does not name: a target that holds the store directory (e.g. the worktree root carrying a stray .capcom-export) fails with ErrHoldsStore, since clearing it would delete .capcom.
NOTES (2026-11-23): Ready list from briefing.ReadyList (default options, so ready.hide issues are left out, as capcom ready does) and dep/dependent links from issue.NewDepGraph(index.Nodes(entries)) over the same entries the pages are written for, the second of the two sources the item allows; every page is rendered before the target directory is touched.
NOTES (2026-11-23): the Issue page also shows the close Reason when one is set; an alias dep prints as "<id> (as <alias>)" linked to <id>, an unresolved one as "<dep> (no such issue)" in plain text.

**What:**
Recast at the regression check (2026-11-23).
**Goal:** a new package `internal/export` writes `index.html` (the Ready list, most urgent first,
and a link to `issues.html`), `issues.html` (every Issue as a nested tree with
status and type) and `issues/<id>.html` (title, status, type, priority, labels, body,
deps and dependents as links) into a target dir; the output is byte-identical across runs on the
same tree; a target inside the store's `.capcom` dir is refused; the marker rule of the header holds.
**Approach (assumed at the header base):**
- One deep module: `export.WriteHTML(dir string, st *store.Store, ix *index.Index) error`; callers
  see nothing else. It gathers data only through existing domain APIs — `briefing.ReadyList`,
  `ix.List(index.Filter{IsClosedIncluded: true})` with `outline.Issues`, the issue readers and
  `issue.NewDepGraph`; it never re-derives readiness or ordering. Where a needed helper sits
  privately in `internal/cli` (e.g. `loadShowGraph`), call the domain functions it wraps; do not
  move cli code in this item.
- `html/template` with templates embedded via `embed`; one inline `<style>` per page (light and
  dark via `prefers-color-scheme`), no JavaScript, no external resource; relative links only, so
  the site opens from the file system. Bodies in `<pre>` with `white-space: pre-wrap`.
- No generation time or absolute path in any page. Pages written with `nodefile.WriteFileAtomic`.
- Dir rules: resolve `dir` with `filepath.Abs` and `filepath.EvalSymlinks` of its nearest existing
  ancestor; inside `st.Root` → error naming `.capcom`; exists, non-empty and no `.capcom-export` →
  error; holding the marker → remove every entry but the marker, then write.
**Regression guard.** index.html links only to issues.html in this item — the knowledge.html link is added by item 5 — so every link item 4 writes resolves and its tests may assert that.
The marker means "capcom owns this dir", not "export complete": write it before the first page and
clear every entry except `.capcom-export`, so a failed write never leaves a dir every rerun refuses.
Resolve `st.Root` with `filepath.EvalSymlinks` too and judge containment with `store.RelativePath`
(equality counts as inside). The Log is the `## Log` section of `Issue.Body`: it renders in the body `<pre>`.
Render each dep through `DepGraph.Resolve` (`Judgement.Deps` from `briefing.ReadyListJudged`, or
`issue.NewDepGraph(index.Nodes(entries))`): resolved → link to `issues/<id>.html`, unresolved → plain
text; an Alias dep (ADR 0002) links to the id it resolves to. issues.html uses
`outline.Issues(entries, true)` with `.Rows(nil, outline.UnlimitedDepth)`, so closed and dropped Issues keep their rows.
**Files:** internal/export/export.go, internal/export/issues.go, internal/export/templates/, internal/export/export_test.go
**Read first:** internal/briefing/ready.go — ReadyListJudged, Judgement; internal/outline/outline.go — Issues, Tree.Rows; internal/issue/deps.go — DepGraph.Resolve, DepGraph.Dependents;
internal/index/query.go — Index.List, Filter; internal/nodestore/read.go — IssueReader; internal/store/store.go — RelativePath;
internal/nodefile/atomic.go — WriteFileAtomic; internal/briefing/ready_test.go — indexIssueFiles
**Tests:** direct package tests on a temp store (real files and SQLite, ADR 0007): page set, a closed Issue present with its status on its page and as a row on `issues.html`, dep links resolve to existing files (fixture deps include an alias, linked to its issue, and a missing id, printed as plain text), every link item 4 writes resolves, determinism (two writes, byte compare), refusal inside `.capcom`, refusal of a foreign non-empty dir, marker replace removing a stale page, a rerun into a dir holding the marker and a partial page set accepted, `<script>` absent, a body with `<b>` escaped, the Log printed once.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/export/`
- `go vet ./internal/export/`
**Commit:** `feat(export): write Issue pages, the Issue tree and the Ready list as static HTML`

## 5. internal/export: the Knowledge tree and Note pages

**What:**
Recast at the regression check (2026-11-23).
**Goal:** `WriteHTML` also writes `knowledge.html` (the Note tree, same Notes as `capcom know
tree`'s default) and `knowledge/<id>.html` (title, labels, refs, body); every page links back to
`index.html`; output stays byte-identical across runs. Depends on item 4.
**Approach (assumed at the header base):** `ix.Notes(false)` with `outline.Notes` for the tree; the
note reader `nodestore.NoteReader()` for bodies; templates beside item 4's.
**Regression guard.** item 5 adds the knowledge.html link to index.html and a test that every link on every page resolves to a written file.
**Files:** internal/export/knowledge.go, internal/export/templates/, internal/export/export.go, internal/export/knowledge_test.go
**Read first:** internal/index/notes.go — Index.Notes, NoteEntry, retiredSubtrees; internal/outline/outline.go — Notes, Tree.Rows;
internal/cli/know_tree.go — newKnowTreeCommand, outline; internal/nodestore/read.go — NoteReader, Reader.Read;
internal/knowledge/note.go — Note, IsBackground; internal/briefing/ready_test.go — indexIssueFiles
**Tests:** Note page set matches the default tree, nested Note links resolve, `index.html` links to `knowledge.html`, every link on every page resolves to a written file, a retired Note is absent, determinism still holds.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/export/`
**Commit:** `feat(export): write the Knowledge tree and Note pages`

## 6. capcom export --html command

**What:**
**Goal:** `capcom export --html <dir>` calls `export.WriteHTML`, prints the dir and page count,
supports `--json` with a new result kind listed in `docs/site/reference/json-output.md`, exits with
a usage error without `--html`, and a target inside `.capcom/` is refused with a non-zero exit and
no file written. Depends on items 4 and 5.
**Approach (assumed at the header base):** `(a *app) newExportCommand()` in `internal/cli/export.go`,
added in `newRootCommand` alphabetically, annotation `indexRefresh`, `Long`/`Example` within
`help_text_test.go`'s rules; `doc.go` file map lists `export.go`; `go run ./cmd/gendocs`.
CHANGELOG entry under Added via the item sidecar.
**Regression guard.** `emittedKinds` finds the new `Kind: "export"`, so `TestGolden_KindsCovered` fails
without a golden file: add `internal/cli/testdata/json/export.golden.json` and a `TestGolden_ReadKinds`
case exporting to a dir under the golden store root (outside `.capcom`), so `normaliseGolden` maps
it to `<DIR>/…`; the acceptance `-run` pattern includes `Golden`.
**Files:** internal/cli/export.go, internal/cli/export_test.go, internal/cli/root.go, internal/cli/doc.go, internal/cli/golden_read_test.go, internal/cli/testdata/json/export.golden.json, docs/site/reference/json-output.md, docs/site/reference/cli/
**Read first:** internal/cli/root.go — newRootCommand, indexRefresh; internal/cli/golden_kv_test.go — TestGolden_KindsCovered, emittedKinds;
internal/cli/golden_read_test.go — TestGolden_ReadKinds; internal/cli/golden_test.go — newGoldenStore, normaliseGolden; internal/cli/testharness_test.go — runCLI
**Closes:** html-export
**Tests:** `runCLI` on a seeded store: export into a temp dir and check `index.html` exists with links to every Issue page; second run identical; `--html .capcom/out` refused; `--html` into a foreign non-empty dir refused; `--json` envelope kind; the `export` golden case.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'Export|Help|DocFileMap|Golden' ./internal/cli/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`
- `go run ./cmd/capcom export --html "$(mktemp -d)/site"` in this repo exits 0
**Commit:** `feat(cli): capcom export --html writes a static site`

## 7. Plan the internal/cli split and file its move Issues

**What:**
**Goal:** a plan in this skill's house format under `docs/plans/` classifies every non-test file
and function of `internal/cli` as command wiring or domain logic (with line counts), names target
packages following `internal/ops` / `internal/briefing`, orders the moves so each commit stays
green, and names the tests that move from `runCLI` to direct unit tests; one child Issue per move
step exists under `code-maturity`, each with `plan:` set to that plan. No Go file changes.
**Approach (assumed at the header base):** measure with `wc -l` excluding `_test.go`; one item per
move step in the plan, each self-contained; its header carries the regression-check line
`not run — run implement-plan Write mode step 3 before executing`; file the Issues with one
`capcom apply` file, then `capcom update <id> --plan "<basename>"` per Issue (AGENTS.md).
**Regression guard.** measure line counts with wc -l at execution time and never quote the issue's stale 26k figure; set plan: on each child Issue in the capcom apply file's create ops, using capcom update <id> --plan only for an Issue created without it
(this replaces the Approach's per-Issue `capcom update` step). The outline check is
`capcom list --tree --parent code-maturity --depth 2`: a positional id refuses `--tree`/`--depth`.
**Files:** docs/plans/ (one new plan file), .capcom/issues/code-maturity/ (new child Issues)
**Read first:** .capcom/issues/code-maturity/split-cli-package.md — Acceptance; internal/cli/list.go — listNamedResult, checkNamedFlags, listFlaggedResult;
internal/cli/apply.go — apply; internal/ops/create.go — MissingPlan; internal/config/config.go — PlanPath; internal/check/issues.go — checkIssue
**Closes:** split-cli-package
**Tests:** none (analysis).
**Acceptance:**
- the new plan file exists and holds a classification table and numbered move items
- `for f in internal/cli/*.go; do case $f in *_test.go) ;; *) grep -qF "$(basename "$f")" "<new plan>" || echo "missing $f";; esac; done` prints nothing
- the new plan names its target packages and the `runCLI` tests that move to direct unit tests
- `git diff --name-only <item 7's start commit> -- '*.go'` is empty
- `capcom list --tree --parent code-maturity --depth 2` shows the new children; `capcom show <child>` shows `plan:` for each
- `capcom check` reports no new problem
**Commit:** `docs(plans): plan the internal/cli split and file one Issue per move`
