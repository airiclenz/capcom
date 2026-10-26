# Plan: the ready P1/P2 ergonomics batch

**Goal:** Land the eleven ready P1/P2 issues: examples-first help, error hints, per-id show
suggestions, inline body flags, diacritic-folding excerpts, unshared refs in prime, `capcom context`,
`capcom recap`, id completion and tab counts in `capcom ui`.
**Date:** 2026-10-26
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 98339f8

**Sources:**
- `.capcom/issues/agent-ergonomics/{context-command,error-next-command,excerpt-diacritics,help-example-first,inline-body-flags,search-excerpt,show-multi-suggestions}.md`
- `.capcom/issues/human-ergonomics/{ui-tab-counts,id-completion,recap-command}.md`, `.capcom/issues/prime-unshared-refs.md`
- Notes `cli-test-harness`, `coding-conventions`, `error-messages`, `store-layout`, `docs-layout`, `adr-0004`, `adr-0007`
- `docs/site/reference/json-output.md` (envelope, error body, kinds)

**Ratified design calls (user, 2026-10-26):**
- **Error hints:** one table gives every `output.Code` a default hint; a site with a specific fix sets `Error.Hint`, which wins. Inline "use reopen…"/"run capcom …" text moves from messages into hints. Known values (the id) are filled in, unknowns stay `<placeholders>`. Text: last stderr line `hint: <command>`; JSON error body `hint`; `--kv` `HINT=`.
- **show with several unknown ids:** one message, each id followed by its own suggestions: `id "a" not found, did you mean x?; id "b" not found, did you mean y?`; top-level `suggestions` empty when more than one id fails; `--kv` unchanged; one unknown id unchanged.
- **recap counting:** snapshot diff of `.capcom/` between the boundary commit and HEAD, keyed by id (a move is no add/remove), committed history only; an issue filed and closed in the window is listed under both.
- **context Notes:** a Note applies when a ref's path part (before `#`) appears in the issue body as a whole path, or as a directory followed by `/`; then Note-search hits on the title (every-word, then any-word), retired excluded, de-duplicated; ≤ 8 Notes listed, top 3 bodies in full; caps: Note body 60 lines, deps and dependents 15 each.
- **Diacritics:** add `golang.org/x/text`; excerpt tokens are NFD-decomposed with `unicode.Mn` dropped.
- **prime:** its own `unshared Note refs: N` list after the broken-refs list; JSON `unshared_refs` (omitzero); `broken_refs` unchanged.
- **Help order:** `--help` only; `docs/site/reference/cli/` keeps cobra/doc's layout and is regenerated.

**Writer calls (2026-10-26):** listed in the items that own them.

**Standing requirements:**
- skills: coding-standards
- Tests per `cli-test-harness`: `GOMEMLIMIT=2GiB`, one package at a time, never `./...` with `-race`.
- After changing a command's Short, Long, Example or flags: `go run ./cmd/gendocs`, commit `docs/site/reference/cli/`.
- A new file in `internal/cli` gets a line in the file map in `internal/cli/doc.go`.
- Errors follow the `error-messages` Note; capcom runs no writing git command (ADR 0004).
- A new command's `Long` already obeys item 4's rules (examples first, paragraphs ≤ 4 lines).

