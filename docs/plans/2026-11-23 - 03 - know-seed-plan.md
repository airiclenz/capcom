# capcom know seed: a base set of Notes, and keeping it current

**Goal:** `capcom know seed` adds a base set of Knowledge Notes, embedded in the binary as markdown
files, to any store without overwriting a Note. `capcom know seed --update` and a `capcom check`
warning let a seeded repository take a Note that a newer capcom corrected.
**Date:** 2026-11-23
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 092b25e
**Closes:** know-seed; seed-drift-check

**Sources:**
- `.capcom/issues/know-seed.md` (decisions, the Note texts under "The base set", acceptance)
- `.capcom/issues/seed-drift-check.md`
- Notes `cli-test-harness`, `error-messages`, `coding-conventions`, `docs-layout`, `label`, `knowledge`

**Ratified design calls:**
- **Set, wording, ids, placement, precedence:** as `know-seed` "Decided" and "The base set" (the user, 2026-11-23).
- **Markdown, not code:** the set is markdown files embedded as a directory; a Note is added or dropped with no Go file changed, tests included (the user, 2026-11-23).
- **Edit detection:** a hash of the seeded title and body is stored with the Note at seed time (the user, 2026-11-23).
- **Seed mark:** two Labels, `seed:<root id>` and `seed-text:<version>:<hash>`; no store format change (the user, 2026-11-23).
- **Drift mention:** `capcom check` only, never `capcom prime` (the user, 2026-11-23); as a `warning:` line, since check has no other non-failing channel (writer).
- **Update:** `capcom know seed --update` rewrites only a Note whose text is still what was seeded; this supersedes "No update logic" in `know-seed` (the user, 2026-11-23).
- **Writes:** one Note at a time, parent first; a failure stops and a re-run finishes (the user, 2026-11-23).
- **Decision record:** ADR 0020 and a glossary term for the base set (the user, 2026-11-23).
- **Retired root:** when the root Note is retired, seed adds nothing and reports every Note `skipped (retired)` (writer).
- **Outcomes:** `added`, `updated`, `skipped (same)`, `skipped (differs)`, `skipped (outdated)`, `skipped (retired)`, `skipped (alias of <id>)` (writer, from the `know-seed` acceptance).

**Standing requirements:**
- skills: coding-standards
- CLI tests go through `runCLI`, never `t.Parallel()` (Note `cli-test-harness`).
- Tests on this host: `GOMEMLIMIT=2GiB`, one package at a time, never `./...` with `-race`.
- A help-text or flag change regenerates the committed reference: `go run ./cmd/gendocs`.
- No Go file, test files included, names a base Note's id, title, text or the count of Notes.
- An authorized deviation from item text lands as a dated NOTES line under the item.

