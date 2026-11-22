# Plan: the rest of the session friction — lock shims, apply, ids, show and list

**Goal:** Close the remaining `session-friction-0160` issues: the holder rule holds behind a capcom
shim, apply and the git-repo failure guide a first try, a refused id names the rule it broke, and
`show`, `know show`, `know add` and `list` answer in one call what agents now loop and pipe for.
**Date:** 2026-11-22
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 6fe6719

**Sources:**
- `.capcom/issues/session-friction-0160/` children named on the Closes lines
- ADR 0002 (`capcom know show adr-0002 --with-children`), ADR 0016, ADR 0017
- Notes `issue-workflow`, `cli-test-harness`, `coding-conventions`, `store-layout`, `error-messages`

**Ratified design calls** (Airic, 2026-11-22):
- **Id length:** ids stay 1 to 4 words and 40 characters; only the refusal messages change (decided after the regression check).
- **list by ids exit code:** `capcom list <id>...` exits 0 when ids are not found; a missing id is a row (`<id>  not found`; JSON `{"id": "<arg>", "status": "not_found"}`).
- **know update --body:** both `know add` and `know update` get `--body <text>`, taken literally, exclusive with every other body flag of that command.
- **--brief / --section in JSON:** `--brief` omits `body`; `--section` puts only the chosen sections, heading lines included, in `body`. Same schema otherwise.

**Standing requirements:**
- skills: coding-standards
- Run single packages with `GOMEMLIMIT=2GiB`; never `./...` with `-race`; never two test commands at once (AGENTS.md).
- CLI tests go through `runCLI` (Note `cli-test-harness`); never `t.Parallel()`.
- After changing a command's help text: `go run ./cmd/gendocs` and commit `docs/site/reference/cli/`; never hand-edit it. Help goldens under `internal/cli/testdata/help/` and JSON/KV goldens refresh with the package's `-update` flag, then are read before committing.
- Knowledge Notes are written with `capcom know add` / `capcom know update`, then `capcom check`.
- Any deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- Title resolution for `capcom show` (issues); only Notes resolve by title.
- `--brief` / `--section` on `next`, `context` or `prime`.
- The `code-maturity` and `human-ergonomics` children.
- New apply operations or apply keys beyond the frontmatter-name aliases of item 3.
- Raising the id word limit: it needs a store format raise (ADR 0009); see explicit-format-upgrade.