**Regression check (2026-10-26, 98339f8):**
- 1: guard folded (writer decision: regenerate the help goldens, golden test in Acceptance; plus the paragraph-length test landed here). Golden wording fixed (writer decision): goldens rewritten from `capcom <verb> --help` output, no update flag.
- 2: guard folded (writer decision: help goldens; plus paragraph-length test widened). Golden wording fixed (writer decision): goldens rewritten from `capcom <verb> --help` output, no update flag.
- 3: guard folded (writer decision: help goldens; plus mv.go/rm.go `long:` fields and widened grep). Golden wording fixed (writer decision): goldens rewritten from `capcom <verb> --help` output, no update flag.
- 4: guard folded (writer decision: every help golden; plus usage template replaced, `lock --help` order, `Usage:` kept). Golden wording fixed (writer decision): goldens rewritten from `capcom <verb> --help` output, no update flag.
- 5: guard folded (writer decision: pinned error tests updated, whole-package Acceptance; plus `<command>` filled in emit, show's per-id HINT=, docs/site error examples).
- 6: guard folded (writer decision: whole-package Acceptance; plus close's joined Hint, finish lock notes kept, joined two-command hints, file-format.md standing sentence).
- 7: guard folded (nothing-ready override dropped, StoreFailure not ErrNotInitialized, agents.go clauses, `capcom check` only for read/parse FileErrors, three bound hints).
- 8: guard folded (writer decision: depends on item 5; plus know show clause dropped, joinAlternatives exported).
- 9: guard folded (writer decision: create golden; plus createLong's body rule and issues.md reworded). Golden wording fixed (writer decision): goldens rewritten from `capcom <verb> --help` output, no update flag.
- 10: guard folded; yields to README.md and docs/site/getting-started/install.md (Go 1.25 minimum).
- 12: guard folded (refs of excluded duplicate-id Notes left out).
- 14: guard folded (SKILL.md fallback for older binaries, Acceptance grep).
- 15: guard folded (unborn HEAD returns empty).
- 16: guard folded (renames paired by alias, Notes changed excludes added/retired, bounded per-file reads as elsewhere does).
- 18: recast (writer decision: `context` completes Issue ids; plus Files, mv/rm argument 1 only, fixture-backed `__complete show stand`). Guard folded: knowledge tab reads Index.Notes(true); Tests cover ui, mv argument 2, context.
- 19: guard folded (StringSlice-only comma rule, --parent branch on idKind, completion funcs registered only under `__complete`).
- 20: guard folded (n counts closed issues under isClosedShown, path label yields at 80 columns).

**Out of scope:**
- `--why`/`--acceptance` on `update`; hints for warnings; examples-first in the site reference.
- `batch-apply`, `tree-status-filter`, `html-export`, `shared-ref-repository` (P3).

## 1. Long help split into short rules: root to init

**What:**
**Goal:** In `root.go`, `agents.go`, `check.go`, `close.go`, `create.go`, `dep.go`, `dep_list.go`,
`dep_graph.go`, `finish.go`, `fmt.go`, `init.go` (all `internal/cli`), every command's `Long` is split
into paragraphs (blank-line separated) or `- ` bullets, one rule each, none longer than four lines when
word-wrapped at 100 columns; the wording of each rule is unchanged.
**Approach (assumed at the header base):** Edit the `…Long` constants beside each command. Bullets
for lists of rules, paragraphs otherwise. Sentences are moved, never reworded, except joining words a
split leaves dangling. Regenerate the CLI reference.
**Regression guard.** the byte-pinned help goldens internal/cli/testdata/help/*.txt (TestHelp_NodeVerbs_MatchGolden) change with any Long this item rewrites — that test has no update flag: rewrite each affected internal/cli/testdata/help/*.txt from the new `capcom <verb> --help` output (e.g. `go run ./cmd/capcom <verb> --help > internal/cli/testdata/help/<file>.txt`, matching how the test names its files), then run the golden test; list internal/cli/testdata/help/ on Files, and add the golden test to the Acceptance -run filter.
The Goal's four-line rule is checked in this item: it lands the paragraph-length half of item 4's test in internal/cli/help_text_test.go, limited to this item's commands; items 2–3 widen its list and item 4 extends it to every available command.
**Files:** internal/cli/root.go; internal/cli/agents.go; internal/cli/check.go; internal/cli/close.go; internal/cli/create.go; internal/cli/dep.go; internal/cli/dep_list.go; internal/cli/dep_graph.go; internal/cli/finish.go; internal/cli/fmt.go; internal/cli/init.go; internal/cli/help_text_test.go; internal/cli/testdata/help/; docs/site/reference/cli/
**Read first:** internal/cli/create.go — createLong; internal/cli/root.go — rootLong, versionLong; internal/cli/dep.go — depLong, depAddLong, depRmLong;
internal/cli/node_verbs_test.go — TestHelp_NodeVerbs_MatchGolden; internal/cli/testdata/help/create.txt; internal/cli/help_text_test.go — TestHelpText_EveryCommand_SetsLongAndExample, availableCommands;
internal/clidoc/reference_test.go — TestReferenceUpToDate
**Tests:** existing help-text tests; the paragraph-length test over this item's commands; the regenerated goldens.
**Acceptance:** `go run ./cmd/gendocs`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`;
`GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'HelpText|DocFileMap|TestHelp_NodeVerbs_MatchGolden'`; `go vet ./internal/cli/`
**Commit:** `docs(cli): split the Long help of root to init into short rules`

## 2. Long help split into short rules: know, list, lock, log, merge-driver

**What:**
**Goal:** Same rule as item 1 for `know.go`, `know_add.go`, `know_update.go`, `know_verify.go`,
`know_tree.go`, `know_show.go`, `know_search.go`, `lock.go`, `list.go`, `log.go`, `merge_driver.go`.
**Approach (assumed at the header base):** As item 1.
**Regression guard.** same golden rule as item 1.
The paragraph-length test item 1 lands widens its list to this item's commands.
**Files:** internal/cli/know.go; internal/cli/know_add.go; internal/cli/know_update.go; internal/cli/know_verify.go; internal/cli/know_tree.go; internal/cli/know_show.go; internal/cli/know_search.go; internal/cli/lock.go; internal/cli/list.go; internal/cli/log.go; internal/cli/merge_driver.go; internal/cli/help_text_test.go; internal/cli/testdata/help/; docs/site/reference/cli/
**Read first:** internal/cli/lock.go — lockLong, lockAcquireLong, lockReleaseLong; internal/cli/lock_test.go — TestLockReleaseHelp_DoesNotSayChangesNothing; internal/cli/know_add.go — knowAddLong;
internal/cli/testdata/help/know-add.txt; internal/cli/merge_driver.go — newMergeDriverCommand; internal/cli/node_verbs_test.go — TestHelp_NodeVerbs_MatchGolden;
internal/clidoc/reference_test.go — TestReferenceUpToDate
**Tests:** existing help-text tests; the paragraph-length test widened to this item's commands; the regenerated goldens.
**Acceptance:** as item 1.
**Commit:** `docs(cli): split the Long help of know to merge-driver into short rules`

## 3. Long help split into short rules: mv to ui

**What:**
**Goal:** Same rule as item 1 for `node_verbs.go`, `next.go`, `reopen.go`, `search.go`, `reindex.go`,
`update.go`, `show.go`, `ready.go`, `related.go`, `prime.go`, `start.go`, `ui.go`, and every other
`Long` in `internal/cli` not covered by items 1–2 (`grep -ln 'Long:' internal/cli/*.go`).
**Approach (assumed at the header base):** As item 1.
**Regression guard.** same golden rule as item 1.
The Goal also covers `mv.go` and `rm.go`: the mv/rm/know mv/know rm Long text sits in the `long:` fields of issueMvHelp/noteMvHelp and issueRmHelp/noteRmHelp, so the rule's grep is `grep -lnE 'Long:|long:' internal/cli/*.go`. The paragraph-length test item 1 lands widens its list to this item's commands.
**Files:** internal/cli/node_verbs.go; internal/cli/next.go; internal/cli/reopen.go; internal/cli/search.go; internal/cli/reindex.go; internal/cli/update.go; internal/cli/show.go; internal/cli/ready.go; internal/cli/related.go; internal/cli/prime.go; internal/cli/start.go; internal/cli/ui.go; internal/cli/mv.go; internal/cli/rm.go; internal/cli/help_text_test.go; internal/cli/testdata/help/; docs/site/reference/cli/
**Read first:** internal/cli/mv.go — issueMvHelp, noteMvHelp; internal/cli/rm.go — issueRmHelp, noteRmHelp; internal/cli/node_verbs.go — commandHelp, newMvCommand, newRmCommand;
internal/cli/node_verbs_test.go — TestHelp_NodeVerbs_MatchGolden; internal/cli/related.go — relatedLong, relatedAddLong, relatedRmLong; internal/cli/help_text_test.go — availableCommands
**Tests:** existing help-text tests; the paragraph-length test widened to this item's commands; the regenerated goldens.
**Acceptance:** as item 1.
**Commit:** `docs(cli): split the Long help of mv to ui into short rules`

## 4. --help prints the examples before the rules

Depends on items 1–3.

**What:**
**Goal:** Every command's `--help` prints, in order: `Short`, Usage, Available Commands (when it has
subcommands), Examples, Flags, Global Flags, then `Long`. A test walks every available command and
fails when an example line comes after the first line of `Long`, or a `Long` paragraph or bullet wraps
to more than four lines at 100 columns.
**Approach (assumed at the header base):** A custom help template (or help func) set once on the root
in `newRootCommand`; children inherit it. The new test sits in `internal/cli/help_text_test.go` beside
`TestHelpText_EveryCommand_SetsLongAndExample`, reusing `availableCommands()`; it renders help through
`runCLI(... "--help")`. Wrapping is greedy word wrap of each blank-line-separated block (a `- ` bullet
is its own block). cobra/doc output is unaffected; regenerate to confirm.
**Regression guard.** same golden rule as item 1 (the reordered help changes every golden).
The new test is named `TestHelpText_…` so `-run 'HelpText'` selects it; it extends the paragraph-length test items 1–3 landed to every available command.
cobra's default usage template prints Examples before Available Commands, so the usage template is replaced too, not only the help template; assert the order once on a group (`capcom lock --help`). Keep the `Usage:` heading, which TestRun_GroupWithoutSubcommand_PrintsHelp and TestRun_NoArgs_PrintsHelp check.
**Files:** internal/cli/root.go; internal/cli/help_text_test.go; internal/cli/testdata/help/; docs/site/reference/cli/
**Read first:** internal/cli/root.go — newRootCommand, withSubcommandDispatch; github.com/spf13/cobra@v1.10.2/command.go — defaultUsageTemplate, defaultHelpFunc, HelpTemplate;
internal/cli/help_text_test.go — availableCommands, TestHelpText_EveryCommand_SetsLongAndExample; internal/cli/testharness_test.go — runCLI; internal/cli/node_verbs_test.go — TestHelp_NodeVerbs_MatchGolden;
internal/cli/cli_test.go — TestRun_GroupWithoutSubcommand_PrintsHelp, TestRun_NoArgs_PrintsHelp; internal/clidoc/reference.go — WriteReference
**Closes:** help-example-first
**Tests:** the new ordering and paragraph-length test; `capcom create --help` output order asserted once; `capcom lock --help` order (Available Commands before Examples) asserted once; the regenerated goldens.
**Acceptance:** `go run ./cmd/gendocs`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`;
`GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'HelpText|TestHelp_NodeVerbs_MatchGolden|PrintsHelp'`; `go vet ./internal/cli/`
**Commit:** `feat(cli): print examples before the rules in command help`

## 5. Errors carry a hint naming the next command

**What:**
**Goal:** Every `output.Code` has a default hint in one table: not_found `capcom search <text>`,
exists `capcom show <id>`, invalid `capcom <command> --help`, conflict `capcom show <id>`, locked
`capcom lock status`, not_initialized `capcom init`. `output.Error` gains `Hint`; a set `Hint` wins over
the default. Text output ends with the line `hint: <command>` after the error line; the JSON error body
carries `hint`; `--kv` prints `HINT=` after `VALID=`. A test fails when a `Code` constant has no table
entry. `<command>` is filled with the failing command's path wherever the command is known.
**Approach (assumed at the header base):** In `internal/cli/output`: `Hint` field on `Error`, a
`hints` table keyed by `Code`, rendering in `textRenderer.RenderError`, `errorBody()`, `ErrorHintPairs`.
The completeness test parses the package with `go/parser` and collects every const of type `Code`, so
a new code without a hint fails. `runE` and the flag-error func in `root.go` fill `<command>`; errors
from `Exit` with no command keep `capcom --help`. Document `hint` in the error table of
`docs/site/reference/json-output.md`.
**Regression guard.** item 5 updates every existing test whose pinned error output gains the hint line (text, JSON, kv) and its Acceptance runs the whole package `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/` so the commit lands green.
`<command>` is also filled where a failure renders before runE sees it: in `emit` before Render (a partial result's Failure, e.g. `capcom close <standing-id>`), or keep cmd.CommandPath() from prepare and fill at render; a test covers a partial-result failure.
show's per-id `--kv` records carry `HINT=` (show.go appends output.ErrorHintPairs(failure)); update the read_test.go records that pin them.
Every `^capcom: ` error example under docs/site (`grep -rn '^capcom: ' docs/site`) gains its `hint:` line.
**Files:** internal/cli/output/errors.go; internal/cli/output/output.go; internal/cli/output/errors_test.go; internal/cli/output/output_test.go; internal/cli/root.go; internal/cli/*_test.go (`grep -rnE '"capcom: .*\\n"|HasSuffix\(stderr|MESSAGE=' internal/cli/*_test.go`); docs/site/
**Read first:** internal/cli/output/output.go — textRenderer.RenderError, errorBody, ErrorHintPairs, kvRenderer.RenderError; internal/cli/output/errors.go — Error, Exit, AsError, StoreFailure;
internal/cli/root.go — runE, emit, Run, unknownCommandOnFlagError, prepare; internal/cli/show.go — showResult; internal/cli/output/errors_test.go — TestExit_MapsEachOutcome;
internal/cli/output/output_test.go — TestRenderError_SuggestionsAndValid_ReportedInEveryFormat; internal/cli/testharness_test.go — runCLI; internal/cli/cli_test.go — decodeEnvelope, lastLine
**Tests:** each format renders a default and an override hint; the completeness test; a CLI test that
`capcom update nosuch --title x` ends in `hint: capcom search <text>` and JSON `hint`; a partial-result failure whose hint names its command; every pinned error output updated.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/output/`;
`GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/`; `go vet ./internal/cli/...`
**Commit:** `feat(cli): end every error with a hint naming the next command`

## 6. Issue write commands set specific hints

Depends on item 5.

**What:**
**Goal:** In `update.go`, `create.go`, `close.go`, `reopen.go`, `start.go`, `finish.go` no error
message names a next command; each such command is that error's `Hint`, with the known id filled in,
e.g. updating to a closed status hints `capcom close <id> --reason <text>`, and out of a closed status
`capcom reopen <id> --reason <text>`; a refusal while another session holds the lock hints
`capcom lock status`. `grep -nE 'use (re)?open|use close|run capcom|then close' ` over these files finds
no error message.
**Approach (assumed at the header base):** At each `output.Errorf`/`UsageErrorf` site whose message
names a command (e.g. the closed-status checks in `update.go`, the `run capcom update %s --%s first`
message, the "create the issue open, then close it" message in `create.go`), drop that clause from
the message and set `Hint`. Update the tests and doc lines quoting the old wording
(`grep -rn` the old clause in `internal/cli/*_test.go docs/site`).
**Regression guard.** Acceptance runs the whole package `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/`.
closeResult's joined failure carries the first failure's Hint, as it carries Suggestions, so `capcom close <standing-id>` keeps its `capcom update <id> --no-standing` hint.
finish's lock-line `note:` text ("left alone: …; run capcom lock status") is result output and stays; the Goal grep excludes it. Where a message names two commands (start's not-Ready resolvers joined ", then ", the doing-and-free "…to hold it, or capcom update…"), the Hint carries the same joined command text as one string.
docs/site/reference/file-format.md's standing sentence ("`capcom ui` shows the same refusal") is among the docs updated, since the UI keeps its full text.
**Files:** internal/cli/update.go; internal/cli/create.go; internal/cli/close.go; internal/cli/reopen.go; internal/cli/start.go; internal/cli/finish.go; internal/cli/write_test.go; internal/cli/start_test.go; docs/site/
**Read first:** internal/cli/update.go — statusChangeFailure; internal/cli/node_edit.go — writeFailure; internal/cli/close.go — closeResult, closeOne; internal/cli/start.go — refuseUnstartable, readinessResolvers, refuseLock, startLockFailure;
internal/cli/create.go — newIssue; internal/cli/finish.go — notHoldingReason; internal/cli/start_test.go — TestStart_RefusedIssue_WritesNothingAndNamesTheResolver; internal/cli/write_test.go — runCLIWith
**Tests:** one test per moved hint: text last line and JSON `hint`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/`; `go vet ./internal/cli/`
**Commit:** `feat(cli): name the fixing command as a hint on issue write errors`

## 7. Store, Note and lock errors set specific hints

Depends on item 6.

**What:**
**Goal:** Across the rest of `internal/cli`, no error message names a next command; it is the `Hint`.
Bound overrides: a malformed file (`nodestore` file error) hints `capcom check`; an unknown Note id hints
`capcom know search <text>`; nothing ready hints `capcom list`; a missing store hints `capcom init`; a
lock held by another session hints `capcom lock status`.
**Approach (assumed at the header base):** `(*app).kindFailure` and `refusalFailure` in
`internal/cli/errors.go`, `notFoundFailure` in `suggest.go` (Note branch), `NothingReadyErrorf`
callers, `store.go`, `lock.go`, `node_edit.go`, `merge_driver.go`, `ui.go`. Find the rest with
`grep -nE 'use [a-z]|run capcom|; capcom' internal/cli/*.go`. Update pinned test and doc wording.
**Regression guard.** Drop the nothing-ready override: next renders nothing-ready as `data: null` with no error (json-output.md, next_test.go), so no hint is observable.
internal/cli/store.go does not exist: leave store.ErrNotInitialized (prime shows it as a warning) alone and strip "; run capcom init" only inside output.StoreFailure (output/errors.go); cli_test.go pins the text.
agents.go's stale-block and guide-unreached errors drop their "run capcom agents" clause where agents.go builds them; Guide.Problem() stays unchanged for capcom check (agents_test.go pins it).
The `capcom check` hint applies only to FileErrors from reading or parsing an existing file, not to write validation (nodestore/write.go validationError, e.g. `know update x --title ""`): mark them in internal/nodestore or exclude validation errors.
Bind `capcom lock status` for lock touch/release on a free lock, `capcom dep list <id>` for a refusal that drops no entry, and `capcom know show <id>` for a Note that exists.
**Files:** internal/cli/errors.go; internal/cli/suggest.go; internal/cli/output/errors.go; internal/cli/agents.go; internal/cli/lock.go; internal/cli/node_edit.go; internal/cli/merge_driver.go; internal/cli/ui.go; internal/nodestore/errors.go; internal/nodestore/write.go; internal/cli/*_test.go; docs/site/
**Read first:** internal/cli/errors.go — kindFailure, refusalFailure; internal/cli/suggest.go — notFoundFailure, closestIDs; internal/cli/show.go — resolveFailure; internal/cli/lock.go — emitLock, failedLockResult;
internal/cli/next.go — emitNothingReady; internal/cli/agents.go — checkAgentsBlock; internal/cli/output/errors.go — StoreFailure; internal/nodestore/errors.go — FileError, fileError
**Closes:** error-next-command
**Tests:** a hint test for each bound override, text and JSON; a write validation error does not hint `capcom check`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/`; `go vet ./internal/cli/`
**Commit:** `feat(cli): name the fixing command as a hint on store, Note and lock errors`

## 8. show names suggestions per unknown id

Depends on item 5.

**What:**
**Goal:** `capcom show a b` with both unknown prints one error line
`id "a" not found, did you mean x?; id "b" not found, did you mean y?` (an id without close matches
keeps just `id "a" not found`); the JSON error carries that message and no top-level `suggestions`;
`--kv` records are unchanged; a single unknown id prints exactly as at the base. `know show` with
several unknown ids behaves the same.
**Approach (assumed at the header base):** In `showResult` (`internal/cli/show.go`), when more than one
id fails, build each message with its own suggestion suffix (the `joinAlternatives` wording) before the
`failureSeparator` join and clear `result.Failure.Suggestions`. Same in `noteShowResult` if it joins.
**Regression guard.** Depends on item 5 — "--kv records unchanged" and "a single unknown id prints exactly as at the base" mean as after item 5 (the HINT= pair and hint line item 5 adds stay).
Drop the `know show` clause: `know show` takes exactly one id (cobra.ExactArgs(1)), so several unknown ids are a usage error.
`joinAlternatives` is unexported in package output: export it (e.g. output.JoinAlternatives) for show.go to reuse.
**Files:** internal/cli/show.go; internal/cli/output/output.go; internal/cli/read_test.go
**Read first:** internal/cli/show.go — showResult, failureSeparator, resolveFailure; internal/cli/output/output.go — joinAlternatives, textRenderer.RenderError, ErrorHintPairs;
internal/cli/read_test.go — TestShow_UnknownID_SuggestsTheClosestIDs, TestShow_KV_ClosesEachIDWithItsStatus, TestShow_OneOfSeveralMissingJSON_CarriesDataAndError; internal/cli/know_show.go — newKnowShowCommand
**Closes:** show-multi-suggestions
**Tests:** two unknown ids each with a close match (text, JSON, kv); one unknown id unchanged.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Show'`; `go vet ./internal/cli/`
**Commit:** `fix(cli): name suggestions per id when show gets several unknown ids`

## 9. create takes the body inline

**What:**
**Goal:** `capcom create` accepts `--body <text>` (literal), `--why <text>` and repeatable
`--acceptance <text>`. The body is: the `--body` or `--body-file` text, then `## Why` with the why
text, then `## Acceptance` with one `- <text>` bullet per `--acceptance`, sections blank-line
separated. `--body` with `--body-file` is refused before anything is written; `--why`/`--acceptance`
combine with either. The no-body warning reads `%s has no body: add what, why and acceptance with capcom update %s --body-append <text>; create takes --body, --why and --acceptance`.
The `create` examples show the inline form.
**Approach (assumed at the header base):** Flag consts in `flags.go`; `createOptions` fields;
`--acceptance` as `StringArrayVar`; `MarkFlagsMutuallyExclusive(flagBody, flagBodyFile)`; compose in
`newIssue` through `edit.AppendBody`. Update `emptyBodyWarning` and every doc quoting it
(`grep -rn 'has no body' docs/site README.md`).
**Regression guard.** same golden rule as item 1 (create's help changes).
createLong's body rule ("The body comes from --body-file (- reads stdin)") is reworded to name --body, --why and --acceptance, one rule within item 4's four-line limit; docs/site/concepts/issues.md's "The body" section says the same.
**Files:** internal/cli/create.go; internal/cli/flags.go; internal/cli/write_test.go; internal/cli/testdata/help/; docs/site/; docs/site/reference/cli/
**Read first:** internal/cli/create.go — newCreateCommand, createOptions, newIssue, create, emptyBodyWarning, createLong; internal/edit/edit.go — AppendBody, AppendBlock; internal/cli/flags.go — flagBodyFile;
internal/cli/write_test.go — TestCreate_EmptyOrWhitespaceBody_WritesAndWarnsInTextAndJSON, TestCreate_WithBody_DoesNotWarn, TestCreate_MissingDep_FailsLikeDepAddAndWritesNothing; internal/cli/flag_once.go — guardSingleValueFlags, isAccumulating;
internal/cli/node_verbs_test.go — TestHelp_NodeVerbs_MatchGolden; docs/site/reference/json-output.md; docs/site/concepts/issues.md
**Closes:** inline-body-flags
**Tests:** the issue's acceptance command writes the exact body; `--body`+`--body-file` refused and no
file written; `--acceptance` alone gives a body and no warning; warning text pinned.
**Acceptance:** `go run ./cmd/gendocs`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Create|TestHelp_NodeVerbs_MatchGolden'`;
`GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`; `go vet ./internal/cli/`
**Commit:** `feat(create): take the body, why and acceptance as flags`

## 10. Excerpts fold diacritics like the search index

**What:**
**Goal:** `index.Excerpt` matches a query word and a body word that differ only by diacritics, in
either direction and in composed or decomposed form, so `capcom search cafe --excerpt` and
`capcom know search cafe --excerpt` print the body line holding `café`. Printed lines keep their
original text. Regression from the `--excerpt` work (field-eval-ergonomics item 4).
**Approach (assumed at the header base):** `textTokens` in `internal/index/excerpt.go` folds each
token: `norm.NFD`, drop `unicode.Mn`, lower-case. Add `golang.org/x/text` to `go.mod`/`go.sum`
(`go get golang.org/x/text`, `go mod tidy`).
**Regression guard.** Pin `go get golang.org/x/text@v0.41.0`, the last release declaring `go 1.25.0` (it pulls golang.org/x/sync v0.22.0, also go 1.25.0); go.mod keeps `go 1.25.0`. The item yields to the documented Go 1.25 minimum (README.md install section, docs/site/getting-started/install.md).
**Files:** internal/index/excerpt.go; internal/index/excerpt_test.go; internal/cli/read_test.go; internal/cli/know_test.go; go.mod; go.sum
**Read first:** internal/index/excerpt.go — textTokens, queryTokens, Excerpt, holdsAnyWord; internal/index/excerpt_test.go — TestExcerpt_BodyAndQuery_ReturnsTheExcerptShape;
internal/cli/read_test.go — TestSearch_Excerpt_PrintsTheMatchingBodyLinesUnderEachHit, hitExcerpts; internal/cli/know_test.go — TestKnowSearch_Excerpt_PrintsTheMatchingBodyLinesUnderEachHit;
internal/cli/search.go — searchExcerpts, withExcerpts; go.mod
**Closes:** excerpt-diacritics; search-excerpt
**Tests:** table rows `cafe`/`café`, `café`/`cafe`, decomposed `café`; CLI `--excerpt` for an
Issue and a Note.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/ -run Excerpt`;
`GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Excerpt'`; `go vet ./internal/index/`; `grep -qx 'go 1.25.0' go.mod`
**Commit:** `fix(index): fold diacritics in search excerpts`

## 11. prime names unshared Note refs

**What:**
**Goal:** `capcom prime` prints `unshared Note refs: N` with one indented `path:line: message` per
`unshared-ref` problem, after the broken-refs list and only when N > 0; JSON carries `unshared_refs`
(omitzero) in the shape of `broken_refs`. `broken Note refs:` and `broken_refs` are unchanged.
**Approach (assumed at the header base):** `briefing.Health` gains `UnsharedRefs`; `readHealth` keeps
`diagnose.UnsharedRef` problems there. `addHealth` and `primeData` in `internal/cli/prime.go`.
Update `primeLong` and `docs/site/agents/briefing.md` (text and JSON table).
**Files:** internal/briefing/briefing.go; internal/briefing/briefing_test.go; internal/cli/prime.go; internal/cli/prime_test.go; docs/site/agents/briefing.md; docs/site/reference/cli/
**Read first:** internal/briefing/briefing.go — Health, readHealth; internal/cli/prime.go — primeData, addHealth, primeLong; internal/check/notes.go — checkRefs;
internal/knowledge/verify.go — finishCheck, Verify; internal/briefing/briefing_test.go — newBriefingRepo, briefingStore, TestCompose_FixtureStore_ReturnsEverySection;
internal/cli/prime_test.go — newPrimeStore, TestPrime_FixtureStore_PrintsTheBriefing, TestPrime_JSONBrokenRef_IsACheckProblemAtItsLine; docs/site/agents/briefing.md
**Closes:** prime-unshared-refs
**Tests:** briefing test with a Note ref to an ignored file; prime text and JSON.
**Acceptance:** `go run ./cmd/gendocs`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/briefing/`;
`GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run Prime`; `go vet ./internal/briefing/`
**Commit:** `fix(prime): name unshared Note refs`

## 12. Resolve a plan's path and list every Note ref

**What:**
**Goal:** `config` offers a resolver returning a plan basename's file path (`<dir>/<plan>.md` or
`<dir>/<plan>` in each configured plans directory, regular files only, non-basenames refused) and
whether it was found; `check` uses it, its behaviour unchanged. `index` returns every `(Note id, ref)`
pair of the Notes not retired (own flag or retired ancestor).
**Approach (assumed at the header base):** Extract `(*checker).isPlanFound` (`internal/check/issues.go`)
into a `config` method taking the root; no copy left behind. New `Index` query over the `refs` table
joined to notes, filtered by `retiredSubtrees`.
**Regression guard.** The refs query keeps only usable Notes (`notes.excluded = 0`, as Notes/searchNotes in internal/index/notes.go do) besides retiredSubtrees: a duplicate-id Note keeps its refs rows but no command can resolve it.
**Files:** internal/config/config.go; internal/config/config_test.go; internal/check/issues.go; internal/index/notes.go; internal/index/notes_test.go
**Read first:** internal/check/issues.go — (*checker).isPlanFound, checkIssue, planExtension; internal/config/config.go — Config.Plans, Default; internal/index/notes.go — retiredSubtrees, retiredFilter, Notes, insertNote;
internal/index/schema.go — refs table; internal/index/duplicates.go — persistMembershipOf; internal/check/check_test.go — missing-plan cases; internal/index/notes_test.go — TestNotes_RetiredSubtree_LeftOutUnlessIncluded
**Tests:** resolver: `.md` and bare file, archived dir, directory refused, path refused; refs query
skips retired subtrees; a duplicate-id Note's refs are left out.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/config/ ./internal/check/`;
`GOMEMLIMIT=2GiB go test -count=1 ./internal/index/ -run Note`; `go vet ./internal/config/ ./internal/check/ ./internal/index/`
**Commit:** `refactor(config): share plan path resolution and list Note refs`

## 13. capcom context prints an issue with what it needs

Depends on items 4, 5, 12.

**What:**
**Goal:** `capcom context <id>` prints, in order: the issue as `show` prints it; `deps:` and
`dependents:` one level deep with status and priority (`dep list` lines); `plan: <path>`, or
`plan: <name> (not found)`; `notes:` with up to 8 applying Notes (ratified rule) as
`  <id>  <title>  (ref <ref>)` or `(search)`, then the bodies of the top three, each under
`--- <id>: <title>`. An empty section prints `deps: none`, `dependents: none`, `plan: none`,
`notes: none`. Caps: 15 entries per dep list, 60 lines per Note body; a cut ends with
`... N more: capcom dep list <id>` or `... cut: capcom know show <id>`; `--help` states the caps.
`--brief` prints ids and titles only (no bodies). `--json`: kind `context`, data
`{issue, deps, dependents, plan, notes}`.
**Approach (assumed at the header base):** New `internal/cli/context.go`; reuse `showResult`,
`depGraph.listResult`/`issueLines`, `searchEveryWordFirst`, `noteShowResult`; no copied rendering.
Register in `newRootCommand`; doc.go file map; `kind` row in `docs/site/reference/json-output.md`.
**Files:** internal/cli/context.go; internal/cli/context_test.go; internal/cli/root.go; internal/cli/flags.go; internal/cli/doc.go; docs/site/reference/json-output.md; docs/site/reference/cli/
**Read first:** internal/cli/show.go — showResult, emitShown; internal/cli/dep_list.go — depGraph.listResult, depListData, depSection.add; internal/cli/search.go — searchEveryWordFirst;
internal/cli/know_show.go — noteShowResult, noteFromFile; internal/cli/root.go — newRootCommand; internal/cli/help_text_test.go — TestHelpText_EveryCommand_SetsLongAndExample, TestHelpText_CommandsWithoutKV_OmitKVFromLongAndExample;
internal/cli/doc_test.go — TestDocFileMap_MatchesDirectory; internal/clidoc/reference_test.go — TestReferenceUpToDate
**Closes:** context-command
**Tests:** full case (deps, dependents, plan, ref Note and search Note); all-empty case; caps cut with
the named command; `--brief`; `--json`.
**Acceptance:** `go run ./cmd/gendocs`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Context|HelpText|DocFileMap'`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`; `go vet ./internal/cli/`
**Commit:** `feat(context): print an issue with its deps, plan and applying Notes`

## 14. The Managed block names capcom context

Depends on item 13.

**What:**
**Goal:** The Managed block tells agents to run `capcom context <id>` before starting an issue; a
repo whose block is the base render is reported stale and refreshed by `capcom agents`. This repo's
`AGENTS.md`, `docs/site/agents/managed-block.md` and `plugins/capcom/skills/capcom/SKILL.md` show the
new block.
**Approach (assumed at the header base):** Add the line in `loopOpening` (`internal/agents/block.go`);
update `goldenBlock`; keep the base render as a historical constant for stale detection, as
`keepTheTokenBlock` does. Refresh with `go run ./cmd/capcom agents`.
**Regression guard.** plugins/capcom/skills/capcom/SKILL.md ships from main to users on older binaries: add the fallback the file already uses for start/finish — when `capcom context` is unknown, use `capcom show <id>` plus `capcom dep list <id>`. SKILL.md holds no Managed block, so its Work loop list is hand-edited.
**Files:** internal/agents/block.go; internal/agents/block_test.go; AGENTS.md; docs/site/agents/managed-block.md; plugins/capcom/skills/capcom/SKILL.md
**Read first:** internal/agents/block.go — loopOpening, claimLine, render; internal/agents/block_test.go — goldenBlock, keepTheTokenBlock, preStartBlock, TestRender_Notes_MatchesGolden, Status table (stale cases);
internal/cli/agents.go — newAgentsCommand (--check); internal/cli/know_add.go — syncAgentsBlock(ModeRefresh); plugins/capcom/skills/capcom/SKILL.md — "## 3. Work loop"; docs/site/agents/managed-block.md
**Tests:** golden; base render detected stale.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/agents/`; `go run ./cmd/capcom agents --check` exits 0; `grep -q 'capcom context <id>' docs/site/agents/managed-block.md plugins/capcom/skills/capcom/SKILL.md`.
**Commit:** `feat(agents): tell agents to run capcom context before an issue`

## 15. gitx reads the store at a past commit

**What:**
**Goal:** `gitx` offers read-only helpers: the last commit on HEAD before a time (none → empty), and
the files under a pathspec at a revision with their blob ids. Both run read-only git as the rest of
the package.
**Approach (assumed at the header base):** `git rev-list -1 --before=<unix> HEAD` and
`git ls-tree -r -z --full-tree <rev> -- <pathspec>`, through the package's `run`. `ShowFile` and
`HasCommit` already exist.
**Regression guard.** On an unborn HEAD (`capcom init`, no commit yet) `git rev-list … HEAD` and `ls-tree HEAD` fail: check `HasCommit(dir, "HEAD")` first and return "" with no error; item 17 prints `nothing changed since <since>` there.
**Files:** internal/gitx/history.go; internal/gitx/history_test.go
**Read first:** internal/gitx/gitx.go — run, Error; internal/gitx/branches.go — HasCommit, ShowFile, ChangedFiles; internal/gitx/gitx_test.go — newRepo, setHermeticGitEnv, git;
internal/gitx/branches_test.go — commitFile, TestHasCommit_UnbornHead_IsFalseWithoutError
**Tests:** real temp repo with dated commits (`GIT_COMMITTER_DATE`): before first commit, between,
after; ls-tree at two revisions; an unborn HEAD returns empty with no error.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/gitx/`; `go vet ./internal/gitx/`
**Commit:** `feat(gitx): read the commit before a time and the files at a revision`

## 16. Compute a recap between two commits

Depends on item 15.

**What:**
**Goal:** New package `internal/recap` computes, from `.capcom/` at a boundary commit (or none) and
HEAD, keyed by id: Issues closed (with reason), filed, status changed (not into a closed category;
reopen counts), Log entries added, and Notes added, changed (blob differs) and retired (own `retired`
turned true, or file removed), each with id and title. Issues removed outright are not listed.
**Approach (assumed at the header base):** One deep entry point (`recap.Compute(root, cfg, boundary)`)
over `gitx` (item 15), `gitx.ShowFile`, `issue.Parse`, `knowledge.Parse`; closed via
`cfg.CategoryOf(...).IsClosed()`. Export a log-entry reader from `internal/issue/merge.go` (wrap
`parseLog`), never a copy of `logEntryPattern`.
**Regression guard.** A rename keeps the old id only as an Alias: before grouping, pair an id gone from HEAD with the HEAD file whose `aliases` holds it and treat the pair as one id.
Notes changed excludes Notes added and Notes retired (as Status changed already excludes closing).
Read files as internal/elsewhere/elsewhere.go branchStatus/issueID do: skip hidden and non-.md names, read at most 1 MiB (tree.ErrTooLarge), turn a failure into a warning, resolve HEAD to its object name once, and read only paths whose blob id differs between the two snapshots (item 15's blob ids).
**Files:** internal/recap/recap.go; internal/recap/recap_test.go; internal/issue/merge.go; internal/issue/merge_test.go
**Read first:** internal/issue/merge.go — parseLog, parseLogContent, logEntryPattern; internal/elsewhere/elsewhere.go — branchStatus, issueID, maxBranchCopySize; internal/issue/file.go — Parse;
internal/knowledge/note.go — Parse, Note.IsRetired; internal/issue/mutate.go — Close, Reopen, logBounds; internal/config/config.go — Config.CategoryOf, Category.IsClosed;
internal/store/store.go — IssuesPathspec, KnowledgeDir; internal/cli/mv.go — issueMvHelp, noteMvHelp
**Tests:** scripted commits (close, create, log, status change, `mv`, Note add/change/retire/remove)
each listed once in the right group; a renamed Issue and Note as one id; a retired Note not also changed; an unparsable file a warning; no boundary lists everything as new.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/recap/ ./internal/issue/`; `go vet ./internal/recap/ ./internal/issue/`
**Commit:** `feat(recap): compute what changed in .capcom between two commits`

## 17. capcom recap

Depends on items 5, 16.

**What:**
**Goal:** `capcom recap [--since <value>]` prints the groups in the order Closed, Filed, Status
changed, Log, Notes added, Notes changed, Notes retired, each as a heading with lines
`  <id>  <title>` (plus the reason, the new status or the entry); empty groups omitted, nothing at
all prints `nothing changed since <since>`. `--since` takes `YYYY-MM-DD` (local midnight), `<n>d`, or
a git ref; default `7d`. An unknown ref is invalid with hint `git log --oneline -- .capcom`. `--json`:
kind `recap`.
**Approach (assumed at the header base):** New `internal/cli/recap.go`; date forms tried before the ref
(`gitx.HasCommit`); boundary via item 15. Register, file map, json-output kind row, gendocs.
**Files:** internal/cli/recap.go; internal/cli/recap_test.go; internal/cli/root.go; internal/cli/doc.go; docs/site/reference/json-output.md; docs/site/reference/cli/
**Read first:** internal/cli/root.go — newRootCommand, flagKV; internal/cli/help_text_test.go — TestHelpText_EveryCommand_SetsLongAndExample, availableCommands; internal/cli/doc_test.go — TestDocFileMap_MatchesDirectory;
internal/cli/testharness_test.go — runCLI, setHermeticGitEnv; internal/clidoc/reference_test.go — TestReferenceUpToDate; internal/gitx/branches.go — HasCommit;
docs/site/reference/json-output.md — envelope `kind` row
**Closes:** recap-command
**Tests:** real temp repo: each `--since` form; unknown ref with hint; text and JSON.
**Acceptance:** `go run ./cmd/gendocs`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Recap|HelpText|DocFileMap'`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`; `go vet ./internal/cli/`
**Commit:** `feat(recap): summarise what changed since a date or ref`

## 18. Completion offers issue ids

Depends on item 13.

**What:** Recast at the regression check (2026-10-26).
**Goal:** Every positional Issue argument completes Issue ids and aliases, each described by its title:
`show`, `update`, `close`, `reopen`, `start`, `finish`, `log` (first arg), `context`, `dep add|rm` (both),
`dep list`, `dep graph`, `related add|rm` (both), `mv`, `rm`, and `ui` (tab name, then an id of that
tab's kind). Open issues come first, then closed, each by id, order kept. Outside a repo or store:
no candidates, no file completion. `capcom __complete show stand` lists `standing-parents`.
**Approach (assumed at the header base):** New `internal/cli/complete.go`: opens `store.Open` +
`index.Open(…, ModeRefresh)` itself (completion runs without `prepare`) and closes it; candidates from
`Index.List(index.Filter{IsClosedIncluded: true})`; `ShellCompDirectiveKeepOrder|NoFileComp`.
`newMvCommand`/`newRmCommand` branch on issue vs Note verbs.
**Regression guard.** add `context <id>` (item 13's command) to the list of positional Issue arguments that complete Issue ids.
mv and rm complete argument 1 only (mv's second is `<new-id>`, where an existing id is never valid). The Goal's `__complete show stand` line reads: in a store holding Issue standing-parents, it lists that issue with its title; the test files that issue.
Index.List holds no Notes: for the `ui knowledge` tab, read Index.Notes(true) (retired included, as the tab lists them), ids and aliases described by title; item 19 reuses this Note helper rather than adding a second.
**Files:** internal/cli/complete.go; internal/cli/complete_test.go; internal/cli/doc.go; internal/cli/show.go; internal/cli/update.go; internal/cli/close.go; internal/cli/node_verbs.go; internal/cli/dep.go; internal/cli/start.go; internal/cli/finish.go; internal/cli/log.go; internal/cli/reopen.go; internal/cli/related.go; internal/cli/dep_list.go; internal/cli/dep_graph.go; internal/cli/ui.go; internal/cli/context.go
**Read first:** internal/cli/root.go — newRootCommand, prepare, prepareIndex; internal/index/query.go — Index.List, Filter.IsClosedIncluded, Index.issues; internal/index/index.go — Open, Entry.Aliases, Entry.Category;
internal/index/notes.go — Index.Notes, NoteEntry; internal/cli/node_verbs.go — newMvCommand, newRmCommand, nodeVerbs.idKind; internal/cli/ui.go — newUICommand, uiTarget;
internal/ui/ui.go — TabReady, TabIssues, TabKnowledge, Tab.String (tabOrder is unexported); internal/cli/doc_test.go — TestDocFileMap_MatchesDirectory
**Tests:** `__complete` via `runCLI`: prefix match, alias, closed after open, outside a store; `__complete ui ""` (issues, ready, knowledge); `__complete ui knowledge <prefix>` (a Note); `__complete mv <id> ""` (no candidates, :NoFileComp); `__complete context <prefix>`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Complete|DocFileMap'`; `go vet ./internal/cli/`
**Commit:** `feat(cli): complete issue ids in shell completion`

## 19. Completion offers Note ids, statuses and types

Depends on item 18.

**What:**
**Goal:** Note ids (not retired, described by title) complete for `know show|update|tree|mv|rm` and
`know add --parent`, `know mv --parent`, `know update --add-superseded-by|--remove-superseded-by`.
Issue ids complete for `--parent` (`create`, `list`, `mv`) and `--dep`. `--status` offers what each
flag accepts, described by category: `create`/`update`/`reopen` not closed, `close` closed only,
`list` all; `list --category` the categories; `--type` the configured types. Comma-separated flags
complete the part after the last comma.
**Approach (assumed at the header base):** `RegisterFlagCompletionFunc` beside each flag; helpers in
`complete.go`; config from the opened store.
**Regression guard.** The after-last-comma rule applies to StringSlice flags only: --add/--remove-superseded-by are StringArray and take no commas. The --parent of mv and know mv is one flag in node_verbs.go: branch its completion on verbs.idKind.
cobra keeps RegisterFlagCompletionFunc entries in a package-global map, never removed: register flag completion funcs only when args[0] is `__complete`/`__completeNoDesc`, so normal runs, tests and cli.Command() register none.
**Files:** internal/cli/complete.go; internal/cli/complete_test.go; internal/cli/know.go; internal/cli/know_add.go; internal/cli/know_update.go; internal/cli/create.go; internal/cli/list.go; internal/cli/reopen.go; internal/cli/update.go; internal/cli/close.go; internal/cli/node_verbs.go; internal/cli/know_show.go; internal/cli/know_tree.go; internal/cli/root.go
**Read first:** internal/cli/flags.go — flagStatus, flagType, flagParent, valueChecker.status, valueChecker.category; internal/cli/node_verbs.go — newMvCommand; internal/cli/update.go — newUpdateCommand;
internal/cli/close.go — newCloseCommand; internal/cli/list.go — newListCommand; internal/cli/know_update.go — flagAddSupersededBy, flagRemoveSupersededBy;
internal/index/notes.go — Index.Notes, NoteEntry; internal/config/config.go — Config.Statuses, Category.IsClosed
**Closes:** id-completion
**Tests:** Note ids; `__complete update x --status ""` lists not-closed statuses; close; types; comma.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Complete'`; `go vet ./internal/cli/`
**Commit:** `feat(cli): complete Note ids, statuses and types`

## 20. UI tab labels carry their counts

**What:**
**Goal:** Each loaded tab label reads `Issues [n]`, `Ready [n]`, `Knowledge [n]`: n is the rows the tab
lists with no filter (Issues: not in a closed category, standing included; Ready: the Ready queue;
Knowledge: Notes the tree shows, outside retired subtrees and not background). With a filter active it
reads `[m/n]`, m the matched items the tab shows (`0` when the filter has a problem or error). Before
its first load a tab shows no count. Counts follow every reload; clicking a label selects its tab.
**Approach (assumed at the header base):** Derive the label at render time in `(m model) tabBar()`
(`internal/ui/layout.go`) from `m.ready`, `m.issues`, `m.knowledge` and `m.filters`; `tabAt` becomes a
model method using the same labels. Regenerate goldens with `go test ./internal/ui -update`.
**Regression guard.** Under an isClosedShown filter (e.g. `status:done`) the Issues tab shows closed matches, so n counts the closed issues too and `[m/n]` keeps m <= n; a filter_test case pins it.
Wider labels move the version-label thresholds: the path label yields at 80 columns (say so in the sidecar CHANGELOG entry); layout_test's width cases use the counted tabs, not strippedTabs, since goldens never show the drop.
**Files:** internal/ui/layout.go; internal/ui/mouse.go; internal/ui/layout_test.go; internal/ui/mouse_test.go; internal/ui/filter_test.go; internal/ui/testdata/golden/
**Read first:** internal/ui/layout.go — tabBar, tabBarLabel, tabAt, fit; internal/ui/mouse.go — paneTabBar case; internal/ui/filter.go — sidebarFilter (matched, isClosedShown, problem, err);
internal/ui/ready.go — readyTab.all/entries, applyReady; internal/ui/knowledge.go — knowledgeTab, visibility (IsRetired, IsBackground); internal/ui/issues.go — loadIssues, issuesTab.entries;
internal/ui/layout_test.go — strippedTabs; internal/ui/mouse_test.go — tabColumn
**Closes:** ui-tab-counts
**Tests:** labels with and without a filter, cleared filter, count after close/create; `[m/n]` with m <= n under `status:done`; tab clicks at the
new widths.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/`; `go vet ./internal/ui/`
**Commit:** `feat(ui): show item counts on the tab labels`
