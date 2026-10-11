# capcom maturity: format version, bad-file tolerance and migration gaps

**Goal:** Close the capcom-side prerequisites and defects found by the apogee beads-migration
feasibility scan. These are a store format version, write verbs that tolerate unrelated bad files,
conflict-marker detection, relative paths in tree errors, an accurate `--kv`, remote branches in
Elsewhere, `capcom fmt`, Codex hooks and a complete beads migration guide.

**Date:** 2026-10-11
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** e2268c2

**Sources:**
- `docs/handoffs/2026-10-11 - 01 - apogee-beads-migration-feasibility.md`
- `docs/format.md`, `docs/migrating-from-beads.md`, ADRs `docs/adr/0004-*`, `docs/adr/0006-*`
- `/home/airic/repos/apogee/.codex/hooks.json` (the Codex hooks shape)
- Knowledge Notes `cli-test-harness`, `coding-conventions`, `error-messages`, `store-layout`

**Ratified design calls** (user, 2026-10-11, unless marked):
- **Format version:** `config.yaml` key `format:`, an integer, with a missing key meaning 1. A store whose format is above the binary's highest fails every command loudly, except `capcom prime`, which reports the too-new error as a single `warning: store:` line and exits 0, because a Briefing must never cost the session its start (writer, 2026-10-11). Unknown keys stay errors, and any new key raises the format.
- **Template (writer, 2026-10-11):** `capcom init` writes `format` only as a comment line. An explicit `format: 1` would make v0.1.0 binaries reject the config as an unknown key.
- **Log merge conflicts:** `check` flags conflict markers and `docs/format.md` documents the resolution. No merge driver in this plan.
- **Elsewhere:** scan `refs/remotes/` as well as `refs/heads/`, de-duplicated against local branches. Never fetch.
- **`--kv`:** a per-command flag, registered only on the commands that emit KV records.
- **`capcom fmt`:** new verb that rewrites every issue and Note canonically. `--check` writes nothing and exits non-zero when a file would change.
- **Codex hooks:** `capcom init --codex-hooks` merges `capcom prime` hooks into `.codex/hooks.json`, mirroring `--claude-hooks`.
- **Init output (user, 2026-10-11):** `--codex-hooks` without `--claude-hooks` still prints the Claude snippet in text mode exactly as plain `capcom init` does; the two flags are independent, and `--codex-hooks` changes no Claude output.
- **Tracking:** one capcom issue per defect, closed by its item. Deferred ideas are filed unplanned.

**Standing requirements:**
- skills: coding-standards
- Every error names the file, and the line when known (Note `error-messages`). Paths are relative to the worktree root.
- CLI tests go through `runCLI`/`runCLIWith`, hermetic, never `t.Parallel()` (Note `cli-test-harness`).
- A new file in `internal/cli` is added to the file map in `internal/cli/doc.go`, which `doc_test.go` enforces.
- Pi host: `GOMEMLIMIT=2GiB`, one package at a time, never `./...` with `-race`, never two test runs at once.

**Out of scope:**
- A git merge driver for `## Log` (issue `log-merge-driver`), an issue `rm` verb (`issue-rm-verb`), first-class `related` links (`related-links`).
- Wiring Tree locks into the implement-plan skill (that lives outside this repo).
- Any store format above 1, and any new frontmatter key.
- Absolute paths outside the worktree (the index DB and lock records under the git dir).
- The apogee cut-over itself.

**Regression check (2026-10-11, e2268c2):**
- 1: guard folded (writer decision: the `prime` exception; ADR content greps)
- 2: guard folded (writer decision: `prime` warns on a too-new store)
- 3: guard folded (writer decision: exported other-file field)
- 4: guard folded (writer decision: other-file match, no double warning)
- 5: guard folded (writer decision: no double warning)
- 6: guard folded
- 7: guard folded
- 8: guard folded; supersedes ADR 0004 line 8 ("`ready` and `prime` scan local branches and worktrees")
- 9: guard folded
- 10: guard folded
- 11: guard folded

## 1. ADR 0009: the store declares its format version — ✅ DONE (2026-10-11)

NOTES (2026-10-11): the header's ratified "Format version" call already names the `capcom prime` exception at the base, so the regression guard's header correction needed no edit; the glossary term sits at the end of CONTEXT.md's Structure section

**What:**
**Goal:** `docs/adr/0009-the-store-declares-its-format-version.md` exists in the house ADR form and
records the format-version policy as the header's ratified calls give it:
- the `format:` key in `config.yaml`, where a missing key means 1
- a store above the binary's highest format fails every command, naming `config.yaml` and its line and telling the user to upgrade capcom
- unknown keys stay errors, and adding any key to issue, Note or config files raises the format
- `capcom init` writes the key only as a comment until a release raises the format
- why: an older binary must fail loudly rather than drop issues as bad files, or lose them on a rewrite

