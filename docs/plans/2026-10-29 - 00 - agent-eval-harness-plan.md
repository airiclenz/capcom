# Plan: the agent evaluation harness

**Goal:** A dev tool, `cmd/agenteval`, runs 12 seeded capcom tasks against any agent command,
judges each from the resulting store files, counts the capcom calls through a PATH shim, and
writes a markdown report. A first report for a local 25–30B model is committed.
**Date:** 2026-10-29
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** bb5722c
**Closes:** agent-eval-harness

**Sources:**
- `.capcom/issues/agent-ergonomics/agent-eval-harness.md`, `.capcom/issues/agent-ergonomics.md` (field findings log)
- Notes `cli-test-harness`, `adr-0007`, `adr-0016`, `store-layout`, `coding-conventions`, `error-messages`
- `cmd/release/main.go`, `cmd/release/doc.go` (dev-tool shape: cobra root, exit codes, doc.go file map)

**Ratified design calls (user, 2026-10-29):**
- **Agent input:** the agent command follows `--`, runs with the prompt on stdin and the seeded repo as its working directory.
- **Local model:** a built-in minimal loop, `agenteval agent --base-url … --model …`, one `shell` tool, used as the agent command.
- **Scenarios:** 12 defined in this plan with stable ids; the field set is mapped onto them later.
- **First report:** the last item runs a local model and commits the report under `docs/reviews/`.

**Writer calls (2026-10-29):**
- **Name:** `cmd/agenteval`; it may import `internal/` (unlike `cmd/release`), to parse store files with capcom's own parsers.
- **Call counting:** the harness copies its own executable to `<tmp>/bin/capcom[.exe]`, first on the agent's PATH; started under the basename `capcom` it is the shim: runs the build under test, passes stdio through, appends one JSON line per call to the call log.
- **Success:** judged only from `.capcom/` files, the lock state under the git dir, and the shim's call log — never the transcript.
- **Prompts:** name ids and goals, never a capcom command; the agent learns capcom from the `AGENTS.md` that `capcom agents` writes in the seed.
- **Exit codes:** 0 when the report is written (failed tasks included), 1 run failed, 2 bad usage — as `cmd/release`.

**Standing requirements:**
- skills: coding-standards; item 9 also forwards manage-llm-server.
- Tests per `cli-test-harness`: `GOMEMLIMIT=2GiB`, one package at a time, never `./...` with `-race`, no `t.Parallel()`; no test reaches a network or a model.
- CI's 90% total coverage floor holds: logic that runs in a subprocess (shim, fake agent) is also unit-tested in process.
- Every file in `cmd/agenteval` is listed in the `# Files` map of `cmd/agenteval/doc.go`.

**Out of scope:**
- Mapping the field agent's nine `capcom-eval` experiments onto these scenarios.
- Repeated runs per task, statistics across runs, a JSON report.
- `mcp-server-design` (unblocked by this plan, not done in it).

**Regression check (2026-10-29, bb5722c):**
- 1: recast — the fake agent's `{prev:<regex>}` placeholder; `.exe` copies on Windows
- 3: guard folded — `.exe` names; `--capcom` resolved absolute; call log and transcript truncated per run; usage checked before runE; transcript file and WaitDelay on timeout
- 4: guard folded — `resume-lost-token` passes the token by `{prev:<regex>}`; `tree-lock` seeded as a child Note, out of the AGENTS.md block
- 6: guard folded — `add-note` script uses `know add` then `know update --body-append`
- 7: guard folded — WaitDelay, empty stdin and a process-group kill bound each command
- 8: guard folded — Acceptance runs `capcom know verify` with no argument
- 9: guard folded — server reached through manage-llm-server or stop; `--out` under docs/skill-runs/; status shown, load confirmed, profile restored
- 1 (re-check): guard folded — `{prev:` placeholder ends at its last `}`; `cmd/agenteval/doc_test.go` pins the `# Files` map
- 4 (re-check): guard folded — `resume-lost-token` script runs `log`, then the token-printing call, then `lock release --token {prev:<regex>}` directly after it