**Regression check (2026-11-22, 6fe6719):**
- 1: guard folded (every holder-process definition, by a wider grep)
- 2: guard folded; supersedes docs/site/reference/json-output.md:121,183 for this one hint
- 3: guard folded
- 4: guard folded; `Closes: apply-schema-guesses` moved here from item 3
- 5: recast (Airic's decision: the limit stays at 4 words); yields to ADR 0009
- former 6 (help, docs and ADR 0002 say five words): dropped by Airic's decision; items 7-13 renumbered 6-12
- 7: guard folded (Airic's decision: `capcom context` unchanged)
- 8: recast (Airic's decision: a separate title lookup only `know show` calls); yields to ADR 0002's membership rules
- 9: guard folded (Airic's decision: `capcom context` unchanged)
- 10: guard folded (Airic's decision: `capcom context` unchanged)
- 11: guard folded
- 12: guard folded
- 5 (re-check): guard folded (the `invalid id "<id>": ` prefix is kept; nodestore and Fmt tests join the Acceptance)
- 7 (re-check): guard folded (Airic's decision: every argument completes; `noteShowResult` keeps its entries parameter for context)
- 8 (re-check): guard folded (Airic's decision: the fallback hooks into item 7's per-argument loop in know_show.go, never `a.resolveNote`; step (c) test case, several-match error and `excluded = 0` rows)
- 9 (re-check): guard folded (Airic's decision: brief/section passed explicitly; context, start and next keep full output)
- 10 (re-check): guard folded (Airic's decision: the trim passed explicitly to `noteShowResult`, zero for context)

## 1. A capcom shim is not the lock's holder process — ✅ DONE (2026-11-22)

NOTES (2026-11-22): the shim definition sentence ("a process named `capcom` that runs capcom once per call") was added to docs/site/concepts/tree-locks.md and the holder-process glossary Note, beyond the bare "or a capcom shim", so the term is defined where the holder process is.
NOTES (2026-11-22): the four Notes were changed with `capcom know update --body-replace-old/--body-replace-new` (a build of this tree), then their touched paragraphs reflowed to 100 columns by hand; `capcom check` (tree build) passes. The installed capcom 0.16.0's `capcom check` exits 1 on `AGENTS.md:31: the capcom block is out of date`, a version skew with the unreleased block, not from this item.
NOTES (2026-11-22): the first `## [Unreleased]` CHANGELOG entry still says "the nearest non-shell ancestor of capcom"; the closeout may want to add "or capcom shim" there when it applies this item's entry.

**What:**
Fix for the residual of plan `2026-11-22 - 00 - lock-holder-and-agent-guide-plan` (bf30b14): under
a per-call capcom shim the shim became the holder process, so the holder's later writes warned and
bare `lock touch` / `release` failed.
**Goal:** `proc.Anchor` skips an ancestor whose base name (lowercased, `.exe` stripped) is
`capcom`, so a lock acquired through a capcom shim records the next non-wrapper ancestor; a later
write from a different shim process under the same harness stays silent, and bare `lock touch` and
`lock release` succeed there.
**Approach (assumed at the header base):** add `capcom` to the skip set behind `isWrapper` in
`internal/proc/proc.go` (`wrapperNames`); the current process is never in the chain (`walk` starts
at the parent), so the real capcom is not skipped. `app.holderProcess` in `internal/cli/root.go`
is unchanged. Every Note or doc that defines the holder process as "the nearest non-shell
ancestor" gains "or capcom shim": find them with
`grep -rn "non-shell ancestor" .capcom/knowledge docs internal/agents`.
**Regression guard.** Every Note, doc or code comment that defines the holder process or `Anchor`'s
skip set gains "or a capcom shim"; find them with `grep -rnE "non-shell ancestor|not a shell( or a
wrapper)?|shells? and wrapper" .capcom docs/site internal cmd`, which replaces the Approach's grep.
Its hits at the base are all in **Files:** below.
**Files:** internal/proc/proc.go, internal/proc/proc_test.go, internal/cli/lock_warn_test.go, .capcom/knowledge/issue-workflow.md, .capcom/knowledge/decisions/adr-0017/, .capcom/knowledge/decisions/adr-0005/, .capcom/knowledge/glossary/glossary-coordination/holder-process.md, docs/site/concepts/tree-locks.md, docs/site/reference/file-format.md, internal/lock/lock.go, cmd/agenteval/doc.go, cmd/agenteval/workspace.go
**Read first:** internal/proc/proc.go — wrapperNames, isWrapper, Anchor, walk; internal/proc/proc_test.go —
TestAnchor_Chains; internal/cli/root.go — holderProcess; internal/cli/lock_warn_test.go — holderChain;
internal/cli/testharness_test.go — withAncestry
**Tests:** `TestAnchor_Chains` gains chains `[capcom, claude]` → claude and `[CAPCOM.EXE, bash, claude]` → claude. In `lock_warn_test.go`, via `withAncestry`: acquire under `[capcom(pid A), claude]`, then `create` and bare `lock touch` under `[capcom(pid B), claude]` — no warning, touch succeeds; a write under `[capcom(pid C), other-harness]` still warns.
**Acceptance:** `go vet ./internal/proc/ ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/proc/ && GOMEMLIMIT=2GiB go test -count=1 -run 'Lock|Warn|Anchor' ./internal/cli/ && capcom check`
**Closes:** anchor-skips-capcom-shim; lock-warns-own-writes; lock-touch-kept-token
**Commit:** `fix(proc): skip a capcom shim when anchoring the lock holder`

## 2. Outside a git repository, the failure says where to run capcom — ✅ DONE (2026-11-22)

NOTES (2026-11-22): `apply <file>` outside git is its own subtest of TestStoreCommands_OutsideAStore_FailAsBefore rather than a row of its command table, since the table builds each hint's command path from all args (the file name would land in the `--help` hint of the bad-config row).

**What:**
**Goal:** every store-backed command run outside a git repository fails with
`capcom: working directory <cwd> is not inside a git repository` and the hint
`hint: run capcom from inside the repository`, never a `capcom <command> --help` hint; `--json`
carries the same message and hint.
**Approach (assumed at the header base):** `store.locate` wraps `ErrNotGitRepository` with the cwd;
change that wrap to the wording above. `output.StoreFailure` gets its own branch for
`ErrNotGitRepository` that sets the hint, overriding `defaultHints[CodeInvalid]`; code `invalid`,
exit 1 (`ExitError`) as today. `complete.go` keeps returning the raw error.
**Regression guard.** The new `StoreFailure` branch builds with `Errorf` and sets `Hint`, never
`UsageErrorf` (exit 1 is pinned in cli_test.go and `TestInit_OutsideGit_FailsInvalid`). `ErrNotGitRepository`'s
own text changes with the wrap, so the sentinel text is not printed twice. Supersedes docs/site/reference/json-output.md:121,183
("a hint names the command to run next", `hint: <command>`): amend both to admit this where-to-run hint.
**Files:** internal/store/store.go, internal/store/store_test.go, internal/cli/output/errors.go, internal/cli/output/errors_test.go, internal/cli/cli_test.go, internal/cli/apply_test.go, docs/site/reference/json-output.md
**Read first:** internal/store/store.go — ErrNotGitRepository, locate; internal/cli/output/errors.go —
StoreFailure, defaultHints, Error.hintText; internal/cli/cli_test.go — TestStoreCommands_OutsideAStore_FailAsBefore;
internal/cli/output/errors_test.go — TestStoreFailure_MapsInitAndOpenErrors; internal/cli/init_test.go — TestInit_OutsideGit_FailsInvalid
**Tests:** `TestStoreCommands_OutsideAStore_FailAsBefore` pins the new message and hint for each command it covers, and `apply <file>` from a non-repo dir is added to it; `TestStoreFailure_MapsInitAndOpenErrors` pins the hint; a test asserts no `--help` in that stderr.
**Acceptance:** `go vet ./internal/store/ ./internal/cli/... && GOMEMLIMIT=2GiB go test -count=1 ./internal/store/ ./internal/cli/output/ && GOMEMLIMIT=2GiB go test -count=1 -run 'OutsideAStore|StoreFailure|Apply' ./internal/cli/`
**Closes:** apply-not-git-message
**Commit:** `fix(cli): say to run capcom inside the repository when there is none`

## 3. apply takes the frontmatter key names and shows the shape it wants — ✅ DONE (2026-11-22)

NOTES (2026-11-22): a value of the wrong shape under an alias is refused naming the alias the file gave (`labels is a mapping; ...`), not the key; the example file shows for any file that is no list (mapping or scalar), since both refusals share one branch in parser.list.

**What:**
**Goal:** an apply entry accepts `labels` wherever it accepts `label`, and `deps` wherever it
accepts `dep`, with the same value forms; an entry giving both spellings of one key is refused
naming both. A file that is a mapping fails with an error that prints a minimal valid file: a list
holding one `create` with `id` and `title`.
**Approach (assumed at the header base):** in `internal/batch/parse.go`, map the alias to its key
before `operationReader.spec` checks `keysOf`; `keyList` in the unknown-key error keeps listing the
canonical names only. The mapping error is built in `parser.list`.
**Regression guard.** `docs/site/reference/apply-file.md` documents the key set (the Operations
table, "a key the operation does not take … is refused", the value forms): name `labels` / `deps` as
aliases there, the both-spellings refusal and the mapping error's example. The existing parse case
"not a list" pins the old mapping message: rewrite its wantError, never add a second case beside it.
**Files:** internal/batch/parse.go, internal/batch/parse_test.go, internal/cli/apply_test.go, docs/site/reference/apply-file.md
**Read first:** internal/batch/parse.go — keysOf, operationReader.spec, parser.list, keyList; internal/batch/parse_test.go —
TestParse_InvalidFile_IsRefusedAtThePositionAndLine; internal/cli/apply.go — app.apply; docs/site/reference/apply-file.md
**Tests:** parse tests: `labels: [a, b]` on create equals `label: [a, b]`; `deps` likewise; `label` + `labels` refused; the "not a list" case's wantError becomes the new mapping message, example included, and that example parses. CLI: an apply file with `labels: [a, b]` on a create applies and the issue carries both labels.
**Acceptance:** `go vet ./internal/batch/ ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/batch/ && GOMEMLIMIT=2GiB go test -count=1 -run Apply ./internal/cli/`
**Commit:** `feat(apply): accept labels and deps, and show the expected shape`

## 4. apply --help carries an example file that applies as printed — ✅ DONE (2026-11-22)

NOTES (2026-11-22): the first create carries a one-line `body` too, so the example applies without the "has no body" warning; the test asserts stdout exactly, which keeps it warning-free.

**What:**
**Goal:** `capcom apply --help` shows a complete small apply file: two `create` entries, one with a
multi-line `body` and a `dep` on the other, and a `log` entry; a test extracts that example from
the help output and applies it in a fresh store successfully.
**Approach (assumed at the header base):** `applyExample` / `applyLong` in `internal/cli/apply.go`;
the test reads the example from `capcom apply --help` output, not from the Go constant.
**Regression guard.** The file goes in `applyLong`, not `applyExample` (every Example line must start
`  capcom apply `): after a blank line, indented 4+ spaces (a Markdown code block in capcom_apply.md),
entries short enough for `helpRules`' 4-line rule at 100 columns, ids of at most 4 words (`tree.ValidateID`).
This item closes apply-schema-guesses: its last Acceptance bullet is this item's work.
**Files:** internal/cli/apply.go, internal/cli/apply_test.go, docs/site/reference/cli/capcom_apply.md
**Read first:** internal/cli/apply.go — applyLong, applyExample; internal/cli/help_text_test.go —
TestHelpText_EveryCommand_SetsLongAndExample, TestHelpText_EveryCommand_PrintsExamplesBeforeShortRules, helpRules;
internal/clidoc/reference_test.go — TestReferenceUpToDate; internal/tree/ids.go — ValidateID; docs/site/reference/cli/capcom_apply.md
**Tests:** `TestApply_HelpExample_Applies`: run `apply --help`, cut the example, write it to a file, `apply` it, assert both issues, the dep and the log line exist.
**Acceptance:** `go vet ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 -run 'Apply|Help' ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`
**Closes:** apply-schema-guesses
**Commit:** `docs(apply): show a complete example file in apply --help`

## 5. A refused id names the rule it broke — ✅ DONE (2026-11-22)

NOTES (2026-11-22): did not run `capcom log id-word-limit ...` (the implementer may not comment on a ticket); the closing agent should log or give as the close reason: "the limit stays at 4 words: older binaries drop five-word ids without a store format raise (ADR 0009); see explicit-format-upgrade", which the issue's Acceptance asks for.
NOTES (2026-11-22): an empty id ("") is refused as `empty word; join words with single hyphens`, the empty-word rule the item lists, rather than with a message of its own.
NOTES (2026-11-22): the `a-b-c-d-e` create case is already covered by TestCreate_BadSlug_FailsInvalid in internal/cli/write_test.go (exit code and `invalid id` only), left unchanged as the item's Files do not list it.

**What:**
Recast at the regression check (2026-11-22, 6fe6719).
**Goal:** `tree.ValidateID`'s accepted set is unchanged (1 to 4 kebab words of a-z and 0-9, at most
40 characters), and each refusal names its rule: `<id>: 5 words, at most 4` (the actual count),
`<id>: upper-case letter "X"; ids are lower-case`, `<id>: character "_" not allowed; use a-z, 0-9
and hyphens`, `<id>: empty word; join words with single hyphens` (leading, trailing or doubled
hyphen), and the existing `longer than 40 characters`. All wrap `ErrInvalidID`.
**Approach (assumed at the header base):** replace the single `idPattern` check in
`internal/tree/ids.go` with ordered checks (length, empty, case, characters, words) so the first
broken rule is reported. Update the `ErrInvalidID` comment in `internal/tree/errors.go`. Pinned
strings: grep `must be 1 to 4 words` across `internal/`.
**Regression guard.** Ids stay 1 to 4 kebab words and 40 characters (Airic, 2026-11-22): a five-word
id would silently vanish for older binaries (ADR 0009) unless a store format raise were added, which
the open issue explicit-format-upgrade opposes; the item yields to ADR 0009. The case check runs
before the characters check, so the pinned `Bad_Name` sites expect the upper-case message. At the
base the old text is pinned only in the three test files listed (write_test.go and know_test.go pin
exit codes only). The item also runs `capcom log id-word-limit "<reason the limit stays at 4: older
binaries drop five-word ids without a store format raise; see explicit-format-upgrade>"`; it keeps
`**Closes:** id-word-limit`.
Each refusal keeps the `%w %q: ` prefix (`invalid id "<id>": <rule>`) and changes only the text after
the colon, so the prefix pinned in internal/cli/fmt_test.go, internal/nodestore/format_test.go and
internal/nodestore/move_refs_test.go still matches.
**Files:** internal/tree/ids.go, internal/tree/errors.go, internal/tree/tree_test.go, internal/check/check_test.go, internal/cli/check_test.go
**Read first:** internal/tree/ids.go — ValidateID, idPattern; internal/tree/tree_test.go — TestValidateID, TestWalk_DirectoryProblems_WordedAndLeftOut;
internal/check/check_test.go — TestRun_DirectoryProblems_WordedAsTheIndexWarnsThem; internal/cli/check_test.go — TestCheck_Problems_ListedOnePerLineAndExitOne;
internal/cli/fmt_test.go — TestFmt_InvalidFiles_WarnedAboutAndUntouchedWhileOthersAreFormatted; internal/nodestore/format_test.go — TestFormat_BrokenFiles_WarnedAboutAndLeftAlone
**Tests:** table test over each rule; a five-word id is refused with `5 words, at most 4`; `Bad_Name` gets the upper-case message at every pinned site; `a-b-c-d-e` is still refused by `capcom create`; the `invalid id "<id>"` prefix pinned by the fmt and nodestore tests still matches.
**Acceptance:** `go vet ./internal/tree/ ./internal/check/ ./internal/nodestore/ ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/tree/ ./internal/check/ ./internal/nodestore/ && GOMEMLIMIT=2GiB go test -count=1 -run 'Check|Create|Fmt' ./internal/cli/`
**Closes:** id-word-limit
**Commit:** `feat(tree): name the rule a refused id breaks`

## 6. know add and know update take --body — ✅ DONE (2026-11-22)

NOTES (2026-11-22): internal/cli/testdata/help/know-add.txt is not refreshed by the package's -update flag (that flag rewrites only the json/kv goldens); it was rewritten from the built binary's `capcom know add --help` output and read before handing over.

**What:**
**Goal:** `capcom know add --id x --title T --body "text"` writes the Note with that body, taken
literally; `capcom know update <id> --body "text"` replaces the body. On `know add`, `--body` with
`--body-file` is refused; on `know update`, `--body` with any of `--body-file`, `--body-append`,
`--body-append-file`, `--body-replace-old` is refused — each with a usage error naming both flags,
writing nothing. Both `--help` texts show a `--body` example.
**Approach (assumed at the header base):** mirror `create`'s `--body` (`newIssue`'s body-source
switch, `MarkFlagsMutuallyExclusive`); in `know_add.go` the body source joins the `readTextInput`
/ `noteBody` path; in `know_update.go` it joins `readNoteInputs` and its exclusive groups.
**Files:** internal/cli/know_add.go, internal/cli/know_update.go, internal/cli/know_test.go, internal/cli/know_update_test.go, internal/cli/testdata/help/know-add.txt, docs/site/reference/cli/
**Read first:** internal/cli/know_add.go — newKnowAddCommand, noteBody, knowAddExample; internal/cli/know_update.go —
newKnowUpdateCommand, readNoteInputs, applyNoteUpdate; internal/cli/create.go — newIssue;
internal/cli/know_test.go — TestKnowAdd_RefusedInputs_FailWithoutWriting
**Tests:** add with `--body` (text with `$`, backticks and a newline kept verbatim); update with `--body`; each refused pair writes nothing and exits 2.
**Acceptance:** `go vet ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 -run 'KnowAdd|KnowUpdate|Help' ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`
**Closes:** know-add-body-flag
**Commit:** `feat(know): take --body on know add and know update`

## 7. know show prints several Notes — ✅ DONE (2026-11-22)

NOTES (2026-11-22): the failure collection of showResult moved into argumentFailures (show.go), called by showResult and by the new shownNotes (know_show.go); resolveNote and know.go are unchanged (know tree still calls resolveNote), and context.go is untouched, with noteShowResult keeping its entries parameter.

NOTES (2026-11-22): consequential edit — docs/site/reference/json-output.md: made necessary by know show joining several unknown-id messages as show does (the paragraph named show alone).

NOTES (2026-11-22): consequential edit — docs/site/concepts/knowledge.md: made necessary by know show taking several arguments (one example line added to the browsing block).

**What:**
**Goal:** `capcom know show <arg>...` takes one or more arguments and prints each Note in turn,
separated as `show` separates issues, in text and `--json`; `--with-children` applies to each. An
argument naming no Note is reported as `capcom show` reports a missing issue: the found Notes still
print, the failure messages are joined, and the exit code is the first failure's.
**Approach (assumed at the header base):** `newKnowShowCommand` takes `cobra.MinimumNArgs(1)`;
`noteShowResult` loops over args through `a.resolveNote`. Reuse `showResult`'s failure collection
in `internal/cli/show.go`: when the loop would be copied, extract it into one helper in `show.go`
called by both (one reason to change: the partial-failure contract).
**Regression guard.** `capcom context` output stays unchanged, pinned by its existing tests.
Keep `noteShowResult(entries []index.NoteEntry)` as is for context; resolve the arguments (and collect
the failures) in the RunE or a new wrapper, not in `noteShowResult`; context.go:292 is the caller left
unchanged. Completion covers every argument (`completeNoteIDs(everyArgument)`, as `show` does), so
complete_test.go:159 (`know show store-layout ""`) flips to expect candidates.
**Files:** internal/cli/know_show.go, internal/cli/know.go, internal/cli/show.go, internal/cli/know_test.go, internal/cli/complete_test.go, internal/cli/context.go, docs/site/reference/cli/
**Read first:** internal/cli/know_show.go — newKnowShowCommand, noteShowResult; internal/cli/show.go — showResult, resolveFailure;
internal/cli/know.go — resolveNote; internal/cli/context.go — noteShowResult caller; internal/cli/complete_test.go —
TestComplete_NoteArguments_ListNotRetiredNoteIDsWithTitles; internal/cli/read_test.go — TestShow_OneOfSeveralMissing_PrintsFoundAndExitsNotFound
**Tests:** `know show a b` text and JSON print both; `know show a missing` prints a, stderr names missing, exit 3; `--with-children` on two Notes; `know show store-layout ""` completes Note ids; the existing `capcom context` tests pass unchanged.
**Acceptance:** `go vet ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 -run 'KnowShow|Show_|Context|Complete_NoteArguments' ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`
**Closes:** know-show-several
**Commit:** `feat(know): show several Notes in one call`

## 8. know show resolves a Note by its title — ✅ DONE (2026-11-22)

NOTES (2026-11-22): ResolveNoteByTitle calls ResolveNote itself and falls back only on a *tree.NotFoundError, so know show makes one call per argument; the title lookup runs in its own read snapshot after ResolveNote's.

NOTES (2026-11-22): several matches fail with a new *index.TitleMatchesError (errors.Is tree.ErrNotFound) naming the ids in its message (`id "<arg>" not found: it matches several Notes: a, b`); it carries no Suggestions. In step (c) an ambiguous slug prefix counts as several matches; a not-found, unusable or duplicate slug counts as no match.

NOTES (2026-11-22): know show's usage line now reads `show <id|alias|prefix|title>...` and its help gains a by-title example.

NOTES (2026-11-22): consequential edit — docs/site/concepts/knowledge.md: made necessary by know show resolving Notes by title (browsing section example and one paragraph)

**What:**
Recast at the regression check (2026-11-22, 6fe6719).
Depends on item 7.
**Goal:** when `index.ResolveNote` fails on an argument with a `*tree.NotFoundError`, `know show`
tries, in order: (a) the title, case-insensitive; (b) the part of the title before the first `:`,
case-insensitive, trimmed; (c) the argument turned into a slug (lower-case, runs of non a-z0-9 to
one hyphen, leading and trailing hyphens dropped), resolved as an id, alias or prefix. The first step with any match decides: exactly one
Note resolves; several give the not-found error naming every match. No match returns
`ResolveNote`'s original error unchanged; an ambiguous, unusable or duplicate failure never falls
back. `know show --help` says what an argument may be. `capcom show` is unchanged.
**Approach (assumed at the header base):** a separate method in `internal/index/notes.go` (e.g.
`ResolveNoteByTitle`) that only `know show` calls; `index.ResolveNote` and its other callers
(`know update`, `know tree`, ui) are untouched; issue `Resolve` is untouched.
**Regression guard.** Leave `index.ResolveNote` untouched; add a separate method in
`internal/index/notes.go` (e.g. `ResolveNoteByTitle`) that only `know show` calls, and only after
`ResolveNote` fails with a `*tree.NotFoundError` (never on ambiguous, unusable or duplicate errors);
when no title step matches, return `ResolveNote`'s original error unchanged. Step (c) is kept only for
spellings `NormalizeArg` does not already resolve. The item yields to ADR 0002's membership rules
(adr-0002-membership: index readers answer every name as `tree.ResolveIn` does), which
`ResolveNote` keeps.
The title fallback hooks into item 7's per-argument loop in internal/cli/know_show.go, never into
a.resolveNote (know tree also calls it). Several matches fail with an error matching `tree.ErrNotFound`
under `errors.Is` that names the matched ids (in its message or as the failure's Suggestions), so the
exit stays 3 and the JSON code `not_found`. Steps (a) and (b) read only `excluded = 0` rows, as `readNote` does.
**Files:** internal/index/notes.go, internal/index/notes_test.go, internal/cli/know_show.go, internal/cli/know_test.go, docs/site/reference/cli/
**Read first:** internal/index/notes.go — ResolveNote, readNote; internal/tree/resolve.go — NormalizeArg, NotFoundError;
internal/cli/know_show.go — newKnowShowCommand; internal/cli/show.go — resolveFailure; internal/index/notes_test.go — noteFixture;
internal/cli/know_test.go — TestKnowShow_CitationSpelling_Resolves
**Tests:** index: each step resolves; two Notes with one title → not found naming both; an id still wins over a title; no match returns the original error; a title only an excluded duplicate file carries does not resolve. CLI: a Note `rel-flow` titled "Release steps: tag then publish" is reached by `know show "Release steps"` (step b) and by `know show "rel flow!"` (step c); both must fail at the base. Two Notes with one title exit 3 with JSON code `not_found` naming both. `know show adr` (an ambiguous prefix) fails as before.
**Acceptance:** `go vet ./internal/index/ ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/index/ && GOMEMLIMIT=2GiB go test -count=1 -run KnowShow ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`
**Closes:** know-show-by-title
**Commit:** `feat(know): resolve a Note by its title`

## 9. show takes --brief and --section — ✅ DONE (2026-11-22)

NOTES (2026-11-22): issue_render.go needed no change: showResult passes issueText the selected body ("" with --brief); the splitter is issue.Sections(body, headings) with a *issue.SectionsError, a plain function on the body (not an Issue method) so a Note body can use it too; SplitLog is untouched.

NOTES (2026-11-22): a missing heading fails as `<path>: no section "x"; its headings are A, B` (or `; the body has no headings`), the path in the message rather than the Error's Path field so that several joined failures each name their own file; argumentFailures gains addFailure for failures that are not lookup failures, and failureKV now builds the --kv record of every failed argument.

NOTES (2026-11-22): consequential edit — internal/cli/context.go: made necessary by show reusing flagBrief (its doc comment named only context)

NOTES (2026-11-22): consequential edit — docs/site/reference/json-output.md: made necessary by --brief and --section changing show's JSON body (one paragraph)

**What:**
**Goal:** `capcom show <id>... --brief` prints the fields only, no body and no Log;
`--section <heading>` (repeatable) prints the fields and each body section whose heading text
matches, case-insensitive, `#` marks and surrounding space ignored, in body order, heading line
included (`--section log` selects `## Log`). A section runs to the next heading of the same or
higher level, fenced code skipped. A heading the body lacks fails that issue (`invalid`) naming the
headings its body has; other issues still print. `--brief` with `--section` is a usage error. In
`--json`, `--brief` omits `body` and `--section` puts only the chosen sections in `body`.
**Approach (assumed at the header base):** one exported section splitter beside `SplitLog` and
`isSectionBoundary` in `internal/issue/mutate.go` (any ATX level, `rawfm.ClosedCodeLines` for
fences); `SplitLog` keeps its behaviour. `emitShown` / `showResult` / `issueText` apply it; `next`
shares `emitShown` but does not get the flags.
**Regression guard.** `capcom context` output stays unchanged (it does not get `--brief` / `--section`).
Pass brief and the sections to `showResult` / `emitShown` as explicit parameters, never read off `cmd`;
context, start and next pass the zero value and keep full output. Reuse the existing `flagBrief`
constant in context.go (a second one won't compile).
**Files:** internal/issue/mutate.go, internal/issue/mutate_test.go, internal/cli/show.go, internal/cli/issue_render.go, internal/cli/read_test.go, internal/cli/context.go, internal/cli/start.go, internal/cli/next.go, docs/site/reference/cli/
**Read first:** internal/cli/show.go — newShowCommand, emitShown, showResult; internal/cli/issue_render.go — issueText;
internal/issue/mutate.go — SplitLog; internal/cli/context.go — flagBrief; internal/cli/start.go — emitStarted;
internal/cli/next.go — emitShown caller
**Tests:** splitter: levels, fenced `##` ignored, Log last. CLI: `--brief` has no body line (text, JSON has no `body`); `--section acceptance` prints fields + that section only; `--section acceptance --section log`; missing heading lists the body's headings; `--brief --section x` exit 2; the existing `capcom context`, `start` and `next` tests pass unchanged.
**Acceptance:** `go vet ./internal/issue/ ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/issue/ && GOMEMLIMIT=2GiB go test -count=1 -run 'Show|Golden|Context|Start|Next' ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`
**Commit:** `feat(show): print the fields only or chosen body sections`

## 10. know show takes --brief and --section — ✅ DONE (2026-11-22)

NOTES (2026-11-22): re-derived from "noteText and noteData apply it": the trim is applied in noteShowResult through item 9's bodyView (show.go), passed as an explicit parameter (context passes bodyView{}); noteText and noteData are unchanged, know.go untouched. noteShowResult also returns the invalid failures, which know show adds to its argumentFailures; shownNotes now takes that collector instead of cmd.

NOTES (2026-11-22): a Note whose body lacks a named heading fails invalid as `<path>: no section "x"; its headings are ...`, the others still print; with --with-children a child lacking it fails alike. Lookup failures are collected before section failures, so with both the exit code is the lookup failure's whatever the argument order (show keeps argument order).

NOTES (2026-11-22): consequential edit — internal/cli/show.go: made necessary by know show reusing bodyView (its doc comments named only issues)

NOTES (2026-11-22): consequential edit — docs/site/reference/json-output.md: made necessary by --brief and --section changing know show's JSON body (one sentence)

NOTES (2026-11-22): consequential edit — docs/site/concepts/knowledge.md: made necessary by know show gaining --brief and --section (the browsing section lists its flags)

NOTES (2026-11-22): TestHelpText_EveryCommand_PrintsExamplesBeforeShortRules/capcom_show fails on item 9's committed showLong (a rule paragraph runs to 5 lines at 100 columns); know show's new help was split into two paragraphs to pass the same rule.

**What:**
Depends on items 7 and 9.
**Goal:** `capcom know show <arg>... --brief` and `--section <heading>` behave as on `show`
(item 9), text and `--json`, including with `--with-children` (each Note trimmed alike).
**Approach (assumed at the header base):** reuse item 9's splitter and flag handling; `noteText`
and `noteData` apply it. No second splitter.
**Regression guard.** `capcom context` output stays unchanged. Pass the trim as an explicit option to
`noteShowResult`, the zero value at context.go:292; context.go is the caller left unchanged.
**Files:** internal/cli/know_show.go, internal/cli/know.go, internal/cli/know_test.go, internal/cli/context.go, docs/site/reference/cli/
**Read first:** internal/cli/know_show.go — newKnowShowCommand, noteShowResult, noteText; internal/cli/know.go — noteData;
internal/cli/context.go — noteShowResult caller; internal/cli/show.go — item 9's flag handling; internal/issue/mutate.go —
item 9's splitter; internal/cli/know_test.go — TestKnowShow_WithChildren_ShowsTheNoteThenItsDirectChildren
**Tests:** `know show x --brief` no body (text, JSON); `--section` on a Note with H2 sections; missing heading lists the Note's headings; the existing `capcom context` tests pass unchanged.
**Acceptance:** `go vet ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 -run 'KnowShow|Context' ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`
**Closes:** show-brief-section
**Commit:** `feat(know): print a Note's fields only or chosen sections`

## 11. list takes ids and answers status and existence in one call — ✅ DONE (2026-11-22)

NOTES (2026-11-22): Index.Resolve's prefix step can refuse a unique prefix of an unusable or duplicate id with the same error as the exact name, and the guard forbids a new Index method, so such a prefix prints the unusable/duplicate row rather than not found (conservative: never reports a claimed name as free).

NOTES (2026-11-22): the unusable JSON row also carries `path` (the claiming file), mirroring the text row's path; documented in json-output.md.

NOTES (2026-11-22): the Help acceptance run fails on TestHelpText_EveryCommand_PrintsExamplesBeforeShortRules/capcom_show (5-line rule from the --brief/--section text in know_show.go, commit d73d0e1, item 10); it fails identically without this item's changes; capcom_list passes.

**What:**
**Goal:** `capcom list <id>...` prints one row per argument in argument order — the usual
`id  status  P<n>  title` row for an issue whose exact id or alias matches, closed ones included,
and `<arg>  not found` otherwise — and exits 0 either way. Unique prefixes do not match here, so
the call answers "is this id free". `--json` carries the same rows (`issueData`; a missing one as
`{"id": "<arg>", "status": "not_found"}`). Positional ids with any filter flag or `--tree` is a
usage error. `--help` shows both uses as examples.
**Approach (assumed at the header base):** `newListCommand` drops `cobra.NoArgs`; with args it
bypasses `rowFilter` and looks each up by the exact-id and alias steps of `tree.ResolveIn`
(`internal/tree/resolve.go`), without its prefix step; rows render through `issueLines` /
`issueListData`.
**Regression guard.** A name only an unusable file or two files claim gets its own row and JSON
status (`<arg>  unusable <path>` / `"unusable"`, `<arg>  duplicate` / `"duplicate"`), never `not found`.
Lookup: `Index.Resolve`, keeping only a match whose id or `Aliases` equal `tree.NormalizeArg(arg)`; no new
Index method. Ids are refused with every list flag but `--no-elsewhere` (`--tree`, `--depth`, `--all`, `--limit` included).
**Files:** internal/cli/list.go, internal/cli/issue_render.go, internal/cli/read_test.go, docs/site/reference/json-output.md, docs/site/reference/cli/
**Read first:** internal/cli/list.go — newListCommand, listResult, checkTreeFlags, rowFilter; internal/cli/issue_render.go —
issueLines, issueListData; internal/index/query.go — Index.Resolve; internal/tree/resolve.go — NormalizeArg
**Tests:** list of open, closed and missing ids (text, JSON, exit 0); an alias prints the current id; a prefix of an existing id is `not found`; a broken issue file's id gets the `unusable` row, never `not found`; ids with `--status` exit 2.
**Acceptance:** `go vet ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 -run 'List|Golden|Help' ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`
**Closes:** list-by-ids
**Commit:** `feat(list): list named ids, with a row for each that is not found`

## 12. The managed block teaches the one-call reads — ✅ DONE (2026-11-22)

NOTES (2026-11-22): the `list <id>...` line is a new line placed before `list --tree --depth 2` (keeping each search beside its outline); the test file also gained `preOneCallReadsBlock` with a "before the one-call reads" stale case and `TestRender_CommandList_TeachesTheOneCallReads`, following the existing pattern for earlier block changes.

**What:**
Depends on items 6, 7, 9 and 11.
**Goal:** the managed block names `capcom list <id>...` for the status of several issues or whether
an id is free, `capcom show <id> --section acceptance` for one section, and `capcom know show
<id>...` for several Notes; the repo's `AGENTS.md` block is refreshed and `capcom check` reports
no out-of-date block.
**Approach (assumed at the header base):** `loopOpening` (the `show` line) and `loopClosing` (the
`list` and `know show` lines) in `internal/agents/block.go` and their copies in `block_test.go`;
refresh `AGENTS.md` with `go run ./cmd/capcom agents`.
**Regression guard.** `docs/site/agents/managed-block.md` quotes the block word for word and
`docs/site/reference/file-format.md` describes its command list; no test checks either, so update
both to match the new lines.
**Files:** internal/agents/block.go, internal/agents/block_test.go, AGENTS.md, docs/site/agents/managed-block.md, docs/site/reference/file-format.md
**Read first:** internal/agents/block.go — loopOpening, loopClosing, render; internal/agents/block_test.go — goldenBlock,
TestRender_Notes_MatchesGolden, TestRender_CommandList_ListsApplyAfterCreate; docs/site/agents/managed-block.md —
block example; docs/site/reference/file-format.md — agents block description
**Tests:** block tests updated to the new lines.
**Acceptance:** `go vet ./internal/agents/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/agents/ && go run ./cmd/capcom check`
**Commit:** `docs(agents): teach list by ids, show sections and several Notes`
