# Did-you-mean suggestions and capcom next

**Goal:** a mistyped command, subcommand or flag names the closest valid one; `capcom next`
prints the top Ready Issue in full.
**Date:** 2026-10-19
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 1cb592c

**Regression check (2026-10-19, 1cb592c):**
- 1: guard folded (cobra suggestion distance, parents' Args, unknown command before unknown flag, regenerated reference, `related`, `SUGGESTION=` docs, full package runs)
- 2: guard folded (extends item 1's flag error func, `errors.As` on `*pflag.NotExistError`, full package run)
- 3: guard folded (doc.go file map, Example comments, typed-nil `data`, exit 4 through `Rendered`, `--kv`, full package runs)
- 4: guard folded; yields to the upgrade note house pattern (CHANGELOG.md:27, CHANGELOG.md:103)

**Sources:**
- `.capcom/issues/agent-ergonomics/suggest-commands-flags.md`
- `.capcom/issues/agent-ergonomics/next-command.md`
- Notes `cli-test-harness`, `error-messages`, `coding-conventions`; `docs/site/reference/json-output.md`

**Ratified design calls** (the user, 2026-10-19):
- **next, nothing Ready:** exits with a new code 4 (`ExitNothingReady`), used for nothing else.
- **next --json, nothing Ready:** a success envelope, kind `next`, `data` null; not an error.
- **Suggestion in --json:** one string field, `error.suggestion`, the closest candidate only,
  omitted when none is close.

**Standing requirements:**
- skills: coding-standards
- CLI tests go through `runCLI`/`runCLIWith`, never `t.Parallel()` (Note `cli-test-harness`).
- Tests on this host: `GOMEMLIMIT=2GiB`, one package at a time, never `./...` with `-race`.
- A help-text or command change regenerates the committed reference: `go run ./cmd/gendocs`.

**Out of scope:**
- Suggestions for unknown ids or bad flag values (`suggest-ids-values`).
- `capcom start` / `finish` and the Lock token (`start-command`, `lock-token-recall`).
- The worktree path in the UI tab bar (`ui-path-label`, plan `2026-10-19 - 02 - ui-path-label-plan`).

## 1. Suggest the closest command or subcommand — ✅ DONE (2026-10-19)

NOTES (2026-10-19): the shared group body is `withSubcommandDispatch` in internal/cli/root.go (also applied to the root); the root flag error func is `unknownCommandOnFlagError`, and the shared distance const is `suggestionDistance` for item 2 to reuse.
NOTES (2026-10-19): `capcom --kv shwo x` still reports `unknown flag: --kv` (cobra strips `shwo` as the flag's value before the word is seen), as it did before this item; `capcom shwo x --kv` gets the suggestion.
NOTES (2026-10-19): cobra's SuggestionsFor returns candidates in command order, not by distance (`know tre` got `know rm`), so unknownCommandError picks the closest with an unexported Levenshtein `editDistance` (case-insensitive, earliest on a tie) in internal/cli/root.go; item 2 can reuse it rather than add its own.

**What:**
**Goal:** `capcom shwo x` and `capcom know shwo x` fail with exit 2, and the message names
`show` / `know show` as `did you mean ...?`; `--json` carries it as `error.suggestion`;
a word with no close candidate gets no suggestion; `capcom know`, `capcom dep`, `capcom lock`
with no arguments still print their help and exit 0.
**Approach (assumed at the header base):**
- `output.Error` gains a `Suggestion string`; `ErrorBody` gains `Suggestion string
  \`json:"suggestion,omitempty"\``, filled by `errorBody`. The text renderer appends
  `; did you mean <suggestion>?` to the message line; `--kv` adds a `SUGGESTION=` line before
  `STATUS=` when set. The suggestion is the full command path below `capcom` (`know show`).
- Drop `DisableSuggestions: true` from `newRootCommand` in `internal/cli/root.go`. cobra's own
  multi-line "Did you mean this?" text must not reach the message: build the unknown-command
  error in capcom from `cmd.SuggestionsFor(word)` (first entry only), not by parsing cobra's text.
- Parent commands without a body (`know`, `dep`, `lock`) currently print help and exit 0 on an
  unknown subcommand. Give each a shared `RunE` that prints help for no arguments and otherwise
  returns the same unknown-command usage error, with suggestion. One helper, used by all three.
- Document `error.suggestion` in `docs/site/reference/json-output.md` (the error section).

**Regression guard.**
- `cmd.SuggestionsFor` returns nothing while `SuggestionsMinimumDistance` is 0 (cobra only sets 2 inside
  `findSuggestions`): the helper sets `cmd.SuggestionsMinimumDistance = 2`, one named const shared with
  item 2's threshold, before it calls `SuggestionsFor`.
- The parents carry `Args: cobra.NoArgs`, whose untyped error runs before any `RunE`, and the root's nil Args
  lets `legacyArgs` build cobra's text in `Find`: the shared helper also sets `Args: cobra.ArbitraryArgs` on
  each parent, and the root gets the same helper and `ArbitraryArgs`.
- A runnable root parses flags before args, so `capcom updat x --status active` would report
  `unknown flag: --status`: set a root flag error func (item 2 extends it) that, when the first entry of
  `cmd.Flags().Args()` names no subcommand of a parent, returns the unknown-command error.
- The rule, not a list: every command with subcommands and no body (`grep -n AddCommand internal/cli/*.go`
  outside root.go: `know`, `dep`, `lock`, `related`) gets the helper.
- A `RunE` makes these commands Runnable, so `doc.GenMarkdown` adds a UseLine block: regenerate
  `docs/site/reference/cli/capcom*.md` with `go run ./cmd/gendocs`.
- Document `SUGGESTION=` in the `--kv` section of `json-output.md`, beside `error.suggestion` in Errors.
- Add a full single-package run of each touched package (`GOMEMLIMIT=2GiB go test ./internal/cli/` and
  `./internal/cli/output/`) to Acceptance, beside the `-run` subsets, so package-wide guards (doc.go file
  map, help-text rules) run.

**Files:** `internal/cli/output/errors.go`, `internal/cli/output/output.go`,
`internal/cli/output/output_test.go`, `internal/cli/root.go`, `internal/cli/know.go`,
`internal/cli/dep.go`, `internal/cli/lock.go`, `internal/cli/related.go`, `internal/cli/cli_test.go`,
`docs/site/reference/json-output.md`, `docs/site/reference/cli/capcom*.md` (regenerated)
**Read first:** `internal/cli/root.go` — newRootCommand, Run; `internal/cli/output/output.go` — errorBody,
textRenderer.RenderError, kvRenderer.RenderError; `internal/cli/output/errors.go` — Error, UsageErrorf;
`internal/cli/know.go` — newKnowCommand; `internal/clidoc/reference_test.go` — TestReferenceUpToDate
**Tests:** CLI tests, text and `--json` each: `shwo x` → `show`; `know shwo x` → `know show`;
`dep lst x` → `dep list`; `related ad x y` → `related add`, exit 2; `updat x --status active` →
`update`; `zzzzqq` → no suggestion, no `suggestion` key; `know` alone → help, exit 0. Output tests:
`suggestion` omitted when empty; `SUGGESTION=` precedes `STATUS=`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/output/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Suggest|Unknown'`
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `GOMEMLIMIT=2GiB go test ./internal/clidoc/`
- `go vet ./internal/cli/...`
**Commit:** `feat(cli): suggest the closest command for an unknown command or subcommand`

## 2. Suggest the closest flag — ✅ DONE (2026-10-19)

NOTES (2026-10-19): re-derived from the assumption that suggest.go writes its own distance function: item 1 already added `editDistance` and `closestName` in internal/cli/root.go, so suggest.go reuses both (left in root.go, not moved) and holds only the flag pick (`unknownFlagError`, `closestFlagName`, `nearFlagNames`, `isNearName`).
NOTES (2026-10-19): consequential edit — internal/cli/doc.go: made necessary by the new internal/cli/suggest.go (the file map test requires every file listed).
NOTES (2026-10-19): consequential edit — docs/site/reference/json-output.md: made necessary by `error.suggestion` now also carrying a flag (the section described commands only).
NOTES (2026-10-19): Levenshtein counts a transposition as 2, so `--statsu` ties at distance 2 with any 4-letter flag two deletions away (e.g. a `--stat`); the tie goes to name order, as the plan asks. A 1–2 letter mistyped flag is within distance 2 of many short flags, the same looseness cobra's command suggestions have.

Depends on item 1.

**What:**
**Goal:** `capcom update x --statsu active` fails with exit 2 and `unknown flag: --statsu; did
you mean --status?`; `--json` carries `"suggestion":"--status"`; candidates are the command's
own and inherited flags (so `--jsno` suggests `--json`); a flag with no close candidate gets none;
an unknown shorthand (`-z`) gets none.
**Approach (assumed at the header base):**
- Set a flag error func on the root (`root.SetFlagErrorFunc`, inherited by every command) that
  recognises pflag's `unknown flag: --name` error, picks the closest name among
  `cmd.Flags()` and `cmd.InheritedFlags()` (hidden flags excluded), and returns a usage
  `output.Error` with `Suggestion` set. Other flag errors pass through unchanged.
- Distance: Levenshtein, case-insensitive, also accepting a candidate the typed name is a prefix
  of, with the same threshold cobra uses for commands (`SuggestionsMinimumDistance`, 2). No
  library has it exported; write one small unexported function in `internal/cli`, shared with
  item 1's helper only if item 1 needed its own (it should not: item 1 uses cobra's).

**Regression guard.** Extend item 1's root flag error func rather than replacing it, recognise the
unknown flag with `errors.As` on `*pflag.NotExistError`, never by matching its text; add a full
`GOMEMLIMIT=2GiB go test ./internal/cli/` run to Acceptance.

**Files:** `internal/cli/root.go`, `internal/cli/suggest.go` (new), `internal/cli/suggest_test.go`
(new), `internal/cli/cli_test.go`
**Read first:** `internal/cli/root.go` — newRootCommand, Run; `internal/cli/flag_once.go` —
guardSingleValueFlags, onceValue.Set; `github.com/spf13/pflag@v1.0.9 errors.go` — NotExistError.GetSpecifiedName,
GetSpecifiedShortnames; `internal/cli/read_test.go` — TestList_KV_FailsAsUnknownFlag; `internal/cli/testharness_test.go` — runCLI
**Tests:** unit tests of the distance and the pick (tie broken by flag name order); CLI tests,
text and `--json`: `update x --statsu active` → `--status`; `ready --jsno` → `--json`;
`ready --zzzzqq` → none; `ready -z` → none.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Suggest|Unknown|Flag'`
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `go vet ./internal/cli/...`
**Commit:** `feat(cli): suggest the closest flag for an unknown flag`
**Closes:** suggest-commands-flags

## 3. capcom next prints the top Ready Issue — ✅ DONE (2026-10-19)

NOTES (2026-10-19): show.go's shared tail (showResult, the Elsewhere report of the shown files, emit) is extracted into `emitShown`, which show and next both call, as the plan's approach allows.
NOTES (2026-10-19): next offers --kv as show does; with nothing Ready it writes no records (`KV: []output.KVRecord{}`) and exits 4. With nothing Ready the Elsewhere report covers the whole issue tree, as ready's does; with an Issue found it covers that Issue's file, as show's does. ready's dropped-dependency warnings are not repeated by next.
NOTES (2026-10-19): the exit-4 failure is `output.NothingReadyErrorf` (code not_found, unexported `isNothingReady` mark), always returned through `output.Rendered`; the Envelope doc comment (output.go) and Run's doc comment (root.go) now name the null data and exit code 4.

**What:**
**Goal:** `capcom next` prints, in `capcom show` format, the Issue on the first line of
`capcom ready` given the same filters; `--json` gives `show`'s envelope for it; with nothing Ready
it prints `nothing is ready to work on` and exits 4, documented in its `--help`, and `--json`
gives kind `next` with `data` null and exit 4.
**Approach (assumed at the header base):**
- New `internal/cli/next.go`, `newNextCommand`, registered in `newRootCommand`. It takes the
  filter flags of `ready` that make sense for one Issue — `--label`, `--no-label`, `--planned`,
  `--unplanned`, `--show-hidden`, `--no-elsewhere` — reusing `readyOptions.listOptions` with
  `Limit: 1` and `briefing.ReadyList`. No copy of the ordering.
- The found Issue is rendered by calling `showResult([]string{id})` and emitting as `show`
  does; the Elsewhere report follows `show`'s. Extract the shared emit tail from `show.go`'s
  command body if `next` would otherwise copy it.
- `output.ExitNothingReady = 4` beside the other exit codes, reached through a typed result,
  never `os.Exit`. Add the row to the exit-code table in `docs/site/reference/json-output.md`.
- Regenerate `docs/site/reference/cli` with `go run ./cmd/gendocs`.

**Regression guard.**
- Add a `next.go` line to the `internal/cli/doc.go` file map (TestDocFileMap_MatchesDirectory), and end every
  Example line with a `# comment` (a bare `  capcom next` fails TestHelpText_EveryCommand_SetsLongAndExample).
- `Envelope.Data` is `omitempty`: set Data to a typed nil (e.g. `[]showData(nil)` or `json.RawMessage("null")`),
  assert the raw key `"data":null` is present, and add the null-on-nothing-ready case to json-output.md's `data` row.
- No `*Error` exits 4 today: emit `Result{Kind:"next", Text:"nothing is ready to work on\n", Data: typed nil}`, then
  return `output.Rendered(e)` where `e` carries exit 4 (new unexported field + constructor in errors.go); add an
  `ExitCode` row to errors_test.go; fix the `Run` (root.go) and `ExitCode` doc comments.
- `emit` rejects `--kv` on a result with nil KV: if `next` offers `--kv`, give the nothing-ready result
  `KV: []output.KVRecord{}` and test `--kv` exit 4; if not, keep `--kv` out of its Long and Example.
- Add full single-package runs of `./internal/cli/` and `./internal/cli/output/` to Acceptance.

**Files:** `internal/cli/next.go` (new), `internal/cli/next_test.go` (new), `internal/cli/root.go`,
`internal/cli/doc.go`, `internal/cli/show.go`, `internal/cli/output/errors.go`,
`internal/cli/output/errors_test.go`, `internal/cli/output/output.go`,
`docs/site/reference/json-output.md`, `docs/site/reference/cli/`
**Read first:** `internal/cli/show.go` — newShowCommand, showResult; `internal/cli/ready.go` — newReadyCommand,
readyOptions.listOptions; `internal/cli/root.go` — emit, Run; `internal/cli/output/errors.go` — Error.ExitCode,
Rendered, Exit; `internal/cli/output/output.go` — Envelope; `internal/cli/check.go` — newCheckCommand (Rendered-failure pattern)
**Tests:** CLI tests: one Ready Issue; several with a priority tie (the one unblocking more wins,
matching `ready`'s first line); none Ready (text, exit 4; `--json` kind `next`, raw `"data":null`
present, exit 4; `--kv` exit 4 if offered); a `--label` filter selecting a lower-priority Issue;
`--json` equal to `show --json <id>`'s data. Output test: the exit-4 row of `ExitCode`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Next|TestDocFileMap|TestHelpText'`
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/output/`
- `GOMEMLIMIT=2GiB go test ./internal/clidoc/`
- `go vet ./internal/cli/...`
**Commit:** `feat(cli): add capcom next, the top Ready Issue in full`

## 4. The Managed block names capcom next — ✅ DONE (2026-10-19)

NOTES (2026-10-19): internal/cli/agents_test.go needed no change: its stale-block tests build the block from render rather than holding a golden copy, so it is left out of FILES.

Depends on item 3.

**What:**
**Goal:** the Managed block `capcom agents` writes lists
`` - `capcom next`: the most urgent ready issue, in full `` as its first command line, before
`capcom ready`; this repository's `AGENTS.md` carries the refreshed block; the
`issue-workflow` Note tells a session to start with `capcom next`.
**Approach (assumed at the header base):**
- Edit `loopOpening` in `internal/agents/block.go`; update the expected text in
  `internal/agents/block_test.go` and any golden block in `internal/cli/agents_test.go`.
- Refresh this repo's block with the built binary (`go run ./cmd/capcom agents`), update
  `docs/site/agents/managed-block.md`'s copy of the list, and edit the Note with
  `capcom know update issue-workflow`.

**Regression guard.** Every existing repo's block reads stale after upgrading, so `capcom check` and
`capcom agents --check` exit 1 until `capcom agents` runs. The item yields to the house upgrade-note
pattern (CHANGELOG.md:27, CHANGELOG.md:103): the item's CHANGELOG sidecar entry reads "An existing block reads as out of
date after upgrading: run `capcom agents` once to refresh it (`capcom check` and `capcom agents --check`
report it until you do)." Add a full single-package run of `./internal/cli/` to Acceptance.

**Files:** `internal/agents/block.go`, `internal/agents/block_test.go`,
`internal/cli/agents_test.go`, `AGENTS.md`, `docs/site/agents/managed-block.md`,
`.capcom/knowledge/issue-workflow.md`
**Read first:** `internal/agents/block.go` — loopOpening, render; `internal/agents/block_test.go` — goldenBlock
(leave preInstallLineBlock and preStoreMapBlock: historical fixtures); `internal/check/agents.go` — agentsBlock;
`docs/site/agents/managed-block.md` — command list sample, opening work-loop prose; `internal/cli/agents_test.go` — check-on-stale-block tests
**Tests:** the existing block tests with the new line.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/agents/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Agents'`
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `go run ./cmd/capcom check` reports no problems
**Commit:** `docs(agents): name capcom next in the Managed block`
**Closes:** next-command