## 1. The agenteval skeleton and the capcom shim — ✅ DONE (2026-10-29)

NOTES (2026-10-29): the shim also exits 1 when the build cannot be started or is killed by a signal (that call is still logged, with exit 1) and when the call log cannot be written (stderr names the log file); the plan's Goal only fixed the missing-variable case.
NOTES (2026-10-29): runE is carried over from cmd/release for the verbs item 3 adds and is exercised only by tests until then; with no verbs yet, `agenteval <anything>` prints help and exits 0. The fake agent and its {prev:<regex>} tests live in main_test.go beside TestMain; the placeholder needs a capturing group, and a regex without one is an error naming the placeholder, as a no-match is.

**What:** Recast at the regression check (2026-10-29).
**Goal:** `cmd/agenteval` builds; started under the basename `capcom` (or `capcom.exe`) it runs the binary named by `AGENTEVAL_CAPCOM` with its own args and stdio, exits with that binary's exit code, and appends `{"args":[…],"exit":n,"ms":n}` as one line to the file named by `AGENTEVAL_CALL_LOG`; a missing variable is an error naming it, exit 1.
**Approach (assumed at the header base):** `main.go` checks `os.Args[0]`'s basename first and calls `runShim(args, getenv, stdin, stdout, stderr) int`; otherwise it builds a cobra root (`newRootCommand`) with no verbs yet, `SilenceErrors`, `exitCodeFor` as in `cmd/release/main.go`. `doc.go` holds the package comment (purpose, usage lines, `# Files` map). `main_test.go` holds a `TestMain` that dispatches on its own basename: `capcom` → `runShim`, `capcom-real` → `cli.Run`, `fake-agent` → a scripted agent reading `AGENTEVAL_FAKE_SCRIPT` (a file of JSON argv arrays, each run as `capcom <argv>` from PATH), so later tests run real subprocesses without `go build`. Shared test helpers live there: `copyTestBinary(t, dir, name)`, the hermetic git env.
**Regression guard.** The fake agent's script lines may hold `{prev:<regex>}`, replaced by the first submatch of <regex> over the previous call's stdout (no match is a script error naming the line), so a script can pass an earlier call's output such as a Lock token to a later call; on Windows every copied test binary carries the `.exe` suffix and the TestMain basename dispatch strips it.
The placeholder is a whole argv element starting `{prev:` and ending at its LAST `}`, the regex being everything between, so a `{n}` quantifier inside it holds (`{prev:token: ([0-9a-f]{32})}` matches a Lock token).
`cmd/agenteval/doc_test.go`, a copy of `TestDocFileMap_MatchesDirectory` from `cmd/release/doc_test.go`, pins that every file in `cmd/agenteval` is listed in the `# Files` map.
**Files:** cmd/agenteval/doc.go; cmd/agenteval/doc_test.go; cmd/agenteval/main.go; cmd/agenteval/shim.go; cmd/agenteval/shim_test.go; cmd/agenteval/main_test.go
**Read first:** cmd/release/main.go — newRootCommand, runE, exitCodeFor; cmd/release/main_test.go — TestMain; cmd/release/runner_test.go — helperExecutable; internal/cli/root.go — Run; internal/cli/testharness_test.go — setHermeticGitEnv (test-only, must be copied, not imported); cmd/release/doc_test.go — TestDocFileMap_MatchesDirectory
**Tests:** `runShim` in process against the test binary copied as `capcom-real`: exit code passed through, stdout passed through, one log line per call with the exact args, missing env vars. One subprocess test: the test binary copied as `capcom` runs `capcom version` and logs it. The fake agent's `{prev:<regex>}` substitution passes a value from one call's stdout to the next, with a regex holding a `{n}` quantifier, and a no-match names the script line; the basename check matches `capcom`/`capcom.exe` alike. `TestDocFileMap_MatchesDirectory` in `doc_test.go`: every file in `cmd/agenteval` is in the `# Files` map.
**Acceptance:**
- `go vet ./cmd/agenteval/`
- `GOMEMLIMIT=2GiB go test -count=1 ./cmd/agenteval/`
**Commit:** `feat(agenteval): add the agent eval dev tool with its capcom call shim`

