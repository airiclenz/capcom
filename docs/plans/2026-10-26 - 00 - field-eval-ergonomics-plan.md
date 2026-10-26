# Plan: the field-evaluation ergonomics batch

**Goal:** Fix the five agent-ergonomics findings of the capcom-eval field evaluation: any-word
search fallback, search excerpts, id and value suggestions on errors, every field in `show --kv`,
and refs a fresh clone would lack.
**Date:** 2026-10-26
**Status:** draft — regression check pending
**sized for:** ~200k-context host
**base:** f150ecf

**Sources:**
- `.capcom/issues/agent-ergonomics/{search-any-word,search-excerpt,suggest-ids-values,kv-all-fields}.md`, `.capcom/issues/ref-outside-tree.md`
- Notes `cli-test-harness`, `coding-conventions`, `error-messages`, `store-layout`, `adr-0004`, `adr-0007`
- `docs/site/reference/json-output.md` (envelope, error body, `--kv` format)

**Ratified design calls (user, 2026-10-26):**
- **Any-word marker:** new top-level envelope field `match` (`"all"` or `"any"`), set only on kinds `search` and `know-search`; kinds unchanged; `SchemaVersion` unchanged.
- **Excerpt shape:** up to 3 body lines holding a query word, 1 line of context either side, overlapping windows merged; ≤ 8 lines per hit, each cut at 120 runes; JSON `excerpt` is a list of lines.
- **Error suggestions:** error body gains `suggestions` (≤ 3 closest ids/aliases) and `valid` (declared values; statuses as `name (category)`); the existing single `suggestion` (flag typos) is untouched.
- **Check code:** new diagnose kind `unshared-ref` for a ref into the git directory or to a git-ignored path; `broken-ref` keeps meaning "missing"; `know verify` reports it too.

