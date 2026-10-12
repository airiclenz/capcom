# Architecture review plan

**Goal:** Land every candidate of the 2026-10-12 architecture review — one frontmatter codec, Remove
as a Tree plan, one node-store module per Tree kind, an index that owns its freshness, walk and
warnings, Briefing and Ready-list modules, pass-throughs moved home, and a semantic repo seam for
`cmd/release` — plus the defects it found on the way.
**Date:** 2026-10-12
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** dc21c39

**Sources:**
- `docs/reviews/archived/architecture-review-20261012-0847.html`
- `docs/adr/0001-*.md`, `0002-*.md`, `0007-*.md`, `0008-*.md`, `0009-*.md`
- `docs/format.md`, `CONTEXT.md`; Knowledge Notes `cli-test-harness`, `coding-conventions`,
  `error-messages`, `release-process`

**Ratified design calls** (user, 2026-10-12):
- **Scope:** all seven candidates plus the defects.
- **prime and index warnings:** discarding the Refresh report is intended; document and pin it.
- **Move/rm rollback:** byte-exact and mode-preserving, CRLF included.
- **Quoting:** both kinds use the hand-rolled issue policy (double quotes, one line); Notes reformat.
- **Unknown key:** both kinds append a hint generated from the kind's key list.
- **`capcom mv --if-match`:** added.
- **Edit resolution:** through the index, injected into the node store as a Resolver.
- **rm dependents:** matched on id plus aliases read from the files, the rule mv uses.
- **Index refresh:** eager in Open; the cobra index annotations stay as the "needs index" marker.
- **Walk/alias wording:** the index wording wins; tree keeps-first and index leaves-both-out policies stay.
- **Warning kind:** internal only; the JSON envelope's `warnings` stays `[]string`.
- **Release seam:** the capcom checkout and GitHub repo only, one `repo` interface; every gh release
  call names `--repo airiclenz/capcom`.
- **ADR 0008:** amended in place, with the `release-process` Note.
- **lock ErrNotHeld failure:** the record path moves from the message into the `path` field.

**Standing requirements:**
- skills: coding-standards
- CLI tests go through `runCLI` / `runCLIWith`, never `t.Parallel()` (Note `cli-test-harness`).
- Tests on this host: one package at a time with `GOMEMLIMIT=2GiB`; never `./...` with `-race`.
- Every error names the file relative to the worktree, and the line when known (Note `error-messages`).
- `issue.Error` / `knowledge.Error` stay as aliases of `frontmatter.Error`.

**Out of scope:**
- Sharing the codec with `internal/config`; giving YAML syntax errors in config a line.
- `--kv` on `know` verbs; unifying create / know add body shaping.
- A seam for tap, bucket or winget repos; komac, brew, go build stay on the runner.
- Removing the cobra index annotations; changing duplicate-claim policies.
- Version, tag or CHANGELOG release-heading changes.

