# Repeated flags are never silently dropped

**Goal:** a single-value flag given twice is refused before anything runs, naming the flag;
`--body-append` and `--body-replace-old`/`--body-replace-new` on `update` and `know update` may
be repeated and all apply, in order, all or none.
**Date:** 2026-10-19
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 68c2aeb
**Closes:** repeated-flag-dropped

**Regression check (2026-10-19, 68c2aeb):**
- 1: guard folded
- 2: guard folded
- 3: guard folded

**Sources:**
- `.capcom/issues/agent-ergonomics/repeated-flag-dropped.md` (the issue; its `## Acceptance`)
- Note `cli-test-harness`, Note `error-messages`

**Ratified design calls** (the user, 2026-10-19):
- **Scope:** every command refuses a repeated single-value flag, read and write commands alike.
- **Bool flags:** a bool flag given twice is refused too (`--standing --standing`, `--retire --retire=false`).
- **Edit order:** all replacement pairs in the order given, then all appends in the order given.
- **--body-append-file:** stays single-value; given twice it is refused like any other.

**Standing requirements:**
- skills: coding-standards
- CLI tests go through `runCLI`/`runCLIWith`, never `t.Parallel()` (Note `cli-test-harness`).
- Tests on this host: `GOMEMLIMIT=2GiB`, one package at a time, never `./...` with `-race`.
- A help-text change regenerates the committed reference: `go run ./cmd/gendocs`.

**Out of scope:**
- Repeatable body edits on any command other than `update` and `know update`.
- Inline body flags (`inline-body-flags`), flag suggestions (`suggest-commands-flags`).
- The UI write path (`internal/ops` callers in `internal/ui`).

## 1. Refuse a repeated single-value flag on every command — ✅ DONE (2026-10-19)

NOTES (2026-10-19): consequential edit — docs/site/reference/json-output.md: made necessary by the new usage error; the exit-code table's row for 2 now names a single-value flag given more than once

**What:**
**Goal:** any flag that holds one value (string, int, bool, duration…), given more than once to
any capcom command, fails as a usage error (exit 2) `--<name> given <n> times; give it once`
before the store or index opens; slice, array and count flags still accumulate.

**Regression guard.** Add a `flag_once.go` entry to the `internal/cli/doc.go` file map beside
`flags.go`, or `TestDocFileMap_MatchesDirectory` goes red; move pflag into go.mod's direct require
block (`go mod tidy`). The refusal covers every command that runs: root and group commands still
print their help, and cobra's help and completion commands are out of scope.

**Approach (assumed at the header base):** fix for the defect in `repeated-flag-dropped`: pflag's
scalar `Value.Set` overwrites. New file `internal/cli/flag_once.go` holds a `pflag.Value` wrapper
that delegates `String`/`Set`/`Type` and counts `Set` calls. `newRootCommand` (`root.go`), after
every `AddCommand`, walks the tree (`Commands()` recursively, root included) and wraps each local
and persistent flag whose `Type()` is not a slice/array/count type (`*Slice`, `*Array`, `count`);
`Flag.NoOptDefVal` is untouched so bare bools still parse. `prepare` checks the counts of
`cmd.Flags()` right after `selectRenderer` succeeds, so the refusal renders in `--json`/`--kv`
too, and returns `output.UsageErrorf`. Type delegation keeps `cli.Command()` and the generated
reference byte-identical. One deep helper; no per-command code.

**Files:** internal/cli/flag_once.go, internal/cli/root.go, internal/cli/flag_once_test.go,
internal/cli/doc.go, go.mod
**Read first:** internal/cli/root.go — newRootCommand, prepare, selectRenderer; internal/cli/doc.go — file map;
internal/cli/doc_test.go — TestDocFileMap_MatchesDirectory; internal/cli/testharness_test.go — runCLI;
internal/cli/cli_test.go — decodeEnvelope; internal/clidoc/reference_test.go — TestReferenceUpToDate

**Tests:** `internal/cli/flag_once_test.go` via `runCLI`: `update --title T1 --title T2`,
`--status` twice, `--priority 1 --priority 2`, `--plan` twice, `--standing --standing` and
`know update --title` twice each exit 2 naming the flag, and the file is byte-identical after;
`ready --limit 1 --limit 2` exits 2; `show <id> --json --json` exits 2 with a JSON error envelope;
`list --label a --label b` and `know update --add-ref a --add-ref b` still succeed.

**Acceptance:**
- `gofmt -l internal/cli` prints nothing; `go vet ./internal/cli/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/` (reference unchanged)

**Commit:** `fix(cli): refuse a single-value flag given more than once`

## 2. Repeated body edits on capcom update — ✅ DONE (2026-10-19)

NOTES (2026-10-19): a single --body-replace-old that does not match now also fails worded `--body-replace-old #1 "<old>": <the former message>`, since the one bodyEdits.apply path names every pair; the uneven-count usage error reads `<n> --body-replace-old and <m> --body-replace-new given; give them in pairs`; bodyEdits.apply also runs checkPairs itself, so a caller that skips the RunE check gets the usage error rather than an index panic

