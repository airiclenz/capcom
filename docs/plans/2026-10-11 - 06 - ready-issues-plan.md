# Ready issues: warning fixes, AGENTS.md source link, rm, Log merge driver, related links

**Goal:** Close the six ready issues. Fix two write-verb warning bugs, add the capcom source link to
the Managed block, add `capcom rm`, add a git merge driver for `## Log`, and add first-class
related links (store format 2).

**Date:** 2026-10-11
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** f8e23cd

**Sources:**
- `.capcom/issues/{dup-id-warning-hidden,mv-warning-stale-path,agents-source-link,issue-rm-verb,log-merge-driver,related-links}.md`
- `docs/adr/0001-the-directory-tree-is-the-hierarchy.md`, `docs/adr/0002-ids-are-chosen-slugs-and-old-names-live-on-as-aliases.md`
- `docs/adr/0004-issues-are-versioned-with-the-code.md`, `docs/adr/0009-the-store-declares-its-format-version.md`
- `docs/format.md`, `CONTEXT.md`

**Ratified design calls (user, 2026-10-11):**
- **rm and dependents:** `capcom rm` refuses an issue other issues depend on, naming them; `--drop-deps` removes those deps entries instead. Children are refused as with `know rm`; `--if-match` as there.
- **Merge driver setup:** `capcom init --merge-driver` adds the `.gitattributes` line and prints the `git config` command for the user to run; capcom never writes `.git/config`.
- **Related storage:** `capcom related add|rm <a> <b>` stores `related: [b]` on `a` only; the index derives the reverse and `show` lists the link on both issues. `mv` rewrites it, `rm` drops it.
- **Format raise:** the first `related add` in a format-1 store writes `format: 2` into `config.yaml` and warns; `init` writes `format: 2`.

**Standing requirements:**
- skills: coding-standards
- Tests per the `cli-test-harness` Note: `GOMEMLIMIT=2GiB`, one package at a time, never `-race` on `./...`.
- Every error names the file (`cli.Error` with `Path`, Note `error-messages`).
- capcom never runs a writing git command; read-only `gitx` calls only.

**Out of scope:**
- Writing `.git/config`, git hooks, or committing.
- A merge driver for frontmatter or Note files; only the issue `## Log` section is merged by entry.
- Making related links affect `ready`, `dep` or blocking.
- Any version or release change.

