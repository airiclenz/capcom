# Plan: agenteval call log long lines and the scenario file split

**Goal:** The agent eval harness scores a task whose call log holds a line over 64 KiB, and the
scenarios live in one file per scenario group, each within about 400 lines.
**Date:** 2026-10-29
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** cb476cd

**Regression check (2026-10-29, cb476cd):**
- 2: guard folded (Acceptance checks for the group files and docs; `hasHeading`, `fileExists` and each scenario's consts move with their group)

**Sources:**
- `.capcom/issues/agent-ergonomics/call-log-long-lines.md`
- `.capcom/issues/agent-ergonomics/split-scenario-file.md`
- Note `agent-eval`; `docs/agent-eval.md`; `cmd/agenteval/doc.go` (`# Files` map)

**Writer calls (2026-10-29):**
- **Call log reader:** a `bufio.Reader` read line by line, no line-length cap; the `path:line: ` error wording stays.
- **Scenario groups:** `scenario.go` keeps the shared types and helpers; three group files: `scenario_work.go`, `scenario_shape.go`, `scenario_find.go` (members in item 2).

**Standing requirements:**
- skills: coding-standards
- Tests on the Pi: `GOMEMLIMIT=2GiB`, one package at a time, never `./...` with `-race`.
- Any deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- Splitting `cmd/agenteval/scenarios_test.go`.
- Changing any scenario's prompt, seed, check or id, or the call log format the shim writes.

## 1. Read call log lines of any length — ✅ DONE (2026-10-29)

**Closes:** call-log-long-lines

**What:** fix: `readCallLog` fails `bufio.Scanner: token too long` on a call log line over
64 KiB, such as a call with a huge `--body`, so the task cannot be scored.

**Goal:** `readCallLog` returns a call log line of any length as that call, arguments intact; blank
lines are still skipped, a last line without a trailing newline still reads, and a malformed line
still fails with an error starting `<path>:<line>: `.

**Approach (assumed at the header base):** in `cmd/agenteval/score.go`, `readCallLog` replaces its
`bufio.Scanner` with `bufio.NewReader(file)` and `ReadBytes('\n')` in a loop, counting lines as
now; a final chunk returned with `io.EOF` is still parsed when non-blank. Strip the trailing
`\n` (and a `\r` before it) before the blank check and `json.Unmarshal`. A read error other than
`io.EOF` returns `fmt.Errorf("%s: %w", path, err)` as the scanner error did. Update the
function's doc comment only if it names a limit. No other file changes.

**Files:** `cmd/agenteval/score.go`, `cmd/agenteval/score_test.go`
**Read first:** `cmd/agenteval/score.go` — readCallLog; `cmd/agenteval/score_test.go` — writeCallLog, TestReadCallLog_MalformedLineNamesTheFileAndLine, TestReadCallLog_ReadsEachLineInOrder;
`cmd/agenteval/shim.go` — shimCall, appendCall; `cmd/agenteval/run.go` — runTask (readCallLog caller)

**Tests:** in `cmd/agenteval/score_test.go`, `TestReadCallLog_ReadsALineLongerThanTheScannerLimit`:
write via `writeCallLog` a log of a short call, then a call whose args are
`create --id x --body <strings.Repeat("a", 100<<10)>` built with `json.Marshal` of a `shimCall`,
then a short call; assert three calls, the long one's `Args` equal to what was marshalled.
`TestReadCallLog_ReadsALastLineWithoutANewline`: a log whose last line has no `\n` reads all calls.
The long-line test must fail against the base tree.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'TestReadCallLog|TestScore' ./cmd/agenteval`
- `go vet ./cmd/agenteval`

**Commit:** `fix(agenteval): read call log lines longer than 64 KiB`

## 2. Split the scenarios into one file per group — ✅ DONE (2026-10-29)

NOTES (2026-10-29): doc.go's `# Files` map entries for the group files also name each group's four scenario ids, wrapped onto a continuation line to keep the comment within about 100 columns; the scenario.go entry wraps the same way.

**Closes:** split-scenario-file

**What:**

**Regression guard.** Each scenario's consts move with it (its id consts, `acceptanceHeading`, `plansDir`,
`contextBeforeStartPlanText`). `hasHeading` and `fileExists` are used by one group only (`file-with-dep`,
`move-under-parent`), so they move to `scenario_shape.go`.
Acceptance checks that each group file holds its 4 scenarios and that `doc.go` and `docs/agent-eval.md` name each group file.