**Regression check (2026-10-12, dc21c39):**
- 1: guard folded (decision: the message text is relativized too; the know mv parse test is a pin)
- 2: guard folded; supersedes the `internal/tree/move.go:186` comment and `docs/plans/archived/2026-10-11 - 00 - capcom-v1-issues-plan.md:222` NOTES (LF rollback)
- 3: guard folded (a no-behaviour-change refactor; the test is a pin)
- 5: guard folded
- 6: guard folded
- 9: guard folded
- 10: guard folded
- 11: guard folded (decision: repoint cli callers; `go build ./...` and cli tests in Acceptance)
- 12: guard folded (decision: repoint cli callers; unparseable own file still removed)
- 13: guard folded (decision: unparseable own file still removed, pinned)
- 15: guard folded (decision: repoint the alias-helper callers)
- 16: guard folded
- 17: guard folded
- 18: guard folded
- 19: guard folded (decision: nil Resolver)
- 20: guard folded
- 21: guard folded
- 22: guard folded
- 23: guard folded (decision: `internal/cli/errors.go`); yields to `docs/format.md:188-189`
- 24: guard folded (decision: `internal/cli/errors.go`)
- 26: guard folded
- 27: guard folded
- 29: guard folded
- 30: guard folded (decision: `lock/claim.go` calls `gitx.CurrentBranch`)
- 31: recast (decision: builds on item 2's exported atomic writer; depends on items 2 and 16)
- 33: recast (decision: injectable repo factory in newRootCommand/execute; nonInteractiveEnv on every remote call)
- 34: recast (decision: ReleaseTags, IsClean, CommitFile only)
- 35: guard folded (decision: `cmd/release/publish.go`)
- 36: recast (decision: ShowFile and LastCommitTouching moved in from item 34)
- 37: guard folded
- 38: guard folded
- 39: guard folded
- 31: guard folded (re-check: Goal restated to the bytes `config.RaiseFormat` keeps; the failed-write trigger named; `internal/cli/doc.go` file map)
- 33: recast (re-check decision: item 33 alone owns the repo factory, reachable from every verb's run options; `cmd/release/prepare.go` added)
- 34: recast (re-check decision: never edits `cmd/release/main.go`; Goal is ReleaseTags, IsClean, CommitFile)
- 36: recast (re-check decision: Goal lists ShowFile and LastCommitTouching)
- 37: guard folded (decision: Acceptance adds `! grep -nE '"git", "(push|ls-remote|fetch|cat-file|show|log)"' cmd/release/publish.go`; replaces the rejected item-36 grep)

## 1. mv and know mv report worktree-relative paths — ✅ DONE (2026-10-12)

NOTES (2026-10-12): the cli-reachable seam is a package var `applyMove` (`(*tree.Move).Apply`) in internal/cli/mv.go, used by mv and know mv, not a rename-level seam: Move's rename field can only be set from tree's export_test, and internal/tree is outside this item's Files; the test fake returns the os.LinkError a failed rename gives.
NOTES (2026-10-12): an Apply failure is invalid with `path` set to the moved node's file; the failing step's path, and any rollback failures, stay in the message, with the worktree root prefix (root + OS separator) stripped by the new `app.worktreeRelativeText`. On Windows the relative paths left in OS error text keep backslashes.
NOTES (2026-10-12): noteMoveFailure's parse branch now delegates to the `noteFailure` app method, which also relativizes `knowledge.Error.Message`; issue and Note parse failures in update, know update and know verify go through the same methods (their paths were already relative, so they are unchanged).

**What:**
**Goal:** every failure `capcom mv` and `capcom know mv` emit — a parse failure of the moved file and
an `Apply` failure — names its file relative to the worktree root.
**Approach (assumed at the header base):** regression from the copied failure helpers: `issueFailure`
(`internal/cli/update.go`) copies `issue.Error.Path` unchanged and `issue.PlanMove` returns absolute
paths, so `moveFailure` in `internal/cli/mv.go` leaks them; `noteMoveFailure` relativizes. Make
`issueFailure` and `noteFailure` app methods that pass `Path` through `a.worktreeRelative`; relativize
`Apply` failures (`asError` path text, `tree.ErrMoveTarget`) in mv and know mv the way rm's
`restoreRewrites` does.
**Regression guard.** The failure's path field is worktree-relative AND its message carries no absolute
worktree path — the worktree root prefix is stripped from OS error text in mv and know mv Apply failures
(Note error-messages). The issue mv parse test is the bite; the know mv parse test is a pin
(`noteMoveFailure` already relativizes parse failures). The Apply-failure half gets a cli-reachable
test through an unexported rename seam (`tree.SetMoveRename` is export_test-only).
**Files:** internal/cli/update.go, internal/cli/know.go, internal/cli/know_update.go, internal/cli/mv.go, internal/cli/know_mv.go, internal/cli/mv_test.go, internal/cli/know_mv_test.go, internal/cli/errors.go
**Read first:** internal/cli/mv.go — move, moveFailure; internal/cli/update.go — issueFailure; internal/cli/know_mv.go — noteMoveFailure;
internal/cli/errors.go — worktreeRelative, asError; internal/cli/rm.go — restoreRewrites; internal/cli/mv_test.go — TestMv_OntoAliasOfUnparseableIssue_FailsExistsNamingTheFile
**Tests:** `mv --json` of a broken issue file gives code `invalid`, path `.capcom/issues/<id>.md`,
line > 0 (the bite); a know mv twin (a pin, green at base); an mv Apply failure injected through the
seam names its file relative to the worktree and its message holds no absolute worktree path.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Mv|Move'`; `go vet ./internal/cli/`
**Commit:** `fix(cli): mv reports worktree-relative paths`

## 2. Byte-exact rollback for Move and rm — ✅ DONE (2026-10-12)

NOTES (2026-10-12): the raw atomic writer is exported as `tree.WriteFileAtomicVerbatim(path, content io.Reader, mode)`, since `internal/cli/rm.go` needs it; `fillTemp` takes a convert flag and the permissions, and `WriteFileAtomic` keeps its signature and behaviour
NOTES (2026-10-12): the rewrite step and rm read the mode with `os.Stat`, not the `Lstat` the plan names: a node file may be a symlink (tree.Load accepts one), and Lstat would report the link's 0777 rather than the permissions of the content being restored
NOTES (2026-10-12): rm captures each rewritten file's mode at plan time in `planDepDrops`, kept beside its `tree.Rewrite` in a new unexported `depRewrite`; the remove seam is the package var `removeFile`, used for both the children directory and the issue file
NOTES (2026-10-12): the archived `docs/plans/archived/2026-10-11 - 00 - capcom-v1-issues-plan.md` NOTES line on LF rollback is left as the dated record it is; the superseded `internal/tree/move.go` comment is replaced by the AddRewrite doc comment

**What:**
**Goal:** when a Move or rm fails part-way, every rewritten file is restored to its original bytes
(CRLF included) and original file mode.
**Approach (assumed at the header base):** defect: the undo in `tree.Move`'s rewrite step and rm's
`restoreRewrites` write through `tree.WriteFileAtomic`, whose `fillTemp` converts to LF and chmods
0644. Factor `fillTemp` to take a convert flag and mode; add an unexported raw atomic writer in
`internal/tree/atomic.go` (exported only if `internal/cli/rm.go` needs it); `AddRewrite` records the
mode at plan time and undo writes `Original` verbatim with it.
**Regression guard.** Capture the mode inside tree, with an Lstat in the rewrite step's apply just before
the overwrite, so `internal/issue/move.go` and `internal/knowledge/move.go` need no change; rm builds its
own `tree.Rewrite` (never `AddRewrite`), so `rm.go` captures the mode itself, in `planDepDrops` or
`restoreRewrites`. The rm rollback twin swaps an unexported package var in `rm.go` (the remove func) to
fail `os.Remove` of the issue file after the rewrites are written. Mode assertions compare with the
original's Stat or skip off-Unix, as `internal/cli/init_test.go:686` does. Supersedes the
`internal/tree/move.go:186` comment ("The undo writes the original bytes with LF line endings"), rewritten
here, and the NOTES of `docs/plans/archived/2026-10-11 - 00 - capcom-v1-issues-plan.md:222`.
**Files:** internal/tree/atomic.go, internal/tree/move.go, internal/tree/move_test.go, internal/cli/rm.go, internal/cli/rm_test.go
**Read first:** internal/tree/move.go — AddRewrite, Apply, rollBack; internal/tree/atomic.go — WriteFileAtomic, fillTemp;
internal/cli/rm.go — applyIssueRemoval, restoreRewrites, planDepDrops
**Tests:** a rewrite whose on-disk original is `"a\r\nb\r\n"` with mode 0600, failure injected via
`SetMoveRename`, asserts identical bytes and the original's mode (mode half skipped off-Unix); an rm
rollback twin through `rm.go`'s remove seam.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/tree/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run Rm`
**Commit:** `fix(tree): restore original bytes and mode on rollback`

## 3. know add reports every validation problem with its line — ✅ DONE (2026-10-12)

NOTES (2026-10-12): no CHANGELOG entry — per the regression guard the routing through writeNoteFile changes nothing a user can observe at base (a second problem and a non-zero line are unreachable for a flag-built Note); the new test TestKnowAdd_BlankTitle_FailsInvalidWithoutWriting is a pin.

**What:**
**Goal:** `capcom know add` reports all validation problems of the new Note, joined as
`writeNoteFile` joins them, with the line of the first, and writes nothing on failure.
**Approach (assumed at the header base):** defect: `addNote` in `internal/cli/know.go` validates
inline and returns only `problems[0].Message` without a line. Replace that block with
`a.writeNoteFile(path, note)` and pass its etag to `noteWriteResult`.
**Regression guard.** A no-behaviour-change refactor: route `addNote` through `writeNoteFile`. At base
`addNote` already fails invalid with `title is required` and writes nothing, a second problem is
unreachable (`ValidateRef` pre-checks refs, `edit.Labels` drops empty labels) and a flag-built Note's
title line is always 0, so "every problem" and "with the line of the first" leave the Goal; the test is a
pin, not a bite.
**Files:** internal/cli/know.go, internal/cli/know_test.go
**Read first:** internal/cli/know.go — addNote, newNote, noteWriteResult; internal/cli/know_update.go — writeNoteFile;
internal/knowledge/note.go — Validate; internal/edit/edit.go — Labels; internal/cli/know_test.go — TestKnowAdd_RefusedInputs_FailWithoutWriting
**Tests:** `know add --id x --title " "` exits invalid with `title is required` and no file exists (a pin,
green at base).
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run KnowAdd`
**Commit:** `fix(cli): know add reports every validation problem`

## 4. lock touch on a free lock names its worktree — ✅ DONE (2026-10-12)

NOTES (2026-10-12): the record path reaches the CLI through a new `lock.NotHeldError{Path}` that unwraps to `ErrNotHeld` (mirroring `HeldError.Path`), since `emitLock` has no `Locks` to compute the path from; `errors.Is(err, lock.ErrNotHeld)` still holds.

**What:**
**Goal:** `capcom lock touch` on a lock that is not held reports `lock: free` with the worktree key
(`_main` or `wt-<name>`), and its `not_found` failure carries the record path in the `path` field,
not in the message — the shape `HeldError` failures use.
**Approach (assumed at the header base):** defect: `lock.Touch` returns its zero status on
`ErrNotHeld`; set it to `current` as `Release` and `Break` do. `emitLock` in `internal/cli/lock.go`
then renders the status as given. Move the record path of the ErrNotHeld failure into `Error.Path`.
**Files:** internal/lock/lock.go, internal/lock/lock_test.go, internal/cli/lock.go, internal/cli/lock_test.go
**Read first:** internal/lock/lock.go — Touch, Release, Break, ErrNotHeld; internal/cli/lock.go — emitLock, failedLockResult;
internal/lock/lock_test.go — TestTouch_FreeLock_FailsNotHeld; internal/cli/lock_test.go — TestLockTouch_FreeLock_FailsNotFoundWithFreeRecord
**Tests:** `TestTouch_FreeLock_FailsNotHeld` asserts the key; a CLI test runs `lock touch --token any
--json` and asserts `data.worktree == "_main"`, `data.lock == "free"`, `error.path` set, exit not found.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/lock/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run Lock`
**Commit:** `fix(lock): touch on a free lock names its worktree`

## 5. Pin that prime leaves index warnings to the check count — ✅ DONE (2026-10-12)

**What:**
**Goal:** a test pins that `capcom prime` prints no `warning:` line for a broken issue file while
counting it in `check: N problems`, and a doc comment at the Refresh call in `internal/cli/prime.go`
states why.
**Approach (assumed at the header base):** `briefIssues` discards the Refresh report; document that
at the call, add the test beside the prime fixture tests.
**Regression guard.** prime renders warnings to stdout, so the test asserts stdout has no `^warning: `
line (the regexp `TestPrime_GarbageIndex_ExitsZeroWithOneWarningLine` uses), stderr is empty, and stdout
contains `check: 1 problem; run capcom check`.
**Files:** internal/cli/prime.go, internal/cli/prime_test.go
**Read first:** internal/cli/prime.go — briefIssues, briefCheck, brief; internal/cli/output.go — textRenderer.Render; internal/cli/root.go — emit;
internal/cli/prime_test.go — TestPrime_GarbageIndex_ExitsZeroWithOneWarningLine, TestPrime_BrokenConfig_ExitsZeroWithOneWarning; internal/cli/reindex_test.go — newIssueStore
**Tests:** prime over a store with one unparseable issue: stdout has no `^warning: ` line and contains
`check: 1 problem; run capcom check`; stderr is empty.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run Prime`
**Commit:** `test(cli): pin that prime leaves index warnings to check`

## 6. frontmatter package: split, decode and one Error — ✅ DONE (2026-10-12)

NOTES (2026-10-12): the per-kind noun is one article-led string on `Kind.Noun` ("an issue" / "a note"); the no-fence message appends " file must start with a --- line" and the hint reads "; <noun> has <keys>". `Split(path, data, noun)` takes the noun too, so it reports the fence failures itself; `Decode` rejects keys outside `Kind.Keys` before calling the callback, so a kind's callback never sees an unknown key.
NOTES (2026-10-12): scalar helpers are `Field.Scalar` (raw text, false for a list or mapping), `Field.IsEmpty` and `Field.Errorf` (an *Error at the value line), enough for issue priority and timestamps; `Field.List` always returns entry lines, which issue callers discard.

**What:**
**Goal:** a leaf package `internal/frontmatter` exports `Error{Path, Line, Message}`, `Split`, and a
`Decode` that drives a per-kind key callback, with fence and CRLF handling, duplicate- and
unknown-key errors (ADR 0009 strictness: unknown keys stay errors), YAML-error lines, the
conflict-marker override, and `Text` / `List` (with entry lines) / scalar helpers on a field. The
unknown-key message appends a hint generated from the kind's key list (`; an issue has …`).
**Approach (assumed at the header base):** lift the generic half of `internal/issue/file.go`
(`splitFrontmatter`, `parser`, `document`, `conflictError`, `yamlError`, `fields`, `isNull`, `text`,
`list`) — Note's `list` variant that keeps entry lines. Uses `internal/rawfm`. No caller yet.
**Regression guard.** `Decode` takes a per-kind noun for the no-fence message (`an issue file` / `a note
file must start with a --- line`) and for the unknown-key hint (`; a note has title, labels, refs and
aliases`, the `internal/knowledge/note.go` form); `decode_test.go` asserts both nouns.
**Files:** internal/frontmatter/frontmatter.go, internal/frontmatter/decode.go, internal/frontmatter/decode_test.go
**Read first:** internal/issue/file.go — Parse, splitFrontmatter, parser.conflictError, parser.yamlError; internal/knowledge/note.go — Parse, parser.list;
internal/rawfm/conflict.go — ConflictMarkers; internal/issue/file_test.go — TestParse_Failures_CiteFileLine
**Tests:** table tests for each failure with file and line; entry lines of a list; the no-fence message
and the unknown-key hint for both kind nouns.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/frontmatter/`; `go vet ./internal/frontmatter/`
**Commit:** `refactor(frontmatter): extract the shared frontmatter decoder`

## 7. frontmatter encode with the one quoting policy — ✅ DONE (2026-10-12)

NOTES (2026-10-12): the encoder is a zero-value-ready `frontmatter.Encoder` with `Text(key, value)`, `List(key, entries)`, `Raw(key, value)` (written verbatim, for a priority or an RFC3339 time) and `File(body) []byte`; every write leaves out an empty value, so issue priority passes its number and a zero time passes "".
NOTES (2026-10-12): the quoting helpers (`scalar`, `isPlainSafe`, `needsEscape`, `doubleQuoted`) are copied, not moved, from `internal/issue/file.go`: that file is outside this item's Files and still serializes with them until item 8 routes issues through the codec, which deletes the issue copies.
NOTES (2026-10-12): consequential edit — internal/frontmatter/frontmatter.go: made necessary by encode.go (the package doc said the package only reads files; it now names Encoder and the quoting policy)

**What:**
**Goal:** `internal/frontmatter` exports an encoder writing text, lists and raw values under the
documented policy (`docs/format.md` quoting: plain when it reads back as the same plain string,
else double-quoted on one line).
**Approach (assumed at the header base):** move `writeText`, `writeList`, `scalar`, `isPlainSafe`,
`needsEscape`, `doubleQuoted` from `internal/issue/file.go`; copy the tricky-text round-trip tests.
Depends on item 6.
**Files:** internal/frontmatter/encode.go, internal/frontmatter/encode_test.go
**Read first:** internal/issue/file.go — Issue.Serialize, writeText, writeList, scalar, isPlainSafe, doubleQuoted;
internal/issue/file_test.go — TestSerialize_TrickyText_RoundTrips; docs/format.md — Canonical form
**Tests:** round trip of tricky text; plain text unquoted; long text on one line.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/frontmatter/`
**Commit:** `refactor(frontmatter): add the shared encoder`

## 8. Issues parse and serialize through the codec — ✅ DONE (2026-10-12)

NOTES (2026-10-12): the issue field table is a `fields` slice of {key, decode, encode} entries built by `textField` / `listField` / `timeField` (priority is spelled out inline); it drives Parse's Decode callback, Serialize's Encoder calls and the `issueKind` key list, so the key order lives in one place. `issue.Error` is `type Error = frontmatter.Error`; the issue copies of the quoting helpers are gone.

**What:**
**Goal:** `internal/issue/file.go` holds only the issue field table and the `priority` / `timestamp`
decoders; `issue.Error` is an alias of `frontmatter.Error`; serialized bytes are unchanged, and an
unknown key's message carries the generated hint.
**Approach (assumed at the header base):** route `Parse` and `Serialize` through items 6–7; update
the two unknown-key pins in `file_test.go`. Golden files under `internal/issue/testdata` stay. Depends on item 7.
**Files:** internal/issue/file.go, internal/issue/issue.go, internal/issue/file_test.go
**Read first:** internal/issue/file.go — Parse, Issue.Serialize, parser.priority, parser.timestamp; internal/issue/issue.go — Error;
internal/issue/file_test.go — TestParse_Failures_CiteFileLine, TestParseSerialize_CanonicalGoldens_AreIdentity; internal/index/refresh.go — parseIssue
**Tests:** existing goldens and failure tables pass; unknown-key test asserts the hint.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/issue/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Fmt|Reindex'`
**Commit:** `refactor(issue): parse and serialize through the frontmatter codec`

## 9. Notes on the codec, double-quoted, and the store reformatted — ✅ DONE (2026-10-12)

NOTES (2026-10-12): the Note field table is a `fields` slice of {key, decode, encode} entries (title and refs spelled out inline, labels and aliases through `listField`); it drives Parse's Decode callback, Serialize's Encoder calls and the `noteKind` key list. `knowledge.Error` is `type Error = frontmatter.Error`; the yaml.v3 parser and Encoder path (`parser`, `splitFrontmatter`, `addText`, `addList`, `stringNode`, the fence and tag constants) is gone. Validate, AliasesLine and RefLine stay in note.go: moving them to another file is not this item's to do.

**What:**
**Goal:** `internal/knowledge/note.go` holds only the Note field table; `knowledge.Error` is an alias
of `frontmatter.Error`; Notes serialize under the issue quoting policy; `docs/format.md`'s Note
paragraph says double quotes; every Note under `.capcom/knowledge/` is in canonical form.
**Approach (assumed at the header base):** drop the yaml.v3 Encoder path (`Serialize`, `addText`,
`addList`, `stringNode`); keep `RefLine` from list entry lines. Then run
`go run ./cmd/capcom fmt` and commit the reformatted Notes (single → double quotes). Depends on item 7.
**Regression guard.** A user store whose Note titles yaml.v3 single-quoted (any title with `: `) fails
`capcom fmt --check` right after upgrading, and the README presents `fmt --check` as a gate: the item
records a CHANGELOG entry under `[Unreleased]` — Notes are now written with double quotes, and `capcom
fmt` once rewrites single-quoted Notes so `fmt --check` passes again.
**Files:** internal/knowledge/note.go, internal/knowledge/knowledge_test.go, docs/format.md, .capcom/knowledge/docs-layout.md, .capcom/knowledge/release-process.md, .capcom/knowledge/coding-conventions.md, .capcom/knowledge/store-layout.md
**Read first:** internal/knowledge/note.go — Parse, Note.Serialize, Note.RefLine; internal/knowledge/knowledge_test.go — TestSerialize_ThenParse_RoundTrips;
internal/cli/fmt.go — canonicalNote; internal/index/notes.go — parseNote; internal/knowledge/move.go — PlanMove; docs/format.md — Note files
**Tests:** a title `Fix: parser` serializes as `title: "Fix: parser"`; round trip stays one line per key.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/knowledge/`; `go run ./cmd/capcom fmt --check`
**Commit:** `refactor(knowledge): serialize Notes through the frontmatter codec`

## 10. One parse-failure path in check and cli — ✅ DONE (2026-10-12)

NOTES (2026-10-12): consequential edit — internal/cli/know_update.go: made necessary by merging noteFailure into frontmatterFailure (updateNote called noteFailure)

NOTES (2026-10-12): the one helper is `app.frontmatterFailure` (in update.go, replacing issueFailure; noteFailure deleted from know.go); it relativizes through the existing worktreeRelative, which already keeps a relative path as is (relativeSlashPath returns "" for it), so no separate filepath.IsAbs check was added — TestUpdate_FrontmatterFailure_RelativizesOnlyAnAbsolutePath pins both cases

NOTES (2026-10-12): moveFailure and noteMoveFailure stay two functions (they differ in their exists failure and not_found handling); only their parse branch now matches *frontmatter.Error and goes through frontmatterFailure

**What:**
**Goal:** `check.addParseFailure` has no type switch over kind error types, and cli turns any
`frontmatter.Error` into a failure through one helper that relativizes the path.
**Approach (assumed at the header base):** merge `issueFailure`, `noteFailure`, mv's parse branch and
`noteMoveFailure` into one app method. Depends on items 1, 8, 9.
**Regression guard.** The helper relativizes only an absolute `Path` (`filepath.IsAbs`) and keeps a relative
one as is: `editIssue`, `updateNote` and `verifyNotes` parse with the index's relative `entry.Path`, for
which `relativeSlashPath` returns "". A unit test of the helper with a relative-path
`frontmatter.Error` pins it.
**Files:** internal/check/check.go, internal/cli/update.go, internal/cli/know.go, internal/cli/mv.go, internal/cli/know_mv.go, internal/cli/cli_test.go
**Read first:** internal/cli/update.go — issueFailure, editIssue; internal/cli/know.go — noteFailure, verifyNotes; internal/cli/know_update.go — updateNote;
internal/cli/know_mv.go — noteMoveFailure; internal/cli/issue_render.go — relativeSlashPath; internal/check/check.go — addParseFailure
**Tests:** existing check and mv/know mv path tests; a unit test of the helper with a relative-path
`frontmatter.Error` (path kept as is), named so the Acceptance `-run` filter runs it.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/check/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Mv|Move|Update|Know'`
**Commit:** `refactor(cli): one failure path for frontmatter errors`

## 11. tree.Plan: Move's step/undo core, generalised — ✅ DONE (2026-10-12)

NOTES (2026-10-12): `(*Move).Apply() error` kept as a wrapper over the embedded `Plan.Apply() ([]error, error)`, so no caller changed: internal/cli/mv.go, internal/cli/know_mv.go, internal/issue/move.go, internal/knowledge/move.go, both of their move_test.go files and internal/tree/export_test.go (SetMoveRename reaches the promoted `rename` field) are untouched.

NOTES (2026-10-12): `Plan` exports `AddStep(apply, undo)`, `AddRewrite`, `AddCleanup(func() error)` and `Apply`; rename and empty-dir steps stay unexported (`renameStep` on Plan in plan.go, `removeEmptyDirStep` still an undoable step in move.go); the zero Plan is usable (nil rename means os.Rename). The three Plan tests sit in internal/tree/move_test.go, the listed test file, rather than a new plan_test.go.

**What:**
**Goal:** `internal/tree` exports a `Plan` (ordered steps with undo, `AddRewrite`, best-effort
cleanups returned as warnings, `Apply`) that `Move` embeds; behaviour of mv and know mv is unchanged.
**Approach (assumed at the header base):** rename `Move`'s `rewrites`/`renames []moveStep` core into
`Plan` in a new `plan.go`; `Move` keeps `Node`, `NewID`, `NewPath`. Depends on item 2.
**Regression guard.** Keep `(*Move).Apply() error`, a method on `Move` wrapping `Plan.Apply`, or update its
callers (`mv.go`, `know_mv.go`, both `move_test.go` files). Also repoint every remaining cli caller of
any tree symbol this item renames or reshapes, add those cli files to Files, and Acceptance adds `go
build ./...` plus `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Mv|Move|Rm'`. `Plan` keeps a rewrite phase that runs before every other step whatever order steps were
added in (`AddRewrite` runs after `planRenames`), and `removeEmptyDirStep` stays an undoable step.
**Files:** internal/tree/plan.go, internal/tree/move.go, internal/tree/move_test.go, internal/tree/export_test.go, internal/issue/move.go, internal/knowledge/move.go, internal/cli/mv.go, internal/cli/know_mv.go, internal/issue/move_test.go, internal/knowledge/move_test.go
**Read first:** internal/tree/move.go — Move, AddRewrite, planRenames, removeEmptyDirStep, Apply, rollBack;
internal/tree/move_test.go — TestPlanMove_RenameWithChildren_RenamesChildrenDirAndRewritesFirst; internal/cli/mv.go — move
**Tests:** existing move tests, `TestPlanMove_RenameWithChildren_RenamesChildrenDirAndRewritesFirst`
included; a Plan test for rollback order and cleanup warnings, and one that a rewrite added after a
rename still applies first.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/tree/ ./internal/issue/ ./internal/knowledge/`; `go build ./...`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Mv|Move|Rm'`
**Commit:** `refactor(tree): generalise Move into Plan`

## 12. tree.PlanRemove; know rm uses it — ✅ DONE (2026-10-12)

NOTES (2026-10-12): `tree.Removal` embeds `Plan` (as `Move` does) and exposes `Node` and `ChildrenDir`. The refusals are typed: `*tree.ChildrenError` (node path) and `*tree.ChildrenDirError` (the directory); a children directory that cannot be read returns its `*fs.PathError`. The cli maps all of them through one `removalFailure` with the same codes, paths and wording as before; `childCount`/`entryCount` moved into tree with `checkChildrenDir`. The emptied-parent cleanup fails as `*tree.CleanupError{Path, Err}`, and the cli renders it with `relativeSlashPath` and the same `; the empty directory was not removed` suffix.

NOTES (2026-10-12): Until item 13, rm.go uses `issues.PlanRemove` only for its refusals and `ChildrenDir`. It keeps its own apply and the `removeFile` test seam, which `TestRm_IssueFileRemovalFails_RestoresRewriteBytesAndMode` swaps, and it calls the exported `tree.RemoveEmptyDir` for the parent cleanup. It also uses a cli `issueKind = "an issue"` constant. Item 13 can drop `RemoveEmptyDir`, `ChildrenDir` and that constant once rm applies the plan.

NOTES (2026-10-12): The node file's undo reads its bytes and mode at apply time, just before the remove, and the children directory comes back with Mkdir+Chmod of its original permissions. `knowledge.PlanRemove` reuses `moveKind` as `removeKind` and never reads the Note file. Unused `noteFileExtension` was deleted from know_rm.go.

**What:**
**Goal:** `(*tree.Tree).PlanRemove` returns a Plan that refuses a node with children or a non-empty
children dir, removes an empty children dir then the file, and cleans an emptied parent directory
best-effort (a warning, never a rollback); `knowledge.PlanRemove` wraps it and `capcom know rm`
applies it.
**Approach (assumed at the header base):** move `checkChildrenDir` into tree;
`removeEmptyParentDir` (`internal/cli/know_rm.go`) becomes the Plan cleanup. Depends on item 11.
**Regression guard.** Also repoint every remaining cli caller of checkChildrenDir and any moved symbol
(rm.go until item 13), add those files to Files, Acceptance adds `go build ./...`; rm and know rm of a
node whose own file does not parse still succeed, as today — `knowledge.PlanRemove` never parses the
Note file. The Plan returns cleanup failures as path plus error; cli renders them with
`relativeSlashPath` and today's `; the empty directory was not removed` suffix. The CLI Acceptance
widens to `-run Rm` so the `TestRm_*` refusal wordings are checked too.
**Files:** internal/tree/remove.go, internal/tree/remove_test.go, internal/knowledge/remove.go, internal/knowledge/remove_test.go, internal/cli/know_rm.go, internal/cli/rm.go, internal/cli/know_rm_test.go, internal/tree/export_test.go
**Read first:** internal/cli/know_rm.go — removeNote, checkChildrenDir, removeEmptyParentDir, childCount; internal/cli/rm.go — planIssueRemoval, removeIssue;
internal/tree/move.go — removeEmptyDirStep; internal/knowledge/move.go — PlanMove
**Tests:** tree unit tests per refusal and rollback (a remove seam in `internal/tree/export_test.go`); a
knowledge and a CLI test that remove an unparseable Note; existing know rm and rm CLI tests.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/tree/ ./internal/knowledge/`; `go build ./...`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run Rm`
**Commit:** `refactor(tree): plan Remove like Move`

## 13. issue.PlanRemove; rm uses it — ✅ DONE (2026-10-12)

NOTES (2026-10-12): `issue.PlanRemove(issues, id, issue.RemoveOptions{IfMatch, ShouldDropDeps})` returns an `*issue.Removal` embedding `*tree.Removal`, with `DepsDropped`, `RelatedDropped` (tree order) and `Unread`. It takes the etag (`IfMatch`) so the check order is unchanged: tree refusals, then a stale etag (`*issue.ETagMismatchError`, conflict), then dependents (`*issue.DependentsError`, invalid). The dependents refusal keeps today's path order (sorted by slash path, as the index's `ORDER BY path` gave it). The removed issue's file is read for its etag but never parsed; its names are the id plus `node.Aliases`, or `rawfm.Aliases` of its bytes when the node has none.

NOTES (2026-10-12): The cli maps an Apply failure through `issueRemovalApplyFailure`: Path is the first planned path (rewritten files, children dir, issue file) the cause's message names as `<path>:`, else the issue file. The message joins the cause and any rollback failures with `; `, as before, and now passes through `worktreeRelativeText`. internal/issue/move.go is untouched: the drop loop needs no move-specific branch, so the loop lives in remove.go (`planDrops`) and no helper was shared.

NOTES (2026-10-12): `TestRm_IssueFileRemovalFails_RestoresRewriteBytesAndMode` and the cli `removeFile` seam are gone. The rollback test is now `TestPlanRemove_ApplyFails_RestoresRewritesChildrenDirAndFile` in internal/issue/remove_test.go. It injects a failing step with the exported `Plan.AddStep` after the removal's own steps, because `tree.SetRemovalRemove` lives in tree's export_test.go, which issue tests cannot reach. Cli rm no longer uses `loadDepGraph` or the index for its dependents.

NOTES (2026-10-12): deviation, deleting dead code (coding-standards [must]; anticipated by item 12's NOTES): internal/tree/remove.go — `tree.RemoveEmptyDir` lost its last caller with this item, so it was deleted and its doc folded into `removeEmptyDir`. internal/tree/remove_test.go — its `TestRemoveEmptyDir_HoldsAnEntry_LeavesIt` became `TestPlanRemove_ParentDirHoldsAnEntry_LeavesIt`, which tests the same behaviour through `PlanRemove`.

**What:**
**Goal:** `issue.PlanRemove` plans the removal plus dependency and related drops, matching dependents
on id plus aliases read from the files (unparseable files listed as Unread), refusing dependents
unless drops are asked for; `capcom rm` applies it and a failed rm restores the children dir too.
**Approach (assumed at the header base):** reuse `planRewrite`'s read-every-file loop; delete
`planDepDrops`, `dropEntries`, `applyIssueRemoval`, `restoreRewrites`, `loadDepGraph` use in
`internal/cli/rm.go`. Depends on item 12.
**Regression guard.** rm of a node whose own file does not parse still succeeds, as today (pin with a
test): the removed issue's own file is never parsed; its names are the id plus `node.Aliases` (or
rawfm-recovered aliases). The etag is checked before the dependents refusal (ifMatch passed in, or
checked before `PlanRemove`). A Plan failure maps back to an Error whose Path is the failing step's
worktree-relative path. The removed issue is never its own dependent (`TestRm_DependsOnItself_IsRemoved`);
the refusal keeps today's path order of dependents, or the item states the new order.
**Files:** internal/issue/remove.go, internal/issue/remove_test.go, internal/cli/rm.go, internal/cli/rm_test.go, internal/issue/move.go
**Read first:** internal/cli/rm.go — removeIssue, planIssueRemoval, planDepDrops, restoreRewrites; internal/issue/move.go — planRewrites, planRewrite;
internal/cli/rm_test.go — TestRm_DependsOnItself_IsRemoved, TestRm_StaleIfMatch_ConflictsAndKeepsTheFile
**Tests:** issue unit tests for drops, Unread, an unparseable removed file and the rollback (an
injected remove, as `SetMoveRename` does) in `internal/issue/remove_test.go`; CLI tests: rm of an
unparseable issue succeeds, and a stale `--if-match` on an issue with a dependent gives conflict;
`rm_test.go` keeps only refusal and success cases.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/issue/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run Rm`
**Commit:** `refactor(issue): plan rm through tree.Plan`

## 14. nodestore skeleton — ✅ DONE (2026-10-12)

NOTES (2026-10-12): the four cli alias lookups are folded into one generic pair, `Kind.ReadAliases` and `Kind.RecoverAliases`, driven by the kind's Codec; the cli originals stay until item 15 repoints their callers.

NOTES (2026-10-12): nodestore also imports `internal/frontmatter` (the `Codec.Validate` result type and `fileError`'s `errors.As`), a package the approach's import list leaves out; it imports neither index nor cli. `edit` is not imported yet: nothing computes an etag until a verb lands.

NOTES (2026-10-12): the error constructors (`fileError`, `conflictError`, `existsError`) are unexported Kind methods and `cli.kindFailure` has no caller yet; both get their callers in items 15-18. The constructors are tested directly from the internal test package; kindFailure is untested until a verb reaches it through runCLI.

NOTES (2026-10-12): `ConflictError` and `ExistsError` carry their message text (the wording of the cli's existsFailure / noteExistsFailure and its etag conflicts, noun from the Codec) in a `Message` field beside their structured fields, so kindFailure maps all three types the same way.

**What:**
**Goal:** a package `internal/nodestore` exports `Kind[T]` with constructors `Issues` and `Notes`, a
`Codec` per kind (parse, serialize, validate, aliases, noun), `Written{ID, Path, ETag}`, typed
errors (file, conflict, exists) with worktree-relative paths, the alias lookups, and a `Resolver`
interface; cli has one `kindFailure` mapping them to codes. No verb uses it yet.
**Approach (assumed at the header base):** one deep module per kind (coding-standards: deep over
shallow); imports store, tree, issue, knowledge, rawfm, edit, config — never index or cli. Copy the
alias lookups (`readAliases`, `recoverAliases`, `readNoteAliases`, `recoverNoteAliases`) in; their
cli originals go in item 15. Depends on item 10.
**Files:** internal/nodestore/kind.go, internal/nodestore/codec.go, internal/nodestore/errors.go, internal/nodestore/kind_test.go, internal/cli/errors.go
**Read first:** internal/cli/create.go — readAliases, recoverAliases, existsFailure; internal/cli/know.go — readNoteAliases, recoverNoteAliases, noteExistsFailure;
internal/cli/errors.go — asError, worktreeRelative
**Tests:** codec round trips per kind; alias lookups on a parseable and an unparseable file.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodestore/`; `go vet ./internal/nodestore/ ./internal/cli/`
**Commit:** `refactor(nodestore): add the per-kind node store`

## 15. Kind.Create for create and know add — ✅ DONE (2026-10-12)

NOTES (2026-10-12): Create returns the non-blocking tree problems (paths worktree-relative) beside the Written, `Create(id, parent, build) (Written, tree.Errors, error)`; cli filters them against the index report with a new `unwarnedTreeProblems` (errors.go) and renders the warnings, so nodestore never sees the index.

NOTES (2026-10-12): know add now validates its --ref values before calling Create, so they still precede the claim refusals, but now also precede id validation: a call with both a bad id and a bad ref reports the ref first (both invalid, exit 1).

NOTES (2026-10-12): Create writes LF line endings for both kinds (the CRLF fold writeIssueFile applied to issues only); a serialized Note carries no CRLF in practice, so the bytes are unchanged.

NOTES (2026-10-12): nodestore holds its own copies of the claim walk (findNodeFile, unparsed-alias holder) and the blocking-problem test; the cli originals (orphanedIDFailure, unparsedAliasFailure, findNodeFile, isBlockingTreeProblem, triageTreeProblems, existsFailure, noteExistsFailure) stay for mv, know mv, rm and know rm until items 17-18 move those verbs.

NOTES (2026-10-12): newIssuePath and newNotePath were deleted with the inline sequences they served; the ordering comment lives on Create, and errors.go's comment on unparsedAliasFailure stays since mv still runs that order.

**What:**
**Goal:** `Kind.Create(id, parent, build)` runs id validation, Tree load, the claim refusals in the
order `internal/cli/errors.go` documents, triage, validation (all problems, line of the first) and
the write; `capcom create` and `capcom know add` call it; the cli alias helpers are gone.
**Approach (assumed at the header base):** replace the inline sequences in `create` and `addNote`;
delete the four alias helpers from `create.go`/`know.go`; the ordering comment moves onto `Create`.
Depends on items 3, 14.
**Regression guard.** Also repoint the remaining callers of readAliases/recoverAliases/readNoteAliases/
recoverNoteAliases (internal/cli/mv.go, know_mv.go, rm.go, know_rm.go, fmt.go) to the nodestore lookups
and add those files to Files; Acceptance adds `go build ./...`. nodestore cannot import index: `Create`
takes an already-warned predicate or returns the non-blocking problems for cli to triage as today — the
item says which; `TestCreate_IndexWarnedFile*_WarnsOnce` stay green. `Create` returns `tree.ErrNotFound`
/ `ErrAmbiguous` unwrapped and `kindFailure` sends them through `resolveFailure` (an unknown `--parent`
stays not_found, exit 3); know add's `--ref` validation stays ahead of the claim refusals.
**Files:** internal/nodestore/create.go, internal/nodestore/create_test.go, internal/cli/create.go, internal/cli/know.go, internal/cli/errors.go, internal/cli/mv.go, internal/cli/know_mv.go, internal/cli/rm.go, internal/cli/know_rm.go, internal/cli/fmt.go
**Read first:** internal/cli/create.go — create, newIssuePath; internal/cli/know.go — addNote;
internal/cli/errors.go — triageTreeProblems, isIndexWarnedAbout, orphanedIDFailure, unparsedAliasFailure; internal/cli/write_test.go — assertWarnedOnce
**Tests:** nodestore tests per refusal without runCLI; existing create/know add CLI tests unchanged.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodestore/`; `go build ./...`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Create|KnowAdd'`
**Commit:** `refactor(cli): create and know add through nodestore`

## 16. Kind.Edit for update, know update and the edit verbs — ✅ DONE (2026-10-12)

NOTES (2026-10-12): once ref has resolved, `Kind.Edit` returns a Written naming the node's id and file even when it fails (ETag empty); the cli's generic `editNode` (update.go) maps every failure through kindFailure and fills the file's path only into the change callback's own error (matched by identity, since Edit returns it unwrapped), so a write failure keeps the shape it had.

NOTES (2026-10-12): the cli adapters `issueResolver` (update.go) and `noteResolver` (know_update.go) make `*index.Index` the Resolver; `editIssue` stays as a one-line wrapper returning `nodestore.Written`, so its seven callers build `writeResult` from it and close reads `written.ETag` (the map cast is gone). `updateNote` keeps its AGENTS.md refresh after the edit.

NOTES (2026-10-12): `Edit` on a Kind built without a Resolver fails with an unexported `errNoResolver` rather than panicking.

NOTES (2026-10-12): a node file that cannot be read now fails with its message naming the file relative to the worktree (it carried the absolute path before); know update now folds CRLF to LF before computing the etag, as WriteFileAtomic already did to the file, so the etag it reports always matches the file.

NOTES (2026-10-12): `readIssueFile`, `writeIssueFile`, `writeNoteFile`, `absoluteIssuePath` and `frontmatterFailure` stay: fmt, show, know show, mv and know mv still use them (items 17 and 19).

**What:**
**Goal:** `Kind.Edit(ref, ifMatch, change)` resolves through the injected Resolver, checks the etag,
validates and writes, returning `Written`; update, know update, log, related, dep, close and reopen
use it, and close reads the etag from `Written`.
**Approach (assumed at the header base):** a cli adapter makes `*index.Index` the Resolver;
`editIssue` and `updateNote` collapse into it; drop close's `Result.Data.(map[string]string)` cast.
Depends on item 14.
**Regression guard.** `Kind.Edit` returns the change callback's error unwrapped; the cli side keeps
`editIssue`'s rule (`asError`, Path = the file's when empty), so dep rm / related rm stay not_found and
know update's plain `ValidateRef`/`ReplaceBody` errors stay invalid at the file. Every `Kind.Edit`
caller, close's per-arg path included, maps the error through `kindFailure`. The order is pinned:
resolve, read, etag check, parse, change, validate, write. Every `internal/cli/doc.go` file-map line
naming a moved function or cycle is updated (grep it for each deleted symbol).
**Files:** internal/nodestore/edit.go, internal/nodestore/edit_test.go, internal/cli/update.go, internal/cli/know_update.go, internal/cli/close.go, internal/cli/log.go, internal/cli/related.go, internal/cli/dep.go, internal/cli/doc.go, internal/cli/related_test.go
**Read first:** internal/cli/update.go — editIssue, writeIssueFile; internal/cli/know_update.go — updateNote; internal/cli/close.go — closeOne;
internal/cli/related.go — newRelatedAddCommand; internal/cli/dep.go — newDepRmCommand; internal/cli/show.go — resolveFailure;
internal/cli/related_test.go — TestRelatedRm_NotRelated_FailsNotFound
**Tests:** nodestore etag conflict and validation tests with a fake Resolver; a related-add stale
`--if-match` test asserts config.yaml is unchanged; existing CLI tests.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodestore/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Update|Close|Reopen|Log|Related|Dep'`
**Commit:** `refactor(cli): edit verbs through nodestore`

