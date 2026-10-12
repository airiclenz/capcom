# Glossary and decision Notes plan

**Goal:** Ship capcom's side of ADR 0010 — one-word ids, citation-shaped lookup, hand-editable Notes,
and the optional `retired` / `superseded-by` Note keys at store format 3, with retired subtrees
left out of browsing, search, the Managed block and ref checks.
**Date:** 2026-10-12
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 45d376a

**Sources:**
- `docs/adr/0010-knowledge-holds-a-glossary-and-decision-records-as-note-trees.md`
- `docs/adr/0002-*.md` (2026-10-12 amendment), `docs/adr/0009-*.md`, `docs/adr/0003-*.md`
- `docs/format.md`, `CONTEXT.md` (**Retired**, **Supersession**); Notes `cli-test-harness`, `error-messages`

**Ratified design calls** (user, 2026-10-12):
- **Id floor:** one to four words for every Id; 40 characters stays; aliases unchanged.
- **Citation lookup:** every id/alias/prefix argument is lowercased and each run of whitespace becomes one hyphen before resolving.
- **Retired reach:** `retired: true` hides the Note and its whole subtree from `know tree`, `know search`, `know show --with-children` and the Managed block, and exempts that subtree from ref checks (`know verify`, `check`, Briefing); `--retired` brings them back; `know show <id>` always resolves.
- **mv / rm:** mirror issue `related`: `know mv` rewrites every `superseded-by` naming the old id, `know rm` drops links to the removed Note; `check` reports dangling targets.
- **Write path:** `know update --retire/--unretire/--add-superseded-by/--remove-superseded-by`; the first write of a format-3 key raises the store to 3 like `related add`; `check` reports a format-3 key in a store declaring less.
- **Hand-editing:** Notes may be edited by hand; `capcom check` and `capcom fmt --check` are the validators.
- **Order:** children keep path (alphabetical) order; no `order:` key.

**Standing requirements:**
- skills: coding-standards
- CLI tests go through `runCLI` / `runCLIWith`, never `t.Parallel()` (Note `cli-test-harness`).
- Tests on this host: one package at a time, `GOMEMLIMIT=2GiB go test -count=1 ./internal/<pkg>/`; never `./...` with `-race`; never two test runs at once.
- Every error names the file relative to the worktree, and the line when known (Note `error-messages`).

**Out of scope:**
- Migrating apogee's glossary and ADRs (an apogee plan); capcom moving its own glossary/ADRs.
- Checking links from human docs into `.capcom/knowledge/` (to be filed as a capcom issue).
- A `know split` verb; an `order:` key; a reverse "supersedes" listing on `show`.

**Regression check (2026-10-12, 45d376a):**
- 1: guard folded
- 2: guard folded
- 3: guard folded
- 4: guard folded
- 5: guard folded (writer decision)
- 6: guard folded
- 7: guard folded (writer decision); yields to `docs/format.md` §The Managed block in AGENTS.md ("rendered from the Note files alone") and the `internal/check/check.go` package comment ("never from the index")
- 8: guard folded; yields to `docs/adr/0009-the-store-declares-its-format-version.md` ("A missing key means format 1")
- 9: guard folded
- 10: guard folded (with the writer decision on the rollback test)
- 11: guard folded
- 11: rejected — add `grep -n 'capcom fmt --check' docs/format.md` (finding a line) to Acceptance to check the validator sentence; it already matches `docs/format.md:168` at 45d376a, so it cannot fail

## 1. Ids may be one word — ✅ DONE (2026-10-12)

