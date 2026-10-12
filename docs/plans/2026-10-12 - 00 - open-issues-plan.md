# Open issues plan

**Goal:** Resolve the three open capcom issues: `create`/`mv` (and `know add`/`know mv`) refuse an
id that an unparseable file may claim as an alias, the parse warning names a conflict marker, the
`--kv` warning order is documented and pinned, and the beads migration guide settles the trial's
open cases.
**Date:** 2026-10-12
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 609a0ee

**Sources:**
- `.capcom/issues/create-alias-unparseable.md`, `.capcom/issues/kv-warning-order.md`,
  `.capcom/issues/beads-guide-trial-gaps.md`
- `docs/format.md` (`### Aliases`, `### Errors`), `docs/migrating-from-beads.md`, `CONTEXT.md`
- Knowledge Notes `cli-test-harness`, `coding-conventions`

**Ratified design calls** (user, 2026-10-12):
- **Unparseable alias holder:** recover the `aliases:` list from the raw frontmatter text when
  parsing fails and refuse an id found there, naming the file; unrelated broken files stay warnings.
- **Knowledge:** `know add` / `know mv` get the same fix, in their own item.
- **`--kv` warning order:** keep `WARNING=` lines before the records so `STATUS=` stays last;
  document it and pin it with tests. No renderer change.
- **Beads notes dates:** only a date at the start of a line starts an entry; the leading date and a
  directly following `:` are removed, the rest of the text kept verbatim.

**Standing requirements:**
- skills: coding-standards
- CLI tests go through `runCLI` / `runCLIWith`, never `t.Parallel()` (Note `cli-test-harness`).
- Tests on this host: one package at a time with `GOMEMLIMIT=2GiB`; never `./...` with `-race`.

**Out of scope:**
- Any change to `kvRenderer.Render` or the plain-text renderer.
- A beads importer (ADR 0006: docs only).
- Version, tag or CHANGELOG release-heading changes.

**Regression check (2026-10-12, 609a0ee):**
- 1: guard folded
- 2: guard folded; supersedes `docs/format.md:202-203` ("markers there are reported beside the frontmatter problem they cause")
- 3: recast (decision: recovered aliases stay out of `tree.Load`, a side map refuses only the new id; qualifies `docs/format.md:180`)
- 4: recast (decision: the same side map for Notes, lowercase `note` message); yields to `docs/format.md:184`
- 5: guard folded
- 6: guard folded
- 3 (re-check): Approach recast to state the side-map design (decision: matches its guard; `orphanedIDFailure` takes the kind noun); yields to `CHANGELOG.md:13` (0.2.0)
- 4 (re-check): Approach recast to state the side-map design (decision: `readNoteAliases` unchanged, `"note"` passed to `orphanedIDFailure`, no `errors.go` edit, no Closes: line)

## 1. Shared raw-frontmatter package with the conflict-marker scan — ✅ DONE (2026-10-12)

NOTES (2026-10-12): consequential edit — internal/check/check.go package doc: no longer claims check finds the conflict markers itself; it now names the rawfm scan among what it collects (made necessary by moving the scan into internal/rawfm).
NOTES (2026-10-12): rawfm API is `ConflictMarker{Line, Text}`, `ConflictMarkers([]byte)` and `ConflictMessage(ConflictMarker) string`; the rawfm unit tests also cover a closed fence followed by markers, an unclosed frontmatter (fences not honoured there) and a file without frontmatter.