## 2. Scoring the call log and rendering the report — ✅ DONE (2026-10-29)

NOTES (2026-10-29): re-derived from the assumed `readCallLog(path) ([]call, error)`: the call type already exists as item 1's `shimCall`, so the reader returns `[]shimCall`, and the name collided with item 1's test helper `readCallLog(t, path)`.
NOTES (2026-10-29): consequential edit — cmd/agenteval/main_test.go: made necessary by the production `readCallLog`; the test helper of the same name became `mustReadCallLog`, which wraps it instead of parsing the log a second time.
NOTES (2026-10-29): consequential edit — cmd/agenteval/shim_test.go: made necessary by the production `readCallLog`; its three callers of the test helper now call `mustReadCallLog`.
NOTES (2026-10-29): a call log that does not exist reads as no calls (the shim only creates it on the first call); blank lines are skipped. Besides escaping `|`, a Why cell collapses line breaks and runs of white space to one space so a row stays on one line; the totals row also sums Time; agent arguments that are empty or hold white space, a quote or a backslash are shown Go-quoted.

**What:**
**Goal:** a task's call log scores to calls, failed calls (non-zero exit) and repeated failures (a non-zero call whose args and exit equal an earlier non-zero call of the same task); a run renders to markdown with a header (date, `capcom version` output, the agent argv) and one table `Task | Result | Calls | Failed | Repeated failures | Time | Why`, with a totals row `n/m passed` and summed counts.
**Approach (assumed at the header base):** `score.go`: `readCallLog(path) ([]call, error)` and the pure `score([]call) taskScore`. `report.go`: `type taskResult` (id, passed, why, score, duration) and `renderReport(w, runInfo, []taskResult) error`; a `|` in a why-text is escaped. Both pure, no I/O beyond the log read.
**Files:** cmd/agenteval/score.go; cmd/agenteval/score_test.go; cmd/agenteval/report.go; cmd/agenteval/report_test.go; cmd/agenteval/doc.go
**Read first:** internal/cli/root.go — Run (exit codes 0–4, all non-zero counted failed), newVersionCommand (`capcom <v>` text for the header); cmd/release/doc.go — package comment, # Files map; cmd/release/doc_test.go — TestDocFileMap_MatchesDirectory
**Tests:** table tests for `score` (no calls, all ok, mixed, two identical failures → 1 repeated, same args different exit → 0); a malformed log line names the file and line; golden-string test of `renderReport` for two tasks.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'Score|Report|CallLog' ./cmd/agenteval/`
**Commit:** `feat(agenteval): score call logs and render the markdown report`
Depends on item 1.

## 3. The run verb, the seeded workspace and the first scenario — ✅ DONE (2026-10-29)

NOTES (2026-10-29): re-derived from the assumed `call` type in `check func(repo string, calls []call) outcome`: the call log's line type is item 1's `shimCall`, so checks take `[]shimCall`; `loadIssues` returns the issues keyed by id as `storedIssue` (id, parsed file, whether its status is in a Closed category), and `logEntries` takes one.
NOTES (2026-10-29): the agent's program (argv[0] after `--`) is resolved with `exec.LookPath` + `filepath.Abs` on the harness's own PATH as `--capcom` is, so a relative agent path still names the same program once the agent runs with the seeded repo as its working directory; the report's Agent line keeps the argv as given.
NOTES (2026-10-29): the shim's two new refusals (relative `AGENTEVAL_CAPCOM`, or one that is the shim's own executable by `os.SameFile`) exit 1 without running or logging the call, as its unset-variable refusal already did; an agent that cannot be started fails the run (exit 1), an agent that exits non-zero only gets its exit status noted at the end of its transcript.
NOTES (2026-10-29): tests added beyond the item's list: a `--timeout 1ns` run fails the task with why `timed out`; an unstartable agent fails the run with exit 1; `checkNextClaimClose` on a directory with no store fails; `loadNotes` and `lockState` read a Note and a Tree lock taken with the real capcom in a seeded workspace. Package coverage 82.8%, module total 91.8% (floor 90).

**What:**
**Goal:** `go run ./cmd/agenteval run --out <file.md> [--capcom <path>] [--only <id,…>] [--timeout 15m] -- <agent argv…>` runs every scenario (or the `--only` ones) and writes the report to `--out`, each agent's stdout and stderr to `<out stem>.transcripts/<id>.log` and its call log to `<id>.calls.jsonl` there; no argv after `--` or an unknown `--only` id is exit 2. Without `--capcom` it builds `./cmd/capcom` from the working directory into a temp dir.
**Approach (assumed at the header base):** `workspace.go`: per task a temp dir with `git init`, `user.name`/`user.email` set in the repo, `capcom init` and `capcom agents` run with the build under test (not through the shim), the scenario's seed, one commit; `bin/` holds the shim copy (`os.Executable`). The agent runs with cwd = repo, prompt on stdin, `PATH` = bin first, `AGENTEVAL_CAPCOM`/`AGENTEVAL_CALL_LOG` set, killed at `--timeout` (a timeout fails the task with why `timed out`). `scenario.go`: `type scenario struct{ id, prompt string; seed func(seedEnv) error; check func(repo string, calls []call) outcome }` and `scenarios()`; `store.go`: `loadIssues(repo)` (walks `.capcom/issues`, `issue.Parse`), `loadNotes(repo)`, `logEntries(issue)`, `lockState(repo)` via `internal/lock`. First scenario `next-claim-close`: three ready Issues of priority 1, 2, 3; prompt "work on the most urgent ready issue: claim it, note progress, close it with a reason"; passes when only the P1 Issue is closed, with a reason and one or more Log entries.
**Regression guard.** The copied-binary names `capcom-real`, `fake-agent` and the shim `bin/capcom` all take the `.exe` suffix on Windows, as item 1 decides. `--capcom` is resolved with `exec.LookPath` + `filepath.Abs` on the harness's own PATH before `bin/` is prepended, and `runShim` refuses (exit 1) an `AGENTEVAL_CAPCOM` that is `os.Executable` or not absolute. `run` creates/truncates each task's `<id>.calls.jsonl` and transcript before starting its agent. The `--` argv and the `--only` ids are checked in cobra Args/PreRunE (outside `runE`), as `cmd/release`'s usage errors are. The agent's `cmd.Stdout`/`Stderr` are the transcript `*os.File` itself and `cmd.WaitDelay` is set, so a timeout returns with why `timed out` even when a grandchild holds the output.
**Files:** cmd/agenteval/run.go; cmd/agenteval/run_test.go; cmd/agenteval/workspace.go; cmd/agenteval/scenario.go; cmd/agenteval/scenarios_test.go; cmd/agenteval/store.go; cmd/agenteval/main.go; cmd/agenteval/doc.go; cmd/agenteval/shim.go; cmd/agenteval/shim_test.go
**Read first:** internal/store/store.go — Open, IssuesDir, KnowledgeDir; internal/lock/worktree.go — ForWorktree, Worktree.Read; internal/issue/file.go — Parse; internal/issue/merge.go — LogEntries; internal/knowledge/note.go — Parse
**Tests:** `run` end to end with `--capcom` = test binary as `capcom-real` and agent = test binary as `fake-agent`: a correct script passes `next-claim-close`, an empty script fails it with a why; the report and transcript files exist; usage errors exit 2, an unknown `--only` id among them; a second `run` with the same `--out` gives the second report the same counts as the first; `--capcom` given as a relative path is resolved absolute; `runShim` refuses its own executable or a relative `AGENTEVAL_CAPCOM` with exit 1. `scenarios_test.go` gets a table helper `runScenario(t, id, script) outcome` the later items reuse.
**Acceptance:**
- `go vet ./cmd/agenteval/`
- `GOMEMLIMIT=2GiB go test -count=1 ./cmd/agenteval/`
**Commit:** `feat(agenteval): run seeded scenarios against an agent command`
Depends on item 2.

## 4. Scenarios: filing, finding a Note, resuming a lock, finding by content — ✅ DONE (2026-10-29)

NOTES (2026-10-29): re-derived from the regression guard's `{prev:token: ([0-9a-f]{32})}` example: `capcom start` prints the Lock token as `token <hex>` with no colon, so the `resume-lost-token` script releases with `{prev:token ([0-9a-f]{32})}`.
NOTES (2026-10-29): `know add` takes a body only from a file, so the seed's `createNotes` adds each Note and then writes its body with `know update --body-append`; `know add` refreshes the AGENTS.md block, which is why `tree-lock` must sit below a top-level Note. `find-by-content` "others unchanged" is judged on priority, labels and closed state; `file-with-dep` checks deps and the `## Acceptance` heading (a whole line) as the Goal states, not the `## Why` the prompt asks for. A `lock break` is recognised by the call's first two non-flag arguments, so `--json lock break` counts.
NOTES (2026-10-29): tests added beyond the item's list: a wrong-dep, no-Acceptance, wrong-Note, broken-lock, release-without-log, missing-label and extra-issue fail case per scenario; `isLockBreak` and `hasHeading` tables; each new check on a directory with no store fails. All new test names hold `Scenario` (or are pure-function tables) so the Acceptance `-run Scenario` selects them. scenario.go is now 534 lines, past the ~400-line guide; items 5 and 6 add seven more scenarios to it, so splitting it is a call for one of them or a follow-up.

