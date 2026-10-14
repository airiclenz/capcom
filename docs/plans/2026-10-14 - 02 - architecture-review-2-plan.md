# Architecture review 2026-10-14, second pass: one Kind reader, one Lock overview

**Goal:** Land review candidate 3 (one exported descriptor and Kind reader per kind, every node-file reader routed through it) with the three defects it exposes, and candidate 5 (a Lock overview in `lock` that Briefing, `prime` and `lock status` read). Behaviour changes follow the ratified calls below; everything else is a pure refactor.
**Date:** 2026-10-14
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 40180c0
**Closes:** arch-review-1014b

**Sources:**
- `docs/reviews/architecture-review-2026-10-14.html` (ground truth; cards 3 and 5, "Top recommendation")
- `docs/reviews/archived/architecture-review-2026-10-14-0848.html` (cards 4 and 5: the original framing of candidate 3)
- Issues `arch-review-1014b`, `kind-reader`, `block-left-out-notes`, `format-key-rule`, `unread-warning-keys`, `lock-overview` (`capcom show <id>`)
- `AGENTS.md`; Notes `coding-conventions`, `error-messages`, `cli-test-harness`, `issue-workflow`, `store-layout`, `adr-0005`, `adr-0007`, `adr-0009`, `adr-0010`

**Ratified design calls (user, 2026-10-14):**
- **Scope:** candidates 3 and 5 only; issues filed under parent `arch-review-1014b`; `arch-review-1014` children untouched.
- **Managed block:** hides a top-level Note the Kind reader judges unusable, as `know tree` does.
- **Format-key rule:** any key line needs its format (empty lists, `retired: false` included); one table, read by codec and check.
- **Unread warning:** names every ref key of the kind ("deps and related not checked" for issues).
- **Noun:** messages say "Note"/"a Note"; "issue" stays lowercase. JSON kind values, flag, command and database names are unchanged.
- **Note in prose:** flag help, the Briefing's broken-refs line and know verify's summary say Note too; flag names, command names, JSON keys and kind values stay lowercase.
- **Lock overview:** a bad foreign record is one per-record problem; only an unlistable locks directory fails the overview.
- **Lock states:** `free · held · stale · invalid · unreadable`, one vocabulary for Briefing, `prime` and `lock status`; `lock status` on an invalid or unreadable own record prints the state, warns naming the file, exits 0; `lock check` exit codes unchanged.
- **Prime advice:** invalid and unreadable get their own `this worktree:` lines; `prime --json` data unchanged.
- **Glossary:** new term Notes "Kind reader" (`glossary-structure`) and "Lock overview" (`glossary-coordination`).
- **CHANGELOG:** one `Unreleased` line per user-visible change, in the item that makes it.