## 17. Kind.Move; capcom mv gains --if-match — ✅ DONE (2026-10-12)

NOTES (2026-10-12): re-derived from "wrap issue.PlanMove": issue.Move carried no copy of the moved file's bytes before the move, which the etag guard needs, so it gains `Original` (set where planRewrite reads the moved issue), mirroring knowledge.Move.Original.

NOTES (2026-10-12): `Kind.Move(ref, to Destination, ifMatch) (Moved, tree.Errors, error)` runs the whole sequence (load, resolve ref and --parent against the Tree, plan, orphan claim, triage, unparsed-alias claim, etag, Apply) and, like Create, returns the non-blocking tree problems; cli filters them against the index report and renames paths inside the moved children directory (`movedWarnings`) so nodestore never sees the index. The per-kind planner is an unexported `planMove` field of Codec.

NOTES (2026-10-12): the apply test seam moved from cli's package var `applyMove` to an unexported Kind field `applyMove`; TestMv_ApplyFails_* and TestKnowMv_ApplyFails_* (cli) are replaced by nodestore's TestMove_ApplyFails_NamesTheFilesRelativeToTheWorktree, which asserts the same relative *FileError (kindFailure maps it to invalid at that path).

NOTES (2026-10-12): consequential edit — internal/cli/know_mv_test.go: made necessary by removing cli's applyMove seam (its ApplyFails test moved to nodestore).