`CONTEXT.md` gains the glossary term **Format version**.

**Regression guard.** The ADR states the one exception to "fails every command": `capcom prime` reports the too-new error as a single `warning: store:` line and exits 0, because a Briefing must never cost the session its start (internal/cli/prime.go brief); also correct the header's ratified "Format version" call to name this exception (writer, 2026-10-11).

**Approach (assumed at the header base):** Copy the structure of the latest ADR (`0008`).
Cross-reference ADR 0004 (issues are versioned with the code).

**Files:** docs/adr/0009-the-store-declares-its-format-version.md, CONTEXT.md
**Read first:** docs/adr/0008-the-repo-owns-its-release-pipeline.md — heading, prose form; CONTEXT.md — Language, Structure, **Elsewhere**;
internal/cli/prime.go — brief; internal/config/config.go — DefaultFile, topLevel
**Tests:** none (docs only).
**Acceptance:**
- `test -f "docs/adr/0009-the-store-declares-its-format-version.md"`
- `grep -n "Format version" CONTEXT.md`
- `grep -n "format:" docs/adr/0009-the-store-declares-its-format-version.md`
- `grep -n "upgrade capcom" docs/adr/0009-the-store-declares-its-format-version.md`
- `grep -n "capcom prime" docs/adr/0009-the-store-declares-its-format-version.md`

**Commit:** `docs(adr): record the store format version and forward-compat policy`

## 2. Config `format` key; newer stores fail loudly — ✅ DONE (2026-10-11)

NOTES (2026-10-11): the `create` failure on a format-2 store is pinned by `TestCreate_NewerStoreFormat_FailsWithoutWriting` in `internal/cli/read_test.go` (beside the list/check tests), since the Goal names `create` but the Tests list gives it no file.

**What:** Depends on item 1.
**Goal:** In `config.yaml`, `format: 1` or a missing key loads normally. `format: 2` makes `list`,
`create` and `check` exit non-zero with
`.capcom/config.yaml:<line>: store format 2 is newer than this capcom reads (up to 1); upgrade capcom`.
`format: 0`, a negative number or a non-integer is a config error citing its line. The config
template that `capcom init` writes carries `# format: 1` with a one-line comment above it, and is
not an active key. `docs/format.md`'s config section documents the key and the policy, and links
ADR 0009.

**Regression guard.** Every command except `prime` fails on a too-new store; `prime` keeps turning the store.Open failure into one `warning: store:` line with exit 0, and a test pins that for `format: 2`.
`check` reports the too-new error as a `config` problem on stdout (`.capcom/config.yaml:<line>: store format 2 is newer...`, or `--json` code `config`) with exit 1; only `list` asserts stderr.
Read and check `format` from the root mapping before `topLevel` walks the other keys. `Default()` sets `Format: FormatVersion`, so a missing key and a missing config.yaml both mean 1.
`docs/format.md`'s config block gains the same `# format: 1` comment lines as `DefaultFile`, and a `format` bullet under it.

**Approach (assumed at the header base):** In `internal/config/config.go`, add a `Format int` field
and a `case "format"` beside the other top-level keys in the parse switch. Add the exported const
`FormatVersion = 1`. The too-new check runs in config loading, so `store.Open`'s failure path
(`storeFailure` in `internal/cli`) carries it to every indexed command. Confirm `check.Run` reaches
the same error, and that `capcom version` still runs outside a store.

**Files:** internal/config/config.go, internal/config/config_test.go, internal/cli/init_test.go, internal/cli/read_test.go, internal/cli/prime_test.go, docs/format.md
**Read first:** internal/config/config.go — Default, Parse, topLevel, eachKey, elsewhere, DefaultFile; internal/store/store.go — loadConfig, Open; internal/cli/init.go — storeFailure; internal/check/check.go — Run, CodeConfig; internal/cli/check.go — newCheckCommand;
internal/config/config_test.go — TestParse_DefaultFile_EqualsDefault, TestParse_InvalidConfig_FailsWithPathAndLine; internal/cli/init_test.go — TestInit_InvalidExistingConfig_FailsWithPathAndLine;
internal/cli/prime_test.go — TestPrime_BrokenConfig_ExitsZeroWithOneWarning
**Tests:**
- `config_test.go`: missing → 1; `format: 1` ok; `format: 2` → the too-new error with its line; `format: 0` and `format: x` → errors. A `format: 2` config with an unknown key above `format:` gives the too-new error, not `unknown key`.
- `read_test.go`: a store with `format: 2`, then `list` exits non-zero and stderr holds the exact message above with the relative path; `check` exits 1 and stdout holds that message as a `config` problem.
- `prime_test.go`: a store with `format: 2`, then `prime` exits 0 and prints one `warning: store:` line.
- `init_test.go`: the written config holds `# format: 1` and loads with Format 1.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/config/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Init|List|Check|Format|Prime'`
- `go vet ./internal/config/ ./internal/cli/`