**What:**
**Goal:** `capcom update` takes `--body-append` and the `--body-replace-old`/`--body-replace-new`
pair several times; every pair applies in order against the body as the pairs before it left it,
then every append in order; a pair whose old text does not occur exactly once fails as invalid,
naming that pair (`--body-replace-old #<n> "<old>": …`), and nothing is written; unequal counts of
old and new are a usage error naming both counts.

**Regression guard.** each new updateExample line is one physical line starting "  capcom update " (help_text_test.go pins the shape)

**Approach (assumed at the header base):** fix for the defect in `repeated-flag-dropped`.
`updateOptions.bodyAppend`, `replaceOld`, `replaceNew` become `[]string` bound with
`StringArrayVar` (never `StringSliceVar`: commas stay literal). One shared helper in
`internal/cli/node_edit.go` — `bodyEdits{olds, news, appends []string}` with a count check and an
`apply(body string) (string, error)` looping `edit.ReplaceBody` then `edit.AppendBody` — is the
single place the sequence lives; item 3 calls it too. `applyUpdate` calls it where it calls
`edit.ReplaceBody`/`edit.AppendBody`, keeping its `output.Errorf(output.CodeInvalid, …)` typing
and the `--body-append-file` append last. The count check runs in `RunE` before `editIssue`.
`updateLong` says the two body flags repeat and apply replacements first; `updateExample` gains
one line with two replace pairs. Regenerate `docs/site/reference/cli/capcom_update.md`.

**Files:** internal/cli/update.go, internal/cli/node_edit.go, internal/cli/write_test.go,
docs/site/reference/cli/capcom_update.md
**Read first:** internal/cli/update.go — applyUpdate, newUpdateCommand, updateOptions; internal/cli/node_edit.go — editIssue;
internal/edit/edit.go — ReplaceBody, AppendBody; internal/cli/write_test.go — newWriteStore;
internal/cli/help_text_test.go — TestHelpText_EveryCommand_SetsLongAndExample

**Tests:** in `write_test.go`: body `alpha beta gamma` with pairs alpha→A, gamma→G gives
`A beta G`; a second pair whose old text is absent exits 1 naming pair 2 and the file is
byte-identical; a later pair matching text an earlier pair inserted applies; two old with one new
exits 2; `--body-append one --body-append two` appends both in order; `--body-append "a, b"` keeps
the comma.

**Acceptance:**
- `gofmt -l internal/cli` prints nothing; `go vet ./internal/cli/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`

**Commit:** `fix(cli): apply every repeated body edit of capcom update`

## 3. Repeated body edits on capcom know update — ✅ DONE (2026-10-19)

NOTES (2026-10-19): a single --body-replace-old that does not match on know update now also fails worded `--body-replace-old #1 "<old>": <the former message>`, still at the Note file, since applyNoteUpdate goes through the shared bodyEdits.apply

Depends on item 2.

**What:**
**Goal:** `capcom know update` takes `--body-append` and the replace pair several times with the
same order, all-or-none, pair-naming and count rules as `capcom update`; `--body-file` still
excludes every body edit flag.

**Regression guard.** each new knowUpdateExample line is one physical line starting "  capcom know update " (help_text_test.go pins the shape)

**Approach (assumed at the header base):** fix for the defect in `repeated-flag-dropped`.
`knowUpdateOptions.bodyAppend`, `replaceOld`, `replaceNew` become `[]string` via
`StringArrayVar`; `applyNoteUpdate` calls item 2's `bodyEdits` helper after the `--body-file`
replacement and before the `--body-append-file` append, returning its error as it returns
`edit.ReplaceBody`'s today. The count check runs in `RunE` before `updateNote`. The
`MarkFlagsMutuallyExclusive`/`MarkFlagsRequiredTogether` calls stay. `knowUpdateLong` and
`knowUpdateExample` show a repeated body edit. Regenerate
`docs/site/reference/cli/capcom_know_update.md`.

**Files:** internal/cli/know_update.go, internal/cli/know_update_test.go,
docs/site/reference/cli/capcom_know_update.md
**Read first:** internal/cli/know_update.go — applyNoteUpdate, newKnowUpdateCommand, knowUpdateOptions;
internal/cli/node_edit.go — editNode; internal/cli/know_update_test.go — newKnowStore, readNoteFile,
TestKnowUpdate_BodyEdits_ReplaceThenAppend; internal/cli/help_text_test.go — TestHelpText_EveryCommand_SetsLongAndExample

**Tests:** in `know_update_test.go`: two replace pairs apply in order; a failing second pair exits
1 naming the Note file and pair 2, file byte-identical; unequal counts exit 2; two
`--body-append` values both appended in order.

**Acceptance:**
- `gofmt -l internal/cli` prints nothing; `go vet ./internal/cli/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`

**Commit:** `fix(cli): apply every repeated body edit of capcom know update`