**What:**
**Goal:** `scenarios()` adds four, each passing for a correct fake-agent script and failing for an empty one:
- `file-with-dep`: seed `auth-tokens`; prompt asks for an Issue for rate limiting the login endpoint that cannot start before `auth-tokens`, with a why and an Acceptance section. Passes: one new Issue, deps include `auth-tokens`, body has `## Acceptance`.
- `find-lock-note`: seed five Notes, one (`tree-lock`, non-obvious title) explains the lock; prompt asks to record that Note's id in a Log entry on `onboarding`. Passes: an `onboarding` Log entry contains `tree-lock`.
- `resume-lost-token`: seed `capcom start cache-layer`, token not given; prompt says the token was lost after a restart: resume, log progress, release. Passes: lock free, new `cache-layer` Log entry, no `lock break` in the call log.
- `find-by-content`: seed eight Issues; prompt names one only by content ("flaky upload retries"): set priority 1 and label `urgent`. Passes: that Issue P1 with the label, the others unchanged.
**Approach (assumed at the header base):** one seed and one check func per scenario in `scenario.go`, checks through `store.go`'s helpers.
**Regression guard.** `{prev:<regex>}` reads only the immediately preceding call's stdout, so the `resume-lost-token` correct script runs its `capcom log` entry first, then the call that prints the token (`capcom start cache-layer` or `capcom lock acquire` with the seeded purpose), then `capcom lock release --token {prev:<regex>}` directly after it. `find-lock-note` seeds `tree-lock` as a child of a neutral top-level Note (`topNotes` in internal/agents/block.go skips a Note whose ParentID is set), so the seed keeps it out of the AGENTS.md block's "Knowledge tree, top level" list.
**Files:** cmd/agenteval/scenario.go; cmd/agenteval/scenarios_test.go
**Read first:** internal/lock/token.go — SessionToken; internal/cli/lock.go — newLockReleaseCommand, newLockBreakCommand; internal/agents/block.go — topNotes; internal/cli/start.go — start; internal/cli/create.go — newCreateCommand (--dep, --why, --acceptance); internal/cli/update.go — newUpdateCommand (--priority, --add-label); internal/cli/search.go — newSearchCommand
**Tests:** `runScenario` pass and fail case per scenario; the `find-lock-note` seed's AGENTS.md does not name `tree-lock`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run Scenario ./cmd/agenteval/`
**Commit:** `feat(agenteval): add filing, Note, lock and search scenarios`
Depends on item 3.