**What:**
**Goal:** a leaf package `internal/rawfm` (importing no capcom package) exports the conflict-marker
scan and the message text `capcom check` reports for `conflict`; `internal/check` uses it and its
`conflict` findings are byte-for-byte unchanged.
**Approach (assumed at the header base):** move `conflictMarkers`, `conflictMarker`,
`bodyStartIndex`, `codeFenceOpener`, `closesCodeFence` and the `conflictOpen`/`conflictClose`/
`conflictBase`/`conflictSeparator` constants from `internal/check/check.go` into
`internal/rawfm/conflict.go` as exported API (`rawfm.ConflictMarkers`, `rawfm.ConflictMarker`).
Export the human message `(*checker).checkConflictMarkers` builds as a function
(`rawfm.ConflictMessage(m ConflictMarker) string` or similar) so item 2 reuses the same words.
`checkConflictMarkers` keeps `CodeConflict` and calls the package. Move the unit tests of the pure
helpers with them.
**Regression guard.** No unit tests of the pure helpers exist to move (the scan is covered only through `check.Run`): write new `rawfm` unit tests from the cases in `docs/format.md` `### Merge conflicts in the Log` (Log, CRLF, diff3, setext underline, both fences, frontmatter). Move `trimFenceIndent` and the `backtickFence`/`tildeFence`/`maxFenceIndent` constants too. Add an `internal/check` test pinning one conflict Problem's full Message, and check the leaf-package rule with `go list -deps`.
**Files:** internal/rawfm/conflict.go, internal/rawfm/conflict_test.go, internal/check/check.go, internal/check/check_test.go
**Read first:** internal/check/check.go — conflictMarkers, checkConflictMarkers, bodyStartIndex, codeFenceOpener, trimFenceIndent;
internal/check/check_test.go — TestRun_ConflictMarkers_ReportedAtEachMarkerLine, locations; docs/format.md — "### Merge conflicts in the Log"
**Tests:** new unit tests of the marker scan pass in `internal/rawfm` (Log, CRLF, diff3, setext
underline, both fences, frontmatter); a new `internal/check` test pins one conflict Problem's full
Message; existing `internal/check` conflict tests pass unchanged.
**Acceptance:**
- `go vet ./internal/rawfm/ ./internal/check/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/rawfm/ ./internal/check/`
- `go list -deps ./internal/rawfm/ | grep 'capcom/internal/'` (lists only `internal/rawfm`)
**Commit:** `refactor(rawfm): share the conflict-marker scan outside check`

## 2. Parse warning names a conflict marker in the frontmatter — ✅ DONE (2026-10-12)

NOTES (2026-10-12): the conflict blame covers every Parse failure after the frontmatter is split (the YAML decode and the key/value decode in `fields`), not only the YAML error path, so a marker is named whichever stage it breaks; unclosed-frontmatter and missing-fence errors keep their wording.
NOTES (2026-10-12): `TestRun_ConflictMarkersInFrontmatter_ReportedBesideTheParseFailure` in internal/check/check_test.go asserted the duplicate frontmatter problem the plan's guard drops; it is replaced by `TestRun_ConflictMarkersInFrontmatter_ReportedOnceWithoutAFrontmatterProblem`, which pins the full problem list and text.