NOTES (2026-10-12): consequential edit — internal/cli/errors.go: made necessary by mv and know mv leaving it — orphanedIDFailure, unparsedAliasFailure, findUnparsedAliasHolder, findNodeFile and hiddenNamePrefix lost their last callers and were deleted; the splitTreeProblems comment no longer names a renaming caller.

NOTES (2026-10-12): consequential edit — internal/cli/create.go: made necessary by mv moving to nodestore — existsFailure lost its last caller and was deleted.

NOTES (2026-10-12): consequential edit — internal/cli/know.go: made necessary by know mv moving to nodestore — noteExistsFailure lost its last caller and was deleted.

NOTES (2026-10-12): an unreadable node file during a move's plan now fails with its message naming files relative to the worktree; a plan-time tree.ErrNotFound on know mv now types not_found as on mv (unreachable after resolution).

**What:**
**Goal:** `Kind.Move(ref, dest, ifMatch)` plans and applies a move with the etag guard; `capcom mv`
and `capcom know mv` call it; `capcom mv --if-match <etag>` refuses on a mismatch with the conflict
failure know mv gives; README and `docs/format.md` list the flag.
**Approach (assumed at the header base):** wrap `issue.PlanMove` / `knowledge.PlanMove` and
`Plan.Apply`. Depends on items 11, 14.
**Regression guard.** `Kind.Move` loads the Tree, resolves `ref` and `--parent` against it, never through
the Resolver, and runs the triage and orphan/unparsed-alias refusals before the etag check and Apply,
as item 15 does for Create (the index-warned set passed in; nodestore may not import index) — or it is
split into Plan/Apply with the cli checks between. mv's Use string gains `[--if-match <etag>]`, and the
issue-side if-match sentences in `docs/format.md` (the rm paragraph) and README's etag line name mv.
**Files:** internal/nodestore/move.go, internal/nodestore/move_test.go, internal/cli/mv.go, internal/cli/know_mv.go, internal/cli/mv_test.go, README.md, docs/format.md
**Read first:** internal/cli/mv.go — move, triageMoveProblems, resolveNode, resolveParent;
internal/cli/errors.go — orphanedIDFailure, unparsedAliasFailure, isIndexWarnedAbout; internal/cli/mv_test.go — TestMv_DuplicateID_FailsNamingBothFiles
**Tests:** `runCLI(t, root, "mv", "--if-match", "<stale>", …)` gives conflict and moves nothing;
the current etag moves; `TestMv_DuplicateID_FailsNamingBothFiles`, `TestMv_OntoIDInsideOrphanDirectory_*`,
`TestMv_OntoAliasOfUnparseableIssue_*` and their know mv twins stay green.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodestore/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Mv|Move'`
**Commit:** `feat(cli): mv --if-match through nodestore`

## 18. Kind.Remove for rm and know rm — ✅ DONE (2026-10-12)

NOTES (2026-10-12): re-derived from the assumption that Remove fits in remove.go alone (Read-first planIssueRemoval and checkChildrenDir no longer exist in cli): the per-kind planner hook (Codec.planRemove) lives in internal/nodestore/codec.go and the test seam for Apply (Kind.applyRemoval) in internal/nodestore/kind.go, as item 17's planMove/applyMove do.

NOTES (2026-10-12): the etag check keeps its place after the children refusals and before the dependents refusal (ifMatch passed into the plan); triage runs before planning against the node and its parent, and the cli drops index-warned problems with unwarnedTreeProblems, as mv does.

NOTES (2026-10-12): consequential edit — internal/cli/mv.go: resolveNode lost its last callers (rm, know rm) and was deleted.

NOTES (2026-10-12): consequential edit — internal/cli/errors.go: triageTreeProblems, splitTreeProblems and isBlockingTreeProblem lost their last callers and were deleted, as item 15's NOTES foresaw for items 17-18.

NOTES (2026-10-12): OS-error text in a children-directory read failure, a know rm Apply failure and a cleanup warning is now relativized to the worktree like the rest; a know rm Apply failure now uses rm's failed-path pick and joined message.

NOTES (2026-10-12): pre-existing: the consts issueNoun (rm.go) and noteNoun (know_rm.go) are unused; left as they are.

**What:**
**Goal:** `Kind.Remove(ref, ifMatch, dropDeps)` applies the plans of items 12–13 with the etag
guard; `capcom rm` and `capcom know rm` call it.
**Approach (assumed at the header base):** thin over `issue.PlanRemove` / `knowledge.PlanRemove`.
Depends on items 13, 14.
**Regression guard.** As item 17: `Remove` loads the Tree, resolves against it (never through the Resolver),
triages tree problems against the node and its parent (index-warned set passed in) before planning,
and returns the plan's deps/related dropped, Unread and cleanup warnings. The etag is checked after the
children refusals and before the dependents refusal (ifMatch passed into the plan), or `Remove` checks
it before planning and the item says the conflict now precedes every refusal.
**Files:** internal/nodestore/remove.go, internal/nodestore/remove_test.go, internal/cli/rm.go, internal/cli/know_rm.go
**Read first:** internal/cli/rm.go — removeIssue, planIssueRemoval; internal/cli/know_rm.go — removeNote, checkChildrenDir; internal/cli/errors.go — triageTreeProblems;
internal/cli/rm_test.go — TestRm_StaleIfMatch_ConflictsAndKeepsTheFile; internal/cli/know_rm_test.go — TestKnowRm_StaleIfMatch_ConflictsAndKeepsTheFile, TestKnowRm_ChildOfDuplicateParent_Fails
**Tests:** nodestore etag and dependents tests; existing rm/know rm CLI tests,
`TestRm_StaleIfMatch_ConflictsAndKeepsTheFile` and `TestKnowRm_ChildOfDuplicateParent_Fails` included.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodestore/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run Rm`
**Commit:** `refactor(cli): rm and know rm through nodestore`

## 19. Kind.Format for fmt — ✅ DONE (2026-10-12)

NOTES (2026-10-12): Kind.Format(isCheck) returns a Formatted{Paths, Warnings []error} (tree problems as relative *tree.Error, file problems as *FileError); cli renders them as before. canonicalForm, canonicalizer, formatTree, canonicalIssue and canonicalNote are removed from internal/cli/fmt.go; formatRun.emit and errorStrings stay.

NOTES (2026-10-12): writes now go through the node store's write, so a rewritten Note is written with LF line endings like issues (and like know update since item 16); a node file that cannot be read now fails with its message naming the file relative to the worktree instead of absolute. Listed paths and warnings are unchanged.

NOTES (2026-10-12): deviation, deleting dead code (coding-standards [must]) — internal/cli/update.go, internal/cli/know_update.go, internal/cli/errors.go: with fmt on the node store, writeIssueFile, writeNoteFile, treeLoadFailure, treeProblems and problemSeparator had no callers left and are removed with the imports this left unused; relativeTreeError, called only by treeLoadFailure, goes with it.

**What:**
**Goal:** `Kind.Format(check)` canonicalizes every node file of a kind and returns written paths and
warnings; `capcom fmt` calls it for both kinds with unchanged output.
**Approach (assumed at the header base):** move `canonicalIssue` / `canonicalNote` from
`internal/cli/fmt.go`. Depends on item 14.
**Regression guard.** Kind.Format works with a nil Resolver (fmt opens no index).
**Files:** internal/nodestore/format.go, internal/nodestore/format_test.go, internal/cli/fmt.go
**Read first:** internal/cli/fmt.go — formatTree, canonicalIssue, canonicalNote, formatRun.emit; internal/cli/update.go — writeIssueFile; internal/cli/know_update.go — writeNoteFile;
internal/cli/fmt_test.go — TestFmt_BrokenFile_WarnsAndLeavesItUntouched, TestFmt_InvalidFiles_WarnedAboutAndUntouchedWhileOthersAreFormatted
**Tests:** nodestore format tests, with a nil Resolver; existing fmt CLI tests.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodestore/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run Fmt`
**Commit:** `refactor(cli): fmt through nodestore`