## 5. Scenarios: context, blockers, moving and renaming — ✅ DONE (2026-10-29)

NOTES (2026-10-29): `seededIssue` gains optional `deps` and `plan` fields that `createIssues` passes as `--dep` and `--plan`; "active" means a status in the config's doing category, read through the new `isActive` and `loadConfigAndIssues` helpers kept in scenario.go (store.go is not in this item's files). The context-before-start Note applies through the title search, not a ref, so the seed writes no source file; a test confirms `capcom context export-csv` names both the plan path and the Note.
NOTES (2026-10-29): tests added beyond the item's list: log-without-start, log-without-plan, stop-at-refusal, drop-dep-and-start, rename-instead-of-move, file-left-at-the-top (built by hand), new-issue-instead-of-rename and dropped-dependent-dep fail cases, plus each new check on a directory with no store. scenario.go is now 863 lines, double the ~400-line guide; item 6 adds three more scenarios, so splitting it (one file per scenario group, each listed in doc.go's `# Files` map) is due there or as a follow-up.

**What:**
**Goal:** `scenarios()` adds four, each passing for a correct script and failing for an empty one:
- `context-before-start`: seed `export-csv` with a plan file under `docs/plans/` linked by `--plan` and a Note that applies; prompt: start it and log the name of its plan. Passes: `export-csv` active, a Log entry holds the plan basename.
- `blocked-start`: seed `deploy` depending on open `ci-green`; prompt: start `deploy`, and if it cannot start, start what blocks it. Passes: `ci-green` active, `deploy` not active.
- `move-under-parent`: seed parent `billing` and top-level `invoice-pdf`; prompt: put `invoice-pdf` under `billing`. Passes: `.capcom/issues/billing/invoice-pdf.md` exists, the old path does not.
- `rename-id`: seed `fix-thing` with two dependents; prompt: rename it to `fix-upload-timeout`, keeping references working. Passes: new file exists with alias `fix-thing`, both dependents' deps name `fix-upload-timeout`.
**Approach (assumed at the header base):** as item 4.
**Files:** cmd/agenteval/scenario.go; cmd/agenteval/scenarios_test.go
**Read first:** internal/cli/start.go — refuseUnstartable; internal/cli/context.go — contextResult; internal/cli/node_verbs.go — newMvCommand, move; internal/nodestore/descriptor.go — IssueDescriptor refs (RefDeps); internal/nodestore/codec.go — setAliases; internal/config/config.go — templateBody (statuses, plans); internal/ops/ops.go — Claim
**Tests:** `runScenario` pass and fail case per scenario.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run Scenario ./cmd/agenteval/`
**Commit:** `feat(agenteval): add context, blocker, move and rename scenarios`
Depends on item 4.

## 6. Scenarios: Knowledge, reopening and closing a Parent's children — ✅ DONE (2026-10-29)

NOTES (2026-10-29): `seededIssue` gains optional `parent`, `isStanding` and `closeReason` fields; `createIssues` passes them as `--parent` and `--standing`, and closes an issue right after creating it when it has a close reason. `checkAddNote` walks `.capcom/knowledge/testing/` at any depth with `knowledge.Parse`, since `loadNotes` keys by id and drops the path. The add-note pass script uses `know add --parent testing` then `know update --body-append` (both flags checked against `--help`).
NOTES (2026-10-29): tests added beyond the item's list: add-note at the top level, appended to `testing` itself, and a child without the fact; reopen-with-reason log-without-reopen, plus an open issue with no Log entry built by hand (no CLI path reopens without logging); close-children leaving one child open, closing the grouping issue after `--no-standing`, and only clearing the standing mark; each new check on a directory with no store. scenario.go is now 1074 lines; splitting it into scenario-group files listed in doc.go's `# Files` map is still due as a follow-up.