**Goal:** `cmd/agenteval/scenario.go` and every `cmd/agenteval/scenario_*.go` file are each at most
about 400 lines; every scenario lives in the group file below; `doc.go`'s `# Files` map lists each
file with its group; `docs/agent-eval.md` names where scenarios live; `go test ./cmd/agenteval`
passes with no test file changed.

**Approach (assumed at the header base):** a pure move, no code change. Each scenario moves whole:
its `…Issues`/`…Notes` vars, its `…Scenario()` func, its `check…` func and their doc comments.
- `scenario.go` keeps: `scenario`, `seedEnv`, `outcome`, `passed`, `failed`, `scenarios`,
  `findScenario`, `seededIssue`, `createIssues`, `seededNote`, `createNotes`, and every helper used
  by scenarios in more than one group file (`isActive`, `loadConfigAndIssues` as the tree
  stands). A helper used by one group moves with it (`isLockBreak`, `hasHeading`, `fileExists`).
- `scenario_work.go` — claiming, locking and closing: `next-claim-close`, `blocked-start`,
  `resume-lost-token`, `reopen-with-reason`.
- `scenario_shape.go` — filing and reshaping the issue tree: `file-with-dep`, `move-under-parent`,
  `rename-id`, `close-children`.
- `scenario_find.go` — finding issues and Notes, and recording a Note: `find-lock-note`,
  `find-by-content`, `context-before-start`, `add-note`.
Each new file gets only the imports it uses (`goimports`/`gofmt` clean). `scenarios()` keeps its
order. In `cmd/agenteval/doc.go`, the `scenario.go` line becomes "the scenario type, the scenario
list and the seed and check helpers they share", plus one line per new file naming its group as
above, in the map's alphabetical order. In `docs/agent-eval.md`, the sentence naming
`cmd/agenteval/scenario.go` names the three group files instead.

**Files:** `cmd/agenteval/scenario.go`, `cmd/agenteval/scenario_work.go`,
`cmd/agenteval/scenario_shape.go`, `cmd/agenteval/scenario_find.go`, `cmd/agenteval/doc.go`,
`docs/agent-eval.md`
**Read first:** `cmd/agenteval/scenario.go` — scenarios, hasHeading, fileExists, isActive, loadConfigAndIssues; `cmd/agenteval/doc.go` — package comment `# Files` map;
`cmd/agenteval/doc_test.go` — TestDocFileMap_MatchesDirectory; `docs/agent-eval.md` — "The scenarios" section

**Tests:** none new; `TestDocFileMap_MatchesDirectory` covers the map, the scenario tests cover the
moved code.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./cmd/agenteval`
- `go vet ./cmd/agenteval && test -z "$(gofmt -l cmd/agenteval)"`
- `wc -l cmd/agenteval/scenario*.go` — each non-test file ≤ about 400 lines
- `git diff --stat HEAD -- cmd/agenteval/*_test.go` prints nothing
- `grep -c 'Scenario() scenario' cmd/agenteval/scenario.go cmd/agenteval/scenario_{work,shape,find}.go` prints 0 for `scenario.go` and 4 for each group file
- `for g in work shape find; do grep -q "scenario_$g.go" cmd/agenteval/doc.go && grep -q "scenario_$g.go" docs/agent-eval.md || echo "missing $g"; done` prints nothing

**Commit:** `refactor(agenteval): split the scenarios into one file per group`