## 20. Typed index warning kind — ✅ DONE (2026-10-12)

**What:**
**Goal:** every index warning carries a `WarningKind` (invalid-id, invalid-alias, duplicate,
repeated-alias, orphan-directory, frontmatter, field) set at its producer and kept when replayed
from stored problems; the index schema version is raised so stored problems are rebuilt.
**Approach (assumed at the header base):** `Warning{Kind, Message}` in `internal/index/index.go`; set
in `walk.go`, `refresh.go` (`aliasProblems`, `excludeClaims`, `parseIssue`), `notes.go`; bump
`schemaVersion` in `schema.go`.
**Regression guard.** The producer-to-kind map: the walk's `directory left out: its node file X.md has an
invalid id` is orphan-directory (the kind tree gives that directory, so create warns once); the parse
fallback without `*Error` (`parseIssue`, `parseNote`) is frontmatter; Validate failures are field.
`Kind` is stored under a json tag so `rewarn` replays it.
**Files:** internal/index/index.go, internal/index/refresh.go, internal/index/walk.go, internal/index/notes.go, internal/index/schema.go, internal/index/index_test.go, internal/index/notes_test.go
**Read first:** internal/index/index.go — Warning, Report; internal/index/refresh.go — rewarn, parseIssue, aliasProblems, excludeClaims;
internal/index/walk.go — treeWalker.walk; internal/index/notes.go — parseNote
**Tests:** one kind assertion per producer, the left-out directory as orphan-directory included, and a replayed warning.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/`
**Commit:** `refactor(index): type the warning kind`

## 21. cli classifies index warnings by kind — ✅ DONE (2026-10-12)

NOTES (2026-10-12): re-derived from the cli-local `problemKind` enum: it is deleted along with `indexWarningKindOf` and its prefix/suffix constants, and `treeProblemKindOf` now returns `(index.WarningKind, bool)`, which `isIndexWarnedAbout` compares with `Warning.Kind`.

**What:**
**Goal:** `isIndexWarnedAbout` decides by `WarningKind`, never by message text; a file with a
duplicate frontmatter key no longer hides the tree's duplicate-id warning of the same file.
**Approach (assumed at the header base):** latent defect: `indexWarningKindOf`
(`internal/cli/errors.go`) reads `duplicate key` as Duplicate. Delete it and its prefix/suffix
constants. Depends on item 20.
**Regression guard.** The bite test puts the duplicate key in the later-path file (tree keeps the first file
of an id), e.g. `zz-parent/dup-id.md` beside `dup-id.md`, and asserts both the `duplicate key` warning
and the `duplicate id "dup-id"` warning on that path.
**Files:** internal/cli/errors.go, internal/cli/write_test.go
**Read first:** internal/cli/errors.go — isIndexWarnedAbout, treeProblemKindOf, indexWarningKindOf, splitTreeProblems; internal/cli/create.go — create;
internal/cli/write_test.go — assertWarnedOnce, TestCreate_IndexWarnedFileSelfAlias_WarnsOnce, newWriteStore
**Tests:** create beside `dup-id.md` and a later-path `zz-parent/dup-id.md` holding a duplicate key warns
about both, on that path.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Create|Warn'`
**Commit:** `fix(cli): classify index warnings by kind`

## 22. index.Open returns a fresh index — ✅ DONE (2026-10-12)

NOTES (2026-10-12): Open(s, mode, options...) takes index.ModeRefresh / index.ModeRebuild; Refresh and Rebuild also record their report, so Report() is the latest pass (Open's own until one runs again) and returns cloned slices. cli keeps a.indexReport (Open's report, never mutated) and adds a.indexWarnings, which emit renders; mv and know mv replace it with movedWarnings' renamed copies.
NOTES (2026-10-12): prime now adds the "; run capcom reindex" hint to every Open failure, since Open's refresh and its mkdir/ping failures arrive on one error; previously a mkdir or ping failure carried no hint.

**What:**
**Goal:** `index.Open(store, mode)` returns an index already refreshed (or rebuilt) with its report
on `Report()`; no caller orders Open before Refresh; mv derives its moved warnings from copies, never
by mutating the shared report.
**Approach (assumed at the header base):** `prepareIndex` (`internal/cli/root.go`), `briefIssues`
(`prime.go`) and `reindex.go` use the mode; `Refresh` stays exported for index tests; the mutation of
`a.indexReport.Warnings` in `mv.go` returns renamed copies instead. Depends on items 5, 20.
**Regression guard.** Tests that pin the first refresh's report (`TestNotes_OldSchemaIndex_RebuildsWithTheNotes`,
`TestRefresh_NewTree_IndexesEveryIssue`) read it from `x.Report()`, not from a second Refresh. know mv's
`movedWarnings` call changes with mv's, and `emit` renders the renamed copies in place of the report's
warnings, not beside them. Refresh failures Open returns keep prime's `; run capcom reindex` hint, and
Open closes its db when its own refresh or rebuild fails.
**Files:** internal/index/index.go, internal/index/refresh.go, internal/index/index_test.go, internal/cli/root.go, internal/cli/prime.go, internal/cli/mv.go, internal/cli/reindex.go, internal/index/notes_test.go, internal/cli/know_mv.go
**Read first:** internal/index/index.go — Open, Report; internal/index/refresh.go — Refresh, Rebuild; internal/cli/root.go — prepareIndex, emit;
internal/cli/prime.go — briefIssues; internal/cli/mv.go — movedWarnings
**Tests:** index Open-is-fresh test; existing prime, reindex, `TestMv_BadFileInMovedChildrenDir_WarnsWithNewPath`
and `TestKnowMv_BadFileInMovedChildrenDir_WarnsWithNewPath`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Prime|Reindex|Mv'`
**Commit:** `refactor(index): Open returns a fresh index`

## 23. One ADR 0001 walk — ✅ DONE (2026-10-12)

NOTES (2026-10-12): internal/cli/errors.go is unchanged — both directory problems still wrap tree.ErrOrphanDirectory (the invalid-id one through the new invalidIDDirectoryError type), so treeProblemKindOf pairs them with WarningOrphanDirectory as before; TestMv_UnrelatedInvalidIDDirectory_MovesAndWarnsOnce pins that pairing.
NOTES (2026-10-12): the "capcom check shows the new wording" test lives in internal/check/check_test.go (Problem.String, which the cli prints verbatim), not internal/cli/check_test.go, which the item's Files does not list.
NOTES (2026-10-12): the index adapter now stats each node file with os.Lstat after tree.Walk instead of DirEntry.Info during the walk; a node file that vanishes between the two is still skipped, but its children directory is now walked rather than warned about as an orphan.

**What:**
**Goal:** `tree.Walk` is the one directory walk; the index adapts it (stat, relative paths); both
report an orphan directory as `directory without its node file: no X.md beside it; left out` and the
directory of an invalid-id node as `directory left out: its node file X.md has an invalid id`.
**Approach (assumed at the header base):** export the walk from `internal/tree/tree.go`; `index/walk.go`
becomes a thin adapter; update tree-wording pins in tree, check and cli tests. Depends on item 20.
**Regression guard.** Add internal/cli/errors.go to Files — treeProblemKindOf must keep pairing each tree
problem with the index warning of the same kind after the wording/sentinel change. The invalid-id
directory problem still wraps `tree.ErrOrphanDirectory` (a small error type, since its text cannot start
with the sentinel); the item yields to `docs/format.md:188-189` (a new id named by a file under a
directory without its node file is refused). The index adapter keeps wrapping walk errors with the
worktree-relative path.
**Files:** internal/tree/tree.go, internal/tree/tree_test.go, internal/index/walk.go, internal/index/index_test.go, internal/check/check_test.go, internal/cli/mv_test.go, internal/cli/errors.go, internal/cli/write_test.go
**Read first:** internal/tree/tree.go — Load, Tree.walk; internal/index/walk.go — treeWalker.walk, treeWalker.relative;
internal/cli/errors.go — orphanedIDFailure, unparsedAliasFailure, treeProblemKindOf; internal/check/check.go — treeCode
**Tests:** tree and index tests assert both exact messages; `capcom check` shows the new wording; create
refuses `--id child-issue` beside `Bad_Name/child-issue.md`; check gives code `orphan-directory` for
that directory.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/tree/ ./internal/index/ ./internal/check/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Mv|Check'`
**Commit:** `refactor(tree): one directory walk for tree and index`

## 24. One per-file alias check — ✅ DONE (2026-10-12)

NOTES (2026-10-12): the signature is `tree.CheckAliases(path, kind, id string, aliases []string) ([]string, tree.Errors)`, not `CheckAliases(id, aliases)`: path so each problem is a ready *tree.Error with Path and Alias set (check cites the aliases line from Alias), kind per the regression guard, and the kept aliases returned so tree.Load maps the first of an alias listed twice and drops only the offending one.
NOTES (2026-10-12): tree.Load gets no kind and no tree.Load caller is touched; it passes the kind "node", so check reads `alias "x" repeats the node's own id`, while the index passes its file kind and keeps `the issue's own id` / `the note's own id` unchanged.
NOTES (2026-10-12): a repeated alias wraps a new sentinel tree.ErrRepeatedAlias as well as ErrDuplicate, with Other left empty; cli treeProblemKindOf matches ErrRepeatedAlias before ErrDuplicate. internal/check/check.go is unchanged: treeCode's default still gives CodeDuplicate.

**What:**
**Goal:** `tree.CheckAliases(id, aliases)` reports invalid aliases, `alias "x" repeats the
<kind>'s own id` and `alias "x" is listed twice`; tree load and index refresh both use it; tree
still drops only the offending alias and the index still leaves the file out.
**Approach (assumed at the header base):** replace tree's own-id/listed-twice `duplicate alias`
messages in `indexAliases` and the index's `aliasProblems`. Depends on item 23.
**Regression guard.** Add internal/cli/errors.go to Files — same pairing rule as item 23. `CheckAliases`
takes a kind parameter and the item says how `tree.Load` gets the kind, listing every `tree.Load` caller
it touches — or the tree message is worded without a kind. The repeated-alias problem keeps wrapping
`ErrDuplicate` with `Alias` set and cli classifies it by its own sentinel, so
`TestCreate_IndexWarnedFileSelfAlias_WarnsOnce` stays green.
**Files:** internal/tree/tree.go, internal/tree/tree_test.go, internal/index/refresh.go, internal/check/check_test.go, internal/cli/write_test.go, internal/cli/errors.go
**Read first:** internal/tree/tree.go — indexAliases, ValidateAlias, Error; internal/index/refresh.go — aliasProblems;
internal/cli/errors.go — treeProblemKindOf, isIndexWarnedAbout; internal/check/check.go — treeCode; internal/cli/write_test.go — TestCreate_IndexWarnedFileSelfAlias_WarnsOnce
**Tests:** exact messages in tree, check and create tests.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/tree/ ./internal/index/ ./internal/check/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run Create`
**Commit:** `refactor(tree): share the per-file alias check with the index`

## 25. index.Entry.Node — ✅ DONE (2026-10-12)

NOTES (2026-10-12): dep's Dependency graph nodes now carry Labels too; NewDepGraph never reads them, so dep's behaviour is unchanged.

**What:**
**Goal:** `index.Entry.Node()` is the one Entry→ready-rules Node conversion; `capcom ready` and
`capcom dep` use it.
**Approach (assumed at the header base):** replace the copies in `internal/cli/ready.go`
(`readyEntries`) and `internal/cli/dep.go` (dep's copy omits Labels). check's copy builds from a
parsed issue and stays.
**Files:** internal/index/query.go, internal/index/query_test.go, internal/cli/ready.go, internal/cli/dep.go
**Read first:** internal/cli/ready.go — readyEntries; internal/cli/dep.go — loadDepGraph; internal/issue/ready.go — Node; internal/issue/deps.go — NewDepGraph;
internal/index/index.go — Entry; internal/index/query_test.go — listFixture, ids; internal/check/check.go — the issue.Node built from a parsed issue (stays)
**Tests:** query test for the conversion; existing ready and dep CLI tests.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Ready|Dep'`
**Commit:** `refactor(index): one Entry to Node conversion`

## 26. Ready-list module — ✅ DONE (2026-10-12)

NOTES (2026-10-12): the label / no-label / plan filter now runs in the index: `briefing.ReadyList` judges over `List(IsClosedIncluded)` and keeps the ids of a second, filtered query through the new `index.Index.ListIDs` (List now builds on it), so readiness is still judged over every issue; `--all`, show-hidden and limit stay in memory after the stable priority sort.
NOTES (2026-10-12): cli imports the package as `readylist`, because the package-level `briefing` type in internal/cli/prime.go already holds that name and renaming it is out of this item's scope.