**What:**
**Goal:** a one-word kebab slug (`turn`, `driver`) is a valid Id for issues and Notes; five words or more, or over 40 characters, stays invalid; every message, flag help and doc states "1 to 4 words".
**Approach (assumed at the header base):** `internal/tree/tree.go` `idPattern` quantifier `{1,3}` → `{0,3}`; update the `ErrInvalidID` and `ValidateID` doc comments and the error string (`must be 1 to 4 words of a-z and 0-9 joined by single hyphens`); flag help in `internal/cli/create.go` and `internal/cli/know.go` (`a kebab slug of 1 to 4 words`); `docs/format.md` §Ids and `docs/migrating-from-beads.md` say one to four. Every pinned string updates; the rule that finds them: `grep -rn '2 to 4 words\|2-4 word\|two to four' --include='*.go' --include='*.md' .` outside `docs/adr/` and `docs/plans/`.
**Regression guard.** The stale-wording rule and the Acceptance grep use `'2 to 4 words\|2-4 word\|two to four'` (excluding `docs/adr/` and `docs/plans/`): a bare `2 to 4` also matches `internal/frontmatter/decode_test.go`'s "lines 2 to 4" and misses `tree.go`'s "2-4 word" `ErrInvalidID` comment. `TestValidateID` already holds `{"login", false}` and the five-word false row `fix-the-login-timeout-race`: flip the `login` row to true; add no second `login` row and no new five-word row.
**Files:** internal/tree/tree.go, internal/tree/tree_test.go, internal/cli/create.go, internal/cli/know.go, internal/cli/check_test.go, internal/check/check_test.go, internal/cli/write_test.go, internal/cli/know_test.go, docs/format.md, docs/migrating-from-beads.md
**Read first:** internal/tree/tree.go — idPattern, ValidateID, ErrInvalidID, treeWalker.walk; internal/tree/tree_test.go — TestValidateID, TestWalk_DirectoryProblems_WordedAndLeftOut; internal/cli/write_test.go — TestCreate_BadSlug_FailsInvalid; internal/cli/know_test.go — TestKnowAdd_RefusedInputs_FailWithoutWriting, newKnowStore;
internal/cli/check_test.go; internal/check/check_test.go; internal/cli/create.go — newCreateCommand flag help; internal/cli/know.go — know add flag help
**Tests:** `TestValidateID` flips its `{"login", false}` row to true (its five-word false row already exists); `TestCreate_BadSlug_FailsInvalid` drops `single`; `TestKnowAdd_RefusedInputs_FailWithoutWriting` replaces its short-id row with a five-word id; new `TestKnowAdd_OneWordID_Writes` (`know add --id turn` writes `turn.md`).
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/tree/` then `./internal/check/` then `./internal/cli/`; `grep -rn '2 to 4 words\|2-4 word\|two to four' --include='*.go' --include='*.md' . | grep -v 'docs/adr/\|docs/plans/'` prints nothing.
**Commit:** `feat(tree): allow one-word ids`

## 2. Citations resolve as typed — ✅ DONE (2026-10-12)

**What:**
**Goal:** `capcom know show "ADR 0076"` resolves Note `adr-0076`, `capcom know show "Floor guard"` resolves `floor-guard`, and `capcom show "Login Timeout Race"` resolves an issue `login-timeout-race`; normalisation trims leading and trailing whitespace, then lowercases and turns each whitespace run into one hyphen, applied before exact-id, alias and prefix matching; ambiguity and not-found messages quote the argument as typed.
**Approach (assumed at the header base):** one exported function in `internal/tree` (e.g. `NormalizeArg`) called at the top of both resolvers — `internal/tree/resolve.go` `Tree.Resolve` and `internal/index/query.go` `resolveID` — so every verb taking an id inherits it; no CLI command normalises on its own. Dots and hyphens are untouched (legacy aliases like `apogee-kk0.4`). `docs/format.md` §Naming a node gains one sentence stating the rule.
**Regression guard.** Normalisation trims first, so `"  Floor   guard "` gives `floor-guard`, never `-floor-guard-`; an argument of whitespace only trims to `""` and keeps today's not-found, quoted as typed. There is no `internal/cli/show_test.go`: the issue `show` test goes in `internal/cli/read_test.go` (`newReadStore`, beside `TestShow_AliasAndPrefix_ResolveTheIssue`) over a store holding a `login-timeout-race` issue. The rule sentence goes in `docs/format.md` §Naming a node, which states the resolution order — not §Ids.
**Files:** internal/tree/resolve.go, internal/tree/resolve_test.go, internal/index/query.go, internal/index/notes_test.go, internal/cli/know_test.go, internal/cli/read_test.go, docs/format.md
**Read first:** internal/tree/resolve.go — Tree.Resolve, Tree.Lookup, AmbiguousError; internal/index/query.go — Index.resolveID, Index.resolved, Index.Resolve; internal/index/notes.go — ResolveNote; internal/cli/show.go — resolveIssue, resolveFailure;
internal/cli/dep.go — depRemoval; internal/cli/related.go — relatedRemoval; internal/tree/resolve_test.go — TestResolve_NoMatch_NotFound, resolveFixture; internal/cli/read_test.go — TestShow_AliasAndPrefix_ResolveTheIssue, newReadStore
**Tests:** unit table for the normaliser (`"ADR 0076"`, `"  Floor   guard "`, `"apogee-kk0.4"`, `"Login\tFlow"`); `TestKnowShow_CitationSpelling_Resolves` (`know show "ADR 0076"` prints `id: adr-0076`); an issue `show "Login Timeout Race"` CLI test in `read_test.go` over a `login-timeout-race` issue; a not-found test asserting the message quotes `"ADR 9999"` as typed.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/tree/` then `./internal/index/` then `./internal/cli/`
**Commit:** `feat(resolve): resolve citation spellings of ids`
Depends on item 1.