**Closes:** store-format-version
**Commit:** `feat(config): add the store format version and refuse newer stores`

## 3. Tree errors name files relative to the worktree, and duplicate aliases cite their line — ✅ DONE (2026-10-11)

NOTES (2026-10-11): tree.Error also gained an exported Alias field (set on invalid and duplicate alias problems) so check can tell a duplicate alias from a duplicate id, which wrap the same ErrDuplicate sentinel; a Message() method renders Err plus Other without the path, and Error() keeps rendering both files.

NOTES (2026-10-11): check cites the aliases line for an invalid alias as well as a duplicate one (both blame the aliases key); no existing fixture covered the invalid-alias line.

NOTES (2026-10-11): the CLI's single relativizing method is app.treeLoadFailure in internal/cli/errors.go; tree.AliasFunc is unchanged and check looks the aliases line up through Issue.AliasesLine / Note.AliasesLine on the files its readers parsed, per the item's regression guard.

**What:** This fixes `tree-error-paths`. `tree.Error` paths are absolute: `tree.Load` joins them
onto the absolute issues or knowledge dir, and the messages "also at %s", "also an alias of %s" and
"the id of %s" name the other file absolutely. `loadTree` in `internal/check` passes line 0.
**Goal:** Every tree problem that `check`, `create`, `mv`, `know add`, `know mv` and `know rm`
print names each file as a slash path relative to the worktree root. That covers both the file at
fault and the other file in duplicate messages. A duplicate-alias problem in `check` cites the line
of the `aliases` key.

**Regression guard.** `tree.Error` exports the other file as its own field (e.g. `Other`), relativized by every consumer, so item 4's blocking rule can match it as well as `Path`.
`tree.Error.Error()` keeps rendering the other file. Keep `tree.AliasFunc` unchanged: `check`'s `loadTree` looks the aliases line up by `problem.Path` in the files its readers parsed.
Every `tree.Load` failure in internal/cli (create.go, mv.go, know.go, know_mv.go, know_rm.go) goes through one app method that relativizes `Path` and the other file with `relativeSlashPath(a.store.WorktreeRoot, …)`.

**Approach (assumed at the header base):** In `internal/tree/tree.go` (`indexAlias`, `indexIDs`),
`tree.Error` carries the other file as its own field instead of formatting it into the message. The
CLI (`asError` in `internal/cli/errors.go`) and `check` (`loadTree`/`c.relative`) relativize both
fields. The alias reader that `tree.Load` calls also returns the `aliases` key's line, which the
parsed issue and Note already know (an `AliasesLine`-style accessor).
Producers and consumers of the reader/`tree.Error` shape (grep `tree.Load(`):
- `internal/cli/create.go` `readAliases`, `mv.go`, `know.go`, `know_mv.go`, `know_rm.go`
- `internal/check/check.go`
- `internal/agents/block.go` `TopNotes`
- `internal/issue/move.go` and `internal/knowledge/move.go`, if they call it

Update each one.

**Files:** internal/tree/tree.go, internal/tree/tree_test.go, internal/check/check.go, internal/check/check_test.go, internal/cli/errors.go, internal/cli/create.go, internal/cli/mv.go, internal/cli/know.go, internal/cli/know_mv.go, internal/cli/know_rm.go, internal/cli/write_test.go, internal/cli/mv_test.go, internal/issue/issue.go, internal/knowledge/note.go
**Read first:** internal/tree/tree.go — Error, indexIDs, indexAlias, Load; internal/check/check.go — loadTree, checker.add, checker.relative, checker.issues, checker.notes; internal/cli/errors.go — asError; internal/cli/create.go — create, readAliases;
internal/cli/issue_render.go — relativeSlashPath; internal/issue/issue.go — DepsLine, PlanLine; internal/check/check_test.go — TestRun_OneFixturePerProblemClass_ReportsPathLineAndCode;
internal/tree/tree_test.go — TestLoad_DuplicateIDInTwoSubtrees_FailsNamingBoth, aliasesByFile
**Tests:**
- `check_test.go`: two issues sharing an alias, then `check` prints `.capcom/issues/b.md:<aliases line>: duplicate alias "x": also an alias of .capcom/issues/a.md`, with no absolute path. The "duplicate alias" row of `TestRun_OneFixturePerProblemClass_ReportsPathLineAndCode` moves from line 0 to line 7.
- `tree_test.go`: `TestLoad_DuplicateIDInTwoSubtrees_FailsNamingBoth` still finds both files in `Error()`.
- `internal/cli` (`mv_test.go`): `mv` of the second holder of a duplicate alias fails (items 4 and 5 keep this blocking), and its error names the first file as a relative path, with no absolute path.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/tree/ ./internal/check/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Check|Create|Mv|Know'`
- `go vet ./internal/tree/ ./internal/check/ ./internal/cli/`