**What:**
**Goal:** `briefing.ReadyList(index, config, options)` returns ready items judged over all issues,
filtered by labels / no-labels / plan state / hidden, stably sorted by priority, then limited;
`capcom ready` and prime use it, and cli no longer re-implements the index filter.
**Approach (assumed at the header base):** new `internal/briefing/ready.go`; delete `selects` in
`internal/cli/ready.go`. Depends on items 22, 25.
**Regression guard.** The options also carry `--all`, show-hidden and limit; each returned item carries its
`issue.Readiness` beside the `index.Entry`, so `readyListData`, `readyLines` and `droppedDepWarnings`
render unchanged (`TestReady_All_JSONCarriesWhatBlocks` stays green).
**Files:** internal/briefing/ready.go, internal/briefing/ready_test.go, internal/cli/ready.go, internal/cli/prime.go, internal/index/query.go
**Read first:** internal/cli/ready.go — readyEntries, readyOptions.selects, readyListData, droppedDepWarnings; internal/cli/prime.go — briefIssues;
internal/index/query.go — Filter, List; internal/issue/ready.go — Readiness
**Tests:** briefing tests per filter and the stable sort, no `t.Chdir`; existing ready CLI tests.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/briefing/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Ready|Prime'`
**Commit:** `refactor(briefing): add the Ready-list module`

## 27. Briefing module — ✅ DONE (2026-10-12)

NOTES (2026-10-12): consequential edit — internal/briefing/ready.go: the package doc comment now names Compose beside ReadyList, made necessary by adding the Briefing to the package
NOTES (2026-10-12): the cli-local `briefing` struct and its brief/briefIssues/briefLocks/briefCheck methods are gone (composition moved to briefing.Compose, rendering to renderBriefing, addCappedList, addLocks, addHealth); prime.go therefore imports the package under its own name, while ready.go keeps the `readylist` alias item 26 gave it
NOTES (2026-10-12): Elsewhere scan warnings come back as Failures of SectionElsewhere but render as the scan worded them, without a `section: ` prefix, because the guard's `section: flatten(err)` form would have changed prime's output; every other section renders as `section: flatten(err)`, the store's through storeFailure
NOTES (2026-10-12): a panic recovered in Compose is a SectionInternal ("prime") Failure rendered as `prime: internal error: <value>`, now flattened onto one line; cli keeps its own recover around rendering with the same wording
NOTES (2026-10-12): prime no longer sets a.store either (Compose opens its own store); internal/cli/prime_test.go is unchanged, and every existing prime and ready test passes as is

**What:**
**Goal:** `briefing.Compose(cwd, now)` returns the Briefing as data (Ready planned/unplanned, Doing,
Tree locks, Elsewhere, broken Refs, check count, per-section failures); `capcom prime` only renders
it, with unchanged output, and no longer sets `a.issueIndex`.
**Approach (assumed at the header base):** move `brief`, `briefIssues`, `briefLocks` composition out
of `internal/cli/prime.go`; keep the recover; CONTEXT.md's Briefing entry points at the package.
Depends on items 5, 26.
**Regression guard.** `Compose` runs `elsewhere.Scan` itself (the store's `Elsewhere.Days`, the issue-dir
pathspec, none when the store lies outside the worktree); cli keeps a recover around render too, so a
panic is still one `prime: internal error` warning. Failures come back as an ordered slice of (section,
error) in today's order — store, index, ready, doing, locks, check, Elsewhere — rendered as
`section: flatten(err)`, with `storeFailure` kept for the store entry.
**Files:** internal/briefing/briefing.go, internal/briefing/briefing_test.go, internal/cli/prime.go, internal/cli/prime_test.go, CONTEXT.md
**Read first:** internal/cli/prime.go — brief, briefIssues, briefLocks, briefing.warn; internal/cli/output.go — addElsewhere, issuesPathspec;
internal/cli/init.go — storeFailure; internal/elsewhere/elsewhere.go — Scan
**Tests:** Compose tests over a fixture store without `t.Chdir`; existing prime tests unchanged.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/briefing/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run Prime`
**Commit:** `refactor(briefing): compose the Briefing outside cli`

## 28. Delete the issue label and body wrappers — ✅ DONE (2026-10-12)

**What:**
**Goal:** `issue.WithLabels`, `issue.AppendBody`, `issue.ReplaceBody` and `issue.ErrReplaceMatch`
no longer exist; issue verbs call `edit.Labels`, `edit.AppendBody`, `edit.ReplaceBody` as Note verbs do.
**Approach (assumed at the header base):** callers in `internal/cli/update.go` and
`internal/cli/create.go`; drop the duplicated mutate test. Depends on items 15, 16.
**Files:** internal/issue/mutate.go, internal/issue/mutate_test.go, internal/cli/update.go, internal/cli/create.go
**Read first:** internal/issue/mutate.go — WithLabels, AppendBody, ReplaceBody, ErrReplaceMatch; internal/cli/update.go — applyUpdate; internal/cli/create.go — newIssue;
internal/edit/edit.go — Labels; internal/issue/mutate_test.go — TestIssue_ReplaceBody_NotExactlyOnce_FailsWithErrReplaceMatch
**Tests:** existing edit and CLI tests.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/issue/ ./internal/edit/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Update|Create'`
**Commit:** `refactor(issue): delete label and body pass-throughs`

## 29. gitx.CurrentBranch — ✅ DONE (2026-10-12)

NOTES (2026-10-12): consequential edit — internal/gitx/merge.go: made necessary by unexporting Run (the "Unlike Run" comment now reads "Unlike run")

**What:**
**Goal:** `gitx.CurrentBranch(dir)` returns the trimmed current branch; `gitx.Run` is unexported;
`internal/cli` imports no gitx argv.
**Approach (assumed at the header base):** the caller is `newClaim` in `internal/cli/lock.go`.
Depends on item 4.
**Regression guard.** `CurrentBranch` keeps `git branch --show-current` semantics: the unborn branch's name
in a fresh repo, and "" with no error on a detached HEAD (never `rev-parse --abbrev-ref HEAD`).
**Files:** internal/gitx/gitx.go, internal/gitx/branches.go, internal/gitx/gitx_test.go, internal/cli/lock.go
**Read first:** internal/cli/lock.go — newClaim; internal/gitx/gitx.go — Run, AuthorName; internal/gitx/branches.go — Branches, ShowFile;
internal/gitx/gitx_test.go — newRepo, TestRun_FailingCommand_ReturnsGitStderr; internal/cli/lock_test.go — TestLockAcquire_JSON_CarriesTokenAndBranch
**Tests:** gitx test on a real temp repo (ADR 0007), with an unborn and a detached-HEAD case.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/gitx/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run Lock`
**Commit:** `refactor(gitx): add CurrentBranch`

## 30. lock.CurrentClaim — ✅ DONE (2026-10-12)

**What:**
**Goal:** `lock.CurrentClaim(worktreeRoot, purpose, holder)` builds the claim (branch, user, host);
`capcom lock acquire` uses it and cli holds no claim detection.
**Approach (assumed at the header base):** move `newClaim`, `currentUser`, `userVariables` from
`internal/cli/lock.go` into `internal/lock/claim.go`. Depends on item 29.
**Regression guard.** lock/claim.go calls gitx.CurrentBranch from item 29 (lock imports gitx; no cycle).
**Files:** internal/lock/claim.go, internal/lock/claim_test.go, internal/cli/lock.go
**Read first:** internal/cli/lock.go — newClaim, currentUser, userVariables, newLockAcquireCommand; internal/lock/lock.go — Claim, Acquire;
internal/lock/lock_test.go — testClaim; internal/gitx/gitx_test.go — newRepo (pattern for a hermetic temp repo; lock has no git helper)
**Tests:** claim test on a temp repo; existing lock CLI tests.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/lock/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run Lock`
**Commit:** `refactor(lock): move claim detection into lock`

## 31. store.RaiseFormat — ✅ DONE (2026-10-12)