**Plan author calls (2026-10-14, no visible change):**
- **Usable verdict:** a file is usable when the reader reports no finding at all (refused read, parse, field or alias), matching the index (`indexFile` leaves out any file with problems) and `tree` (`claims.go`: `IsUsable && len(problems) == 0`).
- **diagnose:** gains no `const` (check's `TestCodes_ClosedSet_PinsEveryCodeString` parses every diagnose constant).
- **Unreadable foreign fixture:** `chmod 000`, skipped on Windows and as root (the `internal/store/store_test.go` pattern).

**Standing requirements:**
- skills: coding-standards
- Raspberry Pi 5: one package per test command with `GOMEMLIMIT=2GiB`; never `-race ./...`; never two test commands at once.
- Commit to `main`; one commit per item; no AI attribution anywhere. Never tag, bump `VERSION` or edit a CHANGELOG release heading.
- After any hand edit under `.capcom/`, run `capcom check`. Tree lock held with `capcom lock acquire --purpose arch-review-1014b`.
- Every `capcom` command in this plan runs a binary built from the tree, never the one on PATH.

**Out of scope:** review candidates 1, 2, 4 and 6–11; `tree.Load`/`Move.Apply`/`RecoverAliases` deletions; `membership-owner`, `know-tree-orphans`.

**Regression check (2026-10-14, 40180c0):**
- P0.1: recast (no `capcom agents` step, tree-built binary); yields to commit 50154fd; retitled, AGENTS.md dropped from the Goal (decision)
- P1.1: guard folded (nouns keep their values until P3.3)
- P1.2: guard folded
- P1.3: guard folded
- P1.4: guard folded
- P1.5: guard folded
- P1.6: guard folded
- P2.1: guard folded; supersedes the raise-on-gain wording of `docs/site/reference/file-format.md:160-164,576-585` and `internal/nodestore/edit.go:34-37`
- P3.1: guard folded; `nounPhrase` now stays until P3.4 (follows the P3.3 split)
- P3.2: guard folded
- P3.3: guard folded; recast (decision): split, keeps nodestore, diagnose and index
- P3.4: new (decision): check and cli literals, `nodeVerbs.nounPhrase` removed
- P3.5: new (decision): Note in flag help, the Briefing and know verify; closes `unread-warning-keys`
- P4.1: guard folded
- P4.2: guard folded
- P4.3: guard folded
- P3.2 (re-check): guard folded (decision): diagnose's Note-wording sites retargeted to `nodestore/read.go`/`read_test.go` (moved there by P1.4)
- P3.3 (re-check): recast (decision): Note decode in `nodestore/read.go`, pin in `read_test.go`; check's alias message moves to P3.4; supersedes the `the note's own id` quotes on CHANGELOG.md:16, reworded in P3.5
- P3.4 (re-check): guard folded (repins check_test.go:859's `loadTree` noun; rule exempts P3.5's summaries and kind/JSON values; `nounPhrase` grep and new message assertions)
- P3.5 (re-check): guard folded (help goldens `know-mv.txt`/`know-add.txt`; CHANGELOG.md:16 quotes reworded; flag-help grep in Acceptance)

## P0.1 Commit the review, the issues and the plan — ✅ ALREADY-DONE (eb617be)

**What:** Recast at the regression check (2026-10-14).
**Goal:** the second review, the archived first review, the `arch-review-1014b` issues and this plan are committed; `capcom check` reports no problem.
**Approach (assumed at the header base):** `git add` the untracked `docs/reviews/architecture-review-2026-10-14.html`, the move of `architecture-review-2026-10-14-0848.html` into `docs/reviews/archived/`, `.capcom/issues/arch-review-1014b.md` and its directory, and this plan.
**Regression guard.** Drop the `capcom agents` step and AGENTS.md from Files: the "block is stale" report came from the PATH capcom (Homebrew 0.6.0, older than the base), and a tree-built binary reports check clean. Acceptance runs a binary built from the tree (`go build -o <scratchpad>/capcom ./cmd/capcom`, then `<scratchpad>/capcom check` exits 0). The item yields to commit 50154fd (the Tree lock paragraph, `internal/agents/block.go:67`), which the old binary's `capcom agents` would strip from AGENTS.md.
**Files:** docs/reviews/architecture-review-2026-10-14.html; docs/reviews/archived/architecture-review-2026-10-14-0848.html; docs/reviews/architecture-review-2026-10-14-0848.html; .capcom/issues/arch-review-1014b.md; .capcom/issues/arch-review-1014b/; docs/plans/2026-10-14 - 02 - architecture-review-2-plan.md
**Read first:** AGENTS.md — capcom block (BEGIN/END CAPCOM markers); internal/agents/block.go — Tree lock paragraph; .capcom/issues/arch-review-1014b.md — plan key;
docs/reviews/archived/architecture-review-2026-10-14-0848.html — staged rename target
**Tests:** none (docs and issue files only).
**Acceptance:** `go build -o <scratchpad>/capcom ./cmd/capcom`, then `<scratchpad>/capcom check` exits 0; `git status --short docs/reviews .capcom AGENTS.md` prints nothing.
**Commit:** `chore(issues): file the second architecture review's issues`

## P1.1 Export a descriptor per kind — ✅ DONE (2026-10-14)

NOTES (2026-10-14): the Descriptor also carries Validate (bound to cfg for issues): it is the only cfg-dependent part of a kind, so it is what "issues from a config.Config" builds; Codec.Validate is taken from it. Behaviour unchanged.

NOTES (2026-10-14): ref fields (key with get/set) moved from codec.go into the Descriptor's unexported refs, exposed as Descriptor.RefKeys(); Codec.refs is filled from it, so codec.go holds no noun or ref-key literal. The Ref* constants stay in refs.go; refs.go only gains doc lines pointing at the Descriptor.

NOTES (2026-10-14): `go build ./...` fails on pre-existing gitignored scratch Go files under docs/skill-runs/release-notes-capcom/2026-10-14/scratch/ (old snapshots of internal/); `go build ./cmd/... ./internal/...` is clean. CI clones would not see them.

**What:**
**Goal:** nodestore exports one descriptor per kind, built without a store (issues from a `config.Config`, Notes from nothing): its noun and noun phrase, its ref keys (`RefDeps`, `RefRelated`, `RefSupersededBy`) and a table of key → format needed → line accessor. `Codec` takes its noun, noun phrase and ref keys from it. Output unchanged (nouns keep their current values until P3.3).
**Approach (assumed at the header base):** a new `internal/nodestore/descriptor.go`; `issueCodec`/`noteCodec` and `refField` read it. The format table lists `related` → `config.FormatRelated` (`issue.Issue.RelatedLine`), `retired` and `superseded-by` → `config.FormatSupersession` (`knowledge.Note.RetiredLine`, `SupersededByKeyLine`). `requiredFormat` keeps its gained-entry semantics here (P2.1 changes it).
**Regression guard.** The nouns keep their current values until P3.3 (not P3.2). Keep `Codec.Noun`, `Codec.refs` and the `issueCodec(cfg)`/`noteCodec()` signatures, filled from the descriptor, so `kind_test.go` (`codec.Noun`) and `refs_test.go` (`issueCodec(cfg).refs`, `noteCodec().refs`) compile unchanged. Add `internal/nodestore/doc.go` to Files with a `descriptor.go` line in its file map (`TestDocFileMap_MatchesDirectory`). Acceptance checks codec.go holds no noun or ref-key literal of its own (its comments included).
**Files:** internal/nodestore/descriptor.go; internal/nodestore/descriptor_test.go; internal/nodestore/codec.go; internal/nodestore/refs.go; internal/nodestore/doc.go
**Read first:** internal/nodestore/codec.go — Codec, issueCodec, noteCodec, gainsSupersession, gainsEntry; internal/nodestore/refs.go — RefDeps, RefRelated, RefSupersededBy, refField; internal/nodestore/doc.go — file map;
internal/nodestore/refs_test.go — TestCodecs_ListTheirReferenceFields; internal/nodestore/kind_test.go — TestCodec_CanonicalFile_RoundTripsPerKind; internal/issue/issue.go — Issue.RelatedLine;
internal/knowledge/note.go — Note.RetiredLine, Note.SupersededByKeyLine; internal/config/config.go — FormatRelated, FormatSupersession
**Tests:** a table test per kind over the descriptor's noun, ref keys and format table.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodestore/`; `go vet ./internal/nodestore/`; `grep -n '"issue"\|"note"\|"an issue"\|"a note"\|key: *Ref' internal/nodestore/codec.go` prints nothing
**Commit:** `refactor(nodestore): export a descriptor per kind`

## P1.2 The Kind reader — ✅ DONE (2026-10-14)

NOTES (2026-10-14): the reader is `Reader[T]`, built by `IssueReader(cfg)` / `NoteReader()` from what builds the kind's Descriptor (the config), not from a `Descriptor` value: until P1.4 moves the verdict in, it decodes through `diagnose.IssueFile`/`NoteFile`, which take neither the Descriptor's noun nor its Validate.

NOTES (2026-10-14): the result type is `Reading[T]` (Node, ETag, Aliases, Refusal, Parse, Fields, AliasFindings; IsRead, IsParsed, Findings, IsUsable); the alias findings are named `AliasFindings` because `Aliases` holds the alias list, and a refused read is its own `Refusal` field (one NotRegular finding worded as the walk words it), so Parse stays split from the rest as the Regression guard asks.

NOTES (2026-10-14): consequential edit — internal/nodestore/refs.go: made necessary by `aliasParser` losing its `node *T` parameter (ReadContent no longer uses it; planRemoval's call drops the `nil`).

**What:**
**Goal:** nodestore exports a Kind reader per kind, built from the descriptor: `Read(path)` reads a file (refusing non-regular and too-large files) and `Decode(relPath, data)` decodes bytes already read; both return the node, etag (`nodefile.ETag`), aliases, findings (refused read, parse, fields, aliases, in `diagnose.Finding` form) and one usable verdict (no finding at all). `Kind.ReadContent` and `aliasParser` are built on it. The glossary Note `kind-reader` exists under `glossary-structure`.
**Approach (assumed at the header base):** a new `internal/nodestore/read.go`; it calls `diagnose.IssueFile`/`NoteFile` for now (P1.4 moves that logic in). Add the Note with `capcom know add --id kind-reader --parent glossary-structure --title "Kind reader: …"` and write its body in the shape of its siblings (definition, `_Avoid_:` line).
**Regression guard.** The reader's read entry point takes the absolute path and the worktree-relative path (`Read(path, relPath)`) so every finding carries the worktree-relative path diagnose.Finding requires; the result exposes the parse finding separately from the field and alias findings (as diagnose.Verdict does: Parse, Fields, Aliases, plus Findings() and IsUsable), so check and cli keep their split; `capcom know add` runs the tree-built binary. `Read` returns (result, error): a refused read becomes a finding, any other read failure is returned as the error, which `ReadContent` wraps in a `*tree.Error` (`TestAliasLookups_MissingFile_FailWithATreeError` stays green). The aliases are what `tree.ReadAliases` returns: recovered for an unparsed file, the full parsed list for a parsed one. Add `internal/nodestore/doc.go` to Files with a `read.go` line in its file map. `kind.go` no longer calls `codec.Parse`/`codec.Validate` itself.
**Files:** internal/nodestore/read.go; internal/nodestore/read_test.go; internal/nodestore/kind.go; internal/nodestore/doc.go; .capcom/knowledge/glossary/glossary-structure/kind-reader.md
**Read first:** internal/nodestore/kind.go — ReadContent, RecoverAliases, aliasParser, isRefusedRead; internal/diagnose/files.go — IssueFile, NoteFile, Verdict.Findings, AliasFindings; internal/diagnose/diagnose.go — ReadRefusal; internal/tree/claims.go — ReadAliases, readContentFacts;
internal/nodefile/etag.go — ETag; internal/nodestore/kind_test.go — kindAliasLookups, TestReadContent_ParseableUnparseableAndInvalidFiles, TestAliasLookups_MissingFile_FailWithATreeError;
internal/nodestore/doc.go — file map; internal/nodestore/refs.go — planRemoval (aliasParser caller)
**Tests:** a table test per kind: clean file, parse failure, field finding, alias finding, not regular, too large; etag equals `nodefile.ETag` of the bytes; a missing file returns an error, not a finding; a broken file gives its recovered aliases; every finding's path is the worktree-relative one.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodestore/`; `go build -o <scratchpad>/capcom ./cmd/capcom`; `<scratchpad>/capcom know show kind-reader`; `<scratchpad>/capcom check` exits 0; `grep -n 'codec.Parse\|codec.Validate' internal/nodestore/kind.go` prints nothing
**Commit:** `refactor(nodestore): one Kind reader per kind`

## P1.3 The index reads through the Kind reader — ✅ DONE (2026-10-14)

**What:**
**Goal:** `internal/index` decodes node files only through the Kind reader; `parseIssue`, `parseNote` and `aliasParser` are gone; index rows, warnings and the agreement test are unchanged.
**Approach (assumed at the header base):** `refresher.indexFile` keeps its `readNodeFile` seam and calls the reader's `Decode`; `walkIssues`, `orphanDirectoryClaims` and `claimedNames` take aliases from it. The refresher holds only `r.config`, so it builds readers from the config. Delete `internal/index/problems.go`. Depends on P1.2.
**Regression guard.** `orphanDirectoryClaims` (walk.go) takes its aliases from the reader too, not from its own `nodefile.ReadRegularFile` + `tree.ReadAliases` decode; Acceptance greps the whole package (comments included) for every direct decode.
**Files:** internal/index/problems.go; internal/index/notes.go; internal/index/walk.go; internal/index/refresh.go
**Read first:** internal/index/refresh.go — indexFile, claimedNames, walkStore, readNodeFile, refresher; internal/index/walk.go — walkIssues, orphanDirectoryClaims, problemClaims, aliasParser, errUnparsed; internal/index/problems.go — parseIssue;
internal/index/notes.go — parseNote, insertNote; internal/index/agreement_test.go — nodestore.Issues/Notes ReadContent, RecoverAliases; internal/index/index_test.go — TestRefresh_FileGoneBeforeItsRead_IsDroppedAndNotParsed
**Tests:** existing index tests and `agreement_test.go` stay green.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/`; `grep -rn 'func parseIssue\|func parseNote\|func aliasParser' internal/index` prints nothing; `grep -rn 'issue\.Parse\|knowledge\.Parse\|diagnose\.IssueFile\|diagnose\.NoteFile\|tree\.ReadAliases' internal/index --include='*.go' | grep -v _test` prints nothing
**Commit:** `refactor(index): read node files through the Kind reader`

## P1.4 check reads through the Kind reader; diagnose's file verdict moves into nodestore — ✅ DONE (2026-10-14)

NOTES (2026-10-14): the Reader is now built from the kind's Descriptor (its Noun names a node in the alias findings, its Validate gives the Field findings) plus the kind's parse and aliases/aliasesLine accessors; `IssueFile`/`NoteFile`/`Verdict` are folded into `Reader.Decode` rather than kept as separate functions in read.go, with `parseFinding`/`fieldFindings` moved in unexported.

NOTES (2026-10-14): `internal/check/problem.go` needed no change (loadTree, readFile and kindCode already fit); check's readAliases closures return the Reading's Aliases, which for a parsed file are the parsed node's.

NOTES (2026-10-14): ported verdict tests are named `TestIssueReader_Decode_SortsItsFindings`/`TestNoteReader_Decode_SortsItsFindings` with helper `assertReadingFindings`; the "verdict" wording in check's parsedIssue/checkIssue/checkNote comments now says "Reading".

**What:**
**Goal:** `internal/check` reads node files only through the Kind reader; `diagnose.IssueFile`, `NoteFile` and `Verdict` are gone, their logic living in the reader; check output is unchanged.
**Approach (assumed at the header base):** the `readAliases` closures in `check/issues.go` and `check/notes.go` call the reader's `Decode` after `c.readFile` and `checkConflictMarkers`; move `IssueFile`/`NoteFile` into `read.go` and port `diagnose/files_test.go` cases into `read_test.go`. `diagnose.AliasFindings` and `parseFinding`'s helpers stay if still used. Depends on P1.3.
**Regression guard.** check adds only the reader's Field findings (the result's Fields), sends its parse finding to `addParseFailure`, and leaves alias findings to `loadTree`; adding them too would report every bad alias twice, once with a raw code outside check's closed set. Fix `diagnose.go`'s package doc (it names the file verdict). Move `parseFinding`/`fieldFindings` (unexported) into nodestore; port only `TestIssueFile_Verdict`/`TestNoteFile_Verdict` and keep `TestAliasFindings_RejectedAliases_ReturnsTheirKinds` and `TestFinding_String_RendersTheLineOnlyWhenKnown` in `files_test.go`.
**Files:** internal/check/issues.go; internal/check/notes.go; internal/check/problem.go; internal/diagnose/files.go; internal/diagnose/files_test.go; internal/diagnose/diagnose.go; internal/nodestore/read.go; internal/nodestore/read_test.go
**Read first:** internal/check/issues.go — issues, graphNodes; internal/check/notes.go — notes, checkNote; internal/check/problem.go — loadTree, readFile, kindCode; internal/check/check.go — addParseFailure, checkConflictMarkers, add;
internal/diagnose/files.go — IssueFile, NoteFile, parseFinding, fieldFindings, AliasFindings; internal/diagnose/files_test.go — TestIssueFile_Verdict, TestNoteFile_Verdict, assertVerdictFindings;
internal/check/check_test.go — TestCodes_ClosedSet_PinsEveryCodeString, codeConstants; internal/diagnose/diagnose.go — package doc
**Tests:** ported verdict cases; the alias and `Finding.String` tests stay in diagnose; existing check tests stay green.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/check/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodestore/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/diagnose/`; `grep -rn 'func IssueFile\|func NoteFile\|type Verdict' internal/diagnose` prints nothing
**Commit:** `refactor(check): read node files through the Kind reader`

## P1.5 The Managed block hides Notes the index leaves out — ✅ DONE (2026-10-14)

NOTES (2026-10-14): re-derived from the assumption that a top-level Note with `labels: [""]` is listed in the block before this change: `knowledge.Parse` (its schema) already refuses an empty label, so that Note never parsed and was already left out. The tests keep the `labels: [""]` case and add Notes that parse but fail `knowledge.Validate` (`refs: ["#symbol"]`, a blank title, superseded-by itself), which are the cases that bite before the change.

NOTES (2026-10-14): topNotes maps the Note reader's Reading to tree.Content with its own `contentOf` (the same mapping as `Kind.ReadContent`), since agents holds only knowledgeDir and no store to build a nodestore Kind from; findings are discarded, so the reader is given the path relative to knowledgeDir.

**What:**
**Goal:** fixes `block-left-out-notes`: the AGENTS.md Managed block lists only top-level Notes the Kind reader judges usable and tree membership keeps, so a top-level Note with `labels: [""]` is absent from the block and from `know tree`, and reported by `capcom check`.
**Approach (assumed at the header base):** `agents.topNotes` replaces its `readAliases` closure and `tree.Load` with `tree.LoadContent` over a content func built on the Note reader (as `nodestore.load` does); retired and background filtering stays. agents → nodestore is acyclic. Depends on P1.2. CHANGELOG line: the block no longer lists a Note that `know tree` hides.
**Regression guard.** Add docs/site/agents/managed-block.md to Files and add Notes that do not parse or validate to its list of what the block leaves out. Keep the `isNamed` filter over `LoadContent`'s `tree.Error`s beside the usable verdict; "tree membership keeps" means "in `loaded.Nodes()`", never `Resolve` succeeding (`Resolve` refuses a clean Note whose id an unparsed Note's recovered alias claims, which `know tree` and the block list today), so `TestTopNotes_SharedAlias_LeavesBothOut` stays green.
**Files:** internal/agents/block.go; internal/agents/block_test.go; internal/cli/agents_test.go; docs/site/agents/managed-block.md
**Read first:** internal/agents/block.go — topNotes, Sync; internal/tree/tree.go — LoadContent, Load, load; internal/nodestore/kind.go — ReadContent, aliasParser; internal/nodestore/load.go — load;
internal/agents/block_test.go — TestTopNotes_SharedAlias_LeavesBothOut, TestTopNotes_DuplicatedTopLevelID_IsLeftOut, writeNoteFiles; internal/cli/agents_test.go — TestAgents_AppendsThenRefreshesInPlace, agentsBlockOf;
internal/cli/know_test.go — writeNoteFiles; internal/check/agents.go — Sync caller
**Tests:** an agents test with a top-level `labels: [""]` Note beside a clean one (bite: fails before the change); a cli test running `capcom agents` and `capcom know tree` on that store, asserting the Note is in neither and `capcom check` reports it; `TestTopNotes_SharedAlias_LeavesBothOut` and `TestTopNotes_DuplicatedTopLevelID_IsLeftOut` stay green.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/agents/`; `GOMEMLIMIT=2GiB go test -count=1 -run 'Agents|KnowTree' ./internal/cli/`
**Closes:** block-left-out-notes
**Commit:** `fix(agents): the Managed block hides Notes the index leaves out`

## P1.6 cli reads node files through the Kind reader — ✅ DONE (2026-10-14)

NOTES (2026-10-14): `readIssueFile` is replaced in update.go by two generic functions on the Kind reader: `readNodeFile` (a failed or refused read is CodeInvalid at the entry path; close's `alreadyClosed` uses it, so it still needs only a readable file) and `parseNodeFile` (adds the parse failure; show, know show and know verify use it). Field and alias findings fail no verb.

NOTES (2026-10-14): the Kind reader turns a refused read into a NotRegular finding worded with the sentinel alone, so `readNodeFile` rebuilds today's `*fs.PathError` text from it (`read <absolute path>: <sentinel text>`, Op in the constant `refusedReadOp`, mirroring nodefile's unexported `readOp`).

NOTES (2026-10-14): `frontmatterFailure` is kept unchanged as the plan's guard asks, but its only remaining caller is `cli_test.go`'s `TestUpdate_FrontmatterFailure_RelativizesOnlyAnAbsolutePath`; the new Finding-taking sibling `parseFailure` does the same mapping for the reader's parse finding.

NOTES (2026-10-14): `issueFromFile`/`noteFromFile` keep their names but now take the parsed node and return no error; `noteFromFile` no longer needs the app and is a plain function.

**What:**
**Goal:** `show`, `know show`, `know verify`, `close` and `update` read, parse and etag node files only through the Kind reader; `readIssueFile` and the direct `issue.Parse`/`knowledge.Parse`/`nodefile.ETag` calls in those files are gone; output, error codes and paths are unchanged.
**Approach (assumed at the header base):** replace `readIssueFile` + `issueFromFile`/`noteFromFile` + `nodefile.ETag` with a reader call; keep `frontmatterFailure`'s mapping (`output.Error{CodeInvalid, Path, Line}`) on the reader's parse finding. Depends on P1.2.
**Regression guard.** Keep `frontmatterFailure(err error)` as it is (add a Finding-taking sibling if needed), so `cli_test.go`'s `TestUpdate_FrontmatterFailure_RelativizesOnlyAnAbsolutePath` compiles unchanged. A refused or failed read keeps `CodeInvalid` at `entry.Path` with today's `*fs.PathError` text (as `readIssueFile` gives it), and only the parse finding fails a verb: field and alias findings never do (the index already leaves such files out).
**Files:** internal/cli/show.go; internal/cli/know_show.go; internal/cli/know_verify.go; internal/cli/close.go; internal/cli/update.go
**Read first:** internal/cli/update.go — readIssueFile, frontmatterFailure, absoluteIssuePath; internal/cli/show.go — showResult, issueFromFile; internal/cli/know_show.go — noteShowResult, noteFromFile; internal/cli/know_verify.go — verifyNotes;
internal/cli/close.go — alreadyClosed; internal/cli/cli_test.go — TestUpdate_FrontmatterFailure_RelativizesOnlyAnAbsolutePath; internal/nodefile/read.go — ReadRegularFile, checkReadable
**Tests:** existing show/know show/verify/close/update tests stay green, `TestUpdate_FrontmatterFailure_RelativizesOnlyAnAbsolutePath` unchanged.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/`; `grep -n 'readIssueFile\|nodefile.ETag\|issue.Parse\|knowledge.Parse' internal/cli/show.go internal/cli/know_show.go internal/cli/know_verify.go internal/cli/close.go internal/cli/update.go` prints nothing
**Commit:** `refactor(cli): read node files through the Kind reader`

## P2.1 One format-key rule for codec and check — ✅ DONE (2026-10-14)

NOTES (2026-10-14): the format raise parses the bytes about to be written (Kind.raise(path, data)) and Codec.requiredFormat reads the descriptor's format table off that parsed node; since Serialize drops empty lists and `retired: false`, only a non-empty `related`/`superseded-by` or `retired: true` line raises on a write, while check flags the hand-written empty forms

NOTES (2026-10-14): descriptor.go needed no change (its Formats table from P1.1 is the one table); check reads it through a new generic addKeysFormatTooOld in internal/check/problem.go, a file the plan did not list

NOTES (2026-10-14): editNode now takes the app and adds the raise warning to a new app.writeWarnings list that emit renders after the index warnings and before the result's own, keeping know update's raise-then-sync warning order

NOTES (2026-10-14): TestKnowUpdate_WritesThatGainNoKey_LeaveTheConfigAlone recast and renamed TestKnowUpdate_WritesCarryingAKeyLine_RaiseTheConfigOnceWhateverTheyGain (refused edits first leave format 2; the first no-gain write raises to 3 with one warning; later writes warn no more)

NOTES (2026-10-14): related add / know update help text and docs/site/concepts pages left as they are: "the first link ... raises" and "retiring a Note ... raises" stay true under the new rule

**What:**
**Goal:** fixes `format-key-rule`: a key line needs its format whatever it holds. Check flags `related: []` below `config.FormatRelated` and `superseded-by: []` and `retired: false` below `config.FormatSupersession`; a write raises the store format when the file it writes carries a key line the store's format lacks. Both read the descriptor's format table; `gainsEntry`/`gainsSupersession` and check's `keyRelated`/`keyRetired`/`keySupersededBy` constants are gone.
**Approach (assumed at the header base):** `requiredFormat` checks the written node against the table's line accessors (parse the serialized bytes, or a presence predicate per key); `checkIssue` and `checkSupersession` loop over the table. Depends on P1.1 and P1.4. CHANGELOG line only if a result visibly changes.
**Regression guard.** Acceptance also runs, one package at a time, the tests of every other package whose tests pin the changed behaviour (find them by grepping *_test.go for the changed strings or symbols), e.g. ./internal/cli/. The raise covers `Create` and `Edit` only; `Move`/`Remove`/`Format` leave config.yaml alone. `Effects.RaisedFormat` is rendered once in `editNode` (cli/node_edit.go) for every edit verb, dropping the calls in `related.go` and `know_update.go`, so `close`/`update`/`reopen`/`log`/`dep` warn too. Recast `TestKnowUpdate_WritesThatGainNoKey_LeaveTheConfigAlone` to the new rule (one raise, one warning). Every nodestore comment describing the raise (`grep -n 'gain' internal/nodestore/*.go`: edit.go step 8, create.go, kind.go `Effects.RaisedFormat`, codec.go `requiredFormat`) states the key-line rule. This supersedes the raise-on-gain wording of `docs/site/reference/file-format.md:160-164,576-585` ("first related add", "retires or adds") and `internal/nodestore/edit.go:34-37`; update the page.
**Files:** internal/nodestore/descriptor.go; internal/nodestore/codec.go; internal/nodestore/format.go; internal/nodestore/kind.go; internal/nodestore/edit.go; internal/nodestore/create.go; internal/nodestore/kind_test.go; internal/check/issues.go; internal/check/notes.go; internal/check/check_test.go; internal/cli/node_edit.go; internal/cli/related.go; internal/cli/know_update.go; internal/cli/know_update_test.go; internal/cli/close_test.go; docs/site/reference/file-format.md
**Read first:** internal/nodestore/codec.go — issueCodec, noteCodec, gainsEntry, gainsSupersession; internal/nodestore/kind.go — Kind.raise; internal/nodestore/edit.go — Kind.Edit; internal/check/issues.go — checkIssue, keyRelated;
internal/check/notes.go — checkSupersession; internal/check/check_test.go — TestRun_SupersessionKeysWithoutConfig_ReportFormatTooOldAtEachKey, TestRun_RelatedBelowFormatTwo_ReportsFormatTooOldAtTheRelatedLine;
internal/cli/know_update_test.go — TestKnowUpdate_WritesThatGainNoKey_LeaveTheConfigAlone; internal/cli/related.go — warnRaisedFormat
**Tests:** check tests pinning `related: []` and `superseded-by: []` below their formats; a nodestore test that a write carrying a key line raises the format and one that does not leaves it; the recast `TestKnowUpdate_WritesThatGainNoKey_LeaveTheConfigAlone`; a `close` test in a format-1 store on an issue holding `related:` that raises and warns once.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodestore/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/check/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/`, one at a time; `grep -rn 'gainsEntry\|gainsSupersession' internal/nodestore` prints nothing
**Closes:** format-key-rule
**Commit:** `fix(check): one format-key rule for codec and check`

## P3.1 node verbs take nouns and ref keys from the descriptor — ✅ DONE (2026-10-14)

NOTES (2026-10-14): nodeVerbs replaces linkKey with a store-free `descriptor nodestore.Descriptor[T]` field (issue Descriptor from a zero config.Config, Note Descriptor) and reads its RefKeys(); nounPhrase stays until P3.4 as the plan's guard says. descriptor.go only gains a doc line on IssueDescriptor stating that only Validate depends on cfg.

NOTES (2026-10-14): added `joinKeys` ("a", "a and b", "a, b and c") with a table test in node_verbs_test.go, which the plan's Files did not name, so the three-key form is pinned; the full ./internal/cli/ package passes too.

**What:**
**Goal:** fixes the first half of `unread-warning-keys`: `mv`/`rm` on an issue with an unparseable bystander warn `<path>: deps and related not checked for <id>: the file does not parse`; Note verbs keep `superseded-by not checked`. `nodeVerbs` has no `nounPhrase`/`linkKey` fields of its own; it reads the descriptor.
**Approach (assumed at the header base):** `unreadWarnings` joins the descriptor's ref keys ("a and b", "a, b and c"); rename `TestMv_UnparseableBystander_WarnsDepsNotChecked` to name deps and related and repin it and `rm_test.go`'s twin. Depends on P1.1. CHANGELOG line for the wording.
**Regression guard.** Acceptance also runs, one package at a time, the tests of every other package whose tests pin the changed behaviour (./internal/nodestore/ and any package pinning the unread wording; find them by grepping *_test.go). Drop only `linkKey` here; `nounPhrase` stays until P3.4, after P3.3 makes the descriptor say "a Note" (read now, the descriptor's "a note" would turn `know mv`/`know rm` Short against `docs/site/reference/cli/capcom_know_mv.md:3`/`capcom_know_rm.md:3` and fail clidoc's `TestReferenceUpToDate`). Read ref keys from a store-free descriptor accessor (Note descriptor; issue descriptor from a zero `config.Config`), never from `a.store`: `Command()` builds the tree with no store for gendocs.
**Files:** internal/cli/node_verbs.go; internal/cli/mv_test.go; internal/cli/rm_test.go; internal/nodestore/descriptor.go
**Read first:** internal/cli/node_verbs.go — nodeVerbs, issueVerbs, noteVerbs, unreadWarnings, newMvCommand, newRmCommand; internal/cli/mv_test.go — TestMv_UnparseableBystander_WarnsDepsNotChecked; internal/cli/rm_test.go — TestRm_UnparseableIssue_WarnsDepsNotChecked;
internal/cli/know_mv_test.go — TestKnowMv_UnparseableBystander_WarnsAndMoves; internal/clidoc/reference_test.go — TestReferenceUpToDate; internal/cli/root.go — Command, newRootCommand, prepare; internal/nodestore/codec.go — refs, nounPhrase
**Tests:** the repinned mv and rm tests (bite: fail before the change); `know_mv_test.go`/`know_rm_test.go` superseded-by pins stay green.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 -run 'Mv|Rm|KnowMv|KnowRm' ./internal/cli/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodestore/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`, one at a time
**Closes:** kind-reader
**Commit:** `fix(cli): the unread warning names every ref key`

## P3.2 "Note" in messages, part 1: knowledge, frontmatter and tree — ✅ DONE (2026-10-14)

NOTES (2026-10-14): consequential edit — internal/frontmatter/decode.go: made necessary by the Note noun change (doc comments quote "a note file must start…" and "; a note has title…")

NOTES (2026-10-14): consequential edit — internal/frontmatter/frontmatter.go: made necessary by the Note noun change (doc comment example noun "a note")

NOTES (2026-10-14): consequential edit — internal/frontmatter/schema.go: made necessary by the Note noun change (doc comment example noun "a note")

NOTES (2026-10-14): consequential edit — internal/tree/move.go: made necessary by the Note noun change (doc comment example noun "a note")

NOTES (2026-10-14): consequential edit — internal/tree/tree.go: made necessary by the Note noun change (doc comment example noun "note")

NOTES (2026-10-14): remove.go changes are doc comments only: bareNoun/childCount already carry the caller's capitalisation ("Note parent-node has 2 child Notes"); tree_test.go's CheckAliases/Load fixtures now pass "Note"; the callers' own lowercase nouns (nodestore descriptor, cli nounPhrase) stay for P3.3/P3.4, so cli's know_rm_test "note testing-rules has 2 child notes" is unchanged

**What:**
**Goal:** messages from `knowledge`, `frontmatter` and `tree` name the Note kind "Note"/"a Note" (e.g. `a Note has title, labels, …`, `a Note file must start with a --- line`, `names the Note itself`, `has 2 child Notes`). "issue" is unchanged.
**Approach (assumed at the header base):** `knowledge.NewSchema("a Note", …)` and `note.go`'s self-reference message; tree messages derive from the noun the caller passes (`bareNoun`, `childCount`), so check they capitalise correctly. Rule for the guard: every user-facing message naming the Note kind (`grep -rn '"[^"]*\bnotes\?\b' internal/knowledge internal/frontmatter internal/tree`) uses "Note".
**Regression guard.** Same cross-package rule as P2.1 — its noun change reaches messages pinned in ./internal/check/, ./internal/index/, ./internal/nodestore/, ./internal/cli/ and ./internal/clidoc/; the grep (`a note has`, `a note file`, `names the note itself` in *_test.go) finds `nodestore/read_test.go`, `index/notes_test.go` and `check/check_test.go`, which join Files and whose packages join Acceptance, one package per command. The Goal's tree examples (`has 2 child Notes`) hold for tree messages given the noun "a Note" (`tree/move_test.go`'s `noteKind` becomes "a Note"); CLI output keeps the callers' lowercase noun until P3.3. Re-check: diagnose's NoteFile and its verdict test have moved to internal/nodestore/read.go and read_test.go by P1.4, so any Note-wording site P3.2 names in diagnose/files.go or files_test.go points at read.go/read_test.go instead (`diagnose/files_test.go:102` at the base is now a `read_test.go` case).
**Files:** internal/knowledge/note.go; internal/knowledge/knowledge_test.go; internal/frontmatter/decode_test.go; internal/tree/remove.go; internal/tree/move_test.go; internal/tree/remove_test.go; internal/tree/tree_test.go; internal/nodestore/read_test.go; internal/index/notes_test.go; internal/check/check_test.go
**Read first:** internal/knowledge/note.go — noteSchema, Validate; internal/frontmatter/decode.go — Kind, keysHint, Decode; internal/frontmatter/frontmatter.go — Split; internal/tree/remove.go — ChildrenError, ChildrenDirError, childCount;
internal/tree/move.go — bareNoun; internal/tree/move_test.go — noteKind; internal/knowledge/knowledge_test.go; internal/nodestore/read_test.go
**Tests:** repinned message tests in those packages, the unknown-key hints in nodestore, index and check included.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/knowledge/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/frontmatter/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/tree/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodestore/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/check/`, one at a time
**Commit:** `fix(knowledge): messages say Note`

## P3.3 "Note" in messages, part 2: nodestore, diagnose and index — ✅ DONE (2026-10-14)

NOTES (2026-10-14): read.go and read_test.go needed no edit: Reader.Decode already passes r.descriptor.Noun to diagnose.AliasFindings, so the descriptor noun change alone rewords its alias findings, and read_test.go holds no Note-noun message pin

NOTES (2026-10-14): consequential edit — internal/diagnose/files.go: made necessary by the descriptor noun change (AliasFindings doc comment quotes the noun "note")

NOTES (2026-10-14): internal/nodestore/descriptor_test.go repinned with the other nodestore tests: it pins the Note descriptor's noun and noun phrase directly

NOTES (2026-10-14): schemaVersion bumped 8 -> 9 so stored problem messages are rebuilt in the new wording; check's own alias message (check_test.go "the note's own id") is left for P3.4 as planned

**What:** Recast at the regression check (2026-10-14).
**Goal:** the Note descriptor's noun is "Note" and noun phrase "a Note", so nodestore's messages say it (e.g. `already taken by Note X`, `show the Note again`); diagnose's alias findings and index's own messages say "Note" (`repeats the Note's own id`, `read Note %q`, `Note id %q`). JSON `kind` values, the index's `kindNote`, flag and command names are unchanged. diagnose gains no `const`.
**Approach (assumed at the header base):** the descriptor in `nodestore/descriptor.go`; `diagnose.AliasFindings` takes the noun from its caller; reword `index/notes.go`'s literals; bump `schemaVersion`. Guard rule: `grep -rn '"[^"]*\bnotes\?\b' internal/nodestore internal/diagnose internal/index --include=*.go | grep -v _test` shows no message noun left lowercase. Depends on P3.1 and P3.2.
**Regression guard.** Same cross-package rule as P2.1: the descriptor noun and `AliasFindings` reach pins in cli (`know_mv_test.go`, `know_rm_test.go`, `know_test.go`; find them by grepping *_test.go), repinned here; check's and cli's own literals stay for P3.4, help text for P3.5. Re-check: same retargeting as P3.2 (diagnose/files.go:86 → nodestore/read.go's Note decode, which passes "Note" to `AliasFindings`; files_test.go:102 → read_test.go); check's alias message (`repeats the note's own id`, check_test.go:859) follows check's own loadTree noun (check/notes.go), so that half moves to P3.4, not P3.3, and ./internal/check/ stays in Acceptance as a no-change check. `nodestore/remove_test.go`/`move_test.go` ("show the note again") and `nodestore/read_test.go` are repinned. The new wording supersedes the `the note's own id` quotes in CHANGELOG.md:16 ([Unreleased]); P3.5 rewords them. `nodeVerbs.nounPhrase` stays until P3.4. Bump `schemaVersion` (index/schema.go; precedent 3804b69) so stored problem messages are rebuilt in the new wording.
**Files:** internal/nodestore/descriptor.go; internal/nodestore/kind_test.go; internal/nodestore/create_test.go; internal/nodestore/remove_test.go; internal/nodestore/move_test.go; internal/nodestore/read.go; internal/nodestore/read_test.go; internal/index/notes.go; internal/index/notes_test.go; internal/index/schema.go; the cli *_test.go files the grep finds
**Read first:** internal/nodestore/codec.go — noteCodec Noun, nounPhrase; internal/nodestore/errors.go — conflictError, existsError; internal/nodestore/claim.go — claimRefusal; internal/nodestore/load.go — load (LoadContent noun);
internal/diagnose/files.go — NoteFile, AliasFindings; internal/index/notes.go — readNote, noteList;
internal/cli/know_rm_test.go — TestKnowRm_WithChildren_IsRefusedNamingTheFileAndCount; internal/cli/know_mv_test.go — TestKnowMv_OntoTakenIDOrAlias_FailsExistsNamingTheNote
**Tests:** repinned tests in nodestore, diagnose and index (`nodestore/remove_test.go`, `nodestore/move_test.go`, `nodestore/read_test.go`, `index/notes_test.go` included), plus the cli pins the descriptor noun and `AliasFindings` reach.
**Acceptance:** each of `./internal/nodestore/`, `./internal/diagnose/`, `./internal/index/`, `./internal/check/`, `./internal/cli/` passes `GOMEMLIMIT=2GiB go test -count=1`, one at a time
**Commit:** `fix(nodestore): messages say Note`

## P3.4 "Note" in messages, part 3: check and cli — ✅ DONE (2026-10-14)

NOTES (2026-10-14): consequential edit — internal/check/problem.go: made necessary by the loadTree noun change (loadTree's doc comment quotes the noun "note")

NOTES (2026-10-14): node_verbs.go's mv/rm Short now reads verbs.descriptor.NounPhrase (field `descriptor` already held the store-free descriptor from P3.1); the descriptor field's doc comment now says it supplies the Short's noun phrase. Short text is unchanged ("an issue"/"a Note"), so clidoc goldens are untouched

NOTES (2026-10-14): the superseded-by assertion is a new test, TestRun_SupersededByResolvingToNothing_NamesNoNote, rather than a fixture in TestRun_NoteFileProblems_PinCodesAndWording, which is the index's twin fixture and the index reports no missing-superseded-by

NOTES (2026-10-14): `go build ./...` fails on untracked scratch copies under docs/skill-runs/release-notes-capcom/2026-10-14/scratch (pre-existing, not this item's); the sanity build used `go build ./cmd/... ./internal/...`

**What:**
**Goal:** check's and cli's own messages naming the Note kind say "Note" (e.g. `superseded-by %q names no Note by id or by an alias only one Note has`, `no Note matches %q`, `know update needs at least one flag that changes the Note`); `nodeVerbs` has no `nounPhrase` of its own and reads the descriptor's "a Note". Flag help is left to P3.5.
**Approach (assumed at the header base):** reword the literals in `check/notes.go` (messages and the noun passed to `loadTree`), `cli/know_search.go` and `cli/know_update.go`; drop `nodeVerbs.nounPhrase` and read the store-free descriptor accessor (P3.1). Guard rule: `grep -rn '"[^"]*\bnotes\?\b' internal/check internal/cli --include=*.go | grep -v _test` shows no message noun left lowercase except flag help. Depends on P3.3.
**Regression guard.** Same cross-package rule as P2.1. The `know mv`/`know rm` Short already reads "a Note" (node_verbs.go `nounPhrase: "a Note"`), so `docs/site/reference/cli` is unchanged; ./internal/clidoc/ confirms it. The guard rule also exempts P3.5's summaries (`know_verify.go` `%d note refs`, `prime.go` `broken note refs`) and kind and JSON values (`Kind: "note"`, `"note-write"`, `"note-remove"`, `json:"note"`). The `loadTree` noun turns check's `repeats the note's own id` (check_test.go:859) to "Note": repinned here, moved from P3.3. New assertions bite on the superseded-by message (check/notes.go:96), `no Note matches` and know update's no-change usage message, none pinned at the base.
**Files:** internal/check/notes.go; internal/check/check_test.go; internal/cli/know_search.go; internal/cli/know_update.go; internal/cli/node_verbs.go; internal/cli/know_test.go; internal/cli/know_update_test.go
**Read first:** internal/check/notes.go — notes, checkSupersession; internal/check/problem.go — loadTree; internal/check/check_test.go — TestRun_NoteFileProblems_PinCodesAndWording;
internal/cli/node_verbs.go — nodeVerbs, noteVerbs, newMvCommand, newRmCommand; internal/cli/know_search.go; internal/cli/know_update.go — options check;
internal/cli/know_update_test.go — TestKnowUpdate_NoChangeOrClashingFlags_AreUsageErrors; internal/cli/node_verbs_test.go — TestHelp_NodeVerbs_MatchGolden
**Tests:** repinned check and cli tests for the reworded literals, check_test.go:859's alias message included; new assertions on the superseded-by message (check_test.go), know search's no-match (know_test.go) and know update's no-change usage message (`TestKnowUpdate_NoChangeOrClashingFlags_AreUsageErrors`) (bite: fail before the change).
**Acceptance:** each of `./internal/check/`, `./internal/cli/`, `./internal/clidoc/` passes `GOMEMLIMIT=2GiB go test -count=1`, one at a time; `grep -n 'nounPhrase' internal/cli/node_verbs.go` prints nothing
**Commit:** `fix(cli): messages say Note`

## P3.5 "Note" in help and summaries — ✅ DONE (2026-10-14)

NOTES (2026-10-14): CHANGELOG.md:16's two `the note's own id` quotes are not reworded here (implementers never edit CHANGELOG.md); the plan's Regression guard assigns that reword to the closeout, so the Acceptance grep `grep -n "the note's own id" CHANGELOG.md` passes only once the closeout applies it

NOTES (2026-10-14): the CHANGELOG entry covers P3.5's own change only: P3.2, P3.3 and P3.4 each already carry their own entry in their sidecars, so the plan's "one line covering P3.2 to P3.5" is met by those four lines together

NOTES (2026-10-14): know_tree.go's --depth help and know_update.go's --add-superseded-by help sit on continuation lines the Acceptance grep cannot see; both reworded to Note as the Goal asks. internal/agents/block.go quotes neither summary, so AGENTS.md is untouched

**What:**
**Goal:** completes `unread-warning-keys`: flag help names the Note kind "Note" (`know_add.go`, `know.go`, `know_tree.go`, `know_update.go`, `know_search.go`, `know_show.go`, `node_verbs.go` flagNoun), the Briefing prints `broken Note refs:` and `know verify` reports `N Note refs do not resolve`. Flag names, command names, JSON keys and kind values stay lowercase.
**Approach (assumed at the header base):** reword the help strings and the two summaries; regenerate `docs/site/reference/cli` with `go run ./cmd/gendocs`; repin `prime_test.go` and `know_test.go`'s `note refs do not resolve`; update `docs/site/agents/briefing.md`, `docs/site/concepts/knowledge.md` and `docs/site/getting-started/quickstart.md` where they quote those strings. `internal/agents/block.go` quotes neither at the base; if it does at run time, reword it and refresh AGENTS.md with a tree-built binary's `capcom agents`. One CHANGELOG `Unreleased` line covering P3.2 to P3.5. Depends on P3.4.
**Regression guard.** `TestHelp_NodeVerbs_MatchGolden` (node_verbs_test.go:13) pins --help byte for byte: repin `internal/cli/testdata/help/know-mv.txt` (:14) and `know-add.txt` (:14,16-18) to the new help. In the same closeout, reword the two `the note's own id` quotes on CHANGELOG.md:16 ([Unreleased]) to `the Note's own id`, superseded by P3.3 and P3.4. Acceptance greps the flag help strings for lowercase `note`.
**Files:** internal/cli/know_add.go; internal/cli/know.go; internal/cli/know_tree.go; internal/cli/know_update.go; internal/cli/know_search.go; internal/cli/know_show.go; internal/cli/node_verbs.go; internal/cli/prime.go; internal/cli/prime_test.go; internal/cli/know_verify.go; internal/cli/know_test.go; internal/cli/testdata/help/know-mv.txt; internal/cli/testdata/help/know-add.txt; docs/site/reference/cli/; docs/site/agents/briefing.md; docs/site/concepts/knowledge.md; docs/site/getting-started/quickstart.md
**Read first:** internal/cli/node_verbs.go — flagNoun, newMvCommand; internal/cli/know_add.go — flag help; internal/cli/know.go — retiredFlagUsage; internal/cli/know_tree.go — backgroundFlagUsage;
internal/cli/prime.go — addHealth; internal/cli/know_verify.go — verifyNotes; internal/cli/node_verbs_test.go — TestHelp_NodeVerbs_MatchGolden;
internal/cli/prime_test.go — TestPrime_FixtureStore_PrintsTheBriefing
**Tests:** repinned `prime_test.go` (`broken Note refs: 1`) and `know_test.go` (`N Note refs do not resolve`); the help goldens `know-mv.txt` and `know-add.txt` repinned for `TestHelp_NodeVerbs_MatchGolden`; clidoc's `TestReferenceUpToDate` against the regenerated pages.
**Acceptance:** each of `./internal/cli/`, `./internal/clidoc/`, `./internal/briefing/`, `./internal/agents/` passes `GOMEMLIMIT=2GiB go test -count=1`, one at a time; `grep -nE '(Var\(|Usage = |flagNoun: ).*"[^"]*\bnotes?\b' internal/cli/know_add.go internal/cli/know.go internal/cli/know_tree.go internal/cli/know_update.go internal/cli/know_search.go internal/cli/know_show.go internal/cli/node_verbs.go` prints nothing; `grep -n "the note's own id" CHANGELOG.md` prints nothing
**Closes:** unread-warning-keys
**Commit:** `docs(cli): help and summaries say Note`

## P4.1 The Lock overview — ✅ DONE (2026-10-14)

NOTES (2026-10-14): internal/lock/lock.go (not in **Files:**) is touched to reuse List's scan as the guard asks: the directory scan with its Lstat check moves out of List into the new `Locks.recordKeys(ownKey)` beside it, which List calls with "" (unchanged behaviour) and Overview with its own key, so the own key's entry is read even when it is a directory.

NOTES (2026-10-14): a problem's Path is always the record file; its Err (read's error, unchanged) names the guard file instead when the guard cannot be opened. The own key's problem is listed in Problems too (as the Briefing's warnings list it today), and the own key is not read at all when it has no record entry, so no guard file is created for it.

**What:**
**Goal:** `lock` exports a Lock overview: every held lock, this worktree's state as a named value (`free`, `held`, `stale`, `invalid`, `unreadable`) and typed per-record problems (key, path, invalid or unreadable). A foreign invalid or unreadable record is one problem beside the other locks; only an unlistable locks directory fails it; a missing locks directory gives an empty overview with state free and creates nothing. A guard that cannot be opened for one key makes that record unreadable. The glossary Note `lock-overview` exists under `glossary-coordination`. No caller changes yet.
**Approach (assumed at the header base):** a new `internal/lock/overview.go` with `Locks.Overview(key)` and `Worktree.Overview()`, built on `read` and the `List` directory scan; the own key is read even when its entry is a directory. Add the Note with `capcom know add --id lock-overview --parent glossary-coordination …`, shaped like `stale.md`.
**Regression guard.** The overview's per-record problem carries read's error unchanged, so P4.2 and P4.3 can word their warnings as today; `capcom know add` runs the tree-built binary. Reuse `List`'s scan with its `Lstat` check (Windows reports a file in the directory's place as not found), and create nothing: no own-key `withGuard` `MkdirAll` on a missing directory. Port `TestList_FileWhereTheLockDirectoryBelongs_Fails` and `TestList_NoLockDirectory_IsEmpty` to Overview.
**Files:** internal/lock/overview.go; internal/lock/overview_test.go; internal/lock/worktree.go; .capcom/knowledge/glossary/glossary-coordination/lock-overview.md
**Read first:** internal/lock/lock.go — List, read, withGuard, Read, errCorruptRecord, Status; internal/lock/worktree.go — Worktree, ForWorktree, List;
internal/lock/lock_test.go — openTest, writeRecord, mustAcquire, TestList_FileWhereTheLockDirectoryBelongs_Fails, TestBreak_UnreadableRecord_FailsAndKeepsIt;
internal/store/store_test.go — TestInit_ReRunOnReadOnlyStore_SucceedsAndCreatesNothing; .capcom/knowledge/glossary/glossary-coordination/stale.md
**Tests:** one test per own state; a foreign invalid record and a foreign unreadable record (chmod, skipped on Windows and root) each beside a held lock, asserting the problem's wording (`<path>: invalid lock record: <cause>`, `<path>: <io error>`); a directory at a foreign `<key>.lck` beside a held lock gives exactly one unreadable problem; a missing directory creates nothing; a file where the locks directory belongs fails the overview.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/lock/`; `go build -o <scratchpad>/capcom ./cmd/capcom`; `<scratchpad>/capcom know show lock-overview`; `<scratchpad>/capcom check` exits 0
**Commit:** `feat(lock): a Lock overview of every lock and this worktree's state`

## P4.2 The Briefing and prime read the Lock overview — ✅ DONE (2026-10-14)

NOTES (2026-10-14): the Briefing's `Locks` and `WorktreeKey` fields are replaced by one `LockOverview *lock.Overview` field (nil when the locks directory cannot be listed); every lock-record problem becomes one SectionLocks failure carrying the overview's RecordProblem, so no code matches lock error text.

NOTES (2026-10-14): added `TestCompose_InvalidOwnLockRecord_IsOneLocksFailureAndKeepsTheOverview` (briefing) and `TestPrime_DirectoryAtTheLockRecord_AdvisesItIsUnreadable` (own unreadable advice via a directory at the record, so it also runs on Windows) beyond the plan's listed tests; `TestPrime_GarbageLockRecord_GivesNoWorktreeAdvice` is renamed `TestPrime_GarbageLockRecord_AdvisesItIsInvalid`.

**What:**
**Goal:** the Briefing carries the Lock overview, not `Locks` plus a `WorktreeKey` where empty means invalid; no code matches lock error text. One unreadable foreign record leaves `prime` listing the other locks and this worktree's advice, with one `warning: locks: <path>: …` line. `prime` gives two new advice lines, `this worktree: invalid; its lock record does not parse, so acquire fails; ask the user before capcom lock break frees it` and `this worktree: unreadable; its lock record cannot be read; ask the user before editing it`. `prime --json` data is unchanged.
**Approach (assumed at the header base):** `briefing.readLocks` calls `Worktree.Overview()`; each problem becomes one `SectionLocks` failure worded as today; `worktreeLockAdvice` switches on the state. Update `docs/site/agents/briefing.md` (the locks section and the "left out when damaged" sentence) and the prime paragraph of `docs/site/concepts/tree-locks.md`. Depends on P4.1. CHANGELOG line superseding the Unreleased "left out when the record is damaged" wording.
**Regression guard.** Every `got.Locks` and `got.WorktreeKey` assertion in `internal/briefing/briefing_test.go` (lines 114, 126-130, 166, 182, 227, 254, 271) moves to the overview field, nil when the overview failed.
**Files:** internal/briefing/briefing.go; internal/briefing/briefing_test.go; internal/cli/prime.go; internal/cli/prime_test.go; docs/site/agents/briefing.md; docs/site/concepts/tree-locks.md
**Read first:** internal/briefing/briefing.go — Briefing, readLocks, Compose; internal/cli/prime.go — renderBriefing, addLocks, worktreeLockAdvice, primeWarnings, worktreeLockFree;
internal/cli/prime_test.go — TestPrime_GarbageLockRecord_GivesNoWorktreeAdvice, TestPrime_LockHeldInMainWorktree_AdvisesALinkedWorktreeItIsFree; internal/briefing/briefing_test.go — TestCompose_FixtureStore_ReturnsEverySection, TestCompose_BrokenIndexAndLocks_FailInOrderAndKeepTheRest;
internal/cli/lock_test.go — newLockStore, addWorktree, writeLockRecord; docs/site/agents/briefing.md — locks section; CHANGELOG.md — Unreleased prime line
**Tests:** `TestPrime_GarbageLockRecord_GivesNoWorktreeAdvice` becomes a test of the invalid line; a new unreadable-foreign-record prime test (skipped on Windows and root); the Briefing tests' `WorktreeKey` assertions move to the state and every `got.Locks` assertion to the overview.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/briefing/`; `GOMEMLIMIT=2GiB go test -count=1 -run Prime ./internal/cli/`; `grep -rn 'HasPrefix(warning.Error()' internal/briefing` prints nothing
**Commit:** `feat(prime): advice for an invalid or unreadable worktree lock`

## P4.3 lock status reads the Lock overview; one state mapping

**What:**
**Goal:** closes `lock-overview`: cli has one mapping from a lock state to its name, used by `lock status`, `status --all`, `emitLock` and `prime`. `lock status` on an invalid or unreadable own record prints `<key>: invalid` (or `unreadable`) in text, `LOCK=` in `--kv` and `lock` in JSON, warns naming the file, and exits 0. `status --all` warns once per problem and lists the rest. `lock check`'s exit codes are unchanged.
**Approach (assumed at the header base):** add `lockInvalid`/`lockUnreadable` beside the state constants and replace `foundState`; update `lockStatusLong` and regenerate the reference with `go run ./cmd/gendocs`; update "Check and status" in `docs/site/concepts/tree-locks.md`. Remove `Locks.List` if nothing calls it. Depends on P4.2. CHANGELOG line for `lock status`.
**Regression guard.** Remove `Locks.List` and `Worktree.List` once only tests call them, porting the `TestList_*` cases to Overview tests first. Plain `lock status` warns only on its own key's problem; foreign problems are warned only under `--all`.
**Files:** internal/cli/lock.go; internal/cli/lock_test.go; internal/cli/prime.go; docs/site/reference/cli/capcom_lock_status.md; docs/site/concepts/tree-locks.md; internal/lock/lock.go; internal/lock/lock_test.go; internal/lock/worktree.go
**Read first:** internal/cli/lock.go — newLockStatusCommand, foundState, emitLock, allLocksResult, newLockData, lockText, lockStatusLong; internal/cli/lock_test.go — TestLockStatusAll_CorruptRecord_ListsTheRestWithAWarning, TestLockCheck_ExitCodes, TestLock_KVShape_EveryCommandEmitsTheLockRecord, writeLockRecord;
internal/cli/prime.go — addLocks; internal/lock/worktree.go — Worktree.List; internal/lock/lock_test.go — TestList_OneCorruptRecord_ReturnsTheValidLockAndOneWarning;
docs/site/concepts/tree-locks.md — Check and status; cmd/gendocs
**Tests:** first, a test pinning `lock check` exit 1 on a corrupt record; `lock status` on a corrupt own record (text, `--kv`, JSON, exit 0); `status --all` with an unreadable foreign record (skipped on Windows and root); plain `lock status` beside a foreign invalid record prints no warning.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 -run 'Lock|Prime' ./internal/cli/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`; `GOMEMLIMIT=2GiB go test -count=1 ./internal/lock/`; `grep -n 'func foundState' internal/cli/lock.go` prints nothing
**Closes:** lock-overview
**Commit:** `feat(lock): lock status reports invalid and unreadable records`