**Regression check (2026-10-11, f8e23cd):**
- 1, 3, 4, 5, 7, 9, 12: guard folded
- 2: guard folded (decision: depends on item 1)
- 6: guard folded (decision: `<ours>` written byte for byte)
- 8: guard folded; yields to ADR 0009 (a missing key means format 1)
- 10: guard folded (decision: relies on item 8's keyless-config guard)
- 11: guard folded (decision: related drops fold into item 4's per-file rewrite)
- 6: recast (decision: seven-argument form, fallback labels)
- 7: recast (decision: seven-argument line, byte-keeping write)
- 1-5, 8-12: guards merged into the item text (size cut)
- 6: guard folded (re-check: label slots in git's %S %X %Y order, base first)

## 1. Write verbs report a second, different problem on an already-warned file

**What:**
**Goal:** a write verb drops a tree problem only when an index warning names the same file and
kind; any other problem there is warned once, one rule for all callers (`create`, `mv`,
`know add/mv/rm`). Fixes `dup-id-warning-hidden`.
**Approach (assumed at the header base):** `(*app).triageTreeProblems` (`internal/cli/errors.go`)
drops on `isIndexWarnedAbout(relative.Path)`. Match path and kind, never text: the tree problem by
`errors.Is` on its sentinel, the index warning by leading text (`ErrInvalidID`/`ErrInvalidAlias`/
`ErrDuplicate`/`ErrOrphanDirectory` text; walk.go's `directory left out:` is orphan). A tree
`ErrDuplicate` alias problem with `Other` == `Path` matches the index's `repeats the ... own id` /
`is listed twice`.
**Files:** internal/cli/errors.go, internal/cli/write_test.go
**Read first:** internal/cli/errors.go — triageTreeProblems, isIndexWarnedAbout; internal/tree/tree.go — indexIDs, indexAlias;
internal/index/refresh.go — excludeClaims, aliasProblems; internal/index/walk.go — treeWalker.walk;
internal/cli/write_test.go — TestCreate_UnrelatedOrphanDirectory_WritesAndWarns
**Tests:** write_test.go: `TestCreate_IndexWarnedFileWithDuplicateID_WarnsAboutBoth`: A has an
index warning (e.g. unknown status) and B's id, after B (the tree flags only the later by
(id, path)): B `dup-id.md`, A `zz-parent/dup-id.md` under a valid `zz-parent.md`; `create` of
another id exits 0, warning each once; so do
`TestCreate_IndexWarnedFileDuplicatedElsewhere_WarnsOnce` (two valid files share an id),
`TestCreate_IndexWarnedFileSelfAlias_WarnsOnce` (an alias repeating its own id; one listed twice)
and `TestCreate_UnrelatedOrphanDirectory_WritesAndWarns`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 -run 'UnrelatedBadFileName|IndexWarnedFile|OrphanDirectory' ./internal/cli/`; `go vet ./internal/cli/`
**Commit:** `fix(cli): report a second tree problem on a file the index already warned about`

## 2. mv and know mv warn with a moved child's post-move path

**What:**
**Goal:** after `capcom mv`/`know mv`, a warning about an unrelated bad file in the moved node's
children directory names its post-move path, once. Fixes `mv-warning-stale-path`.
**Approach (assumed at the header base):** `triageMoveProblems` (`internal/cli/mv.go` `move`,
`internal/cli/know_mv.go`) warns before `move.Apply()`. After `Apply` succeeds, remap every
warning's `Path` (tree problems and `emit`'s `a.indexReport.Warnings`) under the
old children dir (`strings.TrimSuffix(move.Node.Path, ".md")`) to the new
(`strings.TrimSuffix(move.NewPath, ".md")`), matching `dir+"/"` on worktree-relative slash paths
(sibling `login-flow_Bad.md` stays): paths, not messages, after item 1's pre-move dedup; one helper.
**Files:** internal/cli/mv.go, internal/cli/know_mv.go, internal/cli/mv_test.go, internal/cli/know_mv_test.go
**Read first:** internal/cli/mv.go — move, triageMoveProblems; internal/cli/know_mv.go — moveNote; internal/cli/root.go — emit;
internal/cli/errors.go — triageTreeProblems, isIndexWarnedAbout; internal/index/walk.go — treeWalker.walk;
internal/cli/mv_test.go — TestMv_UnrelatedBadFileName_MovesAndWarnsOnce
**Depends on:** item 1
**Tests:** `TestMv_BadFileInMovedChildrenDir_WarnsWithNewPath`,
`TestKnowMv_BadFileInMovedChildrenDir_WarnsWithNewPath`: a `Bad_Name.md` in the children
directory; exit 0; stderr has the old path 0 times, the new once.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 -run 'TestMv_|TestKnowMv_' ./internal/cli/`; `go vet ./internal/cli/`
**Commit:** `fix(cli): name a moved child's bad file by its path after the move`

## 3. The AGENTS.md block links to capcom's source and install instructions

**What:**
**Goal:** the Managed block gains, right after the "capcom is the issue register of record"
paragraph, a paragraph reading exactly:
`` Without a `capcom` binary, install it from https://github.com/airiclenz/capcom#install. ``
An older block reads as stale; this repo's `AGENTS.md` holds the new one; docs/format.md "## The
Managed block in AGENTS.md" names the install pointer.
**Approach (assumed at the header base):** add the line to `blockIntro` (`internal/agents/block.go`)
and `goldenBlock`. Refresh `AGENTS.md` via the built binary's `capcom agents`. The CHANGELOG
sidecar tells upgraders to run `capcom agents` once (or `capcom check` will say so).
**Files:** internal/agents/block.go, internal/agents/block_test.go, internal/cli/agents_test.go, AGENTS.md, docs/format.md
**Read first:** internal/agents/block.go — blockIntro, Render; internal/agents/block_test.go — goldenBlock, TestInspect_States,
TestInspect_TangledMarkers_FailAtTheMarkerLine; internal/cli/agents_test.go — TestAgentsCheck_StaleBlock_FailsAtTheMarkerLine;
internal/cli/check_test.go — TestCheck_StaleAgentsBlock_ReportedAtTheMarkerLine; docs/format.md — The Managed block in AGENTS.md
**Tests:** block_test.go: a pre-change block (old intro, literal) is `StateStale`; `Write` puts the
line inside the markers, every byte outside kept; `TestInspect_TangledMarkers_FailAtTheMarkerLine`'s
pins move: "second begin" 20→22, "second end" 19→21. agents_test.go: `capcom check` on the old
block reports `agents-block`, exit 1; after `capcom agents`, 0.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/agents/`; `GOMEMLIMIT=2GiB go test -count=1 -run 'Agents|StaleAgentsBlock' ./internal/cli/`; `go run ./cmd/capcom agents --check`; `grep -n 'capcom#install' AGENTS.md`
**Closes:** agents-source-link
**Commit:** `feat(agents): point the Managed block at capcom's source and install instructions`

## 4. capcom rm removes an issue

**What:**
**Goal:** `capcom rm <id|alias|prefix> [--if-match <etag>] [--drop-deps]` deletes an issue file and
its aliases. Refused (code `invalid`, file named, nothing changed): child
issues; a non-empty children directory ("the issue's children directory"); dependents (issues
whose `deps` name it or an alias; never itself), listed by id with a `--drop-deps` hint. Stale
`--if-match`: `conflict`. `--kv`/`--json` as on `mv`; kind `issue-remove`, data `id`, `path`,
`deps_dropped` (dependents' ids).
**Approach (assumed at the header base):** model on `removeNote` (`internal/cli/know_rm.go`):
`tree.Load` + `treeProblems`, `resolveNode`, `triageTreeProblems`, `checkChildrenDir` (gains a
`note`/`issue` noun; know_rm_test.go pins only the path prefix), `removeEmptyParentDir`,
`edit.ETag`. Dependents: `loadDepGraph` + `DepGraph.Dependents` (`internal/issue/deps.go`) minus
`node.ID`. The index lacks unparseable files: rm parses every tree node (as
`planRewrites`) just to warn `deps not checked` on unreadable ones (named twice, as on `mv`).
Refuse and plan every rewrite (`Original`+`Content`) before writing; delete last; restore the
originals on failure (as `tree`'s `rollBack`). No AGENTS.md refresh. `newRmCommand` in `root.go`
under `a.withKV`, listed in `internal/cli/doc.go`. Docs: README usage block, command list;
docs/format.md "## Issue files" paragraph, and the verb in "### Errors"'s tree-problem verb list.
**Files:** internal/cli/rm.go, internal/cli/rm_test.go, internal/cli/root.go, internal/cli/doc.go, internal/cli/know_rm.go, README.md, docs/format.md
**Read first:** internal/cli/know_rm.go — removeNote, checkChildrenDir, removeEmptyParentDir; internal/cli/dep.go — loadDepGraph;
internal/issue/deps.go — DepGraph.Dependents; internal/cli/update.go — writeIssueFile; internal/cli/errors.go — triageTreeProblems;
internal/cli/mv.go — resolveNode
**Tests:** rm_test.go: leaf removed, `check` passes; last child takes the empty dir; children
refused; a dependent refused by name, file kept; `--drop-deps` (also via an alias); stale
`--if-match`; an unrelated `Bad_Name.md` warns once; `--json` kind and data; a stray
children-directory file refused (Goal's wording); an issue whose own `deps` name it is removed.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 -run 'TestRm_' ./internal/cli/`; `go vet ./internal/cli/`; `grep -n 'capcom rm' README.md docs/format.md`
**Closes:** issue-rm-verb
**Commit:** `feat(cli): add capcom rm to remove an issue`

## 5. Merge the Log sections of three versions of an issue file

**What:**
**Goal:** a pure `internal/issue` function merges base, ours and theirs bytes of an issue file into
bytes and a conflict flag. `## Log` merges by entry: both sides' entries minus any
in base deleted on one side, never reordering existing ones: if one side's Log equals base, take
the other's verbatim; else keep ours's order, merging theirs's new entries in by date, stable. The
rest goes to a caller-supplied 3-way text merge
`func(base, ours, theirs []byte) ([]byte, bool, error)`.
**Approach (assumed at the header base):** an entry is a `- YYYY-MM-DD <author>: ` line plus its
2-space continuation lines (`logContinuationIndent`), blank-then-indented lines included; any other
non-entry Log line on any side (`### `, prose; trailing blanks aside) sends the whole file to the
text merge. Bounds: `LogHeading`, `isSectionBoundary` (`internal/issue/mutate.go`). Swap each Log
for a fixed alphanumeric placeholder line (git's ZEALOUS_ALNUM), merge, replace every
occurrence with the merged Log (a nearby conflict hunk holds it twice). One side's Log only: use
it; none: the text merge as is. CRLF keeps CRLF.
**Files:** internal/issue/merge.go, internal/issue/merge_test.go
**Read first:** internal/issue/mutate.go — LogHeading, logContinuationIndent, isSectionBoundary, appendLogEntry, AppendLog;
internal/issue/file.go — Parse; internal/edit/edit.go — NormalizeNewlines; docs/format.md — Merge conflicts in the Log
**Tests:** table: both append (clean, both kept, date order); one appends, one edits the title
(clean, fake text merge); both edit the title (conflict, Log merged); multi-line entries; an entry
deleted on one side; a Log not last; CRLF; out-of-order base entries kept when only ours appends;
a blank line in an entry; a non-entry line falls back; a hunk spanning the placeholder.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 -run Merge ./internal/issue/`; `go vet ./internal/issue/`
**Commit:** `feat(issue): merge three versions of an issue's Log by entry`

## 6. capcom merge-driver: the git merge driver command

**What:** Recast at the regression check (2026-10-11).
**Goal:** `capcom merge-driver <base> <ours> <theirs> <path> [<base-label> <ours-label> <theirs-label>]`
(git's `%O %A %B %P %S %X %Y`: %S is the ancestor label) writes the merged issue file to `<ours>`, exits 0 when clean and 1
when conflict markers remain. Outside the Log it merges with `git merge-file -p`, markers labelled
with the given labels or, when they are absent or arrive unexpanded (literally `%S`, `%X` or `%Y`,
as git before 2.44 passes them), with `<path> (ours)`, `<path> (base)`, `<path> (theirs)`. It
works outside a capcom store.
**Approach (assumed at the header base):** read-only `gitx.MergeFile(base, ours, theirs paths,
labels) ([]byte, bool, error)` runs its own `exec.Command` (`gitx.Run` drops stdout on non-zero
exit): `-p`, `-L` labels, `GIT_OPTIONAL_LOCKS=0`; `-L` takes ours, base, theirs (arg 6, 5, 7; merge-file's
`<current> <base> <other>` order). Exit 1..127 is a conflict; any other non-zero
exit (git fails with 255) a failure, writing nothing. Item 5's text-merge adapter writes the
Log-cut versions to temp files, calls `MergeFile`, removes them. `<ours>` is written byte for byte
(temp file plus rename), never via `tree.WriteFileAtomic` or any newline-normalizing writer: CRLF
stays CRLF. In `internal/cli/merge_driver.go`, registered in `root.go`, listed in `doc.go`; no
store or index setup (no `annotationIndex` refresh).
**Regression guard.** Item 7's driver line and test use this form. Label args keep git's order
(5 base, 6 ours, 7 theirs), never the merge-file order.
**Files:** internal/gitx/merge.go, internal/gitx/merge_test.go, internal/cli/merge_driver.go, internal/cli/merge_driver_test.go, internal/cli/root.go, internal/cli/doc.go
**Read first:** internal/gitx/gitx.go — Run, optionalLocksOff; internal/cli/root.go — newRootCommand;
internal/cli/errors.go — Error.ExitCode, CodeConflict; internal/cli/doc_test.go — TestDocFileMap_MatchesDirectory;
internal/cli/testharness_test.go — runCLI; internal/gitx/gitx_test.go — newRepo
**Depends on:** item 5
**Tests:** gitx: clean and conflicting merges; a missing input fails, not conflicts. cli: both
append → exit 0, `<ours>` holds both; conflicting title edits → exit 1, `<<<<<<< ` carries arg 6
and `>>>>>>> ` arg 7 (distinct labels); fallback labels when absent and when literal `%S %X %Y`; CRLF stays CRLF; run
outside any `.capcom/`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/gitx/`; `GOMEMLIMIT=2GiB go test -count=1 -run MergeDriver ./internal/cli/`; `go vet ./internal/gitx/ ./internal/cli/`
**Commit:** `feat(cli): add capcom merge-driver for issue Log sections`

## 7. capcom init --merge-driver turns the driver on, and the docs describe it

**What:** Recast at the regression check (2026-10-11).
**Goal:** `capcom init --merge-driver` adds the line `.capcom/issues/**/*.md merge=capcom-log` to
the worktree root's `.gitattributes` (created if missing, never duplicated, the file's bytes and
line ending kept: a byte-keeping replace as `internal/agents/block.go`'s `replaceFile`) and prints,
for the user to run:
`git config merge.capcom-log.name "capcom Log merge"` and
`git config merge.capcom-log.driver "capcom merge-driver %O %A %B %P %S %X %Y"`. On an existing
store it behaves as `--claude-hooks` does. docs/format.md "### Merge conflicts in the Log" and
README describe the setup and the fallback without the config (git's normal merge, manual
resolution).
**Approach (assumed at the header base):** extend `internal/cli/init.go` beside
`installHooks`/`mergeHooks`, keeping the mode; never `tree.WriteFileAtomic` (CRLF to LF, mode
reset). `.gitattributes` is at the worktree top (`gitx.Discover`), `.capcom` relative.
**Files:** internal/cli/init.go, internal/cli/init_test.go, docs/format.md, README.md, .gitattributes
**Read first:** internal/cli/init.go — newInitCommand, installHooks; internal/agents/block.go — replaceFile;
internal/store/store.go — Store.WorktreeRoot; internal/cli/init_test.go — newGitRepo, TestInit_Plain_PrintsTheHooksSnippetInTextModeOnly;
docs/format.md — Merge conflicts in the Log; .gitattributes — `* text=auto eol=lf`
**Depends on:** item 6
**Tests:** init_test.go: a fresh `.gitattributes` gets the line; an existing one keeps its bytes,
gaining it once over two runs; stdout has both `git config` lines as in the Goal; CRLF stays CRLF,
gaining a CRLF-ended line.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 -run Init ./internal/cli/`; `grep -n 'merge=capcom-log' .gitattributes docs/format.md README.md`
**Closes:** log-merge-driver
**Commit:** `feat(init): add --merge-driver to turn on the Log merge driver`

## 8. Store format 2 and the related key in issue files

**What:**
**Goal:** `config.FormatVersion` is 2 (a keyless config still reads 1). Issue files may carry
`related: [<id>, ...]` (`Issue.Related`, written after `deps`). `capcom init` writes a real
`format: 2`. A new `internal/config` function raises `config.yaml` to a given format, every other
byte kept: it replaces an existing `format:` line, real or commented (with the template comment
line above a commented one), else inserts `format: N` first (never a second key: eachKey refuses
duplicates).
**Approach (assumed at the header base):** `keyDeps`/`p.list`/`writeList` (`internal/issue/issue.go`,
`file.go`) model `keyRelated`. In `internal/config/config.go` (template, `FormatVersion`) Parse sets
`Format` to 1 when no key is found; `TestParse_EmptyDocument_ReturnsDefault` then expects 1.
read_test.go: `writeNewerFormatConfig` writes `format: 3`; `newerFormatMessage` pins
`store format 3 ... (up to 2)`.
**Regression guard.** Yields to ADR 0009 and CONTEXT.md: a missing key means format 1.
**Files:** internal/config/config.go, internal/config/config_test.go, internal/issue/issue.go, internal/issue/file.go, internal/issue/file_test.go, internal/cli/init_test.go, internal/cli/read_test.go
**Read first:** internal/config/config.go — Default, parser.format, DefaultFile, FormatVersion, parser.eachKey;
internal/config/config_test.go — TestParse_Format_MissingOrOneLoads; internal/issue/file.go — writeList;
internal/cli/read_test.go — writeNewerFormatConfig
**Tests:** config: format 2 reads; 3 refused naming the line; raise on the old template, on one
without the comment, at 2 (no change), with `format: 1` (one `format: 2` left); keyless reads 1.
issue: `related` round-trips in canonical order; an empty list omitted as `deps`. init:
`format: 2`. cli: NewerStoreFormat tests pass.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/config/ ./internal/issue/`; `GOMEMLIMIT=2GiB go test -count=1 -run 'Init|NewerStoreFormat' ./internal/cli/`
**Commit:** `feat(issue): add the related key and raise the store format to 2`

## 9. The index and check know related links

**What:**
**Goal:** the index stores issues' `related` entries; `index.Entry` exposes them and the issues
naming it (`Related`, `RelatedBy`). `capcom check` reports a related entry resolving to no issue
(code `missing-related`, at the `related:` line) and one naming the issue itself (`CodeField`;
check has no `invalid`).
**Approach (assumed at the header base):** mirror the `deps` table (`internal/index/schema.go`,
`rowTables`, insert in `refresh.go`, read in `index.go`); bump `schemaVersion`. `RelatedBy`
matches `to_id` against the id and every alias, joins `issues` on `excluded = 0`, and is its own
`Index` query, not in `Index.Issue`. Add `RelatedLine()` beside `DepsLine()` in
`internal/issue/issue.go`; `checkIssue` (`internal/check/check.go`) reports `CodeMissingDep` there.
**Files:** internal/index/schema.go, internal/index/refresh.go, internal/index/index.go, internal/index/index_test.go, internal/check/check.go, internal/check/check_test.go, internal/issue/issue.go
**Read first:** internal/index/schema.go — schemaVersion, rowTables; internal/index/refresh.go — refresher.insertIssue;
internal/index/index.go — Entry, Index.Issue; internal/index/query.go — Index.resolveID;
internal/check/check.go — checker.checkIssue, the Code constants
**Depends on:** item 8
**Tests:** index: `Related`/`RelatedBy` filled after refresh and an edit; a schema bump rebuilds;
`RelatedBy` finds a holder naming an alias, skips a duplicate-excluded file's rows. check: missing
and self entries at the line (self `field`); an alias resolves.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/ ./internal/check/`
**Commit:** `feat(index): index related links and check them`

## 10. capcom related add|rm, and show lists links on both issues

**What:**
**Goal:** `capcom related add <a> <b>` writes `b` into `a`'s `related` (refused if `a == b`; no-op
if linked either way); `capcom related rm <a> <b>` removes it from the side holding it
(`not_found` if neither). `--kv` as `dep add`, `--if-match` as `update`'s `flagIfMatch` against
the file written (`a`; the holder for rm); `--json` kind `issue-write`. `capcom show`
prints `related: [...]` and JSON `related`: own entries in file order, then the issues naming it,
sorted, deduplicated. The first `related add` in a format-1 store (keyless too, per item 8) raises
it to 2 with item 8's function, warning:
`warning: .capcom/config.yaml: store format raised to 2; capcom releases that read only format 1 can no longer read this store`.
**Approach (assumed at the header base):** model on `newDepAddCommand`/`newDepRmCommand`/
`resolveEdge` (`internal/cli/dep.go`), through `a.editIssue`. `related` is only in show's
data (beside `ETag`/`Body` in `showResult`), not the shared `issueData` (list, search, ready,
dep list); only show reads `RelatedBy`. Text in `issueText` (`internal/cli/issue_render.go`).
**Files:** internal/cli/related.go, internal/cli/related_test.go, internal/cli/root.go, internal/cli/doc.go, internal/cli/issue_render.go, internal/cli/read_test.go
**Read first:** internal/cli/dep.go — newDepAddCommand, resolveEdge; internal/cli/update.go — editIssue, flagIfMatch;
internal/cli/issue_render.go — issueData, newIssueData; internal/cli/show.go — showResult;
internal/cli/reindex_test.go — newIssueStore
**Depends on:** item 9
**Tests:** add/rm round-trip; rm from the other side; self refused; duplicate no-op; `show` lists
it on both; raise and warning once (`newIssueStore`'s `init` writes `format: 2`: overwrite
`.capcom/config.yaml` keyless, `related add` twice, `format: 2` in the file); `--json` kind; stale
`--if-match` on add (`a`) and rm (the holder); `list --json` has no `related`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 -run 'Related|TestShow' ./internal/cli/`; `go vet ./internal/cli/`
**Commit:** `feat(cli): add capcom related add and rm`

## 11. mv rewrites related links and rm drops them

**What:**
**Goal:** a renaming `capcom mv` rewrites other issues' `related` entries naming the old id or an
alias, as `deps`. `capcom rm` drops other issues' `related` entries naming it, without
refusing, their ids in `--json` data `related_dropped`.
**Approach (assumed at the header base):** `planRewrites`/`renameDeps` (`internal/issue/move.go`);
`planRewrite` skips a file unless Deps or Aliases changed: add `Related`. In `internal/cli/rm.go`
the drops fold into item 4's per-file pre-write rewrite: a file holding a dep and a related entry
on the removed issue is rewritten once; all refusals precede any write.
**Files:** internal/issue/move.go, internal/issue/move_test.go, internal/cli/rm.go, internal/cli/rm_test.go, internal/cli/mv_test.go
**Read first:** internal/issue/move.go — planRewrite, renameDeps, planRewrites; internal/issue/move_test.go — newMoveTree,
TestPlanMove_Rename_RewritesDepsInOtherFiles; internal/cli/mv_test.go — newMoveStore, TestMv_Rename_OldIDResolvesAndDepsFollow;
internal/cli/dep.go — loadDepGraph
**Depends on:** items 4 and 10
**Tests:** `PlanMove` rename rewrites a file with only `related` naming the old id or an alias (no
deps); `mv`: `show` lists it on both; `rm` drops it, `check` passes; `rm --json` has
`related_dropped`; `rm` with a dependent and a related holder, no `--drop-deps`, refuses, holder
byte for byte; with `--drop-deps`, a file holding both is rewritten once.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 -run PlanMove ./internal/issue/`; `GOMEMLIMIT=2GiB go test -count=1 -run 'TestMv_|TestRm_' ./internal/cli/`
**Commit:** `feat(cli): keep related links through mv and rm`

## 12. Document related links and store format 2

**What:**
**Goal:** docs/format.md documents `related` (Fields table), `related add|rm`, `show` output, format
2 and the raise; README's command list covers `related`; docs/migrating-from-beads.md maps
beads `related` to `capcom related add`, dropping "capcom has no such link"; CONTEXT.md defines
**Related link**. Also update every sentence saying capcom has no related link or about format 1
or the commented key (e.g. format.md's "writes the key only as a comment while the format is 1"),
every write or `--kv` command list (README's output paragraph gains `related add`, `related rm`),
every passage mapping beads `related` to a Log entry (`beads: related to ...`, "Related and
discovered-from links use the date ..."): `grep -rn -i -e 'related' -e 'format' -e 'dep rm'
README.md CONTEXT.md docs/format.md docs/migrating-from-beads.md docs/adr`; ADRs stay.
**Approach (assumed at the header base):** the CHANGELOG sidecar/closeout amends the [Unreleased]
line ending "`capcom init` writes the key only as a `# format: 1` comment" (init writes `format: 2`).
**Files:** docs/format.md, README.md, docs/migrating-from-beads.md, CONTEXT.md
**Read first:** docs/format.md — Fields table, the **format** bullet; README.md — output paragraph (--kv list), command block;
docs/migrating-from-beads.md — dependency-type table, Log entries section; CONTEXT.md — Format version; CHANGELOG.md — [Unreleased]
**Depends on:** items 10 and 11
**Tests:** none (docs only).
**Acceptance:** `grep -n 'related add' README.md docs/format.md docs/migrating-from-beads.md`; `grep -n 'Related link' CONTEXT.md`; `! grep -n 'capcom has no such link' docs/migrating-from-beads.md`
**Closes:** related-links
**Commit:** `docs: describe related links and store format 2`