## 3. Notes carry `retired` and `superseded-by` — ✅ DONE (2026-10-12)

NOTES (2026-10-12): the Note field is `IsRetired bool`, not the plan's `Retired bool`, per the coding-standards rule that booleans take an affirmative `is`/`has` prefix (the repo's own `IsReady` field follows it).
NOTES (2026-10-12): `retired` is read in `internal/knowledge/note.go` (a `retired` helper using `Field.Scalar`/`Field.Errorf`, like issue `priority`), so no bool helper was added to `internal/frontmatter/decode.go`; an empty `retired:` reads as false, as every other helper reads an empty value as its zero value.
NOTES (2026-10-12): `Note.SupersededByLine(i)` exposes each entry's line (kept like `refLines`) for later check citations; `canonicalNote` in `internal/nodestore/kind_test.go` now carries `retired: true` and `superseded-by: [new-rule]`.

**What:**
**Goal:** a Note's frontmatter may carry `retired: true` and `superseded-by: [ids]`; both optional; `retired` accepts only `true` or `false` and is serialized only when true; `superseded-by` entries pass `ValidateAlias` syntax, carry no duplicates and never name the Note's own id; canonical key order is title, labels, refs, retired, superseded-by, aliases; the unknown-key hint lists all six keys.
**Approach (assumed at the header base):** `internal/knowledge/note.go` — new key constants, `Note` fields `Retired bool`, `SupersededBy []string` (keep their source lines for check citations as `refLines` does), extend the `fields` table (write `retired` via the encoder's `Raw`, as issue `priority` does), `Parse`/`Validate`; a bool helper in `internal/frontmatter/decode.go` only if no existing helper fits. Update every pin of the Note key list: the `note.go` key-hint comment, `decode.go` comments, `internal/frontmatter/decode_test.go` `noteKind`, `internal/nodestore/kind_test.go` `canonicalNote`.
**Regression guard.** The self-reference rule takes the Note's own id from the base name of `path` without `.md` — every caller passes the Note's file (index `parseNote`, `check`, nodestore `write` and `Format`). `Validate(path, note)` keeps its signature, which the nodestore `noteCodec` field `Validate: knowledge.Validate` pins.
**Files:** internal/knowledge/note.go, internal/knowledge/knowledge_test.go, internal/frontmatter/decode.go, internal/frontmatter/decode_test.go, internal/nodestore/kind_test.go
**Read first:** internal/knowledge/note.go — fields, listField, Parse, Validate, Serialize; internal/frontmatter/decode.go — Field.Scalar, Field.Errorf, keysHint; internal/frontmatter/encode.go — Encoder.Raw; internal/knowledge/knowledge_test.go — TestParse_UnknownKey_NamesTheNoteKeys, TestSerialize_ThenParse_RoundTrips;
internal/frontmatter/decode_test.go — noteKind, decode (its hint pin rides with noteKind); internal/tree/tree.go — ValidateAlias; internal/index/notes.go — parseNote (a Validate problem keeps the Note out of the index); internal/nodestore/codec.go — noteCodec
**Tests:** round-trip parse/serialize with both keys; `retired: false` parses and is dropped on serialize; `retired: yes` fails naming file and line; self-reference (own id from the file's base name) and duplicate `superseded-by` fail; `TestParse_UnknownKey_NamesTheNoteKeys` expects the six keys.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/knowledge/` then `./internal/frontmatter/` then `./internal/nodestore/`
**Commit:** `feat(knowledge): add retired and superseded-by note keys`

## 4. Store format 3 — ✅ DONE (2026-10-12)

**What:**
**Goal:** the binary reads stores up to format 3; `capcom init` writes `format: 3`; the format bullet in `docs/format.md` says format 3 adds the Note keys `retired` and `superseded-by`; a store declaring 4 fails as too new with the existing message.
**Approach (assumed at the header base):** `internal/config/config.go` `FormatVersion = 3` and its comment, `DefaultFile`'s literal; tests that use 3 as "too new" move to 4 (`config_test.go` `TestParse_NewerFormat_FailsAsTooNewWithLine`, `internal/cli/read_test.go` `newerFormatMessage`/`writeNewerFormatConfig`, used by `prime_test.go`); `TestRaiseFormat_FormatOneTemplate_BecomesCurrentTemplate` raises to `FormatVersion`; `init_test.go` expects `format: 3`; `docs/format.md` config sample and format bullet.
**Regression guard.** `TestParse_DefaultFile_DeclaresCurrentFormat` (pins `Format != 2` and `"\nformat: 2\n"`) and the helper `formatOneTemplate` (fatal unless `DefaultFile` holds `"\nformat: 2\n"`, taking `TestRaiseFormat_FormatOneTemplate_BecomesCurrentTemplate` with it) are pinned to `FormatVersion` (`fmt.Sprintf("\nformat: %d\n", FormatVersion)`), never to the literal 3. `docs/migrating-from-beads.md` ("The store `capcom init` writes is format 2") says init writes the current format, at least 2; `CONTEXT.md` **Format version** adds that format 3 added `retired`/`superseded-by`.
**Files:** internal/config/config.go, internal/config/config_test.go, internal/cli/read_test.go, internal/cli/init_test.go, docs/format.md, docs/migrating-from-beads.md, CONTEXT.md
**Read first:** internal/config/config.go — FormatVersion, DefaultFile, parser.format, RaiseFormat; internal/config/config_test.go — TestParse_DefaultFile_DeclaresCurrentFormat, formatOneTemplate, TestParse_NewerFormat_FailsAsTooNewWithLine, TestRaiseFormat_FormatOneTemplate_BecomesCurrentTemplate;
internal/cli/read_test.go — newerFormatMessage, writeNewerFormatConfig; internal/cli/prime_test.go; internal/cli/init_test.go; internal/cli/related.go — relatedFormatVersion, raiseStoreFormat; docs/format.md §Config
**Tests:** the updated config and init tests above, `TestParse_DefaultFile_DeclaresCurrentFormat` and `formatOneTemplate` pinned to `FormatVersion`; a format-2 store still loads.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/config/` then `./internal/cli/`
**Commit:** `feat(config): raise the store format to 3`

## 5. The index knows retired Notes and their successors — ✅ DONE (2026-10-12)

NOTES (2026-10-12): the Tests line's `Retired` field is named `IsRetired` on NoteEntry, matching knowledge.Note.IsRetired and the house affirmative-boolean rule; the include-retired switch is the parameter `isRetiredIncluded bool` on Notes and SearchNotes.
NOTES (2026-10-12): ancestor hiding is a recursive CTE over notes.parent among usable (non-excluded) Notes only, so a parent kept out as a duplicate passes no hiding to its children; schemaVersion 4 -> 5.

**What:**
**Goal:** the index records each Note's own `retired` flag and its `superseded-by` ids; `Index.Note` returns them; `Notes` and `SearchNotes` take an include-retired switch and, when off, leave out every Note that is retired or has a retired ancestor; `ResolveNote` resolves retired Notes always.
**Approach (assumed at the header base):** `internal/index/schema.go` — `notes.retired` column, a `supersedes(path, from_id, to_id)` row table registered in `rowTables`, `schemaVersion` + 1; `internal/index/notes.go` `parseNote`/`insertNote`, `NoteEntry` fields, `Notes`/`SearchNotes` signatures. Ancestor hiding is computed at query time (a recursive CTE over `parent`, or a Go pass over the parent map) — never stored per row, since an ancestor's file can change alone. Update every caller of `Notes`/`SearchNotes` to pass include-retired true for now (items 6 and 7 switch them).
**Regression guard.** the only callers of index Notes/SearchNotes are in internal/cli/know.go — drop internal/agents/block.go and internal/briefing/briefing.go from item 5's Files and Approach
**Files:** internal/index/schema.go, internal/index/notes.go, internal/index/notes_test.go, internal/cli/know.go
**Read first:** internal/index/notes.go — NoteEntry, insertNote, Index.Note, Notes, SearchNotes, noteList, parseNote; internal/index/schema.go — schemaStatements, rowTables, schemaVersion; internal/index/query.go — resolveID; internal/index/refresh.go — rowTables clear loop;
internal/index/notes_test.go — writeNote, noteFixture, noteIDs, TestNotes_KnowledgeTree_IndexedInTreeOrderWithParentsAndLists; internal/cli/know.go — outline, newKnowShowCommand, newKnowSearchCommand, verifyNotes
**Tests:** `TestNotes_RetiredSubtree_LeftOutUnlessIncluded` (retired parent, current child: both hidden; included with the switch); search hit under a retired ancestor hidden; `Index.Note` returns `Retired` and `SupersededBy`; changing only the parent's file re-hides the child.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/` then `go build ./...`
**Commit:** `feat(index): index retired notes and their successors`
Depends on item 3.

## 6. Browsing hides retired subtrees

**What:**
**Goal:** `capcom know tree`, `capcom know search` and `capcom know show --with-children` leave retired subtrees out unless `--retired` is given; `capcom know show <id>` of a retired Note works and prints `retired: true` and `superseded-by: <ids>` lines (text) and `retired` / `superseded_by` fields (JSON; `retired` omitted when false, `superseded_by` always present); the envelope `SchemaVersion` is unchanged.
**Approach (assumed at the header base):** `internal/cli/know.go` — a shared `--retired` flag constant on the three commands, passed to `Notes`/`SearchNotes`; `noteText` and `noteData` gain the two fields after `refs`. Flag help: `include retired notes and everything below them`.
**Regression guard.** The JSON field is `superseded_by`, matching every multiword `--json` key (`rm.go` `deps_dropped`, `prime.go` `ready_planned`); the text line stays `superseded-by:`. `superseded_by` is always present (`nonNil`, like `aliases`), keeping `noteData`'s "Lists are always present" contract; only `retired` is omitted when false.
**Files:** internal/cli/know.go, internal/cli/know_test.go
**Read first:** internal/cli/know.go — noteData, newNoteData, noteText, outline, childrenByParent, newKnowTreeCommand, newKnowShowCommand, newKnowSearchCommand; internal/cli/know_test.go — writeNoteFiles, newKnowStore, envelopeIDs use in TestKnowShow_WithChildren_ShowsTheNoteThenItsDirectChildren, TestKnowShow_Text_PrintsFrontmatterThenBody;
internal/cli/output.go — SchemaVersion; internal/cli/rm.go — issueRemoveResult (snake_case precedent)
**Tests:** CLI tests for each of the three commands with and without `--retired`; `know show` of a retired Note prints both lines; JSON `--json know show` carries `retired: true` and `superseded_by` (an empty `superseded_by` list for a Note without the key).
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 -run 'TestKnow' ./internal/cli/`
**Commit:** `feat(know): hide retired notes from browsing unless asked`
Depends on item 5.

## 7. Retired subtrees leave the Managed block and ref checks

**What:**
**Goal:** a retired top-level Note is absent from the Managed block; Notes in a retired subtree are exempt from broken-ref reporting in `capcom know verify`, `capcom check` and the Briefing's `broken note refs` count; their frontmatter is still validated by `check`.
**Approach (assumed at the header base):** `internal/agents/block.go` `TopNotes` skips a parsed top-level Note whose `Retired` is true (no index); the Briefing inherits `check`'s exemption; `internal/cli/know.go` verify path passes include-retired false; `internal/check/check.go` `notes()`/`checkNote` skips `knowledge.Verify` for a Note that is retired or under a retired ancestor (resolved from the loaded Knowledge tree, not the index).
**Regression guard.** the Managed block's TopNotes reads Note files, so it detects retired from the parsed Note (and a retired top-level Note drops its whole entry); the Briefing's broken-ref count comes from check.Run, so the check exemption covers the Briefing and internal/briefing needs a change only if its own test must pin it — keep briefing_test.go as the pin, drop briefing.go from Files unless the code needs it. This keeps the file-only reads of `docs/format.md` §The Managed block ("rendered from the Note files alone") and the `internal/check/check.go` package comment ("never from the index"); the item yields to them.
**Files:** internal/agents/block.go, internal/agents/block_test.go, internal/briefing/briefing_test.go, internal/check/check.go, internal/check/check_test.go, internal/cli/know.go, internal/cli/know_test.go
**Read first:** internal/agents/block.go — TopNotes, Sync; internal/check/check.go — notes, checkNote, agentsBlock; internal/cli/know.go — verifyNotes, refreshAgentsAfterWrite; internal/briefing/briefing.go — readHealth; internal/agents/block_test.go — TestTopNotes_KnowledgeTree_ListsTopLevelInIDOrderSkippingUnparsable;
internal/briefing/briefing_test.go — TestCompose_FixtureStore_ReturnsEverySection; internal/check/check_test.go — TestRun_OneFixturePerProblemClass_ReportsPathLineAndCode
**Tests:** Managed block omits a retired top-level Note; a broken ref under a retired parent is not reported by `know verify`, `check` or `prime`; a malformed key in a retired Note is still reported by `check`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/agents/` then `./internal/briefing/` then `./internal/check/` then `-run 'TestKnowVerify' ./internal/cli/`
**Commit:** `feat(knowledge): exempt retired subtrees from the block and ref checks`
Depends on item 5.

## 8. check validates supersession and the store format

**What:**
**Goal:** `capcom check` reports, citing the key's line: a `superseded-by` entry naming no Note by id or by an alias only one Note has (code `missing-superseded-by`, worded as `superseded-by %q names no note by id or by an alias only one note has`); and a Note using `retired` or `superseded-by` in a store whose `config.yaml` declares a format below 3 (new code `format-too-old`, message naming the key and telling the user to run a write verb or set `format: 3`).
**Approach (assumed at the header base):** `internal/check/check.go` — two new codes in the closed code set; `checkNote` gains a resolver over the Knowledge `tree.Tree` (`Lookup` only), mirroring `checkRelated`; the declared format comes from the loaded config, a missing `config.yaml` counting as format 1.
**Regression guard.** Each `superseded-by` entry resolves with `tree.Lookup` only (exact id, then alias), as `checkRelated` takes exact id or alias — never `tree.Resolve`, whose prefix (and, after item 2, citation) matching lets a link pass, then break when a second Note shares the prefix. `format-too-old` fires on item 3's kept key line (line > 0), not on `Retired == true`, so `retired: false` counts. A store without `config.yaml` is format 1 (ADR 0009: "A missing key means format 1"), though `store.loadConfig` maps it to `config.Default()` at `FormatVersion`.
**Files:** internal/check/check.go, internal/check/check_test.go
**Read first:** internal/check/check.go — checkNote, notes, checkRelated, CodeMissingRelated; internal/tree/resolve.go — Lookup, Resolve; internal/store/store.go — loadConfig, RaiseFormat; internal/config/config.go — Default, FormatVersion;
internal/check/check_test.go — TestRun_OneFixturePerProblemClass_ReportsPathLineAndCode, TestRun_RelatedNamingAnAlias_ReportsNothing, newRepo
**Tests:** rows in `TestRun_OneFixturePerProblemClass_ReportsPathLineAndCode` for both codes — the `missing-superseded-by` row writes a `.capcom/config.yaml` of `format: 3` (`newRepo` writes none; each row expects exactly one problem) — plus a `retired: false` row in a format-2 store and a `format-too-old` test for a store without `config.yaml`; a `superseded-by` naming an alias reports nothing; one naming only a unique id prefix is `missing-superseded-by`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/check/`
**Commit:** `feat(check): validate supersession links and their store format`
Depends on items 3 and 4.

## 9. know update writes the new keys

**What:**
**Goal:** `capcom know update <id>` accepts `--retire`, `--unretire`, `--add-superseded-by <id|alias>` (repeatable; resolved at write time to the target's id, refused when it names no Note or the Note itself) and `--remove-superseded-by <id>`; `--retire` with `--unretire` is refused; the first such write in a store below format 3 raises it to 3 and prints the existing format-raised warning.
**Approach (assumed at the header base):** `internal/cli/know_update.go` — flags, `hasAnyFlag`, `applyNoteUpdate`; resolution through the index resolver as `related add` does; raise via `raiseStoreFormat` with a named target format 3 for these keys (`internal/cli/related.go` holds the precedent).
**Regression guard.** The target format is a named constant 3 beside `relatedFormatVersion` in `internal/cli/related.go`, never `config.FormatVersion`. The raise runs inside the `editNode` callback, after `Kind.Edit`'s etag check, and only when the edited Note gains `retired: true` or a new `superseded-by` entry, as `related add` does — so `--unretire`, `--remove-superseded-by`, a no-op write and an etag conflict never rewrite `config.yaml`.
**Files:** internal/cli/know_update.go, internal/cli/know_update_test.go, internal/cli/related.go
**Read first:** internal/cli/know_update.go — newKnowUpdateCommand, updateNote, applyNoteUpdate, noteResolver; internal/cli/related.go — raiseStoreFormat, relatedFormatVersion, formatRaisedWarning, newRelatedAddCommand; internal/cli/update.go — editNode; internal/nodestore/edit.go — Kind.Edit;
internal/cli/know_update_test.go — TestKnowUpdate_RefusedEdits_NameTheFileAndWriteNothing, TestKnowUpdate_NoChangeOrClashingFlags_AreUsageErrors; internal/cli/related_test.go — formatRaisedText
**Tests:** each flag writes the expected frontmatter; alias input stores the canonical id; unknown and self targets refused without writing; format-2 store raised to 3 with the warning on stderr; `--unretire`, `--remove-superseded-by` and a stale `--if-match` on a format-2 store leave `config.yaml` unchanged; `--retire --unretire` refused.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 -run 'TestKnowUpdate' ./internal/cli/`
**Commit:** `feat(know): write retired and superseded-by from know update`
Depends on items 3 and 4.

## 10. know mv and know rm keep supersession links whole

**What:**
**Goal:** `capcom know mv <old> <new>` rewrites every other Note's `superseded-by` entry naming the old id; `capcom know rm <id>` removes the id from every `superseded-by` naming it; both stay atomic with the existing rollback.
**Approach (assumed at the header base):** `internal/knowledge/move.go` `PlanMove` and `internal/knowledge/remove.go` `PlanRemove` gain the dependent-file rewrites the issue side does in `internal/issue/move.go` / `remove.go` (match on id plus aliases read from the files, the rule mv already uses); drop the "Notes have no deps" comment.
**Regression guard.** the rollback test runs at CLI level through a read-only directory, skipped on Windows and when running as root, as TestRelatedAdd_RaiseFails_FailsInvalidAtTheConfig in internal/cli/related_test.go does. An unparseable other Note joins Unread and is skipped, as on the issue side (`issue/move.go` `planRewrite`, `issue/remove.go` `planDrops`): plumbed through nodestore `planNoteMove` / `planNoteRemove` and warned in `know_mv.go` / `know_rm.go` as `cli/mv.go` / `cli/rm.go` do. `planNoteRemove` reports the rewritten Note ids so `removalPaths` names a failing dependent file. A rename keeps only the first entry that becomes the new id (`renameDeps`'s rule), so `[x, y]` renamed x→y is `[y]`. Every comment saying a Note move/removal changes no other file goes — `grep -rn -i 'notes have no\|knowledge tree has neither\|no other file changes' internal`, plus the line-wrapped "the Knowledge tree has neither" in nodestore `remove.go`'s `Remove` comment.
**Files:** internal/knowledge/move.go, internal/knowledge/remove.go, internal/knowledge/knowledge_test.go, internal/nodestore/codec.go, internal/nodestore/remove.go, internal/cli/know_mv.go, internal/cli/know_rm.go, internal/cli/know_mv_test.go, internal/cli/know_rm_test.go
**Read first:** internal/knowledge/move.go — PlanMove; internal/knowledge/remove.go — PlanRemove; internal/issue/move.go — planRewrites, planRewrite, renameDeps; internal/issue/remove.go — PlanRemove, planDrops, dropEntries; internal/nodestore/codec.go — planNoteMove, plannedMove;
internal/nodestore/remove.go — planNoteRemove, plannedRemoval, removalPaths, removalApplyFailure; internal/cli/mv.go — move (Unread warning); internal/cli/know_rm.go — removeNote
**Tests:** mv rewrites a `superseded-by` in another Note; rm drops the link; a failing write rolls every file back (CLI level, read-only directory, skipped on Windows and as root); an unparseable bystander Note is warned as unread and fails neither verb; renaming x to y beside `superseded-by: [x, y]` leaves `[y]`; a failed dependent rewrite during rm names that Note's file.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/knowledge/` then `-run 'TestKnowMv|TestKnowRm' ./internal/cli/`
**Commit:** `feat(knowledge): keep supersession links through mv and rm`
Depends on item 3.

## 11. Docs: hand-editable Notes and the new keys

**What:**
**Goal:** `docs/format.md` §Note files lists the six keys with `retired` and `superseded-by` semantics (subtree hiding, `--retired`, Managed block and ref-check exemption, mv/rm behaviour), says Notes may be edited by hand with `capcom check` and `capcom fmt --check` as validators, and keeps the etag paragraph accurate for hand edits; `README.md`'s Knowledge bullets and example mention glossary and decision trees and `--retired`.
**Approach (assumed at the header base):** rewrite the hand-edit ban sentence in §Note files into "edit by hand or through the write verbs"; extend the key table and the "Any other key" paragraph; README lines describing Notes. The rule for finding stale prose: every sentence in `docs/` and `README.md` saying Notes change only through capcom (`grep -rn 'hand-edit\|by hand' docs README.md`).
**Regression guard.** The stale-prose rule is every sentence in `docs/` and `README.md` about `know tree`/`search`/`show`, the Managed block, ref checks by `know verify`/`check`/`prime`, `know update` flags or `know mv`/`rm` — not only `hand-edit\|by hand` (`docs/format.md`'s "per top-level Note", the block-refresh list's "title", "lists every such Ref", "title, labels, refs or body"). The new check codes `missing-superseded-by` and `format-too-old` are documented as §Related links documents `missing-related`.
**Files:** docs/format.md, README.md
**Read first:** docs/format.md — §Note files, §Refs, §The Managed block in AGENTS.md, §Related links; README.md — Project knowledge on demand bullet, know command example; CONTEXT.md — Managed block
**Tests:** none (docs only).
**Acceptance:** `grep -n 'never by hand-editing' docs/format.md` prints nothing; `grep -n 'superseded-by' docs/format.md README.md` finds both files; `grep -n -- '--retired' README.md docs/format.md` and `grep -n 'glossary' README.md` each find a line.
**Commit:** `docs: describe hand-editable notes, retired and superseded-by`
Depends on items 6–10.