**Writer calls (2026-10-26):**
- **UI filter:** `capcom ui` free-text filtering stays every-word; the fallback lives in the CLI.
- **kv keys:** every new key is always present, `PLAN=` included (the issue's "every key present" rule wins over its "when set").
- **Exit codes:** an invalid flag value keeps the exit code its command returns today (`list` usage, `update`/`create`/`close`/`reopen` invalid).
- **kv docs:** keys are documented in `docs/site/reference/json-output.md`; `docs/format.md` is a stub pointing at the site.

**Standing requirements:**
- skills: coding-standards
- Tests per `cli-test-harness`: `GOMEMLIMIT=2GiB`, one package at a time, never `./...` with `-race`.
- After changing a command's Short, Long, Example or flags: `go run ./cmd/gendocs` and commit `docs/site/reference/cli/` (`TestReferenceUpToDate`).
- A new file in `internal/cli` is added to the file map in `internal/cli/doc.go`.
- Error and warning text follows the `error-messages` Note; capcom runs no writing git command (ADR 0004).

**Out of scope:**
- `context-command` and `error-next-command` (unblocked by this plan, not done in it).
- Any-word fallback or excerpts in `capcom ui`.

## 1. Any-word search queries in the index — ✅ DONE (2026-10-26)

**What:**
**Goal:** `internal/index` offers any-word variants of Issue and Note search: an item matches when
it holds at least one query word, results ranked by FTS5 rank then id, FTS5 operator words literal.
`SearchIssues` and `SearchNotes` return exactly what they return at the base.
**Approach (assumed at the header base):** Extends `internal/index`; no new package. Add
`anyMatchExpression(words)` beside `matchExpression` (same quoting, joined with ` OR `). Add
`SearchIssuesAnyWord(text)` and `SearchNotesAnyWord(text, labels, isRetiredIncluded)`; share the SQL
with the every-word functions through a private parameter, never a copied query.
**Files:** internal/index/index.go; internal/index/query.go; internal/index/notes.go; internal/index/query_test.go; internal/index/notes_test.go
**Tests:** any-word finds items holding one of two words, ranked; a word in no item still returns
the items holding the others; `OR`, `NOT`, `NEAR`, `"` stay literal; labels and retired filters
apply to Notes as in `SearchNotes`.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/index/ -run 'Search'`; `go vet ./internal/index/`
**Commit:** `feat(index): add any-word Issue and Note search`

## 2. search and know search fall back to any-word matches

Depends on item 1.

**What:**
**Goal:** When the every-word query finds nothing, `capcom search` and `capcom know search` print the
any-word results under the heading line `No item holds every word; these hold some of them:`, and
the `--json` envelope carries `"match": "any"`; every-word results print exactly as at the base and
carry `"match": "all"`. With no any-word hits either, the not-found error is unchanged.
**Approach (assumed at the header base):** `output.Envelope` and `output.Result` gain `Match string`
(`json:"match,omitempty"`); only `newSearchCommand` (`internal/cli/search.go`) and
`newKnowSearchCommand` (`internal/cli/know_search.go`) set it. The text heading goes first in
`Result.Text`. Update Short/Long of both commands and every prose site saying search needs every
word (`grep -rn 'every word' internal/cli docs/site README.md`), then regenerate the CLI reference.
Document `match` in `docs/site/reference/json-output.md`.
**Files:** internal/cli/output/output.go; internal/cli/output/output_test.go; internal/cli/search.go; internal/cli/know_search.go; internal/cli/read_test.go; internal/cli/know_test.go; internal/cli/doc.go; docs/site/reference/json-output.md; docs/site/concepts/knowledge.md; README.md; docs/site/reference/cli/
**Closes:** search-any-word
**Tests:** CLI tests for Issues and Notes: one missing word gives the heading and ranked hits with
`match` `any`; all words matching gives the base output and `match` `all`;
`TestSearch_NoMatch_IsNotFound` still passes; operator words stay literal.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Search'`; `GOMEMLIMIT=2GiB go test ./internal/cli/output/`; `GOMEMLIMIT=2GiB go test ./internal/clidoc/`
**Commit:** `feat(search): fall back to any-word matches when no item holds every word`

## 3. Excerpt lines from a body

Depends on item 1.

**What:**
**Goal:** `internal/index` returns, for a body and a query text, the excerpt lines the ratified
excerpt shape prescribes, and returns the indexed body of an Issue or Note by path.
**Approach (assumed at the header base):** New `internal/index/excerpt.go` with a pure
`Excerpt(body, text string) []string`. A line matches when it holds any query word: line and word
are split into lower-cased letter/digit tokens, and a word matches when its token sequence occurs
in the line's (so `login-timeout-race` matches as FTS5 does). Take the first 3 matching lines,
add 1 line of context either side, merge overlapping windows, put a `…` line between non-adjacent
windows, stop at 8 lines, cut each line at 120 runes with `…`. Add `(x *Index) Body(path string)
(string, error)` reading `fts.body`.
**Files:** internal/index/excerpt.go; internal/index/excerpt_test.go; internal/index/index.go
**Tests:** table tests for `Excerpt`: one match, context at body start and end, merged windows,
separator, 3-line and 8-line caps, rune-safe cut, punctuated word, case folding, title-only match
gives no lines; `Body` returns the indexed body for an Issue and a Note.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/index/ -run 'Excerpt|Body'`
**Commit:** `feat(index): build search excerpts from indexed bodies`

## 4. --excerpt on search and know search

Depends on items 2 and 3.

**What:**
**Goal:** `capcom search <words> --excerpt` and `capcom know search <words> --excerpt` print each
hit's line followed by its excerpt lines, each indented four spaces; `--json` hits carry `excerpt`
(omitted without the flag). Without the flag the output is unchanged. The limit (3 matching lines,
8 lines, 120 characters per line) is stated in both commands' `--help`.
**Approach (assumed at the header base):** `issueData` and `noteData` gain
`Excerpt []string `json:"excerpt,omitempty"``. Both commands register `--excerpt`; the excerpt uses
the same query text in every-word and any-word mode. Regenerate the CLI reference; document
`excerpt` in `docs/site/reference/json-output.md`.
**Files:** internal/cli/search.go; internal/cli/know_search.go; internal/cli/issue_render.go; internal/cli/know.go; internal/cli/flags.go; internal/cli/read_test.go; internal/cli/know_test.go; docs/site/reference/json-output.md; docs/site/reference/cli/
**Closes:** search-excerpt
**Tests:** text and `--json` for Issues and Notes, with and without the flag, and with the any-word
fallback; a body longer than the cap gives at most 8 lines.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Search'`; `GOMEMLIMIT=2GiB go test ./internal/clidoc/`
**Commit:** `feat(search): print matching body lines with --excerpt`

## 5. Suggestions and valid values on the error envelope

**What:**
**Goal:** `output.Error` can carry a list of suggestions and a list of valid values. JSON error bodies
show them as `suggestions` and `valid` (omitted when empty); text appends `; did you mean a, b or c?`
and `; use one of: x, y, z`; `--kv` writes `SUGGESTIONS=` and `VALID=` (comma-separated) before
`STATUS=`. Errors without them render exactly as at the base.
**Approach (assumed at the header base):** Extend `output.Error`, `ErrorBody`, `errorBody`, and the
text and kv renderers' `RenderError` in `internal/cli/output`; the existing `Suggestion` field and
`textSuggestionFormat` stay for flag typos. `show`'s hand-built per-id kv failure record
(`internal/cli/show.go`) writes the same keys. Document both fields in `json-output.md`.
**Files:** internal/cli/output/errors.go; internal/cli/output/output.go; internal/cli/output/output_test.go; internal/cli/show.go; docs/site/reference/json-output.md
**Tests:** every format renders one, two and three suggestions and a valid list; empty lists render
as at the base (`TestRenderError_NoSuggestion_OmitsIt` still passes).
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/cli/output/`; `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Show'`
**Commit:** `feat(output): carry suggestions and valid values on errors`

## 6. An unknown id names the closest ids

Depends on item 5.

**What:**
**Goal:** Every Issue or Note id argument that resolves to nothing fails with up to three closest
ids or aliases of the same kind in `suggestions`; `capcom show standng-parents` suggests
`standing-parents`; nothing is suggested when nothing is close.
**Approach (assumed at the header base):** The index gains a light query of every non-excluded id
and alias per kind (beside `snapshotNames`). Ranking lives in `internal/cli/suggest.go`, reusing
`editDistance` and `suggestionDistance` from `root.go`: a candidate is close when its distance is at
most `suggestionDistance` or it shares a hyphen-separated word of 3+ letters with the argument;
order by distance, then shared words, then name. Attach it at every place in `internal/cli` that
maps `tree.ErrNotFound` to `CodeNotFound` (`resolveFailure` and its callers, `show`'s per-id
record, `close.go`, `know_update.go`; find them with `grep -rn 'ErrNotFound\|resolveFailure' internal/cli`).
**Files:** internal/index/query.go; internal/index/query_test.go; internal/cli/suggest.go; internal/cli/suggest_test.go; internal/cli/show.go; internal/cli/know.go; internal/cli/errors.go; internal/cli/read_test.go; internal/cli/know_test.go
**Tests:** Issue and Note ids, an alias suggested, a shared-word match, nothing close, text and `--json`.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Suggest|Show|Know'`; `GOMEMLIMIT=2GiB go test ./internal/index/ -run 'Name|Resolve'`
**Commit:** `feat(cli): suggest the closest ids for an unknown id`

## 7. An invalid --status, --type or --priority lists the valid values

Depends on item 5.

**What:**
**Goal:** `list`, `create`, `update`, `close` and `reopen` reject an undeclared `--status`, `--type`,
`--category` or an out-of-range `--priority` before writing, with the declared values in `valid`
(statuses as `name (category)` from `.capcom/config.yaml`, priorities `0`..`4`), keeping each
command's exit code at the base. `update --status doing` and `update --type bugg` list them.
**Approach (assumed at the header base):** One helper in `internal/cli` builds the valid lists from
`config.Config` and `issue.MinPriority`/`MaxPriority`; `checkListValues` (`list.go`), `targetStatus`
(`reopen.go`), and new early checks in `create.go` and `update.go` call it. Category rules in
`issue.SetStatus` (closed status → reopen) stay as they are.
**Files:** internal/cli/list.go; internal/cli/reopen.go; internal/cli/create.go; internal/cli/update.go; internal/cli/flags.go; internal/cli/read_test.go; internal/cli/write_test.go
**Closes:** suggest-ids-values
**Tests:** each command and flag, text and `--json`; a valid value writes as before.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'List|Create|Update|Close|Reopen'`
**Commit:** `feat(cli): list the declared values for an invalid flag value`

## 8. show --kv prints every field

**What:**
**Goal:** `capcom show <id> --kv` prints, after `TITLE=`, `TYPE=`, `PRIORITY=`, `LABELS=`, `PARENT=`,
`DEPS=` and `PLAN=`, each always present (empty value when unset), lists comma-separated;
`STATUS=` stays last. `docs/site/reference/json-output.md` lists every `show --kv` key.
**Approach (assumed at the header base):** Extend `issueKV` in `internal/cli/issue_render.go` from
`index.Entry` (`Type`, `Priority`, `Labels`, `ParentID`, `Deps`, `Plan`), joining with
`strings.Join(v, ",")`. `start --kv` reuses the record through `emitStarted`; update its tests.
**Files:** internal/cli/issue_render.go; internal/cli/read_test.go; internal/cli/start_test.go; docs/site/reference/json-output.md
**Closes:** kv-all-fields
**Tests:** after `update --add-label alpha --type bug --priority 3`, `show --kv` prints `TYPE=bug`,
`PRIORITY=3`, `LABELS=alpha`; an issue with a parent and deps prints them; empty fields print empty.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'KV|Start'`
**Commit:** `feat(show): print type, priority, labels, parent, deps and plan in --kv`

## 9. gitx reports which paths git ignores

**What:**
**Goal:** `internal/gitx` offers a read-only call that, given a worktree and paths, returns the
subset git ignores (tracked files never count as ignored); none ignored is not an error.
**Approach (assumed at the header base):** `IgnoredPaths(dir string, paths []string) ([]string, error)`
running `git check-ignore -z --stdin` once per call under `GIT_OPTIONAL_LOCKS=0`; exit code 1
means none ignored. Extends `gitx`; the private `run` helper gains stdin only if needed.
**Files:** internal/gitx/gitx.go; internal/gitx/ignore.go; internal/gitx/ignore_test.go
**Tests:** real temp repo (ADR 0007): ignored file, unignored file, tracked file matching a pattern,
no paths, none ignored.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/gitx/`
**Commit:** `feat(gitx): report which paths git ignores`

## 10. check and know verify report refs a clone would lack

Depends on item 9.

**What:**
**Goal:** `capcom check` and `capcom know verify` report `unshared-ref`, naming the Note and the ref,
for a ref whose first path component is `.git` or that resolves inside the git directory or the git
common directory, and for a ref to a git-ignored path, even when the file exists here.
**Approach (assumed at the header base):** `internal/knowledge` gains the classification beside
`checkRef`, taking the git dir, common dir and an ignored-path set; `Verify`'s callers pass them
(`(*checker).checkNote` in `internal/check/notes.go`, `verifyNotes` in `internal/cli/know_verify.go`),
calling `gitx.IgnoredPaths` once per run. Add `UnsharedRef` to `diagnose.Kind` and to
`TestCodes_ClosedSet_PinsEveryCodeString`.
**Files:** internal/knowledge/verify.go; internal/knowledge/knowledge_test.go; internal/diagnose/diagnose.go; internal/check/notes.go; internal/check/check_test.go; internal/cli/know_verify.go; internal/cli/know_test.go
**Tests:** check fails on an existing `.git/HEAD` ref and an ignored-file ref; a tracked ref passes;
retired subtrees skipped as for `broken-ref`; `know verify` lists both.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/knowledge/`; `GOMEMLIMIT=2GiB go test ./internal/check/`; `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'KnowVerify'`
**Commit:** `feat(check): report Note refs into the git directory or to ignored files`

## 11. know add and know update refuse git-dir refs and warn on missing or ignored ones

Depends on item 10.

**What:**
**Goal:** `capcom know add --ref` and `capcom know update --add-ref` refuse a ref item 10 classes as
inside the git directory, before writing, with an error naming the ref; a ref to a missing or
git-ignored path is written with a warning naming the path.
**Approach (assumed at the header base):** `addNote` (`know_add.go`) and `applyNoteUpdate`
(`know_update.go`) call item 10's classification after `knowledge.ValidateRef`; warnings go through
`output.Result.Warnings`. Mention the rule in both commands' Long and regenerate the CLI reference.
**Files:** internal/cli/know_add.go; internal/cli/know_update.go; internal/cli/know_test.go; internal/cli/know_update_test.go; docs/site/reference/cli/
**Closes:** ref-outside-tree
**Tests:** `know add --id x --title X --ref .git/HEAD` fails without writing; missing and ignored
refs warn and write; same for `know update --add-ref`; text and `--json`.
**Acceptance:** `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'KnowAdd|KnowUpdate'`; `GOMEMLIMIT=2GiB go test ./internal/clidoc/`
**Commit:** `feat(know): refuse refs into the git directory and warn on unshared ones`