**What:** part of `.capcom/issues/create-alias-unparseable.md`: the `create` warning names only
the YAML error when the frontmatter holds a git conflict marker.
**Goal:** when an issue file's or Note's frontmatter fails to parse and holds a conflict marker,
the parse error (and so every command's warning for that file) names the marker's line with the
same message text `capcom check` reports for `conflict`; other YAML errors keep their wording.
**Approach (assumed at the header base):** in `internal/issue/file.go` (`parser.yamlError` or the
caller that produces "invalid YAML frontmatter: …") and its copy in `internal/knowledge/note.go`,
run `rawfm.ConflictMarkers` on the raw data when YAML decoding fails and, on a marker inside the
frontmatter, return an error built from `rawfm.ConflictMessage` instead. The warning reaches
`create` through `index/refresh.go` `parseIssue` unchanged.
**Regression guard.** `addParseFailure` (`internal/check/check.go`) copies the parse error's Message, so `capcom check` would print the conflict line twice: drop the frontmatter Problem when the parse error is the conflict one (or keep a distinct frontmatter wording), and pin check's text in a test. Supersedes `docs/format.md:202-203` ("markers there are reported beside the frontmatter problem they cause"): reword it. The Note parse tests live in `internal/knowledge/knowledge_test.go`; `note_test.go` does not exist.
**Files:** internal/issue/file.go, internal/issue/file_test.go, internal/knowledge/note.go, internal/knowledge/knowledge_test.go, internal/check/check.go, internal/check/check_test.go, internal/cli/write_test.go, docs/format.md
**Read first:** internal/issue/file.go — parser.yamlError, splitFrontmatter; internal/knowledge/note.go — parser.yamlError;
internal/check/check.go — addParseFailure, checkConflictMarkers; internal/issue/file_test.go — TestParse_YAMLSyntaxError_CitesFileLine;
internal/knowledge/knowledge_test.go — TestParse_BadFiles_FailCitingTheLine; internal/cli/write_test.go — TestCreate_UnrelatedBadFileName_WritesAndWarnsOnce
**Tests:** unit tests in `internal/issue` (`file_test.go`) and `internal/knowledge`
(`knowledge_test.go`) for a frontmatter with a `<<<<<<<` line before `title:`; an `internal/check`
test pinning that such a file reports the conflict text once; a CLI test in
`internal/cli/write_test.go` running `create` beside such a file and asserting the `warning:` line
carries the conflict wording, not the YAML error.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/issue/ ./internal/knowledge/ ./internal/check/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'TestCreate'`
**Commit:** `fix(issue): name a conflict marker in the frontmatter parse warning`
Depends on item 1.

## 3. create and mv refuse an alias held by an unparseable issue file — ✅ DONE (2026-10-12)

NOTES (2026-10-12): re-derived from the assumed side map with orphanedIDFailure taking the kind noun: the new kind-neutral `unparsedAliasFailure(nodes, problems, id, kind, recovered)` in internal/cli/errors.go searches for the one new id instead of building a deduplicated alias map (only one id is ever checked), covering the tree's nodes and, through the unchanged `findNodeFile` with the recovering lookup `recoverAliases` (create.go), files inside orphan directories; `orphanedIDFailure`, `triageMoveProblems`, `readAliases` and every know call site are unchanged, so item 4 adds one `unparsedAliasFailure(..., noteNoun, <note recovering lookup>)` call per command.
NOTES (2026-10-12): `mv` checks after `PlanMove` and `triageMoveProblems` (still before `Apply`) rather than before `PlanMove`, so a definite "already taken / already an alias" failure wins over the possible one, and `PlanMove` has already refused a moved file that does not parse, so the holder found is always another issue; `create` checks right after `orphanedIDFailure` for the same reason.
NOTES (2026-10-12): consequential edit — internal/rawfm/conflict.go: made necessary by adding rawfm.Aliases (the package doc named only the conflict markers).

**What:** Recast at the regression check (2026-10-12). bug `.capcom/issues/create-alias-unparseable.md`: `readAliases` in
`internal/cli/create.go` returns no aliases for a file that does not parse, so `create` and `mv`
reuse an id that file records as an alias.
**Goal:** `create` and `mv` refuse a new id that an unparseable issue file lists under `aliases:`
(flow `[a, b]` or block `- a` form, read from the raw frontmatter text), failing with `CodeExists`
and the message `id "<id>" may be an alias of issue <holder>, whose file does not parse: fix it
first`; the same holds for files inside a directory without its node file (`orphanedIDFailure`).
A new id that no broken file lists still writes, with the broken file reported as a warning.
**Approach (assumed at the header base):** add `rawfm.Aliases(data []byte) []string`, a lenient
line scan of the frontmatter block for the `aliases:` key that tolerates conflict markers and other
broken lines. `readAliases` stays unchanged and recovered aliases never enter `tree.Load`: a side
map holds the recovered aliases (deduplicated) of the issue files that do not parse, with their
holders, and is consulted only for the new id: after `Lookup` misses in `create`, before
`PlanMove` in `mv`, and in `findNodeFile` for the orphan-directory case. A hit fails with the
message above instead of `existsFailure`'s "is already an alias". `orphanedIDFailure` takes the
kind noun (`"issue"` / `"note"`) as an argument, so item 4 needs no `internal/cli/errors.go` edit.
`rm.go` and `fmt.go` keep their current alias behaviour. In `docs/format.md` `### Errors`, after
the sentence on directories without their node file, add: a new id is also refused when an issue file that does not parse records it among
its aliases, as far as its `aliases:` line can still be read.
**Regression guard.** Recovered aliases never enter tree.Load — tree.Load, Resolve, resolveParent, rm.go and fmt.go keep today's readAliases (nil for a broken file), so no new duplicate/invalid-alias tree problems, warnings or resolutions appear. Instead a separate function collects rawfm.Aliases (deduplicated) of the issue files that do not parse into a side map consulted only for the new id: after Lookup misses in create, before PlanMove in mv, and in findNodeFile for the orphan-directory case. Keep the signatures of orphanedIDFailure and triageMoveProblems kind-neutral (pass the recovering lookup through an argument or option, never hard-code "issue") so know add / know mv keep compiling and their wording is unchanged; add `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'TestKnow'` to Acceptance. Add create and mv tests with an unparseable holder inside a directory without its node file, beside TestCreate_AliasInsideOrphanDirectory_FailsNamingTheFile / TestMv_OntoAliasInsideOrphanDirectory_FailsNamingTheFile; unit-test rawfm.Aliases with an `aliases:` line on both sides of a conflict hunk. In docs/format.md qualify "A command does not fail on a bad issue file." so it does not contradict the new refusal sentence. The item yields to `CHANGELOG.md:13` (0.2.0: `create`/`mv` "fail only when the problem concerns the issue being written or moved"): the refusal concerns only the new id being written or moved, and every other problem of the broken file stays a warning.
**Files:** internal/rawfm/aliases.go, internal/rawfm/aliases_test.go, internal/cli/create.go, internal/cli/mv.go, internal/cli/errors.go, internal/cli/write_test.go, internal/cli/mv_test.go, docs/format.md
**Read first:** internal/cli/create.go — create, readAliases, existsFailure; internal/cli/errors.go — orphanedIDFailure, findNodeFile;
internal/cli/mv.go — move, triageMoveProblems; internal/issue/move.go — PlanMove
**Tests:** `rawfm.Aliases` unit tests (flow, block, conflict marker before `title:`, no key, an
`aliases:` line on both sides of a conflict hunk deduplicated); CLI tests replaying the issue's
repro for `create` and for `mv` onto the alias; `create` and `mv` tests with an unparseable holder
inside a directory without its node file, beside
`TestCreate_AliasInsideOrphanDirectory_FailsNamingTheFile` /
`TestMv_OntoAliasInsideOrphanDirectory_FailsNamingTheFile`; an unrelated broken file still lets
`create` write and warns once; the `TestKnow` tests stay green unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/rawfm/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'TestCreate|TestMv'`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'TestKnow'`
**Closes:** create-alias-unparseable
**Commit:** `fix(cli): refuse an id an unparseable issue file holds as an alias`
Depends on items 1 and 2.

## 4. know add and know mv refuse an alias held by an unparseable Note — ✅ DONE (2026-10-12)

NOTES (2026-10-12): re-derived from the assumed `orphanedIDFailure` kind-noun argument: item 3 landed the kind-neutral `unparsedAliasFailure(nodes, problems, id, kind, recovered)` instead, so `addNote` and `moveNote` each call it with `noteNoun` and the new recovering lookup `recoverNoteAliases` (know.go, beside the unchanged `readNoteAliases`); `orphanedIDFailure` and `errors.go` are untouched. `know mv` checks after `triageMoveProblems` and before the etag check, mirroring item 3's `mv`.

**What:** Recast at the regression check (2026-10-12).
**Goal:** `know add` and `know mv` refuse a new id that an unparseable Note lists under `aliases:`,
with the same rule, failure code and message shape as item 3 (lowercase `note` in place of `issue`); an id no
broken Note lists still writes with a warning.
**Approach (assumed at the header base):** item 3's side-map design, for Notes: `readNoteAliases`
in `internal/cli/know.go` stays unchanged, and the recovered aliases (`rawfm.Aliases`) of the Notes
that do not parse are consulted only to refuse the new id in `know add` and `know mv`. `addNote`
passes `"note"` to `orphanedIDFailure`, so `internal/cli/errors.go` is not edited. In
`docs/format.md` `### Errors`, widen item 3's sentence from "an issue file" to "an issue file or
Note".
**Regression guard.** Same side-map design as item 3: readNoteAliases stays unchanged (and so know rm, fmt and Note resolution are unchanged); recovered Note aliases are consulted only to refuse the new id in know add and know mv. Message is lowercase: `id "<id>" may be an alias of note <holder>, whose file does not parse: fix it first`. Add internal/cli/know_mv.go (and the test file holding TestKnowMv_OntoTakenIDOrAlias_FailsExistsNamingTheNote, if different from know_test.go) to Files; know_rm.go and fmt.go are untouched. The item yields to `docs/format.md:184` (know add/mv/rm treat the Knowledge tree like create/mv/rm): know rm stays unchanged, as rm does in item 3.
**Files:** internal/cli/know.go, internal/cli/know_mv.go, internal/cli/know_test.go, internal/cli/know_mv_test.go, docs/format.md
**Read first:** internal/cli/know.go — addNote, readNoteAliases, noteExistsFailure; internal/cli/know_mv.go — moveNote, noteMoveFailure;
internal/knowledge/move.go — PlanMove; internal/cli/errors.go — orphanedIDFailure;
internal/cli/know_test.go — TestKnowAdd_AliasInsideOrphanDirectory_FailsNamingTheFile
**Tests:** CLI tests: `know mv` a Note, break its frontmatter, then `know add` (in `know_test.go`)
and `know mv` (in `know_mv_test.go`, beside `TestKnowMv_OntoTakenIDOrAlias_FailsExistsNamingTheNote`)
onto the old id are refused with the lowercase `note` message naming the broken Note; an unrelated
broken Note still lets `know add` write.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'TestKnow'`
**Commit:** `fix(know): refuse an id an unparseable Note holds as an alias`
Depends on item 3.

## 5. Document and pin the --kv warning order — ✅ DONE (2026-10-12)

**What:** bug `.capcom/issues/kv-warning-order.md`: `docs/format.md` says warnings come "after its
result", which `--kv` output contradicts.
**Goal:** `docs/format.md` and `README.md` state that `--kv` output puts every `WARNING=` line
before the first record so that `STATUS=` is always the last line, while plain text prints
`warning:` lines after the result; CLI tests pin that order for `show --kv` and `create --kv`
against a tree holding an unparseable file.
**Approach (assumed at the header base):** amend the "after its result" sentence in
`docs/format.md` `### Errors` and the `--kv` paragraph in `README.md`. Tests drive the real CLI with
`runCLI`, not `kvRenderer` alone (`TestKVRenderer_Render_ClosesEveryRecordWithStatus` already pins
the renderer).
**Regression guard.** There is no `show_test.go`: put the `show --kv` test in `internal/cli/read_test.go` (`newReadStore`, `writeIssueFiles`, `lastLine`). The pinned order is today's behaviour, so both tests pass before the item; prove they bite by mutation: move the warning loop after the records in `kvRenderer.Render`, see both tests fail, revert.
**Files:** docs/format.md, README.md, internal/cli/read_test.go, internal/cli/write_test.go
**Read first:** internal/cli/output.go — kvRenderer.Render; internal/cli/read_test.go — TestShow_KV_ClosesEachIDWithItsStatus, newReadStore, writeIssueFiles;
internal/cli/write_test.go — TestCreate_UnrelatedBadFileName_WritesAndWarnsOnce; internal/cli/cli_test.go — lastLine; docs/format.md — ### Errors; README.md — --kv paragraph
**Tests:** `show <id> --kv` (in `read_test.go`) and `create --kv` (in `write_test.go`) beside a
broken file: output starts with `WARNING=` naming that file, then `ID=` …, last line `STATUS=ok`;
both fail under the guard's mutation of `kvRenderer.Render`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'TestShow|TestCreate'`
**Closes:** kv-warning-order
**Commit:** `docs(format): state where --kv puts warnings`

## 6. Settle the beads guide's open migration cases

**What:**
**Goal:** `docs/migrating-from-beads.md` answers every bullet of
`.capcom/issues/beads-guide-trial-gaps.md`:
- mixed notes: an undated lead before dated entries becomes its own entry dated by `created_at`;
- a dated entry starts only with a date at the start of a line; a mid-sentence date does not; the
  leading date and a directly following `:` are removed, the rest kept verbatim;
- a non-plan `spec_id` becomes the Log entry `- <created_at date> beads: spec: <path>`;
- a plan `spec_id` makes the issue Planned with or without the `planned` label, stated next to
  the `planned` label rule;
- a non-empty `close_reason` (e.g. `Closed`) is kept verbatim; only an empty one becomes
  `Closed in beads.`;
- `created_by`, `dependency_count`, `dependent_count`, `comment_count`, `_type`, and a
  dependency's `created_by` / `metadata` are named among the dropped fields.
**Approach (assumed at the header base):** edit the Field mapping table rows (`notes`, `spec_id`,
`close_reason`, anything else), `### Labels`, `### Plans`, `### Log entries` and
`### What does not migrate`; match the guide's existing Log entry example format.
**Regression guard.** `created_at` already matches at the base, so Acceptance checks each bullet with a grep that matches nothing at the base: `undated` (the undated lead), `start of a line`, `kept verbatim` (on the close_reason row), `with or without the .planned. label`, `comment_count`, and `` `_type` `` with its backticks (a bare `_type` matches `issue_type`).
**Files:** docs/migrating-from-beads.md
**Read first:** docs/migrating-from-beads.md — Field mapping table (notes, spec_id, close_reason, anything-else rows), ### Labels, ### Plans,
### Log entries, ### What does not migrate; .capcom/issues/beads-guide-trial-gaps.md — bullet list
**Tests:** none (docs only).
**Acceptance:**
- `grep -n 'created_at' docs/migrating-from-beads.md`
- `grep -n 'dependency_count' docs/migrating-from-beads.md`
- `grep -n 'spec: ' docs/migrating-from-beads.md`
- `grep -n -i 'undated' docs/migrating-from-beads.md`
- `grep -n 'start of a line' docs/migrating-from-beads.md`
- `grep -n 'close_reason.*kept verbatim' docs/migrating-from-beads.md`
- `grep -n 'with or without the .planned. label' docs/migrating-from-beads.md`
- `grep -n 'comment_count' docs/migrating-from-beads.md`
- ``grep -n '`_type`' docs/migrating-from-beads.md``
**Closes:** beads-guide-trial-gaps
**Commit:** `docs(beads): settle notes dating, spec_id and close reason cases`