**What:**
**Goal:** `scenarios()` holds 12 with unique ids; the three added here pass for a correct script and fail for an empty one:
- `add-note`: seed Note `testing`; prompt: record in Knowledge, under the testing Note, that tests here run with `GOMEMLIMIT=2GiB`. Passes: a Note under `.capcom/knowledge/testing/` whose body contains `GOMEMLIMIT=2GiB`.
- `reopen-with-reason`: seed closed `login-bug` (reason `fixed`); prompt: it was closed by mistake, the bug still reproduces; reopen it saying why. Passes: not closed, a new Log entry.
- `close-children`: seed standing `v2-migration` with four children, one closed; prompt: the remaining work under it is done; close it all except the grouping issue. Passes: every child closed with a reason, `v2-migration` open and standing.
**Approach (assumed at the header base):** as item 4; a test asserts 12 unique ids.
**Regression guard.** The `add-note` correct script creates the Note with `capcom know add` and writes its body with `capcom know update --body-append` (know add has no body flag; verify both flag spellings against `--help` at run time).
**Files:** cmd/agenteval/scenario.go; cmd/agenteval/scenarios_test.go
**Read first:** internal/ops/ops.go — Reopen, Closer.Close; internal/issue/mutate.go — AppendLog; internal/cli/reopen.go — newReopenCommand; internal/cli/close.go — newCloseCommand; internal/cli/know_add.go — newKnowAddCommand (only --body-file); internal/cli/know_update.go — --body-append; internal/knowledge/note.go — Parse
**Tests:** `runScenario` pass and fail case per scenario; the id-count test.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run Scenario ./cmd/agenteval/`
**Commit:** `feat(agenteval): add Knowledge, reopen and child-closing scenarios`
Depends on item 5.

## 7. The built-in agent loop for OpenAI-compatible endpoints — ✅ DONE (2026-10-29)

NOTES (2026-10-29): the shell argv and the process-group kill live in two build-tagged files beside agent.go, agent_unix.go (`//go:build unix`: sh -c, Setpgid, Cancel kills the group) and agent_other.go (`//go:build !unix`: cmd /C, default kill), because syscall.SysProcAttr.Setpgid and syscall.Kill do not compile on Windows; both are listed in the doc.go file map.
NOTES (2026-10-29): when the last of --max-turns replies still calls tools, those calls are run before the loop stops with exit 1; the output-cut line and the timeout line are appended to the tool result so the model sees both.