**Closes:** tree-error-paths
**Commit:** `fix(tree): name files relative to the worktree and cite the aliases line`

## 4. `create` and `mv` warn about unrelated tree problems instead of failing — ✅ DONE (2026-10-11)

NOTES (2026-10-11): docs/format.md gains one sentence under "A command does not fail on a bad issue file" saying which tree problems still fail `create` and `mv` (user-facing behaviour change; the path is not in the item's Files list).

NOTES (2026-10-11): "Creating under a parent whose directory is orphaned" is tested as a parent whose file sits inside an orphan directory (`lost-dir/parent-one.md`); that parent is left out of the tree, so `create --parent` fails on resolving it (not_found), as it did before.


**What:** Depends on item 3. This fixes part of `write-verbs-bad-files`, which item 5 completes.
`docs/format.md` says "A command does not fail on a bad issue file". `create` and `mv` still exit 1
when ANY file in the tree has a structural problem (an invalid file name, an orphan directory, a
duplicate id or alias), while `list` only warns.
**Goal:** A tree problem fails `create`/`mv` only when it concerns:
- the issue being written or moved
- its destination path
- the parent the command resolves

Every other tree problem is reported as a `warning:` after the result, exactly as `list` reports
bad files, and the command exits 0.

**Regression guard.** The blocking rule matches a tree problem against the target paths through both `tree.Error.Path` and item 3's exported other-file field. A non-blocking tree problem is never printed twice: when the command's index refresh already reports the same file as a warning (format.md "A command does not fail on a bad issue file"), the tree problem is dropped; it is added as a result warning only when no index warning names that file.
An orphan-directory problem whose path is the destination's children directory (`strings.TrimSuffix(dest, ".md")`) also blocks, for `create`'s path and for `mv`'s `move.NewPath`.

**Approach (assumed at the header base):** `tree.Load` returns the partial tree together with
`tree.Errors`. The callers in `create.go` and `mv.go` discard the tree via `asError(err)`. Add one
internal/cli helper that splits `tree.Errors` into blocking and warning problems by path. The
helper is shared with item 5 and modelled on `agents.TopNotes`, which already tolerates
`tree.Errors`. It feeds the result's warnings the way `prepareIndex` feeds index warnings.

**Files:** internal/cli/create.go, internal/cli/mv.go, internal/cli/errors.go, internal/cli/write_test.go, internal/cli/mv_test.go
**Read first:** internal/cli/create.go — create, newIssuePath; internal/cli/mv.go — move, resolveParent, moveFailure; internal/cli/root.go — emit, prepareIndex; internal/agents/block.go — TopNotes;
internal/index/walk.go — treeWalker.walk; internal/index/refresh.go — excludeClaims; internal/tree/move.go — PlanMove, planRenames;
internal/cli/write_test.go — mustRun, TestCreate_UnderParent_CreatesParentDirectory
**Tests:**
- `write_test.go`: with `.capcom/issues/Bad_Name.md` present, `create --id good-one` exits 0 and writes the file, with a warning naming `.capcom/issues/Bad_Name.md` exactly once. Creating under a parent whose directory is orphaned still fails. `create --id foo-bar` beside an orphan `.capcom/issues/foo-bar/` fails.
- `mv_test.go`: `mv` beside an unrelated bad file name succeeds and warns once. With duplicate id `x.md` and `p/x.md`, `mv x y` fails.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Create|Mv'`
- `go vet ./internal/cli/`

**Commit:** `fix(cli): let create and mv warn about unrelated tree problems`

## 5. `know add`, `know mv` and `know rm` warn about unrelated tree problems

**What:** Depends on item 4. This applies item 4's rule and helper to the Knowledge verbs and
completes `write-verbs-bad-files`.
**Goal:** With an unrelated Note file that has a bad name, or an orphan directory under
`.capcom/knowledge/`, `know add`, `know mv` and `know rm` exit 0 and warn. A problem on the Note
being written, moved or removed, its destination, or its parent still fails the command.

**Regression guard.** Same no-double-warning rule as item 4 for know add / know mv / know rm.
Item 4's corrected helper applies as it stands: an orphan directory at the destination's children directory blocks, and a duplicate blocks when its `Path` or its other file matches.

**Approach (assumed at the header base):** Replace the `asError(err)` on `tree.Load` in
`know.go` (`know add`), `know_mv.go` and `know_rm.go` with item 4's helper.

**Files:** internal/cli/know.go, internal/cli/know_mv.go, internal/cli/know_rm.go, internal/cli/know_test.go, internal/cli/know_mv_test.go, internal/cli/know_rm_test.go
**Read first:** internal/cli/know.go — addNote, newNotePath, readNoteAliases, refreshAgentsAfterWrite; internal/cli/know_mv.go — moveNote, noteMoveFailure; internal/cli/know_rm.go — removeNote, checkChildrenDir;
internal/cli/know_test.go — writeNoteFiles, newKnowStore; internal/cli/know_rm_test.go — knowledgePath; internal/cli/know_mv_test.go — noteFileExists
**Tests:** one test per verb with an unrelated `Bad_Name.md` under `.capcom/knowledge/`: exit 0 and a warning naming it exactly once. One test where the target's own parent is broken: the command fails. `know add --id foo-bar` beside an orphan `.capcom/knowledge/foo-bar/` fails.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Know'`
- `go vet ./internal/cli/`

**Closes:** write-verbs-bad-files
**Commit:** `fix(cli): let the know write verbs warn about unrelated tree problems`

## 6. `check` flags git conflict markers; format.md documents Log conflicts

**What:** Depends on item 3, which also changes `internal/check/check.go`. This fixes
`check-conflict-markers`: a merge of two concurrent `capcom log` calls leaves markers in `## Log`,
and `check` exits 0.
**Goal:** `check` reports a problem with the new code `conflict` and the 1-based line for:
- every line in an issue or Note file that starts with `<<<<<<< `, `>>>>>>> ` or `||||||| `
- a line that is exactly `=======` only when it lies between a `<<<<<<<` and its `>>>>>>>`, because a bare `=======` is a valid setext heading underline

Markers inside the frontmatter yield the `conflict` problem as well as any parse failure.
`docs/format.md` lists the code. It also documents resolving a Log conflict: keep both sides'
entries in date order, delete the markers, run `capcom check`.

**Regression guard.** Skip body lines inside fenced code blocks (opened by three backticks or three tildes); always scan the frontmatter. Trim one trailing `\r` from each line before matching, so a CRLF file gives the same problems.
The CLI test runs the `git merge` with `exec.Command` and expects exit 1 (`runGit` fails the test on any non-zero exit), and the two branches log different texts, since identical entries merge cleanly.

**Approach (assumed at the header base):** Add `CodeConflict` beside the other code constants in
`internal/check/check.go`. Scan the raw bytes in the per-file read closures of `checker.issues()`
and `checker.notes()`, before parsing, so the rule fires even when the parse fails. Record each hit
with `c.add`.

**Files:** internal/check/check.go, internal/check/check_test.go, internal/cli/check_test.go, docs/format.md
**Read first:** internal/check/check.go — checker.issues, checker.notes, checker.readFile, checker.add, addParseFailure; internal/check/check_test.go — TestRun_OneFixturePerProblemClass_ReportsPathLineAndCode, issueFile, writeFiles;
internal/cli/check_test.go — TestCheck_Problems_ListedOnePerLineAndExitOne, newIssueStore; internal/cli/lock_test.go — runGit; internal/cli/testharness_test.go — runCLI, setHermeticGitEnv; docs/format.md — Log, Errors
**Tests:**
- `check_test.go`: markers in a Log body give 3 problems at the right lines, and the same 3 with CRLF line endings. A setext `=======` alone gives none. Markers inside a fenced code block in the body give none. Markers in frontmatter give `conflict` as well as `frontmatter`.
- `internal/cli/check_test.go`: a real `git merge` (run with `exec.Command`, exit 1 expected) of two branches that each ran `capcom log` with a different text on one issue makes `check` exit non-zero and print `.capcom/issues/<id>.md:<line>: ` lines.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/check/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Check'`
- `go vet ./internal/check/ ./internal/cli/`

**Closes:** check-conflict-markers
**Commit:** `feat(check): flag git conflict markers in issue and Note files`

## 7. `--kv` is a flag of the commands that support it

**What:** This fixes `kv-flag-scope`. `--kv` is a persistent root flag shown in `capcom --help`,
but `list`, `ready`, `search`, `check`, `prime`, `dep list`, `reindex`, `agents` and every `know`
subcommand reject it with exit 2.
**Goal:** `capcom --help` and `capcom list --help` do not mention `--kv`. These commands list
`--kv` in their help and emit KV records with it: `show`, `close`, `reopen`, every `lock`
subcommand, `version`, `create`, `update`, `log`, `mv`, `dep add` and `dep rm`. `capcom list --kv`
fails as an unknown flag with exit 2. `--kv --json` is still a usage error.

**Regression guard.** `--kv` before the command name is no longer accepted: `TestVersion_KV_EndsWithStatusOK` moves `--kv` after `version`, and the item's changelog entry (its sidecar) says so. `init` stays without `--kv`:
`TestInit_OutsideGit_FailsInvalid` is rewritten with `--json` (exit 1, `error.code` invalid, message naming the git repository). The `list --kv` test reads stdout's `MESSAGE=` line and
the last line `STATUS=invalid`, because `scanFormat` still routes the failure to stdout and stderr stays empty.

**Approach (assumed at the header base):** Remove the `root.PersistentFlags()` registration of
`flagKV` in `internal/cli/root.go`. Add one helper that registers it on a command and call it from
each supporting command's constructor; `a.isKV` stays the single bound variable. The
`result.KV == nil` rejection in root.go stays only as an internal-error guard. Update the `--kv`
paragraph in README.md.

**Files:** internal/cli/root.go, internal/cli/show.go, internal/cli/close.go, internal/cli/lock.go, internal/cli/create.go, internal/cli/update.go, internal/cli/log.go, internal/cli/mv.go, internal/cli/dep.go, internal/cli/read_test.go, internal/cli/cli_test.go, internal/cli/init_test.go, README.md
**Read first:** internal/cli/root.go — newRootCommand, selectRenderer, emit, scanFormat; internal/cli/cli_test.go — TestVersion_KV_EndsWithStatusOK, TestRun_UsageFailureWithKV_EndsWithErrorStatus, TestApp_Emit_ResultWithoutKVRecordsRejectsKV;
internal/cli/init_test.go — TestInit_OutsideGit_FailsInvalid; internal/cli/read_test.go — TestList_BadFlags_ExitUsage; internal/cli/update.go — writeResult, editIssue; internal/cli/close.go — newCloseCommand, newReopenCommand;
internal/cli/lock.go — newLockCommand; internal/cli/output.go — kvRenderer.RenderError
**Tests:** `read_test.go`:
- `list --kv` exits 2; stdout holds `MESSAGE=unknown flag: --kv` and ends in `STATUS=invalid`.
- `show <id> --kv` ends in `STATUS=`.
- `capcom --help` output holds no `--kv`.
- A table that runs each supporting command with `--kv` and expects a `STATUS=` line.

`cli_test.go`: `TestVersion_KV_EndsWithStatusOK` runs `version --kv`. `init_test.go`: `TestInit_OutsideGit_FailsInvalid` runs `--json init` and expects exit 1, `error.code` invalid and a message naming the git repository.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/`
- `go vet ./internal/cli/`

**Closes:** kv-flag-scope
**Commit:** `fix(cli): offer --kv only on the commands that print KV records`

## 8. Elsewhere scans remote-tracking branches

**What:** Elsewhere lists only `refs/heads/` (`branchRefPrefix` in `internal/gitx/branches.go`).
**Goal:** `gitx.Branches` lists `refs/heads/` and `refs/remotes/`, skipping `refs/remotes/*/HEAD`.
`elsewhere.Scan` skips:
- the current branch and its upstream
- a remote ref whose tip equals, or is an ancestor of, the tip of a local branch with the same short name
- a local branch whose tip is an ancestor of its same-named remote ref, so the remote ref is reported instead

It reports remote refs by their short name, for example `origin/x`. The `elsewhere.days` cutoff and
never fetching are unchanged. `docs/format.md`'s Elsewhere section says remote-tracking refs are as
fresh as the last fetch.

**Regression guard.** The docs clause targets the `elsewhere.days` bullet in `docs/format.md`'s Config section; format.md has no Elsewhere section. Update every site that `grep -rn -i 'local branch' README.md CONTEXT.md docs/adr internal/` finds where it describes the scan; this supersedes ADR 0004 line 8 ("`ready` and `prime` scan local branches and worktrees"), which is amended explicitly.
Skip the current branch's upstream only when it is under `refs/remotes/`. The upstream test commits an issue change in origin and fetches without merging; `branches_test` asserts `Branches` omits `refs/remotes/origin/HEAD`.

**Approach (assumed at the header base):** Add `refs/remotes/` and `%(objectname)` and
`%(upstream)` to the `for-each-ref` format in `gitx.Branches`. Keep the full ref in `Branch.Ref`,
since every git call uses it already. Do the de-duplication in `elsewhere.Scan` before the
`IsAncestor`/`ChangedFiles` loop.

**Files:** internal/gitx/branches.go, internal/gitx/branches_test.go, internal/elsewhere/elsewhere.go, internal/elsewhere/elsewhere_test.go, docs/format.md, docs/adr/0004-issues-are-versioned-with-the-code.md, CONTEXT.md, README.md, internal/cli/output.go, internal/cli/ready.go, internal/cli/list.go, internal/cli/show.go, internal/config/config.go
**Read first:** internal/elsewhere/elsewhere.go — Scan, scanBranch, Entry; internal/gitx/branches.go — Branches, branchFormat, Branch; internal/elsewhere/elsewhere_test.go — newRepo, onBranch, scan, TestScan_MergedOrCurrentOrOldBranch_ReportsNothing;
internal/gitx/branches_test.go — TestBranches_ListsEveryLocalBranchAndMarksTheCurrentOne, commitFile; internal/cli/output.go — addElsewhere; CONTEXT.md — Elsewhere; docs/adr/0004-issues-are-versioned-with-the-code.md
**Tests:** `elsewhere_test.go` uses a clone with an `origin` remote:
- an unmerged remote-only branch is reported as `origin/x`
- a pushed local branch is reported once
- the current branch's upstream is not reported, after origin commits an issue change and the clone fetches without merging
- a branch made with `git switch -c feat --track main` still reports local main's unmerged issue changes
- `origin/HEAD` is ignored

`branches_test.go` checks that remote refs are listed and `refs/remotes/origin/HEAD` is not.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/gitx/ ./internal/elsewhere/`
- `go vet ./internal/gitx/ ./internal/elsewhere/`

**Closes:** elsewhere-remote-refs
**Commit:** `feat(elsewhere): scan remote-tracking branches`

## 9. `capcom fmt` rewrites issues and Notes in canonical form

**What:** Agent-written files churn on their first capcom edit, because every write re-serializes
the frontmatter (for example, `plan:` is written unquoted).
**Goal:** `capcom fmt` rewrites every issue under `.capcom/issues/` and every Note under
`.capcom/knowledge/` whose bytes differ from its canonical serialization. It prints each rewritten
file as a relative path and then `formatted <n> files`. A file that fails to parse is left
untouched and reported as a `warning:` without failing the command; `capcom check` is the
validator. `capcom fmt --check` writes nothing and lists each file that would change. It exits 0
when none would, and otherwise exits with the code `capcom check` uses for problems. A second
`capcom fmt` changes nothing. `--json` gives kind `fmt` with the paths.

**Regression guard.** Run `issue.Validate` / `knowledge.Validate` before writing; a file that fails is skipped with a `warning:` like a parse failure, so the write helpers' invalid error never fires.
`tree.Errors` from `tree.Load` are warnings: format the nodes Load still placed, as `check.loadTree` does.

**Approach (assumed at the header base):**
- Add `newFmtCommand` in a new `internal/cli/fmt.go`, registered in root.go's `AddCommand` list.
- Reuse `issue.Parse`+`Issue.Serialize` and `knowledge.Parse`+`Note.Serialize`, plus the CRLF→LF and `tree.WriteFileAtomic` steps of `writeIssueFile`/`writeNoteFile`. There is one write path per file kind, so do not copy them.
- Add `fmt.go` to `internal/cli/doc.go`'s file map.
- Document the verb in README.md and in `docs/format.md`'s "Canonical form" section.

**Files:** internal/cli/fmt.go, internal/cli/fmt_test.go, internal/cli/root.go, internal/cli/doc.go, README.md, docs/format.md
**Read first:** internal/cli/update.go — writeIssueFile, absoluteIssuePath; internal/cli/know_update.go — writeNoteFile; internal/issue/file.go — Parse, Serialize; internal/knowledge/note.go — Parse, Validate, Serialize;
internal/check/check.go — loadTree, issues; internal/cli/know.go — readNoteAliases; internal/cli/root.go — AddCommand list, prepareIndex; internal/cli/doc_test.go — TestDocFileMap_MatchesDirectory
**Tests:** `fmt_test.go`:
- An issue with `plan: "x"` quoted and reordered keys, and a hand-written Note: `fmt` rewrites both, and the second run reports 0.
- `fmt --check` on a dirty store exits non-zero and changes no bytes.
- A broken file is warned about and left byte-identical.
- An issue with an undeclared status and a dotted file name (`a.b.md`) are each warned about and left byte-identical, and the other files are still formatted.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Fmt|Doc'`
- `go vet ./internal/cli/`

**Closes:** fmt-verb
**Commit:** `feat(cli): add capcom fmt to rewrite files in canonical form`

## 10. `capcom init --codex-hooks`

**What:** apogee runs Codex beside Claude Code. capcom installs hooks only for Claude
(`mergeClaudeHooks` in `internal/cli/init.go`).
**Goal:** `capcom init --codex-hooks` merges two groups into `<worktree>/.codex/hooks.json`, under
`hooks`, creating the file when missing:
- `SessionStart`, with matcher `startup|resume|clear`
- `PreCompact`, with matcher `manual|auto`

Each group holds `{"type": "command", "command": "capcom prime"}`. Other members and their order are
kept byte for byte. A re-run reports `present` and changes nothing. JSON output gains `codex_hooks`
(`added`/`present`). Text output with the flag adds one line saying that Codex runs hooks only with
`[features] hooks = true` in its `config.toml`. Without the flag, nothing about Codex is printed.
README documents the flag beside `--claude-hooks`.

**Regression guard.** Other members keep their order and values but are re-indented two spaces, matching `mergeClaudeHooks`' contract, not "byte for byte". The settings path reaches every error site (`mergeClaudeHooks`, `addHookGroup`, `settingsFailure`), so a bad `.codex/hooks.json` fails with the prefix `capcom: .codex/hooks.json`.
`--codex-hooks` without `--claude-hooks` still prints the Claude snippet in text mode exactly as plain `capcom init` does. The two flags are independent, and `--codex-hooks` changes no Claude output (user, 2026-10-11).

**Approach (assumed at the header base):** Generalize `mergeClaudeHooks` and the `claudeHookEvents`
table into one merge that takes a settings path and an event table. Claude and Codex share the JSON
shape `hooks → event → [{matcher, hooks: [{type, command}]}]`, and the merge has one reason to
change. Both flags call it. The Claude behaviour and output stay byte-identical.

**Files:** internal/cli/init.go, internal/cli/init_test.go, README.md
**Read first:** internal/cli/init.go — newInitCommand, mergeClaudeHooks, addHookGroup, settingsFailure, claudeHookEvents, claudeHooksSnippet, initData; /home/airic/repos/apogee/.codex/hooks.json;
internal/cli/init_test.go — TestInitClaudeHooks_ExistingSettings_ChangesOnlyHooksAndIsIdempotent, TestInitClaudeHooks_NoSettings_CreatesThem, TestInitClaudeHooks_UnusableSettings_FailWithPath,
TestInit_Plain_PrintsTheHooksSnippetInTextModeOnly, writeClaudeSettings
**Tests:** `init_test.go`:
- `--codex-hooks` on a fresh repo writes the exact JSON.
- On an existing `.codex/hooks.json` holding bd entries, both are kept, in order.
- An unusable `.codex/hooks.json` fails with the prefix `capcom: .codex/hooks.json`.
- `--codex-hooks` without `--claude-hooks` writes the Codex file and, in text mode, still prints the Claude snippet exactly as plain `capcom init` does.
- A re-run gives `present`.
- The existing `--claude-hooks` tests pass unchanged.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Init'`
- `go vet ./internal/cli/`

**Closes:** codex-hooks
**Commit:** `feat(init): add --codex-hooks to brief Codex at session start`

## 11. Close the gaps in the beads migration guide

**What:** Depends on items 6, 9 and 10, which the guide now references.
**Goal:** `docs/migrating-from-beads.md` covers every gap the feasibility scan found:
- step 7 uses `git config --local --get core.hooksPath` and `git config --local --unset core.hooksPath`
- `bd remember` memories become Knowledge Notes
- `.beads/interactions.jsonl`, Dolt history, `started_at`, `updated_at` and the assignee have no home, so keep the final `bd export` outside the repo
- a `notes` field holding several dated entries is split into one Log entry per date, not dated by `updated_at`; this resolves the clash with the Field mapping table's `updated_at` row
- a `spec_id` that points at a non-plan document goes to a Log entry, not `plan:`
- label-only `parked` lowers `ready` counts, because waiting issues are not ready
- `related` becomes a Log entry on both issues, and the link is now one-way prose on each side
- dotted ids are refused as slugs and survive only as aliases
- replace beads' `PreToolUse` and `SessionStart` hooks in `.claude/settings.json` and `.codex/hooks.json` with `capcom init --claude-hooks` / `--codex-hooks`
- run `capcom fmt` after writing the files
- resolve `conflict` problems as `docs/format.md` describes

**Regression guard.** The hook flags only add capcom groups and keep every existing group (`addHookGroup`), so the guide says to delete beads' hook groups by hand first (capcom has no `PreToolUse` equivalent), then run the flags.
Name the fallback date for a `notes` value with no dated entries (`closed_at`, else `created_at`) and rewrite the Log entries paragraph that dates by `updated_at`.

**Approach (assumed at the header base):** Edit in place, keeping the Steps / Field mapping
structure. Add a short "What does not migrate" subsection for the losses.

**Files:** docs/migrating-from-beads.md
**Read first:** docs/migrating-from-beads.md — Steps 7 and 8, Field mapping, Labels, Dependencies, Log entries; internal/cli/init.go — mergeClaudeHooks, addHookGroup;
internal/tree/tree.go — ValidateID, ValidateAlias; internal/issue/ready.go — Ready (only todo is ready; waiting parent blocks children)
**Tests:** none (docs only).
**Acceptance:**
- `grep -n "git config --local --get core.hooksPath" docs/migrating-from-beads.md`
- `grep -n "bd remember" docs/migrating-from-beads.md`
- `grep -n "codex-hooks" docs/migrating-from-beads.md`
- `grep -n "capcom fmt" docs/migrating-from-beads.md`

**Closes:** beads-guide-gaps
**Commit:** `docs(migration): cover the gaps found migrating apogee from beads`