NOTES (2026-10-12): re-derived from "export item 2's byte-exact atomic writer": item 2 already exported it as `tree.WriteFileAtomicVerbatim`, so cli's `replaceFileBytes` moved into tree as `tree.ReplaceFileVerbatim` (stat the old file, then `WriteFileAtomicVerbatim` with its mode), not a second writer; merge_driver.go and init.go call it, and `mergedTempPattern` went with `replaceFileBytes` (the merge's temp file is now tree's `.<name>.tmp-*`).
NOTES (2026-10-12): `store.Store` gains `ConfigPath()` beside `IssuesDir`/`KnowledgeDir`, so cli derives the `.capcom/config.yaml` warning and `error.path` without `storeConfigFile`; store returns raw errors and cli maps every one to `CodeInvalid` at that path.
NOTES (2026-10-12): the failed-raise test is `TestRelatedAdd_RaiseFails_FailsInvalidAtTheConfig` in internal/cli/related_test.go (not in the item's Files list); no `internal/cli/doc.go` file-map line named a moved function, so doc.go is unchanged.

**What:** Recast at the regression check (2026-10-12).
**Goal:** `(*store.Store).RaiseFormat(n)` raises the store format (ADR 0009), writing every byte
`config.RaiseFormat` keeps as it returns them, with CRLF and mode kept; cli keeps only the warning text.
**Approach (assumed at the header base):** move `raiseStoreFormat` from `internal/cli/related.go`,
delete `storeConfigFile`; move cli's `replaceFileBytes` (`merge_driver.go`, used by `init.go`) into
tree for both. Depends on item 16.
**Regression guard.** Build on item 2's byte-exact atomic writer in internal/tree (export it) instead of
adding a second writer in replace.go; Depends on items 2 and 16. cli maps every `RaiseFormat` failure to
`CodeInvalid` with Path `.capcom/config.yaml` (or store returns a typed error carrying the relative
path), keeping today's `error.path`. The store test skips the mode assertion on `runtime.GOOS ==
"windows"`, as `internal/cli/init_test.go` does, and checks the CRLF bytes on every platform.
Never adjust `config.RaiseFormat` to fit the Goal (it also rewrites the format-1 template comment and
prepends to keyless content). Only the write can fail: the failed-raise test chmods the `.capcom` dir to
0o555 after setup, skips on `runtime.GOOS == "windows"` and when `os.Geteuid() == 0`, and asserts
`error.path == ".capcom/config.yaml"`. Fix every `internal/cli/doc.go` file-map line naming a moved function.
**Files:** internal/store/store.go, internal/store/store_test.go, internal/tree/atomic.go, internal/cli/related.go, internal/cli/merge_driver.go, internal/cli/init.go, internal/cli/doc.go
**Read first:** internal/cli/related.go — raiseStoreFormat, storeConfigFile, formatRaisedWarning, newRelatedAddCommand; internal/cli/merge_driver.go — replaceFileBytes, mergedTempPattern, mergeIssueFile; internal/cli/init.go — addMergeAttribute;
internal/tree/atomic.go — WriteFileAtomic, fillTemp; internal/config/config.go — RaiseFormat; internal/store/store.go — Store, relative, loadConfig;
internal/cli/related_test.go — TestRelatedAdd_KeylessConfig_RaisesTheFormatOnceWithAWarning, TestRelatedAdd_MissingConfig_CreatesItAtFormatTwo; internal/cli/init_test.go — TestInitMergeDriver_CRLFGitattributes_StayCRLFAndKeepTheirMode
**Tests:** store test raising a CRLF config keeps CRLF (every platform) and mode (skipped on Windows),
and a format-1 template config keeps every byte `config.RaiseFormat` returns; existing related tests, a
failed raise (`.capcom` dir 0o555; skipped on Windows and as root) whose `error.path` is still `.capcom/config.yaml`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/store/ ./internal/tree/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Related|Init|Merge'`
**Commit:** `refactor(store): own the format raise`

## 32. agents.Sync — ✅ DONE (2026-10-12)

NOTES (2026-10-12): refreshAgentsBlock (internal/cli/agents.go) is deleted rather than rewritten: its three callers (capcom agents, refreshAgentsAfterWrite, and --check through checkAgentsBlock) call agents.Sync with ModeWrite / ModeRefresh / ModeInspect directly; Inspect and Write stay exported and now share Sync's one-read reconcile core.

**What:**
**Goal:** `agents.Sync` (inspect, refresh-if-present, write) is the one way to reconcile the AGENTS.md
block; `capcom agents`, the know write verbs and `capcom check` use it with unchanged output.
**Approach (assumed at the header base):** replace `TopNotes` + `Inspect`/`Write` sequences in
`checkAgentsBlock`, `refreshAgentsBlock` (`internal/cli/agents.go`) and `agentsBlock`
(`internal/check/check.go`). Depends on item 15.
**Files:** internal/agents/block.go, internal/agents/block_test.go, internal/cli/agents.go, internal/cli/know.go, internal/check/check.go
**Read first:** internal/cli/agents.go — refreshAgentsBlock, checkAgentsBlock; internal/cli/know.go — refreshAgentsAfterWrite (callers in know_mv.go, know_update.go, know_rm.go);
internal/check/check.go — checker.agentsBlock; internal/agents/block.go — TopNotes, Inspect, Write; internal/agents/block_test.go — TestInspect_States
**Tests:** agents tests per mode; existing agents, know and check tests.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/agents/ ./internal/check/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Agents|Know'`
**Commit:** `refactor(agents): add Sync`

## 33. release: repo seam, exec and in-memory adapters, gate migrated — ✅ DONE (2026-10-12)

NOTES (2026-10-12): revParse and isAncestor are deleted, not re-typed to take repo: their bodies became execRepo.Resolve and execRepo.IsAncestor (exitCoder moved to repo.go with them), and the publish callers (convergeTag, verifyTag) call publishRun.repo directly; hasGreenRun's JSON parsing became execRepo.HasGreenRun.
NOTES (2026-10-12): the factory is a repoOpener (func(dir string) repo) passed to newRootCommand and every verb constructor (status, prepare, publish, smoke) and stored on their run options as openRepo; a nil opener — run options built by hand in tests that call runStatus/runPublish directly — opens execRepo over the run's runner, so those tests keep driving the gate through fakeRepo argv. prepare's and smoke's openRepo are wired but unread until items 34 and 38; convergeSmoke forwards publish's opener into the smoke options.
NOTES (2026-10-12): the gate's fetch and `gh run list` now run with nonInteractiveEnv (they ran with none before). `gh run list` gets no `--repo`: it is not a `gh release` call, and execRepo makes no gh release call yet.
NOTES (2026-10-12): fakeCommit.changed is now a []string shared by fakeRepo (which joins it into diff-tree output) and memRepo; the diff-tree argv assertions that were in TestGatePeelsOnlyTrailingChangelogAndVersionCommits moved to repo_test.go, and memRepo refuses ChangedPaths for a merge, which keeps the "merge never asked about" pin.

**What:** Recast at the regression check (2026-10-12).
**Goal:** `cmd/release` has a `repo` interface for the capcom checkout and GitHub repo; `execRepo`
owns their git/gh argv and exit-code/not-found classification and names `--repo airiclenz/capcom` on
every gh release call; a test-only `memRepo` holds semantic state; gate and its tests use them.
**Approach (assumed at the header base):** methods from the gate calls first (`FetchMain`, `IsClean`,
`Resolve`, `Parents`, `ChangedPaths`, `IsAncestor`, `HasGreenRun`); `checkGate`, `peelBumps`,
`revParse`, `isAncestor` take `repo`; `runner` stays for non-repo tools; argv tests move to
`repo_test.go` over `fakeRunner`.
**Regression guard.** Item 33 alone owns the repo factory — it adds the injectable factory to
newRootCommand/execute in cmd/release/main.go and makes it reachable from every verb's run options; gate
uses it; later items only read the repo from those options in their own verb files and never edit
main.go. execRepo runs every remote call (fetch, push, ls-remote, every gh
call) with nonInteractiveEnv; items 33–38 each keep the shared argv fakes (fakeRepo in gate_test.go,
fakeRemote in probes_test.go) compiling and in step with execRepo's argv until item 39 retires them;
only the gate's own tests move to memRepo. `smoke.go`'s `checkBuildStamps` call of `revParse` wraps
`r.commands` in execRepo (smoke's migration stays in item 38). `--repo airiclenz/capcom` binds every gh
release call execRepo makes, not status's `gh release view`, which stays on the runner until item 35.
**Files:** cmd/release/repo.go, cmd/release/repo_test.go, cmd/release/memrepo_test.go, cmd/release/gate.go, cmd/release/gate_test.go, cmd/release/status.go, cmd/release/publish.go, cmd/release/smoke.go, cmd/release/main.go, cmd/release/main_test.go, cmd/release/prepare.go
**Read first:** cmd/release/gate.go — checkGate, peelBumps, revParse, isAncestor, hasGreenRun; cmd/release/gate_test.go — fakeRepo.respond, TestGateNamesACheckThatCannotRun, TestGatePeelsOnlyTrailingChangelogAndVersionCommits;
cmd/release/main.go — newRootCommand; cmd/release/main_test.go — fakeRunner, execute; cmd/release/status.go — runStatus; cmd/release/publish.go — runPublish, checkPushable, convergeTag, verifyTag;
cmd/release/smoke.go — nonInteractiveEnv, smokeRun.checkBuildStamps
**Tests:** gate tests over memRepo; adapter argv tests, nonInteractiveEnv on every remote call included;
an execute-path test through the injected repo factory.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./cmd/release/`; `go vet ./cmd/release/`
**Commit:** `refactor(release): repo seam for the gate`

## 34. release: prepare on the repo seam — ✅ DONE (2026-10-12)

NOTES (2026-10-12): CommitFile owns the whole write-stage-commit step: prepare.go's commitFile helper is deleted and its body (mode-keeping write, `git add <path>`, `git commit --quiet -m`, same error wording) became execRepo.CommitFile. prepare opens the repo with options.openRepo.open(commands, options.dir) and still reads CHANGELOG.md and VERSION from disk itself.
NOTES (2026-10-12): ReleaseTags returns []string; highestReleaseTag keeps its string signature (status.go and version_test.go still call it, item 35's scope), so prepare passes it strings.Join(tags, "\n").
NOTES (2026-10-12): memRepo keeps value receivers, so gate_test.go and main_test.go are untouched: it gains tags and a madeCommits *[]memCommit log that CommitFile appends {subject, path, content} to (a memRepo without a log refuses CommitFile). Since memRepo writes nothing to disk, the prepare tests pin each commit's one path and content rather than both files as they stood at the commit; the failed-bump test wraps memRepo (bumpRefusingRepo) to fail only the bump. The argv for ReleaseTags and CommitFile, and CommitFile's write/stage/commit failures, are pinned in repo_test.go; TestPrepareRefusalIsARunFailure runs through executeWith with the memRepo opener and a runner that refuses every command, and asserts it is never called. A new refusal case covers an IsClean failure.

**What:** Recast at the regression check (2026-10-12).
**Goal:** prepare's git calls (`ReleaseTags`, `IsClean`, `CommitFile`) go through `repo`; its tests
use memRepo. Depends on item 33.
**Approach (assumed at the header base):** extend `repo`/`execRepo`/`memRepo`.
**Regression guard.** Item 34 never edits cmd/release/main.go — prepare.go reads the repo from the run
options item 33 added; its Goal lists exactly ReleaseTags, IsClean and CommitFile. prepare keeps reading
CHANGELOG.md and VERSION from disk.
**Files:** cmd/release/prepare.go, cmd/release/prepare_test.go, cmd/release/repo.go, cmd/release/repo_test.go, cmd/release/memrepo_test.go
**Read first:** cmd/release/prepare.go — runPrepare, checkPrepare, commitFile, newPrepareCommand; cmd/release/prepare_test.go — prepareRepo.respond, gitCommands, TestPrepareMakesTheRollupThenTheBumpCommit, TestPrepareRefusalIsARunFailure;
cmd/release/main.go — newRootCommand; cmd/release/main_test.go — execute
**Tests:** prepare tests over memRepo.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./cmd/release/`
**Commit:** `refactor(release): prepare on the repo seam`

## 35. release: status and own-repo probes on the repo seam — ✅ DONE (2026-10-12)

NOTES (2026-10-12): repo gains CountCommits, LatestRelease (returns a new githubRelease{Tag, Published} plus an isPublished bool; named githubRelease because smoke_test.go already declares a publishedRelease helper), RemoteHasTag, RemoteTagType and ReleaseAssets. readReleaseAssets is deleted, not re-typed: its body became execRepo.ReleaseAssets, and probeRelease and publish.go's convergeRelease call the repo directly. readBaseline/countBaseline take the repo; the gh-output parsing and the "named no latest release" check moved into execRepo.LatestRelease. highestReleaseTag keeps its string signature (version.go is not in Files), so status passes it strings.Join(tags, "\n") the way prepare does.
NOTES (2026-10-12): stepProbe is now func(ctx, probeEnv, version), and probeEnv is {repo, commands, dir}: tag and release read through repo, while homebrew, scoop and winget stay on commands/dir (the tap/bucket/winget seam is out of scope). Behaviour change from the seam's own rules: the probes' ls-remote, tag-type `gh api` and `gh release view`, and status's `gh release view`, now run with nonInteractiveEnv (before, none), and both `gh release view` calls name `--repo airiclenz/capcom`. fakeRepo's latest-release match and fakeRemote's assets match learned the `--repo` argv; fakeRepo's release/tags/count cases now serve no test but stay until item 39. Status tests run over memRepo (count map, latest release, tags) plus fakeRemote for the channel probes; the "gh cannot run" case became separate LatestRelease / ReleaseTags / CountCommits failures, and the no-probe case now asserts the runner is never called. The half-uploaded-asset case moved from the probe table to execRepo's ReleaseAssets argv test; a probe case for a tag of another object type was added.

**What:**
**Goal:** status (`ReleaseTags`, `CountCommits`, `LatestRelease`) and the own-repo probes
(`RemoteHasTag`, `RemoteTagType`, `ReleaseAssets`) go through `repo`; their tests use memRepo.
**Approach (assumed at the header base):** probe steps take an env holding `repo` and `runner`.
Depends on item 34.
**Regression guard.** Add cmd/release/publish.go to Files — repoint publish.go's callers of the probe steps
and readReleaseAssets to the new signature. Once execRepo names `--repo airiclenz/capcom` on `gh release
view`, both argv matchers (fakeRepo's exact match in `gate_test.go`, fakeRemote's assets match in
`probes_test.go`) learn the `--repo` argv and keep it until item 39.
**Files:** cmd/release/status.go, cmd/release/status_test.go, cmd/release/probes.go, cmd/release/probes_test.go, cmd/release/repo.go, cmd/release/repo_test.go, cmd/release/memrepo_test.go, cmd/release/publish.go, cmd/release/gate_test.go
**Read first:** cmd/release/probes.go — stepProbe, probeTag, probeRelease, readReleaseAssets; cmd/release/publish.go — publishRun.probe;
cmd/release/probes_test.go — fakeRemote.respond; cmd/release/gate_test.go — fakeRepo.respond; cmd/release/status.go — readBaseline
**Tests:** status and probe tests over memRepo.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./cmd/release/`
**Commit:** `refactor(release): status and probes on the repo seam`

## 36. release: publish readiness and tag steps on the repo seam — ✅ DONE (2026-10-12)

NOTES (2026-10-12): repo gains PushMain, FetchTag, IsAnnotated (bool: the tag ref's object is a tag object), ShowFile(revision, path) and LastCommitTouching(revision, path); checkPushable, convergeTag, waitForTag and verifyTag call them, with Resolve, IsAncestor and RemoteHasTag, and publish.go keeps no own-repo push/ls-remote/cat-file/show/log argv (the release step's fetch --tags and rev-parse --show-toplevel stay for item 37). Behaviour change from the seam's own rules: LastCommitTouching fails ("git log found no commit changing VERSION at HEAD") where verifyTag used to compare against an empty commit id.
NOTES (2026-10-12): publishFake now holds a memRepo (repo) plus a fakeRemote (remote) for the channel repositories; fake.repo.remote.* became fake.remote.*, tagObject/release became memRepo remoteTags/releases (a tagged() helper), headVersion became memRepo files[VERSION]. memRepo gains files and a shared *memPublish: PushMain/FetchTag are appended to the fakeRunner's own call record as `repo PushMain` / `repo FetchTag <tag>` (memPublish locks the runner's mu directly, so main_test.go is untouched), and mutatingCommands lists both; a post-push hook (tagAfterPush, driven by tagAppearsAfter/tagKind/tagCommit) makes the tag on the Nth RemoteHasTag poll after the push; Resolve and IsAnnotated know a tag only once FetchTag fetched it; origin/main resolves to HEAD after the push.
NOTES (2026-10-12): the first-run test's "fetched before resolved" line-order check was dropped: memRepo refuses to resolve an unfetched tag, so a run resolving first fails. The two "converges from remote state" tests no longer look for `git status`/`gh run list`/`git show HEAD:VERSION` lines (those are repo calls now); they keep `repo PushMain`/`git push` absence, and their dirty tree plus moved-on VERSION would refuse any run that consulted the gate or VERSION.
NOTES (2026-10-12): consequential edit — cmd/release/gate_test.go: made necessary by publishFake moving to memRepo (fakeRepo's comment said the publish tests still drive it; it now serves no test and stays until item 39, as do releaseJSON and fakeRemote's tag/release branches).

**What:** Recast at the regression check (2026-10-12).
**Goal:** publish's readiness and tag steps (`Resolve`, `IsAncestor`, `PushMain`, `RemoteHasTag`,
`FetchTag`, `IsAnnotated`, `ShowFile`, `LastCommitTouching`) go through `repo`; their tests use memRepo.
Depends on item 35.
**Approach (assumed at the header base):** `publishFake` wires memRepo for repo calls and keeps argv
fakes for channel calls.
**Regression guard.** The Goal lists Resolve, IsAncestor, PushMain, RemoteHasTag, FetchTag, IsAnnotated,
ShowFile and LastCommitTouching (publish.go); keep the shared argv fakes in step (see 33). memRepo appends PushMain/FetchTag to the same ordered log as
fakeRunner (`mutatingLines` reads both), `Resolve(tag)` fails until FetchTag, and a post-PushMain hook
says when the tag appears (RemoteHasTag polls, then its kind and commit), so `TestPublishTagStepFailures`
keeps its pause counts. `TestPublishFirstRunConvergesTheTagThenTheRelease` and the no-push checks at
publish_test.go:389,448,516 and channels_test.go:238,620,690 are recast on that log. A `repo_test.go`
argv test pins nonInteractiveEnv. Scope: every hit of `grep -n 'fake\.repo\.' cmd/release/*_test.go`.
**Files:** cmd/release/publish.go, cmd/release/publish_test.go, cmd/release/channels_test.go, cmd/release/repo.go, cmd/release/repo_test.go, cmd/release/memrepo_test.go, cmd/release/smoke_test.go
**Read first:** cmd/release/publish.go — checkPushable, convergeTag, waitForTag, verifyTag; cmd/release/publish_test.go — publishFake.respond, hasRemoteTag, mutatingLines, TestPublishTagStepFailures, TestPublishConvergesFromRemoteStateOnceTheTagExists;
cmd/release/channels_test.go — releasedFake, TestPublishWingetConvergesFromRemoteStateOnceTheTagExists; cmd/release/smoke_test.go — TestPublishSmokeStepWithoutABuild; cmd/release/probes.go — probeTag
**Tests:** publish readiness/tag tests over memRepo, on the shared ordered log; an execRepo argv test of
nonInteractiveEnv on the remote calls.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./cmd/release/`
**Commit:** `refactor(release): publish tag steps on the repo seam`

## 37. release: publish release step on the repo seam — ✅ DONE (2026-10-12)

NOTES (2026-10-12): repo gains CloneAt(tag, dir) (fetch --tags, rev-parse --show-toplevel, clone from dir's parent, checkout; the old error wording kept), CreateRelease(tag, notesPath, assets), UploadAssets(tag, assets) and ReadAsset(tag, name); convergeRelease, buildReleaseAssets and downloadReleaseSums call them, so publish.go keeps no git or gh release argv and channels.go no release download. Following the ratified "every gh release call names --repo airiclenz/capcom", create, upload and download now pass `--repo airiclenz/capcom` after the tag (same position as smoke's download).
NOTES (2026-10-12): memRepo records CloneAt/CreateRelease/UploadAssets/ReadAsset in the shared ordered log (`repo <Method> <args>`), CloneAt writes writeReleaseTree into dir and requires origin to hold the tag, CreateRelease/UploadAssets stat every asset via requireFiles and add the asset names to releases, and ReadAsset reads the new assetContent map (which replaces publishFake.publishedSums). mutatingCommands swaps the own-repo argv prefixes (git fetch --quiet --tags, git clone, git checkout, gh release create/upload) for the three mutating repo methods; publishFake.respond drops their argv cases.
NOTES (2026-10-12): TestPublishCreatesTheReleaseWithTheTagsArgv became the execRepo "CreateRelease" table case (--verify-tag/--title/--notes-file/--latest/--repo), its asset-order pin moved into TestPublishFirstRunConvergesTheTagThenTheRelease (the created release's assets in release order); TestPublishDownloadsTheSumsOutsideTheCheckout became the "ReadAsset" case (--output -, --repo); "UploadAssets" pins --clobber; new TestExecRepoCloneAtFetchesThenClonesTheTag / TestExecRepoCloneAtNamesTheStepThatFailed pin CloneAt's four calls and failures. publishToplevel now serves only the CloneAt test (comment updated).
NOTES (2026-10-12): consequential edit — cmd/release/channels_test.go: made necessary by downloadReleaseSums reading through run.repo (TestPublishWritesTheRenderedFileIntoTheClone builds its publishRun by hand and now sets repo: fake.repo).
NOTES (2026-10-12): retry fix — TestExecRepoCloneAtFetchesThenClonesTheTag clones into filepath.Join(t.TempDir(), "src") and expects the clone to run in that temp dir, so the expected dirs match filepath.Dir on Windows too.

**What:**
**Goal:** the release step (`CloneAt`, `CreateRelease`, `UploadAssets`, `ReadAsset`) and the sums
download in channels go through `repo`; `TestPublishCreatesTheReleaseWithTheTagsArgv` becomes an
execRepo test. Depends on item 36.
**Approach (assumed at the header base):** extend the adapters.
**Regression guard.** `TestPublishDownloadsTheSumsOutsideTheCheckout`'s `--output -` pin (stdout, never a file)
moves, with `--repo`, into a `repo_test.go` test of execRepo's ReadAsset; execRepo tests pin `--clobber`
(UploadAssets) and `--verify-tag`/`--title`/`--latest` (CreateRelease). memRepo records
CloneAt/CreateRelease/UploadAssets/ReadAsset in the shared ordered log, stats each asset path as
`requireFiles` does, and CloneAt writes `writeReleaseTree`. The Goal's argv test is checked by the
greps in Acceptance. After item 37 no capcom-repo git argv remains in publish.go: Acceptance adds
`! grep -nE '"git", "(push|ls-remote|fetch|cat-file|show|log)"' cmd/release/publish.go`.
**Files:** cmd/release/publish.go, cmd/release/publish_test.go, cmd/release/channels.go, cmd/release/channels_test.go, cmd/release/repo.go, cmd/release/repo_test.go, cmd/release/memrepo_test.go
**Read first:** cmd/release/publish.go — convergeRelease, buildReleaseAssets, checkPublishedSums; cmd/release/channels.go — downloadReleaseSums;
cmd/release/publish_test.go — requireFiles, TestPublishPartialReleaseUploadsOnlyTheMissingAssets;
cmd/release/channels_test.go — TestPublishDownloadsTheSumsOutsideTheCheckout; cmd/release/dist.go — runDist
**Tests:** release-step tests over memRepo (`TestPublishPartialReleaseUploadsOnlyTheMissingAssets`, the
clone/upload absence checks and the wantOrder tail recast on the shared log); execRepo argv tests of
ReadAsset, UploadAssets and CreateRelease.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./cmd/release/`; `! grep -nE '"git"|"gh", "release"' cmd/release/publish.go`; `! grep -n '"release", "download"' cmd/release/channels.go`; `! grep -nE '"git", "(push|ls-remote|fetch|cat-file|show|log)"' cmd/release/publish.go`
**Commit:** `refactor(release): publish release step on the repo seam`

## 38. release: smoke on the repo seam — ✅ DONE (2026-10-12)

NOTES (2026-10-12): repo gains DownloadAsset(tag, name, dir), which execRepo runs as `gh release download <tag> --repo airiclenz/capcom --pattern <name> --dir <dir>` (the same argv as before, with nonInteractiveEnv). smokeRun holds a repo opened in runSmoke with options.openRepo.open(commands, options.dir), as runPublish does. checkTag uses RemoteHasTag and RemoteTagType, checkAssets uses DownloadAsset, checkBuildStamps uses Resolve, publishedSums uses ReadAsset and checkStaleBucket uses LatestRelease, so smoke.go has no own-repo git or gh argv left; only the bucket read (readRepoFile), brew and the released binaries still go through the runner.
NOTES (2026-10-12): re-derived from "newPublishCommand's smoke closure follows runSmoke's new signature": runSmoke's signature stays the same, because item 33 already put openRepo on smokeOptions and convergeSmoke already passes publish's opener on, so publish.go is not edited.
NOTES (2026-10-12): behaviour change: checkStaleBucket asks LatestRelease (`gh release view --repo … --json tagName,publishedAt`) instead of its own `--jq .tagName` argv. When no release is published it now skips with "…, and airiclenz/capcom has no published release" instead of "could not be read: <nil>"; a LatestRelease error still skips with "could not be read: <err>".
NOTES (2026-10-12): memRepo gains tagCommits (the checkout's tags → commit, read by resolve() so Resolve knows a tag that was fetched locally), failures (per-method injected errors, checked after the existing failing name), and DownloadAsset (writes assetContent[name] to dir/name; the release must carry the asset; it is noted in the publish log when one exists). smokeFake now holds a memRepo built by publishedRepo (every asset in releaseDir with its content, the annotated tag on origin and in the checkout, the latest release). Its respond keeps only the brew, bucket and binary argv; tagKind, isTagLocal, corrupt and latest are now memRepo state, and the smokeListTag/TagType/DownloadSum/Latest/RevParse constants and download helper are gone.
NOTES (2026-10-12): TestSmokeVersion's command-line cases and TestSmokeFailureIsARunFailure run through executeWith with the fake's opener. The command-line check that the "first git command lists the tag" became a PASS "annotated tag v0.24.11 on origin" check (the memRepo holds only that tag). The tag-less run now asserts FAIL "origin has no tag v0.24.11". "the release has no SHA256SUMS" removes the asset from the release instead of failing a command. "gh is not installed" fails RemoteTagType and ReadAsset with an error wrapping exec.ErrNotFound. A new "no release is published" case was added, and every case now asserts the output never contains "<nil>". New repo_test.go coverage: a "DownloadAsset" argv case, and TestExecRepoKeepsAMissingToolDetectable, which checks that isMissingTool still recognises exec.ErrNotFound through RemoteHasTag, RemoteTagType, Resolve, LatestRelease, ReadAsset and DownloadAsset.

**What:**
**Goal:** smoke's own-repo calls (`Resolve`, `LatestRelease`, `RemoteTagType`, `ReadAsset`,
`DownloadAsset`, `RemoteHasTag`) go through `repo`; smoke tests use memRepo for them. Depends on item 37.
**Approach (assumed at the header base):** `smokeFake.respond` keeps only channel and binary argv.
**Regression guard.** `newPublishCommand`'s smoke closure (publish.go) follows `runSmoke`'s new signature.
`TestSmokeVersion` and `TestSmokeFailureIsARunFailure` reach memRepo through item 33's repo factory; the
tag-less run asserts FAIL `origin has no tag`. execRepo keeps `%w` on runner errors so `isMissingTool`
still sees `exec.ErrNotFound`; memRepo takes per-method injected errors, and the `gh is not installed`
case wraps `exec.ErrNotFound`. Every origin/GitHub call runs with nonInteractiveEnv (item 33). When
LatestRelease reports no release, `checkStaleBucket`'s skip says so instead of printing a nil error.
**Files:** cmd/release/smoke.go, cmd/release/smoke_test.go, cmd/release/repo.go, cmd/release/repo_test.go, cmd/release/memrepo_test.go, cmd/release/publish.go
**Read first:** cmd/release/smoke.go — runSmoke, checkTag, publishedSums, checkStaleBucket, isMissingTool;
cmd/release/smoke_test.go — TestSmokeVersion, TestSmokeFailureIsARunFailure; cmd/release/publish.go — newPublishCommand
**Tests:** smoke tests over memRepo; `TestSmokeVersion` and `TestSmokeFailureIsARunFailure` through the
injected repo; the `gh is not installed` SKIP; a no-release stale-bucket SKIP without `<nil>`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./cmd/release/`
**Commit:** `refactor(release): smoke on the repo seam`

## 39. release: retire own-repo argv fakes and record the seam — ✅ DONE (2026-10-12)

NOTES (2026-10-12): gate_test.go loses fakeRepo with its runner, respond, resolve and reaches, none of which any test used any more. fakeCommit, linearHistory and exitStatus stay because memRepo, smoke_test.go and repo_test.go use them. probes_test.go's fakeRemote loses its tagObject and release fields and its ls-remote, `gh api …/git/ref/tags/` and `gh release view … assets` branches, so it now answers only the tap, bucket and winget-pkgs gh commands. Its doc comment says so.
NOTES (2026-10-12): consequential edit — cmd/release/probes_test.go: the releaseJSON helper was deleted, because removing fakeRemote.release (and item 36's earlier move of the release probe to memRepo) left it with no caller.
NOTES (2026-10-12): status_test.go, publish_test.go, channels_test.go and smoke_test.go are named in Files but needed no edit. They set no own-repo fakeRemote field, and their only own-repo argv literals are the exempt `git log -1 --format=%ct HEAD` and negative must-not-run assertions (main_test.go's gateCommands, publish_test.go's mutatingCommands), which fake nothing.
NOTES (2026-10-12): ADR 0008's scoped-exception paragraph was rewritten in place to describe two seams. The repo seam (execRepo over git/gh, every `gh release` call naming `--repo airiclenz/capcom`, verb tests on memRepo, argv pinned only in execRepo's own tests) covers capcom's own repository. The runner seam covers the toolchain, dist's commit-time read and the channel repos. A dated "Amended 2026-10-12" paragraph follows it, in the form ADRs 0002 and 0004 use. The Note was updated with `capcom know update release-process --body-replace-old/--body-replace-new`. The main.go package doc and the runner.go runner doc now say the same.

**What:**
**Goal:** no `cmd/release` test fakes a capcom-repo git/gh call by argv (`fakeRepo.respond` and the
own-repo branches of `fakeRemote.respond` are gone); ADR 0008's test-seam section, the
`release-process` Note and the `cmd/release/main.go` doc state the repo seam. Depends on item 38.
**Approach (assumed at the header base):** delete dead fake branches; amend ADR 0008 in place;
`capcom know update release-process` for the Note.
**Regression guard.** repo_test.go's execRepo argv tests and dist's commit-time read (`git log -1
--format=%ct HEAD`, answered in publish_test.go and dist_test.go's `fakeToolchain`) are exempt; the Goal
is checked as `! grep -n 'fakeRepo' cmd/release/*_test.go` and no own-repo argv literal outside
repo_test.go. Scope: every file hit by `grep -ln 'fakeRepo\|tagObject\|remote\.release'
cmd/release/*_test.go`, and every hit of `grep -rn -i 'seam' cmd/release/*.go docs/adr/0008-*.md
.capcom/knowledge/release-process.md`, `runner.go` included.
**Files:** cmd/release/gate_test.go, cmd/release/probes_test.go, cmd/release/main.go, docs/adr/0008-the-repo-owns-its-release-pipeline.md, .capcom/knowledge/release-process.md, cmd/release/runner.go, cmd/release/status_test.go, cmd/release/publish_test.go, cmd/release/channels_test.go, cmd/release/smoke_test.go
**Read first:** cmd/release/gate_test.go — fakeRepo, fakeRepo.respond; cmd/release/probes_test.go — fakeRemote.respond; cmd/release/status_test.go — clearedRepo;
cmd/release/publish_test.go — publishFake; docs/adr/0008-the-repo-owns-its-release-pipeline.md — scoped exception to ADR 0007;
cmd/release/main.go — package doc; cmd/release/runner.go — runner doc
**Tests:** `cmd/release` suite.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./cmd/release/`; `! grep -n 'fakeRepo' cmd/release/*_test.go`; `go run ./cmd/capcom check`
**Commit:** `docs(release): record the repo seam`