**What:**
**Goal:** `agenteval agent --base-url <url> --model <name> [--api-key-env OPENAI_API_KEY] [--max-turns 50]` reads the prompt on stdin, posts to `<base-url>/chat/completions` with one function tool `shell` (`{"command": string}`), runs each call in the working directory, returns its combined output (cut to 16 KiB, with the exit code) as the tool result, and stops on a reply with no tool calls or at `--max-turns` (exit 1, saying so). Every turn is printed to stdout, so it is the transcript. The system message is a fixed short brief plus `AGENTS.md` from the working directory when present.
**Approach (assumed at the header base):** `agent.go` with an injected `http.Client` and base URL; shell via `sh -c` on Unix, `cmd /C` on Windows, 2-minute timeout per command; the API key header only when the env var is non-empty. Not started under the `capcom` basename, so the shim check in `main.go` does not catch it.
**Regression guard.** Each shell command runs with `cmd.WaitDelay` set (Go 1.25, go.mod:3), stdin empty, and on Unix in its own process group killed as a group at the timeout, so the per-command timeout bounds the turn even when the command (an editor from `git commit`, a `cmd &`, the shim's child capcom) leaves a process holding the output pipe.
**Files:** cmd/agenteval/agent.go; cmd/agenteval/agent_test.go; cmd/agenteval/main.go; cmd/agenteval/doc.go
**Read first:** cmd/release/main.go — newRootCommand, exitCodeFor; cmd/release/doc.go — the `# Files` map, TestDocFileMap_MatchesDirectory; cmd/agenteval/shim.go — runShim (item 1); cmd/agenteval/main.go — the basename check, newRootCommand (items 1, 3)
**Tests:** `httptest.Server` scripted replies: a tool call runs `echo` and its output reaches the next request; a no-tool reply ends with exit 0; max turns exits 1; a non-200 reply names the status; `AGENTS.md` lands in the system message; with a short injected per-command timeout, a command that leaves a background `sleep` running returns when the timeout fires (Unix).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run Agent ./cmd/agenteval/`
**Commit:** `feat(agenteval): add a minimal agent loop for OpenAI-compatible endpoints`
Depends on item 1.

## 8. Documenting the harness — ✅ DONE (2026-10-29)

**What:**
**Goal:** `docs/agent-eval.md` explains running the harness (with `claude -p`, with `agenteval agent` against a local endpoint), the 12 scenarios by id, how success and the counts are judged, and where the report and transcripts land; a top-level Knowledge Note `agent-eval` points at it and at `cmd/agenteval`; `AGENTS.md`'s opening paragraph names `cmd/agenteval` among the dev tools; the managed block is refreshed with `capcom agents`.
**Approach (assumed at the header base):** write the doc, `capcom know add` the Note (`refs: [cmd/agenteval/doc.go, docs/agent-eval.md]`), edit the paragraph outside the `BEGIN CAPCOM` markers, run `capcom agents`, `capcom check`.
**Regression guard.** `capcom know verify` takes no argument (`capcom know verify agent-eval` exits 2, unknown command, at bb5722c); the Acceptance runs `capcom know verify`, which checks every Note's refs, the new Note's among them.
**Files:** docs/agent-eval.md; .capcom/knowledge/agent-eval.md; AGENTS.md
**Read first:** AGENTS.md — the opening paragraph above `<!-- BEGIN CAPCOM -->`, the "Knowledge tree, top level" list in the block; internal/agents/block.go — Sync, topNotes, render; capcom know add — `--ref` (repeatable), `--body-file`, which refreshes the AGENTS.md block itself; .capcom/knowledge/docs-layout.md — where docs live
**Tests:** none (docs).
**Acceptance:**
- `capcom check`
- `capcom know verify`
- `grep -n agenteval AGENTS.md`
**Commit:** `docs(agenteval): document the agent eval harness`
Depends on item 7.

## 9. The first report for a local model

**What:**
**Goal:** `docs/reviews/agent-eval-<YYYY-MM-DD>-<model-slug>.md` exists, written by `agenteval run` over all 12 scenarios with `agenteval agent` against a local 25–30B model, and `agent-eval-harness` is closed by the closeout.
**Approach (assumed at the header base):** use the manage-llm-server skill to list profiles and load a 25–30B model; build agenteval to a temp path; `go run ./cmd/agenteval run --out docs/reviews/agent-eval-….md -- <built agenteval> agent --base-url <endpoint>/v1 --model <name>`; commit the report only (transcripts stay local). No 25–30B profile available, or the server will not load one, is a stop to report, not a substitute model.
**Regression guard.** Reach the model server through the manage-llm-server skill — `llama-launcher` on PATH, else its MCP adapter as that skill describes; when neither is reachable, or no 25–30B profile loads, stop and report rather than substituting a model or host. Per that skill's safety rules, show `status`/`list` first, get the user's confirmation before `load`, note the active profile and restore it after the run. `--out` points into the gitignored run directory under `docs/skill-runs/`, so the transcripts stay out of the tree, and only the report `.md` is copied into `docs/reviews/`.
**Files:** docs/reviews/agent-eval-<date>-<model-slug>.md
**Read first:** ~/.claude/skills/manage-llm-server/SKILL.md — safety rules, `list --json`, `status --json`, `load`; .gitignore — /docs/skill-runs/; cmd/agenteval/run.go — `--out`, the `<out stem>.transcripts/` layout (item 3); docs/reviews/ — only archived/ exists, so the Acceptance glob matches one file
**Tests:** none (a run).
**Acceptance:**
- `test -f docs/reviews/agent-eval-*.md && grep -c '^| ' docs/reviews/agent-eval-*.md`
**Commit:** `docs(reviews): add the first agent eval report for a local model`
Depends on items 6 and 8.