**Regression check (2026-11-23, 092b25e):**
- 1: guard folded — `seed.LoadFS` is exported (writer's decision); `Load` refuses an id `tree.ValidateID` refuses.
- 3: guard folded — the golden of kind `know-seed` runs against a made-up set through a loader field on the `app` (writer's decision); ADR 0002 holds: seeded ids are chosen slugs, none generated.
- 4: guard folded — the golden of kind `know-seed-show` runs against the made-up set of item 3 (writer's decision).
- 5: guard folded — goldens on the made-up set (writer's decision); the rewrite goes through `editNode`, not `updateNote`.
- 6: guard folded — fixtures from `seed.Load()`, `Set.Labels` and `seed.Hash` (writer's decision); the warning is on stdout, yields to `docs/site/reference/json-output.md`; the checker's own read.
- 7: guard folded — Notes read from `seed.Load()` (writer's decision); a missing subcommand of a group, inherited flags and `--help`.
- 8: guard folded — Acceptance greps for the flags and Labels; the `docs/site/concepts/knowledge.md` description changes with the `llms.txt` entry.
- 9: guard folded — `adr-0020` joins the list in the Note `decisions`; the title of `knowledge` is updated and checked; `adr-0020-decision` says ADR 0002 holds (writer's decision).

**Out of scope:**
- Seeding this repository's own store.
- A store format change, a new frontmatter key, a `capcom prime` line.
- Seeding from a path or URL; any change to `capcom init`.
- All-or-none staging of Note writes.

## 1. Embed the base set as markdown files — ✅ DONE (2026-11-23)

NOTES (2026-11-23): `seed.LoadFS` also refuses, naming the file, a file that carries `superseded-by` and a file whose name does not end in `.md`; the item lists neither. The embed uses `all:`, so a dot or underscore file reaches the loader and must fail there, and a `superseded-by` entry is as foreign to a set Note as `retired`.
NOTES (2026-11-23): "carries `retired`" is read as the key being present: `retired: false` is refused too.
NOTES (2026-11-23): with no Note file at the top of the file system there is no file to name; the error names the directory `.`.
NOTES (2026-11-23): the eleven Note files were written by a throwaway program (deleted, not in the tree) that copied title and body from `.capcom/issues/know-seed.md` and wrote each with `knowledge.Note.Serialize`; a title with a colon is therefore double-quoted in the YAML, and each body follows one blank line after the frontmatter, as `capcom know add` writes it.

**What**
**Goal:** package `internal/seed` holds the base set as markdown files and loads them from the
binary: `seed.Load()` returns the Notes of the set, root first, each with its id, parent id, title
and body, and no Go file names any of them.
**Approach (assumed at the header base):** new package `internal/seed`, one deep module; it imports
`internal/knowledge` and `internal/tree` only. The files live in `internal/seed/notes/` in a store's
Knowledge layout: `baseline.md`, children in `baseline/`. Each file is a Note file with a `title`
key and the body; no Labels, Refs or Aliases. Title and body are copied verbatim from
`.capcom/issues/know-seed.md`, section "The base set" (bodies de-indented by four spaces).
`//go:embed all:notes` as an `embed.FS`; `Load` walks it with `io/fs`, derives the id with
`tree.IDOf` and the parent from the directory, parses with `knowledge.Parse`, and fails naming the
file when: a file does not parse or validate, two files share an id, there is not exactly one root,
a child id lacks the `<root id>-` prefix, a file carries Labels, Refs, Aliases or `retired`, or a
file sits outside the root's subtree. `Set` exposes `Root()`, `Notes()` (root first, then ids
sorted) and `Note(id)`.

**Regression guard.** the loader that takes an `fs.FS` is exported as `seed.LoadFS(fsys fs.FS) (Set, error)`, and `seed.Load()` is `LoadFS` over the embedded directory, so a caller's tests can load a made-up set.
`LoadFS` also fails, naming the file, on an id `tree.ValidateID` refuses: `(*nodestore.Kind).Create`
refuses such an id only after the root is written.

**Files:** `internal/seed/doc.go`, `internal/seed/seed.go`, `internal/seed/seed_test.go`, `internal/seed/notes/baseline.md`, `internal/seed/notes/baseline/*.md` (ten files)
**Read first:** internal/knowledge/note.go — Parse, Validate, Note; internal/tree/tree.go — IDOf; internal/tree/ids.go — ValidateID, maxIDWords; .capcom/issues/know-seed.md — "The base set"; version.go — the go:embed pattern in use

**Tests** (`internal/seed/seed_test.go`): `Load` succeeds on the embedded set and every Note has a
title and a body; each failure above, and an id `tree.ValidateID` refuses (five words), driven
through `seed.LoadFS` (`testing/fstest.MapFS`). No test holds a list or count of the shipped Notes.

**Acceptance**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/seed/`
- `find internal/seed/notes -name '*.md' | wc -l` prints 11, and each file's id is a `### ` heading of "The base set" with the same title and body.
- `grep -rn "baseline" internal/seed/*.go` prints nothing.

**Commit:** `feat(seed): embed the base set of Notes as markdown files`

## 2. Hash, seed Labels and the state of a seeded Note

**What**
**Goal:** `internal/seed` answers, for one Note of the set and the store's Note of that id, which
state it is in: same, outdated or differs; and it builds and reads the two seed Labels.
**Approach (assumed at the header base):** extends `internal/seed`; no new package.
`Hash(title, body)`: the first 12 hex digits of SHA-256 over title, a newline and body, after CRLF
becomes LF and trailing newlines are trimmed. `Set.Labels(version)` returns `seed:<root id>` and
`seed-text:<version>:<hash>` for a Note; `Set.IsSeeded(labels)` reports the first Label.
`Set.State(id, existing knowledge.Note)` returns `Same` when title and body equal the embedded text
(Labels, Refs and line endings ignored), `Outdated` when they differ and the Note's `seed-text`
Label carries the hash of its current text, else `Differs`. A Note without a readable `seed-text`
Label is never `Outdated`.
**Files:** `internal/seed/mark.go`, `internal/seed/mark_test.go`, `internal/seed/doc.go`
**Read first:** internal/knowledge/note.go — Note (Body keeps the blank line after the frontmatter), Parse (CRLF read as LF), Serialize; internal/cli/know_add.go — noteBody; internal/edit/edit.go — NormalizeNewlines, List
Depends on item 1.

**Tests** (`internal/seed/mark_test.go`): hash equal under CRLF and a trailing newline; each state;
an edited Note is `Differs`; a Note whose Labels were removed is `Differs`; a malformed
`seed-text` Label is `Differs`. Test Notes are built from `Set.Notes()`, never named.

**Acceptance**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/seed/`

**Commit:** `feat(seed): hash a seeded Note and tell same, outdated and edited apart`

## 3. capcom know seed, with --dry-run

**What**
**Goal:** `capcom know seed` adds every Note of the base set that the store lacks, each carrying
the two seed Labels, and prints one line per Note with its outcome; `--dry-run` prints the same and
writes nothing.
**Approach (assumed at the header base):** new `internal/cli/know_seed.go`, registered in
`(*app).newKnowCommand`, annotation `indexRefresh`, `Args: cobra.NoArgs`. For each Note of
`seed.Load()`, root first: look the id up in the Knowledge tree as id or Alias
(`tree.Tree.Lookup`). Absent: create it through `a.notes()` and `(*nodestore.Kind).Create` with the
embedded parent id, the path `capcom know add` uses, then `a.noteKindWrite`. Present: never
written; the outcome is `skipped (same)` or `skipped (differs)` from `Set.State` (`Outdated` prints
as `skipped (differs)` in this item), `skipped (retired)` for a retired Note, `skipped (alias of
<id>)` for an Alias. Before any write: a root that exists without the `seed:<root id>` Label fails
with `output.CodeInvalid`, naming its file; a retired root makes every outcome `skipped (retired)`.
A failed write stops the run after printing the outcomes so far, and exits 1. Version from
`capcom.Version()`. Text: one line `<id>  <outcome>`, then `dryRunLine` on a dry run. JSON kind
`know-seed`, `data` `{dryRun, notes: [{id, path, outcome}]}`. No `--kv`. A dry run notes no write.

**Regression guard.** the `app` holds the set loader as a field that defaults to `seed.Load`, and the golden case and golden file of kind `know-seed` run against a made-up set of Notes loaded with `seed.LoadFS` from a `testing/fstest.MapFS`, so no golden file or test holds an id, title, text or count of a shipped Note; the behaviour tests may run against the shipped set but take every expected id from `seed.Load()`.
ADR 0002 holds: an id of the base set is a slug chosen by the set's author and read from its file name; capcom generates none, and seed never derives, renames or numbers an id (`.capcom/knowledge/decisions/adr-0002/adr-0002-decision.md`).

**Files:** `internal/cli/know_seed.go`, `internal/cli/know_seed_test.go`, `internal/cli/know.go`, `internal/cli/root.go`, `internal/cli/doc.go`, `internal/cli/golden_know_test.go`, `internal/cli/testdata/json/know-seed.golden.json`, `docs/site/reference/cli/capcom_know_seed.md`, `docs/site/reference/cli/capcom_know.md`, `docs/site/reference/json-output.md`
**Read first:** internal/cli/know.go — newKnowCommand; internal/cli/node_verbs.go — createNode, noteVerbs; internal/cli/know_add.go — notes, addSyncEffects, noteWriteResult; internal/nodestore/create.go — Kind.Create, newNodePath;
internal/tree/resolve.go — Tree.Lookup; internal/cli/know_show.go — NoteReader use; internal/cli/golden_know_test.go — TestGolden_KnowKinds; internal/cli/lock.go — failedLockResult (a result with a Failure)
Depends on item 2.

**Tests** (`runCLI`): fresh store adds every Note and `capcom check` passes; second run all
`skipped (same)` and no file changes; an edited seeded Note keeps its edit, `skipped (differs)`; a
child id held by the repo's own Note is `skipped (differs)` and the rest are added; an id held as
an Alias; a retired child stays retired; a child removed with `know rm` is added again; a moved
child is skipped where it is; a root without the seed Label fails and writes nothing; `--dry-run`
writes nothing; the agent guide's capcom block gains exactly one line; CRLF text is `same`.
Expected ids come from `seed.Load()`. The golden case of kind `know-seed` runs against a made-up
set (`seed.LoadFS` over a `testing/fstest.MapFS`) put in the `app`'s loader field.

**Acceptance**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'KnowSeed|Golden|HelpText|DocFileMap'`
- `GOMEMLIMIT=2GiB go test -count=1 ./cmd/gendocs/ ./internal/clidoc/`

**Commit:** `feat(cli): add capcom know seed`

## 4. capcom know seed --show

**What**
**Goal:** `capcom know seed --show <id>` prints the embedded title and body of one Note of the base
set and writes nothing; an id outside the set fails and names the ids of the set.
**Approach (assumed at the header base):** extends `internal/cli/know_seed.go`. `--show` excludes
`--dry-run`. Text: `id: <id>`, `title: <title>`, a blank line, the body. JSON kind `know-seed-show`,
`data` `{id, parent, title, body}`. The failure is `output.CodeNotFound` (exit 3). Shell completion
of the flag value lists `Set.Notes()` ids through its own `cobra.CompletionFunc`.

**Regression guard.** the golden case and golden file of kind `know-seed-show` run against the same made-up set as item 3, never a shipped Note.

**Files:** `internal/cli/know_seed.go`, `internal/cli/know_seed_test.go`, `internal/cli/golden_know_test.go`, `internal/cli/testdata/json/know-seed-show.golden.json`, `docs/site/reference/cli/capcom_know_seed.md`, `docs/site/reference/json-output.md`
**Read first:** internal/cli/complete.go — completeFlag, completeIDValue; internal/cli/complete_test.go — TestComplete_FlagValues_ListWhatTheFlagAccepts, runComplete; internal/cli/output/errors.go — CodeNotFound, ExitCode, Error.Valid; internal/cli/golden_kv_test.go — TestGolden_KindsCovered
Depends on item 3.

**Tests** (`runCLI`): `--show` of the root and of a child print the embedded text and leave the
store untouched, also in a store that was never seeded; an unknown id exits 3 and its message holds
every id of the set; `--show` with `--dry-run` is a usage error; completion lists the ids. The
golden case of kind `know-seed-show` shows a Note of the made-up set of item 3, its id read from
that set.

**Acceptance**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'KnowSeed|Golden|HelpText|Complete'`
- `GOMEMLIMIT=2GiB go test -count=1 ./cmd/gendocs/ ./internal/clidoc/`

**Commit:** `feat(cli): print a base Note's embedded text with capcom know seed --show`

## 5. capcom know seed --update

**What**
**Goal:** `capcom know seed --update` replaces the title and body of every seeded Note whose text
is still what was seeded and differs from the embedded text, and never touches an edited Note;
without `--update` such a Note prints `skipped (outdated)`.
**Approach (assumed at the header base):** extends `internal/cli/know_seed.go`. A Note in state
`seed.Outdated` prints `skipped (outdated)`; with `--update` it is rewritten through the write path
`capcom know update` uses (`editNode`, `ops.EditNode`): title and body from the set, the
`seed-text` Label replaced by the new one, every other Label, Ref and Alias kept; outcome
`updated`. `--update` also adds missing Notes as plain seed does. A retired Note is never updated.
`--update --dry-run` reports and writes nothing. `--update` excludes `--show`.

**Regression guard.** any golden case this item adds or changes runs against the made-up set of item 3, never a shipped Note.
The rewrite calls `editNode` (`internal/cli/node_edit.go`) on `a.notes(noteResolver{...})` with the
item's own change func, which assigns the set's Title and Body as parsed; never `updateNote`, whose
`applyNoteUpdate` writes title and body only for flags this command lacks, and never `noteBody`,
which puts a second newline before a parsed Body, so a second `--update` would not be `skipped (same)`.

**Files:** `internal/cli/know_seed.go`, `internal/cli/know_seed_test.go`, `internal/cli/golden_know_test.go`, `internal/cli/testdata/json/know-seed.golden.json`, `docs/site/reference/cli/capcom_know_seed.md`
**Read first:** internal/cli/node_edit.go — editNode, written; internal/cli/know_update.go — updateNote, applyNoteUpdate, noteResolver; internal/nodestore/edit.go — Kind.Edit (sync hook on every Edit); internal/cli/know_add.go — noteBody, addSyncEffects; internal/index/notes.go — NoteEntry
Depends on item 4.

**Tests** (`runCLI`): an outdated Note is built by seeding, then rewriting a seeded file's body
and its `seed-text` hash to match (a test helper using `seed.Hash`); plain seed prints
`skipped (outdated)` and writes nothing; `--update` rewrites it, keeps a Label and a Ref the user
added, and a second `--update` prints `skipped (same)`; an edited Note stays `skipped (differs)`
under `--update`; a retired outdated Note is not updated; `--update --dry-run` writes nothing.
A golden case this item adds or changes runs against the made-up set of item 3.

**Acceptance**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'KnowSeed|Golden|HelpText'`
- `GOMEMLIMIT=2GiB go test -count=1 ./cmd/gendocs/ ./internal/clidoc/`

**Commit:** `feat(cli): replace outdated seeded Notes with capcom know seed --update`

## 6. capcom check warns of an outdated seeded Note

**What**
**Goal:** `capcom check` prints one warning per seeded Note in state outdated, naming its file and
`capcom know seed --update`, and its exit code and problem list are what they are without it; a
store that was never seeded, and an edited Note, produce no warning.
**Approach (assumed at the header base):** new `check.SeedWarnings(*store.Store) ([]string, error)`
in `internal/check/seed.go`, using `seed.Load()` and `Set.State` over the store's Notes; retired
Notes are left out. `internal/cli/check.go` puts them in `output.Result.Warnings`, also when
problems are found. Wording: `<path>: seeded Note <id> is outdated: capcom know seed --update
replaces it, capcom know seed --show <id> prints the new text`. `check.Run`, `check.Problem` and
`briefing.readHealth` are untouched, so `capcom prime` counts no problem for it.

**Regression guard.** test fixtures build seeded and outdated Notes from `seed.Load()`, `Set.Labels` and `seed.Hash` alone, and no test names a base Note.
In text mode the warning is a `warning: <line>` on stdout, before any problem line, and stderr stays empty: the item yields to `docs/site/reference/json-output.md` ("each warning is a `warning:` line printed before the command's result").
`SeedWarnings` reads through the checker's own `loadTree`/`readFile`, skips unparsed and left-out
Notes and fails only where `check.Run` fails; the cli calls it only when `store.Open` succeeded.

**Files:** `internal/check/seed.go`, `internal/check/seed_test.go`, `internal/cli/check.go`, `internal/cli/check_test.go`, `docs/site/reference/cli/capcom_check.md`
**Read first:** internal/cli/check.go — newCheckCommand, checkProblems, checkLong; internal/cli/output/output.go — Result.Warnings, textRenderer.Render; internal/cli/root.go — app.emit; internal/check/notes.go — checker.notes, retiredSubtrees;
internal/check/problem.go — checker.loadTree, checker.readFile; internal/check/check_test.go — newRepo, writeFiles, runCheck; internal/cli/check_test.go — TestCheck_CleanStore_ExitsZeroWithNoOutput, TestCheck_Help_SaysWritesAreValidatedAndWhenToRunIt;
internal/cli/prime.go — "check: no problems"
Depends on item 5.

**Tests**: `internal/check`: no warning for an unseeded store, a freshly seeded one, an edited
Note, a retired outdated Note; one warning for an outdated Note; a store with a problem (an
unparseable Note) plus an outdated Note returns no error. `internal/cli` (`runCLI`): check
exits 0 with the exact line as `warning: <line>` on stdout, stderr empty, and in the `warnings` of
`--json`; a store with a problem plus an outdated Note keeps its problem list and exit 1, the
warning before the problem lines; `capcom prime` still prints `check: no problems`. Fixtures are
built from `seed.Load()`, `Set.Labels` and `seed.Hash`.

**Acceptance**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/check/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'Check|Prime|Golden|HelpText'`
- `GOMEMLIMIT=2GiB go test -count=1 ./cmd/gendocs/ ./internal/clidoc/`

**Commit:** `feat(check): warn of a seeded Note that capcom ships a newer text for`

## 7. Test that the base Notes name only commands and flags that exist

**What** A test-only item. `internal/cli/know_seed_notes_test.go` reads every Note of
`seed.Load()`, finds each backtick span that starts with `capcom `, resolves its words against the
root command tree (`cobra.Command.Find`), and fails when the command does not exist or a `--flag`
in the span is not a flag of that command. A span such as `capcom ...` with a placeholder after the
command (`<id>`, `<word>...`, `<path>`) is resolved up to the placeholder. A bare `--flag` span in
a sentence that names a command earlier in the same bullet is checked against that command.

**Regression guard.** the test reads the Notes from `seed.Load()` and names none of them.
`Find` returns a group and a nil error for a missing subcommand: after `Find`, a leftover word that
is neither a flag nor a placeholder is a missing command on a command that `HasSubCommands`, and an
argument on a leaf (`capcom know tree <literal id>` must pass). A flag is looked up in
`cmd.Flags()` and `cmd.InheritedFlags()`, and `--help` is accepted.

**Files:** `internal/cli/know_seed_notes_test.go`
**Read first:** internal/cli/help_text_test.go — availableCommands, refPaths, TestHelpText_RefExamples_ResolveInRepo; internal/cli/root.go — newRootCommand, withSubcommandDispatch; internal/cli/know.go — newKnowCommand;
.capcom/issues/know-seed.md — "The base set" (spans: `capcom know tree baseline`, `capcom know add --ref <path>`, `--add-superseded-by <id>`, and the git flags `--force-with-lease`, `--no-verify` that no capcom command precedes)
Depends on item 5.

**Tests**: the test itself, plus table cases over its span parser with made-up Note text: a missing
command, a missing subcommand of a group, a literal argument on a leaf command, a missing flag, an
inherited flag (`--json`), `--help` and a placeholder each give the expected result.

**Acceptance**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'KnowSeedNotes'`

**Commit:** `test(cli): check every command and flag the base Notes name`

## 8. Document capcom know seed

**What**
**Goal:** the hand-written docs name `capcom know seed`, its `--dry-run`, `--show` and `--update`,
the two seed Labels and the check warning, wherever they list the Knowledge commands.
**Approach (assumed at the header base):** add the command to the README `# Knowledge` command
block, the `llms.txt` Knowledge line, `docs/site/concepts/knowledge.md` (a short section: what the
base set is, that a seeded Note is the repo's own, never overwritten, how `--update` and the check
warning work, that removing the seed Labels takes a Note out of both) and
`plugins/capcom/skills/capcom/SKILL.md`. `capcom init` help and docs stay as they are.

**Regression guard.** `docs/site/concepts/knowledge.md` carries `--dry-run`, `--show`, `--update`, both seed Labels and the
check warning; the other three files name the command. The `llms.txt` Knowledge entry is the
frontmatter `description` of `docs/site/concepts/knowledge.md` verbatim: change both together, to
the same sentence.

**Files:** `README.md`, `llms.txt`, `docs/site/concepts/knowledge.md`, `plugins/capcom/skills/capcom/SKILL.md`
**Read first:** README.md — "# Knowledge" command block; llms.txt — the Knowledge entry (line 24) and the work-loop paragraph (line 5); docs/site/concepts/knowledge.md — frontmatter description, "## Label", "## Retired Notes and Supersession";
plugins/capcom/skills/capcom/SKILL.md — "## 3. Work loop" (the `capcom know tree` bullet); internal/cli/help_text_test.go — TestHelpText_RefExamples_ResolveInRepo (reads README.md --ref paths)
Depends on item 6.

**Tests**: none new.

**Acceptance**
- `grep -c "know seed" README.md llms.txt docs/site/concepts/knowledge.md plugins/capcom/skills/capcom/SKILL.md` prints a count above 0 for each file.
- `for w in "--dry-run" "--show" "--update" "seed:" "seed-text" "capcom check"; do grep -c -e "$w" docs/site/concepts/knowledge.md; done` prints six counts above 0.
- The `description:` of `docs/site/concepts/knowledge.md` names `know seed`, and the `llms.txt` Knowledge entry ends in the same sentence.
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'HelpText|Doc'`

**Commit:** `docs: describe capcom know seed and the base set`

## 9. Record the decision: ADR 0020 and the glossary

**What**
**Goal:** the Knowledge tree holds decision record `adr-0020` for the base set, a glossary term
for it, and a Note `knowledge` whose definition covers a rule the repository adopts.
**Approach (assumed at the header base):** written with `capcom know add` and `capcom know update`,
never by hand. `adr-0020` below `decisions`, titled in the form of its siblings, with the child
`adr-0020-decision`: capcom ships a base set of Notes embedded as markdown files; embedded-only
source, because a Note is an instruction; a seeded Note is the repo's own and is never overwritten,
except by `--update` while its text is still the seeded text; two Labels carry the seed and the
hash, so no store format change; the repo's own rule wins; the ids of the base set are chosen slugs read from the file names, so ADR 0002 holds. A glossary Note `base-set` below
`glossary-knowledge` in the form of its siblings ("Base set: ..."). `knowledge` amended with
`--body-replace-old/--body-replace-new` so its definition reads "facts about the repository, and
rules it adopts". Refs on the ADR: `internal/seed`.

**Regression guard.** The Note `decisions` lists every record under "The records:": add the
`adr-0020` line in the form of its siblings with `capcom know update decisions --body-append`.
The title of `knowledge` also reads "body of facts about the repository": update it with `--title`
so it covers the rules the repository adopts too.

**Files:** `.capcom/knowledge/decisions/adr-0020.md`, `.capcom/knowledge/decisions/adr-0020/adr-0020-decision.md`, `.capcom/knowledge/glossary/glossary-knowledge/base-set.md`, `.capcom/knowledge/glossary/glossary-knowledge/knowledge.md`, `.capcom/knowledge/decisions.md`
**Read first:** .capcom/knowledge/decisions.md — "The records:" list; .capcom/knowledge/decisions/adr-0019.md and adr-0019/adr-0019-decision.md — the sibling form (Status line, summary, "ADR NNNN decision: ..." title);
.capcom/knowledge/glossary/glossary-knowledge/knowledge.md — title and first body line ("facts about the repository" occurs once in the body); .capcom/knowledge/glossary/glossary-knowledge/label.md — sibling term form;
internal/cli/know_update.go — --body-replace-old/--body-replace-new (text must occur exactly once); .capcom/knowledge/docs-layout.md — records live in Knowledge, docs/adr only points
Depends on item 6.

**Tests**: none new.

**Acceptance**
- `capcom know show adr-0020 adr-0020-decision base-set knowledge` prints all four.
- `capcom know show knowledge | grep -c "rules it adopts"` prints a count above 0, and `capcom know show decisions | grep -c "adr-0020"` prints 1.
- `capcom check` and `capcom know verify` report no problem.

**Commit:** `docs(knowledge): record ADR 0020 and the base set in the glossary`
